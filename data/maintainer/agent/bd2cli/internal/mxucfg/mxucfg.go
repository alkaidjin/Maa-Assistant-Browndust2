// Package mxucfg 读写 MXU 的用户配置（config/mxu-<project>.json）。
package mxucfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// OptionValue 是任务选项的当前取值：select 用 CaseName，checkbox 用 CaseNames。
type OptionValue struct {
	Type      string   `json:"type"`
	CaseName  string   `json:"caseName,omitempty"`
	CaseNames []string `json:"caseNames,omitempty"`
}

// Task 是实例里的一条任务。
type Task struct {
	ID                  string                 `json:"id"`
	TaskName            string                 `json:"taskName"`
	Enabled             bool                   `json:"enabled"`
	EnabledByController map[string]bool        `json:"enabledByController"`
	OptionValues        map[string]OptionValue `json:"optionValues"`
}

// SavedDevice 记录上次连接的游戏窗口与程序路径。
type SavedDevice struct {
	WindowName           string `json:"windowName"`
	ConnectedProgramPath string `json:"connectedProgramPath"`
}

// Instance 是一个 MXU 实例（一个标签页）。
type Instance struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	ControllerName string      `json:"controllerName"`
	ResourceName   string      `json:"resourceName"`
	SavedDevice    SavedDevice `json:"savedDevice"`
	Tasks          []Task      `json:"tasks"`
}

// Settings 只解析我们关心的键。
type Settings struct {
	WebServerEnabled bool `json:"webServerEnabled"`
	WebServerPort    int  `json:"webServerPort"`
	AutoRunOnLaunch  bool `json:"autoRunOnLaunch"`
}

// Config 是整份 MXU 配置；raw 保留全部字段以便只改单键。
type Config struct {
	Version              string         `json:"version"`
	Instances            []Instance     `json:"instances"`
	Settings             Settings       `json:"settings"`
	LastActiveInstanceID string         `json:"lastActiveInstanceId"`
	raw                  map[string]any
	path                 string
}

// Find 返回 root/config 下的 mxu-*.json。
func Find(root string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "config", "mxu-*.json"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no config/mxu-*.json under %s", root)
	}
	sort.Strings(matches)
	return matches[0], nil
}

// Load 读取并解析配置。
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := json.Unmarshal(b, &c.raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	c.path = path
	return &c, nil
}

// Path 返回配置文件路径。
func (c *Config) Path() string { return c.path }

// Active 返回 lastActiveInstanceId 对应的实例，找不到时退回第一个。
func (c *Config) Active() *Instance {
	for i := range c.Instances {
		if c.Instances[i].ID == c.LastActiveInstanceID {
			return &c.Instances[i]
		}
	}
	if len(c.Instances) > 0 {
		return &c.Instances[0]
	}
	return nil
}

// InstanceByName 按实例名（显示名或 ID）查找。
func (c *Config) InstanceByName(name string) *Instance {
	for i := range c.Instances {
		if c.Instances[i].Name == name || c.Instances[i].ID == name {
			return &c.Instances[i]
		}
	}
	return nil
}

// EnsureAutoRun 确保 MXU 的"启动后自动执行"指向 instID，并打开总开关。
//
// 实测：`--autostart` 只有同时满足
//   settings.autoRunOnLaunch = true（布尔）
//   settings.autoStartInstanceId = <实例 ID>
// 才会真的开始跑任务；缺一个就只启动界面。
func (c *Config) EnsureAutoRun(instID string) error {
	settings, ok := c.raw["settings"].(map[string]any)
	if !ok {
		settings = map[string]any{}
		c.raw["settings"] = settings
	}
	changed := false
	if cur, ok := settings["autoRunOnLaunch"].(bool); !ok || !cur {
		settings["autoRunOnLaunch"] = true
		changed = true
	}
	if cur, ok := settings["autoStartInstanceId"].(string); !ok || cur != instID {
		settings["autoStartInstanceId"] = instID
		changed = true
	}
	if !changed {
		return nil
	}
	return c.Save()
}

// IndexOf 返回实例在配置里的下标（找不到返回 -1）。
func (c *Config) IndexOf(inst *Instance) int {
	for i := range c.Instances {
		if &c.Instances[i] == inst || c.Instances[i].ID == inst.ID {
			return i
		}
	}
	return -1
}

// SetTaskEnabled 改实例里某任务的 enabled（同时改 enabledByController 的每一项），
// 直接操作 raw，保证其余字段原样保留。返回是否找到该任务。
func (c *Config) SetTaskEnabled(instIdx int, taskName string, enabled bool) (bool, error) {
	insts, ok := c.raw["instances"].([]any)
	if !ok || instIdx < 0 || instIdx >= len(insts) {
		return false, fmt.Errorf("instances[%d] 不存在", instIdx)
	}
	im, ok := insts[instIdx].(map[string]any)
	if !ok {
		return false, fmt.Errorf("instances[%d] 不是对象", instIdx)
	}
	tasks, ok := im["tasks"].([]any)
	if !ok {
		return false, fmt.Errorf("instances[%d].tasks 不是数组", instIdx)
	}
	for _, item := range tasks {
		tm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tm["taskName"].(string)
		if name != taskName {
			continue
		}
		tm["enabled"] = enabled
		if ebc, ok := tm["enabledByController"].(map[string]any); ok {
			for k := range ebc {
				ebc[k] = enabled
			}
		}
		for i := range c.Instances[instIdx].Tasks {
			if c.Instances[instIdx].Tasks[i].TaskName == taskName {
				c.Instances[instIdx].Tasks[i].Enabled = enabled
			}
		}
		return true, nil
	}
	return false, fmt.Errorf("实例里没有任务 %q", taskName)
}

// TaskNames 返回实例里所有任务名（按配置顺序）。
func (c *Config) TaskNames(instIdx int) []string {
	if instIdx < 0 || instIdx >= len(c.Instances) {
		return nil
	}
	out := make([]string, 0, len(c.Instances[instIdx].Tasks))
	for _, t := range c.Instances[instIdx].Tasks {
		out = append(out, t.TaskName)
	}
	return out
}

// Get 按点路径读 raw（例如 "settings.webServerPort"）。
func (c *Config) Get(path []string) (any, bool) {
	var cur any = c.raw
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// Set 按点路径改 raw 里的单个键；只有在键已存在时保留原类型写入。
func (c *Config) Set(path []string, value any) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}
	cur := c.raw
	for _, key := range path[:len(path)-1] {
		next, ok := cur[key]
		if !ok {
			m := map[string]any{}
			cur[key] = m
			cur = m
			continue
		}
		m, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("%s is not an object", key)
		}
		cur = m
	}
	leaf := path[len(path)-1]
	if old, ok := cur[leaf]; ok {
		if coerced, ok := coerce(old, value); ok {
			value = coerced
		}
	}
	cur[leaf] = value
	return nil
}

// coerce 让写入值跟随已有值的类型，避免把端口写成字符串。
func coerce(old, value any) (any, bool) {
	if b, ok := value.(string); ok {
		switch old.(type) {
		case float64:
			var f float64
			if _, err := fmt.Sscanf(b, "%g", &f); err == nil {
				return f, true
			}
		case bool:
			switch b {
			case "true", "1", "yes":
				return true, true
			case "false", "0", "no":
				return false, true
			}
		}
	}
	return nil, false
}

// Save 写回配置，并先把原文件备份为 .bak。
func (c *Config) Save() error {
	b, err := json.MarshalIndent(c.raw, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if old, err := os.ReadFile(c.path); err == nil {
		if err := os.WriteFile(c.path+".bak", old, 0o644); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
