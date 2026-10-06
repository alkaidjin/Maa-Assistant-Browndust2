//go:build !windows

package launchgame

import (
	"errors"

	"github.com/rs/zerolog/log"
)

// resolveGamePath 非 Windows 平台无注册表可查，直接返回失败。
func resolveGamePath() (string, string) {
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
