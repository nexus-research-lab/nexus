// INPUT: Automation create/update 提交的 SDK permission_mode。
// OUTPUT: 可持久化的具体模式规范化结果与枚举校验。
// POS: 定时任务权限模式协议真相源；创建时 copy 语义由 service 对空值解析。
package types

import (
	"errors"
	"strings"
)

const (
	PermissionModeDefault           = "default"
	PermissionModePlan              = "plan"
	PermissionModeAcceptEdits       = "acceptEdits"
	PermissionModeBypassPermissions = "bypassPermissions"
	PermissionModeDontAsk           = "dontAsk"
)

// NormalizePermissionMode 返回定时任务 SDK 权限模式的兼容默认值。
func NormalizePermissionMode(mode string) string {
	normalized := strings.TrimSpace(mode)
	if normalized == "" {
		return PermissionModeDefault
	}
	return normalized
}

func validatePermissionMode(mode string) error {
	if strings.TrimSpace(mode) == "" {
		return nil
	}
	switch NormalizePermissionMode(mode) {
	case PermissionModeDefault,
		PermissionModePlan,
		PermissionModeAcceptEdits,
		PermissionModeBypassPermissions,
		PermissionModeDontAsk:
		return nil
	default:
		return errors.New("permission_mode must be one of default, plan, acceptEdits, bypassPermissions, dontAsk")
	}
}
