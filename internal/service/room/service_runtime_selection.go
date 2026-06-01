package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) resolveAgentRuntimeSelection(
	ctx context.Context,
	roundValue *activeRoomRound,
	agentValue *protocol.Agent,
) (string, string, error) {
	if agentValue == nil {
		return "", "", nil
	}
	provider := strings.TrimSpace(agentValue.Options.Provider)
	model := strings.TrimSpace(agentValue.Options.Model)
	if provider != "" && model != "" {
		return provider, model, nil
	}
	defaultProvider, defaultModel, err := s.preferenceRuntimeSelection(ctx, roundValue, agentValue)
	if err != nil || defaultProvider != "" || defaultModel != "" {
		return defaultProvider, defaultModel, err
	}
	return provider, model, nil
}

func (s *RealtimeService) preferenceRuntimeSelection(
	ctx context.Context,
	roundValue *activeRoomRound,
	agentValue *protocol.Agent,
) (string, string, error) {
	if s.prefs == nil {
		return "", "", nil
	}
	ownerUserID := ""
	if currentUserID, ok := authctx.CurrentUserID(ctx); ok {
		ownerUserID = currentUserID
	}
	if ownerUserID == "" && roundValue != nil {
		ownerUserID = strings.TrimSpace(roundValue.OwnerUserID)
	}
	if ownerUserID == "" && agentValue != nil {
		ownerUserID = strings.TrimSpace(agentValue.OwnerUserID)
	}
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
