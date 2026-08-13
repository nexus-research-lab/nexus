// INPUT: Agent 工具白名单、黑名单与 Room 私信开关。
// OUTPUT: 不扩大 Agent allow、不移除 Agent deny 的 Room 通讯工具策略与权限处理器。
// POS: Room slot runtime 装配使用的就近策略，不构成独立子包边界。
package realtime

import (
	"context"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
)

const (
	roomSendDirectedMessageTool  = "mcp__nexus_room__send_directed_message"
	roomPublishPublicMessageTool = "mcp__nexus_room__publish_public_message"
)

func roomAllowedTools(values []string, _ bool) []string {
	// Room policy is a lower layer: it may disable communication, but cannot
	// widen an explicit Agent allowlist. An empty allowlist remains unrestricted.
	return slices.Clone(values)
}

func cloneRuntimeToolPolicy(policy *protocol.RuntimeToolPolicy) *protocol.RuntimeToolPolicy {
	if policy == nil {
		return nil
	}
	return &protocol.RuntimeToolPolicy{
		AllowedTools:    slices.Clone(policy.AllowedTools),
		DisallowedTools: slices.Clone(policy.DisallowedTools),
	}
}

func cloneAutomationRunContext(value *protocol.AutomationRunContext) *protocol.AutomationRunContext {
	if value == nil {
		return nil
	}
	result := value.Normalized()
	return &result
}

func roomRoundToolPolicy(round *activeRoomRound, agent *protocol.Agent) (allowed []string, denied []string, snapshotted bool) {
	if round != nil && round.RuntimeToolPolicy != nil {
		return slices.Clone(round.RuntimeToolPolicy.AllowedTools), slices.Clone(round.RuntimeToolPolicy.DisallowedTools), true
	}
	if agent == nil {
		return nil, nil, false
	}
	return slices.Clone(agent.Options.AllowedTools), slices.Clone(agent.Options.DisallowedTools), false
}

func roomDisallowedTools(values []string, privateMessagesEnabled bool) []string {
	// Deny is monotonic across Agent -> Room. Enabling a Room feature never
	// removes an Agent-level deny, including the broad nexus_room family.
	result := slices.Clone(values)
	if !privateMessagesEnabled {
		result = appendDistinctTools(result, roomSendDirectedMessageTool, roomPublishPublicMessageTool)
	}
	return result
}

func withRoomPermissionPolicy(
	next sdkpermission.Handler,
	privateMessagesEnabled bool,
	allowedTools []string,
	disallowedTools []string,
) sdkpermission.Handler {
	allowed := toolpolicy.NormalizeSet(allowedTools)
	denied := toolpolicy.NormalizeSet(disallowedTools)
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if !isRoomCommunicationTool(request.ToolName) {
			if next == nil {
				return sdkpermission.Allow(request.Input, nil), nil
			}
			return next(ctx, request)
		}
		if !privateMessagesEnabled {
			return sdkpermission.Deny("Room communication tools are disabled", false), nil
		}
		if toolpolicy.Contains(denied, request.ToolName) {
			return sdkpermission.Deny("Room communication tool is denied by the Agent policy", false), nil
		}
		if len(allowed) > 0 && !toolpolicy.Contains(allowed, request.ToolName) {
			return sdkpermission.Deny("Room communication tool is outside the Agent allowlist", false), nil
		}
		return sdkpermission.Allow(request.Input, nil), nil
	}
}

func appendDistinctTools(values []string, extra ...string) []string {
	result := make([]string, 0, len(values)+len(extra))
	seen := make(map[string]struct{}, len(values)+len(extra))
	for _, value := range slices.Concat(values, extra) {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func isPrivateMessageTool(toolName string) bool {
	return isRoomTool(toolName, "send_directed_message")
}

func isPublicMessageTool(toolName string) bool {
	return isRoomTool(toolName, "publish_public_message")
}

func isRoomCommunicationTool(toolName string) bool {
	return isPrivateMessageTool(toolName) || isPublicMessageTool(toolName)
}

func isRoomTool(toolName string, leaf string) bool {
	normalized := strings.TrimSpace(toolName)
	switch normalized {
	case leaf,
		"mcp__nexus_room__" + leaf,
		"nexus_room__" + leaf,
		"nexus_room." + leaf,
		"nexus_room/" + leaf:
		return true
	default:
		return false
	}
}
