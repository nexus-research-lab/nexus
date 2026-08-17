// INPUT: active-paired 外部 IM 私聊、持久审批请求与当前任务/授权快照。
// OUTPUT: /y、/a、/d 的幂等决策及当前 IM 回执；历史长命令仅作兼容别名。
// POS: 外部 IM 控制命令到 Automation 持久审批流水线的唯一适配层；不进入 Agent runtime。
package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

type permissionIMCommand struct {
	name      protocol.IMPermissionCommand
	requestID string
	bare      bool
	malformed bool
}

type permissionIMCommandContextKey struct{}

// CountPendingIngressPermissionRequests 只读统计当前可信 IM session 的 Automation
// pending 权限请求，供入口在执行无 ID 短命令前与普通 runtime 请求统一消歧。
func (s *Service) CountPendingIngressPermissionRequests(
	ctx context.Context,
	request channels.IngressCommandRequest,
) (int, error) {
	if err := s.ensureReady(ctx); err != nil {
		return 0, err
	}
	if strings.TrimSpace(request.OwnerUserID) == "" ||
		strings.TrimSpace(request.AgentID) == "" ||
		strings.TrimSpace(request.SessionKey) == "" ||
		!trustedPermissionIMCommandSession(request.SessionKey, request.AgentID) {
		return 0, nil
	}
	matches, err := s.matchingPermissionIMRequests(
		contextForOwner(ctx, request.OwnerUserID),
		request,
	)
	return len(matches), err
}

// HandleIngressCommand 消费当前可信 IM 会话中的 Automation 权限命令。
func (s *Service) HandleIngressCommand(
	ctx context.Context,
	request channels.IngressCommandRequest,
) (channels.IngressCommandResult, error) {
	command, recognized := parsePermissionIMCommand(request.Content)
	if !recognized {
		return channels.IngressCommandResult{}, nil
	}
	if err := s.ensureReady(ctx); err != nil {
		return channels.IngressCommandResult{}, err
	}
	ownerUserID := strings.TrimSpace(request.OwnerUserID)
	if ownerUserID == "" || strings.TrimSpace(request.AgentID) == "" || strings.TrimSpace(request.SessionKey) == "" {
		return channels.IngressCommandResult{Handled: true, Reply: permissionIMUnavailableText()}, nil
	}
	if !trustedPermissionIMCommandSession(request.SessionKey, request.AgentID) {
		// 普通“是/否”在群聊或非外部 Session 中仍属于对话内容；显式斜杠命令
		// 则由控制面消费并明确拒绝，避免它落入 Agent runtime 造成误导。
		if command.bare {
			return channels.IngressCommandResult{}, nil
		}
		return channels.IngressCommandResult{Handled: true, Reply: permissionIMDMOnlyText()}, nil
	}
	if command.malformed {
		return channels.IngressCommandResult{Handled: true, Reply: permissionIMCommandUsage()}, nil
	}
	ownerCtx := contextForOwner(ctx, ownerUserID)
	if command.bare {
		resolved, reply, err := s.resolveBarePermissionIMCommand(ownerCtx, request, command)
		if err != nil {
			return channels.IngressCommandResult{}, err
		}
		if !resolved {
			return channels.IngressCommandResult{}, nil
		}
		return channels.IngressCommandResult{Handled: true, Reply: reply}, nil
	}
	if command.requestID == "" {
		resolved, reply, err := s.resolveBarePermissionIMCommand(ownerCtx, request, command)
		if err != nil {
			return channels.IngressCommandResult{}, err
		}
		if !resolved {
			return channels.IngressCommandResult{
				Handled: true,
				Reply:   "【Nexus 定时任务权限】\n当前会话没有待确认的定时任务请求。",
			}, nil
		}
		return channels.IngressCommandResult{Handled: true, Reply: reply}, nil
	}
	reply, err := s.resolvePermissionIMCommand(ownerCtx, request, command)
	if err != nil {
		return channels.IngressCommandResult{}, err
	}
	return channels.IngressCommandResult{Handled: true, Reply: reply}, nil
}

func trustedPermissionIMCommandSession(sessionKey string, agentID string) bool {
	parsed := protocol.ParseSessionKey(strings.TrimSpace(sessionKey))
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		protocol.NormalizeSessionChatType(parsed.ChatType) != protocol.RoomTypeDM ||
		strings.TrimSpace(parsed.AgentID) != strings.TrimSpace(agentID) {
		return false
	}
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	return externalIMChannel(channel)
}

