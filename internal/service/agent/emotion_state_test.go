// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package agent

// SetRuntimeEmotionBaseAtVersion 仅在 version 匹配时更新基础情绪。
func SetRuntimeEmotionBaseAtVersion(
	workspacePath string,
	update RuntimeEmotionBaseUpdate,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	return setRuntimeEmotionBaseAtVersion(workspacePath, update, &expectedVersion)
}

// SetRuntimeEmotionContextAtVersion 仅在 version 匹配时更新指定上下文情绪。
func SetRuntimeEmotionContextAtVersion(
	workspacePath string,
	update RuntimeEmotionContextUpdate,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	return setRuntimeEmotionContextAtVersion(workspacePath, update, &expectedVersion)
}
