package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"bd2cli/internal/logx"
)

// taskPlan 是这一轮要跑的任务（来自实例已启用列表）。
type taskPlan struct {
	TaskName string `json:"task_name"` // MXU 实例里的任务名
	Label    string `json:"label"`
	Entry    string `json:"entry"`
}

// taskResult 是单个任务的结果。
type taskResult struct {
	TaskName string `json:"task_name"`
	Label    string `json:"label"`
	OK       bool   `json:"ok"`
}

// mxuOutcome 是一次 MXU 运行的最终判定。
type mxuOutcome struct {
	Code    int
	Status  string
	Reason  string
	Nodes   []string
	Tasks   []taskResult
	Elapsed float64
	Err     error
	// reachedEnd 记录本轮命中的完成标志节点（便于排查）
	reachedEnd []string
}

// runMXUAndWait 启动 MXU 并**等整批任务真正结束**，再按需要关窗口。
//
// 完成判据（按可靠性排序）：
//  1. MXU 自身日志出现 `kind: tasks-completed`（它跑完整批任务后的信号）
//  2. MXU 进程退出
//
// 任务成败：MaaFramework 只为**失败**的任务写 `task end ... [ret=false]`，
// 成功的任务不写，所以"成功数 = 计划数 − 失败数"。
func runMXUAndWait(root, exe, instanceID, entry string, plan []taskPlan, timeout time.Duration,
	closeMXU, closeGame bool, out io.Writer) mxuOutcome {

	res := mxuOutcome{Status: "unknown"}
	logPath := filepath.Join(root, "debug", "maafw.log")
	start := time.Now()

	cmd := exec.Command(exe, "--autostart", "-i", instanceID, "--quit-after-run")
	cmd.Dir = root
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		res.Code, res.Status, res.Err = exitPreflight, "failed", fmt.Errorf("启动 MXU 失败: %w", err)
		return res
	}
	go func() { _ = cmd.Wait() }()

	appeared := false
	for i := 0; i < 30; i++ {
		if processRunning("mxu.exe") {
			appeared = true
			break
		}
		time.Sleep(time.Second)
	}
	if !appeared {
		res.Code, res.Status, res.Err = exitPreflight, "failed", fmt.Errorf("MXU 没有启动")
		return res
	}

	deadline := start.Add(timeout)
	completed := false
	stopped := false
	missing := 0
	for {
		if _, err := os.Stat(stopFlagPath(root)); err == nil {
			stopped = true
			break
		}
		if mxuTasksCompleted(root, start) {
			completed = true
			break
		}
		if processRunning("mxu.exe") {
			missing = 0
		} else {
			// MXU 启动器会短暂消失后由真正的实例接管，连续 3 次(约 6 秒)不在才算退出
			missing++
			time.Sleep(2 * time.Second)
			if missing >= 3 {
				completed = true
				break
			}
			continue
		}
		if time.Now().After(deadline) {
			res.Reason = fmt.Sprintf("超过 %s 仍未结束", timeout)
			break
		}
		time.Sleep(3 * time.Second)
	}

	res.Elapsed = time.Since(start).Seconds()
	res.Nodes = lastNodes(logPath, 12)

	ends := findTaskEnds(logPath, start)
	failed := map[string]bool{}
	for _, e := range ends {
		if !e.OK {
			failed[e.Entry] = true
		}
	}

	// 真实提交了哪些任务，以 MXU 日志为准（计划表可能因为临时改配置而与实际不符）
	submitted := submittedEntries(root, start)
	byEntry := map[string]taskPlan{}
	for _, p := range plan {
		if p.Entry != "" {
			byEntry[p.Entry] = p
		}
	}

	if len(submitted) > 0 {
		ran := executedEntries(logPath, submitted)
		idx := loadPipelines(root)
		autoEnds := idx.TaskEnds(submitted)
		customEnds := loadTaskEnds(root)
		seen := map[string]bool{}
		for _, e := range submitted {
			if seen[e] {
				continue
			}
			seen[e] = true
			label, taskName := e, e
			if p, ok := byEntry[e]; ok {
				label, taskName = p.Label, p.TaskName
			}
			if !ran[e] {
				res.Tasks = append(res.Tasks, taskResult{TaskName: taskName, Label: label + "（未执行）", OK: false})
				continue
			}
			// 完成标志节点：优先用 config/bd2cli.json 的 task_ends，其次自动探测
			ends := customEnds[e]
			if len(ends) == 0 {
				ends = autoEnds[e]
			}
			finished := true
			if len(ends) > 0 {
				finished = false
				for _, end := range ends {
					if ranNode(logPath, end) {
						finished = true
						res.reachedEnd = append(res.reachedEnd, end)
						break
					}
				}
			}
			if !finished {
				res.Tasks = append(res.Tasks, taskResult{TaskName: taskName, Label: label + "（未跑完：未见完成节点）", OK: false})
				continue
			}
			res.Tasks = append(res.Tasks, taskResult{TaskName: taskName, Label: label, OK: !failed[e]})
		}
		for _, p := range plan {
			if p.Entry == "" || seen[p.Entry] {
				continue
			}
			res.Tasks = append(res.Tasks, taskResult{TaskName: p.TaskName, Label: p.Label + "（未执行）", OK: false})
		}
	} else if len(plan) == 0 {
		for _, e := range ends {
			res.Tasks = append(res.Tasks, taskResult{TaskName: e.Entry, Label: e.Entry, OK: e.OK})
		}
	} else {
		for _, p := range plan {
			res.Tasks = append(res.Tasks, taskResult{TaskName: p.TaskName, Label: p.Label, OK: !failed[p.Entry]})
		}
	}

	failedCount := 0
	for _, t := range res.Tasks {
		if !t.OK {
			failedCount++
		}
	}

	switch {
	case stopped:
		res.Code, res.Status = exitAborted, "aborted"
		res.Reason = "用户请求停止（bdcli control stop）"
	case !completed:
		res.Code, res.Status = exitFailed, "timeout"
		if res.Reason == "" {
			res.Reason = "没有等到任务完成信号"
		}
	case failedCount > 0:
		res.Code, res.Status = exitFailed, "failed"
		var names []string
		for _, t := range res.Tasks {
			if !t.OK {
				names = append(names, t.Label)
			}
		}
		res.Reason = joinReason(res.Reason, "失败任务："+strings.Join(names, "、"))
	default:
		res.Code, res.Status = 0, "success"
	}

	if closeMXU || closeGame {
		targets := make([]string, 0, 3)
		if closeMXU {
			targets = append(targets, "mxu.exe", "go-service.exe")
		}
		if closeGame {
			targets = append(targets, "BrownDust II.exe")
		}
		closed := false
		if taskInstalled() {
			if er, err := runElevated(root, "kill", targets, 45*time.Second); err == nil && er.Done && er.Error == "" {
				closed = true
			}
		}
		if !closed {
			allOK := true
			for _, t := range targets {
				if !killProcess(t) {
					allOK = false
				}
			}
			closed = allOK
		}
		if !closed {
			res.Reason = joinReason(res.Reason, "窗口没能关闭：权限不足（执行 bd2cli elevate install 授权一次即可）")
		}
	}
	return res
}