func parsePermissionIMCommand(content string) (permissionIMCommand, bool) {
	content = strings.TrimSpace(content)
	fields := strings.Fields(content)
	if len(fields) > 0 {
		if name, ok := protocol.ParseIMPermissionSlashName(fields[0]); ok {
			if len(fields) > 2 {
				return permissionIMCommand{name: name, malformed: true}, true
			}
			requestID := ""
			if len(fields) == 2 {
				requestID = strings.TrimSpace(fields[1])
			}
			return permissionIMCommand{name: name, requestID: requestID}, true
		}
	}
	switch strings.ToLower(content) {
	case "是", "确认", "同意", "允许", "yes", "y":
		return permissionIMCommand{name: protocol.IMPermissionCommandAllowOnce, bare: true}, true
	case "否", "拒绝", "不同意", "no", "n":
		return permissionIMCommand{name: protocol.IMPermissionCommandDeny, bare: true}, true
	default:
		return permissionIMCommand{}, false
	}
}

func (s *Service) resolveBarePermissionIMCommand(
	ctx context.Context,
	ingress channels.IngressCommandRequest,
	command permissionIMCommand,
) (bool, string, error) {
	matches, err := s.matchingPermissionIMRequests(ctx, ingress)
	if err != nil {
		return false, "", err
	}
	if len(matches) == 0 {
		// 普通“是/否”只有在当前 IM 会话确有待处理请求时才属于控制面。
		return false, "", nil
	}
	if len(matches) > 1 {
		return true, "【Nexus 定时任务权限】\n当前会话有多个待确认请求，为避免误批，请在 Nexus 中逐项处理。", nil
	}
	command.requestID = matches[0].RequestID
	command.bare = false
	reply, err := s.resolvePermissionIMCommand(ctx, ingress, command)
	return true, reply, err
}

func (s *Service) matchingPermissionIMRequests(
	ctx context.Context,
	ingress channels.IngressCommandRequest,
) ([]automationdomain.AutomationPermissionRequest, error) {
	requests, err := s.repository.ListPermissionRequests(
		ctx,
		strings.TrimSpace(ingress.OwnerUserID),
		automationdomain.PermissionRequestStatusPending,
		"",
	)
	if err != nil {
		return nil, err
	}
	matches := make([]automationdomain.AutomationPermissionRequest, 0, 1)
	for _, pending := range requests {
		job, loadErr := s.repository.GetScheduledTask(ctx, ingress.OwnerUserID, pending.JobID)
		if loadErr != nil {
			return nil, loadErr
		}
		if !permissionIMRequestMatchesIngress(pending, job, ingress) {
			continue
		}
		matches = append(matches, pending)
	}
	return matches, nil
}

func (s *Service) resolvePermissionIMCommand(
	ctx context.Context,
	ingress channels.IngressCommandRequest,
	command permissionIMCommand,
) (string, error) {
	request, err := s.repository.GetPermissionRequest(ctx, ingress.OwnerUserID, command.requestID)
	if err != nil {
		if errors.Is(err, automationdomain.ErrPermissionRequestNotFound) {
			return "【Nexus 定时任务权限】\n未找到这个权限请求；请使用最新通知中的完整命令。", nil
		}
		return "", err
	}
	job, err := s.repository.GetScheduledTask(ctx, ingress.OwnerUserID, request.JobID)
	if err != nil {
		return "", err
	}
	if !permissionIMRequestMatchesIngress(*request, job, ingress) {
		return "【Nexus 定时任务权限】\n这个请求不属于当前 IM 会话或当前 Agent，未执行任何操作。", nil
	}
	if err = s.validatePermissionIMPairing(ctx, ingress, *request, *job); err != nil {
		return "【Nexus 定时任务权限】\n当前 IM 配对授权已失效，请在 Nexus 中重新配对后再处理。", nil
	}
	if request.Status != automationdomain.PermissionRequestStatusPending {
		return fmt.Sprintf(
			"【Nexus 定时任务权限】\n当前请求已处理（%s），不会重复执行。",
			permissionIMRequestStatusText(*request),
		), nil
	}
	decision, decisionErr := permissionIMDecision(command.name, request.Kind)
	if decisionErr != nil {
		return "【Nexus 定时任务权限】\n" + decisionErr.Error(), nil
	}
	decisionCtx := context.WithValue(ctx, permissionIMCommandContextKey{}, true)
	result, err := s.ResolvePermissionRequest(decisionCtx, request.RequestID, automationdomain.PermissionDecisionInput{
		Decision:       decision,
		JobID:          request.JobID,
		RunID:          request.RunID,
		PolicyRevision: request.PolicyRevision,
	})
	if err != nil {
		return permissionIMDecisionErrorText(err), nil
	}
	return permissionIMDecisionResultText(decision, result), nil
}

func permissionIMRequestMatchesIngress(
	request automationdomain.AutomationPermissionRequest,
	job *automationdomain.ScheduledTask,
	ingress channels.IngressCommandRequest,
) bool {
	if job == nil ||
		strings.TrimSpace(job.AgentID) != strings.TrimSpace(ingress.AgentID) ||
		(request.Status == automationdomain.PermissionRequestStatusPending &&
			strings.TrimSpace(job.PendingPermissionRequestID) != strings.TrimSpace(request.RequestID)) {
		return false
	}
	deliverySessionKey := automationPermissionRequestRecipientSessionKey(request)
	return deliverySessionKey != "" && deliverySessionKey == strings.TrimSpace(ingress.SessionKey)
}

