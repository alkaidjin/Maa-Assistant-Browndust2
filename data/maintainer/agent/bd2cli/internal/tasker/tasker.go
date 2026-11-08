// Package tasker 用 MaaFramework 的 Go 绑定在进程内跑**单个任务**。
//
// 它替代 runner.mxu 的粗粒度（整实例），是 P2 的核心。
// 依赖 github.com/MaaXYZ/maa-framework-go/v4（与 agent/go-service 同一个版本）。
package tasker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"
	"unsafe"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/MaaXYZ/maa-framework-go/v4/controller/win32"
)

// 退出码，与 main 保持同一套语义。
const (
	exitOK        = 0
	exitFailed    = 1
	exitPreflight = 3
)

// Config 是一次单任务运行的输入。
type Config struct {
	Root            string
	Task            string
	Entry           string
	Override        map[string]any
	Class           string
	WindowTitle     string
	ScreencapMethod string
	MouseMethod     string
	KeyboardMethod  string
	Timeout         time.Duration
}

// SelfCheckResult 是 selfcheck 的输出：证明 MaaFramework 能初始化、resource 能加载。
type SelfCheckResult struct {
	MaaFW    string `json:"maafw"`
	Resource string `json:"resource"`
	InitMS   int64  `json:"init_ms"`
	BundleOK bool   `json:"bundle_ok"`
	BundleMS int64  `json:"bundle_ms"`
	Error    string `json:"error,omitempty"`
}

// SelfCheck 不需要游戏窗口，只验证 DLL 初始化与资源加载。
func SelfCheck(root string) SelfCheckResult {
	r := SelfCheckResult{
		MaaFW:    filepath.Join(root, "maafw"),
		Resource: filepath.Join(root, "resource"),
	}
	if !maa.IsInited() {
		start := time.Now()
		if err := maa.Init(maa.WithLibDir(r.MaaFW)); err != nil {
			r.Error = fmt.Sprintf("初始化失败: %v", err)
			return r
		}
		r.InitMS = time.Since(start).Milliseconds()
	}
	res, err := maa.NewResource()
	if err != nil {
		r.Error = fmt.Sprintf("创建 Resource 失败: %v", err)
		return r
	}
	start := time.Now()
	job := res.PostBundle(r.Resource)
	job.Wait()
	r.BundleMS = time.Since(start).Milliseconds()
	r.BundleOK = job.Success()
	if !r.BundleOK {
		r.Error = "resource 加载失败"
	}
	return r
}

// Run 在进程内初始化 MaaFramework 并跑 task.Entry，返回退出码与详情 JSON。
//
// hwnd 为 0 表示没找到游戏窗口，直接返回 exitPreflight。
func Run(ctx context.Context, cfg Config, hwnd unsafe.Pointer, out io.Writer) (int, string, error) {
	if hwnd == nil {
		return exitPreflight, "", fmt.Errorf("找不到游戏窗口（class=%q title=%q），请先启动游戏", cfg.Class, cfg.WindowTitle)
	}

	if !maa.IsInited() {
		if err := maa.Init(maa.WithLibDir(filepath.Join(cfg.Root, "maafw"))); err != nil {
			return exitPreflight, "", fmt.Errorf("初始化 MaaFramework（maafw 目录）失败: %w", err)
		}
	}

	screencap, err := win32.ParseScreencapMethod(cfg.ScreencapMethod)
	if err != nil {
		return exitPreflight, "", fmt.Errorf("截图方式 %q 非法: %w", cfg.ScreencapMethod, err)
	}
	mouse, err := win32.ParseInputMethod(cfg.MouseMethod)
	if err != nil {
		return exitPreflight, "", fmt.Errorf("鼠标方式 %q 非法: %w", cfg.MouseMethod, err)
	}
	keyboard, err := win32.ParseInputMethod(cfg.KeyboardMethod)
	if err != nil {
		return exitPreflight, "", fmt.Errorf("键盘方式 %q 非法: %w", cfg.KeyboardMethod, err)
	}

	res, err := maa.NewResource()
	if err != nil {
		return exitPreflight, "", fmt.Errorf("创建 Resource 失败: %w", err)
	}
	job := res.PostBundle(filepath.Join(cfg.Root, "resource"))
	job.Wait()
	if job.Failure() {
		return exitFailed, "", fmt.Errorf("加载 resource 失败（%s）", filepath.Join(cfg.Root, "resource"))
	}

	ctrl, err := maa.NewWin32Controller(hwnd, screencap, mouse, keyboard)
	if err != nil {
		return exitPreflight, "", fmt.Errorf("创建 Win32 控制器失败: %w", err)
	}
	conn := ctrl.PostConnect()
	conn.Wait()
	if !conn.Success() {
		return exitPreflight, "", fmt.Errorf("连接游戏窗口失败（窗口句柄 %p）", hwnd)
	}

	t, err := maa.NewTasker()
	if err != nil {
		return exitPreflight, "", fmt.Errorf("创建 Tasker 失败: %w", err)
	}
	if err := t.BindResource(res); err != nil {
		return exitPreflight, "", fmt.Errorf("绑定 Resource 失败: %w", err)
	}
	if err := t.BindController(ctrl); err != nil {
		return exitPreflight, "", fmt.Errorf("绑定 Controller 失败: %w", err)
	}

	fmt.Fprintf(out, "[tasker] entry=%s 覆盖节点数=%d\n", cfg.Entry, len(cfg.Override))
	tj := t.PostTask(cfg.Entry, cfg.Override)

	done := make(chan struct{})
	go func() {
		tj.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return exitFailed, "", fmt.Errorf("超过 timeout，已放弃等待（任务可能在后台继续）: %w", ctx.Err())
	}

	detail, derr := tj.GetDetail()
	detailJSON := ""
	if derr == nil && detail != nil {
		if b, err := json.MarshalIndent(detail, "", "  "); err == nil {
			detailJSON = string(b)
		}
	}
	if tj.Success() {
		return exitOK, detailJSON, nil
	}
	if derr != nil {
		return exitFailed, detailJSON, fmt.Errorf("任务失败（取详情失败: %v）", derr)
	}
	return exitFailed, detailJSON, fmt.Errorf("任务失败")
}
