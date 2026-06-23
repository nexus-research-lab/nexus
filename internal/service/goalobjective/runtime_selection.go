package goalobjective

import (
	"cmp"
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type agentLookup interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

type roomContextLookup interface {
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
}

func (s *Service) resolveConversationRuntimeSelection(ctx context.Context, request Request) (string, string, bool, error) {
	sessionKey := strings.TrimSpace(request.SessionKey)
	if sessionKey == "" {
		return "", "", false, nil
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured {
		return "", "", false, nil
	}
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if s.agents == nil || strings.TrimSpace(parsed.AgentID) == "" {
			return "", "", false, nil
		}
		agentValue, err := s.agents.GetAgent(ctx, parsed.AgentID)
		if err != nil || agentValue == nil {
			return "", "", false, err
		}
		return s.runtimeSelectionForAgent(ctx, request.OwnerUserID, agentValue)
	case protocol.SessionKeyKindRoom:
		if s.rooms == nil || strings.TrimSpace(parsed.ConversationID) == "" {
			return "", "", false, nil
		}
		contextValue, err := s.rooms.GetConversationContext(ctx, parsed.ConversationID)
		if err != nil || contextValue == nil {
			return "", "", false, err
		}
		agentValue, err := s.goalConversationTargetAgent(ctx, contextValue)
		if err != nil || agentValue == nil {
			return "", "", false, err
		}
		return s.runtimeSelectionForAgent(ctx, request.OwnerUserID, agentValue)
	default:
		return "", "", false, nil
	}
}

func (s *Service) goalConversationTargetAgent(ctx context.Context, contextValue *protocol.ConversationContextAggregate) (*protocol.Agent, error) {
	if contextValue == nil {
		return nil, nil
	}
	targetAgentID := goalConversationTargetAgentID(contextValue)
	if targetAgentID == "" {
		return nil, nil
	}
	if agentValue := findConversationAgent(contextValue.MemberAgents, targetAgentID); agentValue != nil {
		return agentValue, nil
	}
	if s.agents == nil {
		return nil, nil
	}
	return s.agents.GetAgent(ctx, targetAgentID)
}

func goalConversationTargetAgentID(contextValue *protocol.ConversationContextAggregate) string {
	if contextValue == nil {
		return ""
	}
	agentIDs := make(map[string]struct{}, len(contextValue.MemberAgents))
	for _, agentValue := range contextValue.MemberAgents {
		agentID := strings.TrimSpace(agentValue.AgentID)
		if agentID != "" {
			agentIDs[agentID] = struct{}{}
		}
	}
	if len(agentIDs) == 1 {
		for agentID := range agentIDs {
			return agentID
		}
	}
	if !contextValue.Room.HostAutoReplyEnabled {
		return ""
	}
	hostAgentID := strings.TrimSpace(contextValue.Room.HostAgentID)
	if hostAgentID == "" {
		return ""
	}
	if _, ok := agentIDs[hostAgentID]; !ok {
		return ""
	}
	return hostAgentID
}

func findConversationAgent(agents []protocol.Agent, agentID string) *protocol.Agent {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil
	}
	for index := range agents {
		if strings.TrimSpace(agents[index].AgentID) == agentID {
			return &agents[index]
		}
	}
	return nil
}

func (s *Service) runtimeSelectionForAgent(ctx context.Context, ownerUserID string, agentValue *protocol.Agent) (string, string, bool, error) {
	if agentValue == nil {
		return "", "", false, nil
	}
	provider := strings.TrimSpace(agentValue.Options.Provider)
	model := strings.TrimSpace(agentValue.Options.Model)
	if provider != "" && model != "" {
		return provider, model, true, nil
	}
	ownerUserID = cmp.Or(strings.TrimSpace(ownerUserID), strings.TrimSpace(agentValue.OwnerUserID))
	defaultProvider, defaultModel, err := s.defaultAgentRuntimeSelection(ctx, ownerUserID)
	if err != nil {
		return "", "", false, err
	}
	if defaultProvider != "" && defaultModel != "" {
		return defaultProvider, defaultModel, true, nil
	}
	return "", "", false, nil
}

func (s *Service) defaultAgentRuntimeSelection(ctx context.Context, ownerUserID string) (string, string, error) {
	if s.prefs == nil {
		return "", "", nil
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return "", "", nil
	}
	prefs, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return "", "", err
	}
	provider := strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
	model := strings.TrimSpace(prefs.DefaultAgentOptions.Model)
	if provider == "" || model == "" {
		return "", "", nil
	}
	return provider, model, nil
}
