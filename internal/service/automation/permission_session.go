// INPUT: owner-scoped Automation 持久权限请求、任务结果接收 Session 与 WebSocket 决策。
// OUTPUT: 可重放的 DM/Room permission 事件，以及经 Session 身份复验的持久审批结果。
// POS: Automation 权限事实到 Nexus 会话交互面的唯一投影；不把接收 Session 当授权所有者。
package automation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type automationPermissionSessionRoute struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	AgentID        string
}

// ListSessionPermissionEvents 返回结果接收 Session 当前可操作的持久审批事件。
// WebSocket 每次 bind 都调用它，因此浏览器刷新不会丢失后台任务的确认面。
func (s *Service) ListSessionPermissionEvents(
	ctx context.Context,
	sessionKey string,
) ([]protocol.EventMessage, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		return nil, errors.New("automation permission replay requires an owner scope")
	}
	sessionKey = strings.TrimSpace(sessionKey)
	requests, err := s.repository.ListPermissionRequests(
		ctx,
		ownerUserID,
		automationdomain.PermissionRequestStatusPending,
		"",
	)
	if err != nil {
		return nil, err
	}
	events := make([]protocol.EventMessage, 0, len(requests))
	for _, request := range requests {
		job, loadErr := s.repository.GetScheduledTask(ctx, ownerUserID, request.JobID)
		if loadErr != nil {
			return nil, loadErr
		}
		if !currentAutomationPermissionRequest(job, request) ||
			automationPermissionRequestRecipientSessionKey(request) != sessionKey {
			continue
		}
		event, visible, eventErr := s.automationPermissionSessionEvent(ctx, *job, request)
		if eventErr != nil {
			s.loggerFor(ctx).Warn("定时任务权限请求无法恢复到结果接收 Session",
				"job_id", request.JobID,
				"request_id", request.RequestID,
				"session_key", sessionKey,
				"err", eventErr,
			)
			continue
		}
		if visible {
			events = append(events, event)
		}
	}
	slices.Reverse(events)
	return events, nil
}

// ResolveSessionPermissionResponse 把 DM/Room Composer 的通用 allow/deny
// 响应收窄为 Automation 持久决策；只有请求冻结的结果接收 Session 可以命中。
func (s *Service) ResolveSessionPermissionResponse(
	ctx context.Context,
	sessionKey string,
	response map[string]any,
) (bool, error) {
	if err := s.ensureReady(ctx); err != nil {
		return false, err
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		return false, errors.New("automation permission decision requires an owner scope")
	}
	requestID := automationPermissionResponseString(response, "request_id")
	if requestID == "" {
		return false, nil
	}
	request, err := s.repository.GetPermissionRequest(ctx, ownerUserID, requestID)
	if errors.Is(err, automationdomain.ErrPermissionRequestNotFound) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, request.JobID)
	if err != nil {
		return true, err
	}
	if job == nil || automationPermissionRequestRecipientSessionKey(*request) != strings.TrimSpace(sessionKey) {
		return false, nil
	}
	if !currentAutomationPermissionRequest(job, *request) {
		return true, automationdomain.ErrPermissionRequestStale
	}
	_, visible, routeErr := s.automationPermissionSessionEvent(ctx, *job, *request)
	if routeErr != nil {
		return true, routeErr
	}
	if !visible {
		return false, nil
	}
	decision, decisionErr := automationPermissionSessionDecision(response)
	if decisionErr != nil {
		return true, decisionErr
	}
	_, err = s.ResolvePermissionRequest(ctx, request.RequestID, automationdomain.PermissionDecisionInput{
		Decision:       decision,
		JobID:          request.JobID,
		RunID:          request.RunID,
		PolicyRevision: request.PolicyRevision,
	})
	return true, err
}

