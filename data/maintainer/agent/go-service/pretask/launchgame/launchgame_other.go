//go:build !windows

package launchgame

import (
	"errors"

	"github.com/rs/zerolog/log"
)

// findInstalledGame 非 Windows 平台无注册表 / 默认安装布局可查。
// 记忆兜底（config/launchgame.json）由跨平台主文件负责。
func findInstalledGame() (string, string) {
	log.Error().
		Msg("launchgame: path resolution is only implemented on windows")
	return "", ""
}

func findGameWindow() (uintptr, bool) {
	return 0, false
}

func clientSize(hwnd uintptr) (int, int, error) {
	return 0, 0, errors.New("launchgame: client size is only implemented on windows")
}
