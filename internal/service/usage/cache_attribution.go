// INPUT: 宿主可证明的 Goal/Execution responsibility 与脱敏 runtime surface。
// OUTPUT: 固定枚举、低基数 scope/lane 和严格 SHA-256 指纹。
// POS: usage service 的 cache correlation 信任边界；不接受 prompt 派生身份。
package usage

import (
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const (
	ScopeUnknown = "unknown"
	ScopeNone    = "none"
	ScopeBound   = "bound"

	ToolSurfaceUnknown = "unknown"
	ToolSurfaceAbsent  = "absent"
	ToolSurfacePresent = "present"
)

var responsibilityLanes = map[string]struct{}{
	"unknown":      {},
	"unbound":      {},
	"planning":     {},
	"execution":    {},
	"coordination": {},
	"work":         {},
	"review":       {},
}

// CacheAttribution 只保存低基数宿主状态与单向指纹，不保存 Goal/Execution ID、
// prompt、tool name、description 或 schema 明文。
type CacheAttribution struct {
	GoalScope               string
	ExecutionScope          string
	ResponsibilityLane      string
	RuntimeKind             string
	ProviderFingerprint     string
	ModelFingerprint        string
	GoalToolSurface         string
	ExecutionToolSurface    string
	HostToolSurfaceComplete bool
	ToolPolicyFingerprint   string
	MCPServersFingerprint   string
	ToolSurfaceFingerprint  string
}

func BoundScope(bound bool) string {
	if bound {
		return ScopeBound
	}
	return ScopeNone
}

func ToolSurface(present bool) string {
	if present {
		return ToolSurfacePresent
	}
	return ToolSurfaceAbsent
}

// RuntimeCacheAttribution 把 Manager 已成功采用的 runtime surface 与调用点
// 同时读取的 responsibility snapshot 合并。observed=false 时工具面保持 unknown。
func RuntimeCacheAttribution(
	profile runtimectx.CacheSurfaceInput,
	observed bool,
	goalBound bool,
	executionBound bool,
	responsibilityLane string,
) CacheAttribution {
	result := CacheAttribution{
		GoalScope:            BoundScope(goalBound),
		ExecutionScope:       BoundScope(executionBound),
		ResponsibilityLane:   responsibilityLane,
		RuntimeKind:          ScopeUnknown,
		GoalToolSurface:      ToolSurfaceUnknown,
		ExecutionToolSurface: ToolSurfaceUnknown,
	}
	if !observed {
		return normalizeCacheAttribution(result)
	}
	result.RuntimeKind = profile.RuntimeKind
	result.ProviderFingerprint = profile.ProviderFingerprint
	result.ModelFingerprint = profile.ModelFingerprint
	if profile.ManagedToolPresenceKnown {
		result.GoalToolSurface = ToolSurface(profile.GoalToolSurfacePresent)
		result.ExecutionToolSurface = ToolSurface(profile.ExecutionToolSurfacePresent)
	}
	result.HostToolSurfaceComplete = profile.HostToolSurfaceComplete
	result.ToolPolicyFingerprint = profile.ToolPolicyFingerprint
	result.MCPServersFingerprint = profile.MCPServersFingerprint
	result.ToolSurfaceFingerprint = profile.ToolSurfaceFingerprint
	return normalizeCacheAttribution(result)
}

func normalizeCacheAttribution(input CacheAttribution) CacheAttribution {
	return CacheAttribution{
		GoalScope:               normalizeEnum(input.GoalScope, ScopeUnknown, ScopeNone, ScopeBound),
		ExecutionScope:          normalizeEnum(input.ExecutionScope, ScopeUnknown, ScopeNone, ScopeBound),
		ResponsibilityLane:      normalizeResponsibilityLane(input.ResponsibilityLane),
		RuntimeKind:             normalizeEnum(input.RuntimeKind, ScopeUnknown, "nxs", "claude"),
		ProviderFingerprint:     normalizeFingerprint(input.ProviderFingerprint),
		ModelFingerprint:        normalizeFingerprint(input.ModelFingerprint),
		GoalToolSurface:         normalizeEnum(input.GoalToolSurface, ToolSurfaceUnknown, ToolSurfaceAbsent, ToolSurfacePresent),
		ExecutionToolSurface:    normalizeEnum(input.ExecutionToolSurface, ToolSurfaceUnknown, ToolSurfaceAbsent, ToolSurfacePresent),
		HostToolSurfaceComplete: input.HostToolSurfaceComplete,
		ToolPolicyFingerprint:   normalizeFingerprint(input.ToolPolicyFingerprint),
		MCPServersFingerprint:   normalizeFingerprint(input.MCPServersFingerprint),
		ToolSurfaceFingerprint:  normalizeFingerprint(input.ToolSurfaceFingerprint),
	}
}

func normalizeResponsibilityLane(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := responsibilityLanes[value]; ok {
		return value
	}
	return ScopeUnknown
}

func normalizeEnum(value string, fallback string, allowed ...string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

func normalizeFingerprint(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return ""
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return ""
		}
	}
	return value
}
