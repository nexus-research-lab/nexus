// INPUT: 外部 IM 中权限确认命令的首个 token。
// OUTPUT: 与展示缩写解耦的稳定权限意图；旧长命令仅作为兼容别名。
// POS: 普通 runtime 与 Automation 共用的 IM 权限命令协议真相源。
package protocol

import "strings"

type IMPermissionCommand string

const (
	IMPermissionCommandAllowOnce   IMPermissionCommand = "allow_once"
	IMPermissionCommandAllowAlways IMPermissionCommand = "allow_always"
	IMPermissionCommandDeny        IMPermissionCommand = "deny"
	IMPermissionCommandRetry       IMPermissionCommand = "retry"

	IMPermissionSlashAllowOnce   = "/y"
	IMPermissionSlashAllowAlways = "/a"
	IMPermissionSlashDeny        = "/d"
)

// ParseIMPermissionSlashName 将公开短命令和历史长命令归一成权限意图。
// /retry 只为旧 Automation connector 通知保留；新通知统一用 /y。
func ParseIMPermissionSlashName(value string) (IMPermissionCommand, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "／") {
		value = "/" + strings.TrimPrefix(value, "／")
	}
	switch strings.ToLower(value) {
	case IMPermissionSlashAllowOnce, "/approve":
		return IMPermissionCommandAllowOnce, true
	case IMPermissionSlashAllowAlways, "/always":
		return IMPermissionCommandAllowAlways, true
	case IMPermissionSlashDeny, "/deny":
		return IMPermissionCommandDeny, true
	case "/retry":
		return IMPermissionCommandRetry, true
	default:
		return "", false
	}
}
