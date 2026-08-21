// INPUT: owner/source-scoped WorkGraph Draft、不可变完整草图版本与 revision CAS。
// OUTPUT: 跨重启可恢复的 Draft、可续期空闲 lease、编辑 Session 绑定、版本选择和保存状态。
// POS: 临时草图生命周期的关系数据库真相源；命名 WorkGraph aggregate 仍由 repository.go 持久化。
package workgraphworkflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CreateDraft 原子写入新 Draft 及 revision 1。
func (r *Repository) CreateDraft(
	ctx context.Context,
	draft protocol.WorkGraphWorkflowDraft,
) (*protocol.WorkGraphWorkflowDraft, error) {
	if len(draft.Versions) != 1 || draft.Versions[0].Revision != 1 {
		return nil, errors.New("new WorkGraph Draft requires revision 1")
	}
	payload, err := marshalJSON(draft.Versions[0].Preview)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
INSERT INTO workgraph_workflow_drafts (
    preview_id, owner_user_id, source_execution_id, source_session_key,
    source_agent_id, source_conversation_id, output_language,
    head_revision, selected_revision, editor_id, editor_agent_id,
    editor_session_key, editor_display_after_unix_milli,
    save_scheduled, saved_workflow_id, saved_revision, expires_at, created_at, updated_at
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+
		r.bind(5)+`,`+r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+
		r.bind(9)+`,`+r.bind(10)+`,`+r.bind(11)+`,`+r.bind(12)+`,`+
		r.bind(13)+`,`+r.bind(14)+`,`+r.bind(15)+`,`+r.bind(16)+`,`+
		r.bind(17)+`,`+r.bind(18)+`,`+r.bind(19)+`)`,
		draft.PreviewID, draft.OwnerUserID, draft.SourceExecutionID, draft.SourceSessionKey,
		draft.SourceAgentID, draft.SourceConversationID, draft.OutputLanguage,
		draft.HeadRevision, draft.SelectedRevision, draft.EditorID, draft.EditorAgentID,
		draft.EditorSessionKey, draft.EditorDisplayAfter, draft.SaveScheduled,
		draft.SavedWorkflowID, draft.SavedRevision, r.timestamp(draft.ExpiresAt), r.timestamp(draft.CreatedAt), r.timestamp(draft.UpdatedAt),
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO workgraph_workflow_draft_versions (
    preview_id, revision, preview_json, created_at
) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.jsonBind(3)+`,`+r.bind(4)+`)`,
		draft.PreviewID, int64(1), payload, r.timestamp(draft.Versions[0].CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetDraftByID(ctx, draft.OwnerUserID, draft.PreviewID)
}

// GetDraftByID 按 owner + preview_id 读取 Draft 和全部不可变版本。
func (r *Repository) GetDraftByID(
	ctx context.Context,
	ownerUserID string,
	previewID string,
) (*protocol.WorkGraphWorkflowDraft, error) {
	draft, err := scanDraft(r.db.QueryRowContext(ctx, r.draftSelect()+`
WHERE owner_user_id = `+r.bind(1)+` AND preview_id = `+r.bind(2),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(previewID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = r.loadDraftVersions(ctx, &draft); err != nil {
		return nil, err
	}
	return &draft, nil
}

// GetDraftBySource 保证同一 owner/session/execution 只需提取一次。
func (r *Repository) GetDraftBySource(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
	sourceExecutionID string,
) (*protocol.WorkGraphWorkflowDraft, error) {
	draft, err := scanDraft(r.db.QueryRowContext(ctx, r.draftSelect()+`
WHERE owner_user_id = `+r.bind(1)+`
  AND source_session_key = `+r.bind(2)+`
  AND source_execution_id = `+r.bind(3),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(sourceSessionKey), strings.TrimSpace(sourceExecutionID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = r.loadDraftVersions(ctx, &draft); err != nil {
		return nil, err
	}
	return &draft, nil
}

// GetDraftByEditorSession 恢复隐藏编辑 Session 的 round-scoped policy。
func (r *Repository) GetDraftByEditorSession(
	ctx context.Context,
	ownerUserID string,
	editorSessionKey string,
) (*protocol.WorkGraphWorkflowDraft, error) {
	draft, err := scanDraft(r.db.QueryRowContext(ctx, r.draftSelect()+`
WHERE owner_user_id = `+r.bind(1)+` AND editor_session_key = `+r.bind(2),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(editorSessionKey)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = r.loadDraftVersions(ctx, &draft); err != nil {
		return nil, err
	}
	return &draft, nil
}

// GetDraftBySavedWorkflowID 从能力目录恢复命名图关联的 Draft。
func (r *Repository) GetDraftBySavedWorkflowID(
	ctx context.Context,
	ownerUserID string,
	workflowID string,
) (*protocol.WorkGraphWorkflowDraft, error) {
	draft, err := scanDraft(r.db.QueryRowContext(ctx, r.draftSelect()+`
WHERE owner_user_id = `+r.bind(1)+` AND saved_workflow_id = `+r.bind(2),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(workflowID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = r.loadDraftVersions(ctx, &draft); err != nil {
		return nil, err
	}
	return &draft, nil
}

// ListDrafts 返回 exact source Session 下的 Draft 目录。
func (r *Repository) ListDrafts(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
) ([]protocol.WorkGraphWorkflowDraft, error) {
	rows, err := r.db.QueryContext(ctx, r.draftSelect()+`
WHERE owner_user_id = `+r.bind(1)+` AND source_session_key = `+r.bind(2)+`
ORDER BY updated_at DESC, preview_id ASC`,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(sourceSessionKey))
	if err != nil {
		return nil, err
	}
	items := make([]protocol.WorkGraphWorkflowDraft, 0)
	for rows.Next() {
		draft, scanErr := scanDraft(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		items = append(items, draft)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	for index := range items {
		if err = r.loadDraftVersions(ctx, &items[index]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// AppendDraftVersion 以 head revision CAS 追加不可变版本并选中它。
func (r *Repository) AppendDraftVersion(
	ctx context.Context,
	ownerUserID string,
	previewID string,
	expectedHeadRevision int64,
	preview protocol.WorkGraphWorkflowPreview,
	createdAt time.Time,
	expiresAt time.Time,
) (*protocol.WorkGraphWorkflowDraft, error) {
	nextRevision := expectedHeadRevision + 1
	payload, err := marshalJSON(preview)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
UPDATE workgraph_workflow_drafts
SET head_revision = `+r.bind(1)+`, selected_revision = `+r.bind(2)+`,
    expires_at = `+r.bind(3)+`, updated_at = `+r.bind(4)+`
WHERE owner_user_id = `+r.bind(5)+` AND preview_id = `+r.bind(6)+`
  AND head_revision = `+r.bind(7)+` AND save_scheduled = `+r.bind(8),
		nextRevision, nextRevision, r.timestamp(expiresAt), r.timestamp(createdAt),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(previewID), expectedHeadRevision, false,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, errors.New("WorkGraph Draft revision changed")
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO workgraph_workflow_draft_versions (
    preview_id, revision, preview_json, created_at
) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.jsonBind(3)+`,`+r.bind(4)+`)`,
		strings.TrimSpace(previewID), nextRevision, payload, r.timestamp(createdAt),
	)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetDraftByID(ctx, ownerUserID, previewID)
}

// SelectDraftVersion 只改变当前选择，不重写或复制既有版本。
func (r *Repository) SelectDraftVersion(
	ctx context.Context,
	ownerUserID string,
	previewID string,
	expectedHeadRevision int64,
	selectedRevision int64,
	updatedAt time.Time,
) (*protocol.WorkGraphWorkflowDraft, error) {
	var exists int
	err := r.db.QueryRowContext(ctx, `
SELECT 1 FROM workgraph_workflow_draft_versions
WHERE preview_id = `+r.bind(1)+` AND revision = `+r.bind(2),
		strings.TrimSpace(previewID), selectedRevision).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("WorkGraph Draft version not found")
	}
	if err != nil {
		return nil, err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE workgraph_workflow_drafts
SET selected_revision = `+r.bind(1)+`, updated_at = `+r.bind(2)+`
WHERE owner_user_id = `+r.bind(3)+` AND preview_id = `+r.bind(4)+`
  AND head_revision = `+r.bind(5)+` AND save_scheduled = `+r.bind(6),
		selectedRevision, r.timestamp(updatedAt), strings.TrimSpace(ownerUserID),
		strings.TrimSpace(previewID), expectedHeadRevision, false,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, errors.New("WorkGraph Draft revision changed")
	}
	return r.GetDraftByID(ctx, ownerUserID, previewID)
}

// BindDraftEditor 持久化隐藏编辑 Session，使关闭页面或宿主重启后仍能恢复同一对话。
func (r *Repository) BindDraftEditor(
	ctx context.Context,
	ownerUserID string,
	previewID string,
	editorID string,
	editorAgentID string,
	editorSessionKey string,
	displayAfterUnixMilli int64,
	updatedAt time.Time,
) (*protocol.WorkGraphWorkflowDraft, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE workgraph_workflow_drafts
SET editor_id = `+r.bind(1)+`, editor_agent_id = `+r.bind(2)+`,
    editor_session_key = `+r.bind(3)+`, editor_display_after_unix_milli = `+r.bind(4)+`,
    updated_at = `+r.bind(5)+`
WHERE owner_user_id = `+r.bind(6)+` AND preview_id = `+r.bind(7),
		strings.TrimSpace(editorID), strings.TrimSpace(editorAgentID),
		strings.TrimSpace(editorSessionKey), displayAfterUnixMilli, r.timestamp(updatedAt),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(previewID),
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, errors.New("WorkGraph Draft not found")
	}
	return r.GetDraftByID(ctx, ownerUserID, previewID)
}

// RenewDraftLease keeps a durable Draft reopenable while it remains in active use.
func (r *Repository) RenewDraftLease(
	ctx context.Context,
	ownerUserID string,
	previewID string,
	expiresAt time.Time,
	updatedAt time.Time,
) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE workgraph_workflow_drafts
SET expires_at = `+r.bind(1)+`, updated_at = `+r.bind(2)+`
WHERE owner_user_id = `+r.bind(3)+` AND preview_id = `+r.bind(4),
		r.timestamp(expiresAt), r.timestamp(updatedAt), strings.TrimSpace(ownerUserID), strings.TrimSpace(previewID),
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("WorkGraph Draft not found")
	}
	return nil
}

// SetDraftSaveState 记录后台保存 claim 或最终命名图结果。
func (r *Repository) SetDraftSaveState(
	ctx context.Context,
	ownerUserID string,
	previewID string,
	scheduled bool,
	savedWorkflowID string,
	savedRevision int64,
	updatedAt time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE workgraph_workflow_drafts
SET save_scheduled = `+r.bind(1)+`, saved_workflow_id = `+r.bind(2)+`,
    saved_revision = `+r.bind(3)+`, updated_at = `+r.bind(4)+`
WHERE owner_user_id = `+r.bind(5)+` AND preview_id = `+r.bind(6),
		scheduled, strings.TrimSpace(savedWorkflowID), savedRevision, r.timestamp(updatedAt),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(previewID),
	)
	return err
}

func (r *Repository) loadDraftVersions(
	ctx context.Context,
	draft *protocol.WorkGraphWorkflowDraft,
) error {
	if draft == nil {
		return nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT revision, `+r.dialect.JSONText("preview_json")+`, created_at
FROM workgraph_workflow_draft_versions
WHERE preview_id = `+r.bind(1)+`
ORDER BY revision ASC`, draft.PreviewID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var version protocol.WorkGraphWorkflowPreviewVersion
		var payload string
		if err = rows.Scan(&version.Revision, &payload, &version.CreatedAt); err != nil {
			return err
		}
		if err = unmarshalJSON(payload, &version.Preview); err != nil {
			return err
		}
		version.CreatedAt = version.CreatedAt.UTC()
		draft.Versions = append(draft.Versions, version)
		if version.Revision == draft.SelectedRevision {
			draft.Preview = version.Preview
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if draft.Preview.PreviewID == "" {
		return fmt.Errorf("WorkGraph Draft %s selected version is missing", draft.PreviewID)
	}
	return nil
}

func (r *Repository) draftSelect() string {
	return `SELECT preview_id, owner_user_id, source_execution_id, source_session_key,
       source_agent_id, source_conversation_id, output_language,
       head_revision, selected_revision, editor_id, editor_agent_id,
       editor_session_key, editor_display_after_unix_milli,
       save_scheduled, saved_workflow_id, saved_revision, expires_at, created_at, updated_at
FROM workgraph_workflow_drafts`
}

func scanDraft(scanner rowScanner) (protocol.WorkGraphWorkflowDraft, error) {
	var draft protocol.WorkGraphWorkflowDraft
	err := scanner.Scan(
		&draft.PreviewID,
		&draft.OwnerUserID,
		&draft.SourceExecutionID,
		&draft.SourceSessionKey,
		&draft.SourceAgentID,
		&draft.SourceConversationID,
		&draft.OutputLanguage,
		&draft.HeadRevision,
		&draft.SelectedRevision,
		&draft.EditorID,
		&draft.EditorAgentID,
		&draft.EditorSessionKey,
		&draft.EditorDisplayAfter,
		&draft.SaveScheduled,
		&draft.SavedWorkflowID,
		&draft.SavedRevision,
		&draft.ExpiresAt,
		&draft.CreatedAt,
		&draft.UpdatedAt,
	)
	if err != nil {
		return protocol.WorkGraphWorkflowDraft{}, err
	}
	draft.ExpiresAt = draft.ExpiresAt.UTC()
	draft.CreatedAt = draft.CreatedAt.UTC()
	draft.UpdatedAt = draft.UpdatedAt.UTC()
	return draft, nil
}
