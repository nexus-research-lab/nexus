package agent

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const systemOwnerUserID = authctx.SystemUserID

func scopedOwnerUserID(ctx context.Context) (string, bool) {
	return authctx.CurrentUserID(ctx)
}

func effectiveOwnerUserID(ctx context.Context) string {
	return authctx.OwnerUserID(ctx)
}

func isOwnedMainAgent(agentValue *protocol.Agent) bool {
	if agentValue == nil {
		return false
	}
	return agentValue.IsMain
}
