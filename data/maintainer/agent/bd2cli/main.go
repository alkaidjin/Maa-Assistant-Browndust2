// bd2cli 是给 AI Agent 用的命令行外壳：查状态、跑任务、改配置、读日志。
//
// 设计见 docs/agent-cli-design.md。退出码契约与三月七小助手的 m7a.exe 对齐。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bd2cli/internal/logx"
	"bd2cli/internal/mxucfg"
	"bd2cli/internal/notify"
	"bd2cli/internal/project"
	"bd2cli/internal/runs"
	"bd2cli/internal/runner"
	"bd2cli/internal/status"
	"bd2cli/internal/tasker"
)

const cliVersion = "0.1.0"

// 退出码契约。
const (
	exitOK         = 0
	exitFailed     = 1
	exitUsage      = 2
	exitPreflight  = 3
	exitBusy       = 4
	exitPermission = 5
	exitAborted    = 6
)

func main() { os.Exit(realMain(os.Args[1:])) }

func realMain(args []string) int {
	if len(args) == 0 {
		usage()
		return exitUsage
	}
	switch args[0] {
	case "--version", "-v":
		fmt.Printf("bd2cli %s\n", cliVersion)
		return exitOK
	case "--help", "-h", "help":
		usage()
		return exitOK
	case "--status":
		return cmdStatus(args[1:])
	case "--list":
		return cmdList(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "config":
		return cmdConfig(args[1:])
	case "logs":
		return cmdLogs(args[1:])
	case "selfcheck":
		return cmdSelfCheck(args[1:])
	case "inputcheck":
		return cmdInputCheck(args[1:])
	case "click":
		return cmdAct("click", args[1:])
	case "key":
		return cmdAct("key", args[1:])
	case "only":
		return cmdOnly(args[1:])
	case "unlock":
		return cmdUnlock(args[1:])
	case "elevate":
		return cmdElevate(args[1:])
	case "autostart":
		return cmdAuto(args[1:])
	case "notify":
		return cmdNotify(args[1:])
	case "report":
		return cmdReport(args[1:])
	case "diagnose":
		return cmdDiagnose(args[1:])
	case "result":
		return cmdResult(args[1:])
	case "wait":
		return cmdWait(args[1:])
	case "ends":
		return cmdEnds(args[1:])
	case "follow":
		return cmdFollow(args[1:])
	case "control":
		return cmdControl(args[1:])
	case "doctor":
		return cmdDoctor(args[1:])
	case "update":
		return cmdUpdate(args[1:])
	case "elev-run":
		root, err := rootFrom(args[1:])
		if err != nil {
			return fail(err, exitPreflight)
		}
		return cmdElevRun(root)
	case "rawkey":
		return cmdRawKey(args[1:])
	case "rawclick":
		return cmdRawClick(args[1:])
	}
	fmt.Fprintf(os.Stderr, "未知命令 %q\n", args[0])
	usage()
	return exitUsage
}

func usage() {
	fmt.Fprint(os.Stderr, `bd2cli `+cliVersion+` —— Maa-Assistant-Browndust2 的 AI 命令行外壳

用法（在含 interface.json 的目录执行）：
  bd2cli --status [--json]              查状态：MXU、游戏窗口、实例、运行中任务、最近 5 次运行、日志尾部
  bd2cli --list [--json]                列全部任务 ID / label / 入口节点 / 选项
  bd2cli doctor                         环境体检（9 项：文件/MXU/maafw/资源/agent/授权/锁/首跑/游戏路径）
  bd2cli run <任务名> [--json] [--timeout 30m] [--wait-game 5m] [--no-close|--keep-game] [--no-retry] [--dry-run]
                                        启动→等结束→判定→关窗口→出卡片；失败只重试失败项
  bd2cli follow [--id X] [--lines 20]   跟随当前这一轮直到结束
  bd2cli control stop|status|clear      停止整轮（MXU 无暂停接口，pause/resume 不支持）
  bd2cli result                         最近一轮结果（JSON，含卡片路径）
  bd2cli wait [--timeout 3h]            阻塞到这一轮结束再输出结果
  bd2cli report [--id X] [--embed] [--history]  结果卡片（只针对最近一轮）
  bd2cli diagnose [--id X]              打包失败证据（截图/日志/脱敏配置）
  bd2cli ends                           打印每个任务的完成标志节点
  bd2cli only <任务名> | --restore      临时只跑一个任务 / 还原勾选
  bd2cli unlock                         清残留锁（进程被强杀后）
  bd2cli elevate status|install|remove|kill
                                        一次性授权（建计划任务，之后免 UAC）
  bd2cli autostart [daily HH:MM|logon|off|status]  自动化：每天定时 / 开机自启
  bd2cli notify status|test             可选手机推送通道
  bd2cli inputcheck [--to x,y]          体检：能否截图、能否动鼠标、窗口是否接收消息
  bd2cli selfcheck                      只验证 MaaFramework 初始化与 resource 加载
  bd2cli config get|set mxu.<路径> [值]  改 MXU 配置单键（自动 .bak）
  bd2cli logs tail [--lines 50]         打印 debug\maafw.log 尾部

退出码：0 成功 / 1 任务失败 / 2 参数错误 / 3 前置条件不满足 / 4 已有任务在跑 /
        5 权限或能力不支持 / 6 被中止

说明：run 默认 runner=mxu，跑的是该实例已启用的任务列表；只跑一个任务请先 only <任务名>。
      run 默认等待任务结束并自动关闭 MXU 与游戏（需先 elevate install 授权一次）。
      任何失败先看 logs\runs\<id>.json 与 debug\on_error\*.png，再查别的。
`)
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// valueOf 取 "--name value" 里的 value。
func valueOf(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"=")
		}
	}
	return ""
}

