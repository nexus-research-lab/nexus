// INPUT: 桌面运行模式与 Linux launcher 权限托管状态。
// OUTPUT: 启动迁移是否需要执行 Unix 文件权限收紧。
// POS: 状态、workspace、补偿恢复与 Room 文件迁移共享的权限职责边界。
package migration

import (
	"os"
	"strings"
)

const nexusAppModeEnvironment = "NEXUS_APP_MODE"

// shouldHardenMigratedPermissions 只保留非桌面、非 launcher 托管部署的旧行为。
//
// Windows/macOS 桌面进程与所有 Agent 共用同一个 OS 用户，递归 chmod 不能形成
// Agent 隔离，反而会改写用户文件并被 Windows 保留设备名阻断。Linux enforce
// 的 owner/group/ACL 则由 launcher identity sync 统一维护。
func shouldHardenMigratedPermissions(launcherManagesPermissions bool) bool {
	if launcherManagesPermissions {
		return false
	}
	return !strings.EqualFold(
		strings.TrimSpace(os.Getenv(nexusAppModeEnvironment)),
		"desktop",
	)
}