// PendingPermissionRequestIDsForRoom 返回 Room 订阅恢复时需要合并的
// Automation 请求身份；conversationID 为空时覆盖该 Room 的全部会话。
func (s *Service) PendingPermissionRequestIDsForRoom(
	ctx context.Context,
	roomID string,
	conversationID string,
) ([]string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped || strings.TrimSpace(ownerUserID) == "" {
		return nil, errors.New("automation room permission snapshot requires an owner scope")
	}
	requests, err := s.repository.ListPermissionRequests(
		ctx,
		ownerUserID,
		automationdomain.PermissionRequestStatusPending,
		"",
	)
	if err != nil {
		return nil, err
	}
	roomID = strings.TrimSpace(roomID)
	conversationID = strings.TrimSpace(conversationID)
	requestIDs := make([]string, 0, len(requests))
	for _, request := range requests {
		job, loadErr := s.repository.GetScheduledTask(ctx, ownerUserID, request.JobID)
		if loadErr != nil {
			return nil, loadErr
		}
		if !currentAutomationPermissionRequest(job, request) {
			continue
		}
		event, visible, eventErr := s.automationPermissionSessionEvent(ctx, *job, request)
		if eventErr != nil || !visible || event.RoomID != roomID ||
			(conversationID != "" && event.ConversationID != conversationID) {
			continue
		}
		requestIDs = append(requestIDs, request.RequestID)
	}
	slices.Reverse(requestIDs)
	return requestIDs, nil
}

func (s *Service) notifyAutomationPermissionSessionRequest(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	request automationdomain.AutomationPermissionRequest,
) {
	if s.permissionNotifier == nil {
		return
	}
	event, visible, err := s.automationPermissionSessionEvent(
		contextForJobOwner(ctx, job),
		job,
		request,
	)
	if err != nil {
		s.loggerFor(ctx).Warn("定时任务权限请求无法投影到结果接收 Session",
			"job_id", job.JobID,
			"request_id", request.RequestID,
			"session_key", request.DeliverySessionKey,
			"err", err,
		)
		return
	}
	if visible {
		s.permissionNotifier.NotifyAutomationPermissionEvent(ctx, event)
	}
}

func (s *Service) notifyAutomationPermissionSessionResolution(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	request automationdomain.AutomationPermissionRequest,
) {
	if s.permissionNotifier == nil || !automationPermissionSessionKind(request.Kind) {
		return
	}
	route, internal, err := s.resolveAutomationPermissionSessionRoute(
		contextForJobOwner(ctx, job),
		job,
		request,
	)
	if err != nil || !internal {
		return
	}
	event := protocol.NewPermissionRequestResolvedEvent(
		route.SessionKey,
		request.RequestID,
		request.Status,
	)
	event.RoomID = route.RoomID
	event.ConversationID = route.ConversationID
	event.AgentID = route.AgentID
	s.permissionNotifier.NotifyAutomationPermissionEvent(ctx, event)
}

func (s *Service) automationPermissionSessionEvent(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	request automationdomain.AutomationPermissionRequest,
) (protocol.EventMessage, bool, error) {
	if !automationPermissionSessionKind(request.Kind) {
		return protocol.EventMessage{}, false, nil
	}
	route, internal, err := s.resolveAutomationPermissionSessionRoute(ctx, job, request)
	if err != nil || !internal {
		return protocol.EventMessage{}, internal, err
	}
	riskLevel, riskLabel := automationPermissionSessionRisk(request.Capability.Effect)
	taskName := firstNonEmpty(job.Name, job.JobID, "未命名任务")
	summary := strings.TrimSpace(request.Description)
	if summary == "" {
		summary = strings.TrimSpace(request.Title)
	}
	if summary == "" {
		summary = "任务需要额外权限，确认后会继续本次运行。"
	}
	event := protocol.NewEvent(protocol.EventTypePermissionRequest, map[string]any{
		"request_id":                 request.RequestID,
		"tool_use_id":                strings.TrimSpace(request.ToolUseID),
		"tool_name":                  automationPermissionSessionToolName(request),
		"tool_input":                 request.InputSummary,
		"interaction_mode":           "permission",
		"risk_level":                 riskLevel,
		"risk_label":                 riskLabel,
		"summary":                    fmt.Sprintf("定时任务「%s」：%s", taskName, summary),
		"request_source":             "automation",
		"automation_job_id":          request.JobID,
		"automation_run_id":          request.RunID,
		"automation_policy_revision": request.PolicyRevision,
		"automation_request_kind":    request.Kind,
		"automation_task_name":       taskName,
		"automation_allow_task":      true,
		"producer_agent_id":          strings.TrimSpace(job.AgentID),
	})
	event.SessionKey = route.SessionKey
	event.RoomID = route.RoomID
	event.ConversationID = route.ConversationID
	event.AgentID = route.AgentID
	return event, true, nil
}

