package automation

import (
	"context"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func (s *Service) scheduledTaskPermissionHandler(ctx context.Context, job automationdomain.ScheduledTask) sdkpermission.Handler {
	options := protocol.Options{}
	if s.agents != nil && strings.TrimSpace(job.AgentID) != "" {
		if agentValue, err := s.requireAgent(ctx, job.AgentID); err == nil && agentValue != nil {
			options = agentValue.Options
		}
	}
	return scheduledTaskPermissionHandlerForOptions(options, s.runtimeImagegenDefaultEnabled(ctx))
}

func scheduledTaskPermissionHandlerForOptions(options protocol.Options, imagegenDefaultEnabled bool) sdkpermission.Handler {
	options.AllowedTools = toolpolicy.WithManagedRuntimeAllowedTools(options.AllowedTools, imagegenDefaultEnabled)
	allowedByAgent := toolpolicy.NormalizeSet(options.AllowedTools)
	disallowedByAgent := toolpolicy.NormalizeSet(options.DisallowedTools)
	return func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		toolName := strings.TrimSpace(request.ToolName)
		if toolName == "" {
			return sdkpermission.Deny("定时任务后台运行收到空工具授权请求", false), nil
		}
		if toolName == "AskUserQuestion" {
			return sdkpermission.Deny("定时任务后台运行不支持交互式确认；请先把必要信息写入任务配置", true), nil
		}
		if toolpolicy.Contains(disallowedByAgent, toolName) {
			return sdkpermission.Deny(fmt.Sprintf("当前 Agent 已禁用工具 %s，定时任务不会在后台自动授权", toolName), false), nil
		}
		if len(allowedByAgent) == 0 || !toolpolicy.Contains(allowedByAgent, toolName) {
			return sdkpermission.Deny(
				fmt.Sprintf("当前 Agent 未授权工具 %s；请先在 Agent 允许工具中配置该工具，或把任务改为无需该工具", toolName),
				false,
			), nil
		}
		return sdkpermission.Allow(request.Input, nil), nil
	}
}
