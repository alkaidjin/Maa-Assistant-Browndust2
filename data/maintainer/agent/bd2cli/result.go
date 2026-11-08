package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bd2cli/internal/runs"
)

// latestResult 是"最近一轮结果"的机器可读快照，agent 不必再问用户。
type latestResult struct {
	ID         string           `json:"id"`
	Task       string           `json:"task"`
	Status     string           `json:"status"`
	ExitCode   int              `json:"exit_code"`
	DurationS  float64          `json:"duration_s"`
	Reason     string           `json:"reason,omitempty"`
	Card       string           `json:"card"`
	Diagnose   string           `json:"diagnose,omitempty"`
	FinishedAt string           `json:"finished_at,omitempty"`
	Tasks      []runs.TaskDone  `json:"tasks,omitempty"`
	Summary    string           `json:"summary"`
}

func latestPath(root string) string { return filepath.Join(root, "logs", "latest-result.json") }

// writeLatest 在每轮结束后落一份快照，供 agent 直接读取。
func writeLatest(root, label string, rec *runs.Record) {
	card := filepath.Join(root, "logs", "report-"+rec.ID+".html")
	lr := latestResult{
		ID:        rec.ID,
		Task:      label,
		Status:    rec.Status,
		ExitCode:  rec.ExitCode,
		DurationS: rec.DurationS,
		Reason:    rec.Reason,
		Card:      card,
		Diagnose:  rec.Diagnose,
		Tasks:     rec.Tasks,
	}
	if rec.FinishedAt != nil {
		lr.FinishedAt = rec.FinishedAt.Format(time.RFC3339)
	}
	ok, bad := 0, 0
	for _, t := range rec.Tasks {
		if t.OK {
			ok++
		} else {
			bad++
		}
	}
	lr.Summary = fmt.Sprintf("%s · %s · 用时 %s · 成功 %d/%d",
		label, statusLabel(rec.Status), humanDuration(rec.DurationS), ok, len(rec.Tasks))
	if bad > 0 {
		lr.Summary += fmt.Sprintf("（失败 %d）", bad)
	}
	b, err := json.MarshalIndent(lr, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Join(root, "logs"), 0o755)
	_ = os.WriteFile(latestPath(root), b, 0o644)
}

// cmdResult 直接给出最近一轮结果（含卡片路径）。agent 跑完即可调用，不必问用户。
func cmdResult(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	b, err := os.ReadFile(latestPath(root))
	if err == nil {
		var lr latestResult
		if json.Unmarshal(b, &lr) == nil {
			if _, err := os.Stat(lr.Card); err != nil {
				if p, e2 := emitReport(root, mustRecord(root, lr.ID), nil); e2 == nil {
					lr.Card = p
				}
			}
			return printJSON(lr)
		}
	}
	// 没有快照就退回最新一条记录
	store := runs.New(filepath.Join(root, "logs", "runs"))
	recs, _ := store.Recent(1)
	if len(recs) == 0 {
		return fail(fmt.Errorf("还没有运行记录"), exitFailed)
	}
	rec := recs[0]
	label := taskLabel(root, rec.Task)
	card, _ := emitReport(root, rec, nil)
	writeLatest(root, label, rec)
	b, _ = os.ReadFile(latestPath(root))
	var lr latestResult
	_ = json.Unmarshal(b, &lr)
	if card != "" {
		lr.Card = card
	}
	return printJSON(lr)
}

func mustRecord(root, id string) *runs.Record {
	store := runs.New(filepath.Join(root, "logs", "runs"))
	if r, err := store.Get(id); err == nil {
		return r
	}
	return &runs.Record{ID: id}
}

// cmdWait 阻塞到当前这一轮结束，然后直接输出结果与卡片路径。
//
// 用法：agent 在后台跑 `bd2cli wait --timeout 3h`，进程退出时就能立刻拿到结果。
func cmdWait(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	timeout := 3 * time.Hour
	if v := valueOf(args, "--timeout"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}
	store := runs.New(filepath.Join(root, "logs", "runs"))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, busy := store.Active(); !busy {
			return cmdResult(args)
		}
		time.Sleep(3 * time.Second)
	}
	return fail(fmt.Errorf("等待超时（%s）", timeout), exitFailed)
}
