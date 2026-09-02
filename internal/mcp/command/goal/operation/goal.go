// INPUT: Goal read/create/retarget/update intent 与当前 runtime authority。
// OUTPUT: 当前 Goal 投影、持久 mutation 结果、暂停恢复指引与 exact Goal/revision capability fence。
// POS: Goal command 生命周期操作的统一实现。
package operation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func getGoal(svc contract.Service, sctx contract.Context) command.Operation {
	return readGoalOperation("get_goal", "Get the current goal for this thread, including the authoritative objective revision and completion criteria, status, budgets, token and elapsed-time usage, and remaining token budget.", svc, sctx)
}

func readGoalOperation(name string, description string, svc contract.Service, sctx contract.Context) command.Operation {
	return command.Operation{
		Name:        name,
		Description: description,
		SearchHint:  searchHintGetGoal,
		InputSchema: objectSchema(map[string]any{}),
		Annotations: &command.OperationAnnotations{
			ReadOnlyHint: true,
			ReadOnly:     true,
		},
		Handler: func(ctx context.Context, input map[string]any) (command.Result, error) {
			item, err := svc.CurrentOptional(ctx, sctx.CurrentSessionKey)
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult("current goal loaded", goalPayload(item)), nil
		},
	}
}

type createGoalInput struct {
	Objective   string `json:"objective"`
	TokenBudget *int64 `json:"token_budget,omitempty"`
}

func createGoal(svc contract.Service, sctx contract.Context) command.Operation {
	return command.Operation{
		Name:        "create_goal",
		Description: createGoalDescription(sctx.CurrentSessionKey),
		SearchHint:  searchHintCreateGoal,
		InputSchema: objectSchema(map[string]any{
			"objective":    stringProperty("Required. The complete, concrete, execution-ready objective, including all confirmed material requirements. Never use a broad or placeholder objective while clarification is still needed. This starts a new active goal only when no goal is currently defined; if a goal already exists, this operation fails."),
			"token_budget": integerProperty("Optional positive token budget for the new active goal."),
		}, "objective"),
		Handler: func(ctx context.Context, input map[string]any) (command.Result, error) {
			var parsed createGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			if sctx.PlanMode {
				return planModeGoalMutationResult("create_goal"), nil
			}
			item, err := svc.Create(goalsvc.WithActiveGoalContinuationSuppressed(ctx), protocol.CreateGoalRequest{
				SessionKey:  sctx.CurrentSessionKey,
				Objective:   parsed.Objective,
				TokenBudget: parsed.TokenBudget,
				CreatedBy:   "model",
				RoundID:     sctx.CurrentRoundID,
				OwnerUserID: sctx.OwnerUserID,
				AgentID:     sctx.CurrentAgentID,
				Metadata: map[string]any{
					"created_via": "goal_command",
				},
			})
			if err != nil {
				return createGoalErrorResult(err), nil
			}
			sctx.StoreGoalMutationAuthority(*item)
			return appliedResult("goal created", goalMutationPayload(item)), nil
		},
	}
}

const createGoalBaseDescription = "Create one standalone active Goal only after explicit user or system Goal intent and a complete execution-ready objective. Do not create a broad placeholder while material clarification is still required. Set token_budget only from an explicit budget. This operation never creates, reserves, or binds an Execution. If a transient WorkGraph is already current, use promote_execution_to_goal with activation_reason=persistence_requested instead. When no WorkGraph exists and one is also required, wait for this operation to succeed before prepare_plan_execution with goal_binding=current; never launch them in parallel. The operation fails when a current Goal exists; explicit objective correction uses retarget_goal on that same Goal."

func createGoalDescription(_ string) string {
	return createGoalBaseDescription
}

const createGoalConflictMessage = "cannot create a new goal because this thread already has a goal; if the user explicitly corrected its objective, use retarget_goal on that same goal"

func createGoalErrorResult(err error) command.Result {
	if isGoalConflictError(err) {
		return errorResultText(createGoalConflictMessage)
	}
	return errorResult(err)
}

