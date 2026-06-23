package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func inputQueueTargetAgentIDs(item protocol.InputQueueItem) []string {
	targets := make([]string, 0, len(item.TargetAgentIDs)+1)
	seen := make(map[string]struct{}, len(item.TargetAgentIDs)+1)
	appendTarget := func(agentID string) {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			return
		}
		if _, exists := seen[agentID]; exists {
			return
		}
		seen[agentID] = struct{}{}
		targets = append(targets, agentID)
	}
	appendTarget(item.AgentID)
	for _, agentID := range item.TargetAgentIDs {
		appendTarget(agentID)
	}
	return targets
}

func inputQueueLocationAgentID(location workspacestore.InputQueueLocation) string {
	return strings.TrimSpace(protocol.ParseSessionKey(location.SessionKey).AgentID)
}

func inputQueueLocationKey(location workspacestore.InputQueueLocation) string {
	return strings.TrimSpace(location.WorkspacePath) + "::" + strings.TrimSpace(location.SessionKey)
}

func contextWithQueueOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return ctx
	}
	if _, ok := authctx.CurrentUserID(ctx); ok {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: ownerUserID,
		Role:   authctx.RoleOwner,
	})
}
