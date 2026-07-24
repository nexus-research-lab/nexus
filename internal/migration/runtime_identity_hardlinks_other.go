//go:build !linux

package migration

// validateRuntimeIsolationHardLinks 在非 Linux 平台不启用 OS runtime 隔离。
// 这些平台保持单用户兼容路径，真正的检查只在 Linux enforce 启动链路执行。
func validateRuntimeIsolationHardLinks(stateRoot string) error {
	return nil
}
