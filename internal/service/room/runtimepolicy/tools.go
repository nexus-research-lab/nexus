package runtimepolicy

import (
	"context"
	"slices"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
)

const (
	PublishPublicMessageTool = "mcp__nexus_room__publish_public_message"
	SendDirectedMessageTool  = "mcp__nexus_room__send_directed_message"
)

func AllowedTools(values []string, privateMessagesEnabled bool) []string {
	if len(toolpolicy.NormalizeSet(values)) == 0 {
		return values
	}
	extra := []string{PublishPublicMessageTool}
	if privateMessagesEnabled {
		extra = append(extra, SendDirectedMessageTool)
	}
	return appendDistinctTools(values, extra...)
}

func DisallowedTools(values []string, privateMessagesEnabled bool) []string {
	result := make([]string, 0, len(values)+1)
	for _, value := range values {
		if isPublicMessageTool(value) ||
			strings.TrimSpace(value) == "nexus_room" ||
			(privateMessagesEnabled && isPrivateMessageTool(value)) {
			continue
		}
		result = append(result, value)
	}
	if !privateMessagesEnabled {
		result = appendDistinctTools(result, SendDirectedMessageTool)
	}
	return result
}

func PermissionHandler(next sdkpermission.Handler, privateMessagesEnabled bool) sdkpermission.Handler {
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if isPublicMessageTool(request.ToolName) ||
			(isPrivateMessageTool(request.ToolName) && privateMessagesEnabled) {
			return sdkpermission.Allow(request.Input, nil), nil
		}
		if isPrivateMessageTool(request.ToolName) {
			return sdkpermission.Deny("Room private messages are disabled", false), nil
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

func isPublicMessageTool(toolName string) bool {
	return isRoomTool(toolName, "publish_public_message")
}

func isPrivateMessageTool(toolName string) bool {
	return isRoomTool(toolName, "send_directed_message")
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