// joinReason 拼接多条原因。
func joinReason(a, b string) string {
	if a == "" {
		return b
	}
	return a + "；" + b
}

// loadNoRetry 读取"失败也不重试"的任务名（例如救赎之塔：快速战斗次数用完/层未解锁，重试无意义）。
func loadNoRetry(root string) map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(filepath.Join(root, "config", "bd2cli.json"))
	if err != nil {
		return out
	}
	var w struct {
		NoRetry []string `json:"no_retry_tasks"`
	}
	if json.Unmarshal(b, &w) != nil {
		return out
	}
	for _, n := range w.NoRetry {
		out[n] = true
	}
	return out
}

// loadTaskEnds 读取 config/bd2cli.json 的 task_ends（entry → 完成标志节点列表）。
// 用户改完 pipeline 节点后可以直接改这里，不必改代码。
func loadTaskEnds(root string) map[string][]string {
	b, err := os.ReadFile(filepath.Join(root, "config", "bd2cli.json"))
	if err != nil {
		return nil
	}
	var w struct {
		TaskEnds map[string][]string `json:"task_ends"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return nil
	}
	return w.TaskEnds
}

// ranNode 判断某个节点是否在本轮日志里执行过。
func ranNode(path, name string) bool {
	lines, err := logx.Lines(path, 8<<20)
	if err != nil {
		return false
	}
	needle := `"name":"` + name + `"`
	return strings.Contains(strings.Join(lines, "\n"), needle)
}

// executedEntries 判断每个任务的入口节点是否真的执行过（一次扫描日志）。
func executedEntries(path string, entries []string) map[string]bool {
	out := map[string]bool{}
	lines, err := logx.Lines(path, 8<<20)
	if err != nil {
		return out
	}
	joined := strings.Join(lines, "\n")
	for _, e := range entries {
		if e == "" {
			continue
		}
		if strings.Contains(joined, `"name":"`+e+`"`) {
			out[e] = true
		}
	}
	return out
}

// submittedEntries 从 MXU 自身日志里读本次真正提交的任务入口（最可靠的事实来源）。
func submittedEntries(root string, since time.Time) []string {
	files, _ := filepath.Glob(filepath.Join(root, "debug", "2026-*.log"))
	var out []string
	for _, f := range files {
		lines, err := logx.Lines(f, 1<<20)
		if err != nil {
			continue
		}
		for _, ln := range lines {
			if !strings.Contains(ln, "]: entry=") {
				continue
			}
			if ts, ok := mafwTime(ln); ok && ts.Before(since.Add(-2*time.Second)) {
				continue
			}
			i := strings.Index(ln, "]: entry=")
			rest := ln[i+len("]: entry="):]
			e := rest
			if j := strings.IndexAny(e, ", "); j > 0 {
				e = e[:j]
			}
			if e != "" {
				out = append(out, e)
			}
		}
	}
	return out
}

// mxuTasksCompleted 在 MXU 自身日志里找本次运行之后的 tasks-completed 信号。
func mxuTasksCompleted(root string, since time.Time) bool {
	files, _ := filepath.Glob(filepath.Join(root, "debug", "2026-*.log"))
	for _, f := range files {
		lines, err := logx.Lines(f, 1<<20)
		if err != nil {
			continue
		}
		for _, ln := range lines {
			if !strings.Contains(ln, "tasks-completed") {
				continue
			}
			if ts, ok := mafwTime(ln); ok && !ts.Before(since.Add(-2*time.Second)) {
				return true
			}
		}
	}
	return false
}

// taskEnd 是 MaaFramework 写的一条任务结束记录（只出现在失败任务上）。
type taskEnd struct {
	Entry string
	OK    bool
}

// findTaskEnds 收集 since 之后的任务结束记录（MaaFramework 只写失败任务）。
func findTaskEnds(path string, since time.Time) []taskEnd {
	lines, err := logx.Lines(path, 8<<20)
	if err != nil {
		return nil
	}
	var out []taskEnd
	for _, ln := range lines {
		if !strings.Contains(ln, "task end:") {
			continue
		}
		if ts, ok := lineTime(ln); ok && ts.Before(since.Add(-2*time.Second)) {
			continue
		}
		i := strings.Index(ln, `"entry":"`)
		if i < 0 {
			continue
		}
		rest := ln[i+len(`"entry":"`):]
		j := strings.Index(rest, `"`)
		if j <= 0 {
			continue
		}
		out = append(out, taskEnd{
			Entry: rest[:j],
			OK:    !strings.Contains(ln, "[ret=false]"),
		})
	}
	return out
}