func rootFrom(args []string) (string, error) {
	if r := valueOf(args, "--root"); r != "" {
		return project.FindRoot(r)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if root, err := project.FindRoot(cwd); err == nil {
		return root, nil
	}
	exe, err := os.Executable()
	if err == nil {
		return project.FindRoot(filepath.Dir(exe))
	}
	return "", fmt.Errorf("找不到 interface.json（请在项目根目录执行，或用 --root 指定）")
}

func fail(err error, code int) int {
	fmt.Fprintf(os.Stderr, "错误: %v\n", err)
	return code
}

func printJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fail(err, exitFailed)
	}
	return exitOK
}

func cmdStatus(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	rep, err := status.Build(root, cliVersion, checks())
	if err != nil {
		return fail(err, exitPreflight)
	}
	if hasFlag(args, "--json") {
		return printJSON(rep)
	}
	fmt.Printf("工程      %s %s\n", rep.Project.Name, rep.Project.Version)
	fmt.Printf("MXU       running=%v  web=%v(port %d, reachable=%v)\n",
		rep.MXU.Running, rep.MXU.Web.Enabled, rep.MXU.Web.Port, rep.MXU.Web.Reachable)
	fmt.Printf("游戏窗口  found=%v  (%s / %s)  process=%v\n",
		rep.Game.WindowFound, rep.Game.Class, rep.Game.Title, rep.Game.ProcessFound)
	if rep.Instance != nil {
		fmt.Printf("实例      %s  controller=%s  resource=%s\n",
			rep.Instance.Name, rep.Instance.Controller, rep.Instance.Resource)
		for _, t := range rep.Instance.Tasks {
			mark := "  "
			if t.Enabled {
				mark = "✓ "
			}
			fmt.Printf("          %s%s\n", mark, t.Name)
		}
	}
	if rep.ActiveRun != nil {
		fmt.Printf("运行中    %s  task=%s  since=%s\n",
			rep.ActiveRun.ID, rep.ActiveRun.Task, rep.ActiveRun.StartedAt.Format(time.RFC3339))
	} else {
		fmt.Println("运行中    （无）")
	}
	if len(rep.RecentRuns) > 0 {
		fmt.Println("最近运行")
		for _, r := range rep.RecentRuns {
			fmt.Printf("          %s  %-8s exit=%-3d %.0fs  %s\n",
				r.ID, r.Status, r.ExitCode, r.DurationS, r.Error)
		}
	}
	if rep.Log != nil {
		fmt.Printf("日志      %s (%d bytes)\n", rep.Log.Path, rep.Log.Bytes)
		if rep.Log.LastError != "" {
			fmt.Printf("最后报错  %s\n", rep.Log.LastError)
		}
	}
	return exitOK
}

func cmdList(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	pi, err := project.Load(root)
	if err != nil {
		return fail(err, exitPreflight)
	}
	if hasFlag(args, "--json") {
		return printJSON(struct {
			Name    string         `json:"name"`
			Version string         `json:"version"`
			Root    string         `json:"root"`
			Tasks   []project.Task `json:"tasks"`
		}{pi.Name, pi.Version, pi.Root, pi.Task})
	}
	fmt.Printf("%s %s  (%s)\n", pi.Name, pi.Version, pi.Root)
	for _, t := range pi.Task {
		fmt.Printf("  %-24s entry=%-16s %s\n", t.Name, t.Entry, t.Label)
	}
	return exitOK
}

