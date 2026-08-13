// INPUT: active-paired IM 私聊的 runtime permission 事件与显式斜杠决定。
// OUTPUT: 精确回投当前 session 的权限通知，或对同一 pending request 的一次性/持续允许与拒绝。
// POS: 普通 Agent 工具权限在外部 IM transport 上的呈现和命令适配层。
package channels

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

const runtimePermissionRequestIDPrefix = "perm_"

type runtimePermissionIMSender struct {
	ownerUserID string
	agentID     string
	sessionKey  string
	router      *Router
	control     *ControlService
}

type runtimePermissionIMCommand struct {
	name      protocol.IMPermissionCommand
	requestID string
	malformed bool
}

func (s *IngressService) permissionCommandAmbiguity(
	ctx context.Context,
	request IngressCommandRequest,
) (*IngressCommandResult, error) {
	command, ok := parseRuntimePermissionIMCommand(request.Content)
	if !ok || command.malformed || command.requestID != "" {
		return nil, nil
	}
	runtimePending := 0
	if s != nil && s.permission != nil {
		runtimePending = s.permission.CountSessionPermissionRequests(request.SessionKey, "")
	}
	otherPending := 0
	if inspector, ok := s.commands.(IngressPermissionCommandInspector); ok {
		count, err := inspector.CountPendingIngressPermissionRequests(ctx, request)
		if err != nil {
			return nil, err
		}
		otherPending = count
	}
	if runtimePending+otherPending <= 1 {
		return nil, nil
	}
	return &IngressCommandResult{
		Handled: true,
		Reply:   "【Nexus 权限确认】\n当前会话有多个待确认请求，为避免误批，请在 Nexus 中逐项处理。",
	}, nil
}

func (s *IngressService) bindRuntimePermissionSession(request normalizedIngressRequest) {
	if s == nil || s.permission == nil || s.router == nil || s.control == nil ||
		!request.trustedExternalInteractive {
		return
	}
	sender := &runtimePermissionIMSender{
		ownerUserID: request.ownerUserID,
		agentID:     request.agentID,
		sessionKey:  request.sessionKey,
		router:      s.router,
		control:     s.control,
	}
	if s.permission.IsBound(request.sessionKey, sender) {
		return
	}
	s.permission.BindSession(request.sessionKey, sender)
}

func (s *IngressService) handleRuntimePermissionCommand(
	ctx context.Context,
	request normalizedIngressRequest,
) *IngressCommandResult {
	if s == nil || s.permission == nil || !request.trustedExternalInteractive {
		return nil
	}
	command, ok := parseRuntimePermissionIMCommand(request.content)
	if !ok {
		return nil
	}
	if command.malformed {
		return &IngressCommandResult{
			Handled: true,
			Reply:   "【Nexus 权限确认】\n命令格式不正确，请直接发送 /y、/a 或 /d。",
		}
	}
	decision := sdkpermission.BehaviorAllow
	persist := command.name == protocol.IMPermissionCommandAllowAlways
	if command.name == protocol.IMPermissionCommandDeny {
		decision = sdkpermission.BehaviorDeny
	}
	resolution := s.permission.ResolveSessionPermissionRequest(
		contextWithIngressOwner(ctx, request.ownerUserID),
		request.sessionKey,
		command.requestID,
		decision,
		persist,
	)
	if !resolution.Found {
		if command.requestID == "" {
			if resolution.MatchingRequests > 1 {
				return &IngressCommandResult{
					Handled: true,
					Reply:   "【Nexus 权限确认】\n当前会话有多个待确认请求，为避免误批，请在 Nexus 中逐项处理。",
				}
			}
			// 让 Automation 审批链继续尝试处理同一个无 ID 命令。
			return nil
		}
		if strings.HasPrefix(command.requestID, runtimePermissionRequestIDPrefix) {
			return &IngressCommandResult{
				Handled: true,
				Reply:   runtimePermissionStaleReply(),
			}
		}
		return nil
	}
	if persist && !resolution.PersistenceSupported {
		return &IngressCommandResult{
			Handled: true,
			Reply:   "【Nexus 权限确认】\n当前请求不支持持续允许；请使用 /y 仅允许本次，或使用 /d 拒绝。",
		}
	}
	if !resolution.Resolved {
		return &IngressCommandResult{
			Handled: true,
			Reply:   runtimePermissionStaleReply(),
		}
	}
	return &IngressCommandResult{
		Handled: true,
		Reply:   runtimePermissionDecisionReply(command, resolution),
	}
}

