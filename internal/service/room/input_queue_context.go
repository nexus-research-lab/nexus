// INPUT: Room queue 请求、成员会话、@mention 与当前活跃 root round。
// OUTPUT: 显式 @ 优先、否则绑定最近活跃 slots 的队列存储位置与目标列表。
// POS: Room input queue 入队目标解析入口；与直接 chat 共享活跃 round 优先语义。
package room

import (
	"cmp"
	"context"
	"errors"
	"sort"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *RealtimeService) resolveInputQueueContext(
	ctx context.Context,
	request InputQueueRequest,
) (string, *protocol.ConversationContextAggregate, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", nil, err
	}
	if !protocol.IsRoomSharedSessionKey(sessionKey) {
		return "", nil, errors.New("session_key must be room shared key")
	}
	conversationID := cmp.Or(strings.TrimSpace(request.ConversationID), protocol.ParseRoomConversationID(sessionKey))
	if conversationID == "" {
		return "", nil, errors.New("conversation_id is required")
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return "", nil, err
	}
	if contextValue == nil {
		return "", nil, errors.New("room conversation not found")
	}
	return sessionKey, contextValue, nil
}

func (s *RealtimeService) resolveRoomInputQueuePrimaryLocation(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	content string,
) (workspacestore.InputQueueLocation, []string, error) {
	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return workspacestore.InputQueueLocation{}, nil, err
	}
	targetAgentIDs := roomdomain.ResolveMentionAgentIDs(content, roomdomain.BuildMentionAliases(contextValue))
	if len(targetAgentIDs) == 0 {
		targetAgentIDs = s.latestActiveRootRoundAgentIDs(
			protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID),
			contextValue.Conversation.ID,
		)
	}
	if len(targetAgentIDs) == 0 && len(locationsByAgentID) == 1 {
		for agentID := range locationsByAgentID {
			targetAgentIDs = []string{agentID}
		}
	}
	if len(targetAgentIDs) == 0 {
		if hostAgentID, ok := resolveRoomHostDefaultTarget(contextValue, agentNameByIDFromInputLocations(locationsByAgentID)); ok {
			targetAgentIDs = []string{hostAgentID}
		}
	}
	if len(targetAgentIDs) == 0 {
		return workspacestore.InputQueueLocation{}, nil, errors.New("room input_queue content must mention target agent")
	}

	cleanTargets := make([]string, 0, len(targetAgentIDs))
	for _, agentID := range targetAgentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			continue
		}
		if _, ok := locationsByAgentID[agentID]; !ok {
			continue
		}
		cleanTargets = append(cleanTargets, agentID)
	}
	if len(cleanTargets) == 0 {
		return workspacestore.InputQueueLocation{}, nil, errors.New("room input_queue target agent not found")
	}
	return locationsByAgentID[cleanTargets[0]].Location, cleanTargets, nil
}

func agentNameByIDFromInputLocations(locations map[string]roomInputQueueLocation) map[string]string {
	result := make(map[string]string, len(locations))
	for agentID := range locations {
		normalizedAgentID := strings.TrimSpace(agentID)
		if normalizedAgentID == "" {
			continue
		}
		result[normalizedAgentID] = normalizedAgentID
	}
	return result
}

func (s *RealtimeService) roomInputQueueLocations(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) ([]roomInputQueueLocation, error) {
	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	locations := make([]roomInputQueueLocation, 0, len(locationsByAgentID))
	for _, member := range contextValue.Members {
		if member.MemberType != protocol.MemberTypeAgent {
			continue
		}
		if location, ok := locationsByAgentID[strings.TrimSpace(member.MemberAgentID)]; ok {
			locations = append(locations, location)
		}
	}
	sort.SliceStable(locations, func(i int, j int) bool {
		return locations[i].AgentID < locations[j].AgentID
	})
	return locations, nil
}

func (s *RealtimeService) roomInputQueueLocationsByAgent(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) (map[string]roomInputQueueLocation, error) {
	if contextValue == nil {
		return map[string]roomInputQueueLocation{}, nil
	}
	agentsByID := make(map[string]protocol.Agent, len(contextValue.MemberAgents))
	for _, agentValue := range contextValue.MemberAgents {
		agentID := strings.TrimSpace(agentValue.AgentID)
		if agentID != "" {
			agentsByID[agentID] = agentValue
		}
	}
	for _, member := range contextValue.Members {
		agentID := strings.TrimSpace(member.MemberAgentID)
		if member.MemberType != protocol.MemberTypeAgent || agentID == "" {
			continue
		}
		if _, exists := agentsByID[agentID]; exists {
			continue
		}
		agentValue, err := s.agents.GetAgent(ctx, agentID)
		if err != nil {
			return nil, err
		}
		agentsByID[agentID] = *agentValue
	}

	result := make(map[string]roomInputQueueLocation, len(agentsByID))
	for agentID, agentValue := range agentsByID {
		workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
		if workspacePath == "" {
			continue
		}
		result[agentID] = roomInputQueueLocation{
			AgentID: agentID,
			Location: workspacestore.InputQueueLocation{
				Scope:          protocol.InputQueueScopeRoom,
				WorkspacePath:  workspacePath,
				SessionKey:     protocol.BuildRoomAgentSessionKey(contextValue.Conversation.ID, agentID, contextValue.Room.RoomType),
				RoomID:         contextValue.Room.ID,
				ConversationID: contextValue.Conversation.ID,
			},
		}
	}
	return result, nil
}
