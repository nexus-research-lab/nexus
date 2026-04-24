package room

import (
	"context"
	"errors"
	"fmt"
	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/conversation/titlegen"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"strings"
	"time"
	"unicode/utf8"
)

// HandleChat 处理 Room 主对话消息。
func (s *RealtimeService) HandleChat(ctx context.Context, request ChatRequest) error {
	sessionKey, conversationID, err := s.validateChatRequest(request)
	if err != nil {
		return err
	}

	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	roomID := firstNonEmpty(strings.TrimSpace(request.RoomID), contextValue.Room.ID)

	if err = s.interruptRound(ctx, sessionKey, "", "收到新的用户消息，上一轮已停止", true); err != nil {
		return err
	}

	agentNameByID, agentByID, err := s.buildAgentDirectory(ctx, contextValue.Members)
	if err != nil {
		return err
	}
	targetAgentIDs := ResolveMentionAgentIDs(request.Content, reverseAgentNames(agentNameByID))
	if len(targetAgentIDs) == 0 && len(agentNameByID) == 1 {
		// 单成员直聊 Room 再强制 @mention 只会制造额外交互噪音，
		// 这里直接把唯一成员当作默认目标，保持与 DM 直觉一致。
		for agentID := range agentNameByID {
			targetAgentIDs = []string{agentID}
		}
	}
	s.loggerFor(ctx).Info("受理 Room 会话消息",
		"session_key", sessionKey,
		"room_id", roomID,
		"conversation_id", conversationID,
		"round_id", request.RoundID,
		"target_agent_count", len(targetAgentIDs),
		"target_agents", append([]string(nil), targetAgentIDs...),
		"content_chars", utf8.RuneCountInString(strings.TrimSpace(request.Content)),
	)

	history, err := s.roomHistory.ReadMessages(conversationID, nil)
	if err != nil {
		return err
	}

	userMessage := sessionmodel.Message{
		"message_id":      request.RoundID,
		"session_key":     sessionKey,
		"room_id":         roomID,
		"conversation_id": conversationID,
		"agent_id":        "",
		"round_id":        request.RoundID,
		"role":            "user",
		"content":         strings.TrimSpace(request.Content),
		"timestamp":       time.Now().UnixMilli(),
	}
	if err = s.persistSharedInlineMessage(conversationID, userMessage); err != nil {
		return err
	}
	s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomMessageEvent(roomID, conversationID, userMessage, request.RoundID))
	s.scheduleTitleGeneration(ctx, sessionKey, contextValue, strings.TrimSpace(request.Content))

	if len(targetAgentIDs) == 0 {
		s.loggerFor(ctx).Warn("Room 消息未命中任何目标成员",
			"session_key", sessionKey,
			"room_id", roomID,
			"conversation_id", conversationID,
			"round_id", request.RoundID,
		)
		hintMessage := sessionmodel.Message{
			"message_id":      "result_" + request.RoundID,
			"session_key":     sessionKey,
			"room_id":         roomID,
			"conversation_id": conversationID,
			"agent_id":        "",
			"round_id":        request.RoundID,
			"role":            "result",
			"subtype":         "success",
			"duration_ms":     0,
			"duration_api_ms": 0,
			"num_turns":       0,
			"result":          "请使用 @AgentName 指定要对话的成员",
			"is_error":        false,
			"timestamp":       time.Now().UnixMilli(),
		}
		if err = s.persistSharedInlineMessage(conversationID, hintMessage); err != nil {
			return err
		}
		s.broadcastSharedEvent(
			ctx,
			sessionKey,
			roomID,
			wrapRoomMessageEvent(
				roomID,
				conversationID,
				sessionmodel.ProjectResultMessage(nil, hintMessage),
				request.RoundID,
			),
		)
		s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomRoundStatusEvent(sessionKey, roomID, conversationID, request.RoundID, "finished", "success"))
		return nil
	}

	sessionsByAgent := make(map[string]SessionRecord, len(contextValue.Sessions))
	for _, item := range contextValue.Sessions {
		sessionsByAgent[item.AgentID] = item
	}

	activeRound := &activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         roomID,
		ConversationID: conversationID,
		RoomType:       contextValue.Room.RoomType,
		RoundID:        request.RoundID,
		Slots:          make(map[string]*activeRoomSlot),
		Done:           make(chan struct{}),
	}

	pending := make([]map[string]any, 0, len(targetAgentIDs))
	for index, agentID := range targetAgentIDs {
		sessionRecord, ok := sessionsByAgent[agentID]
		if !ok {
			continue
		}
		agentValue := agentByID[agentID]
		if agentValue == nil {
			continue
		}
		msgID := newRealtimeID()
		agentRoundID := request.RoundID
		if len(targetAgentIDs) > 1 {
			agentRoundID = fmt.Sprintf("%s:%s", request.RoundID, agentID)
		}
		activeRound.Slots[msgID] = &activeRoomSlot{
			RoomSessionID:     sessionRecord.ID,
			SDKSessionID:      strings.TrimSpace(sessionRecord.SDKSessionID),
			AgentID:           agentID,
			AgentRoundID:      agentRoundID,
			MsgID:             msgID,
			RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, agentID, contextValue.Room.RoomType),
			WorkspacePath:     agentValue.WorkspacePath,
			Status:            "pending",
			Index:             index,
			TimestampMS:       normalizeInt64(userMessage["timestamp"]),
			Done:              make(chan struct{}),
		}
		_ = sessionRecord
		pending = append(pending, map[string]any{
			"agent_id":  agentID,
			"msg_id":    msgID,
			"round_id":  agentRoundID,
			"status":    "pending",
			"timestamp": userMessage["timestamp"],
			"index":     index,
		})
	}
	if len(activeRound.Slots) == 0 {
		s.loggerFor(ctx).Warn("Room 中没有可用成员会话",
			"session_key", sessionKey,
			"room_id", roomID,
			"conversation_id", conversationID,
			"round_id", request.RoundID,
		)
		s.broadcastSharedEvent(ctx, sessionKey, roomID, s.newRoomErrorEvent(sessionKey, roomID, conversationID, "room_error", "Room 中没有可用成员会话", request.RoundID))
		s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomRoundStatusEvent(sessionKey, roomID, conversationID, request.RoundID, "error", "error"))
		return nil
	}

	roundCtx, cancel := context.WithCancel(context.Background())
	activeRound.Cancel = cancel
	s.registerRound(activeRound)
	s.runtime.StartRound(sessionKey, request.RoundID, cancel)

	s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomRoundStatusEvent(sessionKey, roomID, conversationID, request.RoundID, "running", ""))
	s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomChatAckEvent(sessionKey, roomID, conversationID, firstNonEmpty(request.ReqID, request.RoundID), request.RoundID, pending))
	s.broadcastSessionStatus(ctx, sessionKey)

	go s.runRound(roundCtx, activeRound, history, request.Content, agentNameByID, agentByID)
	return nil
}

