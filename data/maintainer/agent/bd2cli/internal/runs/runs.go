// Package runs 管理结构化运行记录（logs/runs/<id>.json）与单飞锁。
package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 运行状态取值。
const (
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusAborted = "aborted"
	StatusTimeout = "timeout"
	StatusUnknown = "unknown"
)

// ErrBusy 表示已有任务在运行。
var ErrBusy = errors.New("another run is active")

// staleAfter 之后认为残留锁已失效。
const staleAfter = 6 * time.Hour

// TaskDone 是任务列表里单个任务的结果。
type TaskDone struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

// Record 是一次运行的完整记录，AI 主要消费这份 JSON。
type Record struct {
	ID         string     `json:"id"`
	Task       string     `json:"task,omitempty"`
	Instance   string     `json:"instance,omitempty"`
	Runner     string     `json:"runner,omitempty"`
	PID        int        `json:"pid,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	DurationS  float64    `json:"duration_s"`
	Status     string     `json:"status"`
	Reason     string     `json:"reason,omitempty"`
	Error      string     `json:"error,omitempty"`
	ExitCode   int        `json:"exit_code"`
	LogPath    string     `json:"log_path,omitempty"`
	Screenshot string     `json:"screenshot,omitempty"`
	Nodes      []string   `json:"nodes,omitempty"`
	Tasks      []TaskDone `json:"tasks,omitempty"`
	Diagnose   string     `json:"diagnose,omitempty"`
	CLIVersion string     `json:"cli_version"`
}

// Store 是 logs/runs 目录的封装。
type Store struct {
	Dir string
}

// New 返回以 dir 为根（通常是 <root>/logs/runs）的 Store。
func New(dir string) *Store { return &Store{Dir: dir} }

func (s *Store) file(id string) string { return filepath.Join(s.Dir, id+".json") }

func (s *Store) activeFile() string { return filepath.Join(s.Dir, ".active.json") }

// NewRecord 生成一个按时间排序、同一秒内递增的运行编号。
func (s *Store) NewRecord() *Record {
	now := time.Now()
	base := now.Format("20060102-150405")
	id := base + "-1"
	for i := 2; ; i++ {
		if _, err := os.Stat(s.file(id)); os.IsNotExist(err) {
			break
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	return &Record{ID: id, StartedAt: now, Status: StatusRunning}
}

// Save 落盘一条记录。
func (s *Store) Save(r *Record) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.file(r.ID) + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.file(r.ID))
}

// Get 读取指定编号的记录。
func (s *Store) Get(id string) (*Record, error) {
	b, err := os.ReadFile(s.file(id))
	if err != nil {
		return nil, err
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", id, err)
	}
	return &r, nil
}

// Recent 返回最近 n 条记录（按编号倒序）。
func (s *Store) Recent(n int) ([]*Record, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	out := make([]*Record, 0, n)
	for _, id := range ids {
		r, err := s.Get(id)
		if err != nil {
			continue
		}
		out = append(out, r)
		if len(out) >= n {
			break
		}
	}
	return out, nil
}

// Active 返回当前正在运行的记录（锁文件内容）。
func (s *Store) Active() (*Record, bool) {
	b, err := os.ReadFile(s.activeFile())
	if err != nil {
		return nil, false
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, false
	}
	if time.Since(r.StartedAt) > staleAfter {
		return nil, false
	}
	return &r, true
}

// Acquire 抢单飞锁并登记一条 running 记录；返回 release。
func (s *Store) Acquire(task, instance string) (*Record, func(), error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, nil, err
	}
	if _, busy := s.Active(); busy {
		return nil, nil, ErrBusy
	}
	r := s.NewRecord()
	r.Task = task
	r.Instance = instance
	if err := s.Save(r); err != nil {
		return nil, nil, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(s.activeFile(), b, 0o644); err != nil {
		return nil, nil, err
	}
	release := func() { _ = os.Remove(s.activeFile()) }
	return r, release, nil
}

// Finalize 结束一条记录并清锁。
func (s *Store) Finalize(r *Record, status string, code int, runErr error) error {
	now := time.Now()
	r.FinishedAt = &now
	r.DurationS = now.Sub(r.StartedAt).Seconds()
	r.Status = status
	r.ExitCode = code
	if runErr != nil {
		r.Error = runErr.Error()
	}
	return s.Save(r)
}
