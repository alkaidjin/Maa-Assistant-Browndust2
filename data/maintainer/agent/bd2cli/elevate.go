package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 与三月七小助手 March7thAssistantNoUAC 同一套做法：
// 用「计划任务 + 最高权限」绕开每次 UAC。创建时只授权一次，之后由 schtasks /run 触发，
// 不再弹窗。
const elevTaskName = "MaaBrownDust2NoUAC"

type elevCmd struct {
	Nonce   string   `json:"nonce"`
	Op      string   `json:"op"`
	Targets []string `json:"targets,omitempty"`
}

type elevResult struct {
	Nonce string   `json:"nonce"`
	Done  bool     `json:"done"`
	Log   []string `json:"log,omitempty"`
	Error string   `json:"error,omitempty"`
}

func elevDir(root string) string        { return filepath.Join(root, "logs", "elev") }
func elevCmdPath(root string) string    { return filepath.Join(elevDir(root), "cmd.json") }
func elevResultPath(root string) string { return filepath.Join(elevDir(root), "result.json") }

// taskScriptPath 是计划任务真正执行的那一行——用 .bat 包一层，
// 避免 schtasks 对带引号参数的处理差异（实测直接写带参数命令行会得到 Last Result=2）。
func taskScriptPath(root string) string { return filepath.Join(elevDir(root), "elev-task.bat") }

// taskInstalled 判断计划任务是否已创建。
func taskInstalled() bool {
	cmd := exec.Command("schtasks", "/query", "/tn", elevTaskName)
	cmd.SysProcAttr = hiddenWindowAttr()
	return cmd.Run() == nil
}

