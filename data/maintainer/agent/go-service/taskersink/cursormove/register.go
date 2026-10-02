package cursormove

// Register adds the cursor-move sinks when the controller is Win32.
//
// 上游 MaaEnd 64fac11 在原函数体开头以一条裸 return 禁用了本 sink
// （光标归位在终末地端引入回归后被临时关闭，之后未恢复）。
// 本仓保持与随仓 go-service.exe 完全一致的行为：不注册任何 sink。
// CursorMoveSink 的完整实现保留在 sink.go，将来上游恢复时可直接重新挂载。
func Register() {
	// Intentionally empty — matches MaaEnd 64fac11 runtime behaviour.
}