func cmdRun(args []string) int {
	var task, instance, notifyPath, rootFlag, mxuFlag, runnerFlag string
	var js, dryRun, waitMode, closeAfter, keepGame, noNotify, noRetry bool
	waitMode, closeAfter = true, true
	var waitGame time.Duration
	timeout := 30 * time.Minute
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--json":
			js = true
		case "--dry-run":
			dryRun = true
		case "--no-wait":
			waitMode = false
		case "--no-close":
			closeAfter = false
		case "--keep-game":
			keepGame = true
		case "--no-notify":
			noNotify = true
		case "--no-retry":
			noRetry = true
		case "--wait-game":
			if i+1 >= len(args) {
				return fail(fmt.Errorf("--wait-game 缺参数（例如 5m）"), exitUsage)
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fail(fmt.Errorf("--wait-game 需要时长，例如 5m"), exitUsage)
			}
			waitGame = d
		case "--root", "--instance", "--notify-file", "--timeout", "--mxu", "--runner":
			if i+1 >= len(args) {
				return fail(fmt.Errorf("%s 缺参数", a), exitUsage)
			}
			i++
			switch a {
			case "--root":
				rootFlag = args[i]
			case "--mxu":
				mxuFlag = args[i]
			case "--runner":
				runnerFlag = args[i]
			case "--instance":
				instance = args[i]
			case "--notify-file":
				notifyPath = args[i]
			case "--timeout":
				d, err := time.ParseDuration(args[i])
				if err != nil {
					return fail(err, exitUsage)
				}
				timeout = d
			}
		default:
			if strings.HasPrefix(a, "-") {
				return fail(fmt.Errorf("未知参数 %s", a), exitUsage)
			}
			task = a
		}
	}

	root, err := rootFrom([]string{"--root", rootFlag})
	if rootFlag == "" {
		root, err = rootFrom(nil)
	}
	if err != nil {
		return fail(err, exitPreflight)
	}
	pi, err := project.Load(root)
	if err != nil {
		return fail(err, exitPreflight)
	}

	cfgPath, cfgErr := mxucfg.Find(root)
	var cfg *mxucfg.Config
	var inst *mxucfg.Instance
	if cfgErr == nil {
		if cfg, err = mxucfg.Load(cfgPath); err == nil {
			if instance == "" {
				inst = cfg.Active()
			} else {
				inst = cfg.InstanceByName(instance)
			}
		}
	}
	if instance == "" && inst != nil {
		instance = inst.Name
	}

	if task == "" {
		return fail(fmt.Errorf("缺少任务名（用 bd2cli --list 查看）"), exitUsage)
	}
	if _, ok := pi.TaskByName(task); !ok {
		return fail(fmt.Errorf("未知任务 %q（用 bd2cli --list 查看）", task), exitUsage)
	}
	if inst != nil {
		found, enabled := false, false
		for _, it := range inst.Tasks {
			if it.TaskName == task || matchesLabel(pi, it.TaskName, task) {
				found = true
				enabled = it.Enabled
				break
			}
		}
		if !found {
			return fail(fmt.Errorf("任务 %q 不在实例 %q 的任务列表里，MXU 不会执行它", task, inst.Name), exitPreflight)
		}
		if !enabled {
			return fail(fmt.Errorf("任务 %q 在实例 %q 中被禁用", task, inst.Name), exitPreflight)
		}
	}

	exe, err := resolveMXU(root, mxuFlag)
	if err != nil {
		return fail(err, exitPreflight)
	}
	runnerName := runnerFlag
	if runnerName == "" {
		runnerName = "mxu"
	}
	switch runnerName {
	case "mxu", "tasker":
	default:
		return fail(fmt.Errorf("未知 runner %q（mxu|tasker）", runnerName), exitUsage)
	}

	var tk tasker.Config
	if runnerName == "tasker" {
		taskDef, _ := pi.TaskByName(task)
		class, title, screencap, mouse, keyboard := controllerInfo(pi, inst)
		tk = tasker.Config{
			Root:            root,
			Task:            task,
			Entry:           taskDef.Entry,
			Override:        project.BuildOverride(*taskDef, pi.Option, selectedOptions(pi, inst, task)),
			Class:           class,
			WindowTitle:     title,
			ScreencapMethod: screencap,
			MouseMethod:     mouse,
			KeyboardMethod:  keyboard,
			Timeout:         timeout,
		}
	}

	if dryRun {
		if runnerName == "mxu" {
			fmt.Printf("runner: mxu\nexe:  %s\nargs: %s\ncwd:  %s\n",
				exe, strings.Join(runner.MXU{}.Args(runner.Job{Instance: instance}), " "), root)
		} else {
			fmt.Printf("runner: tasker\ntask:  %s\nentry: %s\noverride nodes: %d\nwindow: %s / %s\nscreencap: %s  mouse: %s  keyboard: %s\n",
				tk.Task, tk.Entry, len(tk.Override), tk.Class, tk.WindowTitle,
				tk.ScreencapMethod, tk.MouseMethod, tk.KeyboardMethod)
		}
		return exitOK
	}

	store := runs.New(filepath.Join(root, "logs", "runs"))
	_ = os.Remove(stopFlagPath(root)) // 清掉上一轮的停止标记：黑屏说明游戏没加载完/网络异常；给了 --wait-game 就先等（必要时拉起游戏）
	if runnerName == "mxu" && waitMode {
		class, title, sc, ms, kb := controllerInfo(pi, inst)
		check := func() float64 {
			chk := tasker.CheckInput(tasker.Config{
				Root: root, Class: class, WindowTitle: title,
				ScreencapMethod: sc, MouseMethod: ms, KeyboardMethod: kb,
			}, findWindowPtr(class, title), "")
			if !chk.Screencap {
				return -1
			}
			return chk.MeanLuma
		}
		luma := check()
		if luma >= 0 && luma < 10 && waitGame > 0 {
			if inst != nil && inst.SavedDevice.ConnectedProgramPath != "" && !processRunning("BrownDust II.exe") {
				if err := exec.Command(inst.SavedDevice.ConnectedProgramPath).Start(); err == nil {
					fmt.Fprintf(os.Stderr, "[bdcli] 游戏未运行，已拉起；等待画面就绪（最多 %s）\n", waitGame)
				}
			}
			deadline := time.Now().Add(waitGame)
			for time.Now().Before(deadline) {
				time.Sleep(5 * time.Second)
				if v := check(); v >= 10 {
					luma = v
					fmt.Fprintf(os.Stderr, "[bdcli] 画面已就绪（亮度 %.0f）\n", v)
					break
				}
			}
		}
		if luma >= 0 && luma < 10 {
			return fail(fmt.Errorf("游戏画面是全黑的（平均亮度 %.1f/255）：多半还在加载或网络异常；"+
				"请等游戏进入主界面后再跑（用 bd2cli inputcheck 可看当前画面）", luma), exitPreflight)
		}
	}

	rec, release, err := store.Acquire(task, instance)
	if err == runs.ErrBusy {
		active, _ := store.Active()
		if active != nil {
			fmt.Fprintf(os.Stderr, "已有任务在运行: %s (task=%s)\n", active.ID, active.Task)
		}
		return exitBusy
	}
	if err != nil {
		return fail(err, exitFailed)
	}
	defer release()
	rec.CLIVersion = cliVersion
	rec.Runner = runnerName
	rec.LogPath = filepath.Join("debug", "maafw.log")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var code int
	var runErr error
	var mxuRes *mxuOutcome
	if runnerName == "tasker" {
		fmt.Fprintf(os.Stderr, "[bd2cli] %s tasker entry=%s override=%d\n", rec.ID, tk.Entry, len(tk.Override))
		code, _, runErr = tasker.Run(ctx, tk, findWindowPtr(tk.Class, tk.WindowTitle), os.Stdout)
	} else if waitMode {
		instID := instance
		if inst != nil {
			instID = inst.ID
		}
		if cfg != nil {
			if err := cfg.EnsureAutoRun(instID); err != nil {
				fmt.Fprintf(os.Stderr, "[bd2cli] 警告：写自动执行开关失败: %v\n", err)
			}
		}
		entry := ""
		if taskDef, ok := pi.TaskByName(task); ok {
			entry = taskDef.Entry
		}
		// 计划表：实例里已启用的任务（用于逐任务汇报与"只重试失败项"）
		var plan []taskPlan
		if inst != nil {
			for _, it := range inst.Tasks {
				if strings.HasPrefix(it.TaskName, "__MXU_PRETASK__") || !it.Enabled {
					continue
				}
				label, ent := it.TaskName, ""
				if td, ok := pi.TaskByName(it.TaskName); ok {
					if td.Label != "" {
						label = td.Label
					}
					ent = td.Entry
				}
				plan = append(plan, taskPlan{TaskName: it.TaskName, Label: label, Entry: ent})
			}
		}
		var restore func()
		defer func() {
			if restore != nil {
				restore()
			}
		}()
		// 一次跑多个任务时，用"自动化名称"作为这一轮的名字（如：棕色尘埃2日常）
		displayTask := task
		if len(plan) > 1 {
			displayTask = automationName(root)
		}
		rec.Task = displayTask
		attempts := 1
		if !noRetry {
			attempts = 2 // 失败自动重试一次
		}
		for i := 0; i < attempts; i++ {
			if i > 0 {
				// 重试会重启 MXU（清空 debug/），先把上一次的证据固化下来
				_ = preserveEvidence(root, rec.ID)
				// 只重跑失败的任务：临时把其他任务关掉，跑完自动还原
				names := failedTaskNames(mxuRes)
				if nr := loadNoRetry(root); len(nr) > 0 {
					kept := names[:0]
					for _, n := range names {
						if nr[n] {
							fmt.Fprintf(os.Stderr, "[bdcli] %s 在免重试名单里，跳过重试\n", n)
							continue
						}
						kept = append(kept, n)
					}
					names = kept
				}
				if len(names) == 0 {
					fmt.Fprintf(os.Stderr, "[bdcli] 没有需要重试的任务\n")
					break
				}
				if r, ferr := focusOnly(cfg, inst, names); ferr == nil {
					restore = r
					fmt.Fprintf(os.Stderr, "[bdcli] 只重跑失败任务：%s\n", strings.Join(names, "、"))
				}
				rec = store.NewRecord()
				rec.Task, rec.Instance, rec.Runner = displayTask, instance, "mxu"
				rec.CLIVersion = cliVersion
				rec.LogPath = filepath.Join("debug", "maafw.log")
				if err := store.Save(rec); err != nil {
					break
				}
				fmt.Fprintf(os.Stderr, "[bdcli] 自动重试一次（%s）\n", rec.ID)
			}
			fmt.Fprintf(os.Stderr, "[bdcli] %s 启动 MXU 并等待 entry=%s（最多 %s）\n", rec.ID, entry, timeout)
			out := runMXUAndWait(root, exe, instID, entry, plan, timeout, closeAfter, closeAfter && !keepGame, os.Stdout)
			mxuRes = &out
			code, runErr = out.Code, out.Err
			rec.Reason = out.Reason
			if out.Status == runs.StatusSuccess || i == attempts-1 {
				break
			}
			_ = store.Finalize(rec, out.Status, out.Code, out.Err)
		}
	} else {
		r := runner.MXU{Exe: exe, Dir: root}
		fmt.Fprintf(os.Stderr, "[bd2cli] %s 启动：%s\n", rec.ID, strings.Join(r.Args(runner.Job{Instance: instance}), " "))
		code, runErr = r.Run(ctx, runner.Job{Task: task, Instance: instance, Timeout: timeout}, os.Stdout)
	}

	var state, reason string
	if mxuRes != nil {
		state, reason = mxuRes.Status, mxuRes.Reason
		rec.Nodes = mxuRes.Nodes
		for _, t := range mxuRes.Tasks {
			rec.Tasks = append(rec.Tasks, runs.TaskDone{Name: t.Label, OK: t.OK})
		}
	} else {
		state = runs.StatusSuccess
		switch {
		case runErr != nil && ctx.Err() == context.DeadlineExceeded:
			state, reason, code = runs.StatusTimeout, fmt.Sprintf("超过 --timeout %s", timeout), exitFailed
		case runErr != nil:
			state, code = runs.StatusFailed, exitFailed
		case code != 0:
			state, reason = runs.StatusFailed, fmt.Sprintf("MXU 退出码 %d", code)
		}
	}
	rec.Reason = reason
	if err := store.Finalize(rec, state, code, runErr); err != nil {
		fmt.Fprintf(os.Stderr, "写运行记录失败: %v\n", err)
	}

	var nodes []string
	if mxuRes != nil {
		nodes = mxuRes.Nodes
	}
	if reportPath, rerr := emitReport(root, rec, nodes); rerr == nil {
		fmt.Fprintf(os.Stderr, "[bdcli] 看板: %s\n", reportPath)
	}
	writeLatest(root, taskLabel(root, rec.Task), rec)

	if state != runs.StatusSuccess {
		if p, derr := collectDiagnosis(root, rec, task, state, reason); derr == nil {
			rec.Diagnose = p
			_ = store.Save(rec)
			fmt.Fprintf(os.Stderr, "[bdcli] 诊断包: %s（可直接发给开发者）\n", p)
		} else {
			fmt.Fprintf(os.Stderr, "[bdcli] 诊断包生成失败: %v\n", derr)
		}
	}

	if !noNotify {
		n, desc := notify.Load(root)
		if notifyPath != "" {
			n = notify.File{Path: notifyPath}
			desc = "file(" + notifyPath + ")"
		}
		body := fmt.Sprintf("任务: %s\n实例: %s\n状态: %s\n耗时: %.0fs\n退出码: %d\n记录: logs/runs/%s.json",
			task, instance, state, rec.DurationS, code, rec.ID)
		if reason != "" {
			body += "\n原因: " + reason
		}
		if err := n.Notify(fmt.Sprintf("bd2cli %s — %s", state, task), body); err != nil {
			fmt.Fprintf(os.Stderr, "[bd2cli] 通知失败（%s）: %v\n", desc, err)
		} else if desc != "已关闭" {
			fmt.Fprintf(os.Stderr, "[bd2cli] 已通知: %s\n", desc)
		}
	}

	if js {
		printJSON(rec)
	} else {
		fmt.Printf("%s %s  %.0fs  exit=%d  logs/runs/%s.json\n", rec.ID, state, rec.DurationS, code, rec.ID)
	}

	if state == runs.StatusSuccess {
		return exitOK
	}
	if code >= 1 && code <= 6 {
		return code
	}
	return exitFailed
}