func (s *Service) resolveAutomationPermissionSessionRoute(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	request automationdomain.AutomationPermissionRequest,
) (automationPermissionSessionRoute, bool, error) {
	sessionKey := automationPermissionRequestRecipientSessionKey(request)
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured {
		return automationPermissionSessionRoute{}, false, nil
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		if s.room == nil {
			return automationPermissionSessionRoute{}, true, errors.New("automation permission Room resolver is not configured")
		}
		contextValue, err := s.room.GetConversationContext(ctx, parsed.ConversationID)
		if err != nil {
			return automationPermissionSessionRoute{}, true, err
		}
		if contextValue == nil ||
			strings.TrimSpace(contextValue.Conversation.ID) != strings.TrimSpace(parsed.ConversationID) {
			return automationPermissionSessionRoute{}, true, automationdomain.ErrTaskDeliverySessionUnavailable
		}
		return automationPermissionSessionRoute{
			SessionKey:     sessionKey,
			RoomID:         strings.TrimSpace(contextValue.Room.ID),
			ConversationID: strings.TrimSpace(contextValue.Conversation.ID),
			AgentID:        strings.TrimSpace(job.AgentID),
		}, true, nil
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return automationPermissionSessionRoute{}, false, nil
	}
	if s.deliverySessions == nil {
		return automationPermissionSessionRoute{}, true, errors.New("automation permission Session resolver is not configured")
	}
	stored, err := s.deliverySessions.ResolveDeliverySession(ctx, sessionKey)
	if err != nil {
		return automationPermissionSessionRoute{}, true, err
	}
	if stored == nil || strings.TrimSpace(stored.SessionKey) != sessionKey ||
		strings.TrimSpace(stored.AgentID) != strings.TrimSpace(parsed.AgentID) {
		return automationPermissionSessionRoute{}, true, automationdomain.ErrTaskDeliverySessionUnavailable
	}
	if externalIMChannel(parsed.Channel) &&
		(stored.ExternalIdentity == nil || !stored.ExternalIdentity.CurrentPairing) {
		return automationPermissionSessionRoute{}, true, automationdomain.ErrTaskDeliverySessionUnavailable
	}
	return automationPermissionSessionRoute{
		SessionKey:     sessionKey,
		RoomID:         optionalAutomationSessionString(stored.RoomID),
		ConversationID: optionalAutomationSessionString(stored.ConversationID),
		AgentID:        strings.TrimSpace(stored.AgentID),
	}, true, nil
}

func currentAutomationPermissionRequest(
	job *automationdomain.ScheduledTask,
	request automationdomain.AutomationPermissionRequest,
) bool {
	return job != nil &&
		request.Status == automationdomain.PermissionRequestStatusPending &&
		strings.TrimSpace(job.PendingPermissionRequestID) == strings.TrimSpace(request.RequestID) &&
		job.PermissionPolicy.Revision == request.PolicyRevision
}

func automationPermissionSessionKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case automationdomain.PermissionRequestKindTool, automationdomain.PermissionRequestKindScript:
		return true
	default:
		return false
	}
}

func automationPermissionSessionDecision(response map[string]any) (string, error) {
	switch automationPermissionResponseString(response, "decision") {
	case "deny":
		return automationdomain.PermissionDecisionDeny, nil
	case "allow":
		if automationPermissionResponseString(response, "automation_scope") == "task" {
			return automationdomain.PermissionDecisionAllowTask, nil
		}
		return automationdomain.PermissionDecisionAllowOnce, nil
	default:
		return "", fmt.Errorf("%w: DM/Room response must be allow or deny", automationdomain.ErrPermissionDecisionInvalid)
	}
}

func automationPermissionSessionToolName(
	request automationdomain.AutomationPermissionRequest,
) string {
	if request.Kind == automationdomain.PermissionRequestKindScript {
		return "Bash"
	}
	return strings.TrimSpace(request.Capability.ToolName)
}

func automationPermissionSessionRisk(effect string) (string, string) {
	switch strings.TrimSpace(effect) {
	case automationdomain.PermissionEffectRead:
		return "low", "只读"
	case automationdomain.PermissionEffectWrite:
		return "medium", "写入"
	default:
		return "high", "执行"
	}
}

func automationPermissionResponseString(response map[string]any, key string) string {
	value, _ := response[key].(string)
	return strings.TrimSpace(value)
}

func optionalAutomationSessionString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