func isGoalConflictError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "already has a goal") ||
		strings.Contains(message, "current goal already exists")
}

type retargetGoalInput struct {
	Objective string `json:"objective"`
}

const retargetGoalDescription = "Retarget the existing current goal only when the user explicitly corrects or replaces its objective.\n" +
	"Keep the same Goal identity and accumulated usage; never complete the old Goal and create another for a correction. " +
	"For a confirmed Goal+WorkGraph binding, this reserves the successor Execution identity and enters awaiting_plan; it does not author the successor WorkGraph. Follow the returned next action to prepare and materialize that Plan. A shared Room Goal may be retargeted only by its assigned lead."

func retargetGoal(svc contract.Service, sctx contract.Context) command.Operation {
	return command.Operation{
		Name:        "retarget_goal",
		Description: retargetGoalDescription,
		SearchHint:  searchHintRetargetGoal,
		InputSchema: objectSchema(map[string]any{
			"objective": stringProperty("Required. The replacement objective explicitly requested by the user for the existing current goal."),
		}, "objective"),
		Handler: func(ctx context.Context, input map[string]any) (command.Result, error) {
			var parsed retargetGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			if sctx.PlanMode {
				return planModeGoalMutationResult("retarget_goal"), nil
			}
			current, expectedRevision, err := currentGoalForRetarget(ctx, svc, sctx)
			if err != nil {
				if isGoalNotFoundError(err) {
					return errorResultText("cannot retarget goal because this thread has no current goal"), nil
				}
				return errorResult(err), nil
			}
			item, err := svc.RetargetByModel(ctx, sctx.CurrentSessionKey, protocol.RetargetGoalRequest{
				Objective:                 parsed.Objective,
				RoundID:                   sctx.CurrentRoundID,
				AgentID:                   sctx.CurrentAgentID,
				ExpectedGoalID:            current.ID,
				ExpectedObjectiveRevision: expectedRevision,
			})
			if err != nil {
				if isGoalNotFoundError(err) {
					return errorResultText("cannot retarget goal because this thread has no current goal"), nil
				}
				return errorResult(err), nil
			}
			sctx.StoreGoalMutationAuthority(*item)
			return appliedResult("goal retargeted", goalMutationPayload(item)), nil
		},
	}
}

type updateGoalInput struct {
	Status      string `json:"status"`
	BlockerID   string `json:"blocker_id,omitempty"`
	Reason      string `json:"reason,omitempty"`
	NeededInput string `json:"needed_input,omitempty"`
}

const updateGoalDescription = "Update the terminal status of an existing current Goal. Never use this operation to create, set, or change a Goal objective.\n" +
	"Use it only to mark the Goal complete or blocked; when explicit Goal intent exists but get_goal returns no current Goal, create_goal is the only creation path. Objective correction of an existing Goal uses retarget_goal, and pause, resume, budget and usage states belong to the user or system. If the current Goal is paused, ask the user to click the Play control labeled 「继续」 on the right side of the Goal status bar directly above this conversation's message composer. Nexus then schedules a new Goal continuation to perform the remaining audit and completion work.\n" +
	"Complete requires the objective to be achieved with no required work remaining. Only a Goal whose managed WorkGraph binding is confirmed also requires an aligned Objective Alignment report for the current revision and round, plus backend WorkGraph readiness; Goal-only and reserved Goals do not. Room readiness remains independently enforced.\n" +
	"A rejected completion can return a domain-qualified nextAction. Follow it exactly: Goal audit_objective_alignment supplies missing Goal evidence, while Execution get_execution resumes unfinished WorkGraph responsibility.\n" +
	"Blocked requires a stable blocker_id, concrete reason, and needed_input so restart recovery and the user-facing audit explain the unblock path. Reuse the same blocker_id only for the same condition. Model policy still requires that blocker to persist for at least three consecutive Goal turns; the backend preserves its exact identity but does not infer provider turns. A shared Room Goal may be updated only by its assigned lead."

