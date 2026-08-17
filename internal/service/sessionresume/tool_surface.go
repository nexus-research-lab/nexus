// INPUT: resolved provider/model 与旧、新模型工具面脱敏指纹。
// OUTPUT: 是否必须结束旧 K3 工具基线并创建 fresh SDK session。
// POS: Nexus 产品层对 K3 会话内动态工具兼容性的单一判定入口。
package sessionresume

import (
	"net/url"
	"strings"
)

// RequiresK3ToolSurfaceReset 判断旧 K3 SDK session 是否不能安全复用。
//
// K3 会话的顶层全局工具应保持稳定；旧会话没有指纹或工具面已变化时，
// Nexus 保留旧 transcript lineage，并让 fresh session 从首轮采用当前工具面。
func RequiresK3ToolSurfaceReset(
	provider string,
	model string,
	baseURL string,
	storedFingerprint string,
	currentFingerprint string,
) bool {
	if !IsKimiK3Runtime(provider, model, baseURL) {
		return false
	}
	currentFingerprint = strings.TrimSpace(currentFingerprint)
	if currentFingerprint == "" {
		return false
	}
	return strings.TrimSpace(storedFingerprint) != currentFingerprint
}

// IsKimiK3Runtime 判断 resolved runtime 是否遵循 K3 的稳定全局工具基线语义。
func IsKimiK3Runtime(provider string, model string, baseURL string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.ToLower(strings.TrimSpace(model))
	modelIsK3 := model == "k3" || model == "kimi-k3" || strings.HasPrefix(model, "kimi-k3-")
	if !modelIsK3 {
		return false
	}
	return strings.Contains(provider, "kimi") || strings.Contains(provider, "moonshot") ||
		strings.HasPrefix(model, "kimi-") || isKimiAPIURL(baseURL)
}

func isKimiAPIURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "api.kimi.com" || host == "api.moonshot.ai"
}
