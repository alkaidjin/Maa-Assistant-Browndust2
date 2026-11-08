// Package project 解析 PI v2 的 interface.json 与其 import 的任务文件。
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound 表示向上查找时没有找到 interface.json。
var ErrNotFound = errors.New("interface.json not found")

// Win32 是 controller 的 win32 段：窗口探测与输入方式。
type Win32 struct {
	ClassRegex  string `json:"class_regex"`
	WindowRegex string `json:"window_regex"`
	Screencap   string `json:"screencap"`
	Mouse       string `json:"mouse"`
	Keyboard    string `json:"keyboard"`
}

// Controller 是 interface.json 的一个控制器。
type Controller struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Win32 *Win32 `json:"win32"`
}

// Resource 是资源包。
type Resource struct {
	Name string   `json:"name"`
	Path []string `json:"path"`
}

// Task 是一个任务定义（interface.json 的 task[] 项）。
type Task struct {
	Name             string         `json:"name"`
	Label            string         `json:"label"`
	Entry            string         `json:"entry"`
	Description      string         `json:"description"`
	Controller       []string       `json:"controller"`
	Option           []string       `json:"option"`
	PipelineOverride map[string]any `json:"pipeline_override"`
}

// Interface 是合并 import 之后的完整工程定义。
type Interface struct {
	InterfaceVersion int          `json:"interface_version"`
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	Version          string       `json:"version"`
	Controller       []Controller `json:"controller"`
	Resource         []Resource   `json:"resource"`
	Task             []Task       `json:"task"`
	Option           []Option     `json:"option"`
	Import           []string     `json:"import"`
	MirrorChyanRID   string       `json:"mirrorchyan_rid"`

	// Root 是 interface.json 所在目录（绝对路径）。
	Root string `json:"-"`
}

// FindRoot 从 start 开始向上查找含 interface.json 的目录。
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		info, err := os.Stat(filepath.Join(dir, "interface.json"))
		if err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

// Load 读取 root/interface.json，并展开 import 里的任务文件。
func Load(root string) (*Interface, error) {
	raw, err := os.ReadFile(filepath.Join(root, "interface.json"))
	if err != nil {
		return nil, fmt.Errorf("read interface.json: %w", err)
	}
	var pi Interface
	if err := json.Unmarshal(raw, &pi); err != nil {
		return nil, fmt.Errorf("parse interface.json: %w", err)
	}
	pi.Root = root

	seen := make(map[string]bool, len(pi.Task))
	tasks := make([]Task, 0, len(pi.Task))
	for _, t := range pi.Task {
		if t.Name == "" || seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		tasks = append(tasks, t)
	}
	seenOpt := make(map[string]bool, len(pi.Option))
	opts := make([]Option, 0, len(pi.Option))
	for _, o := range pi.Option {
		if o.Name == "" || seenOpt[o.Name] {
			continue
		}
		seenOpt[o.Name] = true
		opts = append(opts, o)
	}

	for _, imp := range pi.Import {
		p := filepath.Join(root, filepath.FromSlash(imp))
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("import %s: %w", imp, err)
		}
		var frag struct {
			Task   []Task          `json:"task"`
			Option json.RawMessage `json:"option"`
		}
		if err := json.Unmarshal(b, &frag); err != nil {
			return nil, fmt.Errorf("parse %s: %w", imp, err)
		}
		fragOpts, err := parseOptions(frag.Option)
		if err != nil {
			return nil, fmt.Errorf("parse %s option: %w", imp, err)
		}
		for _, t := range frag.Task {
			if t.Name == "" || seen[t.Name] {
				continue
			}
			seen[t.Name] = true
			tasks = append(tasks, t)
		}
		for _, o := range fragOpts {
			if o.Name == "" || seenOpt[o.Name] {
				continue
			}
			seenOpt[o.Name] = true
			opts = append(opts, o)
		}
	}
	pi.Task = tasks
	pi.Option = opts
	return &pi, nil
}

// TaskByName 按 name 或 label 查任务。
func (p *Interface) TaskByName(name string) (*Task, bool) {
	for i := range p.Task {
		if p.Task[i].Name == name || p.Task[i].Label == name {
			return &p.Task[i], true
		}
	}
	return nil, false
}

// TaskByEntry 按流水线入口节点名查任务（MXU 日志里只有 entry）。
func (p *Interface) TaskByEntry(entry string) (*Task, bool) {
	for i := range p.Task {
		if p.Task[i].Entry == entry {
			return &p.Task[i], true
		}
	}
	return nil, false
}

// Win32Target 返回用于窗口探测的 class/title。优先取名为 Win32 的控制器。
func (p *Interface) Win32Target() (class, title string, ok bool) {
	pick := func(c Controller) bool {
		return c.Type == "Win32" && c.Win32 != nil && c.Win32.WindowRegex != "" && c.Win32.ClassRegex != ""
	}
	for _, c := range p.Controller {
		if c.Name == "Win32" && pick(c) {
			return c.Win32.ClassRegex, c.Win32.WindowRegex, true
		}
	}
	for _, c := range p.Controller {
		if pick(c) {
			return c.Win32.ClassRegex, c.Win32.WindowRegex, true
		}
	}
	return "", "", false
}
