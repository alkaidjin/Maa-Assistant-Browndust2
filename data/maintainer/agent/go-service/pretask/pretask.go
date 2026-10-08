// Package pretask 实现 PI v2.7.0 pretask CLI 模式入口。
//
// MXU（v2.5.3+）按 interface.json 顶层 `pretask` 声明，在连接控制器前直接拉起
// `agent/go-service --pretask <Name> [args...]` 并同步等待退出：
//   - 退出码非 0 仅告警、不中止客户端启动，因此失败原因必须写进 debug/go-service.log；
//   - stdout 被客户端丢弃，不要依赖控制台输出传达信息。
package pretask

import (
	"os"

	"github.com/rs/zerolog/log"
	"go-service/pretask/launchgame"
)

// Handler 处理一个 pretask 任务；返回 false 表示执行失败。
type Handler func(args []string) bool

var registry = map[string]Handler{}

func init() {
	Register("LaunchGame", launchgame.Run)
}

// Register 注册一个 pretask 任务处理器。
func Register(name string, handler Handler) {
	registry[name] = handler
}

// Run 是 `go-service --pretask` 的 CLI 入口：
//   - 未知任务名 / 缺参 → exit 2（配置错误）；
//   - 处理器返回 false → exit 1（执行失败）；
//   - 成功 → exit 0。
func Run(args []string) {
	if len(args) < 1 {
		log.Error().
			Strs("available", availableNames()).
			Msg("pretask: missing task name, usage: go-service --pretask <Name> [args...]")
		os.Exit(2)
	}

	name := args[0]
	handler, ok := registry[name]
	if !ok {
		log.Error().
			Str("name", name).
			Strs("available", availableNames()).
			Msg("pretask: unknown task name")
		os.Exit(2)
	}

	log.Info().
		Str("name", name).
		Msg("pretask: start")

	if !handler(args[1:]) {
		log.Error().
			Str("name", name).
			Msg("pretask: failed")
		os.Exit(1)
	}

	log.Info().
		Str("name", name).
		Msg("pretask: done")
}

func availableNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