const updateGoalStatusDescription = "Required. Set to complete only when the objective is achieved and no required work remains. Set to blocked only after the same blocker has repeated for at least three consecutive goal turns and progress is impossible without user input or external unblock."

func updateGoal(svc contract.Service, sctx contract.Context) command.Operation {
	return command.Operation{
		Name:        "update_goal",
		Description: updateGoalDescription,
		SearchHint:  searchHintUpdateGoal,
		InputSchema: objectSchema(map[string]any{
			"status":       enumStringProperty(updateGoalStatusDescription, string(protocol.GoalStatusComplete), string(protocol.GoalStatusBlocked)),
			"blocker_id":   stringProperty("Required when status=blocked. Stable identifier for this exact blocking condition; reuse it only while the same blocker persists."),
			"reason":       stringProperty("Required when status=blocked. The concrete blocker identity and why autonomous progress cannot continue."),
			"needed_input": stringProperty("Required when status=blocked. The exact user input, permission, or external state change that will unblock the Goal."),
		}, "status"),
		Handler: func(ctx context.Context, input map[string]any) (command.Result, error) {
			expectedRevision := sctx.ExpectedGoalObjectiveRevision()
			var parsed updateGoalInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			status := protocol.GoalStatus(strings.TrimSpace(parsed.Status))
			if status != protocol.GoalStatusComplete && status != protocol.GoalStatusBlocked {
				return errorResult(fmt.Errorf("the Goal update operation can only mark the existing goal complete or blocked; pause, resume, budget-limited, and usage-limited status changes are controlled by the user or system")), nil
			}
			if status == protocol.GoalStatusBlocked &&
				(strings.TrimSpace(parsed.BlockerID) == "" || strings.TrimSpace(parsed.Reason) == "" || strings.TrimSpace(parsed.NeededInput) == "") {
				return errorResult(fmt.Errorf("blocked status requires blocker_id, reason, and needed_input so the Goal has a durable recovery path")), nil
			}
			if sctx.PlanMode {
				return planModeGoalMutationResult("update_goal"), nil
			}
			current, err := currentGoalForMutation(ctx, svc, sctx, expectedRevision)
			if err != nil {
				return updateGoalCurrentErrorResult(err), nil
			}
			item, err := updateGoalStatus(ctx, svc, current.ID, status, parsed.BlockerID, parsed.Reason, parsed.NeededInput, sctx.CurrentRoundID, sctx.CurrentAgentID, expectedRevision)
			if err != nil {
				if result, ok := pausedGoalRecoveryResult(err, current); ok {
					return result, nil
				}
				if status == protocol.GoalStatusComplete {
					return goalCompletionErrorResult(err), nil
				}
				return errorResult(err), nil
			}
			if status == protocol.GoalStatusComplete {
				return appliedResult("goal marked complete", goalCompletionMutationPayload(item)), nil
			}
			return appliedResult("goal marked blocked", goalMutationPayload(item)), nil
		},
	}
}

func goalCompletionErrorResult(err error) command.Result {
	switch {
	case errors.Is(err, goalsvc.ErrGoalExecutionNotReady):
		return errorResultWithNextAction(err, map[string]any{
			"domain":    command.DomainExecution,
			"operation": "get_execution",
			"reason":    "inspect the bound Execution and finish every required responsibility; a current same-round Goal audit remains valid, then retry Goal completion",
		})
	case errors.Is(err, goalsvc.ErrGoalAlignmentRefreshRequired):
		return errorResultWithNextAction(err, map[string]any{
			"domain":    command.DomainGoal,
			"operation": "audit_objective_alignment",
			"reason":    "audit the authoritative Goal criteria in this physical round, then follow the audit result; WorkGraph readiness remains a separate completion gate",
		})
	default:
		return errorResult(err)
	}
}

