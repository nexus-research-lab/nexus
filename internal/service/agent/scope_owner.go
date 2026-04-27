package agent

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
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

func isOwnedMainAgent(agentValue *protocol.Agent) bool {
	if agentValue == nil {
		return false
	}
	return agentValue.IsMain
}
