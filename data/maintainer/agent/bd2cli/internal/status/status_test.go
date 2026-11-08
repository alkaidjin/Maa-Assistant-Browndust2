package status

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fakeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "interface.json", `{
	  "interface_version": 2, "name": "MaaBrownDust2", "version": "v26.11.1",
	  "controller": [{"name":"Win32","type":"Win32","win32":{"class_regex":"UnityWndClass","window_regex":"BrownDust II"}}],
	  "import": ["tasks/DailyAll.json"]
	}`)
	write(t, root, "tasks/DailyAll.json", `{"task":[{"name":"DailyAll","label":"一条龙日常","entry":"StartGame"}]}`)
	write(t, root, "config/mxu-MaaBrownDust2.json", `{
	  "instances":[{"id":"i1","name":"配置 1","controllerName":"Win32","resourceName":"官服",
	    "tasks":[{"id":"t1","taskName":"DailyAll","enabled":true},{"id":"t2","taskName":"DailyAll","enabled":false}]}],
	  "settings":{"webServerEnabled":true,"webServerPort":12701},
	  "lastActiveInstanceId":"i1"
	}`)
	write(t, root, "debug/maafw.log", "line1\n[ERR] node timeout at StartGame\nline3\n")
	return root
}

func TestBuildAggregates(t *testing.T) {
	root := fakeRoot(t)
	rep, err := Build(root, "0.1.0", Checks{
		MXURunning: func() bool { return true },
		PortOpen:   func(p int) bool { return p == 12701 },
		GameWindow: func(class, title string) bool {
			return class == "UnityWndClass" && title == "BrownDust II"
		},
		Process: func(name string) bool { return name == "BrownDust II.exe" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Project.Name != "MaaBrownDust2" || rep.Project.Version != "v26.11.1" {
		t.Fatalf("project = %+v", rep.Project)
	}
	if !rep.MXU.Running || !rep.MXU.Web.Reachable || rep.MXU.Web.Port != 12701 {
		t.Fatalf("mxu = %+v", rep.MXU)
	}
	if !rep.Game.WindowFound || !rep.Game.ProcessFound || !rep.GameReady() {
		t.Fatalf("game = %+v", rep.Game)
	}
	if rep.Instance == nil || rep.Instance.Name != "配置 1" || len(rep.Instance.Tasks) != 2 {
		t.Fatalf("instance = %+v", rep.Instance)
	}
	if rep.Instance.Tasks[0].Label != "一条龙日常" {
		t.Fatalf("label = %q", rep.Instance.Tasks[0].Label)
	}
	if rep.Log == nil || rep.Log.LastError == "" {
		t.Fatalf("log = %+v", rep.Log)
	}
	if rep.ActiveRun != nil {
		t.Fatalf("active run should be nil, got %+v", rep.ActiveRun)
	}
}

func TestBuildWithNilChecks(t *testing.T) {
	root := fakeRoot(t)
	rep, err := Build(root, "0.1.0", Checks{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.GameReady() || rep.MXU.Running {
		t.Fatalf("nil checks should stay false: %+v", rep)
	}
	if rep.Instance == nil {
		t.Fatal("instance should still come from config")
	}
}

func TestBuildFailsWithoutInterface(t *testing.T) {
	if _, err := Build(t.TempDir(), "0.1.0", Checks{}); err == nil {
		t.Fatal("want error when interface.json is missing")
	}
}
