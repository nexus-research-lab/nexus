// INPUT: 已解析的逻辑会话目标、统一 Session 读模型、Automation run 身份与结果正文。
// OUTPUT: run_id 幂等的 Nexus assistant 投影、数据库 Session 的 workspace 物化及平台回执。
// POS: Automation 结果同时进入真实 Nexus/Room/IM Session 的一致性投影边界。
package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const automationDeliverySource = "automation_delivery"

func (r *Router) projectAutomationResult(
	ctx context.Context,
	agentID string,
	text string,
	target DeliveryTarget,
	delivery AutomationDeliveryContext,
) (DeliveryResult, error) {
	sessionKey := strings.TrimSpace(target.SessionKey)
	if sessionKey == "" {
		return DeliveryResult{}, errors.New("automation delivery target is missing session_key")
	}
	parsed, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return DeliveryResult{}, err
	}
	parsedSession := protocol.ParseSessionKey(parsed)
	if parsedSession.Kind == protocol.SessionKeyKindRoom {
		return r.projectAutomationRoomResult(ctx, agentID, text, target, delivery, parsedSession)
	}
	if parsedSession.Kind != protocol.SessionKeyKindAgent {
		return DeliveryResult{}, errors.New("automation delivery requires an agent or room session")
	}
	channel := r.sessionProjector(ctx, parsedSession.AgentID, target.Channel)
	if channel == nil {
		return DeliveryResult{}, errors.New("automation delivery session projector is not configured")
	}
	receipt, err := channel.projectAutomationAgentResult(ctx, parsedSession, parsed, text, delivery)
	return DeliveryResult{Target: target.Normalized(), Receipt: receipt}, err
}

func (r *Router) sessionProjector(ctx context.Context, agentID string, preferredChannel string) *sessionDeliveryChannel {
	ownerUserID := r.resolveDeliveryOwner(ctx, agentID)
	channelTypes := []string{normalizeChannelType(preferredChannel), ChannelTypeWebSocket, ChannelTypeInternal}
	seen := make(map[string]struct{}, len(channelTypes))
	for _, channelType := range channelTypes {
		if channelType != ChannelTypeWebSocket && channelType != ChannelTypeInternal {
			continue
		}
		if _, exists := seen[channelType]; exists {
			continue
		}
		seen[channelType] = struct{}{}
		channel := r.GetForOwner(ownerUserID, channelType)
		if channel == nil && ownerUserID != "" {
			channel = r.GetForOwner("", channelType)
		}
		if projector, ok := channel.(*sessionDeliveryChannel); ok {
			return projector
		}
	}
	return nil
}

func (r *Router) projectAutomationRoomResult(
	ctx context.Context,
	agentID string,
	text string,
	target DeliveryTarget,
	delivery AutomationDeliveryContext,
	parsed protocol.SessionKey,
) (DeliveryResult, error) {
	projector := r.sessionProjector(ctx, agentID, ChannelTypeWebSocket)
	if projector == nil {
		return DeliveryResult{}, errors.New("automation room projector is not configured")
	}
	receipt, err := projector.projectAutomationRoomResult(ctx, agentID, parsed, text, delivery)
	return DeliveryResult{Target: target.Normalized(), Receipt: receipt}, err
}

func (r *Router) attachAutomationExternalReceipt(
	ctx context.Context,
	agentID string,
	result DeliveryResult,
	delivery AutomationDeliveryContext,
) error {
	if result.Receipt == nil || strings.TrimSpace(result.Target.SessionKey) == "" {
		return nil
	}
	parsed := protocol.ParseSessionKey(result.Target.SessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		return nil
	}
	projector := r.sessionProjector(ctx, agentID, ChannelTypeWebSocket)
	if projector == nil {
		return errors.New("automation delivery receipt projector is not configured")
	}
	return projector.appendAutomationExternalReceipt(ctx, parsed, result.Target.SessionKey, delivery, result)
}

