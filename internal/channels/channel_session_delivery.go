package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentsvc "github.com/nexus-research-lab/nexus/internal/agent"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type agentWorkspaceResolver interface {
	GetAgent(context.Context, string) (*agentsvc.Agent, error)
	GetDefaultAgent(context.Context) (*agentsvc.Agent, error)
}

type sessionDeliveryChannel struct {
	channelType string
	agents      agentWorkspaceResolver
	permission  *permissionctx.Context
	files       *workspacestore.SessionFileStore
	history     *workspacestore.AgentHistoryStore
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
		idFactory:   newDeliveryID,
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

// SendDeliveryText 按 session_key 追加 assistant 正文与内部 result overlay，
// 对外统一只广播挂载 result_summary 的 assistant。
func (c *sessionDeliveryChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	sessionKey := firstNonEmpty(target.SessionKey, target.To)
	sessionKey, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return err
	}

	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return errors.New("shared room delivery 暂不支持")
	}
	if c.agents == nil {
		return errors.New("session delivery 缺少 agent 解析器")
	}

	agentValue, err := c.agents.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return err
	}
	sessionValue, workspacePath, err := c.files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil {
		return err
	}
	if sessionValue == nil || strings.TrimSpace(workspacePath) == "" {
		return fmt.Errorf("delivery target session is not available: %s", sessionKey)
	}

	now := time.Now().UTC()
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

	updated, err := c.persistMessage(workspacePath, *sessionValue, assistantMessage)
	if err != nil {
		return err
	}
	if _, err = c.persistMessage(workspacePath, updated, resultMessage); err != nil {
		return err
	}

	c.broadcastMessage(ctx, sessionKey, parsed.AgentID, protocol.ProjectResultMessage(assistantMessage, resultMessage))
	return nil
}

func (c *sessionDeliveryChannel) persistMessage(
	workspacePath string,
	sessionValue session.Session,
	message protocol.Message,
) (session.Session, error) {
	if err := c.appendHistoryMessage(workspacePath, sessionValue, message); err != nil {
		return session.Session{}, err
	}

	sessionValue.MessageCount++
	sessionValue.LastActivity = time.Now().UTC()
	if strings.TrimSpace(stringValue(message["session_id"])) != "" {
		sessionID := strings.TrimSpace(stringValue(message["session_id"]))
		sessionValue.SessionID = &sessionID
	}
	sessionValue.Status = "active"
	updated, err := c.files.UpsertSession(workspacePath, sessionValue)
	if err != nil {
		return session.Session{}, err
	}
	if updated == nil {
		return sessionValue, nil
	}
	return *updated, nil
}

func (c *sessionDeliveryChannel) appendHistoryMessage(
	workspacePath string,
	sessionValue session.Session,
	message protocol.Message,
) error {
	return c.history.AppendOverlayMessage(workspacePath, sessionValue.SessionKey, message)
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

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
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
