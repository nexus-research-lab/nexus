// INPUT: DM runtime system messages and the round's trusted Execution actor.
// OUTPUT: backend-observed compact boundaries recorded on the current Execution.
// POS: DM runtime lifecycle to adaptive Goal evidence bridge.
package dm

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

func (r *roundRunner) observeExecutionPersistenceEvidence(
	actor orchestration.ActorContext,
	incoming sdkprotocol.ReceivedMessage,
) {
	if r == nil || r.service == nil || !isCompactBoundaryMessage(incoming) {
		return
	}
	recorder, ok := r.service.executionContext.(executionPersistenceEvidenceRecorder)
	if !ok {
		return
	}
	commandID := fmt.Sprintf(
		"runtime:%s:%s:compact-boundary",
		strings.TrimSpace(r.sessionKey),
		strings.TrimSpace(r.agentRoundID),
	)
	if err := recorder.RecordPersistenceEvidence(
		context.Background(),
		actor,
		orchestration.PersistenceEvidenceContextBoundary,
		commandID,
	); err != nil {
		r.service.loggerFor(context.Background()).Error(
			"记录 DM Execution context boundary 失败",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
			"agent_round_id", r.agentRoundID,
			"err", err,
		)
	}
}

func isCompactBoundaryMessage(incoming sdkprotocol.ReceivedMessage) bool {
	return incoming.Type == sdkprotocol.MessageTypeSystem &&
		incoming.System != nil &&
		strings.TrimSpace(incoming.System.Subtype) == "compact_boundary"
}
