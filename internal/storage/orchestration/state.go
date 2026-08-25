// INPUT: explicit waiting_input/resume command 与 active Plan current projection。
// OUTPUT: Work Item lifecycle CAS、跨同一 Execution Plan revision 的稳定验收继承、派生 readiness 与 completion blockers。
// POS: stable state 和 delivery projection 不重复持久化的边界。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Block 记录确定的外部输入阻塞，并释放 current Assignment、中断其未完成执行链；
// dependency blocking 仍保持派生。
func (r *Repository) Block(ctx context.Context, command BlockCommand) (*protocol.ExecutionSnapshot, error) {
	state := command.State
	if state.WorkItemID == "" || state.ExecutionID == "" || state.CurrentSpecID == "" ||
		state.Status != protocol.WorkItemStatusWaitingInput ||
		state.BlockReason == "" || state.NeededInput == "" {
		return nil, fmt.Errorf("%w: waiting_input state requires chain, reason and needed input", ErrInvariant)
	}
	command.Meta.Payload = mergeStateEventPayload(command.Meta.Payload, map[string]any{
		"status":          protocol.WorkItemStatusWaitingInput,
		"block_reason":    state.BlockReason,
		"needed_input":    state.NeededInput,
		"current_spec_id": state.CurrentSpecID,
	})
	mutation, err := r.beginMutation(
		ctx, state.ExecutionID, command.ExpectedExecutionVersion,
		command.Meta, protocol.ExecutionEventStatusChanged,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	if err = validateExpectedVersion(command.ExpectedStateVersion, "expected Work Item state version"); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = r.rejectUnreviewedSubmission(
		ctx,
		mutation.tx,
		state.ExecutionID,
		state.WorkItemID,
		state.CurrentSpecID,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	metadataJSON, err := marshalMap(state.Metadata)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE execution_work_item_states
SET status = 'waiting_input',
    block_reason = `+r.bind(1)+`,
    needed_input = `+r.bind(2)+`,
    version = version + 1,
    updated_at = `+r.bind(3)+`,
    metadata_json = `+r.jsonBind(4)+`
WHERE work_item_id = `+r.bind(5)+`
  AND execution_id = `+r.bind(6)+`
  AND current_spec_id = `+r.bind(7)+`
  AND version = `+r.bind(8)+`
  AND status IN ('open', 'waiting_input')`,
		state.BlockReason, state.NeededInput, r.timestamp(r.currentTime()), metadataJSON,
		state.WorkItemID, state.ExecutionID, state.CurrentSpecID, command.ExpectedStateVersion,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	now := r.currentTime()
	terminalReason := "blocked: " + state.BlockReason
	if err = r.enqueueAttemptCancellations(
		ctx,
		mutation.tx,
		cancellationAttemptScope{
			ExecutionID: state.ExecutionID,
			WorkItemID:  state.WorkItemID,
			SpecID:      state.CurrentSpecID,
		},
		command.Meta.CommandID,
		terminalReason,
		now,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if _, err = mutation.tx.ExecContext(ctx, `
UPDATE execution_attempts
SET status = 'interrupted',
    failure_reason = `+r.bind(1)+`,
    version = version + 1,
    finished_at = `+r.bind(2)+`
WHERE execution_id = `+r.bind(3)+`
  AND work_item_id = `+r.bind(4)+`
  AND spec_id = `+r.bind(5)+`
  AND status IN ('pending', 'running')
  AND assignment_id IN (
      SELECT assignment_id
      FROM execution_work_assignments
      WHERE execution_id = `+r.bind(6)+`
        AND work_item_id = `+r.bind(7)+`
        AND spec_id = `+r.bind(8)+`
        AND status IN ('assigned', 'active')
  )`,
		terminalReason, r.timestamp(now), state.ExecutionID, state.WorkItemID, state.CurrentSpecID,
		state.ExecutionID, state.WorkItemID, state.CurrentSpecID,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if _, err = mutation.tx.ExecContext(ctx, `
UPDATE execution_dispatches
SET status = 'cancelled',
    last_error = `+r.bind(1)+`,
    version = version + 1,
    updated_at = `+r.bind(2)+`,
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE execution_id = `+r.bind(3)+`
  AND work_item_id = `+r.bind(4)+`
  AND spec_id = `+r.bind(5)+`
  AND status IN ('pending', 'claimed', 'failed')
  AND assignment_id IN (
      SELECT assignment_id
      FROM execution_work_assignments
      WHERE execution_id = `+r.bind(6)+`
        AND work_item_id = `+r.bind(7)+`
        AND spec_id = `+r.bind(8)+`
        AND status IN ('assigned', 'active')
  )`,
		terminalReason, r.timestamp(now), state.ExecutionID, state.WorkItemID, state.CurrentSpecID,
		state.ExecutionID, state.WorkItemID, state.CurrentSpecID,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if _, err = mutation.tx.ExecContext(ctx, `
UPDATE execution_work_assignments
SET status = 'released',
    version = version + 1,
    released_at = `+r.bind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND work_item_id = `+r.bind(3)+`
  AND spec_id = `+r.bind(4)+`
  AND status IN ('assigned', 'active')`,
		r.timestamp(now), state.ExecutionID, state.WorkItemID, state.CurrentSpecID,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityWorkItem,
		EntityID:      state.WorkItemID,
		EntityVersion: command.ExpectedStateVersion + 1,
		WorkItemID:    state.WorkItemID,
		SpecID:        state.CurrentSpecID,
	})
}

