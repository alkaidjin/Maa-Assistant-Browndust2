// Package main implements the BD2MAA "trader" agent: a companion process that
// supplies the custom action `TradeRun`, used by 跑商 → `Trade_Start`
// (see resource/pipeline/MapTrade.json).
//
// Why a separate process instead of pipeline JSON:
// the daily shop arbitrage is a data-driven loop — read today's price
// calendar, decide which items peak today, iterate the shop list, verify each
// candidate against the in-game ↑120% marker, sell with precise quantity, then
// bargain + buy all favorites. MaaFramework pipelines cannot iterate a dynamic
// list or run price arithmetic, so the orchestration lives in code while the
// static UI navigation (entering the merchant, opening shop tabs) stays in the
// pipeline and is driven via ctx.RunTask.
//
// Why not inside go-service: that binary carries the weekday gate used by every
// other task; a panic here must not take it down. PI v2 allows an `agent`
// array and MXU 2.5.3 starts each entry in its own process.
//
// Build (see build.bat in this directory):
//
//	go build -trimpath -ldflags "-s -w" -o ../trader.exe .
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

func main() {
	logf("trader starting (args=%v)", os.Args)

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

	registerAll(socketID)

	maa.AgentServerJoin()
	maa.AgentServerShutDown()
	logf("trader stopped")
}

func registerAll(socketID string) {
	if err := maa.AgentServerRegisterCustomAction(tradeActionName, &TradeRun{}); err != nil {
		logf("FATAL: register %s failed: %v", tradeActionName, err)
		os.Exit(1)
	}
	logf("registered custom action: %s", tradeActionName)

	if err := maa.AgentServerRegisterCustomAction("TradeChainTest", &TradeChainTest{}); err != nil {
		logf("FATAL: register TradeChainTest failed: %v", err)
		os.Exit(1)
	}
	logf("registered custom action: TradeChainTest")

	if err := maa.AgentServerStartUp(socketID); err != nil {
		logf("FATAL: AgentServerStartUp failed: %v", err)
		os.Exit(1)
	}
	logf("agent server up, waiting for the client")
}

// resolveLibDir finds the directory holding MaaFramework.dll / MaaAgentServer.dll.
//
// Candidates, in order:
//
//  1. <exe>/../maafw — this project's layout (agent/trader.exe + maafw/)
//  2. <exe>/maafw — in case the binary is ever placed one level higher
//  3. maafw relative to the working directory — what go-service relies on
//
// An empty result means "let Windows search normally".
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
	fmt.Printf("[trader %s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
