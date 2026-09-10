// INPUT: exact owner/source Draft revision、已确认元信息与命名图的预期版本。
// OUTPUT: 同事务提交的完整命名图、不可变元信息版本与 Draft 保存标记。
// POS: UI 直接保存与模型保存共用的 CAS 边界，拒绝迟到保存覆盖新版本。
package workgraphworkflow

import (
	"context"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrRevisionConflict 表示草图或命名图已变化，本次事务未应用。
var ErrRevisionConflict = errors.New("WorkGraph revision changed")

// SaveDraft 原子提交已确认草图与其命名图；expectedWorkflowVersion 为零时创建。
func (r *Repository) SaveDraft(
	ctx context.Context,
	draft protocol.WorkGraphWorkflowDraft,
	preview protocol.WorkGraphWorkflowPreview,
	workflow protocol.WorkGraphWorkflow,
	expectedWorkflowVersion int64,
	savedAt time.Time,
) (*protocol.WorkGraphWorkflow, error) {
	if draft.OwnerUserID != workflow.OwnerUserID || draft.PreviewID != preview.PreviewID ||
		draft.SourceSessionKey != preview.SourceSessionKey || draft.SourceExecutionID != preview.SourceExecutionID ||
		(draft.SavedWorkflowID != "" && draft.SavedWorkflowID != workflow.ID) {
		return nil, ErrRevisionConflict
	}
	head, selected := draft.HeadRevision, draft.SelectedRevision
	metadataChanged := preview.SlashName != draft.Preview.SlashName ||
		preview.Title != draft.Preview.Title || preview.Description != draft.Preview.Description
	if metadataChanged {
		head++
		selected = head
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
UPDATE workgraph_workflow_drafts
SET head_revision = `+r.bind(1)+`, selected_revision = `+r.bind(2)+`,
    save_scheduled = `+r.bind(3)+`, saved_workflow_id = `+r.bind(4)+`,
    saved_revision = `+r.bind(5)+`, updated_at = `+r.bind(6)+`
WHERE owner_user_id = `+r.bind(7)+` AND preview_id = `+r.bind(8)+`
  AND head_revision = `+r.bind(9)+` AND selected_revision = `+r.bind(10)+`
  AND saved_workflow_id = `+r.bind(11)+` AND saved_revision = `+r.bind(12),
		head, selected, false, workflow.ID, selected, r.timestamp(savedAt),
		draft.OwnerUserID, draft.PreviewID, draft.HeadRevision, draft.SelectedRevision,
		draft.SavedWorkflowID, draft.SavedRevision,
	)
	if err != nil {
		return nil, err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil {
		return nil, affectedErr
	} else if affected != 1 {
		return nil, ErrRevisionConflict
	}
	if metadataChanged {
		payload, encodeErr := marshalJSON(preview)
		if encodeErr != nil {
			return nil, encodeErr
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO workgraph_workflow_draft_versions (preview_id, revision, preview_json, created_at)
VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.jsonBind(3)+`,`+r.bind(4)+`)`,
			draft.PreviewID, head, payload, r.timestamp(savedAt)); err != nil {
			return nil, err
		}
	}
	switch {
	case expectedWorkflowVersion == 0 && workflow.Version == 1:
		err = r.createWorkflow(ctx, tx, workflow)
	case expectedWorkflowVersion > 0 && workflow.Version == expectedWorkflowVersion+1:
		err = r.updateWorkflow(ctx, tx, workflow)
	case expectedWorkflowVersion > 0 && workflow.Version == expectedWorkflowVersion:
		// Even a content-identical save must fence concurrent aggregate updates.
		var currentVersion int64
		err = tx.QueryRowContext(ctx, `SELECT version FROM workgraph_workflows
WHERE owner_user_id = `+r.bind(1)+` AND workflow_id = `+r.bind(2), workflow.OwnerUserID, workflow.ID).Scan(&currentVersion)
		if err == nil && currentVersion != expectedWorkflowVersion {
			err = ErrRevisionConflict
		}
	default:
		err = ErrRevisionConflict
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &workflow, nil
}
