// Package main implements the BD2MAA "rock-picker" agent: a small companion
// process to the MaaEnd go-service agent that supplies the custom recognition
// `LeastRockPicker`, used by 狩猎场 → 圣石洞穴 → 「自动：刷数量最少的圣石」.
//
// Why this exists at all:
// MaaFramework's recognition algorithms all answer a boolean question and OCR
// `expected` is a plain regex with no arithmetic, so "read five numbers and take
// the argmin" cannot be expressed in pipeline JSON. A custom recognition is the
// only route, and it lives in its own process (PI v2 allows an `agent` array) so
// a fault here can never take down go-service — which also carries the weekday
// gate used by every other task in this project.
//
// Build (see README.md in this directory):
//
//	go build -trimpath -ldflags "-s -w" -o ../rock-picker.exe .
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

func main() {
	logf("rock-picker starting (args=%v)", os.Args)

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

	if err := maa.AgentServerRegisterCustomRecognition(leastRockRecognitionName, &LeastRockPicker{}); err != nil {
		logf("FATAL: register %s failed: %v", leastRockRecognitionName, err)
		os.Exit(1)
	}
	logf("registered custom recognition: %s", leastRockRecognitionName)

	if err := maa.AgentServerStartUp(socketID); err != nil {
		logf("FATAL: AgentServerStartUp failed: %v", err)
		os.Exit(1)
	}
	logf("agent server up, waiting for the client")

	maa.AgentServerJoin()
	maa.AgentServerShutDown()
	logf("rock-picker stopped")
}

// resolveLibDir finds the directory holding MaaFramework.dll / MaaAgentServer.dll.
//
// The Go binding calls SetDllDirectoryW(libDir) and then loads the libraries by
// bare name, so this must be an absolute path. Candidates, in order:
//
//  1. <exe>/../maafw — this project's layout (agent/rock-picker.exe + maafw/)
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
	fmt.Printf("[rock-picker %s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
