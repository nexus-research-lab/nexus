// INPUT: Agent Session 投递目标、统一 Session 读模型与 owner-confined workspace。
// OUTPUT: 已存在或经精确身份校验后物化的 Session 投影，以及兼容历史收件箱的消息写入。
// POS: 普通 Nexus/IM Session 主动投递边界；不得从裸 key 合成新的用户会话。
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
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type agentWorkspaceResolver interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
	GetDefaultAgent(context.Context) (*protocol.Agent, error)
}

type sessionDeliveryChannel struct {
	channelType string
	agents      agentWorkspaceResolver
	permission  *permissionctx.Context
	sessions    sessionProjectionResolver
	files       *workspacestore.SessionFileStore
	history     *workspacestore.AgentHistoryStore
	roomHistory *workspacestore.RoomHistoryStore
	idFactory   func(string) string
}

func newSessionDeliveryChannel(
	channelType string,
	agents agentWorkspaceResolver,
	permission *permissionctx.Context,
	workspaceRoot string,
) *sessionDeliveryChannel {
	return &sessionDeliveryChannel{
		channelType: channelType,
		agents:      agents,
		permission:  permission,
		files:       workspacestore.NewSessionFileStore(workspaceRoot),
		history:     workspacestore.NewAgentHistoryStore(workspaceRoot),
		roomHistory: workspacestore.NewRoomHistoryStore(workspaceRoot),
		idFactory:   channelcontract.NewID,
	}
}

func (c *sessionDeliveryChannel) ChannelType() string {
	return c.channelType
}

func (c *sessionDeliveryChannel) Start(context.Context) error {
	return nil
}

func (c *sessionDeliveryChannel) Stop(context.Context) error {
	return nil
}

// SendDeliveryMessage 按 session_key 完成消息投递。
func (c *sessionDeliveryChannel) SendDeliveryMessage(ctx context.Context, target DeliveryTarget, text string) (DeliveryResult, error) {
	return c.SendAgentDeliveryMessage(ctx, "", target, text)
}

// SendAgentDeliveryMessage 按 session_key 完成消息投递，agentID 用于 room 公区消息归属。
func (c *sessionDeliveryChannel) SendAgentDeliveryMessage(
	ctx context.Context,
	agentID string,
	target DeliveryTarget,
	text string,
) (DeliveryResult, error) {
	normalized := target.Normalized()
	sessionKey := firstNonEmpty(target.SessionKey, target.To)
	sessionKey, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return DeliveryResult{}, err
	}

	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		receipt, err := c.sendRoomDeliveryText(ctx, strings.TrimSpace(agentID), parsed, sessionKey, text)
		return channelcontract.NewDeliveryResult(normalized, receipt), err
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return DeliveryResult{}, errors.New("shared room delivery 暂不支持")
	}
	receipt, err := c.sendAgentSessionDeliveryText(ctx, parsed, sessionKey, text)
	return channelcontract.NewDeliveryResult(normalized, receipt), err
}

// sendAgentSessionDeliveryText 追加 assistant 正文与内部 result overlay，
// 对外统一只广播挂载 result_summary 的 assistant。
func (c *sessionDeliveryChannel) sendAgentSessionDeliveryText(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionKey string,
	text string,
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
	ownerUserID := strings.TrimSpace(agentValue.OwnerUserID)
	authOwnerUserID := strings.TrimSpace(authctx.OwnerUserID(ctx))
	if ownerUserID != "" && authOwnerUserID != "" && ownerUserID != authOwnerUserID {
		return nil, fmt.Errorf(
			"delivery target agent owner mismatch: agent=%s principal=%s",
			ownerUserID,
			authOwnerUserID,
		)
	}
	if ownerUserID == "" {
		ownerUserID = authOwnerUserID
	}
	if ownerUserID == "" {
		return nil, fmt.Errorf("delivery target agent has no owner: %s", parsed.AgentID)
	}

	now := time.Now().UTC()
	sessionValue, err := c.ensureSession(ctx, ownerUserID, workspacePath, parsed, sessionKey, now)
	if err != nil {
		return nil, err
	}

	roundID := c.idFactory("delivery_round")
	assistantMessage := protocol.Message{
		"message_id":  c.idFactory("assistant"),
		"session_key": sessionKey,
		"agent_id":    parsed.AgentID,
		"round_id":    roundID,
		"session_id":  stringPointerValue(sessionValue.SessionID),
		"role":        "assistant",
		"timestamp":   now.UnixMilli(),
		"content": []map[string]any{
			{
				"type": "text",
				"text": strings.TrimSpace(text),
			},
		},
		"is_complete": true,
	}
	resultMessage := protocol.Message{
		"message_id":      c.idFactory("result"),
		"session_key":     sessionKey,
		"agent_id":        parsed.AgentID,
		"round_id":        roundID,
		"session_id":      stringPointerValue(sessionValue.SessionID),
		"parent_id":       assistantMessage["message_id"],
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
	}

	updated, err := c.persistMessage(ownerUserID, workspacePath, *sessionValue, assistantMessage)
	if err != nil {
		return nil, err
	}
	if _, err = c.persistMessage(ownerUserID, workspacePath, updated, resultMessage); err != nil {
		return nil, err
	}

	c.broadcastMessage(ctx, sessionKey, parsed.AgentID, message.ProjectResultMessage(assistantMessage, resultMessage))
	return channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  c.channelType,
		Target:   sessionKey,
		ThreadID: parsed.ThreadID,
		Parts: []channelmessage.ReceiptPart{
			channelmessage.TextPart(stringValue(assistantMessage["message_id"])),
		},
	}), nil
}