func pausedGoalRecoveryResult(err error, item *protocol.Goal) (command.Result, bool) {
	if !errors.Is(err, goalsvc.ErrGoalInvalidState) || item == nil ||
		protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusPaused {
		return command.Result{}, false
	}
	return errorResultText(
		"the current Goal is paused and requires the user to resume it; ask the user to click the Play control labeled 「继续」 on the right side of the Goal status bar directly above this conversation's message composer; Nexus then automatically schedules a new Goal continuation to perform the remaining audit and completion work",
	), true
}

func updateGoalCurrentErrorResult(err error) command.Result {
	if isGoalNotFoundError(err) {
		return errorResultText("cannot update goal because this thread has no current goal; do not retry update_goal—if the user explicitly requested a new Goal and its objective is execution-ready, use create_goal")
	}
	return errorResult(err)
}

func isGoalNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "goal not found")
}

func updateGoalStatus(ctx context.Context, svc contract.Service, goalID string, status protocol.GoalStatus, blockerID string, reason string, neededInput string, roundID string, agentID string, expectedRevision int64) (*protocol.Goal, error) {
	switch status {
	case protocol.GoalStatusComplete:
		return svc.CompleteByModel(ctx, goalID, protocol.CompleteGoalRequest{RoundID: roundID, AgentID: agentID, ExpectedObjectiveRevision: expectedRevision})
	case protocol.GoalStatusBlocked:
		return svc.BlockByModel(ctx, goalID, protocol.BlockGoalRequest{BlockerID: strings.TrimSpace(blockerID), Reason: strings.TrimSpace(reason), NeededInput: strings.TrimSpace(neededInput), RoundID: roundID, AgentID: agentID, ExpectedObjectiveRevision: expectedRevision})
	default:
		return nil, fmt.Errorf("the Goal update operation can only mark the existing goal complete or blocked; pause, resume, budget-limited, and usage-limited status changes are controlled by the user or system")
	}
}

func currentGoalForMutation(
	ctx context.Context,
	svc contract.Service,
	sctx contract.Context,
	expectedRevision int64,
) (*protocol.Goal, error) {
	authority, ok := sctx.GoalAuthority.Load()
	if sctx.ResponsibilityAuthority != nil {
		authority, ok = sctx.ResponsibilityAuthority.LoadGoalAuthority()
	}
	if !ok || authority.ObjectiveRevision != expectedRevision {
		return nil, fmt.Errorf(
			"this round cannot mutate the current Goal because it has no exact Goal/revision capability; use the user Goal controls or continue with the responsible Goal Agent",
		)
	}
	current, err := svc.Current(ctx, sctx.CurrentSessionKey)
	if err != nil {
		return nil, err
	}
	if current.ID != authority.GoalID ||
		current.ObjectiveRevision() != expectedRevision ||
		(authority.ExecutionID != "" &&
			protocol.GoalReservedExecutionID(*current) != authority.ExecutionID) {
		return nil, fmt.Errorf(
			"this round's Goal/Execution capability is stale; reload the current Goal through a newly authorized continuation",
		)
	}
	return current, nil
}

// currentGoalForRetarget 只允许可信可见用户 round 为 retarget_goal 延迟绑定当前 revision。
func currentGoalForRetarget(
	ctx context.Context,
	svc contract.Service,
	sctx contract.Context,
) (*protocol.Goal, int64, error) {
	expectedRevision := sctx.ExpectedGoalObjectiveRevision()
	if expectedRevision > 0 {
		current, err := currentGoalForMutation(ctx, svc, sctx, expectedRevision)
		return current, expectedRevision, err
	}
	if !sctx.AllowUserRetarget {
		return nil, 0, fmt.Errorf(
			"this round cannot mutate the current Goal because it has no exact Goal/revision capability; use the user Goal controls or continue with the responsible Goal Agent",
		)
	}
	current, err := svc.Current(ctx, sctx.CurrentSessionKey)
	if err != nil {
		return nil, 0, err
	}
	if current == nil || current.ObjectiveRevision() <= 0 {
		return nil, 0, fmt.Errorf("the current Goal has no valid objective revision")
	}
	return current, current.ObjectiveRevision(), nil
}
