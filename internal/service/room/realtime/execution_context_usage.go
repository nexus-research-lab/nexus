// INPUT: 已结束一轮的 Room slot runtime client 与共享 Room 身份。
// OUTPUT: 向 Room session 广播该 Agent 的权威上下文占用快照。
// POS: Room slot 终态与前端 Composer 按 Agent 快照之间的事件桥。
package realtime

import (
	"context"
	"maps"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const roomContextUsageReadTimeout = 1500 * time.Millisecond

// broadcastContextUsage 在 runtime 支持时广播当前 slot 的上下文快照。
func (e *slotExecution) broadcastContextUsage(client runtimectx.Client) {
	if e == nil ||
		e.service == nil ||
		e.round == nil ||
		e.slot == nil ||
		client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		roomContextUsageReadTimeout,
	)
	defer cancel()
	usage, available, err := runtimectx.ReadContextUsage(ctx, client)
	if err != nil {
		e.logger.Debug(
			"Room context usage 读取失败",
			"agent_id", e.slot.AgentID,
			"agent_round_id", e.slot.AgentRoundID,
			"err", err,
		)
		return
	}
	if !available {
		return
	}
	if err := e.persistContextUsage(usage); err != nil {
		e.logger.Warn(
			"Room context usage 持久化失败",
			"agent_id", e.slot.AgentID,
			"agent_round_id", e.slot.AgentRoundID,
			"err", err,
		)
	}
	e.service.runtime.RecordContextUsage(
		e.round.SessionKey,
		e.slot.AgentID,
		usage,
	)
	event := roomdomain.WrapContextUsageEvent(
		e.round.SessionKey,
		e.round.RoomID,
		e.round.ConversationID,
		e.round.RootRoundID,
		e.slot.AgentRoundID,
		e.slot.AgentID,
		usage,
	)
	e.service.broadcastSharedEventWithTimeout(
		context.Background(),
		e.round.SessionKey,
		e.round.RoomID,
		event,
	)
}

func (e *slotExecution) persistContextUsage(
	usage protocol.ContextUsageData,
) error {
	if e.service.files == nil {
		return nil
	}
	files := e.service.files.ForOwner(e.slot.OwnerUserID)
	current, _, err := files.FindSession(
		[]string{e.slot.WorkspacePath},
		e.slot.RuntimeSessionKey,
	)
	if err != nil {
		return err
	}
	sessionValue := e.contextUsageSessionMeta()
	if current != nil {
		sessionValue = *current
	}
	usageSnapshot := usage
	sessionValue.ContextUsage = &usageSnapshot
	sessionValue.Status = "closed"
	sessionValue.IsActive = false
	sessionValue.LastActivity = time.Now().UTC()
	_, err = files.UpsertSession(e.slot.WorkspacePath, sessionValue)
	return err
}

func (e *slotExecution) contextUsageSessionMeta() protocol.Session {
	now := time.Now().UTC()
	roomSessionID := contextUsageStringPointer(e.slot.RoomSessionID)
	roomID := contextUsageStringPointer(e.round.RoomID)
	conversationID := contextUsageStringPointer(e.round.ConversationID)
	sessionID := contextUsageStringPointer(e.slot.getSDKSessionID())
	title := "New Chat"
	messageCount := 0
	createdAt := now
	lastActivity := now
	options := map[string]any{}

	if contextValue := e.round.Context; contextValue != nil {
		title = firstContextUsageString(
			contextValue.Conversation.Title,
			contextValue.Room.Name,
			title,
		)
		messageCount = contextValue.Conversation.MessageCount
		createdAt = firstContextUsageTime(
			contextValue.Conversation.CreatedAt,
			contextValue.Room.CreatedAt,
			createdAt,
		)
		lastActivity = firstContextUsageTime(
			contextValue.Conversation.LastActivityAt,
			contextValue.Conversation.UpdatedAt,
			lastActivity,
		)
		if record, found := findRoomSessionForAgent(
			contextValue.Sessions,
			e.slot.AgentID,
		); found {
			roomSessionID = contextUsageStringPointer(firstContextUsageString(
				e.slot.RoomSessionID,
				record.ID,
			))
			sessionID = contextUsageStringPointer(firstContextUsageString(
				e.slot.getSDKSessionID(),
				record.SDKSessionID,
			))
			createdAt = firstContextUsageTime(record.CreatedAt, createdAt)
			lastActivity = firstContextUsageTime(
				contextValue.Conversation.LastActivityAt,
				record.LastActivityAt,
				lastActivity,
			)
			options = maps.Clone(record.Options)
		}
	}
	if options == nil {
		options = map[string]any{}
	}
	chatType := "group"
	if strings.TrimSpace(e.round.RoomType) == protocol.RoomTypeDM {
		chatType = "dm"
	}
	return protocol.Session{
		SessionKey:     e.slot.RuntimeSessionKey,
		AgentID:        e.slot.AgentID,
		SessionID:      sessionID,
		RoomSessionID:  roomSessionID,
		RoomID:         roomID,
		ConversationID: conversationID,
		ChannelType:    protocol.SessionChannelWebSocket,
		ChatType:       chatType,
		Status:         "closed",
		CreatedAt:      createdAt,
		LastActivity:   lastActivity,
		Title:          title,
		MessageCount:   messageCount,
		Options:        options,
		IsActive:       false,
	}
}

func contextUsageStringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func firstContextUsageString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstContextUsageTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Time{}
}
