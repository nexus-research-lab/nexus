package goal

import "strings"

// ShouldIgnoreRuntimeForPermissionMode 对齐 Codex should_ignore_goal_for_mode：
// Plan 模式下不注入 Goal 上下文、不记录 Goal runtime usage，也不自动续跑。
func ShouldIgnoreRuntimeForPermissionMode(permissionMode string) bool {
	return strings.TrimSpace(permissionMode) == "plan"
}