func (s *Service) validatePermissionIMPairing(
	ctx context.Context,
	ingress channels.IngressCommandRequest,
	request automationdomain.AutomationPermissionRequest,
	job automationdomain.ScheduledTask,
) error {
	if s.deliveryGrants == nil {
		return errors.New("automation IM delivery grant resolver is not configured")
	}
	return s.deliveryGrants.ValidateAutomationDeliveryGrant(
		ctx,
		ingress.OwnerUserID,
		job.AgentID,
		automationPermissionRequestRecipientSessionKey(request),
	)
}

func permissionIMDecision(command protocol.IMPermissionCommand, kind string) (string, error) {
	switch command {
	case protocol.IMPermissionCommandDeny:
		return automationdomain.PermissionDecisionDeny, nil
	case protocol.IMPermissionCommandAllowAlways:
		if kind == automationdomain.PermissionRequestKindTool || kind == automationdomain.PermissionRequestKindScript {
			return automationdomain.PermissionDecisionAllowTask, nil
		}
		return "", errors.New("这个请求不支持永久允许；请按通知完成处理，或使用 /d 拒绝")
	case protocol.IMPermissionCommandAllowOnce, protocol.IMPermissionCommandRetry:
		switch kind {
		case automationdomain.PermissionRequestKindTool, automationdomain.PermissionRequestKindScript:
			if command == protocol.IMPermissionCommandRetry {
				return "", errors.New("工具权限请求请使用 /y 或 /a")
			}
			return automationdomain.PermissionDecisionAllowOnce, nil
		case automationdomain.PermissionRequestKindConnectorReauth:
			return automationdomain.PermissionDecisionRetry, nil
		case automationdomain.PermissionRequestKindHumanInput:
			return "", errors.New("该任务缺少必要输入，请在 Nexus 中编辑任务后重试，或使用 /d 结束本次运行")
		default:
			return "", errors.New("这个请求类型不支持该命令")
		}
	default:
		return "", errors.New("未知权限命令")
	}
}

func permissionIMDecisionResultText(
	decision string,
	result *automationdomain.PermissionDecisionResult,
) string {
	prefix := "【Nexus 定时任务权限】\n"
	if decision == automationdomain.PermissionDecisionDeny {
		return prefix + "已拒绝，本次运行已停止。"
	}
	if result != nil && result.ResumeStarted {
		if decision == automationdomain.PermissionDecisionAllowTask {
			return prefix + "已设为此任务始终允许，并已继续运行。"
		}
		return prefix + "已批准，并已继续运行。"
	}
	if result != nil && result.Task.PermissionState == automationdomain.TaskPermissionStateReadyToRetry {
		return prefix + "已批准；为避免重复外部副作用，请在 Nexus 中显式确认重试。"
	}
	return prefix + "已处理。"
}

func permissionIMDecisionErrorText(err error) string {
	switch {
	case errors.Is(err, automationdomain.ErrPermissionRequestResolved):
		return "【Nexus 定时任务权限】\n该请求已经处理，不会重复执行。"
	case errors.Is(err, automationdomain.ErrPermissionRequestStale):
		return "【Nexus 定时任务权限】\n该请求已失效，请使用最新通知中的命令。"
	case errors.Is(err, automationdomain.ErrPermissionConnectorNotReady):
		return "【Nexus 定时任务权限】\n连接器仍未恢复，请先在 Nexus 中重新连接，再发送 /y。"
	case errors.Is(err, automationdomain.ErrPermissionDecisionInvalid):
		return "【Nexus 定时任务权限】\n该命令不适用于当前请求，请使用通知中列出的命令。"
	default:
		return "【Nexus 定时任务权限】\n处理失败，请在 Nexus 的定时任务页面重试。"
	}
}

func permissionIMRequestStatusText(request automationdomain.AutomationPermissionRequest) string {
	if strings.TrimSpace(request.Decision) != "" {
		return request.Decision
	}
	return firstNonEmpty(request.Status, "已结束")
}

func permissionIMCommandUsage() string {
	return "【Nexus 定时任务权限】\n命令格式不正确，请直接发送 /y、/a 或 /d。"
}

func permissionIMUnavailableText() string {
	return "【Nexus 定时任务权限】\n当前消息缺少可信的 IM 会话身份，未执行任何操作。"
}

func permissionIMDMOnlyText() string {
	return "【Nexus 定时任务权限】\n权限命令只接受与当前 Agent 精确配对的 IM 私聊；群聊或内部会话未执行任何操作。"
}
