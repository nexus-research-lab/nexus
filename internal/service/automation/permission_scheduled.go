// INPUT: 绑定 job/run/revision 的 SDK 工具权限请求与当前 Agent 默认工具设置。
// OUTPUT: 立即放行、硬拒绝，或持久化审批/重连/补充输入请求后暂停 logical run。
// POS: 定时任务 runtime 权限入口；session 仅作为请求回链字段，不作为授权来源。
package automation

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

const (
	automationPermissionRequiredCode   sdkpermission.ErrorCode = "automation_permission_required"
	automationReauthRequiredCode       sdkpermission.ErrorCode = "automation_connector_reauth_required"
	automationInputRequiredCode        sdkpermission.ErrorCode = "automation_input_required"
	automationPermissionPublishTimeout                         = 30 * time.Second
)

type scheduledPermissionScope struct {
	Job           automationdomain.ScheduledTask
	RunID         string
	SessionKey    string
	RoundID       string
	ResumeAttempt *permissionResumeAttempt
}

type permissionResumeAttempt struct {
	toolName      string
	resourceScope string

	mu        sync.Mutex
	attempted bool
	allowed   bool
}

func newPermissionResumeAttempt(request *automationdomain.AutomationPermissionRequest) *permissionResumeAttempt {
	if request == nil {
		return nil
	}
	toolName := strings.TrimSpace(request.Capability.ToolName)
	if toolName == "" {
		return nil
	}
	return &permissionResumeAttempt{
		toolName:      toolName,
		resourceScope: strings.TrimSpace(request.Capability.ResourceScope),
	}
}

func (a *permissionResumeAttempt) observe(request sdkpermission.Request, decision sdkpermission.Decision, err error) {
	if a == nil || strings.TrimSpace(request.ToolName) != a.toolName {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.attempted = true
	if err == nil && decision.Behavior == sdkpermission.BehaviorAllow {
		a.allowed = true
	}
}

func (a *permissionResumeAttempt) validationError() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.attempted {
		return fmt.Errorf("权限批准后的续跑没有重新调用已授权工具 %s", a.toolName)
	}
	if !a.allowed {
		return fmt.Errorf("权限批准后的续跑重新请求了工具 %s，但没有获得任务权限放行", a.toolName)
	}
	return nil
}

func (s *Service) scheduledTaskPermissionHandler(ctx context.Context, scope scheduledPermissionScope) sdkpermission.Handler {
	job := scope.Job
	if strings.TrimSpace(job.JobID) == "" || strings.TrimSpace(scope.RunID) == "" {
		options := protocol.Options{}
		if s.agents != nil && strings.TrimSpace(job.AgentID) != "" {
			if agentValue, err := s.requireAgent(ctx, job.AgentID); err == nil && agentValue != nil {
				options = agentValue.Options
			}
		}
		return scheduledTaskPermissionHandlerForOptions(options, s.runtimeImagegenDefaultEnabled(ctx))
	}
	return func(requestCtx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		decision, err := s.decideScheduledTaskPermission(requestCtx, scope, request)
		scope.ResumeAttempt.observe(request, decision, err)
		return decision, err
	}
}

