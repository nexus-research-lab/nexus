package launcher

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

const (
	actionOpenAgentDM = "open_agent_dm"
	actionOpenRoom    = "open_room"
	actionOpenApp     = "open_app"
)

// Service 提供 Launcher 查询和推荐能力。
type Service struct {
	config       config.Config
	agentService *agentsvc.Service
	roomService  *roomsvc.Service
	session      *sessionsvc.Service
}

// NewService 创建 Launcher 服务。
func NewService(
	cfg config.Config,
	agentService *agentsvc.Service,
	roomService *roomsvc.Service,
	sessionService *sessionsvc.Service,
) *Service {
	return &Service{
		config:       cfg,
		agentService: agentService,
		roomService:  roomService,
		session:      sessionService,
	}
}

// Query 解析 Launcher 输入并生成动作。
func (s *Service) Query(ctx context.Context, query string) (QueryResponse, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return QueryResponse{
			ActionType: actionOpenApp,
			TargetID:   "app",
		}, nil
	}

	if keyword, initialMessage, matched := splitTriggeredQuery(trimmedQuery, "@"); matched {
		agentValue, err := s.findAgentByKeyword(ctx, keyword)
		if err != nil {
			return QueryResponse{}, err
		}
		if agentValue != nil {
			return QueryResponse{
				ActionType:     actionOpenAgentDM,
				TargetID:       agentValue.AgentID,
				InitialMessage: initialMessage,
			}, nil
		}
		return QueryResponse{
			ActionType:     actionOpenApp,
			TargetID:       "app",
			InitialMessage: trimmedQuery,
		}, nil
	}

	if keyword, initialMessage, matched := splitTriggeredQuery(trimmedQuery, "#"); matched {
		roomValue, err := s.findRoomByKeyword(ctx, keyword)
		if err != nil {
			return QueryResponse{}, err
		}
		if roomValue != nil {
			return QueryResponse{
				ActionType:     actionOpenRoom,
				TargetID:       roomValue.Room.ID,
				InitialMessage: initialMessage,
			}, nil
		}
		return QueryResponse{
			ActionType:     actionOpenApp,
			TargetID:       "app",
			InitialMessage: trimmedQuery,
		}, nil
	}

	agentValue, err := s.findAgentByKeyword(ctx, trimmedQuery)
	if err != nil {
		return QueryResponse{}, err
	}
	if agentValue != nil {
		return QueryResponse{
			ActionType: actionOpenAgentDM,
			TargetID:   agentValue.AgentID,
		}, nil
	}

	roomValue, err := s.findRoomByKeyword(ctx, trimmedQuery)
	if err != nil {
		return QueryResponse{}, err
	}
	if roomValue != nil {
		return QueryResponse{
			ActionType: actionOpenRoom,
			TargetID:   roomValue.Room.ID,
		}, nil
	}

	return QueryResponse{
		ActionType:     actionOpenApp,
		TargetID:       "app",
		InitialMessage: trimmedQuery,
	}, nil
}

