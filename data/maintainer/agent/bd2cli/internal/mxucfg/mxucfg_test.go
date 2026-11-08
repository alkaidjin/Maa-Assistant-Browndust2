package mxucfg

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `{
  "version": "1.0",
  "instances": [
    {"id": "i1", "name": "配置 1", "controllerName": "Win32", "resourceName": "官服",
     "savedDevice": {"windowName": "BrownDust II"},
     "tasks": [{"id": "t1", "taskName": "DailyAll", "enabled": true}]}
  ],
  "settings": {"webServerEnabled": true, "webServerPort": 12701, "theme": "light"},
  "lastActiveInstanceId": "i1"
}`

func loadSample(t *testing.T) (*Config, string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config", "mxu-MaaBrownDust2.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return c, p
}

func TestLoadAndActive(t *testing.T) {
	c, _ := loadSample(t)
	if !c.Settings.WebServerEnabled || c.Settings.WebServerPort != 12701 {
		t.Fatalf("settings = %+v", c.Settings)
	}
	inst := c.Active()
	if inst == nil || inst.Name != "配置 1" || len(inst.Tasks) != 1 || !inst.Tasks[0].Enabled {
		t.Fatalf("instance = %+v", inst)
	}
}

func TestFindPicksMxuFile(t *testing.T) {
	c, p := loadSample(t)
	if c.Path() != p {
		t.Fatalf("path = %q", c.Path())
	}
	root := filepath.Dir(filepath.Dir(p))
	got, err := Find(root)
	if err != nil || got != p {
		t.Fatalf("Find = %q, %v", got, err)
	}
}

func TestSetKeepsTypeAndBacksUp(t *testing.T) {
	c, p := loadSample(t)
	if err := c.Set([]string{"settings", "webServerPort"}, "12702"); err != nil {
		t.Fatal(err)
	}
	v, ok := c.Get([]string{"settings", "webServerPort"})
	if !ok {
		t.Fatal("missing key")
	}
	if f, isFloat := v.(float64); !isFloat || f != 12702 {
		t.Fatalf("port = %#v, want float64(12702)", v)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p + ".bak"); err != nil {
		t.Fatalf("want .bak backup: %v", err)
	}
	reloaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Settings.WebServerPort != 12702 {
		t.Fatalf("reloaded port = %d", reloaded.Settings.WebServerPort)
	}
}

func TestGetMissing(t *testing.T) {
	c, _ := loadSample(t)
	if _, ok := c.Get([]string{"settings", "nope"}); ok {
		t.Fatal("want miss")
	}
}
