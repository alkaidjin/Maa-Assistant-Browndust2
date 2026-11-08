package tasker

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/MaaXYZ/maa-framework-go/v4/controller/win32"
)

// ActionResult 是一次单动作（点击/按键）的回执，附带动作后的截图。
type ActionResult struct {
	Action      string `json:"action"`
	Point       string `json:"point,omitempty"`
	InputMethod string `json:"input_method"`
	Ok          bool   `json:"ok"`
	PNG         string `json:"png,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Act 做一次最小动作：先建控制器，再点击或按键，最后（可选）截图留证。
//
// kind: "click" 用 x,y；"key" 用 vk。
// 坐标是**截图坐标系**（与 inputcheck 存下来的 PNG 同一套），MaaFramework 自己换算到屏幕。
func Act(cfg Config, hwnd unsafe.Pointer, kind string, x, y, vk int32, shotPath string, wait time.Duration) ActionResult {
	r := ActionResult{Action: kind, InputMethod: cfg.MouseMethod}
	if kind == "key" {
		r.InputMethod = cfg.KeyboardMethod
	}
	if hwnd == nil {
		r.Error = fmt.Sprintf("找不到游戏窗口（class=%q title=%q）", cfg.Class, cfg.WindowTitle)
		return r
	}

	if !maa.IsInited() {
		if err := maa.Init(maa.WithLibDir(filepath.Join(cfg.Root, "maafw"))); err != nil {
			r.Error = fmt.Sprintf("初始化 MaaFramework 失败: %v", err)
			return r
		}
	}

	screencap, err := win32.ParseScreencapMethod(cfg.ScreencapMethod)
	if err != nil {
		r.Error = fmt.Sprintf("截图方式 %q 非法: %v", cfg.ScreencapMethod, err)
		return r
	}
	mouse, err := win32.ParseInputMethod(cfg.MouseMethod)
	if err != nil {
		r.Error = fmt.Sprintf("鼠标方式 %q 非法: %v", cfg.MouseMethod, err)
		return r
	}
	keyboard, err := win32.ParseInputMethod(cfg.KeyboardMethod)
	if err != nil {
		r.Error = fmt.Sprintf("键盘方式 %q 非法: %v", cfg.KeyboardMethod, err)
		return r
	}

	ctrl, err := maa.NewWin32Controller(hwnd, screencap, mouse, keyboard)
	if err != nil {
		r.Error = fmt.Sprintf("创建 Win32 控制器失败: %v", err)
		return r
	}
	conn := ctrl.PostConnect()
	conn.Wait()
	if !conn.Success() {
		r.Error = "连接游戏窗口失败"
		return r
	}

	var job *maa.Job
	switch kind {
	case "click":
		r.Point = fmt.Sprintf("%d,%d", x, y)
		job = ctrl.PostClick(x, y)
	case "key":
		r.Point = fmt.Sprintf("vk=%#x", vk)
		job = ctrl.PostClickKey(vk)
	default:
		r.Error = "未知动作: " + kind
		return r
	}
	job.Wait()
	r.Ok = job.Success()

	if wait > 0 {
		time.Sleep(wait)
	}
	if shotPath != "" {
		sc := ctrl.PostScreencap()
		sc.Wait()
		if sc.Success() {
			if img, err := ctrl.CacheImage(); err == nil {
				if err := os.MkdirAll(filepath.Dir(shotPath), 0o755); err == nil {
					if f, err := os.Create(shotPath); err == nil {
						_ = png.Encode(f, img)
						_ = f.Close()
						r.PNG = shotPath
					}
				}
			}
		}
	}
	if !r.Ok {
		r.Error = "动作失败（MaaFramework 返回失败）"
	}
	return r
}
