// Package stopall 提供一个 custom action：终止当前任务，并清空同一 Tasker 中
// 排队（尚未开始）的其余任务。
//
// 为什么需要它：pipeline 的 `StopTask` 动作只把当前 Context 标记为 need_to_stop，
// 结束的是"这条 task chain"（MaaTaskerPostTask 提交的那一个）；同一 Tasker 里
// 已排队但还没跑的任务不受影响。真正清空队列的是 MaaTaskerPostStop
// （Tasker::post_stop：need_to_stop_ = true + task_runner_->clear()）。
// 这里把它暴露成节点可以引用的动作，供"检测到游戏维护 → 整轮收工"这类场景使用。
package stopall

import (
	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"

	"go-service/pkg/maafocus"
)

// ActionName 是 pipeline 里 custom_action 引用的名字。
const ActionName = "StopAll"

// StopAllAction 实现 maa.CustomActionRunner。
type StopAllAction struct{}

// 编译期接口检查
var _ maa.CustomActionRunner = &StopAllAction{}

// Register 把本动作注册到 agent server。
func Register() {
	if err := maa.AgentServerRegisterCustomAction(ActionName, &StopAllAction{}); err != nil {
		log.Error().
			Err(err).
			Str("component", ActionName).
			Msg("failed to register custom action")
		return
	}
	log.Info().
		Str("component", ActionName).
		Msg("custom action registered")
}

// Run 发送 post_stop：清空排队任务 + 通知正在运行的任务停止。
//
// 注意：这里不能等待 PostStop 返回的 job —— 当前任务正占着 runner，
// 同步等待会死锁。发出去之后直接返回，由 PipelineTask 的 need_to_stop 收尾。
func (a *StopAllAction) Run(ctx *maa.Context, arg *maa.CustomActionArg) bool {
	node := ""
	if arg != nil {
		node = arg.CurrentTaskName
	}

	if ctx == nil {
		log.Error().
			Str("component", ActionName).
			Str("node", node).
			Msg("got nil context")
		return false
	}

	tasker := ctx.GetTasker()
	if tasker == nil {
		log.Error().
			Str("component", ActionName).
			Str("node", node).
			Msg("got nil tasker")
		return false
	}

	// 先给 UI 一个可见提示，再发停止信号（停止后 context 可能不再接受新动作）。
	maafocus.Print(ctx, "检测到游戏维护公告，无法登录：已结束当前任务，并清空排队中的任务")

	log.Warn().
		Str("component", ActionName).
		Str("node", node).
		Msg("stop requested: clearing queued tasks via MaaTaskerPostStop")

	tasker.PostStop()

	return true
}
