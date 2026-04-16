// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：session_delivery_channel.go
// @Date   ：2026/04/11 22:18:00
// @Author ：leemysw
// 2026/04/11 22:18:00   Create
// =====================================================

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
}

type sessionDeliveryChannel struct {
	channelType string
	agents      agentWorkspaceResolver
	permission  *permissionctx.Context
	files       *workspacestore.SessionFileStore
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

// SendDeliveryText 按 session_key 追加一组 assistant/result 消息。
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
	assistantMessage := session.Message{
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
	resultMessage := session.Message{
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

	c.broadcastMessage(ctx, sessionKey, parsed.AgentID, assistantMessage)
	c.broadcastMessage(ctx, sessionKey, parsed.AgentID, resultMessage)
	return nil
}

func (c *sessionDeliveryChannel) persistMessage(
	workspacePath string,
	sessionValue session.Session,
	message session.Message,
) (session.Session, error) {
	if err := c.files.AppendSessionMessage(workspacePath, sessionValue.SessionKey, message); err != nil {
		return session.Session{}, err
	}
	if strings.TrimSpace(stringValue(message["role"])) == "result" {
		if err := c.files.AppendSessionCost(workspacePath, sessionValue.SessionKey, buildDeliveryCostRow(message)); err != nil {
			return session.Session{}, err
		}
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

func (c *sessionDeliveryChannel) broadcastMessage(
	ctx context.Context,
	sessionKey string,
	agentID string,
	message session.Message,
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

func buildDeliveryCostRow(message session.Message) map[string]any {
	usage, _ := message["usage"].(map[string]any)
	return map[string]any{
		"entry_id":                    "cost_" + stringValue(message["message_id"]),
		"agent_id":                    stringValue(message["agent_id"]),
		"session_key":                 stringValue(message["session_key"]),
		"session_id":                  stringValue(message["session_id"]),
		"round_id":                    stringValue(message["round_id"]),
		"message_id":                  stringValue(message["message_id"]),
		"subtype":                     stringValue(message["subtype"]),
		"input_tokens":                intValue(usage["input_tokens"]),
		"output_tokens":               intValue(usage["output_tokens"]),
		"cache_creation_input_tokens": intValue(usage["cache_creation_input_tokens"]),
		"cache_read_input_tokens":     intValue(usage["cache_read_input_tokens"]),
		"total_cost_usd":              floatValue(message["total_cost_usd"]),
		"duration_ms":                 intValue(message["duration_ms"]),
		"duration_api_ms":             intValue(message["duration_api_ms"]),
		"num_turns":                   intValue(message["num_turns"]),
		"created_at":                  time.Now().UTC().Format(time.RFC3339),
	}
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

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func floatValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}