// failedTaskNames 从一次运行结果里取出失败任务的实例任务名。
func failedTaskNames(out *mxuOutcome) []string {
	if out == nil {
		return nil
	}
	var names []string
	for _, t := range out.Tasks {
		if !t.OK {
			names = append(names, t.TaskName)
		}
	}
	return names
}

// focusOnly 临时只启用指定任务（保留前置任务），返回还原函数。
func focusOnly(cfg *mxucfg.Config, inst *mxucfg.Instance, names []string) (func(), error) {
	if cfg == nil || inst == nil {
		return nil, fmt.Errorf("没有可用的 MXU 配置")
	}
	idx := cfg.IndexOf(inst)
	if idx < 0 {
		return nil, fmt.Errorf("找不到实例下标")
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	prev := map[string]bool{}
	for _, t := range inst.Tasks {
		prev[t.TaskName] = t.Enabled
		if strings.HasPrefix(t.TaskName, "__MXU_PRETASK__") {
			continue
		}
		if _, err := cfg.SetTaskEnabled(idx, t.TaskName, want[t.TaskName]); err != nil {
			return nil, err
		}
	}
	if err := cfg.Save(); err != nil {
		return nil, err
	}
	restore := func() {
		for _, t := range inst.Tasks {
			if strings.HasPrefix(t.TaskName, "__MXU_PRETASK__") {
				continue
			}
			if v, ok := prev[t.TaskName]; ok {
				_, _ = cfg.SetTaskEnabled(idx, t.TaskName, v)
			}
		}
		_ = cfg.Save()
	}
	return restore, nil
}

func matchesLabel(pi *project.Interface, instanceTaskName, wanted string) bool {
	t, ok := pi.TaskByName(instanceTaskName)
	if !ok {
		return false
	}
	return t.Name == wanted || t.Label == wanted
}

// resolveMXU 按 --mxu 覆盖 > 工程根 > 开发用缓存目录 的顺序找 mxu.exe。
func resolveMXU(root, override string) (string, error) {
	cands := make([]string, 0, 3)
	if override != "" {
		cands = append(cands, override)
	}
	cands = append(cands,
		filepath.Join(root, "mxu.exe"),
		filepath.Join(root, "cache", "_mxu_253", "mxu.exe"),
	)
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("找不到 mxu.exe（试过: %s）；可用 --mxu 指定", strings.Join(cands, ", "))
}

// controllerInfo 取出实例对应控制器的窗口与输入方式（tasker 需要）。
func controllerInfo(pi *project.Interface, inst *mxucfg.Instance) (class, title, screencap, mouse, keyboard string) {
	pick := func(c project.Controller) bool { return c.Type == "Win32" && c.Win32 != nil }
	want := ""
	if inst != nil {
		want = inst.ControllerName
	}
	grab := func(c project.Controller) (string, string, string, string, string) {
		return c.Win32.ClassRegex, c.Win32.WindowRegex, c.Win32.Screencap, c.Win32.Mouse, c.Win32.Keyboard
	}
	for _, c := range pi.Controller {
		if pick(c) && (want == "" || c.Name == want) {
			return grab(c)
		}
	}
	for _, c := range pi.Controller {
		if pick(c) {
			return grab(c)
		}
	}
	return "", "", "", "", ""
}

// selectedOptions 把 MXU 里该任务的选项取值转成 option 名 → 选中的 case 名。
func selectedOptions(pi *project.Interface, inst *mxucfg.Instance, task string) map[string][]string {
	out := map[string][]string{}
	if inst == nil {
		return out
	}
	for _, it := range inst.Tasks {
		if it.TaskName != task && !matchesLabel(pi, it.TaskName, task) {
			continue
		}
		for name, v := range it.OptionValues {
			if v.CaseName != "" {
				out[name] = []string{v.CaseName}
			}
			if len(v.CaseNames) > 0 {
				out[name] = v.CaseNames
			}
		}
		break
	}
	return out
}

func cmdConfig(args []string) int {
	if len(args) < 2 {
		return fail(fmt.Errorf("用法: bd2cli config get|set <mxu.路径> [值]"), exitUsage)
	}
	sub := args[0]
	rawPath := args[1]
	parts := strings.Split(strings.TrimPrefix(rawPath, "mxu."), ".")
	if !strings.HasPrefix(rawPath, "mxu.") {
		return fail(fmt.Errorf("只支持 mxu.* 路径，例如 mxu.settings.webServerPort"), exitUsage)
	}
	for _, p := range parts {
		low := strings.ToLower(p)
		if strings.Contains(low, "cdk") || strings.Contains(low, "token") || strings.Contains(low, "password") {
			return fail(fmt.Errorf("%s 是敏感字段，请让用户在 MXU 界面里填写", rawPath), exitPermission)
		}
	}
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	cfgPath, err := mxucfg.Find(root)
	if err != nil {
		return fail(err, exitPreflight)
	}
	cfg, err := mxucfg.Load(cfgPath)
	if err != nil {
		return fail(err, exitPreflight)
	}

	switch sub {
	case "get":
		v, ok := cfg.Get(parts)
		if !ok {
			return fail(fmt.Errorf("没有这个配置项: %s", rawPath), exitUsage)
		}
		return printJSON(v)
	case "set":
		if len(args) < 3 {
			return fail(fmt.Errorf("用法: bd2cli config set %s <值>", rawPath), exitUsage)
		}
		if err := cfg.Set(parts, args[2]); err != nil {
			return fail(err, exitUsage)
		}
		if err := cfg.Save(); err != nil {
			return fail(err, exitFailed)
		}
		fmt.Printf("已写入 %s = %s（原文件已备份为 .bak）\n", rawPath, args[2])
		return exitOK
	}
	return fail(fmt.Errorf("未知子命令 %q（get|set）", sub), exitUsage)
}

// cmdSelfCheck 只验证 MaaFramework 初始化与 resource 加载，不需要游戏在跑。
func cmdSelfCheck(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	res := tasker.SelfCheck(root)
	if code := printJSON(res); code != exitOK {
		return code
	}
	if !res.BundleOK {
		return exitFailed
	}
	return exitOK
}

// cmdInputCheck 把"能不能动鼠标"和"能不能看见游戏"分开验证：只截图，不点击。
func cmdInputCheck(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	pi, err := project.Load(root)
	if err != nil {
		return fail(err, exitPreflight)
	}
	var inst *mxucfg.Instance
	if p, err := mxucfg.Find(root); err == nil {
		if c, err := mxucfg.Load(p); err == nil {
			inst = c.Active()
		}
	}
	class, title, screencap, mouse, keyboard := controllerInfo(pi, inst)

	out := map[string]any{}

	// 1) 本进程有没有资格动鼠标（直接调 Windows 接口，不经过 MaaFramework）。
	x, y, got := rawGetCursorPos()
	cursor := map[string]any{"x": x, "y": y, "got_pos": got}
	ok, errno := rawSetCursorPos(x, y)
	cursor["set_same_ok"] = ok
	cursor["set_same_errno"] = int(errno)
	if to := valueOf(args, "--to"); to != "" {
		var tx, ty int
		if n, _ := fmt.Sscanf(to, "%d,%d", &tx, &ty); n == 2 {
			ok2, errno2 := rawSetCursorPos(tx, ty)
			cursor["set_to"] = fmt.Sprintf("%d,%d", tx, ty)
			cursor["set_to_ok"] = ok2
			cursor["set_to_errno"] = int(errno2)
			_, _ = rawSetCursorPos(x, y) // 立刻把鼠标还回去
			cursor["restored"] = true
		}
	}
	out["cursor"] = cursor

	// 1.5) 游戏窗口愿不愿意接受我们的消息（UIPI 拦截会返回错误 5）。
	if hw := findWindowHandle(class, title); hw != 0 {
		okMsg, errnoMsg := rawSendMessageTimeout(hw)
		out["window_message"] = map[string]any{
			"accepted": okMsg,
			"errno":    int(errnoMsg),
		}
	}

	// 2) MaaFramework 能不能看见游戏（只截图，不点击）。
	png := filepath.Join(root, "logs", "inputcheck-"+time.Now().Format("20060102-150405")+".png")
	chk := tasker.CheckInput(tasker.Config{
		Root:            root,
		Class:           class,
		WindowTitle:     title,
		ScreencapMethod: screencap,
		MouseMethod:     mouse,
		KeyboardMethod:  keyboard,
	}, findWindowPtr(class, title), png)
	out["capture"] = chk

	if code := printJSON(out); code != exitOK {
		return code
	}
	if !chk.Screencap {
		return exitFailed
	}
	return exitOK
}

// cmdAct 做一次单动作（click / key），动作后自动截图留证。
func cmdAct(kind string, args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	pi, err := project.Load(root)
	if err != nil {
		return fail(err, exitPreflight)
	}
	var inst *mxucfg.Instance
	if p, err := mxucfg.Find(root); err == nil {
		if c, err := mxucfg.Load(p); err == nil {
			inst = c.Active()
		}
	}
	class, title, screencap, mouse, keyboard := controllerInfo(pi, inst)
	if v := valueOf(args, "--mouse"); v != "" {
		mouse = v
	}
	if v := valueOf(args, "--keyboard"); v != "" {
		keyboard = v
	}

	// 位置参数：写完就遇到 - 开头的开关
	var pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		pos = append(pos, a)
	}

	var x, y, vk int32
	switch kind {
	case "click":
		if len(pos) < 2 {
			return fail(fmt.Errorf("用法: bd2cli click <x> <y> [--mouse SendMessage]"), exitUsage)
		}
		fmt.Sscanf(pos[0], "%d", &x)
		fmt.Sscanf(pos[1], "%d", &y)
	case "key":
		if len(pos) < 1 {
			return fail(fmt.Errorf("用法: bd2cli key <J 或 0x4A> [--keyboard SendMessage]"), exitUsage)
		}
		v, err := parseVK(pos[0])
		if err != nil {
			return fail(err, exitUsage)
		}
		vk = v
	}

	shot := valueOf(args, "--shot")
	if shot == "" {
		shot = filepath.Join(root, "logs", kind+"-"+time.Now().Format("20060102-150405")+".png")
	}
	wait := 1200 * time.Millisecond
	if v := valueOf(args, "--wait"); v != "" {
		var ms int
		if n, _ := fmt.Sscanf(v, "%d", &ms); n == 1 {
			wait = time.Duration(ms) * time.Millisecond
		}
	}

	res := tasker.Act(tasker.Config{
		Root:            root,
		Class:           class,
		WindowTitle:     title,
		ScreencapMethod: screencap,
		MouseMethod:     mouse,
		KeyboardMethod:  keyboard,
	}, findWindowPtr(class, title), kind, x, y, vk, shot, wait)

	if code := printJSON(res); code != exitOK {
		return code
	}
	if !res.Ok {
		return exitFailed
	}
	return exitOK
}

