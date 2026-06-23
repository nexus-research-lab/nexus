package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) buildAgentDirectory(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) (map[string]string, map[string]*protocol.Agent, error) {
	agentNameByID := make(map[string]string)
	agentByID := make(map[string]*protocol.Agent)
	if contextValue == nil {
		return agentNameByID, agentByID, nil
	}
	memberIDs := make(map[string]struct{})
	for _, member := range contextValue.Members {
		if member.MemberType != protocol.MemberTypeAgent || strings.TrimSpace(member.MemberAgentID) == "" {
			continue
		}
		memberIDs[strings.TrimSpace(member.MemberAgentID)] = struct{}{}
	}
	for _, agentValue := range contextValue.MemberAgents {
		if _, ok := memberIDs[agentValue.AgentID]; !ok {
			continue
		}
		item := agentValue
		agentNameByID[item.AgentID] = item.Name
		agentByID[item.AgentID] = &item
	}
	for agentID := range memberIDs {
		if _, ok := agentByID[agentID]; ok {
			continue
		}
		agentValue, err := s.agents.GetAgent(ctx, agentID)
		if err != nil {
			return nil, nil, err
		}
		agentNameByID[agentValue.AgentID] = agentValue.Name
		agentByID[agentValue.AgentID] = agentValue
	}
	return agentNameByID, agentByID, nil
}
