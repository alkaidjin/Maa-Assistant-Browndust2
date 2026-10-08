// Package launchgame 一键启动游戏 pretask：自动检索本机棕 2 安装路径并以
// 窗口化 1920x1080 拉起游戏本体；游戏已运行则直接放行，由客户端按
// class/title 搜索绑定（与 interface.json 控制器同口径）。
package launchgame

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	gameProcessName = "BrownDust II.exe"
	// 与 interface.json 控制器的 class_regex / window_regex 同口径：
	// 类名精确等于 UnityWndClass，标题忽略大小写包含 "browndust ii"。
	windowClassName = "UnityWndClass"
	windowTitleKey  = "browndust ii"
	// Unity 启动参数：窗口化 1920x1080（本仓实机验证过的唯一支持档位）。
	launchArgs     = "-screen-width 1920 -screen-height 1080 -screen-fullscreen 0"
	launchTimeout  = 120 * time.Second
	pollInterval   = 200 * time.Millisecond
	sampleInterval = 500 * time.Millisecond
	stablePeriod   = 2 * time.Second
)

// rememberedPathFile 记忆上次成功启动用的游戏 exe 路径，作为所有
// 平台检索链失败后的最后兜底（注册表被清理 / 启动器升级换键等场景）。
var rememberedPathFile = filepath.Join("config", "launchgame.json")

// Run 执行 LaunchGame pretask，返回是否成功。
func Run(_ []string) bool {
	// ① 游戏窗口已存在 → 客户端稍后自行搜窗绑定，直接放行。
	if hwnd, ok := findGameWindow(); ok {
		log.Info().
			Uint64("hwnd", uint64(hwnd)).
			Msg("launchgame: game window already present, skip launching")
		return true
	}

	// ② 进程在但窗口还没出来（启动器/加载中）→ 等窗口即可，不要重复拉起。
	running, enumOK := isGameRunning()
	if running {
		log.Info().
			Msg("launchgame: game process is running, wait for its window")
		return waitWindowStable()
	}
	if !enumOK {
		log.Warn().
			Msg("launchgame: failed to enumerate processes, try launching anyway (game has single-instance guard)")
	}

	// ③ 检索安装路径（平台三链 + 记忆兜底，见 resolveGamePath）。
	gamePath, source := resolveGamePath()
	if gamePath == "" {
		log.Error().
			Msg("launchgame: game executable not found; searched Neowiz starter registry, default install locations on all fixed drives, uninstall entries and remembered path in config/launchgame.json")
		return false
	}
	log.Info().
		Str("source", source).
		Str("path", gamePath).
		Msg("launchgame: resolved game path")

	// ④ 拉起游戏本体（不等待退出；窗口就绪由 waitWindowStable 轮询确认）。
	if err := startGame(gamePath); err != nil {
		log.Error().
			Err(err).
			Str("path", gamePath).
			Msg("launchgame: failed to start game")
		return false
	}
	log.Info().
		Str("args", launchArgs).
		Msg("launchgame: game process launched")

	// ⑤ 等待窗口出现且客户区尺寸稳定（期间窗口若被关掉则回到找窗步骤）。
	if !waitWindowStable() {
		return false
	}

	// ⑥ 只有真正拉起并等到窗口后才更新记忆路径；失败不覆盖旧记忆。
	saveRememberedPath(gamePath)
	return true
}

// resolveGamePath 按优先级检索游戏 exe，返回 (路径, 来源标签)：
//  1. 平台检索链 findInstalledGame（Windows：启动器注册表 → 全部本地盘
//     默认路径 → 卸载信息表）；
//  2. 记忆兜底：config/launchgame.json 里上次成功启动的路径。
func resolveGamePath() (string, string) {
	if path, source := findInstalledGame(); path != "" {
		return path, source
	}
	if path := loadRememberedPath(); path != "" {
		log.Info().
			Str("path", path).
			Msg("launchgame: using remembered game path")
		return path, "remembered-path"
	}
	return "", ""
}

