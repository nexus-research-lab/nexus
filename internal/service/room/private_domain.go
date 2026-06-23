package room

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/room/privateview"
)

var (
	// ErrPrivateThreadNotFound 表示私域线程不存在。
	ErrPrivateThreadNotFound = errors.New("private thread not found")
)

// AgentPrivateDomainQuery 描述 Agent 私域投影的过滤条件。
type AgentPrivateDomainQuery = privateview.Query

// ListAgentPrivateThreads 返回某个 Agent 视角下的全局私域线程。
func (s *Service) ListAgentPrivateThreads(
	ctx context.Context,
	agentID string,
	query AgentPrivateDomainQuery,
) (protocol.AgentPrivateThreadPage, error) {
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedAgentID == "" {
		return protocol.AgentPrivateThreadPage{}, agentsvc.ErrAgentNotFound
	}
	if _, err := s.agents.GetAgent(ctx, normalizedAgentID); err != nil {
		return protocol.AgentPrivateThreadPage{}, err
	}

	builders, err := s.collectAgentPrivateDomain(ctx, normalizedAgentID, query)
	if err != nil {
		return protocol.AgentPrivateThreadPage{}, err
	}

	threads := make([]protocol.AgentPrivateThread, 0, len(builders))
	for _, builder := range builders {
		threads = append(threads, builder.Thread)
	}
	slices.SortStableFunc(threads, func(left protocol.AgentPrivateThread, right protocol.AgentPrivateThread) int {
		if result := cmp.Compare(right.LastTimestamp, left.LastTimestamp); result != 0 {
			return result
		}
		return cmp.Compare(left.ThreadID, right.ThreadID)
	})

	limit := privateview.ThreadLimit(query.Limit)
	if len(threads) > limit {
		threads = threads[:limit]
	}
	return protocol.AgentPrivateThreadPage{Items: threads}, nil
}

// ListAgentPrivateEvents 返回某个 Agent 私域线程内的 action 事件。
func (s *Service) ListAgentPrivateEvents(
	ctx context.Context,
	agentID string,
	threadID string,
	query AgentPrivateDomainQuery,
) (protocol.AgentPrivateEventPage, error) {
	normalizedAgentID := strings.TrimSpace(agentID)
	normalizedThreadID := strings.TrimSpace(threadID)
	if normalizedAgentID == "" {
		return protocol.AgentPrivateEventPage{}, agentsvc.ErrAgentNotFound
	}
	if normalizedThreadID == "" {
		return protocol.AgentPrivateEventPage{}, ErrPrivateThreadNotFound
	}
	if _, err := s.agents.GetAgent(ctx, normalizedAgentID); err != nil {
		return protocol.AgentPrivateEventPage{}, err
	}

	builders, err := s.collectAgentPrivateDomain(ctx, normalizedAgentID, query)
	if err != nil {
		return protocol.AgentPrivateEventPage{}, err
	}
	builder, ok := builders[normalizedThreadID]
	if !ok {
		return protocol.AgentPrivateEventPage{}, ErrPrivateThreadNotFound
	}

	events := slices.Clone(builder.Events)
	slices.SortStableFunc(events, func(left protocol.AgentPrivateEvent, right protocol.AgentPrivateEvent) int {
		if result := cmp.Compare(left.Timestamp, right.Timestamp); result != 0 {
			return result
		}
		return cmp.Compare(left.MessageID, right.MessageID)
	})
	limit := privateview.EventLimit(query.Limit)
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return protocol.AgentPrivateEventPage{
		Thread: builder.Thread,
		Items:  events,
	}, nil
}

func (s *Service) collectAgentPrivateDomain(
	ctx context.Context,
	agentID string,
	query AgentPrivateDomainQuery,
) (map[string]*privateview.ThreadBuilder, error) {
	contexts, err := s.loadPrivateDomainContexts(ctx, query)
	if err != nil {
		return nil, err
	}
	return privateview.Project(agentID, contexts, s.config.WorkspacePath)
}

func (s *Service) loadPrivateDomainContexts(
	ctx context.Context,
	query AgentPrivateDomainQuery,
) ([]protocol.ConversationContextAggregate, error) {
	roomID := strings.TrimSpace(query.RoomID)
	conversationID := strings.TrimSpace(query.ConversationID)
	if conversationID != "" {
		contextValue, err := s.GetConversationContext(ctx, conversationID)
		if err != nil {
			return nil, err
		}
		if roomID != "" && contextValue.Room.ID != roomID {
			return nil, ErrConversationNotFound
		}
		return []protocol.ConversationContextAggregate{*contextValue}, nil
	}
	if roomID != "" {
		return s.GetRoomContexts(ctx, roomID)
	}

	roomLimit := privateview.RoomLimit(query.RoomLimit)
	rooms, err := s.ListRooms(ctx, roomLimit)
	if err != nil {
		return nil, err
	}
	contexts := make([]protocol.ConversationContextAggregate, 0, len(rooms))
	for _, roomValue := range rooms {
		roomContexts, contextErr := s.GetRoomContexts(ctx, roomValue.Room.ID)
		if errors.Is(contextErr, ErrRoomNotFound) {
			continue
		}
		if contextErr != nil {
			return nil, contextErr
		}
		contexts = append(contexts, roomContexts...)
	}
	return contexts, nil
}
