package channels

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

var defaultReadOnlyApprovedTools = map[string]struct{}{
	"Glob":      {},
	"Grep":      {},
	"LS":        {},
	"Read":      {},
	"Skill":     {},
	"WebFetch":  {},
	"WebSearch": {},
}

var defaultScheduledTaskApprovedTools = map[string]struct{}{
	"create_scheduled_task":           {},
	"delete_scheduled_task":           {},
	"disable_scheduled_task":          {},
	"enable_scheduled_task":           {},
	"get_scheduled_task_daily_report": {},
	"get_scheduled_task_events":       {},
	"get_scheduled_task_runs":         {},
	"get_scheduled_task_status":       {},
	"list_scheduled_tasks":            {},
	"recover_scheduled_task":          {},
	"retry_scheduled_task_delivery":   {},
	"run_scheduled_task":              {},
	"search_scheduled_task_history":   {},
	"update_scheduled_task":           {},
}

var defaultGoalApprovedTools = map[string]struct{}{
	"create_goal": {},
	"get_goal":    {},
	"update_goal": {},
}

var defaultManagedSupportTools = map[string]struct{}{
	"Skill": {},
}

var defaultExternalApprovedTools = toolpolicy.MergeSets(defaultReadOnlyApprovedTools, defaultScheduledTaskApprovedTools, defaultGoalApprovedTools)

func (s *IngressService) resolveApprovedTools(channel string, explicit []string) map[string]struct{} {
	if len(explicit) > 0 {
		return toolpolicy.NormalizeSet(explicit)
	}
	if channel == ChannelTypeInternal {
		return toolpolicy.CopySet(defaultReadOnlyApprovedTools)
	}
	return toolpolicy.CopySet(defaultExternalApprovedTools)
}

func (s *IngressService) buildPermissionHandler(
	agentValue *protocol.Agent,
	request normalizedIngressRequest,
) sdkpermission.Handler {
	allowedByAgent := toolpolicy.NormalizeSet(agentValue.Options.AllowedTools)
	approved := request.autoApproveTools
	if request.channelStored == ChannelTypeInternal && len(approved) == 0 {
		if len(allowedByAgent) > 0 {
			approved = toolpolicy.CopySet(allowedByAgent)
		} else {
			approved = toolpolicy.CopySet(defaultReadOnlyApprovedTools)
		}
	}
	return func(_ context.Context, permissionRequest sdkpermission.Request) (sdkpermission.Decision, error) {
		toolName := strings.TrimSpace(permissionRequest.ToolName)
		if toolName == "" {
			return sdkpermission.Deny("permission tool_name is required", true), nil
		}
		// 外部通道没有前端问答能力，AskUserQuestion 必须直接拒绝，
		// 否则 SDK 会卡在等待人工输入，导致整个会话超时。
		if toolName == "AskUserQuestion" {
			return sdkpermission.Deny("当前通道不支持交互式问题确认", true), nil
		}
		if request.autoApproveAll {
			return sdkpermission.Allow(permissionRequest.Input, nil), nil
		}
		if len(allowedByAgent) > 0 {
			if !toolpolicy.Contains(allowedByAgent, toolName) && !isManagedIngressTool(toolName) {
				return sdkpermission.Deny("当前 agent 未授权该工具", false), nil
			}
		}
		if len(approved) == 0 {
			return sdkpermission.Deny("当前通道未配置自动授权工具", false), nil
		}
		if !toolpolicy.Contains(approved, toolName) {
			return sdkpermission.Deny("当前通道不允许自动授权该工具", false), nil
		}
		return sdkpermission.Allow(permissionRequest.Input, nil), nil
	}
}

func isManagedIngressTool(toolName string) bool {
	return toolpolicy.Contains(defaultScheduledTaskApprovedTools, toolName) ||
		toolpolicy.IsManagedGoalTool(toolName) ||
		toolpolicy.Contains(defaultManagedSupportTools, toolName)
}
