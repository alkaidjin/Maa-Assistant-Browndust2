package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMergesImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "interface.json"), `{
	  "interface_version": 2,
	  "name": "MaaBrownDust2",
	  "version": "v26.11.1",
	  "controller": [
	    {"name": "Win32", "type": "Win32", "win32": {"class_regex": "UnityWndClass", "window_regex": "BrownDust II"}}
	  ],
	  "task": [{"name": "Inline", "entry": "StartGame"}],
	  "import": ["tasks/DailyAll.json"]
	}`)
	writeFile(t, filepath.Join(dir, "tasks", "DailyAll.json"), `{
	  "task": [{"name": "DailyAll", "label": "一条龙日常", "entry": "StartGame", "option": ["开启自动战斗"]}]
	}`)

	pi, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(pi.Task); got != 2 {
		t.Fatalf("tasks = %d, want 2", got)
	}
	task, ok := pi.TaskByName("一条龙日常")
	if !ok || task.Name != "DailyAll" {
		t.Fatalf("TaskByName(label) = %+v, %v", task, ok)
	}
	class, title, ok := pi.Win32Target()
	if !ok || class != "UnityWndClass" || title != "BrownDust II" {
		t.Fatalf("Win32Target = %q,%q,%v", class, title, ok)
	}
}

func TestLoadReportsBadImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "interface.json"), `{"import": ["tasks/missing.json"]}`)
	if _, err := Load(dir); err == nil {
		t.Fatal("want error for missing import")
	}
}

func TestFindRootWalksUp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "interface.json"), `{"name": "x"}`)
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := FindRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("root = %q, want %q", root, dir)
	}
}
