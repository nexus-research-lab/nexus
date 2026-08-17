// INPUT: 外部 channel ingress 配对信任、Agent 工具配置与模型工具请求。
// OUTPUT: 普通外部来源的受限决策，或 active-paired 私聊的同 Agent 工具决策。
// POS: pairing transport 身份与 Agent 工具权限相交的 channel 入口边界。
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

// Legacy Automation MCP names remain permission-compatible only for resumed
// pre-migration transcripts. New runtimes do not mount these tools.
var defaultScheduledTaskReadTools = map[string]struct{}{
	"automation_query": {},
}

var scheduledTaskMutationTools = map[string]struct{}{
	"automation_update": {},
}

var defaultGoalApprovedTools = map[string]struct{}{
	"create_goal":               {},
	"get_goal":                  {},
	"retarget_goal":             {},
	"audit_objective_alignment": {},
	"update_goal":               {},
}

var defaultManagedSupportTools = map[string]struct{}{
	"Skill": {},
}

var defaultExternalApprovedTools = toolpolicy.MergeSets(defaultReadOnlyApprovedTools, defaultScheduledTaskReadTools, defaultGoalApprovedTools)

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
	if request.trustedExternalInteractive {
		return s.buildPairedDMPermissionHandler(agentValue, request)
	}
	allowedByAgent := toolpolicy.NormalizeSet(agentValue.Options.AllowedTools)
	deniedByAgent := toolpolicy.NormalizeSet(agentValue.Options.DisallowedTools)
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
		if toolpolicy.Contains(deniedByAgent, toolName) {
			return sdkpermission.Deny("当前 agent 已禁止该工具", false), nil
		}
		// 旧 transcript 即使重放 legacy Automation MCP tool name，也不能借外部
		// unpaired ingress 恢复 mutation；新命令的最终边界是 round-scoped broker。
		if isScheduledTaskMutationTool(toolName) {
			return sdkpermission.Deny("外部通道不能通过旧 Automation 工具名执行持久化变更", false), nil
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

func (s *IngressService) buildPairedDMPermissionHandler(
	agentValue *protocol.Agent,
	request normalizedIngressRequest,
) sdkpermission.Handler {
	allowedByAgent := toolpolicy.NormalizeSet(agentValue.Options.AllowedTools)
	deniedByAgent := toolpolicy.NormalizeSet(agentValue.Options.DisallowedTools)
	approved := toolpolicy.MergeSets(request.autoApproveTools, allowedByAgent)
	return func(ctx context.Context, permissionRequest sdkpermission.Request) (sdkpermission.Decision, error) {
		toolName := strings.TrimSpace(permissionRequest.ToolName)
		if toolName == "" {
			return sdkpermission.Deny("permission tool_name is required", true), nil
		}
		if toolName == "AskUserQuestion" {
			return sdkpermission.Deny("当前 IM 通道不支持结构化问题回答", true), nil
		}
		if toolpolicy.Contains(deniedByAgent, toolName) {
			return sdkpermission.Deny("当前 agent 已禁止该工具", false), nil
		}
		if request.autoApproveAll || toolpolicy.Contains(approved, toolName) {
			return sdkpermission.Allow(permissionRequest.Input, nil), nil
		}
		if s.permission == nil {
			return sdkpermission.Deny("当前 IM 权限确认通道不可用", false), nil
		}
		return s.permission.RequestPermission(ctx, request.sessionKey, permissionRequest)
	}
}

func isManagedIngressTool(toolName string) bool {
	return toolpolicy.Contains(defaultScheduledTaskReadTools, toolName) ||
		toolpolicy.IsManagedGoalTool(toolName) ||
		toolpolicy.Contains(defaultManagedSupportTools, toolName)
}

func isScheduledTaskMutationTool(toolName string) bool {
	return toolpolicy.Contains(scheduledTaskMutationTools, toolName)
}
