package room

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
)

func (s *RealtimeService) resolveAgentRuntimeSelection(
	ctx context.Context,
	roundValue *activeRoomRound,
	agentValue *protocol.Agent,
) (runtimeselectionsvc.Selection, error) {
	ownerUserIDs := []string(nil)
	if roundValue != nil {
		ownerUserIDs = append(ownerUserIDs, roundValue.OwnerUserID)
	}
	return runtimeselectionsvc.NewService(s.prefs).Resolve(ctx, runtimeselectionsvc.Request{
		Agent:        agentValue,
		OwnerUserIDs: ownerUserIDs,
	})
}
