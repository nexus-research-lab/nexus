package goal

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestManagedGoalCompletionRequiresCurrentRoundAlignedAudit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetExecutionGoalCompletionReadiness(
		&fakeExecutionGoalCompletionReadiness{},
	)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:alignment",
		Objective:  "Ship a verified report",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-alignment",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataCompletionCriteria: []string{
				"report delivered",
				"verification passed",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	completion := protocol.CompleteGoalRequest{
		RoundID:                   "round-final",
		AgentID:                   "agent-1",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	}
	if _, err = service.CompleteByModel(
		ctx,
		created.ID,
		completion,
	); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("completion without alignment error = %v, want ErrGoalInvalidState", err)
	}

	report := protocol.ObjectiveAlignmentReport{
		Decision: protocol.ObjectiveAlignmentAligned,
		CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{
			{
				Criterion: "verification passed",
				Status:    protocol.ObjectiveAlignmentCriterionSatisfied,
				Evidence: []protocol.ObjectiveAlignmentEvidence{{
					Ref:   "command:make-check",
					Claim: "the complete verification suite passed",
				}},
			},
			{
				Criterion: "report delivered",
				Status:    protocol.ObjectiveAlignmentCriterionSatisfied,
				Evidence: []protocol.ObjectiveAlignmentEvidence{{
					Ref:   "file:/workspace/report.md",
					Claim: "the requested report exists at the delivery path",
				}},
			},
		},
		Summary: "Every authoritative completion criterion is proven.",
	}
	record, err := service.AuditObjectiveAlignmentByModel(
		ctx,
		created.ID,
		protocol.AuditGoalObjectiveAlignmentRequest{
			Report:                    report,
			RoundID:                   completion.RoundID,
			AgentID:                   completion.AgentID,
			ExpectedObjectiveRevision: created.ObjectiveRevision(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if record.Report.CriteriaResults[0].Criterion != "report delivered" ||
		record.Report.CriteriaResults[1].Criterion != "verification passed" {
		t.Fatalf("record criteria = %#v, want authoritative order", record.Report.CriteriaResults)
	}

	completed, err := service.CompleteByModel(ctx, created.ID, completion)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("status = %q, want complete", completed.Status)
	}
	if len(repo.events) < 3 ||
		repo.events[len(repo.events)-2].EventType != "objective_alignment_audited" ||
		repo.events[len(repo.events)-1].EventType != "completed" {
		t.Fatalf("events = %#v, want audit before completion", repo.events)
	}
}

func TestManagedGoalCompletionRejectsNonAlignedOrStaleAudit(t *testing.T) {
	for _, test := range []struct {
		name            string
		decision        protocol.ObjectiveAlignmentDecision
		status          protocol.ObjectiveAlignmentCriterionStatus
		auditRound      string
		completionRound string
	}{
		{
			name:            "confirmed gap",
			decision:        protocol.ObjectiveAlignmentNotAligned,
			status:          protocol.ObjectiveAlignmentCriterionUnsatisfied,
			auditRound:      "round-final",
			completionRound: "round-final",
		},
		{
			name:            "stale round",
			decision:        protocol.ObjectiveAlignmentAligned,
			status:          protocol.ObjectiveAlignmentCriterionSatisfied,
			auditRound:      "round-before",
			completionRound: "round-final",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			service.SetExecutionGoalCompletionReadiness(
				&fakeExecutionGoalCompletionReadiness{},
			)
			created, err := service.Create(
				context.Background(),
				protocol.CreateGoalRequest{
					SessionKey: "agent:nexus:ws:dm:" + strings.ReplaceAll(test.name, " ", "-"),
					Objective:  "Ship report",
					Metadata: map[string]any{
						protocol.GoalMetadataExecutionID: "execution-" + strings.ReplaceAll(test.name, " ", "-"),
						protocol.GoalMetadataExecutionBindingState: string(
							protocol.GoalExecutionBindingStateConfirmed,
						),
						protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginAdaptiveInitial),
						protocol.GoalMetadataCompletionCriteria: []string{
							"report accepted",
						},
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			result := protocol.ObjectiveAlignmentCriterionResult{
				Criterion: "report accepted",
				Status:    test.status,
			}
			if test.status == protocol.ObjectiveAlignmentCriterionSatisfied {
				result.Evidence = []protocol.ObjectiveAlignmentEvidence{{
					Ref:   "review:accepted",
					Claim: "the final report was accepted",
				}}
			} else {
				result.Gap = "the report has not been accepted"
			}
			if _, err = service.AuditObjectiveAlignmentByModel(
				context.Background(),
				created.ID,
				protocol.AuditGoalObjectiveAlignmentRequest{
					Report: protocol.ObjectiveAlignmentReport{
						Decision:        test.decision,
						CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{result},
						Summary:         "current alignment state",
					},
					RoundID:                   test.auditRound,
					AgentID:                   "agent-1",
					ExpectedObjectiveRevision: created.ObjectiveRevision(),
				},
			); err != nil {
				t.Fatal(err)
			}
			if _, err = service.CompleteByModel(
				context.Background(),
				created.ID,
				protocol.CompleteGoalRequest{
					RoundID:                   test.completionRound,
					AgentID:                   "agent-1",
					ExpectedObjectiveRevision: created.ObjectiveRevision(),
				},
			); !errors.Is(err, ErrGoalInvalidState) {
				t.Fatalf("completion error = %v, want ErrGoalInvalidState", err)
			}
		})
	}
}

func TestManagedGoalCompletionToolMissCannotBypassObjectiveAlignment(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetExecutionGoalCompletionReadiness(
		&fakeExecutionGoalCompletionReadiness{},
	)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:alignment-miss",
		Objective:  "Ship a verified report",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-alignment-miss",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataCompletionCriteria: []string{
				"report verified",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionToolMiss(
		ctx,
		created.ID,
		"round-first",
		"first miss",
	); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionToolMiss(
		ctx,
		created.ID,
		"round-final",
		"second miss",
	); err != nil {
		t.Fatal(err)
	}

	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusActive {
		t.Fatalf("status = %q, want active without aligned audit", current.Status)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_suppressed" ||
		!strings.Contains(got.Payload["reason"].(string), "objective alignment audit") {
		t.Fatalf("last event = %#v, want alignment suppression", got)
	}
}

func TestManagedGoalCompletionToolMissCanUseSameRoundAlignedAudit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetExecutionGoalCompletionReadiness(
		&fakeExecutionGoalCompletionReadiness{},
	)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:aligned-miss",
		Objective:  "Ship a verified report",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-aligned-miss",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataCompletionCriteria: []string{
				"report verified",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionToolMiss(
		ctx,
		created.ID,
		"round-first",
		"first miss",
	); err != nil {
		t.Fatal(err)
	}
	const finalRound = "round-final"
	if _, err = service.AuditObjectiveAlignmentByModel(
		ctx,
		created.ID,
		protocol.AuditGoalObjectiveAlignmentRequest{
			Report: protocol.ObjectiveAlignmentReport{
				Decision: protocol.ObjectiveAlignmentAligned,
				CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{{
					Criterion: "report verified",
					Status:    protocol.ObjectiveAlignmentCriterionSatisfied,
					Evidence: []protocol.ObjectiveAlignmentEvidence{{
						Ref:   "command:make-check",
						Claim: "the report verification passed",
					}},
				}},
				Summary: "The report is verified.",
			},
			RoundID:                   finalRound,
			AgentID:                   "agent-1",
			ExpectedObjectiveRevision: created.ObjectiveRevision(),
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionToolMiss(
		ctx,
		created.ID,
		finalRound,
		"second miss",
	); err != nil {
		t.Fatal(err)
	}

	completed, err := repo.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("status = %q, want system completion backed by aligned audit", completed.Status)
	}
}