func (c *sessionDeliveryChannel) ensureSession(
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
	if c.channelType != ChannelTypeInternal ||
		protocol.NormalizeStoredChannelType(parsed.Channel) != protocol.SessionChannelInternalSegment ||
		strings.TrimSpace(parsed.Ref) != protocol.AutomationInboxSessionRef {
		return nil, fmt.Errorf("delivery target session is not available: %s", sessionKey)
	}

	// 普通内部/网页投递只接受已存在的真实 Session；仅为旧版明确的
	// automation-inbox 保留延迟创建兼容。
	session := protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      parsed.AgentID,
		ChannelType:  protocol.SessionChannelInternalSegment,
		ChatType:     protocol.NormalizeSessionChatType(parsed.ChatType),
		Status:       "closed",
		CreatedAt:    now,
		LastActivity: now,
		Title:        internalSessionTitle(parsed),
		Options: map[string]any{
			"created_by": "automation_delivery",
			"ref":        parsed.Ref,
		},
		IsActive: false,
	}
	created, err := files.UpsertSession(workspacePath, session)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return &session, nil
	}
	return created, nil
}

// materializeDeliverySession 是数据库 Room-backed Session 进入 workspace
// 投影的唯一入口；只有统一读模型返回精确匹配的 Session 才会写入。
func (c *sessionDeliveryChannel) materializeDeliverySession(
	ctx context.Context,
	files *workspacestore.SessionFileStore,
	workspacePath string,
	parsed protocol.SessionKey,
	sessionKey string,
) (*protocol.Session, error) {
	if c.sessions == nil {
		return nil, nil
	}
	resolved, err := c.sessions.ResolveDeliverySession(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	if resolved == nil ||
		strings.TrimSpace(resolved.SessionKey) != strings.TrimSpace(sessionKey) ||
		strings.TrimSpace(resolved.AgentID) != strings.TrimSpace(parsed.AgentID) {
		return nil, nil
	}
	materialized, err := files.UpsertSession(workspacePath, *resolved)
	if err != nil {
		return nil, err
	}
	if materialized != nil {
		return materialized, nil
	}
	return resolved, nil
}

func internalSessionTitle(parsed protocol.SessionKey) string {
	if strings.TrimSpace(parsed.Ref) == protocol.AutomationInboxSessionRef {
		return "定时任务收件箱"
	}
	return "自动投递"
}

func (c *sessionDeliveryChannel) persistMessage(
	ownerUserID string,
	workspacePath string,
	sessionValue protocol.Session,
	message protocol.Message,
) (protocol.Session, error) {
	history := c.history.ForOwner(ownerUserID)
	if err := history.AppendOverlayMessage(workspacePath, sessionValue.SessionKey, message); err != nil {
		return protocol.Session{}, err
	}

	sessionValue.MessageCount++
	sessionValue.LastActivity = time.Now().UTC()
	if strings.TrimSpace(stringValue(message["session_id"])) != "" {
		sessionID := strings.TrimSpace(stringValue(message["session_id"]))
		sessionValue.SessionID = &sessionID
	}
	sessionValue.Status = "closed"
	sessionValue.IsActive = false
	updated, err := c.files.ForOwner(ownerUserID).PatchSessionRuntime(workspacePath, sessionValue)
	if err != nil {
		return protocol.Session{}, err
	}
	if updated == nil {
		return sessionValue, nil
	}
	return *updated, nil
}

func (c *sessionDeliveryChannel) broadcastMessage(
	ctx context.Context,
	sessionKey string,
	agentID string,
	message protocol.Message,
) {
	if c.permission == nil {
		return
	}
	event := protocol.NewEvent(protocol.EventTypeMessage, message)
	event.DeliveryMode = "durable"
	event.SessionKey = sessionKey
	event.AgentID = agentID
	event.MessageID = strings.TrimSpace(stringValue(message["message_id"]))
	c.permission.BroadcastEvent(ctx, sessionKey, event)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
