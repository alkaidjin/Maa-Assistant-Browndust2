// Package main implements the BD2MAA "fishing" agent: a companion process that
// supplies the custom action `FishingMinigame`, used by 钓鱼 → 自动钓鱼 →
// `Fishing_Minigame` (see resource/pipeline/AutoFishing.json).
//
// Why a separate process instead of pipeline JSON:
// the minigame is a closed control loop — read the bar, predict where the cursor
// will be in N milliseconds, click at that moment, repeat until the bar goes
// away. MaaFramework's recognitions only answer "hit / no hit" per frame and no
// pipeline field can express "wait 137ms then click", so the whole loop has to
// live in code.
//
// Why not inside go-service: that binary is an upstream MaaEnd artifact that
// also carries the weekday gate used by every other task; a panic here must not
// take it down. PI v2 allows an `agent` array and MXU 2.5.3 starts each entry in
// its own process.
//
// Build (see README.md in this directory):
//
//	go build -trimpath -ldflags "-s -w" -o ../fishing.exe .
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

func main() {
	logf("fishing starting (args=%v)", os.Args)

	libDir := resolveLibDir()
	if libDir == "" {
		logf("WARNING: 未找到 maafw 目录，回退到系统默认 DLL 搜索路径")
	} else {
		logf("MaaFramework libraries: %s", libDir)
	}

	if err := maa.Init(maa.WithLibDir(libDir)); err != nil {
		logf("FATAL: maa.Init failed: %v", err)
		os.Exit(1)
	}

	// MXU appends the agent socket id as the last argument (after child_args).
	if len(os.Args) < 2 {
		logf("FATAL: 缺少 socket id 参数（MXU 会把 socket id 作为最后一个参数传入）")
		os.Exit(2)
	}
	socketID := os.Args[len(os.Args)-1]

	if err := maa.AgentServerRegisterCustomAction(fishingActionName, &FishingMinigame{}); err != nil {
		logf("FATAL: register %s failed: %v", fishingActionName, err)
		os.Exit(1)
	}
	logf("registered custom action: %s", fishingActionName)

	if err := maa.AgentServerStartUp(socketID); err != nil {
		logf("FATAL: AgentServerStartUp failed: %v", err)
		os.Exit(1)
	}
	logf("agent server up, waiting for the client")

	maa.AgentServerJoin()
	maa.AgentServerShutDown()
	logf("fishing stopped")
}

// resolveLibDir finds the directory holding MaaFramework.dll / MaaAgentServer.dll.
//
// The Go binding calls SetDllDirectoryW(libDir) and then loads the libraries by
// bare name, so this must be an absolute path. Candidates, in order:
//
//  1. <exe>/../maafw — this project's layout (agent/fishing.exe + maafw/)
//  2. <exe>/maafw — in case the binary is ever placed one level higher
//  3. maafw relative to the working directory — what go-service relies on
//
// An empty result means "let Windows search normally", which only works when the
// DLLs are already next to the executable or on PATH.
func resolveLibDir() string {
	var candidates []string

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "..", "maafw"),
			filepath.Join(exeDir, "maafw"),
		)
	}
	candidates = append(candidates, "maafw")

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		return filepath.Clean(abs)
	}
	return ""
}

// logf writes a line to stdout, which MXU captures into
// logs/mxu-agent-<index>-<pid>.log and shows in the run log.
func logf(format string, args ...any) {
	fmt.Printf("[fishing %s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