func (s *Service) decideScheduledTaskPermission(
	ctx context.Context,
	scope scheduledPermissionScope,
	request sdkpermission.Request,
) (sdkpermission.Decision, error) {
	toolName := strings.TrimSpace(request.ToolName)
	if toolName == "" {
		return sdkpermission.Deny("定时任务后台运行收到空工具授权请求", true), nil
	}
	currentJob, err := s.repository.GetScheduledTask(ctx, scope.Job.OwnerUserID, scope.Job.JobID)
	if err != nil {
		return sdkpermission.Deny("读取定时任务授权状态失败", true), err
	}
	if currentJob == nil || currentJob.PermissionPolicy.Revision != scope.Job.PermissionPolicy.Revision {
		return sdkpermission.Deny("任务配置已在本次运行期间改变，请重新运行任务", true), nil
	}
	activeRun, runErr := s.repository.GetRun(ctx, currentJob.OwnerUserID, currentJob.JobID, scope.RunID)
	if runErr != nil || activeRun == nil {
		return sdkpermission.Deny("定时任务运行已经结束，忽略迟到的权限请求", true), nil
	}
	switch strings.TrimSpace(activeRun.Status) {
	case automationdomain.RunStatusPending, automationdomain.RunStatusRunning, automationdomain.RunStatusQueuedToMain:
	default:
		return sdkpermission.Deny("定时任务运行已经结束，忽略迟到的权限请求", true), nil
	}
	if blockState := strings.TrimSpace(activeRun.BlockState); blockState != "" {
		// 第一个交互请求落库后，旧物理 attempt 的精确中断需要异步完成。
		// 在这段窗口内拒绝该 attempt 的所有后续工具，避免模型把审批阻塞
		// 当成普通 tool_result 后继续调用其他已授权能力或产生副作用。
		s.stopBlockedPhysicalAttempt(scope.Job, scope.RunID, scope.SessionKey, scope.RoundID)
		reason := "定时任务正在等待用户处理，已停止本次执行尝试"
		if activeRun.ErrorMessage != nil && strings.TrimSpace(*activeRun.ErrorMessage) != "" {
			reason = strings.TrimSpace(*activeRun.ErrorMessage)
		}
		return sdkpermission.DenyWithErrorCode(
			reason,
			scheduledPermissionErrorCodeForBlockState(blockState),
			true,
		), nil
	}
	capability := buildPermissionCapability(request)
	if toolpolicy.MatchesItem(toolName, "AskUserQuestion") {
		return s.blockScheduledPermissionRequest(
			ctx,
			scope,
			request,
			capability,
			automationdomain.PermissionRequestKindHumanInput,
			automationdomain.TaskPermissionStateAwaitingInput,
			automationdomain.RunBlockStateAwaitingInput,
			"定时任务需要补充执行信息",
			"后台任务不能依赖临时会话问答；请编辑任务，把必要信息写入任务配置后重新运行。",
			automationInputRequiredCode,
		)
	}
	allowed, hardDenied, err := s.taskPolicyAllowsCapability(ctx, *currentJob, capability)
	if err != nil {
		return sdkpermission.Deny("检查任务授权失败", true), err
	}
	if hardDenied {
		return sdkpermission.Deny(
			fmt.Sprintf("当前 Agent 已明确禁用工具 %s，任务级审批不能绕过该限制", toolName),
			true,
		), nil
	}
	if !allowed {
		allowed, err = s.repository.HasApprovedRunPermission(
			ctx,
			currentJob.OwnerUserID,
			currentJob.JobID,
			scope.RunID,
			currentJob.PermissionPolicy.Revision,
			capability.InputFingerprint,
		)
		if err != nil {
			return sdkpermission.Deny("检查本次运行授权失败", true), err
		}
	}
	if !allowed {
		defaultTitle, defaultDescription := scheduledPermissionRequestCopy(capability)
		return s.blockScheduledPermissionRequest(
			ctx,
			scope,
			request,
			capability,
			automationdomain.PermissionRequestKindTool,
			automationdomain.TaskPermissionStateAwaitingApproval,
			automationdomain.RunBlockStateAwaitingApproval,
			firstNonEmpty(strings.TrimSpace(request.Title), strings.TrimSpace(request.DisplayName), defaultTitle),
			firstNonEmpty(strings.TrimSpace(request.Description), defaultDescription),
			automationPermissionRequiredCode,
		)
	}
	if capability.ConnectorID != "" && s.connectors != nil {
		connection, connectionErr := s.connectors.LoadActiveConnection(ctx, currentJob.OwnerUserID, capability.ConnectorID)
		if connectionErr != nil || connection == nil {
			return s.blockScheduledPermissionRequest(
				ctx,
				scope,
				request,
				capability,
				automationdomain.PermissionRequestKindConnectorReauth,
				automationdomain.TaskPermissionStateAwaitingReauth,
				automationdomain.RunBlockStateAwaitingReauth,
				"连接器需要重新连接",
				fmt.Sprintf("%s 的任务授权仍然有效，但连接凭证不可用；重新连接后可继续同一次运行。", capability.ConnectorID),
				automationReauthRequiredCode,
			)
		}
	}
	if capability.Effect != automationdomain.PermissionEffectRead {
		if err = s.repository.MarkRunEffectStarted(ctx, currentJob.OwnerUserID, scope.RunID); err != nil {
			return sdkpermission.Deny("无法记录任务副作用边界，已停止执行", true), err
		}
	}
	return sdkpermission.Allow(request.Input, nil), nil
}

func scheduledPermissionErrorCodeForBlockState(blockState string) sdkpermission.ErrorCode {
	switch strings.TrimSpace(blockState) {
	case automationdomain.RunBlockStateAwaitingReauth:
		return automationReauthRequiredCode
	case automationdomain.RunBlockStateAwaitingInput:
		return automationInputRequiredCode
	default:
		return automationPermissionRequiredCode
	}
}

func scheduledPermissionRequestCopy(capability automationdomain.PermissionCapability) (string, string) {
	target := strings.TrimSpace(capability.ToolName)
	if capability.ConnectorID == "feishu-docx" {
		target = "飞书文档"
	}
	if target == "" {
		target = "外部能力"
	}
	switch capability.Effect {
	case automationdomain.PermissionEffectRead:
		return target + "读取需要确认", "任务需要读取" + target + "中的指定资源。确认后会继续本次运行。"
	case automationdomain.PermissionEffectWrite:
		return target + "修改需要确认", "任务需要修改" + target + "中的指定资源，可能产生外部副作用。请确认是否继续。"
	default:
		return target + "调用需要确认", "任务需要调用" + target + "。请确认是否允许本次运行继续。"
	}
}

