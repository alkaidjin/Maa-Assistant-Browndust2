package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"bd2cli/internal/logx"
	"bd2cli/internal/mxucfg"
	"bd2cli/internal/project"
	"bd2cli/internal/runs"
)

func stopFlagPath(root string) string { return filepath.Join(root, "logs", ".stop-requested") }

// cmdControl 停止当前这一轮（pause/resume MXU 不支持，会明确拒绝）。
func cmdControl(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	action := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && a != "current" {
			action = a
			break
		}
	}
	store := runs.New(filepath.Join(root, "logs", "runs"))
	active, busy := store.Active()

	switch action {
	case "stop":
		if !busy {
			fmt.Println("当前没有在跑的任务")
			return exitOK
		}
		_ = os.WriteFile(stopFlagPath(root), []byte(active.ID), 0o644)
		// 先礼后兵：通知 MXU 退出；超时则强杀
		_ = exec.Command("taskkill", "/IM", "mxu.exe").Run()
		time.Sleep(2 * time.Second)
		if processRunning("mxu.exe") {
			_ = exec.Command("taskkill", "/IM", "mxu.exe", "/F").Run()
		}
		fmt.Printf("已请求停止：%s（游戏与 MXU 会在收尾时按配置关闭）\n", active.ID)
		return exitOK
	case "pause", "resume":
		return fail(fmt.Errorf("MXU 没有暂停接口，%s 不支持；请用 bdcli control stop 结束本轮", action), exitPermission)
	case "", "status":
		return printJSON(map[string]any{
			"active_run": active,
			"supported":  []string{"stop", "status"},
			"unsupported": []string{"pause", "resume"},
			"note":       "MXU 不提供暂停/继续，只能停止整轮",
		})
	case "clear":
		_ = os.Remove(stopFlagPath(root))
		fmt.Println("已清除停止标记")
		return exitOK
	}
	return fail(fmt.Errorf("用法: bd2cli control stop|status|clear"), exitUsage)
}

// cmdFollow 跟随某一轮直到结束，并持续打印最新日志（对齐三月七的 --follow）。
func cmdFollow(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	id := valueOf(args, "--id")
	lines := 20
	if v := valueOf(args, "--lines"); v != "" {
		fmt.Sscanf(v, "%d", &lines)
	}
	store := runs.New(filepath.Join(root, "logs", "runs"))

	pick := func() *runs.Record {
		if id != "" {
			if r, err := store.Get(id); err == nil {
				return r
			}
		}
		recs, _ := store.Recent(1)
		if len(recs) > 0 {
			return recs[0]
		}
		return nil
	}

	rec := pick()
	if rec == nil {
		return fail(fmt.Errorf("还没有运行记录"), exitFailed)
	}
	fmt.Printf("跟随 %s（%s）\n", rec.ID, rec.Task)

	lastLen := 0
	for {
		cur := pick()
		if cur == nil || cur.ID != rec.ID {
			cur = rec
		}
		if t, err := logx.Read(filepath.Join(root, "debug", "maafw.log"), lines, 512<<10); err == nil && len(t.LastLines) > lastLen {
			for _, ln := range t.LastLines[lastLen:] {
				fmt.Println(ln)
			}
			lastLen = len(t.LastLines)
		}
		if cur.Status != runs.StatusRunning {
			fmt.Printf("\n结论：%s · %s · 用时 %s\n", cur.Status, cur.Task, humanDuration(cur.DurationS))
			if cur.Reason != "" {
				fmt.Printf("原因：%s\n", cur.Reason)
			}
			if cur.Status == runs.StatusSuccess {
				return exitOK
			}
			return exitFailed
		}
		time.Sleep(2 * time.Second)
	}
}

