package permission

import (
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// NormalizeMode 将宿主配置归一到 runtime 支持的权限模式。
// 外部请求可能来自旧版本前端或手写 API，未知值必须回到安全的 default，
// 不能把未识别字符串继续传给不同内核后产生不一致行为。
func NormalizeMode(mode sdkpermission.Mode) sdkpermission.Mode {
	normalized := sdkpermission.Mode(strings.TrimSpace(string(mode)))
	switch normalized {
	case sdkpermission.ModeDefault,
		sdkpermission.ModeAcceptEdits,
		sdkpermission.ModeBypassPermissions,
		sdkpermission.ModePlan,
		sdkpermission.ModeDontAsk,
		sdkpermission.ModeAuto:
		return normalized
	default:
		return sdkpermission.ModeDefault
	}
}
