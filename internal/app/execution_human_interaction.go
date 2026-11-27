// INPUT: runtime permission Context 的待响应人工交互快照。
// OUTPUT: orchestration 消费侧声明的 PendingHumanInteraction 视图。
// POS: 编排层与 runtime permission 之间的单向适配，避免 orchestration 反向依赖 runtime 包。
package app

import (
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionHumanInteractionReader struct {
	permission *permissionctx.Context
}

func newExecutionHumanInteractionReader(
	permission *permissionctx.Context,
) orchestrationsvc.PendingHumanInteractionReader {
	if permission == nil {
		return nil
	}
	return executionHumanInteractionReader{permission: permission}
}

func (r executionHumanInteractionReader) PendingHumanInteractions(
	dispatchSessionKey string,
) []orchestrationsvc.PendingHumanInteraction {
	snapshots := r.permission.PendingInteractionsForDispatchSession(dispatchSessionKey)
	if len(snapshots) == 0 {
		return nil
	}
	interactions := make([]orchestrationsvc.PendingHumanInteraction, 0, len(snapshots))
	for _, snapshot := range snapshots {
		interactions = append(interactions, orchestrationsvc.PendingHumanInteraction{
			RequestID:         snapshot.RequestID,
			RuntimeSessionKey: snapshot.RuntimeSessionKey,
			AgentID:           snapshot.AgentID,
			AgentRoundID:      snapshot.AgentRoundID,
			ToolName:          snapshot.ToolName,
			CreatedAt:         snapshot.CreatedAt,
		})
	}
	return interactions
}
