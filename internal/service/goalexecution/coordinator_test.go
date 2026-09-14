package goalexecution

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type retargetGoals struct {
	lifecycleService
	item     protocol.Goal
	commands []goalsvc.ObjectiveRetargetCommand
	calls    *[]string
}

func (f *retargetGoals) PrepareObjectiveRetarget(_ context.Context, c goalsvc.ObjectiveRetargetCommand) (*protocol.Goal, error) {
	*f.calls = append(*f.calls, "prepare")
	f.commands = append(f.commands, c)
	f.item = protocol.Goal{ID: c.Goal.ID, Metadata: map[string]any{protocol.GoalMetadataObjectiveTransition: map[string]any{
		"transition_id": c.TransitionID, "command_id": c.CommandID, "phase": "prepared", "old_revision": int64(1), "new_revision": int64(2), "old_execution_id": "old", "successor_execution_id": c.SuccessorExecutionID, "target_objective": c.Objective,
	}}}
	return &f.item, nil
}
func (f *retargetGoals) FenceObjectiveRetargetPredecessor(_ context.Context, _, _, _ string) (*protocol.Goal, error) {
	*f.calls = append(*f.calls, "fence")
	f.item.Metadata[protocol.GoalMetadataObjectiveTransition].(map[string]any)["old_execution_fenced"] = true
	return &f.item, nil
}
func (f *retargetGoals) CommitObjectiveRetarget(_ context.Context, _, _ string) (*protocol.Goal, error) {
	*f.calls = append(*f.calls, "commit")
	return &f.item, nil
}

type retargetExecutions struct {
	executionService
	calls  *[]string
	inputs []orchestrationsvc.GoalRevisionSupersedeInput
	fail   bool
}

func (f *retargetExecutions) SupersedeGoalRevision(_ context.Context, input orchestrationsvc.GoalRevisionSupersedeInput) (*protocol.ExecutionSnapshot, error) {
	*f.calls = append(*f.calls, "supersede")
	f.inputs = append(f.inputs, input)
	if f.fail {
		f.fail = false
		return nil, errors.New("temporary failure")
	}
	return nil, nil
}

func TestRetargetRetryKeepsIdentityAndFencesBeforeCommit(t *testing.T) {
	var calls []string
	goals := &retargetGoals{calls: &calls}
	executions := &retargetExecutions{calls: &calls, fail: true}
	coordinator := NewExplicitExecutionCoordinator(goals, executions)
	command := goalsvc.ObjectiveRetargetCommand{Goal: protocol.Goal{ID: "goal-a"}, Objective: "新目标"}
	if _, err := coordinator.RetargetGoalObjective(context.Background(), command); err == nil {
		t.Fatal("首次失败不能提交目标修订")
	}
	if _, err := coordinator.RetargetGoalObjective(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"prepare", "supersede", "prepare", "supersede", "fence", "commit"}) {
		t.Fatalf("calls=%v", calls)
	}
	if goals.commands[0].TransitionID == "" || !reflect.DeepEqual(goals.commands[0], goals.commands[1]) || !reflect.DeepEqual(executions.inputs[0], executions.inputs[1]) {
		t.Fatal("重试必须复用相同修订、命令和后继 Execution 身份")
	}
}

func TestRetargetRejectsAnotherOwnerBeforeMutation(t *testing.T) {
	var calls []string
	coordinator := NewExplicitExecutionCoordinator(&retargetGoals{calls: &calls}, &retargetExecutions{calls: &calls})
	_, err := coordinator.RetargetGoalObjective(context.Background(), goalsvc.ObjectiveRetargetCommand{
		Goal: protocol.Goal{ID: "goal-a", Metadata: map[string]any{protocol.GoalMetadataOwnerUserID: "owner-a"}}, Objective: "新目标", Source: protocol.GoalUpdateSourceUser, OwnerUserID: "owner-b",
	})
	if !errors.Is(err, goalsvc.ErrGoalForbidden) || len(calls) != 0 {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
}

type promotionGoals struct {
	current *protocol.Goal
	creates int
}

func (f *promotionGoals) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return f.current, nil
}
func (f *promotionGoals) Create(_ context.Context, r protocol.CreateGoalRequest) (*protocol.Goal, error) {
	f.creates++
	f.current = &protocol.Goal{ID: "promoted", Objective: r.Objective, Metadata: r.Metadata}
	// 模拟并发创建胜出，调用方必须重新读取并复用已存在的绑定。
	return nil, goalsvc.ErrGoalConflict
}
func TestPromotionConflictRetryReusesPendingGoal(t *testing.T) {
	goals := &promotionGoals{}
	gateway := NewExecutionPromotionGateway(config.Config{GoalEnabled: true, GoalAutoContinueEnabled: true}, goals)
	request := orchestrationsvc.GoalPromotionRequest{CommandID: "promote-a", Snapshot: &protocol.ExecutionSnapshot{Execution: protocol.Execution{ID: "execution-a", SessionKey: "session-a", Objective: "目标"}}}
	first, err := gateway.PromoteExecution(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gateway.PromoteExecution(context.Background(), request)
	if err != nil || first != second || first.GoalID != "promoted" || goals.creates != 1 {
		t.Fatalf("first=%+v second=%+v creates=%d err=%v", first, second, goals.creates, err)
	}
	if protocol.GoalExecutionBindingStateFromGoal(*goals.current) != protocol.GoalExecutionBindingStatePending {
		t.Fatal("SQL 绑定前不能确认 Goal")
	}
	request.Snapshot.Execution.ID = "execution-b"
	if _, err := gateway.PromoteExecution(context.Background(), request); !errors.Is(err, orchestrationsvc.ErrGoalPromotionConflict) {
		t.Fatalf("其他 Execution 不得复用 Goal: %v", err)
	}
}
