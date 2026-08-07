package orchestration

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryRecordEvidencePersistsFlagAndIsCommandIdempotent(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("evidence"))
	if err != nil {
		t.Fatal(err)
	}
	command := RecordEvidenceCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		MetadataKey:              "goal_promotion_context_boundary_evidence",
		Meta:                     testMeta("evidence-context-boundary"),
	}
	recorded, err := repository.RecordEvidence(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Execution.Version != 2 ||
		recorded.Execution.Metadata[command.MetadataKey] != true {
		t.Fatalf("recorded Execution = %#v", recorded.Execution)
	}
	replayed, err := repository.RecordEvidence(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.Version != 2 ||
		replayed.Execution.Metadata[command.MetadataKey] != true {
		t.Fatalf("replayed Execution = %#v", replayed.Execution)
	}
	var eventType string
	var actorKind string
	var evidenceKey string
	if err = repository.db.QueryRow(`
SELECT event_type, actor_kind, json_extract(payload_json, '$.evidence_key')
FROM execution_events
WHERE execution_id = ? AND command_id = ?`,
		snapshot.Execution.ID,
		command.Meta.CommandID,
	).Scan(&eventType, &actorKind, &evidenceKey); err != nil {
		t.Fatal(err)
	}
	if eventType != string(protocol.ExecutionEventEvidenceRecorded) ||
		actorKind != string(protocol.ExecutionActorSystem) ||
		evidenceKey != command.MetadataKey {
		t.Fatalf("event = %q/%q/%q", eventType, actorKind, evidenceKey)
	}
}