// installTask 创建计划任务（需要管理员，只需一次）。
func installTask(root string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(elevDir(root), 0o755); err != nil {
		return err
	}
	script := fmt.Sprintf("@echo off\r\n\"%s\" elev-run --root \"%s\"\r\n", exe, root)
	if err := os.WriteFile(taskScriptPath(root), []byte(script), 0o644); err != nil {
		return err
	}
	tr := taskScriptPath(root)
	cmd := exec.Command("schtasks", "/create",
		"/tn", elevTaskName,
		"/tr", tr,
		"/sc", "ONCE", "/st", "23:59",
		"/RL", "HIGHEST",
		"/F")
	cmd.SysProcAttr = hiddenWindowAttr()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("创建计划任务失败（需要管理员权限）: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// removeTask 删除计划任务。
func removeTask() error {
	cmd := exec.Command("schtasks", "/delete", "/tn", elevTaskName, "/F")
	cmd.SysProcAttr = hiddenWindowAttr()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("删除失败: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// relaunchAsAdmin 用一次 UAC 把自己提权执行指定子命令。
func relaunchAsAdmin(root string, sub ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := append(sub, "--root", root)
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", "''")+"'")
	}
	ps := fmt.Sprintf("Start-Process -Verb RunAs -FilePath '%s' -ArgumentList %s -Wait",
		strings.ReplaceAll(exe, "'", "''"), strings.Join(quoted, ","))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = hiddenWindowAttr()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("提权失败（可能取消 UAC）: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runElevated 通过计划任务以管理员身份执行一次操作，并等它完成。
func runElevated(root, op string, targets []string, timeout time.Duration) (elevResult, error) {
	var res elevResult
	if !taskInstalled() {
		return res, fmt.Errorf("计划任务 %s 未安装", elevTaskName)
	}
	if err := os.MkdirAll(elevDir(root), 0o755); err != nil {
		return res, err
	}
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	payload, _ := json.Marshal(elevCmd{Nonce: nonce, Op: op, Targets: targets})
	if err := os.WriteFile(elevCmdPath(root), payload, 0o644); err != nil {
		return res, err
	}
	_ = os.Remove(elevResultPath(root))

	run := exec.Command("schtasks", "/run", "/tn", elevTaskName)
	run.SysProcAttr = hiddenWindowAttr()
	if out, err := run.CombinedOutput(); err != nil {
		return res, fmt.Errorf("触发计划任务失败: %v\n%s", err, strings.TrimSpace(string(out)))
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		b, err := os.ReadFile(elevResultPath(root))
		if err != nil {
			continue
		}
		if err := json.Unmarshal(b, &res); err != nil {
			continue
		}
		if res.Nonce == nonce && res.Done {
			return res, nil
		}
	}
	return res, fmt.Errorf("等待提权操作超时")
}

// cmdElevate 是用户可见的管理入口。
func cmdElevate(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	sub := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			sub = a
			break
		}
	}
	switch sub {
	case "", "status":
		installed := taskInstalled()
		return printJSON(map[string]any{
			"task":      elevTaskName,
			"installed": installed,
			"hint":      hintText(installed),
		})
	case "install":
		if taskInstalled() {
			fmt.Println("计划任务已存在，无需重复安装")
			return exitOK
		}
		if err := installTask(root); err != nil {
			fmt.Println("当前权限不足，正在请求一次 UAC 授权……（请在弹窗中点“是”）")
			if err2 := relaunchAsAdmin(root, "elevate", "install"); err2 != nil {
				return fail(fmt.Errorf("%v", err2), exitPermission)
			}
		}
		if !taskInstalled() {
			return fail(fmt.Errorf("安装后仍未检测到计划任务"), exitFailed)
		}
		fmt.Printf("已安装计划任务 %s（高权限、无需再授权）\n", elevTaskName)
		return exitOK
	case "remove":
		if !taskInstalled() {
			fmt.Println("计划任务不存在")
			return exitOK
		}
		if err := removeTask(); err != nil {
			if err2 := relaunchAsAdmin(root, "elevate", "remove"); err2 != nil {
				return fail(fmt.Errorf("%v", err2), exitPermission)
			}
		}
		fmt.Println("已删除计划任务")
		return exitOK
	case "kill":
		var targets []string
		for _, a := range args {
			if !strings.HasPrefix(a, "-") && a != "kill" {
				targets = append(targets, a)
			}
		}
		if len(targets) == 0 {
			targets = []string{"mxu.exe", "go-service.exe", "BrownDust II.exe"}
		}
		er, err := runElevated(root, "kill", targets, 45*time.Second)
		if err != nil {
			return fail(err, exitFailed)
		}
		return printJSON(er)
	case "run":
		return cmdElevRun(root)
	}
	return fail(fmt.Errorf("用法: bd2cli elevate status|install|remove"), exitUsage)
}

func hintText(installed bool) string {
	if installed {
		return "已启用免 UAC：关窗口/发输入会自动通过计划任务提权"
	}
	return "未启用：执行 bd2cli elevate install（会弹一次 UAC）后即可全自动"
}

// cmdElevRun 由计划任务以管理员身份拉起，处理一条待执行命令后退出。
func cmdElevRun(root string) int {
	b, err := os.ReadFile(elevCmdPath(root))
	if err != nil {
		return fail(fmt.Errorf("读取命令失败: %w", err), exitFailed)
	}
	var c elevCmd
	if err := json.Unmarshal(b, &c); err != nil {
		return fail(err, exitFailed)
	}
	res := elevResult{Nonce: c.Nonce, Done: true}
	switch c.Op {
	case "kill":
		for _, name := range c.Targets {
			if killProcess(name) {
				res.Log = append(res.Log, "已结束 "+name)
			} else {
				res.Log = append(res.Log, "结束失败 "+name)
				res.Error = "有进程未能结束"
			}
		}
	default:
		res.Error = "未知操作: " + c.Op
	}
	out, _ := json.Marshal(res)
	_ = os.MkdirAll(elevDir(root), 0o755)
	_ = os.WriteFile(elevResultPath(root), out, 0o644)
	_ = os.Remove(elevCmdPath(root))
	return exitOK
}