// lastNodes 取日志尾部最近若干次成功执行的节点名。
func lastNodes(path string, n int) []string {
	lines, err := logx.Lines(path, 4<<20)
	if err != nil {
		return nil
	}
	var names []string
	for _, ln := range lines {
		if !strings.Contains(ln, "node done [result") {
			continue
		}
		i := strings.Index(ln, `"name":"`)
		if i < 0 {
			continue
		}
		rest := ln[i+len(`"name":"`):]
		if j := strings.Index(rest, `"`); j > 0 {
			names = append(names, rest[:j])
		}
	}
	if len(names) > n {
		names = names[len(names)-n:]
	}
	return names
}

// lineTime 解析 `[2006-01-02 15:04:05.000]`（MaaFramework 日志）。
func lineTime(line string) (time.Time, bool) {
	if !strings.HasPrefix(line, "[") {
		return time.Time{}, false
	}
	i := strings.Index(line, "]")
	if i < 0 {
		return time.Time{}, false
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04:05.000", line[1:i], time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

// mafwTime 兼容 MXU 自身日志（无方括号）的时间格式。
func mafwTime(line string) (time.Time, bool) {
	if t, ok := lineTime(line); ok {
		return t, ok
	}
	if len(line) >= 19 {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", line[:19], time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// killProcess 强制结束某个进程；返回是否成功（本来就不在也算成功）。
func killProcess(name string) bool {
	if !processRunning(name) {
		return true
	}
	if err := exec.Command("taskkill", "/IM", name, "/F").Run(); err == nil {
		return true
	}
	return !processRunning(name)
}
