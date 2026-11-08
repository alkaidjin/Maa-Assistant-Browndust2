// Package status 把工程、MXU 配置、运行记录、日志聚合成一份给 AI 读的状态。
package status

import (
	"path/filepath"

	"bd2cli/internal/logx"
	"bd2cli/internal/mxucfg"
	"bd2cli/internal/project"
	"bd2cli/internal/runs"
)

// Checks 是可注入的外部环境探测，测试时全部替换为假函数。
type Checks struct {
	MXURunning func() bool
	PortOpen   func(port int) bool
	GameWindow func(class, title string) bool
	Process    func(name string) bool
}

// ProjectView 是工程概要。
type ProjectView struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Root    string `json:"root"`
}

// WebView 是 MXU 内置 web server 的状态。
type WebView struct {
	Enabled   bool `json:"enabled"`
	Port      int  `json:"port"`
	Reachable bool `json:"reachable"`
}

// MXUView 是 MXU 进程状态。
type MXUView struct {
	Running bool    `json:"running"`
	Web     WebView `json:"web"`
}

// GameView 是游戏客户端探测结果。
type GameView struct {
	WindowFound  bool   `json:"window_found"`
	Class        string `json:"class,omitempty"`
	Title        string `json:"title,omitempty"`
	ProcessFound bool   `json:"process_found"`
}

// TaskView 是实例里的一条任务。
type TaskView struct {
	Name    string `json:"name"`
	Label   string `json:"label,omitempty"`
	Enabled bool   `json:"enabled"`
}

// InstanceView 是当前实例。
type InstanceView struct {
	Name       string     `json:"name"`
	Controller string     `json:"controller"`
	Resource   string     `json:"resource"`
	Tasks      []TaskView `json:"tasks"`
}

// Report 是 `bd2cli --status --json` 的输出结构。
type Report struct {
	CLIVersion string        `json:"cli_version"`
	Project    ProjectView   `json:"project"`
	MXU        MXUView       `json:"mxu"`
	Game       GameView      `json:"game"`
	Instance   *InstanceView `json:"instance"`
	ActiveRun  *runs.Record  `json:"active_run"`
	RecentRuns []*runs.Record `json:"recent_runs"`
	Log        *logx.Tail    `json:"log"`
}

// Build 汇总状态。root 必须含 interface.json。
func Build(root, cliVersion string, ch Checks) (*Report, error) {
	pi, err := project.Load(root)
	if err != nil {
		return nil, err
	}
	rep := &Report{
		CLIVersion: cliVersion,
		Project:    ProjectView{Name: pi.Name, Version: pi.Version, Root: pi.Root},
	}

	class, title, ok := pi.Win32Target()
	if ok {
		rep.Game.Class, rep.Game.Title = class, title
		if ch.GameWindow != nil {
			rep.Game.WindowFound = ch.GameWindow(class, title)
		}
	}
	if ch.Process != nil {
		rep.Game.ProcessFound = ch.Process("BrownDust II.exe")
	}
	if ch.MXURunning != nil {
		rep.MXU.Running = ch.MXURunning()
	}

	if cfgPath, err := mxucfg.Find(root); err == nil {
		if cfg, err := mxucfg.Load(cfgPath); err == nil {
			rep.MXU.Web.Enabled = cfg.Settings.WebServerEnabled
			rep.MXU.Web.Port = cfg.Settings.WebServerPort
			if rep.MXU.Web.Port > 0 && ch.PortOpen != nil {
				rep.MXU.Web.Reachable = ch.PortOpen(rep.MXU.Web.Port)
			}
			if inst := cfg.Active(); inst != nil {
				iv := &InstanceView{
					Name:       inst.Name,
					Controller: inst.ControllerName,
					Resource:   inst.ResourceName,
				}
				for _, t := range inst.Tasks {
					tv := TaskView{Name: t.TaskName, Enabled: t.Enabled}
					if pt, ok := pi.TaskByName(t.TaskName); ok {
						tv.Label = pt.Label
					}
					iv.Tasks = append(iv.Tasks, tv)
				}
				rep.Instance = iv
			}
		}
	}

	store := runs.New(filepath.Join(root, "logs", "runs"))
	if active, busy := store.Active(); busy {
		rep.ActiveRun = active
	}
	if recent, err := store.Recent(5); err == nil {
		rep.RecentRuns = recent
	}

	if tail, err := logx.Read(filepath.Join(root, "debug", "maafw.log"), 20, 256<<10); err == nil {
		rep.Log = tail
	}
	return rep, nil
}

// GameReady 判断窗口是否可操作。
func (r *Report) GameReady() bool { return r.Game.WindowFound }
