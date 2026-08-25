// INPUT: 当前 WorkGraph 的 Plan 或 Execution identity。
// OUTPUT: 同一 Execution 跨 Plan revision 的完整、稳定排序 Assignment/Attempt/Submission/Review/Acceptance 历史。
// POS: WorkGraph 专用只读历史边界；不得扩大模型与运行状态机使用的有界 Snapshot。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListWorkGraphHistory 返回画布重建每个不可变交付轮次所需的完整历史。
// command 继续使用有界 Snapshot；此读取面只服务 owner/session 已校验后的 UI。
func (r *Repository) ListWorkGraphHistory(
	ctx context.Context,
	planID string,
) (protocol.ExecutionWorkGraphHistory, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return protocol.ExecutionWorkGraphHistory{}, err
	}
	defer func() { _ = tx.Rollback() }()
	plan, err := r.getPlan(ctx, tx, planID)
	if err != nil {
		return protocol.ExecutionWorkGraphHistory{}, err
	}
	if plan == nil {
		return protocol.ExecutionWorkGraphHistory{}, nil
	}
	result, err := r.listWorkGraphHistory(ctx, tx, plan.ExecutionID)
	if err != nil {
		return protocol.ExecutionWorkGraphHistory{}, err
	}
	if err = tx.Commit(); err != nil {
		return protocol.ExecutionWorkGraphHistory{}, err
	}
	return result, nil
}

// GetWorkGraphState 在同一 read transaction 中读取 current Snapshot 与完整
// 画布历史，避免 mutation 边界上出现节点先出现、随后又消失的撕裂投影。
func (r *Repository) GetWorkGraphState(
	ctx context.Context,
	executionID string,
) (*protocol.ExecutionSnapshot, protocol.ExecutionWorkGraphHistory, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, protocol.ExecutionWorkGraphHistory{}, err
	}
	defer func() { _ = tx.Rollback() }()
	snapshot, err := r.getSnapshot(ctx, tx, executionID)
	if err != nil || snapshot == nil || snapshot.Plan == nil {
		return snapshot, protocol.ExecutionWorkGraphHistory{}, err
	}
	history, err := r.listWorkGraphHistory(ctx, tx, executionID)
	if err != nil {
		return nil, protocol.ExecutionWorkGraphHistory{}, err
	}
	if err = tx.Commit(); err != nil {
		return nil, protocol.ExecutionWorkGraphHistory{}, err
	}
	return snapshot, history, nil
}

func (r *Repository) listWorkGraphHistory(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
) (protocol.ExecutionWorkGraphHistory, error) {
	var result protocol.ExecutionWorkGraphHistory
	queries := []struct {
		name string
		load func() error
	}{
		{name: "assignments", load: func() error {
			rows, err := queryer.QueryContext(ctx, r.assignmentSelect("assignment.")+`
FROM execution_work_assignments assignment
WHERE assignment.execution_id = `+r.bind(1)+`
ORDER BY assignment.work_item_id, assignment.assigned_at, assignment.assignment_id`, executionID)
			if err != nil {
				return err
			}
			result.Assignments, err = scanRows(rows, scanAssignment)
			return err
		}},
		{name: "attempts", load: func() error {
			rows, err := queryer.QueryContext(ctx, r.attemptSelect("attempt.")+`
FROM execution_attempts attempt
WHERE attempt.execution_id = `+r.bind(1)+`
ORDER BY attempt.work_item_id, attempt.created_at, attempt.attempt_id`, executionID)
			if err != nil {
				return err
			}
			result.Attempts, err = scanRows(rows, scanAttempt)
			return err
		}},
		{name: "submissions", load: func() error {
			rows, err := queryer.QueryContext(ctx, r.submissionSelect("submission.")+`
FROM execution_submissions submission
WHERE submission.execution_id = `+r.bind(1)+`
ORDER BY submission.work_item_id, submission.submission_sequence, submission.submission_id`, executionID)
			if err != nil {
				return err
			}
			result.Submissions, err = scanRows(rows, scanSubmission)
			return err
		}},
		{name: "review dispatches", load: func() error {
			rows, err := queryer.QueryContext(ctx, r.reviewDispatchSelect("review_dispatch.")+`
FROM execution_review_dispatches review_dispatch
WHERE review_dispatch.execution_id = `+r.bind(1)+`
ORDER BY review_dispatch.created_at, review_dispatch.review_dispatch_id`, executionID)
			if err != nil {
				return err
			}
			result.ReviewDispatches, err = scanRows(rows, scanReviewDispatch)
			return err
		}},
		{name: "acceptances", load: func() error {
			rows, err := queryer.QueryContext(ctx, r.acceptanceSelect("acceptance.")+`
FROM execution_acceptances acceptance
WHERE acceptance.execution_id = `+r.bind(1)+`
ORDER BY acceptance.work_item_id, acceptance.created_at, acceptance.acceptance_id`, executionID)
			if err != nil {
				return err
			}
			result.Acceptances, err = scanRows(rows, scanAcceptance)
			return err
		}},
	}
	for _, query := range queries {
		if err := query.load(); err != nil {
			return protocol.ExecutionWorkGraphHistory{}, fmt.Errorf("load WorkGraph %s: %w", query.name, err)
		}
	}
	return result, nil
}

// ListWorkGraphChildAttempts 返回当前 Plan 的全部 child Attempt。Snapshot 会按
// root/child lane 压缩终态以保持运行上下文有界；画布必须读取完整历史，才能把
// 每次真实 Subagent 执行投影成独立节点。
func (r *Repository) ListWorkGraphChildAttempts(
	ctx context.Context,
	planID string,
) ([]protocol.WorkAttempt, error) {
	rows, err := r.db.QueryContext(ctx, r.attemptSelect("attempt.")+`
FROM execution_attempts attempt
WHERE attempt.plan_id = `+r.bind(1)+`
  AND attempt.parent_attempt_id IS NOT NULL
ORDER BY attempt.work_item_id, attempt.created_at, attempt.attempt_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanAttempt)
}