// Resume 关闭 waiting_input，并保留 resolution/evidence 作为 state 审计元数据。
// 它不会复活 Block 已终止的 Attempt 或 Dispatch。
func (r *Repository) Resume(ctx context.Context, command ResumeCommand) (*protocol.ExecutionSnapshot, error) {
	state := command.State
	command.Resolution = strings.TrimSpace(command.Resolution)
	if err := protocol.ValidateExecutionProjectionLimit(
		"resume_evidence",
		len(command.Evidence),
	); err != nil {
		return nil, err
	}
	evidence := make([]string, 0, len(command.Evidence))
	for _, item := range command.Evidence {
		if item = strings.TrimSpace(item); item != "" {
			evidence = append(evidence, item)
		}
	}
	if state.WorkItemID == "" || state.ExecutionID == "" || state.CurrentSpecID == "" ||
		state.Status != protocol.WorkItemStatusOpen ||
		command.Resolution == "" || len(evidence) == 0 {
		return nil, fmt.Errorf("%w: resume requires current chain, resolution and evidence", ErrInvariant)
	}
	command.Meta.Payload = mergeStateEventPayload(command.Meta.Payload, map[string]any{
		"status":          protocol.WorkItemStatusOpen,
		"resolution":      command.Resolution,
		"evidence":        evidence,
		"current_spec_id": state.CurrentSpecID,
	})
	mutation, err := r.beginMutation(
		ctx, state.ExecutionID, command.ExpectedExecutionVersion,
		command.Meta, protocol.ExecutionEventStatusChanged,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	if err = validateExpectedVersion(command.ExpectedStateVersion, "expected Work Item state version"); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	metadata := make(map[string]any, len(state.Metadata)+2)
	for key, value := range state.Metadata {
		metadata[key] = value
	}
	metadata["last_resume_resolution"] = command.Resolution
	metadata["last_resume_evidence"] = evidence
	metadataJSON, err := marshalMap(metadata)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE execution_work_item_states
SET status = 'open',
    block_reason = NULL,
    needed_input = NULL,
    version = version + 1,
    updated_at = `+r.bind(1)+`,
    metadata_json = `+r.jsonBind(2)+`
WHERE work_item_id = `+r.bind(3)+`
  AND execution_id = `+r.bind(4)+`
  AND current_spec_id = `+r.bind(5)+`
  AND version = `+r.bind(6)+`
  AND status = 'waiting_input'`,
		r.timestamp(r.currentTime()), metadataJSON, state.WorkItemID, state.ExecutionID,
		state.CurrentSpecID, command.ExpectedStateVersion,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityWorkItem,
		EntityID:      state.WorkItemID,
		EntityVersion: command.ExpectedStateVersion + 1,
		WorkItemID:    state.WorkItemID,
		SpecID:        state.CurrentSpecID,
	})
}

// CompletionBlockers 返回当前 active Plan 的派生完成阻塞项。
func (r *Repository) CompletionBlockers(ctx context.Context, executionID string) ([]string, error) {
	snapshot, err := r.GetSnapshot(ctx, executionID)
	if err != nil || snapshot == nil {
		return nil, err
	}
	return append([]string(nil), snapshot.CompletionBlockers...), nil
}

func (r *Repository) workEligible(
	ctx context.Context,
	queryer sqlQueryer,
	planID string,
	workItemID string,
	specID string,
) (bool, string, error) {
	state, err := r.getState(ctx, queryer, workItemID)
	if err != nil {
		return false, "", err
	}
	if state == nil || state.CurrentSpecID != specID {
		return false, "Work Item state/spec fence is stale", nil
	}
	if state.Status != protocol.WorkItemStatusOpen {
		return false, "Work Item lifecycle is " + string(state.Status), nil
	}
	var accepted int
	if err = queryer.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_acceptances acceptance
JOIN execution_plan_revisions active_plan
  ON active_plan.plan_id = `+r.bind(1)+`
 AND active_plan.execution_id = acceptance.execution_id
WHERE acceptance.work_item_id = `+r.bind(2)+`
  AND acceptance.spec_id = `+r.bind(3)+`
  AND acceptance.decision = 'accepted'`,
		planID, workItemID, specID,
	).Scan(&accepted); err != nil {
		return false, "", err
	}
	if accepted != 0 {
		return false, "Work Item spec is already accepted", nil
	}
	var blockedDependencies int
	if err = queryer.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_plan_dependencies dependency
JOIN execution_plan_items upstream
  ON upstream.plan_id = dependency.plan_id
 AND upstream.work_item_id = dependency.depends_on_work_item_id
WHERE dependency.plan_id = `+r.bind(1)+`
  AND dependency.work_item_id = `+r.bind(2)+`
  AND dependency.dependency_kind = 'hard'
  AND NOT EXISTS (
      SELECT 1
      FROM execution_acceptances acceptance
      WHERE acceptance.execution_id = dependency.execution_id
        AND acceptance.work_item_id = upstream.work_item_id
        AND acceptance.spec_id = upstream.spec_id
        AND acceptance.decision = 'accepted'
  )`,
		planID, workItemID,
	).Scan(&blockedDependencies); err != nil {
		return false, "", err
	}
	if blockedDependencies != 0 {
		return false, "hard dependencies are not accepted", nil
	}
	var pendingSubmission int
	if err = queryer.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_submissions submission
LEFT JOIN execution_acceptances acceptance
  ON acceptance.submission_id = submission.submission_id
JOIN execution_plan_revisions active_plan
  ON active_plan.plan_id = `+r.bind(1)+`
 AND active_plan.execution_id = submission.execution_id
WHERE submission.work_item_id = `+r.bind(2)+`
  AND submission.spec_id = `+r.bind(3)+`
  AND acceptance.acceptance_id IS NULL`,
		planID, workItemID, specID,
	).Scan(&pendingSubmission); err != nil {
		return false, "", err
	}
	if pendingSubmission != 0 {
		return false, "Submission is awaiting review", nil
	}
	return true, "", nil
}