// parseVK 把 "J" / "F10" / "0x79" / "121" 统一成虚拟键码。
func parseVK(s string) (int32, error) {
	if len(s) == 1 {
		c := s[0]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			return int32(c), nil
		}
	}
	if len(s) >= 2 && (s[0] == 'F' || s[0] == 'f') {
		if n, err := strconv.Atoi(s[1:]); err == nil && n >= 1 && n <= 24 {
			return int32(0x6F + n), nil // F1 = 0x70
		}
	}
	if v, err := strconv.ParseInt(s, 0, 32); err == nil && v > 0 {
		return int32(v), nil
	}
	return 0, fmt.Errorf("看不懂的按键 %q（可写 J、F10 或 0x79）", s)
}

// cmdOnly 临时只保留一个任务（其余非前置任务全部关掉），跑完用 --restore 还原。
func cmdOnly(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	cfgPath, err := mxucfg.Find(root)
	if err != nil {
		return fail(err, exitPreflight)
	}
	cfg, err := mxucfg.Load(cfgPath)
	if err != nil {
		return fail(err, exitPreflight)
	}
	inst := cfg.Active()
	if inst == nil {
		return fail(fmt.Errorf("配置里没有实例"), exitPreflight)
	}
	idx := cfg.IndexOf(inst)
	if idx < 0 {
		return fail(fmt.Errorf("找不到当前实例的下标"), exitPreflight)
	}

	if hasFlag(args, "--restore") {
		bak := cfgPath + ".bak"
		b, err := os.ReadFile(bak)
		if err != nil {
			return fail(fmt.Errorf("没有可还原的备份 %s", bak), exitPreflight)
		}
		if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
			return fail(err, exitFailed)
		}
		fmt.Printf("已从 %s 还原配置\n", bak)
		return exitOK
	}

	target := ""
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		target = a
	}
	if target == "" {
		return fail(fmt.Errorf("用法: bd2cli only <任务名> | bd2cli only --restore"), exitUsage)
	}
	// 允许用 label 指代
	for _, name := range cfg.TaskNames(idx) {
		if name == target || matchesLabel(mustProject(root), name, target) {
			target = name
			break
		}
	}

	kept := []string{}
	disabled := []string{}
	for _, name := range cfg.TaskNames(idx) {
		if strings.HasPrefix(name, "__MXU_PRETASK__") {
			kept = append(kept, name) // 前置任务（启动游戏）永远保留
			continue
		}
		if name == target {
			if ok, err := cfg.SetTaskEnabled(idx, name, true); err != nil || !ok {
				return fail(fmt.Errorf("启用 %q 失败: %v", name, err), exitPreflight)
			}
			kept = append(kept, name)
			continue
		}
		if ok, err := cfg.SetTaskEnabled(idx, name, false); err != nil || !ok {
			return fail(fmt.Errorf("停用 %q 失败: %v", name, err), exitPreflight)
		}
		disabled = append(disabled, name)
	}
	if err := cfg.Save(); err != nil {
		return fail(err, exitFailed)
	}
	fmt.Printf("配置已改：只跑 %s（其余 %d 个任务已临时关闭，原文件备份为 .bak）\n", target, len(disabled))
	return printJSON(map[string]any{
		"target":   target,
		"kept":     kept,
		"disabled": disabled,
		"config":   cfgPath,
	})
}

