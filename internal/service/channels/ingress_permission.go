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
		// 外部 ingress 的内容并不等同于已认证的人类控制面请求。即使通道配置了
		// autoApproveAll 或把整个 nexus_automation server 放入 allowlist，也只能
		// 查询任务，不能创建、修改、删除、修复或立即运行持久化任务。
		if isScheduledTaskMutationTool(toolName) {
			return sdkpermission.Deny("外部通道只允许查询定时任务；持久化变更和立即执行必须在 Nexus 的可信 DM/Room 中发起", false), nil
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