func deriveSnapshot(snapshot *protocol.ExecutionSnapshot) {
	if snapshot == nil {
		return
	}
	if snapshot.Plan == nil {
		if snapshot.Execution.Status != protocol.ExecutionStatusCompleted &&
			snapshot.Execution.Status != protocol.ExecutionStatusFailed &&
			snapshot.Execution.Status != protocol.ExecutionStatusCancelled &&
			snapshot.Execution.Status != protocol.ExecutionStatusSuperseded {
			snapshot.CompletionBlockers = []string{"active_plan_missing"}
		}
		return
	}
	stateByWork := make(map[string]protocol.WorkItemState, len(snapshot.WorkItemStates))
	for _, state := range snapshot.WorkItemStates {
		stateByWork[state.WorkItemID] = state
	}
	accepted := make(map[string]bool)
	decisionBySubmission := make(map[string]protocol.WorkAcceptanceDecision)
	for _, acceptance := range snapshot.Acceptances {
		decisionBySubmission[acceptance.SubmissionID] = acceptance.Decision
		if acceptance.Decision == protocol.WorkAcceptanceAccepted {
			accepted[acceptance.WorkItemID+"\x00"+acceptance.SpecID] = true
		}
	}
	currentAssignment := make(map[string]bool)
	for _, assignment := range snapshot.Assignments {
		if assignment.Status == protocol.WorkAssignmentStatusAssigned ||
			assignment.Status == protocol.WorkAssignmentStatusActive {
			currentAssignment[assignment.WorkItemID] = true
			snapshot.CompletionBlockers = append(
				snapshot.CompletionBlockers,
				fmt.Sprintf("assignment:%s:%s", assignment.ID, assignment.Status),
			)
		}
	}
	pendingSubmission := make(map[string]bool)
	for _, submission := range snapshot.Submissions {
		if _, reviewed := decisionBySubmission[submission.ID]; !reviewed {
			pendingSubmission[submission.WorkItemID] = true
			snapshot.CompletionBlockers = append(
				snapshot.CompletionBlockers,
				"submission:"+submission.ID+":unreviewed",
			)
		}
	}
	dependencies := make(map[string][]protocol.ExecutionPlanDependency)
	for _, dependency := range snapshot.Dependencies {
		if dependency.Kind == protocol.WorkDependencyHard {
			dependencies[dependency.WorkItemID] = append(dependencies[dependency.WorkItemID], dependency)
		}
	}
	specByWork := make(map[string]string, len(snapshot.PlanItems))
	for _, item := range snapshot.PlanItems {
		specByWork[item.WorkItemID] = item.SpecID
	}
	for _, item := range snapshot.PlanItems {
		key := item.WorkItemID + "\x00" + item.SpecID
		isAccepted := accepted[key]
		if (item.Required || item.Terminal) && !isAccepted {
			kind := "required"
			if item.Terminal {
				kind = "terminal"
			}
			snapshot.CompletionBlockers = append(
				snapshot.CompletionBlockers,
				fmt.Sprintf("work_item:%s:%s_not_accepted", item.WorkItemID, kind),
			)
		}
		state := stateByWork[item.WorkItemID]
		if !isAccepted && state.Status == protocol.WorkItemStatusWaitingInput {
			snapshot.CompletionBlockers = append(
				snapshot.CompletionBlockers,
				"work_item:"+item.WorkItemID+":waiting_input",
			)
		}
		if isAccepted || state.Status != protocol.WorkItemStatusOpen ||
			state.CurrentSpecID != item.SpecID || currentAssignment[item.WorkItemID] ||
			pendingSubmission[item.WorkItemID] {
			continue
		}
		ready := true
		for _, dependency := range dependencies[item.WorkItemID] {
			upstreamSpec := specByWork[dependency.DependsOnWorkItemID]
			if !accepted[dependency.DependsOnWorkItemID+"\x00"+upstreamSpec] {
				ready = false
				break
			}
		}
		if ready {
			snapshot.ReadyWorkItemIDs = append(snapshot.ReadyWorkItemIDs, item.WorkItemID)
		}
	}
	for _, attempt := range snapshot.Attempts {
		if attempt.Status == protocol.WorkAttemptStatusPending ||
			attempt.Status == protocol.WorkAttemptStatusRunning {
			snapshot.CompletionBlockers = append(
				snapshot.CompletionBlockers,
				fmt.Sprintf("attempt:%s:%s", attempt.ID, attempt.Status),
			)
		}
	}
	for _, dispatch := range snapshot.Dispatches {
		if dispatch.Status == protocol.ExecutionDispatchStatusPending ||
			dispatch.Status == protocol.ExecutionDispatchStatusClaimed {
			snapshot.CompletionBlockers = append(
				snapshot.CompletionBlockers,
				fmt.Sprintf("dispatch:%s:%s", dispatch.ID, dispatch.Status),
			)
		}
	}
	sort.Strings(snapshot.ReadyWorkItemIDs)
	sort.Strings(snapshot.CompletionBlockers)
	snapshot.ReadyWorkItemIDs = compactStrings(snapshot.ReadyWorkItemIDs)
	snapshot.CompletionBlockers = compactStrings(snapshot.CompletionBlockers)
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	write := 1
	for read := 1; read < len(values); read++ {
		if values[read] == values[write-1] {
			continue
		}
		values[write] = values[read]
		write++
	}
	return values[:write]
}

func mergeStateEventPayload(base map[string]any, values map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(values))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range values {
		result[key] = value
	}
	return result
}

var _ sqlQueryer = (*sql.Tx)(nil)
