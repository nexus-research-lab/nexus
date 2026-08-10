// INPUT: 内部 protocol/workspace 完整模型。
// OUTPUT: 移除路径、options、owner、DB/runtime/SDK 标识后的 manager 投影。
// POS: nexus_manager 的统一数据最小化边界。
package nexusmanager

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

func agentView(value protocol.Agent) AgentView {
	return AgentView{
		AgentID: value.AgentID, Name: value.Name, IsMain: value.IsMain,
		DisplayName: value.DisplayName, Headline: value.Headline, Status: value.Status,
		Avatar: value.Avatar, Description: value.Description,
		VibeTags: slices.Clone(value.VibeTags), SkillsCount: value.SkillsCount,
		RuntimeVersion: value.RuntimeVersion, CreatedAt: value.CreatedAt,
	}
}

func roomView(value protocol.RoomAggregate) RoomView {
	memberAgentIDs := make([]string, 0, len(value.Members))
	for _, member := range value.Members {
		if member.MemberType != protocol.MemberTypeAgent {
			continue
		}
		if id := strings.TrimSpace(member.MemberAgentID); id != "" {
			memberAgentIDs = append(memberAgentIDs, id)
		}
	}
	return RoomView{
		ID: value.Room.ID, RoomType: value.Room.RoomType, Name: value.Room.Name,
		Description: value.Room.Description, Avatar: value.Room.Avatar,
		SkillNames: slices.Clone(value.Room.SkillNames), HostAgentID: value.Room.HostAgentID,
		HostAutoReplyEnabled:   value.Room.HostAutoReplyEnabled,
		PrivateMessagesEnabled: value.Room.PrivateMessagesEnabled,
		ConfigurationVersion:   value.Room.ConfigurationVersion,
		AuthorityEpoch:         value.Room.AuthorityEpoch, MemberAgentIDs: memberAgentIDs,
		CreatedAt: value.Room.CreatedAt, UpdatedAt: value.Room.UpdatedAt,
	}
}

func contextView(value protocol.ConversationContextAggregate) ConversationContextView {
	memberAgents := make([]AgentView, 0, len(value.MemberAgents))
	for _, agentValue := range value.MemberAgents {
		memberAgents = append(memberAgents, agentView(agentValue))
	}
	sessions := make([]RoomRuntimeView, 0, len(value.Sessions))
	for _, session := range value.Sessions {
		sessions = append(sessions, RoomRuntimeView{
			AgentID: session.AgentID, VersionNo: session.VersionNo,
			IsPrimary: session.IsPrimary, Status: session.Status,
			LastActivityAt: session.LastActivityAt, CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
		})
	}
	return ConversationContextView{
		Room: roomView(protocol.RoomAggregate{Room: value.Room, Members: value.Members}),
		Conversation: ConversationView{
			ID: value.Conversation.ID, RoomID: value.Conversation.RoomID,
			ConversationType: value.Conversation.ConversationType,
			Title:            value.Conversation.Title, IsDraft: value.Conversation.IsDraft,
			MessageCount:   value.Conversation.MessageCount,
			LastActivityAt: value.Conversation.LastActivityAt,
			CreatedAt:      value.Conversation.CreatedAt, UpdatedAt: value.Conversation.UpdatedAt,
		},
		MemberAgents: memberAgents, Sessions: sessions,
	}
}

func sessionView(value protocol.Session) SessionView {
	return SessionView{
		SessionKey: value.SessionKey, AgentID: value.AgentID,
		RoomID:         stringPointerValue(value.RoomID),
		ConversationID: stringPointerValue(value.ConversationID),
		ChannelType:    value.ChannelType, ChatType: value.ChatType, Status: value.Status,
		CreatedAt: value.CreatedAt, LastActivity: value.LastActivity, Title: value.Title,
		MessageCount: value.MessageCount, IsActive: value.IsActive,
	}
}

func workspaceEntry(value workspacepkg.FileEntry) WorkspaceEntry {
	return WorkspaceEntry{
		Path: value.Path, Name: value.Name, IsDir: value.IsDir, Size: value.Size,
		ModifiedAt: value.ModifiedAt, Depth: value.Depth,
	}
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func boundedUTF8(value string, maxBytes int) (string, bool) {
	if len(value) <= maxBytes {
		return value, false
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value, true
}
