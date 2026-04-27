package room

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// NewEntityID 创建 Room 内部短 ID。
func NewEntityID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%d", len(buffer))
}

// BuildMembers 构建 Room 成员记录。
func BuildMembers(roomID string, ownerUserID string, agentIDs []string) []protocol.MemberRecord {
	members := []protocol.MemberRecord{
		{
			ID:           NewEntityID(),
			RoomID:       roomID,
			MemberType:   protocol.MemberTypeUser,
			MemberUserID: ownerUserID,
		},
	}
	for _, agentID := range agentIDs {
		members = append(members, protocol.MemberRecord{
			ID:            NewEntityID(),
			RoomID:        roomID,
			MemberType:    protocol.MemberTypeAgent,
			MemberAgentID: agentID,
		})
	}
	return members
}

// BuildSessions 构建 Room conversation 的成员运行时会话记录。
func BuildSessions(conversationID string, refs []AgentRuntimeRef) []protocol.SessionRecord {
	sessions := make([]protocol.SessionRecord, 0, len(refs))
	for _, ref := range refs {
		sessions = append(sessions, protocol.SessionRecord{
			ID:             NewEntityID(),
			ConversationID: conversationID,
			AgentID:        ref.AgentID,
			RuntimeID:      ref.RuntimeID,
			VersionNo:      1,
			BranchKey:      "main",
			IsPrimary:      true,
			Status:         "active",
		})
	}
	return sessions
}

// BuildRoomName 根据成员生成默认 Room 名称。
func BuildRoomName(refs []AgentRuntimeRef, roomType string) string {
	if len(refs) == 0 {
		return ""
	}
	if roomType == protocol.RoomTypeDM {
		return PickDisplayName(refs[0])
	}
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, PickDisplayName(ref))
	}
	return strings.Join(names, "、")
}

// BuildNextConversationTitle 根据现有话题数生成下一条话题标题。
func BuildNextConversationTitle(roomName string, contexts []protocol.ConversationContextAggregate) string {
	baseName := NormalizeOptionalText(roomName)
	if baseName == "" {
		baseName = "未命名 room"
	}
	topicCount := 0
	for _, contextValue := range contexts {
		if contextValue.Conversation.ConversationType == protocol.ConversationTypeTopic {
			topicCount++
		}
	}
	return fmt.Sprintf("%s · 对话 %d", baseName, topicCount+1)
}

// PickMainConversationType 返回 Room 主对话类型。
func PickMainConversationType(roomType string) string {
	if roomType == protocol.RoomTypeDM {
		return protocol.ConversationTypeDM
	}
	return protocol.ConversationTypeMain
}

// PickDisplayName 返回 Agent 的 Room 展示名。
func PickDisplayName(ref AgentRuntimeRef) string {
	if strings.TrimSpace(ref.DisplayName) != "" {
		return ref.DisplayName
	}
	return ref.Name
}

// ListAgentIDs 从成员列表提取 Agent 成员 ID。
func ListAgentIDs(members []protocol.MemberRecord) []string {
	agentIDs := make([]string, 0)
	for _, member := range members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID != "" {
			agentIDs = append(agentIDs, member.MemberAgentID)
		}
	}
	return agentIDs
}

// NormalizeOptionalText 归一化可选文本字段。
func NormalizeOptionalText(value string) string {
	return strings.TrimSpace(value)
}

// NormalizeOptionalPatch 归一化可选 patch 字段，并保留是否显式传入。
func NormalizeOptionalPatch(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	return strings.TrimSpace(value), true
}

// ContainsString 判断字符串切片是否包含目标值。
func ContainsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

// HasConversation 判断上下文集合里是否包含指定 conversation。
func HasConversation(contexts []protocol.ConversationContextAggregate, conversationID string) bool {
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return true
		}
	}
	return false
}

// FindConversation 查找指定 conversation 记录。
func FindConversation(contexts []protocol.ConversationContextAggregate, conversationID string) (protocol.ConversationRecord, bool) {
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return contextValue.Conversation, true
		}
	}
	return protocol.ConversationRecord{}, false
}

// FindConversationContext 查找指定 conversation 上下文。
func FindConversationContext(contexts []protocol.ConversationContextAggregate, conversationID string) (protocol.ConversationContextAggregate, bool) {
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return contextValue, true
		}
	}
	return protocol.ConversationContextAggregate{}, false
}
