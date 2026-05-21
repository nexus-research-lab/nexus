package room

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	// ErrPrivateThreadNotFound 表示私域线程不存在。
	ErrPrivateThreadNotFound = errors.New("private thread not found")
)

const (
	defaultPrivateThreadLimit = 50
	defaultPrivateEventLimit  = 80
	defaultPrivateRoomLimit   = 120
	maxPrivateThreadLimit     = 200
	maxPrivateEventLimit      = 300
	maxPrivateRoomLimit       = 300
	privateDomainPreviewRunes = 120
)

// AgentPrivateDomainQuery 描述 Agent 私域投影的过滤条件。
type AgentPrivateDomainQuery struct {
	RoomID         string
	ConversationID string
	Limit          int
	RoomLimit      int
}

type privateDomainThreadBuilder struct {
	thread protocol.AgentPrivateThread
	events []protocol.AgentPrivateEvent
}

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
		threads = append(threads, builder.thread)
	}
	sort.SliceStable(threads, func(i, j int) bool {
		if threads[i].LastTimestamp == threads[j].LastTimestamp {
			return threads[i].ThreadID < threads[j].ThreadID
		}
		return threads[i].LastTimestamp > threads[j].LastTimestamp
	})

	limit := normalizePrivateLimit(query.Limit, defaultPrivateThreadLimit, maxPrivateThreadLimit)
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

	events := append([]protocol.AgentPrivateEvent(nil), builder.events...)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Timestamp == events[j].Timestamp {
			return events[i].ActionID < events[j].ActionID
		}
		return events[i].Timestamp < events[j].Timestamp
	})
	limit := normalizePrivateLimit(query.Limit, defaultPrivateEventLimit, maxPrivateEventLimit)
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return protocol.AgentPrivateEventPage{
		Thread: builder.thread,
		Items:  events,
	}, nil
}

