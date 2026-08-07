package orchestration

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryProjectionLimitDefenseInDepth(t *testing.T) {
	values := storageProjectionValues(protocol.ExecutionProjectionCollectionLimit)
	overflow := storageProjectionValues(protocol.ExecutionProjectionCollectionLimit + 1)

	t.Run("Create boundary", func(t *testing.T) {
		repository := newRepositoryTestStore(t)
		command := createTestCommand("projection-limit-valid")
		command.Execution.CompletionCriteria = values
		if _, err := repository.Create(context.Background(), command); err != nil {
			t.Fatalf("32 completion criteria rejected: %v", err)
		}
		over := createTestCommand("projection-limit-overflow")
		over.Execution.CompletionCriteria = overflow
		if _, err := repository.Create(context.Background(), over); !errors.Is(
			err,
			ErrProjectionLimitExceeded,
		) {
			t.Fatalf("33 completion criteria error = %v", err)
		}
	})

	t.Run("WritePlan spec collections", func(t *testing.T) {
		tests := []struct {
			name   string
			mutate func(*WritePlanCommand)
		}{
			{name: "work items", mutate: func(command *WritePlanCommand) {
				for len(command.WorkItems) <= protocol.ExecutionProjectionCollectionLimit {
					command.WorkItems = append(command.WorkItems, PlanWorkItem{})
				}
			}},
			{name: "acceptance criteria", mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].Spec.AcceptanceCriteria = overflow
			}},
			{name: "input refs", mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].Spec.InputRefs = overflow
			}},
			{name: "output scopes", mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims = make(
					[]protocol.ExecutionPlanOutputClaim,
					len(overflow),
				)
			}},
			{name: "direct dependencies", mutate: func(command *WritePlanCommand) {
				command.Dependencies = make(
					[]protocol.ExecutionPlanDependency,
					len(overflow),
				)
				for index := range command.Dependencies {
					command.Dependencies[index].WorkItemID = command.WorkItems[1].WorkItem.ID
				}
			}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				repository := newRepositoryTestStore(t)
				suffix := "projection-plan-" + fmt.Sprintf("%d", len(test.name))
				if _, err := repository.Create(
					context.Background(),
					createTestCommand(suffix),
				); err != nil {
					t.Fatal(err)
				}
				command := testPlanCommand(suffix, 1, suffix, "", 1)
				test.mutate(&command)
				if _, err := repository.WritePlan(context.Background(), command); !errors.Is(
					err,
					ErrProjectionLimitExceeded,
				) {
					t.Fatalf("error = %v", err)
				}
			})
		}
	})

	t.Run("Submit direct command", func(t *testing.T) {
		repository := newRepositoryTestStore(t)
		for _, test := range []struct {
			name     string
			refs     []string
			evidence []string
		}{
			{name: "refs", refs: overflow},
			{name: "evidence", evidence: overflow},
		} {
			t.Run(test.name, func(t *testing.T) {
				_, err := repository.Submit(context.Background(), SubmitCommand{
					Submission: protocol.WorkSubmission{
						ID:               "submission-limit",
						ExecutionID:      "execution-limit",
						AssignmentID:     "assignment-limit",
						AttemptID:        "attempt-limit",
						SubmitterAgentID: "agent-limit",
						ResultSummary:    "done",
						ResultRefs:       test.refs,
						Evidence:         test.evidence,
					},
				})
				if !errors.Is(err, ErrProjectionLimitExceeded) {
					t.Fatalf("Submission error = %v", err)
				}
			})
		}
	})

	t.Run("Review direct command", func(t *testing.T) {
		repository := newRepositoryTestStore(t)
		results := make([]protocol.WorkAcceptanceCriterionResult, len(overflow))
		_, err := repository.Review(context.Background(), ReviewCommand{
			Acceptance: protocol.WorkAcceptance{
				ID:              "acceptance-limit",
				ExecutionID:     "execution-limit",
				SubmissionID:    "submission-limit",
				ReviewerID:      "agent-limit",
				Decision:        protocol.WorkAcceptanceRejected,
				CriteriaResults: results,
			},
		})
		if !errors.Is(err, ErrProjectionLimitExceeded) {
			t.Fatalf("Acceptance error = %v", err)
		}
		_, err = repository.Review(context.Background(), ReviewCommand{
			Acceptance: protocol.WorkAcceptance{
				ID:           "acceptance-evidence-limit",
				ExecutionID:  "execution-limit",
				SubmissionID: "submission-limit",
				ReviewerID:   "agent-limit",
				Decision:     protocol.WorkAcceptanceRejected,
				CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
					Criterion: "criterion",
					Evidence:  overflow,
				}},
			},
		})
		if !errors.Is(err, ErrProjectionLimitExceeded) {
			t.Fatalf("Acceptance evidence error = %v", err)
		}
	})

	t.Run("Resume direct command", func(t *testing.T) {
		repository := newRepositoryTestStore(t)
		_, err := repository.Resume(context.Background(), ResumeCommand{
			Resolution: "available",
			Evidence:   overflow,
		})
		if !errors.Is(err, ErrProjectionLimitExceeded) {
			t.Fatalf("Resume error = %v", err)
		}
	})
}

func storageProjectionValues(count int) []string {
	values := make([]string, count)
	for index := range values {
		values[index] = fmt.Sprintf("value-%02d", index)
	}
	return values
}