// launchgameMemory 是 config/launchgame.json 的持久化结构。
type launchgameMemory struct {
	GameExe   string `json:"game_exe"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func loadRememberedPath() string {
	data, err := os.ReadFile(rememberedPathFile)
	if err != nil {
		return ""
	}
	var memory launchgameMemory
	if err := json.Unmarshal(data, &memory); err != nil {
		log.Warn().
			Err(err).
			Str("file", rememberedPathFile).
			Msg("launchgame: failed to parse remembered path file, ignore")
		return ""
	}
	if !isGameExe(memory.GameExe) {
		return ""
	}
	return memory.GameExe
}

func saveRememberedPath(gamePath string) {
	if err := os.MkdirAll(filepath.Dir(rememberedPathFile), 0755); err != nil {
		log.Warn().
			Err(err).
			Msg("launchgame: failed to create config dir for remembered path")
		return
	}
	memory := launchgameMemory{
		GameExe:   gamePath,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(memory, "", "    ")
	if err != nil {
		return
	}
	if err := os.WriteFile(rememberedPathFile, data, 0644); err != nil {
		log.Warn().
			Err(err).
			Str("file", rememberedPathFile).
			Msg("launchgame: failed to save remembered game path")
		return
	}
	log.Info().
		Str("file", rememberedPathFile).
		Str("path", gamePath).
		Msg("launchgame: remembered game path updated")
}

// isGameRunning 按 exe 名（忽略大小写）检查游戏进程是否在运行。
// enumOK=false 表示进程枚举本身失败，调用方应继续尝试拉起（游戏有单实例守卫兜底）。
func isGameRunning() (running, enumOK bool) {
	processes, err := process.Processes()
	if err != nil {
		return false, false
	}
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if strings.EqualFold(name, gameProcessName) {
			return true, true
		}
	}
	return false, true
}

func startGame(gamePath string) error {
	cmd := exec.Command(gamePath, strings.Fields(launchArgs)...)
	cmd.Dir = filepath.Dir(gamePath)
	return cmd.Start()
}

// waitWindowStable 在 launchTimeout 内等待游戏窗口出现，且客户区尺寸连续
// stablePeriod 不变（窗口若中途被关闭则重新寻找）。pretask 退出后客户端
// 只搜一次窗、不重试，因此必须等窗口稳定再返回。
func waitWindowStable() bool {
	deadline := time.Now().Add(launchTimeout)
	for {
		hwnd, ok := findGameWindow()
		if ok && waitClientSizeStable(hwnd, deadline) {
			return true
		}
		if time.Now().After(deadline) {
			log.Error().
				Str("timeout", launchTimeout.String()).
				Msg("launchgame: timed out waiting for a stable game window")
			return false
		}
		time.Sleep(pollInterval)
	}
}

// waitClientSizeStable 等待窗口客户区尺寸连续 stablePeriod 不变化。
// hwnd 若中途失效（窗口被关闭/重建）返回 false，交由外层重新找窗。
func waitClientSizeStable(hwnd uintptr, deadline time.Time) bool {
	var lastW, lastH int
	var stableSince time.Time
	for {
		w, h, err := clientSize(hwnd)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("launchgame: game window vanished while waiting, re-polling")
			return false
		}
		now := time.Now()
		if w != lastW || h != lastH {
			lastW, lastH = w, h
			stableSince = now
			log.Info().
				Int("client_width", w).
				Int("client_height", h).
				Msg("launchgame: game window appeared, waiting for stable client size")
		} else if !stableSince.IsZero() && now.Sub(stableSince) >= stablePeriod {
			log.Info().
				Int("client_width", w).
				Int("client_height", h).
				Msg("launchgame: game window client size is stable")
			return true
		}
		if now.After(deadline) {
			log.Error().
				Msg("launchgame: timed out waiting for stable client size")
			return false
		}
		time.Sleep(sampleInterval)
	}
}

// isGameExe 校验候选路径存在、是文件且文件名与游戏进程名一致（忽略大小写）。
// 平台无关（纯文件系统判断），各平台检索链与记忆兜底共用。
func isGameExe(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return strings.EqualFold(filepath.Base(path), gameProcessName)
}
