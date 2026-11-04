// INPUT: Owner-confirmed snapshot revision, exact root and affected work routing.
// OUTPUT: Atomic result supersession, cancellation outbox and durable retry policies.
// POS: User retry transaction preserves immutable history and fences late workers/reviewers.
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func activeSubmissionSQL(alias string) string {
	return "NOT EXISTS (SELECT 1 FROM execution_submission_supersessions superseded WHERE superseded.submission_id = " + alias + ".submission_id)"
}

type RetryWorkCommand struct {
	OwnerUserID string
	Preview     protocol.WorkRetryPreview
	Policies    []protocol.ExecutionDispatchPolicy
	Meta        CommandMeta
}

func (r *Repository) RetryWork(ctx context.Context, command RetryWorkCommand) (*protocol.ExecutionSnapshot, error) {
	preview := command.Preview
	mutation, err := r.beginMutation(ctx, preview.ExecutionID, preview.SnapshotRevision, command.Meta, protocol.ExecutionEventWorkRetried, true)
	if err != nil {
		return nil, err
	}
	defer r.abortMutation(mutation)
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	snapshot, err := r.getSnapshot(ctx, mutation.tx, preview.ExecutionID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || snapshot.Plan == nil || snapshot.Execution.OwnerUserID != command.OwnerUserID {
		return nil, ErrInvariant
	}
	latest, err := r.listManaged(ctx, mutation.tx, command.OwnerUserID, snapshot.Execution.SessionKey, 1)
	if err != nil {
		return nil, err
	}
	if len(latest) != 1 || latest[0].ID != preview.ExecutionID {
		return nil, ErrVersionConflict
	}
	var competing int
	err = mutation.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM executions WHERE owner_user_id=`+r.bind(1)+` AND session_key=`+r.bind(2)+` AND execution_id<>`+r.bind(3)+` AND status IN ('active','waiting','paused')`, command.OwnerUserID, snapshot.Execution.SessionKey, preview.ExecutionID).Scan(&competing)
	if err != nil {
		return nil, err
	}
	if competing != 0 {
		return nil, ErrVersionConflict
	}
	if len(preview.Affected) == 0 || len(preview.Affected) > protocol.ExecutionProjectionCollectionLimit {
		return nil, ErrInvariant
	}
	now := r.timestamp(r.currentTime())
	// Preserve the pause of unrelated automatic policies when reopening a paused graph.
	if snapshot.Execution.Status == protocol.ExecutionStatusPaused {
		if _, err = mutation.tx.ExecContext(ctx, `UPDATE execution_dispatch_policies SET reason='user_paused' WHERE execution_id=`+r.bind(1)+` AND state='pending'`, preview.ExecutionID); err != nil {
			return nil, err
		}
	}
	for _, work := range preview.Affected {
		var member int
		if err = mutation.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_plan_items WHERE plan_id=`+r.bind(1)+` AND work_item_id=`+r.bind(2), snapshot.Plan.ID, work.WorkItemID).Scan(&member); err != nil {
			return nil, err
		}
		if member != 1 {
			return nil, ErrInvariant
		}
		if err = r.enqueueAttemptCancellations(ctx, mutation.tx, cancellationAttemptScope{ExecutionID: preview.ExecutionID, WorkItemID: work.WorkItemID}, command.Meta.CommandID, "user requested task retry", r.currentTime()); err != nil {
			return nil, err
		}
		statements := []struct {
			query string
			args  []any
		}{
			{`INSERT INTO execution_submission_supersessions (submission_id,command_id,created_at) SELECT submission.submission_id,` + r.bind(1) + `,` + r.bind(2) + ` FROM execution_submissions submission WHERE submission.execution_id=` + r.bind(3) + ` AND submission.work_item_id=` + r.bind(4) + ` AND ` + activeSubmissionSQL("submission"), []any{command.Meta.CommandID, now, preview.ExecutionID, work.WorkItemID}},
			{`UPDATE execution_attempts SET status='interrupted',failure_reason='user requested task retry',version=version+1,finished_at=` + r.bind(1) + ` WHERE execution_id=` + r.bind(2) + ` AND work_item_id=` + r.bind(3) + ` AND status IN ('pending','running')`, []any{now, preview.ExecutionID, work.WorkItemID}},
			{`UPDATE execution_work_assignments SET status='revoked',version=version+1,released_at=` + r.bind(1) + ` WHERE execution_id=` + r.bind(2) + ` AND work_item_id=` + r.bind(3) + ` AND status IN ('assigned','active','completed')`, []any{now, preview.ExecutionID, work.WorkItemID}},
			{`UPDATE execution_dispatches SET status='cancelled',version=version+1,lease_owner=NULL,lease_expires_at=NULL,last_error='user requested task retry' WHERE execution_id=` + r.bind(1) + ` AND work_item_id=` + r.bind(2) + ` AND status IN ('pending','claimed','failed')`, []any{preview.ExecutionID, work.WorkItemID}},
			{`UPDATE execution_review_dispatches SET status='cancelled',version=version+1,lease_owner=NULL,lease_expires_at=NULL,last_error='submission superseded by retry' WHERE execution_id=` + r.bind(1) + ` AND work_item_id=` + r.bind(2) + ` AND status IN ('pending','claimed','failed','delivered')`, []any{preview.ExecutionID, work.WorkItemID}},
			{`UPDATE execution_dispatch_policies SET state='cancelled',reason='user_retry' WHERE execution_id=` + r.bind(1) + ` AND work_item_id=` + r.bind(2) + ` AND state!='cancelled'`, []any{preview.ExecutionID, work.WorkItemID}},
			{`UPDATE execution_work_item_states SET status='open',block_reason=NULL,needed_input=NULL,version=version+1,updated_at=` + r.bind(1) + ` WHERE execution_id=` + r.bind(2) + ` AND work_item_id=` + r.bind(3), []any{now, preview.ExecutionID, work.WorkItemID}},
		}
		for _, statement := range statements {
			if _, err = mutation.tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
				return nil, fmt.Errorf("reset retry work: %w", err)
			}
		}
	}
	// Fence completion workers from the prior accepted generation and reopen its audit.
	if _, err = mutation.tx.ExecContext(ctx, `UPDATE execution_completion_audits SET state='pending',version=version+1,settled_at=NULL,next_attempt_at=`+r.bind(1)+`,updated_at=`+r.bind(2)+` WHERE execution_id=`+r.bind(3), now, now, preview.ExecutionID); err != nil {
		return nil, err
	}
	for _, policy := range command.Policies {
		raw, encodeErr := json.Marshal(policy)
		if encodeErr != nil {
			return nil, encodeErr
		}
		_, err = mutation.tx.ExecContext(ctx, `INSERT INTO execution_dispatch_policies (policy_id,execution_id,plan_id,work_item_id,spec_id,intent_json,state,next_attempt_at,created_at) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+r.bind(6)+`,'pending',`+r.bind(7)+`,`+r.bind(8)+`)`, policy.ID, preview.ExecutionID, snapshot.Plan.ID, policy.WorkItemID, policy.SpecID, string(raw), now, now)
		if err != nil {
			return nil, err
		}
	}
	if _, err = mutation.tx.ExecContext(ctx, `UPDATE executions SET status='active',completed_at=NULL WHERE execution_id=`+r.bind(1), preview.ExecutionID); err != nil {
		return nil, err
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{EntityType: protocol.ExecutionEntityExecution, EntityID: preview.ExecutionID, EntityVersion: preview.SnapshotRevision + 1, WorkItemID: preview.WorkItemID, PlanID: snapshot.Plan.ID, AttemptID: preview.AttemptID})
}