func (c *sessionDeliveryChannel) projectAutomationAgentResult(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionKey string,
	text string,
	delivery AutomationDeliveryContext,
) (*channelmessage.Receipt, error) {
	if c.agents == nil {
		return nil, errors.New("session delivery 缺少 agent 解析器")
	}
	agentValue, err := c.agents.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
	if workspacePath == "" {
		return nil, fmt.Errorf("delivery target agent has no workspace path: %s", parsed.AgentID)
	}
	ownerUserID, err := automationDeliveryOwner(ctx, agentValue)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sessionValue, err := c.ensureAutomationTargetSession(ctx, ownerUserID, workspacePath, parsed, sessionKey, now)
	if err != nil {
		return nil, err
	}

	roundID, assistantID, resultID := automationDeliveryMessageIDs(delivery.RunID)
	metadata := automationDeliveryMetadata(delivery)
	assistantMessage := protocol.Message{
		"message_id":  assistantID,
		"session_key": sessionKey,
		"agent_id":    parsed.AgentID,
		"round_id":    roundID,
		"session_id":  stringPointerValue(sessionValue.SessionID),
		"role":        "assistant",
		"timestamp":   now.UnixMilli(),
		"content":     []map[string]any{{"type": "text", "text": strings.TrimSpace(text)}},
		"stop_reason": "end_turn",
		"is_complete": true,
		"metadata":    metadata,
	}
	resultMessage := protocol.Message{
		"message_id":      resultID,
		"session_key":     sessionKey,
		"agent_id":        parsed.AgentID,
		"round_id":        roundID,
		"session_id":      stringPointerValue(sessionValue.SessionID),
		"parent_id":       assistantID,
		"role":            "result",
		"timestamp":       now.UnixMilli(),
		"subtype":         "success",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"usage":           map[string]any{},
		"total_cost_usd":  0.0,
		"result":          strings.TrimSpace(text),
		"is_error":        false,
		"metadata":        metadata,
	}
	history := c.history.ForOwner(ownerUserID)
	alreadyProjected, err := automationMessageExists(history, workspacePath, *sessionValue, assistantID)
	if err != nil {
		return nil, err
	}
	if !alreadyProjected {
		if err = history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, roundID, delivery.Instruction, now.UnixMilli(), workspacestore.RoundMarkerOptions{
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        automationDeliverySource,
			Metadata:       automationDeliveryStringMetadata(delivery),
		}); err != nil {
			return nil, err
		}
	}
	if err = history.AppendOverlayMessage(workspacePath, sessionKey, assistantMessage); err != nil {
		return nil, err
	}
	if err = history.AppendOverlayMessage(workspacePath, sessionKey, resultMessage); err != nil {
		return nil, err
	}
	if !alreadyProjected {
		if _, err = c.refreshAutomationSession(ownerUserID, workspacePath, *sessionValue, assistantMessage, 1); err != nil {
			return nil, err
		}
	}
	c.broadcastMessage(ctx, sessionKey, parsed.AgentID, message.ProjectResultMessage(assistantMessage, resultMessage))
	return automationProjectionReceipt(c.channelType, sessionKey, parsed.ThreadID, assistantID), nil
}

func (c *sessionDeliveryChannel) ensureAutomationTargetSession(
	ctx context.Context,
	ownerUserID string,
	workspacePath string,
	parsed protocol.SessionKey,
	sessionKey string,
	now time.Time,
) (*protocol.Session, error) {
	files := c.files.ForOwner(ownerUserID)
	sessionValue, foundPath, err := files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		return nil, err
	}
	if sessionValue != nil && strings.TrimSpace(foundPath) != "" {
		return sessionValue, nil
	}
	resolved, err := c.materializeDeliverySession(ctx, files, workspacePath, parsed, sessionKey)
	if err != nil {
		return nil, err
	}
	if resolved != nil {
		return resolved, nil
	}
	if protocol.NormalizeStoredChannelType(parsed.Channel) != protocol.SessionChannelInternalSegment ||
		strings.TrimSpace(parsed.Ref) != protocol.AutomationInboxSessionRef {
		return nil, fmt.Errorf(
			"%s delivery target session is not available: %s",
			protocol.NormalizeStoredChannelType(parsed.Channel),
			sessionKey,
		)
	}
	// 只有旧版明确的 automation-inbox 可以在投递时补建；新任务和其他目标必须
	// 在配置时绑定已经存在的真实 Session。
	return c.ensureSession(ctx, ownerUserID, workspacePath, parsed, sessionKey, now)
}

