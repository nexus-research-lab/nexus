package agent

import (
	"context"

	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
)

const systemOwnerUserID = authsvc.SystemUserID

func scopedOwnerUserID(ctx context.Context) (string, bool) {
	return authsvc.CurrentUserID(ctx)
}

func effectiveOwnerUserID(ctx context.Context) string {
	if ownerUserID, ok := scopedOwnerUserID(ctx); ok {
		return ownerUserID
	}
	return systemOwnerUserID
}

func isOwnedMainAgent(agentValue *Agent) bool {
	if agentValue == nil {
		return false
	}
	return agentValue.IsMain
}
