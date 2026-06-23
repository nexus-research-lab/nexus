package roomrepo

import (
	"database/sql"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
)

type Scanner interface {
	Scan(...any) error
}

func ScanRoomRecord(scanner Scanner) (protocol.RoomRecord, error) {
	var (
		item           protocol.RoomRecord
		skillNamesJSON string
		createdAt      time.Time
		updatedAt      time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.OwnerUserID,
		&item.RoomType,
		&item.Name,
		&item.Description,
		&item.Avatar,
		&skillNamesJSON,
		&item.HostAgentID,
		&item.HostAutoReplyEnabled,
		&item.PrivateMessagesEnabled,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return protocol.RoomRecord{}, err
	}
	item.SkillNames = jsoncodec.ParseStringSlice(skillNamesJSON)
	if item.SkillNames == nil {
		item.SkillNames = []string{}
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func ScanMemberRecord(scanner Scanner) (protocol.MemberRecord, error) {
	var (
		item     protocol.MemberRecord
		joinedAt time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.RoomID,
		&item.MemberType,
		&item.MemberUserID,
		&item.MemberAgentID,
		&joinedAt,
	)
	if err != nil {
		return protocol.MemberRecord{}, err
	}
	item.JoinedAt = joinedAt
	return item, nil
}

func ScanRoomMemberAgent(scanner Scanner) (protocol.Agent, error) {
	var (
		item                protocol.Agent
		vibeTagsJSON        string
		allowedToolsJSON    string
		disallowedToolsJSON string
		mcpServersJSON      string
		settingSourcesJSON  string
		maxTurns            sql.NullInt64
		maxThinkingTokens   sql.NullInt64
		createdAt           time.Time
	)

	err := scanner.Scan(
		&item.AgentID,
		&item.Name,
		&item.OwnerUserID,
		&item.WorkspacePath,
		&item.Status,
		&item.IsMain,
		&item.Avatar,
		&item.Description,
		&vibeTagsJSON,
		&createdAt,
		&item.Options.Provider,
		&item.Options.Model,
		&item.Options.PermissionMode,
		&allowedToolsJSON,
		&disallowedToolsJSON,
		&mcpServersJSON,
		&maxTurns,
		&maxThinkingTokens,
		&settingSourcesJSON,
	)
	if err != nil {
		return protocol.Agent{}, err
	}

	item.CreatedAt = createdAt
	item.VibeTags = jsoncodec.ParseStringSlice(vibeTagsJSON)
	item.Options.AllowedTools = jsoncodec.ParseStringSlice(allowedToolsJSON)
	item.Options.DisallowedTools = jsoncodec.ParseStringSlice(disallowedToolsJSON)
	item.Options.MCPServers = jsoncodec.ParseMap(mcpServersJSON)
	item.Options.SettingSources = jsoncodec.ParseStringSlice(settingSourcesJSON)
	if maxTurns.Valid {
		value := int(maxTurns.Int64)
		item.Options.MaxTurns = &value
	}
	if maxThinkingTokens.Valid {
		value := int(maxThinkingTokens.Int64)
		item.Options.MaxThinkingTokens = &value
	}
	return item, nil
}

func ScanConversationRecord(scanner Scanner) (protocol.ConversationRecord, error) {
	var (
		item      protocol.ConversationRecord
		createdAt time.Time
		updatedAt time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.RoomID,
		&item.ConversationType,
		&item.Title,
		&item.MessageCount,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return protocol.ConversationRecord{}, err
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func ScanSessionRecord(scanner Scanner) (protocol.SessionRecord, error) {
	var (
		item           protocol.SessionRecord
		lastActivityAt time.Time
		createdAt      time.Time
		updatedAt      time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.ConversationID,
		&item.AgentID,
		&item.RuntimeID,
		&item.VersionNo,
		&item.BranchKey,
		&item.IsPrimary,
		&item.SDKSessionID,
		&item.Status,
		&lastActivityAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return protocol.SessionRecord{}, err
	}
	item.LastActivityAt = lastActivityAt
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func PickMainConversation(conversations []protocol.ConversationRecord) *protocol.ConversationRecord {
	for _, conversation := range conversations {
		if conversation.ConversationType == protocol.ConversationTypeMain || conversation.ConversationType == protocol.ConversationTypeDM {
			item := conversation
			return &item
		}
	}
	if len(conversations) == 0 {
		return nil
	}
	item := conversations[0]
	return &item
}

func PickLatestConversationContext(contexts []protocol.ConversationContextAggregate) *protocol.ConversationContextAggregate {
	if len(contexts) == 0 {
		return nil
	}

	latestIndex := 0
	latestAt := conversationContextLastActivityAt(contexts[0])
	for index := 1; index < len(contexts); index++ {
		candidateAt := conversationContextLastActivityAt(contexts[index])
		if candidateAt.After(latestAt) {
			latestIndex = index
			latestAt = candidateAt
		}
	}
	return &contexts[latestIndex]
}

func conversationContextLastActivityAt(contextValue protocol.ConversationContextAggregate) time.Time {
	latestAt := contextValue.Conversation.UpdatedAt
	if latestAt.IsZero() {
		latestAt = contextValue.Conversation.CreatedAt
	}

	for _, sessionValue := range contextValue.Sessions {
		sessionAt := sessionValue.LastActivityAt
		if sessionAt.IsZero() {
			sessionAt = sessionValue.UpdatedAt
		}
		if sessionAt.IsZero() {
			sessionAt = sessionValue.CreatedAt
		}
		if sessionAt.After(latestAt) {
			latestAt = sessionAt
		}
	}
	return latestAt
}