func (c *sessionDeliveryChannel) refreshAutomationSession(
	ownerUserID string,
	workspacePath string,
	sessionValue protocol.Session,
	messageValue protocol.Message,
	messageDelta int,
) (*protocol.Session, error) {
	sessionValue.MessageCount += messageDelta
	sessionValue.LastActivity = time.Now().UTC()
	if sessionID := strings.TrimSpace(stringValue(messageValue["session_id"])); sessionID != "" {
		sessionValue.SessionID = &sessionID
	}
	sessionValue.Status = "closed"
	sessionValue.IsActive = false
	return c.files.ForOwner(ownerUserID).PatchSessionRuntime(workspacePath, sessionValue)
}

func (c *sessionDeliveryChannel) appendAutomationExternalReceipt(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionKey string,
	delivery AutomationDeliveryContext,
	result DeliveryResult,
) error {
	agentValue, err := c.agents.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return err
	}
	ownerUserID, err := automationDeliveryOwner(ctx, agentValue)
	if err != nil {
		return err
	}
	_, assistantID, _ := automationDeliveryMessageIDs(delivery.RunID)
	roundID, _, _ := automationDeliveryMessageIDs(delivery.RunID)
	receipt := result.Receipt
	return c.history.ForOwner(ownerUserID).AppendExternalDeliveryReceipt(
		agentValue.WorkspacePath,
		sessionKey,
		workspacestore.ExternalDeliveryReceipt{
			RoundID:                  roundID,
			MessageID:                assistantID,
			Channel:                  receipt.Channel,
			Target:                   receipt.Target,
			ThreadID:                 receipt.ThreadID,
			PrimaryPlatformMessageID: receipt.PrimaryPlatformMessageID,
			PlatformMessageIDs:       append([]string(nil), receipt.PlatformMessageIDs...),
			Timestamp:                receipt.SentAt,
		},
	)
}

func automationDeliveryOwner(ctx context.Context, agentValue *protocol.Agent) (string, error) {
	if agentValue == nil {
		return "", errors.New("automation delivery agent is missing")
	}
	ownerUserID := strings.TrimSpace(agentValue.OwnerUserID)
	authOwnerUserID := authctx.OwnerUserID(ctx)
	if ownerUserID != "" && authOwnerUserID != "" && ownerUserID != authOwnerUserID {
		return "", fmt.Errorf("delivery target agent owner mismatch: agent=%s principal=%s", ownerUserID, authOwnerUserID)
	}
	if ownerUserID == "" {
		ownerUserID = authOwnerUserID
	}
	if ownerUserID == "" {
		return "", fmt.Errorf("delivery target agent has no owner: %s", agentValue.AgentID)
	}
	return ownerUserID, nil
}

func automationMessageExists(
	history *workspacestore.AgentHistoryStore,
	workspacePath string,
	sessionValue protocol.Session,
	messageID string,
) (bool, error) {
	rows, err := history.ReadMessages(workspacePath, sessionValue, nil)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if strings.TrimSpace(stringValue(row["message_id"])) == messageID {
			return true, nil
		}
	}
	return false, nil
}

func automationDeliveryMessageIDs(runID string) (string, string, string) {
	suffix := stableAutomationDeliveryID(runID)
	return "automation_delivery_round_" + suffix,
		"automation_delivery_assistant_" + suffix,
		"automation_delivery_result_" + suffix
}

func stableAutomationDeliveryID(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func automationDeliveryMetadata(delivery AutomationDeliveryContext) map[string]any {
	metadata := map[string]any{
		"source": automationDeliverySource,
		"job_id": delivery.JobID,
		"run_id": delivery.RunID,
	}
	if delivery.ProducerAgentID != "" {
		metadata["producer_agent_id"] = delivery.ProducerAgentID
	}
	if delivery.TaskName != "" {
		metadata["task_name"] = delivery.TaskName
	}
	if delivery.ExecutionSessionKey != "" {
		metadata["execution_session_key"] = delivery.ExecutionSessionKey
	}
	if delivery.ExecutionRoundID != "" {
		metadata["execution_round_id"] = delivery.ExecutionRoundID
	}
	return metadata
}

func automationDeliveryStringMetadata(delivery AutomationDeliveryContext) map[string]string {
	result := make(map[string]string)
	for key, value := range automationDeliveryMetadata(delivery) {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			result[key] = strings.TrimSpace(text)
		}
	}
	return result
}

func automationProjectionReceipt(channel string, target string, threadID string, assistantID string) *channelmessage.Receipt {
	return channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  channel,
		Target:   target,
		ThreadID: threadID,
		Parts:    []channelmessage.ReceiptPart{channelmessage.TextPart(assistantID)},
	})
}