// Suggestions 返回 Launcher 推荐项。
func (s *Service) Suggestions(ctx context.Context) (SuggestionsResponse, error) {
	agents, err := s.agentService.ListAgentRecords(ctx)
	if err != nil {
		return SuggestionsResponse{}, err
	}
	rooms, err := s.roomService.ListRooms(ctx, 15)
	if err != nil {
		return SuggestionsResponse{}, err
	}

	agentItems := make([]Suggestion, 0, len(agents))
	for _, agentValue := range agents {
		if agentValue.IsMain {
			continue
		}
		agentItems = append(agentItems, Suggestion{
			Type:   "agent",
			ID:     agentValue.AgentID,
			Name:   agentValue.Name,
			Avatar: agentValue.Avatar,
		})
		if len(agentItems) >= 8 {
			break
		}
	}

	roomItems := make([]Suggestion, 0, len(rooms))
	for _, roomValue := range rooms {
		if roomValue.Room.RoomType != protocol.RoomTypeGroup {
			continue
		}
		lastActivity := roomValue.Room.UpdatedAt
		if lastActivity.IsZero() {
			lastActivity = roomValue.Room.CreatedAt
		}
		roomName := strings.TrimSpace(roomValue.Room.Name)
		if roomName == "" {
			roomName = "未命名 Room"
		}
		roomItems = append(roomItems, Suggestion{
			Type:         "room",
			ID:           roomValue.Room.ID,
			Name:         roomName,
			LastActivity: isoString(lastActivity),
		})
		if len(roomItems) >= 5 {
			break
		}
	}

	return SuggestionsResponse{
		Agents: agentItems,
		Rooms:  roomItems,
	}, nil
}

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
	for _, agentValue := range agents {
		if agentValue.IsMain {
			continue
		}
		agentItems = append(agentItems, BootstrapAgent{
			ID:     agentValue.AgentID,
			Name:   agentValue.Name,
			Avatar: agentValue.Avatar,
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
		})
	}

	conversationItems := make([]BootstrapConversation, 0)
	if s.session != nil {
		sessions, listErr := s.session.ListSessions(ctx)
		if listErr != nil {
			return BootstrapResponse{}, listErr
		}
		conversationItems = buildBootstrapConversations(sessions, roomTypeByID)
	}

	return BootstrapResponse{
		Agents:        agentItems,
		Rooms:         roomItems,
		Conversations: conversationItems,
	}, nil
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
			Title:          normalizeBootstrapConversationTitle(item.Title, roomType),
			LastActivity:   isoString(lastActivity),
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

func (s *Service) findAgentByKeyword(ctx context.Context, keyword string) (*protocol.Agent, error) {
	agents, err := s.agentService.ListAgentRecords(ctx)
	if err != nil {
		return nil, err
	}

	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return nil, nil
	}

	for _, agentValue := range agents {
		if agentValue.IsMain {
			continue
		}
		if strings.ToLower(agentValue.Name) == normalizedKeyword {
			item := agentValue
			return &item, nil
		}
	}
	for _, agentValue := range agents {
		if agentValue.IsMain {
			continue
		}
		if strings.Contains(strings.ToLower(agentValue.Name), normalizedKeyword) {
			item := agentValue
			return &item, nil
		}
	}
	return nil, nil
}

func (s *Service) findRoomByKeyword(ctx context.Context, keyword string) (*protocol.RoomAggregate, error) {
	rooms, err := s.roomService.ListRooms(ctx, 100)
	if err != nil {
		return nil, err
	}

	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return nil, nil
	}

	for _, roomValue := range rooms {
		if roomValue.Room.RoomType != protocol.RoomTypeGroup {
			continue
		}
		if strings.ToLower(roomValue.Room.Name) == normalizedKeyword {
			item := roomValue
			return &item, nil
		}
	}
	for _, roomValue := range rooms {
		if roomValue.Room.RoomType != protocol.RoomTypeGroup {
			continue
		}
		if strings.Contains(strings.ToLower(roomValue.Room.Name), normalizedKeyword) {
			item := roomValue
			return &item, nil
		}
	}
	return nil, nil
}

func splitTriggeredQuery(value string, trigger string) (string, string, bool) {
	trimmedValue := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmedValue, trigger) {
		return "", "", false
	}

	body := strings.TrimSpace(strings.TrimPrefix(trimmedValue, trigger))
	if body == "" {
		return "", "", true
	}
	parts := strings.Fields(body)
	if len(parts) == 0 {
		return "", "", true
	}
	keyword := parts[0]
	initialMessage := ""
	if len(parts) > 1 {
		initialMessage = strings.Join(parts[1:], " ")
	}
	return keyword, initialMessage, true
}

func firstRoomAgentID(roomValue protocol.RoomAggregate) string {
	for _, member := range roomValue.Members {
		if strings.TrimSpace(member.MemberAgentID) != "" {
			return member.MemberAgentID
		}
	}
	return ""
}

func isoString(value interface {
	IsZero() bool
	Format(string) string
}) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02T15:04:05Z07:00")
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
