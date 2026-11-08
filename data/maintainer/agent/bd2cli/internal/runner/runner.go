// Package runner 抽象"怎么真正跑一次任务"，实现可替换、可分阶段。
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Job 是一次运行请求。
type Job struct {
	Task     string
	Instance string
	Timeout  time.Duration
}

// Runner 是所有运行后端的统一接口。
type Runner interface {
	Name() string
	Run(ctx context.Context, j Job, out io.Writer) (exitCode int, err error)
}

// Stub 不启动任何东西，用于测试退出码映射与记录落盘。
type Stub struct {
	Exit  int
	Sleep time.Duration
	Err   error
	Got   Job
}

// Name 返回后端名。
func (s *Stub) Name() string { return "stub" }

// Run 按配置模拟一次运行。
func (s *Stub) Run(ctx context.Context, j Job, out io.Writer) (int, error) {
	s.Got = j
	if s.Sleep > 0 {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(s.Sleep):
		}
	}
	return s.Exit, s.Err
}

// MXU 用 `mxu.exe --autostart --instance <名字> --quit-after-run` 跑整个实例的已启用任务。
//
// 局限（必须如实报告给调用者）：它不接受单个任务名，粒度是"实例的启用任务列表"；
// 退出码是否反映任务失败取决于 MXU 自身，尚未验证。
type MXU struct {
	Exe   string
	Dir   string
	RunFn func(ctx context.Context, exe string, args []string, dir string, out io.Writer) (int, error)
}

// Name 返回后端名。
func (m MXU) Name() string { return "mxu" }

// Args 拼出 MXU 的命令行。单独抽出来是为了能被测试直接断言。
func (m MXU) Args(j Job) []string {
	args := []string{"--autostart"}
	if j.Instance != "" {
		args = append(args, "--instance", j.Instance)
	}
	args = append(args, "--quit-after-run")
	return args
}

// Run 启动 MXU 并等它退出。
func (m MXU) Run(ctx context.Context, j Job, out io.Writer) (int, error) {
	fn := m.RunFn
	if fn == nil {
		fn = ExecRun
	}
	return fn(ctx, m.Exe, m.Args(j), m.Dir, out)
}

// ExecRun 是默认实现：跑子进程并把 stdout/stderr 透传出去，返回退出码。
func ExecRun(ctx context.Context, exe string, args []string, dir string, out io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	return 0, fmt.Errorf("exec %s: %w", exe, err)
}