// cmdDoctor 环境体检：一次性告诉你"缺什么、卡在哪一步"（对齐三月七的状态字段）。
func cmdDoctor(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	checks := []map[string]any{}
	add := func(name string, ok bool, detail string, fix string) {
		checks = append(checks, map[string]any{"check": name, "ok": ok, "detail": detail, "fix": fix})
	}

	// 1) 项目文件
	pi, perr := project.Load(root)
	add("interface.json", perr == nil, errText(perr, "已解析"), "确认在项目根目录，或解压完整用户包")

	// 2) mxu.exe 位置（根目录才算正常）
	mxu := filepath.Join(root, "mxu.exe")
	_, statErr := os.Stat(mxu)
	cacheMxu := filepath.Join(root, "cache", "_mxu_253", "mxu.exe")
	_, cacheErr := os.Stat(cacheMxu)
	add("mxu.exe", statErr == nil,
		ternary(statErr == nil, "在项目根目录", "不在根目录（源码目录常见）"),
		ternary(cacheErr == nil, "把 cache\\_mxu_253\\mxu.exe 复制到项目根目录", "从 Releases 重新解压用户包"))

	// 3) MaaFramework 运行库
	dlls, _ := filepath.Glob(filepath.Join(root, "maafw", "*.dll"))
	add("maafw 运行库", len(dlls) >= 10, fmt.Sprintf("找到 %d 个 DLL", len(dlls)), "重新解压 maafw 目录")

	// 4) 资源
	_, rerr := os.Stat(filepath.Join(root, "resource", "pipeline"))
	add("resource 资源", rerr == nil, errText(rerr, "pipeline 目录存在"), "重新解压 resource 目录")

	// 5) agent 可执行
	agents, _ := filepath.Glob(filepath.Join(root, "agent", "*.exe"))
	add("agent 程序", len(agents) > 0, fmt.Sprintf("找到 %d 个", len(agents)), "重新解压 agent 目录")

	// 6) 一次性授权（免 UAC 计划任务）
	add("免 UAC 授权", taskInstalled(), ternary(taskInstalled(), "已授权", "未授权"), "执行 bdcli elevate install（需用户点一次 UAC）")

	// 7) 本次会话的运行锁
	store := runs.New(filepath.Join(root, "logs", "runs"))
	active, busy := store.Active()
	lockDetail, lockFix := "空闲", ""
	if busy && active != nil {
		lockDetail = "有任务在跑：" + active.ID
		lockFix = "等待结束，或 bdcli unlock 清残留锁"
	}
	add("运行锁", !busy, lockDetail, lockFix)

	// 8) 是否已经跑成功过（首跑确认的替代指标）
	recs, _ := store.Recent(50)
	firstRun := false
	for _, r := range recs {
		if r.Status == runs.StatusSuccess {
			firstRun = true
			break
		}
	}
	add("首次成功运行", firstRun, ternary(firstRun, "已有成功记录", "还没有成功记录"), "先跑一次最短任务，例如 奖励一键领取")

	// 9) 游戏路径与窗口
	var inst *mxucfg.Instance
	if p, err := mxucfg.Find(root); err == nil {
		if c, err := mxucfg.Load(p); err == nil {
			inst = c.Active()
		}
	}
	gamePath := ""
	if inst != nil {
		gamePath = inst.SavedDevice.ConnectedProgramPath
	}
	add("游戏路径", gamePath != "", ternary(gamePath != "", gamePath, "配置里没有保存游戏路径"), "在 MXU 里连接一次游戏")

	failed := 0
	for _, c := range checks {
		if c["ok"] == false {
			failed++
		}
	}
	name := "MaaBrownDust2"
	if pi != nil && pi.Name != "" {
		name = pi.Name
	}
	fmt.Printf("%s 体检：%d 项通过，%d 项需要处理\n\n", name, len(checks)-failed, failed)
	for _, c := range checks {
		mark := "✓"
		if c["ok"] == false {
			mark = "✗"
		}
		fmt.Printf("  %s %-14s %v\n", mark, c["check"], c["detail"])
		if c["ok"] == false && c["fix"] != "" {
			fmt.Printf("      → %v\n", c["fix"])
		}
	}
	if code := printJSON(checks); code != exitOK {
		return code
	}
	if failed > 0 {
		return exitFailed
	}
	return exitOK
}

func errText(err error, ok string) string {
	if err != nil {
		return err.Error()
	}
	return ok
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
