package launcher

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Bootstrap 返回 Launcher 首屏最小必要数据。
func (s *Service) Bootstrap(ctx context.Context) (BootstrapResponse, error) {
	agents, err := s.agentService.ListAgentRecords(ctx)
	if err != nil {
		return BootstrapResponse{}, err
	}
	rooms, err := s.roomService.ListRooms(ctx, 200)
	if err != nil {
		return BootstrapResponse{}, err
	}

	agentItems := make([]BootstrapAgent, 0, len(agents))
	agentByID := make(map[string]protocol.Agent, len(agents))
	for _, agentValue := range agents {
		agentByID[agentValue.AgentID] = agentValue
		if agentValue.IsMain {
			continue
		}
		agentItems = append(agentItems, BootstrapAgent{
			ID:          agentValue.AgentID,
			Name:        agentValue.Name,
			Avatar:      agentValue.Avatar,
			Description: agentValue.Description,
		})
	}

	roomItems := make([]BootstrapRoom, 0, len(rooms))
	roomTypeByID := make(map[string]string, len(rooms))
	for _, roomValue := range rooms {
		roomTypeByID[roomValue.Room.ID] = roomValue.Room.RoomType
		roomItems = append(roomItems, BootstrapRoom{
			ID:              roomValue.Room.ID,
			RoomType:        normalizeLauncherRoomType(roomValue.Room.RoomType),
			Name:            roomValue.Room.Name,
			Avatar:          roomValue.Room.Avatar,
			DMTargetAgentID: firstRoomAgentID(roomValue),
			CreatedAt:       isoString(roomValue.Room.CreatedAt),
			UpdatedAt:       isoString(roomValue.Room.UpdatedAt),
			Members:         buildBootstrapRoomMembers(roomValue, agentByID),
		})
	}

	conversationItems := make([]BootstrapConversation, 0)
	if s.session != nil {
		sessions, listErr := s.session.ListSessions(ctx)
		if listErr != nil {
			return BootstrapResponse{}, listErr
		}
		conversationItems = buildBootstrapConversations(sessions, roomTypeByID)
		if previewErr := s.attachLatestReplyPreviews(ctx, conversationItems); previewErr != nil {
			return BootstrapResponse{}, previewErr
		}
	}

	return BootstrapResponse{
		Agents:        agentItems,
		Rooms:         roomItems,
		Conversations: conversationItems,
	}, nil
}

func (s *Service) attachLatestReplyPreviews(
	ctx context.Context,
	items []BootstrapConversation,
) error {
	seenRoomIDs := make(map[string]struct{}, len(items))
	for index := range items {
		roomID := strings.TrimSpace(items[index].RoomID)
		if roomID == "" {
			continue
		}
		if _, exists := seenRoomIDs[roomID]; exists {
			continue
		}
		seenRoomIDs[roomID] = struct{}{}

		preview, err := s.session.GetSessionLatestReplyPreview(ctx, items[index].SessionKey)
		if err != nil {
			return err
		}
		items[index].LastReplyPreview = preview
	}
	return nil
}

func buildBootstrapRoomMembers(
	roomValue protocol.RoomAggregate,
	agentByID map[string]protocol.Agent,
) []BootstrapRoomMember {
	members := make([]BootstrapRoomMember, 0, len(roomValue.Members))
	for _, member := range roomValue.Members {
		if member.MemberType != protocol.MemberTypeAgent {
			continue
		}
		agentValue, ok := agentByID[member.MemberAgentID]
		if !ok {
			continue
		}
		members = append(members, BootstrapRoomMember{
			ID:     agentValue.AgentID,
			Name:   agentValue.Name,
			Avatar: agentValue.Avatar,
		})
	}
	return members
}

func buildBootstrapConversations(
	sessions []protocol.Session,
	roomTypeByID map[string]string,
) []BootstrapConversation {
	items := make([]BootstrapConversation, 0, len(sessions))
	for _, item := range sessions {
		roomID := strings.TrimSpace(stringPointerValue(item.RoomID))
		conversationID := strings.TrimSpace(stringPointerValue(item.ConversationID))
		agentID := strings.TrimSpace(item.AgentID)
		roomType := normalizeBootstrapConversationRoomType(item.ChatType, roomTypeByID[roomID])

		// Launcher 推荐项必须能稳定打开到具体会话；无法定位的会话不参与推荐。
		if roomID == "" && conversationID == "" && agentID == "" {
			continue
		}
		lastActivity := item.LastActivity
		if lastActivity.IsZero() {
			lastActivity = item.CreatedAt
		}
		items = append(items, BootstrapConversation{
			SessionKey:     item.SessionKey,
			AgentID:        agentID,
			RoomID:         roomID,
			ConversationID: conversationID,
			RoomType:       roomType,
			ChannelType:    strings.TrimSpace(item.ChannelType),
			ChatType:       strings.TrimSpace(item.ChatType),
			Title:          normalizeBootstrapConversationTitle(item.Title, roomType),
			Status:         strings.TrimSpace(item.Status),
			IsActive:       item.IsActive,
			LastActivity:   isoString(lastActivity),
			MessageCount:   item.MessageCount,
		})
	}
	return items
}

func normalizeBootstrapConversationRoomType(chatType string, roomType string) string {
	normalizedRoomType := strings.TrimSpace(roomType)
	if normalizedRoomType == protocol.RoomTypeDM || normalizedRoomType == protocol.RoomTypeGroup {
		return normalizeLauncherRoomType(normalizedRoomType)
	}
	if strings.TrimSpace(chatType) == protocol.RoomTypeDM {
		return protocol.RoomTypeDM
	}
	return "room"
}

func defaultLauncherConversationTitle(roomType string) string {
	if roomType == protocol.RoomTypeDM {
		return "未命名会话"
	}
	return "未命名话题"
}

func normalizeLauncherRoomType(roomType string) string {
	if strings.TrimSpace(roomType) == protocol.RoomTypeDM {
		return protocol.RoomTypeDM
	}
	return "room"
}

func normalizeBootstrapConversationTitle(title string, roomType string) string {
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle != "" {
		return trimmedTitle
	}
	return defaultLauncherConversationTitle(roomType)
}

func firstRoomAgentID(roomValue protocol.RoomAggregate) string {
	for _, member := range roomValue.Members {
		if strings.TrimSpace(member.MemberAgentID) != "" {
			return member.MemberAgentID
		}
	}
	return ""
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
