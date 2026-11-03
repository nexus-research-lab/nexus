// INPUT: Actual SQL responsibility, checkpoint and takeover transactions.
// OUTPUT: Progress durability, authorization, idempotency and no premature completion proofs.
// POS: SQLite/PostgreSQL checkpoint acceptance without external effects.
package storage_test

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	"reflect"
	"testing"
)

func TestWorkGraphCheckpointIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			repository := repo.NewSQLRepository(driver, openIntegrationDatabase(t, cfg))
			service := svc.NewService(repository)
			actor, _ := runtimeResultFixture(t, repository, "checkpoint")
			before, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
			input := svc.RecordCheckpointInput{ExecutionID: actor.ExecutionID, CommandID: "checkpoint-one", SnapshotRevision: before.Execution.Version, Data: protocol.WorkCheckpointData{Summary: "draft prepared", NextStep: "reconcile receipt before publish", ArtifactRefs: []string{"fixture:draft"}, Effects: []protocol.WorkCheckpointEffect{{Operation: "publish", Outcome: "unknown"}}}}
			saved, err := service.RecordCheckpoint(t.Context(), actor, input)
			if err != nil || saved.Outcome != svc.MutationApplied {
				t.Fatalf("checkpoint %+v %v", saved, err)
			}
			after := saved.Snapshot
			if len(after.Checkpoints) != 1 || len(after.Submissions) != 0 || len(after.Acceptances) != 0 || !reflect.DeepEqual(before.ReadyWorkItemIDs, after.ReadyWorkItemIDs) {
				t.Fatal("checkpoint changed completion or readiness")
			}
			input.SnapshotRevision = after.Execution.Version
			repeated, err := service.RecordCheckpoint(t.Context(), actor, input)
			if err != nil || repeated.Outcome == svc.MutationRejected || len(repeated.Snapshot.Checkpoints) != 1 {
				t.Fatalf("repeated %+v %v", repeated, err)
			}
			input.Data.Summary = "changed intent"
			rejected, err := service.RecordCheckpoint(t.Context(), actor, input)
			if err != nil || rejected.Outcome != svc.MutationRejected {
				t.Fatalf("conflicting intent %+v %v", rejected, err)
			}
			input.CommandID = "bad-receipt"
			input.Data.Effects[0].Outcome = "confirmed"
			rejected, err = service.RecordCheckpoint(t.Context(), actor, input)
			if err != nil || rejected.Outcome != svc.MutationRejected {
				t.Fatal("unproven receipt accepted")
			}
			coordinator := actor
			coordinator.WorkBinding = nil
			coordinator.AgentID = "lead"
			coordinator.Role = svc.ExecutionActorCoordinator
			if err = service.ActivateRuntimeCoordination(t.Context(), coordinator, after); err != nil {
				t.Fatal(err)
			}
			service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
			takeover, err := service.TakeOverWork(t.Context(), coordinator, svc.TakeOverWorkInput{ExecutionID: actor.ExecutionID, SnapshotRevision: after.Execution.Version, CommandID: "takeover-checkpoint", WorkItemID: actor.WorkBinding.WorkItemID, TargetAgentID: "replacement", ReturnToAgentID: "lead", Reason: "explicit recovery after inspecting checkpoint"})
			if err != nil || takeover.Outcome != svc.MutationApplied {
				t.Fatalf("takeover %+v %v", takeover, err)
			}
			current, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
			if len(current.Checkpoints) != 1 || current.Checkpoints[0].AttemptID != actor.WorkBinding.AttemptID || len(current.Submissions) != 0 {
				t.Fatal("takeover lost evidence or invented delivery")
			}
			input.SnapshotRevision = current.Execution.Version
			input.CommandID = "stale-attempt"
			input.Data.Effects[0].Outcome = "unknown"
			stale, err := service.RecordCheckpoint(t.Context(), actor, input)
			if err == nil && stale.Outcome != svc.MutationRejected {
				t.Fatal("old owner wrote after takeover")
			}
			// A fresh service and queue-bound successor see the same immutable checkpoint.
			var assignment protocol.WorkAssignment
			var attempt protocol.WorkAttempt
			for _, a := range current.Assignments {
				if a.OwnerAgentID == "replacement" {
					assignment = a
				}
			}
			for _, a := range current.Attempts {
				if a.AssignmentID == assignment.ID {
					attempt = a
				}
			}
			successor := actor
			successor.AgentID = "replacement"
			successor.WorkBinding = &protocol.ExecutionWorkBinding{ExecutionID: actor.ExecutionID, PlanID: assignment.PlanID, WorkItemID: assignment.WorkItemID, SpecID: assignment.SpecID, AssignmentID: assignment.ID, AttemptID: attempt.ID, DispatchID: attempt.DispatchID}
			resumed, err := svc.NewService(repository).GetSnapshot(t.Context(), successor, actor.ExecutionID)
			if err != nil || len(resumed.Checkpoints) != 1 || resumed.Checkpoints[0].Effects[0].Outcome != "unknown" {
				t.Fatalf("resume %+v %v", resumed, err)
			}
		})
	}
}
