// INPUT: Agent 工具白名单、黑名单与 Room 私信开关。
// OUTPUT: Room 通讯工具策略与权限处理器。
// POS: Room slot runtime 装配使用的就近策略，不构成独立子包边界。
package realtime

import (
	"context"
	"slices"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
)

const (
	roomSendDirectedMessageTool  = "mcp__nexus_room__send_directed_message"
	roomPublishPublicMessageTool = "mcp__nexus_room__publish_public_message"
)

func roomAllowedTools(values []string, privateMessagesEnabled bool) []string {
	if len(toolpolicy.NormalizeSet(values)) == 0 {
		return values
	}
	var extra []string
	if privateMessagesEnabled {
		extra = append(extra, roomSendDirectedMessageTool, roomPublishPublicMessageTool)
	}
	return appendDistinctTools(values, extra...)
}

func roomDisallowedTools(values []string, privateMessagesEnabled bool) []string {
	result := make([]string, 0, len(values)+2)
	for _, value := range values {
		if strings.TrimSpace(value) == "nexus_room" ||
			(privateMessagesEnabled && isRoomCommunicationTool(value)) {
			continue
		}
		result = append(result, value)
	}
	if !privateMessagesEnabled {
		result = appendDistinctTools(result, roomSendDirectedMessageTool, roomPublishPublicMessageTool)
	}
	return result
}

func withRoomPermissionPolicy(next sdkpermission.Handler, privateMessagesEnabled bool) sdkpermission.Handler {
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if isRoomCommunicationTool(request.ToolName) && privateMessagesEnabled {
			return sdkpermission.Allow(request.Input, nil), nil
		}
		if isRoomCommunicationTool(request.ToolName) {
			return sdkpermission.Deny("Room communication tools are disabled", false), nil
		}
		if next == nil {
			return sdkpermission.Allow(request.Input, nil), nil
		}
		return next(ctx, request)
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
