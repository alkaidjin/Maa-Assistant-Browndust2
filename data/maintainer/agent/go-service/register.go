package main

// register.go —— BD2MAA 裁剪版。
//
// 上游 MaaEnd 64fac11（AGPL-3.0）的 registerAll() 会注册约 50 个终末地业务组件；
// 本项目 pipeline 实际只引用 ScheduleRecognition 一个自定义识别，其余所需能力
// 全部以 tasker sink 方式自动挂载（分辨率 / HDR / 进程 / 任务失败 / 光标）。
// 裁剪清单与依据见本目录 README.md。

import (
	"github.com/rs/zerolog/log"
	"go-service/common/schedule"
	"go-service/pkg/resource"
	"go-service/taskersink/aspectratio"
	"go-service/taskersink/cursormove"
	"go-service/taskersink/hdrcheck"
	"go-service/taskersink/processcheck"
	"go-service/taskersink/taskfail"
)

func registerAll() {
	// Resource Sink —— 资源路径修正
	resource.EnsureResourcePathSink()

	// Pre-Check Custom Sinks（任务启动前自动生效，pipeline 无需引用）
	aspectratio.Register()  // 分辨率 / 16:9 守护，不符则弹 HTML 警告并 PostStop
	hdrcheck.Register()     // Windows HDR 状态守护
	processcheck.Register() // 黑名单进程守护 + HTML 警告页
	taskfail.Register()     // 任务失败反馈引导
	cursormove.Register()   // 光标归位：上游 64fac11 起 Register() 内即 return 空操作，原样保留挂载点

	// Business Custom —— pipeline 经 custom_recognition 引用
	schedule.Register() // ScheduleRecognition：星期门控（WarcraftShop / 吸收召集）

	log.Info().
		Msg("BD2MAA custom components and sinks registered")
}
