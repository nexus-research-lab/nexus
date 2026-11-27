// INPUT: 用户确认、旧保存标记、同源历史命名图与多窗口草稿版本。
// OUTPUT: 直接保存、身份隔离、结果核对、选择栅栏与删除后恢复的回归证据。
// POS: 使用真实迁移和 SQLite 的 WorkGraph 保存全链路测试。
package workgraphworkflow

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workflowstore "github.com/nexus-research-lab/nexus/internal/storage/workgraphworkflow"
	"github.com/pressly/goose/v3"
)

func newDurableMetadataTestService(t *testing.T) (*Service, *workflowstore.Repository, *protocol.WorkGraphWorkflowPreview) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "metadata.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	repository := workflowstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	service := NewService(repository, workflowExecutionViewer{view: workflowSourceView()})
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))
	service.SetEditorSessionManager(&workflowEditorSessionManager{})
	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a", OutputLanguage: "zh",
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, repository, preview
}

func TestSaveDialogMetadataSurvivesReopeningEditor(t *testing.T) {
	service, repository, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	request := protocol.StartWorkGraphWorkflowEditorRequest{
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID, OutputLanguage: "zh",
	}
	first, err := service.StartMetadataEditor(ctx, "owner-a", request)
	if err != nil {
		t.Fatal(err)
	}
	request.SlashName, request.Title, request.Description = "renamed", "新标题", "新描述"
	// Reopening the chat editor must preserve the form edits and its existing conversation.
	reopened, err := service.StartMetadataEditor(ctx, "owner-a", request)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.EditorID != first.EditorID || reopened.SessionKey != first.SessionKey ||
		reopened.Preview.SlashName != request.SlashName || reopened.Preview.Title != request.Title ||
		reopened.Preview.Description != request.Description || reopened.Revision != 2 {
		t.Fatalf("form metadata lost on reopen: %#v", reopened)
	}
	plainReopen, err := service.StartMetadataEditor(ctx, "owner-a", protocol.StartWorkGraphWorkflowEditorRequest{
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID, OutputLanguage: "zh",
	})
	if err != nil || plainReopen.Revision != reopened.Revision || plainReopen.Preview.SlashName != request.SlashName {
		t.Fatalf("plain reopen changed saved editor metadata: %#v, err=%v", plainReopen, err)
	}
	applied, err := service.ApplyMetadataEditor(context.Background(), "owner-a", protocol.ApplyWorkGraphWorkflowEditorRequest{
		SourceSessionKey: "session-a", EditorID: reopened.EditorID, Revision: reopened.Revision, SelectedRevision: reopened.SelectedRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ConfirmSave(ctx, "owner-a", protocol.ConfirmWorkGraphWorkflowSaveRequest{
		SourceSessionKey: "session-a", PreviewID: applied.PreviewID, HeadRevision: applied.HeadRevision, SelectedRevision: applied.SelectedRevision,
		SlashName: applied.SlashName, Title: applied.Title, Description: applied.Description,
	}); err != nil {
		t.Fatal(err)
	}
	// A restarted service must save the same confirmed metadata from the durable Draft.
	restarted := NewService(repository, nil)
	saved, err := restarted.SavePreview(ctx, "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		CommandID: "first-save", SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	})
	if err != nil || saved.SlashName != request.SlashName || saved.Title != request.Title {
		t.Fatalf("saved = %#v, err=%v", saved, err)
	}
}

func TestSaveStateChecksActualContentAndKeepsHistoricalCommandsIndependent(t *testing.T) {
	s, repository, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	s.SetMainAgentResolver(workflowMainAgentResolver{agent: &protocol.Agent{AgentID: "main", OwnerUserID: "owner-a"}})
	first, err := s.SavePreview(ctx, "owner-a", protocol.SaveWorkGraphWorkflowRequest{PreviewID: preview.PreviewID, SourceSessionKey: "session-a", CommandID: "first"})
	if err != nil {
		t.Fatal(err)
	}
	next := *preview
	next.SlashName = "chain"
	if _, err = repository.AppendDraftVersion(ctx, "owner-a", preview.PreviewID, 1, next, s.now(), preview.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	// Reproduce the old false saved marker: v2 says chain but the actual command is v1.
	if err = repository.SetDraftSaveState(ctx, "owner-a", preview.PreviewID, false, first.ID, 2, s.now()); err != nil {
		t.Fatal(err)
	}
	state, err := s.GetSaveState(ctx, "owner-a", "session-a", preview.PreviewID)
	if err != nil || state.Status != "unsaved" || state.SavedRevision != 1 || state.Workflow.SlashName != first.SlashName || state.Preview.SlashName != "chain" {
		t.Fatalf("false saved marker trusted: %#v, %v", state, err)
	}
	legacy := *first
	legacy.ID, legacy.SlashName = "older-command", "older-report"
	if _, err = repository.Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	restored, err := s.PreviewSavedWorkflow(ctx, "owner-a", legacy.ID, "zh")
	if err != nil || restored.PreviewID == preview.PreviewID || restored.SlashName != legacy.SlashName {
		t.Fatalf("legacy command borrowed another Draft: %#v, %v", restored, err)
	}
	again, err := s.PreviewSavedWorkflow(ctx, "owner-a", legacy.ID, "zh")
	if err != nil || again.PreviewID != restored.PreviewID {
		t.Fatalf("restored duplicate Draft: %#v, %v", again, err)
	}
	if _, err = s.ConfirmSave(ctx, "owner-a", protocol.ConfirmWorkGraphWorkflowSaveRequest{
		PreviewID: restored.PreviewID, SourceSessionKey: "session-a", HeadRevision: 1, SelectedRevision: 1, SlashName: "older-renamed",
	}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := repository.GetByID(ctx, "owner-a", first.ID)
	if err != nil || unchanged.SlashName != first.SlashName {
		t.Fatalf("renamed unrelated command: %#v, %v", unchanged, err)
	}
	source, err := repository.GetDraftBySource(ctx, "owner-a", "session-a", "execution-a")
	if err != nil || source.PreviewID != preview.PreviewID || source.Preview.SlashName != "chain" {
		t.Fatalf("source Draft was replaced: %#v, %v", source, err)
	}
	if deleted, err := repository.Delete(ctx, "owner-a", first.ID); err != nil || !deleted {
		t.Fatalf("delete: %v %v", deleted, err)
	}
	state, err = s.GetSaveState(ctx, "owner-a", "session-a", preview.PreviewID)
	if err != nil || state.Workflow != nil || state.Status != "unsaved" {
		t.Fatalf("deleted command remained bound: %#v, %v", state, err)
	}
	if _, err = s.ConfirmSave(ctx, "owner-a", protocol.ConfirmWorkGraphWorkflowSaveRequest{
		PreviewID: preview.PreviewID, SourceSessionKey: "session-a", HeadRevision: 2, SelectedRevision: 2,
	}); err != nil {
		t.Fatalf("retained Draft cannot be saved again: %v", err)
	}
}

func TestEditorApplyAndConfirmSaveFenceBothHeadAndSelection(t *testing.T) {
	s, repository, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	editor, err := s.StartMetadataEditor(ctx, "owner-a", protocol.StartWorkGraphWorkflowEditorRequest{PreviewID: preview.PreviewID, SourceSessionKey: "session-a", OutputLanguage: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	next := *preview
	next.SlashName = "chain"
	if _, err = repository.AppendDraftVersion(ctx, "owner-a", preview.PreviewID, 1, next, s.now(), preview.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	request := protocol.ApplyWorkGraphWorkflowEditorRequest{SourceSessionKey: "session-a", EditorID: editor.EditorID, Revision: 1, SelectedRevision: 1}
	if _, err = s.ApplyMetadataEditor(context.Background(), "owner-a", request); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale cached editor applied: %v", err)
	}
	if _, err = repository.SelectDraftVersion(ctx, "owner-a", preview.PreviewID, 2, 1, s.now()); err != nil {
		t.Fatal(err)
	}
	request.Revision, request.SelectedRevision = 2, 2
	if _, err = s.ApplyMetadataEditor(context.Background(), "owner-a", request); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("changed selection applied: %v", err)
	}
	if _, err = s.ConfirmSave(ctx, "owner-a", protocol.ConfirmWorkGraphWorkflowSaveRequest{
		PreviewID: preview.PreviewID, SourceSessionKey: "session-a", HeadRevision: 2, SelectedRevision: 2, SlashName: "chain",
	}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("invisible selected graph saved: %v", err)
	}
	request.SelectedRevision = 1
	applied, err := s.ApplyMetadataEditor(context.Background(), "owner-a", request)
	if err != nil || applied.HeadRevision != 2 || applied.SelectedRevision != 1 || applied.SlashName != preview.SlashName {
		t.Fatalf("exact old version apply: %#v %v", applied, err)
	}
}

func TestDurableEditorWorksWithoutProcessCacheAndHonorsRequestCancellation(t *testing.T) {
	original, repository, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	editor, err := original.StartMetadataEditor(ctx, "owner-a", protocol.StartWorkGraphWorkflowEditorRequest{PreviewID: preview.PreviewID, SourceSessionKey: "session-a", OutputLanguage: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewService(repository, nil)
	revision := editorRevisionRequest(editor.Revision, editor.Preview)
	revision.Title = "重启后的修改"
	next, err := restarted.ReviseEditorPreview(ctx, "owner-a", editor.SessionKey, revision)
	if err != nil || next.Revision != editor.Revision+1 || next.Preview.Title != revision.Title {
		t.Fatalf("direct mutation required cache priming: %+v %v", next, err)
	}
	// A second process must see the committed revision without stale cache authority.
	current, err := original.GetMetadataEditor(ctx, "owner-a", protocol.GetWorkGraphWorkflowEditorRequest{SourceSessionKey: "session-a", EditorID: editor.EditorID})
	if err != nil || current.Revision != next.Revision {
		t.Fatalf("stale editor view: %+v %v", current, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := restarted.GetMetadataEditor(cancelled, "owner-a", protocol.GetWorkGraphWorkflowEditorRequest{SourceSessionKey: "session-a", EditorID: editor.EditorID}); !errors.Is(err, context.Canceled) {
		t.Fatalf("request cancellation lost: %v", err)
	}
	if restarted.RuntimeEditorActive(cancelled, "owner-a", editor.SessionKey) {
		t.Fatal("cancelled request granted editor authority")
	}
}