func (s *Service) collectAgentPrivateDomain(
	ctx context.Context,
	agentID string,
	query AgentPrivateDomainQuery,
) (map[string]*privateDomainThreadBuilder, error) {
	contexts, err := s.loadPrivateDomainContexts(ctx, query)
	if err != nil {
		return nil, err
	}

	actionStore := workspacestore.NewRoomActionStore(s.config.WorkspacePath)
	builders := make(map[string]*privateDomainThreadBuilder)
	for _, contextValue := range contexts {
		actions, readErr := actionStore.ReadActions(contextValue.Conversation.ID)
		if readErr != nil {
			return nil, readErr
		}
		if len(actions) == 0 {
			continue
		}
		participantsByID := privateDomainParticipantsByID(contextValue)
		for _, action := range actions {
			event, ok := buildPrivateDomainEvent(agentID, contextValue, participantsByID, action)
			if !ok {
				continue
			}
			builder := builders[event.ThreadID]
			if builder == nil {
				builder = &privateDomainThreadBuilder{
					thread: privateDomainThreadFromEvent(agentID, event),
				}
				builders[event.ThreadID] = builder
			}
			builder.events = append(builder.events, event)
			builder.thread.ActionCount++
			if event.Timestamp >= builder.thread.LastTimestamp {
				updatePrivateDomainThreadFromEvent(&builder.thread, event)
			}
		}
	}
	return builders, nil
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

	roomLimit := normalizePrivateLimit(query.RoomLimit, defaultPrivateRoomLimit, maxPrivateRoomLimit)
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

func buildPrivateDomainEvent(
	agentID string,
	contextValue protocol.ConversationContextAggregate,
	participantsByID map[string]protocol.AgentPrivateParticipant,
	action protocol.RoomActionRecord,
) (protocol.AgentPrivateEvent, bool) {
	participantIDs := privateDomainParticipantIDs(action)
	if !containsPrivateDomainAgent(participantIDs, agentID) {
		return protocol.AgentPrivateEvent{}, false
	}
	if !privateDomainActionVisible(action) {
		return protocol.AgentPrivateEvent{}, false
	}

	scope, _ := privateDomainScope(agentID, participantIDs)
	threadID := privateDomainThreadID(scope, participantIDs)
	return protocol.AgentPrivateEvent{
		ActionID:          strings.TrimSpace(action.ActionID),
		ThreadID:          threadID,
		Direction:         privateDomainDirection(agentID, action),
		ActionType:        action.ActionType,
		RequestID:         strings.TrimSpace(action.RequestID),
		SourceAgentID:     strings.TrimSpace(action.SourceAgentID),
		TargetAgentID:     strings.TrimSpace(action.TargetAgentID),
		AudienceAgentIDs:  normalizedPrivateDomainAgents(action.AudienceAgentIDs),
		Content:           strings.TrimSpace(action.Content),
		Visibility:        strings.TrimSpace(action.Visibility),
		ReplyTarget:       action.ReplyTarget,
		WakePolicy:        action.WakePolicy,
		DelaySeconds:      action.DelaySeconds,
		RoomID:            contextValue.Room.ID,
		RoomName:          contextValue.Room.Name,
		RoomType:          contextValue.Room.RoomType,
		ConversationID:    contextValue.Conversation.ID,
		ConversationTitle: contextValue.Conversation.Title,
		Participants:      buildPrivateDomainParticipants(participantIDs, participantsByID),
		Timestamp:         action.Timestamp,
	}, true
}

func privateDomainActionVisible(action protocol.RoomActionRecord) bool {
	if strings.TrimSpace(action.SourceAgentID) == "" || strings.TrimSpace(action.ActionID) == "" {
		return false
	}
	switch action.ActionType {
	case protocol.RoomActionTypePrivateMessage,
		protocol.RoomActionTypeRequestReply,
		protocol.RoomActionTypePrivateNote:
		return true
	case protocol.RoomActionTypeMarker:
		return strings.TrimSpace(action.Visibility) != protocol.RoomActionVisibilityPublic &&
			action.ReplyTarget != protocol.RoomReplyTargetPublicFeed
	default:
		return false
	}
}

func privateDomainThreadFromEvent(agentID string, event protocol.AgentPrivateEvent) protocol.AgentPrivateThread {
	thread := protocol.AgentPrivateThread{
		ThreadID: event.ThreadID,
		AgentID:  agentID,
	}
	updatePrivateDomainThreadFromEvent(&thread, event)
	return thread
}

func updatePrivateDomainThreadFromEvent(thread *protocol.AgentPrivateThread, event protocol.AgentPrivateEvent) {
	participantIDs := make([]string, 0, len(event.Participants))
	for _, participant := range event.Participants {
		if strings.TrimSpace(participant.AgentID) != "" {
			participantIDs = append(participantIDs, strings.TrimSpace(participant.AgentID))
		}
	}
	scope, peers := privateDomainScope(thread.AgentID, participantIDs)
	thread.Scope = scope
	thread.ParticipantAgentIDs = participantIDs
	thread.PeerAgentIDs = peers
	thread.Participants = append([]protocol.AgentPrivateParticipant(nil), event.Participants...)
	thread.RoomID = event.RoomID
	thread.RoomName = event.RoomName
	thread.RoomType = event.RoomType
	thread.ConversationID = event.ConversationID
	thread.ConversationTitle = event.ConversationTitle
	thread.LastActionID = event.ActionID
	thread.LastActionType = event.ActionType
	thread.LastContentPreview = privateDomainContentPreview(event.Content)
	thread.LastTimestamp = event.Timestamp
}

func privateDomainParticipantIDs(action protocol.RoomActionRecord) []string {
	ids := make([]string, 0, 2+len(action.AudienceAgentIDs))
	ids = append(ids, strings.TrimSpace(action.SourceAgentID))
	if strings.TrimSpace(action.TargetAgentID) != "" {
		ids = append(ids, strings.TrimSpace(action.TargetAgentID))
	}
	ids = append(ids, action.AudienceAgentIDs...)
	return normalizedPrivateDomainAgents(ids)
}

func privateDomainScope(agentID string, participantIDs []string) (string, []string) {
	peers := make([]string, 0, len(participantIDs))
	for _, participantID := range participantIDs {
		if participantID != agentID {
			peers = append(peers, participantID)
		}
	}
	sort.Strings(peers)
	switch len(peers) {
	case 0:
		return "self", peers
	case 1:
		return "direct", peers
	default:
		return "audience", peers
	}
}

func privateDomainThreadID(scope string, participantIDs []string) string {
	hash := sha256.Sum256([]byte(scope + ":" + strings.Join(participantIDs, ",")))
	return "pd_" + hex.EncodeToString(hash[:])[:16]
}

func privateDomainDirection(agentID string, action protocol.RoomActionRecord) string {
	if action.ActionType == protocol.RoomActionTypePrivateNote {
		return "self"
	}
	if strings.TrimSpace(action.SourceAgentID) == agentID {
		return "outgoing"
	}
	return "incoming"
}

func privateDomainParticipantsByID(
	contextValue protocol.ConversationContextAggregate,
) map[string]protocol.AgentPrivateParticipant {
	participants := make(map[string]protocol.AgentPrivateParticipant, len(contextValue.MemberAgents))
	for _, agent := range contextValue.MemberAgents {
		participants[agent.AgentID] = protocol.AgentPrivateParticipant{
			AgentID: agent.AgentID,
			Name:    agent.Name,
			Avatar:  agent.Avatar,
		}
	}
	return participants
}

func buildPrivateDomainParticipants(
	participantIDs []string,
	participantsByID map[string]protocol.AgentPrivateParticipant,
) []protocol.AgentPrivateParticipant {
	participants := make([]protocol.AgentPrivateParticipant, 0, len(participantIDs))
	for _, participantID := range participantIDs {
		participant := participantsByID[participantID]
		if strings.TrimSpace(participant.AgentID) == "" {
			participant = protocol.AgentPrivateParticipant{
				AgentID: participantID,
				Name:    participantID,
			}
		}
		participants = append(participants, participant)
	}
	return participants
}

func normalizedPrivateDomainAgents(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func containsPrivateDomainAgent(values []string, agentID string) bool {
	for _, value := range values {
		if value == agentID {
			return true
		}
	}
	return false
}

func privateDomainContentPreview(content string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	runes := []rune(normalized)
	if len(runes) <= privateDomainPreviewRunes {
		return normalized
	}
	return string(runes[:privateDomainPreviewRunes]) + "..."
}

func normalizePrivateLimit(value int, defaultValue int, maxValue int) int {
	if value <= 0 {
		return defaultValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