func (s *Service) blockScheduledPermissionRequest(
	ctx context.Context,
	scope scheduledPermissionScope,
	request sdkpermission.Request,
	capability automationdomain.PermissionCapability,
	kind string,
	taskState string,
	runBlockState string,
	title string,
	description string,
	errorCode sdkpermission.ErrorCode,
) (sdkpermission.Decision, error) {
	run, err := s.repository.GetRun(ctx, scope.Job.OwnerUserID, scope.Job.JobID, scope.RunID)
	if err != nil || run == nil {
		return sdkpermission.Deny("无法建立定时任务审批请求", true), err
	}
	reason := strings.TrimSpace(description)
	pending, created, err := s.repository.CreatePermissionRequestAndBlockRun(
		ctx,
		automationstore.PermissionRequestCreateInput{
			Request: automationdomain.AutomationPermissionRequest{
				RequestID:          s.idFactory("permission"),
				OwnerUserID:        scope.Job.OwnerUserID,
				JobID:              scope.Job.JobID,
				RunID:              scope.RunID,
				PolicyRevision:     scope.Job.PermissionPolicy.Revision,
				Kind:               kind,
				Capability:         capability,
				InputSummary:       summarizePermissionInput(request.Input),
				Title:              truncatePermissionText(title, 255),
				Description:        truncatePermissionText(description, 1000),
				Reason:             truncatePermissionText(reason, 1000),
				SessionKey:         strings.TrimSpace(scope.SessionKey),
				DeliverySessionKey: automationPermissionRunRecipientSessionKey(scope.Job, *run),
				RoundID:            strings.TrimSpace(scope.RoundID),
				ToolUseID:          strings.TrimSpace(request.ToolUseID),
				ResumeSafe:         !run.EffectStarted,
			},
			TaskState:  taskState,
			BlockState: runBlockState,
		},
	)
	if err != nil {
		return sdkpermission.Deny("持久化定时任务审批请求失败", true), err
	}
	s.setJobPermissionState(scope.Job.JobID, taskState, pending.RequestID)
	s.pauseJobRuntimeForPermission(scope.Job, scope.RunID, taskState, &pending.Reason)
	// 权限阻塞必须结束当前物理 attempt。SDK 的“本次工具拒绝”不会保证
	// Agent 停止后续推理；异步精确中断可避免在 permission callback 内自锁。
	s.stopBlockedPhysicalAttempt(scope.Job, scope.RunID, scope.SessionKey, scope.RoundID)
	if created {
		s.publishScheduledPermissionRequest(ctx, scope, *pending)
	}
	return sdkpermission.DenyWithErrorCode(reason, errorCode, true), nil
}

func (s *Service) publishScheduledPermissionRequest(
	ctx context.Context,
	scope scheduledPermissionScope,
	pending automationdomain.AutomationPermissionRequest,
) {
	publishCtx, cancelPublish := context.WithTimeout(
		context.WithoutCancel(ctx),
		automationPermissionPublishTimeout,
	)
	defer cancelPublish()
	detail := map[string]any{
		"request_id":   pending.RequestID,
		"request_kind": pending.Kind,
		"tool_name":    pending.Capability.ToolName,
		"connector_id": pending.Capability.ConnectorID,
		"effect":       pending.Capability.Effect,
		"resume_safe":  pending.ResumeSafe,
	}
	s.recordTaskEvent(
		contextForJobOwner(publishCtx, scope.Job),
		automationdomain.TaskEventActionPermissionRequested,
		scope.Job,
		scope.RunID,
		detail,
	)
	s.notifyAutomationPermissionRequest(publishCtx, scope.Job, pending)
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
		if toolpolicy.MatchesItem(toolName, "AskUserQuestion") {
			return sdkpermission.Deny("后台 heartbeat 不支持交互式确认；请先把必要信息写入配置", true), nil
		}
		if toolpolicy.IsManagedRuntimeCommandTool(toolName) {
			if readOnlyAutomationCommandRequest(request) {
				return sdkpermission.Allow(request.Input, nil), nil
			}
			return sdkpermission.Deny("后台 scheduled run 只能读取 Automation contract/inspect，不能调用 runtime mutation 命令", true), nil
		}
		if toolpolicy.Contains(disallowedByAgent, toolName) {
			return sdkpermission.Deny(fmt.Sprintf("当前 Agent 已禁用工具 %s，后台运行不会自动授权", toolName), false), nil
		}
		if len(allowedByAgent) == 0 || !toolpolicy.Contains(allowedByAgent, toolName) {
			return sdkpermission.Deny(
				fmt.Sprintf("当前 Agent 未授权工具 %s；请先在 Agent 允许工具中配置该工具", toolName),
				false,
			), nil
		}
		return sdkpermission.Allow(request.Input, nil), nil
	}
}

func readOnlyAutomationCommandRequest(request sdkpermission.Request) bool {
	domain, domainOK := request.Input["domain"].(string)
	action, actionOK := request.Input["action"].(string)
	if !domainOK || !actionOK || strings.TrimSpace(domain) != command.DomainAutomation {
		return false
	}
	return strings.TrimSpace(action) == automationdomain.AutomationCommandActionContract ||
		strings.TrimSpace(action) == automationdomain.AutomationCommandActionInspect
}
