// INPUT: Agent/Room 持久摘要与只读 Session metadata 目录。
// OUTPUT: 保持 wire 兼容的 Launcher 首屏 agents、rooms 与 conversations 摘要。
// POS: Launcher 首屏投影；禁止读取 transcript、overlay 或按会话计算回复预览。
package launcher

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

const slowBootstrapLogThreshold = 500 * time.Millisecond

// Bootstrap 幂等保证主智能体默认聊天存在，并返回 Launcher 首屏最小必要数据。
func (s *Service) Bootstrap(ctx context.Context) (BootstrapResponse, error) {
	startedAt := time.Now()
	agentsStartedAt := time.Now()
	agents, err := s.agentService.ListAgentRecords(ctx)
	if err != nil {
		return BootstrapResponse{}, err
	}
	agentsDuration := time.Since(agentsStartedAt)
	mainAgentID := ""
	for _, agentValue := range agents {
		if agentValue.IsMain {
			mainAgentID = agentValue.AgentID
			break
		}
	}
	if mainAgentID == "" {
		return BootstrapResponse{}, agentsvc.ErrAgentNotFound
	}
	ensureDirectRoomStartedAt := time.Now()
	if _, err = s.roomService.EnsureDirectRoom(ctx, mainAgentID); err != nil {
		return BootstrapResponse{}, err
	}
	ensureDirectRoomDuration := time.Since(ensureDirectRoomStartedAt)
	roomsStartedAt := time.Now()
	rooms, err := s.roomService.ListRooms(ctx, 200)
	if err != nil {
		return BootstrapResponse{}, err
	}
	roomsDuration := time.Since(roomsStartedAt)

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

	sessionsStartedAt := time.Now()
	sessions, listErr := s.session.ListDirectorySessions(ctx)
	sessionsDuration := time.Since(sessionsStartedAt)
	if listErr != nil {
		return BootstrapResponse{}, listErr
	}
	conversationItems := buildBootstrapConversations(sessions, roomTypeByID)
	duration := time.Since(startedAt)
	if duration >= slowBootstrapLogThreshold {
		slog.InfoContext(
			ctx,
			"Launcher bootstrap 慢查询",
			"duration_ms", duration.Milliseconds(),
			"agents_ms", agentsDuration.Milliseconds(),
			"ensure_dm_ms", ensureDirectRoomDuration.Milliseconds(),
			"rooms_ms", roomsDuration.Milliseconds(),
			"sessions_ms", sessionsDuration.Milliseconds(),
			"agent_count", len(agents),
			"room_count", len(roomItems),
			"conversation_count", len(conversationItems),
		)
	}

	return BootstrapResponse{
		Agents:        agentItems,
		Rooms:         roomItems,
		Conversations: conversationItems,
	}, nil
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
		roomID := stringPointerValue(item.RoomID)
		conversationID := stringPointerValue(item.ConversationID)
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
			Title:          normalizeBootstrapConversationTitle(item.Title, roomType),
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