func parseRuntimePermissionIMCommand(content string) (runtimePermissionIMCommand, bool) {
	content = strings.TrimSpace(content)
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return runtimePermissionIMCommand{}, false
	}
	name, ok := protocol.ParseIMPermissionSlashName(fields[0])
	if !ok || name == protocol.IMPermissionCommandRetry {
		return runtimePermissionIMCommand{}, false
	}
	if len(fields) > 2 {
		return runtimePermissionIMCommand{name: name, malformed: true}, true
	}
	requestID := ""
	if len(fields) == 2 {
		requestID = strings.TrimSpace(fields[1])
	}
	return runtimePermissionIMCommand{name: name, requestID: requestID}, true
}

func runtimePermissionDecisionReply(
	command runtimePermissionIMCommand,
	resolution permissionctx.SessionPermissionResolution,
) string {
	prefix := "【Nexus 权限确认】\n"
	switch command.name {
	case protocol.IMPermissionCommandDeny:
		return prefix + "已拒绝。"
	case protocol.IMPermissionCommandAllowAlways:
		if resolution.Persisted {
			return prefix + "已按当前 runtime 的权限建议持续允许，并继续执行。"
		}
	}
	return prefix + "已允许本次，并继续执行。"
}

func runtimePermissionStaleReply() string {
	return "【Nexus 权限确认】\n当前请求已处理或失效，不会重复执行。"
}

func (s *runtimePermissionIMSender) Key() string {
	return "runtime-permission-im:" + strings.TrimSpace(s.sessionKey)
}

func (s *runtimePermissionIMSender) IsClosed() bool {
	return s == nil || s.router == nil || s.control == nil || strings.TrimSpace(s.sessionKey) == ""
}

func (s *runtimePermissionIMSender) SendEvent(
	ctx context.Context,
	event protocol.EventMessage,
) error {
	if s.IsClosed() || event.EventType != protocol.EventTypePermissionRequest {
		return nil
	}
	ownerCtx := contextWithIngressOwner(ctx, s.ownerUserID)
	if err := s.control.ValidateExternalSessionGrant(
		ownerCtx,
		s.ownerUserID,
		s.agentID,
		s.sessionKey,
	); err != nil {
		return err
	}
	text, ok := runtimePermissionIMNotice(event)
	if !ok {
		return nil
	}
	_, err := s.router.DeliverMessage(ownerCtx, s.agentID, text, DeliveryTarget{
		Mode:       DeliveryModeLast,
		SessionKey: s.sessionKey,
	})
	return err
}

func runtimePermissionIMNotice(event protocol.EventMessage) (string, bool) {
	requestID := runtimePermissionEventString(event.Data, "request_id")
	toolName := runtimePermissionEventString(event.Data, "tool_name")
	if requestID == "" || toolName == "" ||
		runtimePermissionEventString(event.Data, "interaction_mode") == "question" {
		return "", false
	}
	riskLabel := runtimePermissionEventString(event.Data, "risk_label")
	summary := truncateRuntimePermissionSummary(
		runtimePermissionEventString(event.Data, "summary"),
		240,
	)
	requestLine := "智能体请求使用：" + toolName
	if riskLabel != "" {
		requestLine += "（" + riskLabel + "）"
	}
	lines := []string{
		"【Nexus 权限确认】",
		requestLine,
	}
	if summary != "" && summary != toolName {
		lines = append(lines, "目标："+summary)
	}
	lines = append(lines, "/y：允许本次")
	if runtimePermissionEventHasSuggestions(event.Data["suggestions"]) {
		lines = append(lines, "/a：持续允许")
	}
	lines = append(lines, "/d：拒绝")
	return strings.Join(lines, "\n"), true
}

func runtimePermissionEventString(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return strings.TrimSpace(value)
}

func runtimePermissionEventHasSuggestions(value any) bool {
	switch items := value.(type) {
	case []any:
		return len(items) > 0
	case []map[string]any:
		return len(items) > 0
	default:
		return false
	}
}

func truncateRuntimePermissionSummary(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if limit <= 0 || len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}
