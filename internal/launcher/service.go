// =====================================================
// @File   ：service.go
// @Date   ：2026/04/10 23:10:00
// @Author ：leemysw
// 2026/04/10 23:10:00   Create
// =====================================================

package launcher

import (
	"context"
	"strings"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
	roomsvc "github.com/nexus-research-lab/nexus/internal/room"
)

const (
	actionOpenAgentDM = "open_agent_dm"
	actionOpenRoom    = "open_room"
	actionOpenApp     = "open_app"
)

// Service 提供 Launcher 查询和推荐能力。
type Service struct {
	config       config.Config
	agentService *agent2.Service
	roomService  *roomsvc.Service
}

// NewService 创建 Launcher 服务。
func NewService(cfg config.Config, agentService *agent2.Service, roomService *roomsvc.Service) *Service {
	return &Service{
		config:       cfg,
		agentService: agentService,
		roomService:  roomService,
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
	agents, err := s.agentService.ListAgents(ctx)
	if err != nil {
		return SuggestionsResponse{}, err
	}
	rooms, err := s.roomService.ListRooms(ctx, 15)
	if err != nil {
		return SuggestionsResponse{}, err
	}

	agentItems := make([]Suggestion, 0, len(agents))
	for _, agentValue := range agents {
		if agentValue.AgentID == s.config.DefaultAgentID {
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
		if roomValue.Room.RoomType != "room" {
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

func (s *Service) findAgentByKeyword(ctx context.Context, keyword string) (*agent2.Agent, error) {
	agents, err := s.agentService.ListAgents(ctx)
	if err != nil {
		return nil, err
	}

	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return nil, nil
	}

	for _, agentValue := range agents {
		if agentValue.AgentID == s.config.DefaultAgentID {
			continue
		}
		if strings.ToLower(agentValue.Name) == normalizedKeyword {
			item := agentValue
			return &item, nil
		}
	}
	for _, agentValue := range agents {
		if agentValue.AgentID == s.config.DefaultAgentID {
			continue
		}
		if strings.Contains(strings.ToLower(agentValue.Name), normalizedKeyword) {
			item := agentValue
			return &item, nil
		}
	}
	return nil, nil
}

func (s *Service) findRoomByKeyword(ctx context.Context, keyword string) (*roomsvc.RoomAggregate, error) {
	rooms, err := s.roomService.ListRooms(ctx, 100)
	if err != nil {
		return nil, err
	}

	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return nil, nil
	}

	for _, roomValue := range rooms {
		if roomValue.Room.RoomType != "room" {
			continue
		}
		if strings.ToLower(roomValue.Room.Name) == normalizedKeyword {
			item := roomValue
			return &item, nil
		}
	}
	for _, roomValue := range rooms {
		if roomValue.Room.RoomType != "room" {
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

func isoString(value interface {
	IsZero() bool
	Format(string) string
}) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02T15:04:05Z07:00")
}
