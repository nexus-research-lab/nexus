package orchestration

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type evidenceTestRepository struct {
	*fakeRepository
	recordCalls []orchestrationstore.RecordEvidenceCommand
	currentID   string
	snapshots   map[string]*protocol.ExecutionSnapshot
}

func (r *evidenceTestRepository) FindCurrent(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.Execution, error) {
	if r.currentID == "" {
		return r.fakeRepository.FindCurrent(ctx, ownerUserID, sessionKey)
	}
	snapshot := r.snapshots[r.currentID]
	if snapshot == nil {
		return nil, nil
	}
	item := snapshot.Execution
	return &item, nil
}

func (r *evidenceTestRepository) GetSnapshot(
	ctx context.Context,
	executionID string,
) (*protocol.ExecutionSnapshot, error) {
	if r.snapshots == nil {
		return r.fakeRepository.GetSnapshot(ctx, executionID)
	}
	return r.snapshots[executionID], nil
}

func (r *evidenceTestRepository) RecordEvidence(
	_ context.Context,
	command orchestrationstore.RecordEvidenceCommand,
) (*protocol.ExecutionSnapshot, error) {
	r.recordCalls = append(r.recordCalls, command)
	snapshot := r.snapshot
	if r.snapshots != nil {
		snapshot = r.snapshots[command.ExecutionID]
	}
	snapshot.Execution.Version++
	if snapshot.Execution.Metadata == nil {
		snapshot.Execution.Metadata = map[string]any{}
	}
	snapshot.Execution.Metadata[command.MetadataKey] = true
	return snapshot, nil
}

func TestServiceRecordPersistenceEvidenceUsesRuntimeActorAndSuppressesDuplicates(t *testing.T) {
	snapshot := executionSnapshot()
	repository := &evidenceTestRepository{
		fakeRepository: &fakeRepository{snapshot: snapshot},
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.RootRoundID = "round-1"
	actor.RuntimeRoundID = "round-1"
	actor.AgentRoundID = "agent-round-1"
	snapshot.Execution.RootRoundID = actor.RootRoundID

	if err := service.RecordPersistenceEvidence(
		context.Background(),
		actor,
		PersistenceEvidenceContextBoundary,
		"runtime:agent-round-1:compact-boundary",
	); err != nil {
		t.Fatal(err)
	}
	if len(repository.recordCalls) != 1 {
		t.Fatalf("record calls = %#v", repository.recordCalls)
	}
	command := repository.recordCalls[0]
	if command.MetadataKey != ExecutionMetadataContextBoundaryEvidence ||
		command.Meta.ActorKind != protocol.ExecutionActorRuntime ||
		command.Meta.CommandID != "runtime:agent-round-1:compact-boundary:record-context_boundary" {
		t.Fatalf("record command = %#v", command)
	}
	if err := service.RecordPersistenceEvidence(
		context.Background(),
		actor,
		PersistenceEvidenceContextBoundary,
		"runtime:agent-round-1:compact-boundary",
	); err != nil {
		t.Fatal(err)
	}
	if len(repository.recordCalls) != 1 {
		t.Fatalf("duplicate boundary wrote again: %#v", repository.recordCalls)
	}
}

func TestServiceRecordPersistenceEvidenceIgnoresUnboundConversationRound(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.RootRoundID = "execution-root"
	repository := &evidenceTestRepository{
		fakeRepository: &fakeRepository{snapshot: snapshot},
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.RootRoundID = "casual-chat-root"
	actor.RuntimeRoundID = "casual-chat-root"
	actor.AgentRoundID = "casual-agent-round"

	if err := service.RecordPersistenceEvidence(
		context.Background(),
		actor,
		PersistenceEvidenceContextBoundary,
		"runtime:casual-agent-round:compact-boundary",
	); err != nil {
		t.Fatal(err)
	}
	if len(repository.recordCalls) != 0 {
		t.Fatalf("casual round wrote background Execution evidence: %#v", repository.recordCalls)
	}
}

func TestServiceRecordPersistenceEvidenceNeverRetargetsPredecessorToSuccessor(t *testing.T) {
	predecessor := executionSnapshot()
	predecessor.Execution.ID = "execution-old"
	predecessor.Execution.RootRoundID = "old-root"
	predecessor.Execution.Status = protocol.ExecutionStatusSuperseded
	successor := executionSnapshot()
	successor.Execution.ID = "execution-new"
	successor.Execution.RootRoundID = "new-root"
	repository := &evidenceTestRepository{
		fakeRepository: &fakeRepository{snapshot: successor},
		currentID:      successor.Execution.ID,
		snapshots: map[string]*protocol.ExecutionSnapshot{
			predecessor.Execution.ID: predecessor,
			successor.Execution.ID:   successor,
		},
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.ExecutionID = predecessor.Execution.ID
	actor.RootRoundID = predecessor.Execution.RootRoundID
	actor.RuntimeRoundID = "old-runtime-round"
	actor.AgentRoundID = "old-agent-round"

	if err := service.RecordPersistenceEvidence(
		context.Background(),
		actor,
		PersistenceEvidenceContextBoundary,
		"runtime:old-agent-round:compact-boundary",
	); err != nil {
		t.Fatal(err)
	}
	if len(repository.recordCalls) != 0 {
		t.Fatalf("late predecessor evidence reached successor: %#v", repository.recordCalls)
	}
	if metadataBool(
		successor.Execution.Metadata,
		ExecutionMetadataContextBoundaryEvidence,
	) {
		t.Fatal("successor was marked by predecessor compact boundary")
	}
}
