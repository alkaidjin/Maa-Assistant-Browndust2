package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 自动化计划任务：每天定时 / 开机(登录)自启，二选一，由用户决定。
const (
	taskDaily = "MaaBrownDust2Daily"
	taskLogon = "MaaBrownDust2Logon"
)

func autoBatPath(root string) string { return filepath.Join(root, "logs", "auto-run.bat") }

// ensureAutoBat 生成计划任务调用的脚本（纯 ASCII，避免 cmd 中文乱码）。
func ensureAutoBat(root string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0o755); err != nil {
		return "", err
	}
	lines := []string{
		"@echo off",
		`cd /d "` + root + `"`,
		`echo ===== start %date% %time% ===== >> logs\auto-run.log`,
		`"` + exe + `" unlock >> logs\auto-run.log 2>&1`,
		`"` + exe + `" run DailyAll --timeout 150m --wait-game 5m --json >> logs\auto-run.log 2>&1`,
		`echo ===== done %date% %time% ===== >> logs\auto-run.log`,
	}
	p := autoBatPath(root)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o644); err != nil {
		return "", err
	}
	return p, nil
}

func taskExists(name string) bool {
	cmd := exec.Command("schtasks", "/query", "/tn", name)
	cmd.SysProcAttr = hiddenWindowAttr()
	return cmd.Run() == nil
}

func createScheduledTask(name, tr, schedule, at string) error {
	args := []string{"/create", "/tn", name, "/tr", tr, "/sc", schedule, "/RL", "HIGHEST", "/F"}
	if schedule == "DAILY" && at != "" {
		args = append(args, "/st", at)
	}
	cmd := exec.Command("schtasks", args...)
	cmd.SysProcAttr = hiddenWindowAttr()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func deleteScheduledTask(name string) error {
	cmd := exec.Command("schtasks", "/delete", "/tn", name, "/F")
	cmd.SysProcAttr = hiddenWindowAttr()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// cmdAuto 管理"自动化"：每天定时 / 开机自启，两者二选一（也可都开）。
//
//	bd2cli autostart                查看状态与用法
//	bd2cli autostart daily [HH:MM]  每天定时（默认 08:30）
//	bd2cli autostart logon          开机(登录)自启
//	bd2cli autostart off            全部关闭
func cmdAuto(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	bat, err := ensureAutoBat(root)
	if err != nil {
		return fail(err, exitFailed)
	}

	var pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		pos = append(pos, a)
	}
	sub := ""
	if len(pos) > 0 {
		sub = pos[0]
	}

	switch sub {
	case "", "status":
		return printJSON(map[string]any{
			"daily": map[string]any{"task": taskDaily, "installed": taskExists(taskDaily)},
			"logon": map[string]any{"task": taskLogon, "installed": taskExists(taskLogon)},
			"script": bat,
			"usage": []string{
				"bd2cli autostart daily 08:30   每天定时跑（推荐）",
				"bd2cli autostart logon         开机/登录后自动跑",
				"bd2cli autostart off           关闭全部自动化",
			},
			"note": "两者都基于 Windows 计划任务，最高权限运行；创建时需一次管理员授权",
		})

	case "daily":
		at := "08:30"
		if len(pos) > 1 {
			at = pos[1]
		}
		if v := valueOf(args, "--at"); v != "" {
			at = v
		}
		if _, err := time.Parse("15:04", at); err != nil {
			return fail(fmt.Errorf("时间格式应为 HH:MM，例如 08:30（收到 %q）", at), exitUsage)
		}
		if err := createScheduledTask(taskDaily, bat, "DAILY", at); err != nil {
			fmt.Println("需要一次管理员授权以创建计划任务……（请在 UAC 弹窗点“是”）")
			if e2 := relaunchAsAdmin(root, "autostart", "daily", at); e2 != nil {
				return fail(fmt.Errorf("创建失败：%v", e2), exitPermission)
			}
		}
		if !taskExists(taskDaily) {
			return fail(fmt.Errorf("创建后仍未检测到计划任务"), exitFailed)
		}
		fmt.Printf("已设置：每天 %s 自动跑全部任务（计划任务 %s）\n", at, taskDaily)
		return exitOK

	case "logon":
		if err := createScheduledTask(taskLogon, bat, "ONLOGON", ""); err != nil {
			fmt.Println("需要一次管理员授权以创建计划任务……（请在 UAC 弹窗点“是”）")
			if e2 := relaunchAsAdmin(root, "autostart", "logon"); e2 != nil {
				return fail(fmt.Errorf("创建失败：%v", e2), exitPermission)
			}
		}
		if !taskExists(taskLogon) {
			return fail(fmt.Errorf("创建后仍未检测到计划任务"), exitFailed)
		}
		fmt.Printf("已设置：开机/登录后自动跑全部任务（计划任务 %s）\n", taskLogon)
		return exitOK

	case "off":
		removed := []string{}
		for _, n := range []string{taskDaily, taskLogon} {
			if !taskExists(n) {
				continue
			}
			if err := deleteScheduledTask(n); err != nil {
				if e2 := relaunchAsAdmin(root, "autostart", "off"); e2 != nil {
					return fail(fmt.Errorf("关闭失败：%v", e2), exitPermission)
				}
				removed = append(removed, n)
				continue
			}
			removed = append(removed, n)
		}
		if len(removed) == 0 {
			fmt.Println("当前没有开启任何自动化")
		} else {
			fmt.Printf("已关闭：%s\n", strings.Join(removed, "、"))
		}
		return exitOK
	}
	return fail(fmt.Errorf("用法: bd2cli autostart [daily HH:MM | logon | off | status]"), exitUsage)
}