func (s *RealtimeService) scheduleTitleGeneration(
	ctx context.Context,
	sessionKey string,
	contextValue *ConversationContextAggregate,
	content string,
) {
	if s.titles == nil || contextValue == nil {
		return
	}
	s.titles.Schedule(ctx, titlegen.Request{
		SessionKey:               sessionKey,
		Provider:                 "",
		Content:                  content,
		ConversationID:           contextValue.Conversation.ID,
		ConversationRoomID:       contextValue.Room.ID,
		ConversationTitle:        contextValue.Conversation.Title,
		ConversationRoomName:     contextValue.Room.Name,
		ConversationMessageCount: contextValue.Conversation.MessageCount,
	})
}

// HandleInterrupt 处理中断请求。
func (s *RealtimeService) HandleInterrupt(ctx context.Context, request InterruptRequest) error {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return err
	}
	return s.interruptRound(ctx, sessionKey, strings.TrimSpace(request.MsgID), "", false)
}

func (s *RealtimeService) validateChatRequest(request ChatRequest) (string, string, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", "", err
	}
	if !protocol.IsRoomSharedSessionKey(sessionKey) {
		return "", "", errors.New("session_key must be room shared key")
	}
	if strings.TrimSpace(request.RoundID) == "" {
		return "", "", errors.New("round_id is required")
	}
	if strings.TrimSpace(request.Content) == "" {
		return "", "", errors.New("content is required")
	}
	conversationID := firstNonEmpty(strings.TrimSpace(request.ConversationID), protocol.ParseRoomConversationID(sessionKey))
	if conversationID == "" {
		return "", "", errors.New("conversation_id is required")
	}
	return sessionKey, conversationID, nil
}

func (s *RealtimeService) buildAgentDirectory(
	ctx context.Context,
	members []MemberRecord,
) (map[string]string, map[string]*agent2.Agent, error) {
	agentNameByID := make(map[string]string)
	agentByID := make(map[string]*agent2.Agent)
	for _, member := range members {
		if member.MemberType != "agent" || strings.TrimSpace(member.MemberAgentID) == "" {
			continue
		}
		agentValue, err := s.agents.GetAgent(ctx, member.MemberAgentID)
		if err != nil {
			return nil, nil, err
		}
		agentNameByID[agentValue.AgentID] = agentValue.Name
		agentByID[agentValue.AgentID] = agentValue
	}
	return agentNameByID, agentByID, nil
}

func (s *RealtimeService) persistSharedInlineMessage(conversationID string, message sessionmodel.Message) error {
	return s.roomHistory.AppendInlineMessage(conversationID, message)
}

func (s *RealtimeService) persistSharedDurableMessage(
	conversationID string,
	slot *activeRoomSlot,
	message sessionmodel.Message,
) error {
	if slot == nil || !sessionmodel.IsTranscriptNativeMessage(sessionmodel.Message(message)) {
		return s.persistSharedInlineMessage(conversationID, message)
	}
	return s.roomHistory.AppendTranscriptReference(
		conversationID,
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		message,
	)
}