// mustProject 读工程定义；失败时返回空壳，不影响调用方判断。
func mustProject(root string) *project.Interface {
	pi, err := project.Load(root)
	if err != nil {
		return &project.Interface{}
	}
	return pi
}

// firstPositional 取第一个不以 - 开头的参数。
func firstPositional(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// cmdRawKey 用系统级按键事件按一下（不经过 MaaFramework）。
func cmdRawKey(args []string) int {
	k := firstPositional(args)
	if k == "" {
		return fail(fmt.Errorf("用法: bd2cli rawkey <F10 或 0x79>"), exitUsage)
	}
	vk, err := parseVK(k)
	if err != nil {
		return fail(err, exitUsage)
	}
	rawKey(uint32(vk))
	return printJSON(map[string]any{"key": k, "vk": fmt.Sprintf("%#x", vk), "sent": true})
}

// cmdRawClick 用系统级鼠标事件真实点击一次（会移动鼠标）。
func cmdRawClick(args []string) int {
	var pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		pos = append(pos, a)
	}
	if len(pos) < 2 {
		return fail(fmt.Errorf("用法: bd2cli rawclick <x> <y>"), exitUsage)
	}
	var x, y int
	fmt.Sscanf(pos[0], "%d", &x)
	fmt.Sscanf(pos[1], "%d", &y)
	ok, errno := rawClick(x, y)
	if code := printJSON(map[string]any{
		"point": fmt.Sprintf("%d,%d", x, y),
		"ok":    ok,
		"errno": int(errno),
	}); code != exitOK {
		return code
	}
	if !ok {
		return exitFailed
	}
	return exitOK
}

