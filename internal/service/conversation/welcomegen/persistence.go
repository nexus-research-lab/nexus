// INPUT: 已生成欢迎语、发言 Agent 与新建 conversation 身份。
// OUTPUT: 幂等写入 DM overlay 或 Room inline ledger 的 assistant 消息。
// POS: 欢迎语持久化边界；不推进 draft、runtime round 或 Goal 状态。
package welcomegen

import (
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) persistWelcome(
	aggregate protocol.ConversationContextAggregate,
	speaker protocol.Agent,
	generation welcomeGeneration,
) error {
	conversationID := strings.TrimSpace(aggregate.Conversation.ID)
	ownerUserID := strings.TrimSpace(aggregate.Room.OwnerUserID)
	sessionKey, messageID := welcomeMessageIdentity(aggregate, speaker)
	if s.welcomeExists(aggregate, speaker, sessionKey, messageID) {
		return nil
	}

	message := protocol.Message{
		"message_id":      messageID,
		"session_key":     sessionKey,
		"room_id":         strings.TrimSpace(aggregate.Room.ID),
		"conversation_id": conversationID,
		"agent_id":        strings.TrimSpace(speaker.AgentID),
		"round_id":        "round_welcome_" + conversationID,
		"role":            "assistant",
		"content": []map[string]any{{
			"type": "text",
			"text": strings.TrimSpace(generation.text),
		}},
		"is_complete": true,
		"stop_reason": "end_turn",
		"timestamp":   time.Now().UTC().UnixMilli(),
		"metadata": map[string]any{
			"subtype":      "conversation_welcome",
			"generated_by": strings.TrimSpace(generation.source),
			"welcome_kind": strings.TrimSpace(string(generation.kind)),
		},
	}
	if model := strings.TrimSpace(generation.model); model != "" {
		message["model"] = model
	}

	if aggregate.Room.RoomType == protocol.RoomTypeDM {
		if strings.TrimSpace(speaker.WorkspacePath) == "" {
			return fmt.Errorf("DM 欢迎语 Agent workspace 为空: %s", speaker.AgentID)
		}
		return s.history.ForOwner(ownerUserID).AppendOverlayMessage(
			speaker.WorkspacePath,
			sessionKey,
			message,
		)
	}
	return s.rooms.AppendInlineMessage(ownerUserID, conversationID, message)
}

func welcomeMessageIdentity(
	aggregate protocol.ConversationContextAggregate,
	speaker protocol.Agent,
) (string, string) {
	conversationID := strings.TrimSpace(aggregate.Conversation.ID)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	if aggregate.Room.RoomType == protocol.RoomTypeDM {
		sessionKey = protocol.BuildRoomAgentSessionKey(
			conversationID,
			speaker.AgentID,
			protocol.RoomTypeDM,
		)
	}
	return sessionKey, "msg_assistant_welcome_" + conversationID
}

func (s *Service) welcomeExists(
	aggregate protocol.ConversationContextAggregate,
	speaker protocol.Agent,
	sessionKey string,
	messageID string,
) bool {
	var (
		messages []protocol.Message
		err      error
	)
	if aggregate.Room.RoomType == protocol.RoomTypeDM {
		messages, err = s.history.ForOwner(aggregate.Room.OwnerUserID).ReadMessages(
			speaker.WorkspacePath,
			protocol.Session{SessionKey: sessionKey, AgentID: speaker.AgentID},
			nil,
		)
	} else {
		messages, err = s.rooms.ReadMessages(
			aggregate.Room.OwnerUserID,
			aggregate.Conversation.ID,
			nil,
		)
	}
	if err != nil {
		return false
	}
	for _, message := range messages {
		if strings.TrimSpace(fmt.Sprint(message["message_id"])) == messageID {
			return true
		}
	}
	return false
}
