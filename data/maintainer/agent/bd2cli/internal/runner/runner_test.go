package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestMXUArgsIncludeInstanceAndQuit(t *testing.T) {
	m := MXU{Exe: "mxu.exe"}
	got := m.Args(Job{Instance: "配置 1"})
	want := []string{"--autostart", "--instance", "配置 1", "--quit-after-run"}
	if len(got) != len(want) {
		t.Fatalf("args = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMXUArgsWithoutInstance(t *testing.T) {
	m := MXU{}
	got := m.Args(Job{})
	if len(got) != 2 || got[0] != "--autostart" || got[1] != "--quit-after-run" {
		t.Fatalf("args = %v", got)
	}
}

func TestMXURunUsesInjectedExec(t *testing.T) {
	var gotArgs []string
	var gotDir string
	m := MXU{
		Exe: "mxu.exe",
		Dir: "F:\\proj",
		RunFn: func(ctx context.Context, exe string, args []string, dir string, out io.Writer) (int, error) {
			gotArgs, gotDir = args, dir
			_, _ = out.Write([]byte("hi"))
			return 0, nil
		},
	}
	var buf bytes.Buffer
	code, err := m.Run(context.Background(), Job{Instance: "配置 1"}, &buf)
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if gotDir != "F:\\proj" || len(gotArgs) != 4 {
		t.Fatalf("dir=%q args=%v", gotDir, gotArgs)
	}
	if buf.String() != "hi" {
		t.Fatalf("out = %q", buf.String())
	}
}

func TestStubRunnerRecordsJob(t *testing.T) {
	s := &Stub{Exit: 4}
	code, err := s.Run(context.Background(), Job{Task: "DailyAll"}, io.Discard)
	if code != 4 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if s.Got.Task != "DailyAll" {
		t.Fatalf("job = %+v", s.Got)
	}
}

func TestStubRunnerHonoursContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	s := &Stub{Sleep: time.Second}
	if _, err := s.Run(ctx, Job{}, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}
