package orchestration

import (
	"context"
	"errors"
	"testing"

	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestServiceGoalRevisionSupersedeFencesDanglingExecutionBinding(t *testing.T) {
	var fenced bool
	service := testService(&fakeRepository{
		fenceGoalIdentity: func(
			_ context.Context,
			command orchestrationstore.FenceGoalExecutionIdentityCommand,
		) (bool, error) {
			fenced = command.ExecutionID == "execution-missing" &&
				command.GoalID == "goal-1" &&
				command.GoalObjectiveRevision == 1 &&
				command.SuccessorExecutionID == "execution-successor"
			return true, nil
		},
	})
	snapshot, err := service.SupersedeGoalRevision(context.Background(), GoalRevisionSupersedeInput{
		ExecutionID:              "execution-missing",
		GoalID:                   "goal-1",
		OldGoalObjectiveRevision: 1,
		NewGoalObjectiveRevision: 2,
		SuccessorExecutionID:     "execution-successor",
		CommandID:                "goal-retarget",
		Reason:                   "user changed the Goal objective",
	})
	if err != nil || snapshot != nil || !fenced {
		t.Fatalf("fenced snapshot=%#v err=%v fenced=%t", snapshot, err, fenced)
	}
}

func TestServiceGoalRevisionSupersedeFailsClosedIfMaterializationClaimHasNoRow(t *testing.T) {
	service := testService(&fakeRepository{
		fenceGoalIdentity: func(
			context.Context,
			orchestrationstore.FenceGoalExecutionIdentityCommand,
		) (bool, error) {
			return false, nil
		},
	})
	_, err := service.SupersedeGoalRevision(context.Background(), GoalRevisionSupersedeInput{
		ExecutionID:              "execution-missing",
		GoalID:                   "goal-1",
		OldGoalObjectiveRevision: 1,
		NewGoalObjectiveRevision: 2,
		SuccessorExecutionID:     "execution-successor",
		CommandID:                "goal-retarget",
		Reason:                   "user changed the Goal objective",
	})
	var domainErr *DomainError
	if !errors.As(err, &domainErr) ||
		domainErr.Code != ErrorCodeGoalBindingConflict {
		t.Fatalf("dangling materialization error = %v", err)
	}
}