// cmdUnlock 清掉"正在运行"的残留锁（进程被强杀等情况）。
func cmdUnlock(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	store := runs.New(filepath.Join(root, "logs", "runs"))
	rec, busy := store.Active()
	if busy {
		rec.Status = runs.StatusAborted
		rec.Reason = "手动 unlock 清理"
		_ = store.Save(rec)
	}
	if err := os.Remove(filepath.Join(root, "logs", "runs", ".active.json")); err != nil && !os.IsNotExist(err) {
		return fail(err, exitFailed)
	}
	if !busy {
		fmt.Println("没有残留锁")
		return exitOK
	}
	fmt.Printf("已解锁: %s\n", rec.ID)
	return exitOK
}

// cmdNotify 查看或测试通知通道。
func cmdNotify(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	n, desc := notify.Load(root)
	sub := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			sub = a
			break
		}
	}
	switch sub {
	case "", "status":
		return printJSON(map[string]any{
			"provider": desc,
			"config":   filepath.Join(root, "config", "bd2cli.json"),
		})
	case "test":
		if err := n.Notify("bd2cli 测试", "这是一条测试通知；收到说明通道正常。"); err != nil {
			return fail(err, exitFailed)
		}
		fmt.Printf("已通过 %s 发送测试通知\n", desc)
		if desc == "已关闭" || strings.HasPrefix(desc, "未配置") {
			fmt.Println("提示：先编辑 config/bd2cli.json 填 provider 与凭据")
		}
		return exitOK
	}
	return fail(fmt.Errorf("用法: bd2cli notify status|test"), exitUsage)
}

func cmdLogs(args []string) int {	if len(args) == 0 || args[0] != "tail" {
		return fail(fmt.Errorf("用法: bd2cli logs tail [--lines N]"), exitUsage)
	}
	n := 50
	if v := valueOf(args, "--lines"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	t, err := logx.Read(filepath.Join(root, "debug", "maafw.log"), n, 512<<10)
	if err != nil {
		return fail(err, exitPreflight)
	}
	for _, ln := range t.LastLines {
		fmt.Println(ln)
	}
	return exitOK
}
