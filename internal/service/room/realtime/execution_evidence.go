// INPUT: Room slot runtime system messages and its trusted WorkBinding actor.
// OUTPUT: backend-observed compact boundaries recorded on the shared Execution.
// POS: Room runtime lifecycle to adaptive Goal evidence bridge.
package realtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/service/orchestration"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type executionPersistenceEvidenceRecorder interface {
	RecordPersistenceEvidence(
		context.Context,
		orchestration.ActorContext,
		orchestration.PersistenceEvidenceKind,
		string,
	) error
}

func (e *slotExecution) observeExecutionPersistenceEvidence(
	actor orchestration.ActorContext,
	incoming sdkprotocol.ReceivedMessage,
) {
	if e == nil || e.service == nil || !isExecutionCompactBoundary(incoming) {
		return
	}
	recorder, ok := e.service.executionContext.(executionPersistenceEvidenceRecorder)
	if !ok {
		return
	}
	commandID := fmt.Sprintf(
		"runtime:%s:%s:compact-boundary",
		strings.TrimSpace(e.slot.RuntimeSessionKey),
		strings.TrimSpace(e.slot.AgentRoundID),
	)
	if err := recorder.RecordPersistenceEvidence(
		context.Background(),
		actor,
		orchestration.PersistenceEvidenceContextBoundary,
		commandID,
	); err != nil {
		e.logger.Error(
			"记录 Room Execution context boundary 失败",
			"execution_id", executionIDFromWorkBinding(e.slot.currentWorkBinding()),
			"agent_round_id", e.slot.AgentRoundID,
			"err", err,
		)
	}
}

func isExecutionCompactBoundary(incoming sdkprotocol.ReceivedMessage) bool {
	return incoming.Type == sdkprotocol.MessageTypeSystem &&
		incoming.System != nil &&
		strings.TrimSpace(incoming.System.Subtype) == "compact_boundary"
}
