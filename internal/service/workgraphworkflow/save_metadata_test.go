// INPUT: 用户确认、旧保存标记、同源历史命名图与多窗口草稿版本。
// OUTPUT: 直接保存、身份隔离、结果核对、选择栅栏与删除后恢复的回归证据。
// POS: 使用真实迁移和 SQLite 的 WorkGraph 保存全链路测试。
package workgraphworkflow

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
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
	applied, err := service.ApplyMetadataEditor("owner-a", protocol.ApplyWorkGraphWorkflowEditorRequest{
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

func TestSaveReplayDoesNotMarkRenamedDraftAsSaved(t *testing.T) {
	service, repository, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	request := protocol.SaveWorkGraphWorkflowRequest{
		CommandID: "first-save", SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	}
	changeCount := 0
	service.SetChangeNotifier(func(context.Context, string) { changeCount++ })
	created, err := service.SavePreview(ctx, "owner-a", request)
	if err != nil {
		t.Fatal(err)
	}
	next := cloneWorkflowPreview(*preview)
	next.SlashName = "renamed"
	if _, err = repository.AppendDraftVersion(ctx, "owner-a", preview.PreviewID, 1, next, service.now(), preview.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	if err = repository.SetDraftSaveState(ctx, "owner-a", preview.PreviewID, true, created.ID, 1, service.now()); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SavePreview(ctx, "owner-a", request); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("old request returned success for a renamed Draft: %v", err)
	}
	draft, err := repository.GetDraftByID(ctx, "owner-a", preview.PreviewID)
	if err != nil || draft.SavedRevision != 1 || !draft.SaveScheduled {
		t.Fatalf("replay changed the new save claim: %#v, err=%v", draft, err)
	}
	request.CommandID = "rename-save"
	updated, err := service.SavePreview(ctx, "owner-a", request)
	if err != nil || updated.ID != created.ID || updated.SlashName != "renamed" || updated.Version != 2 {
		t.Fatalf("rename save = %#v, err=%v", updated, err)
	}
	old, err := repository.GetBySlashName(ctx, "owner-a", preview.SlashName)
	if err != nil || old != nil {
		t.Fatalf("old Slash still resolves: %#v, err=%v", old, err)
	}
	expanded, err := service.ExpandRuntimePrompt(ctx, "owner-a", "/renamed next task")
	if err != nil || expanded == "/renamed next task" {
		t.Fatalf("renamed command did not expand: %q, err=%v", expanded, err)
	}
	commands, err := service.CommandDescriptors(ctx, "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	foundRename := false
	for _, command := range commands {
		if command.Name == preview.SlashName {
			t.Fatal("command catalog still contains the old name")
		}
		foundRename = foundRename || command.Name == "renamed"
	}
	if !foundRename {
		t.Fatal("command catalog did not publish the renamed WorkGraph")
	}
	replayed, err := service.SavePreview(ctx, "owner-a", request)
	if err != nil || replayed.Version != updated.Version || changeCount != 2 {
		t.Fatalf("rename replay = %#v, changes=%d, err=%v", replayed, changeCount, err)
	}
	draft, err = repository.GetDraftByID(ctx, "owner-a", preview.PreviewID)
	if err != nil || draft.SaveScheduled || draft.SavedRevision != 2 {
		t.Fatalf("completed rename state = %#v, err=%v", draft, err)
	}
}

func TestConfirmedSaveRepairsIncorrectSavedRevision(t *testing.T) {
	service, repository, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	created, err := service.SavePreview(ctx, "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		CommandID: "first-save", SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	})
	if err != nil {
		t.Fatal(err)
	}
	next := cloneWorkflowPreview(*preview)
	next.SlashName = "renamed"
	if _, err = repository.AppendDraftVersion(ctx, "owner-a", preview.PreviewID, 1, next, service.now(), preview.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	if err = repository.SetDraftSaveState(ctx, "owner-a", preview.PreviewID, true, created.ID, 1, service.now()); err != nil {
		t.Fatal(err)
	}
	// Reproduce older data whose save marker advanced without updating the aggregate.
	if err = repository.SetDraftSaveState(ctx, "owner-a", preview.PreviewID, false, created.ID, 2, service.now()); err != nil {
		t.Fatal(err)
	}
	updated, err := service.SavePreview(ctx, "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		CommandID: "confirmed-save", SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	})
	if err != nil || updated.SlashName != "renamed" || updated.ID != created.ID || updated.Version != 2 {
		t.Fatalf("save marker hid unsaved metadata: %#v, err=%v", updated, err)
	}
}

func TestConfirmSaveWritesMetadataImmediatelyAndRecoversPendingDraft(t *testing.T) {
	service, repository, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	// The sketch already exists. Saving must work without any model or runtime.
	service.SetAbstractor(nil)
	service.SetEditorSessionManager(nil)
	if err := repository.SetDraftSaveState(ctx, "owner-a", preview.PreviewID, true, "", 0, service.now()); err != nil {
		t.Fatal(err)
	}
	staleDraft, err := repository.GetDraftByID(ctx, "owner-a", preview.PreviewID)
	if err != nil {
		t.Fatal(err)
	}
	changes := 0
	service.SetChangeNotifier(func(ctx context.Context, owner string) {
		changes++
		draft, readErr := repository.GetDraftByID(ctx, owner, preview.PreviewID)
		if readErr != nil || draft.SaveScheduled || draft.SavedRevision != draft.SelectedRevision {
			t.Fatalf("notification preceded commit: %#v, err=%v", draft, readErr)
		}
	})
	request := protocol.ConfirmWorkGraphWorkflowSaveRequest{
		HeadRevision: 1, SelectedRevision: 1,
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
		SlashName: "/Research-Rich", Title: "并行调研", Description: "生成交叉核验后的报告",
	}
	receipt, err := service.ConfirmSave(ctx, "owner-a", request)
	if err != nil || receipt.Status != "saved" || receipt.Workflow == nil || receipt.Workflow.SlashName != "research-rich" {
		t.Fatalf("direct save receipt = %#v, err=%v", receipt, err)
	}
	draft, err := repository.GetDraftByID(ctx, "owner-a", preview.PreviewID)
	if err != nil || draft.HeadRevision != 2 || draft.SavedRevision != 2 || draft.SaveScheduled ||
		draft.SavedWorkflowID != receipt.Workflow.ID || draft.Versions[0].Preview.SlashName != preview.SlashName {
		t.Fatalf("direct save did not commit immutable metadata and save state: %#v, err=%v", draft, err)
	}
	// A late background save cannot create a second command from its older snapshot.
	late := *receipt.Workflow
	late.ID, late.SlashName = "late-background-workflow", preview.SlashName
	if _, err = repository.SaveDraft(ctx, *staleDraft, *preview, late, 0, service.now()); !errors.Is(err, workflowstore.ErrRevisionConflict) {
		t.Fatalf("late save was not fenced: %v", err)
	}
	repeated, err := service.ConfirmSave(ctx, "owner-a", request)
	if err != nil || repeated.Workflow.ID != receipt.Workflow.ID || repeated.Workflow.Version != 1 || changes != 1 {
		t.Fatalf("repeated save duplicated a command/version: %#v, changes=%d, err=%v", repeated, changes, err)
	}
	request.SlashName = "renamed"
	request.HeadRevision, request.SelectedRevision = draft.HeadRevision, draft.SelectedRevision
	renamed, err := service.ConfirmSave(ctx, "owner-a", request)
	if err != nil || renamed.Workflow.ID != receipt.Workflow.ID || renamed.Workflow.Version != 2 || changes != 2 {
		t.Fatalf("metadata rename = %#v, changes=%d, err=%v", renamed, changes, err)
	}
	items, err := repository.List(ctx, "owner-a")
	if err != nil || len(items) != 1 || items[0].SlashName != "renamed" {
		t.Fatalf("persisted commands = %#v, err=%v", items, err)
	}
	if _, err = service.ConfirmSave(ctx, "owner-b", request); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner save error = %v", err)
	}
	request.SourceSessionKey = "other-session"
	if _, err = service.ConfirmSave(ctx, "owner-a", request); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-session save error = %v", err)
	}
}

func TestDraftSaveRollsBackMetadataWhenCommandNameConflicts(t *testing.T) {
	service, repository, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	receipt, err := service.ConfirmSave(ctx, "owner-a", protocol.ConfirmWorkGraphWorkflowSaveRequest{
		HeadRevision: 1, SelectedRevision: 1,
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	})
	if err != nil {
		t.Fatal(err)
	}
	existing := *receipt.Workflow
	existing.ID, existing.SlashName = "other-workflow", "taken"
	if _, err = repository.Create(ctx, existing); err != nil {
		t.Fatal(err)
	}
	draft, err := repository.GetDraftByID(ctx, "owner-a", preview.PreviewID)
	if err != nil {
		t.Fatal(err)
	}
	next := cloneWorkflowPreview(draft.Preview)
	next.SlashName = "taken"
	updated := *receipt.Workflow
	updated.SlashName, updated.Version = "taken", updated.Version+1
	if _, err = repository.SaveDraft(ctx, *draft, next, updated, receipt.Workflow.Version, service.now()); err == nil {
		t.Fatal("unique Slash conflict was accepted")
	}
	unchanged, err := repository.GetDraftByID(ctx, "owner-a", preview.PreviewID)
	if err != nil || unchanged.HeadRevision != draft.HeadRevision || len(unchanged.Versions) != len(draft.Versions) ||
		unchanged.Preview.SlashName != draft.Preview.SlashName || unchanged.SavedRevision != draft.SavedRevision {
		t.Fatalf("failed aggregate write changed the Draft: %#v, err=%v", unchanged, err)
	}
	_, err = service.ConfirmSave(ctx, "owner-a", protocol.ConfirmWorkGraphWorkflowSaveRequest{
		HeadRevision: 1, SelectedRevision: 1,
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID, SlashName: "goal",
	})
	if !errors.Is(err, ErrNameConflict) {
		t.Fatalf("reserved name save error = %v", err)
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
	if _, err = s.ApplyMetadataEditor("owner-a", request); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale cached editor applied: %v", err)
	}
	if _, err = repository.SelectDraftVersion(ctx, "owner-a", preview.PreviewID, 2, 1, s.now()); err != nil {
		t.Fatal(err)
	}
	request.Revision, request.SelectedRevision = 2, 2
	if _, err = s.ApplyMetadataEditor("owner-a", request); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("changed selection applied: %v", err)
	}
	if _, err = s.ConfirmSave(ctx, "owner-a", protocol.ConfirmWorkGraphWorkflowSaveRequest{
		PreviewID: preview.PreviewID, SourceSessionKey: "session-a", HeadRevision: 2, SelectedRevision: 2, SlashName: "chain",
	}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("invisible selected graph saved: %v", err)
	}
	request.SelectedRevision = 1
	applied, err := s.ApplyMetadataEditor("owner-a", request)
	if err != nil || applied.HeadRevision != 2 || applied.SelectedRevision != 1 || applied.SlashName != preview.SlashName {
		t.Fatalf("exact old version apply: %#v %v", applied, err)
	}
}

func TestEditorRevisionReceiptRemainsExactDuringConcurrentReads(t *testing.T) {
	s, _, preview := newDurableMetadataTestService(t)
	ctx := context.Background()
	editor, err := s.StartMetadataEditor(ctx, "owner-a", protocol.StartWorkGraphWorkflowEditorRequest{
		PreviewID: preview.PreviewID, SourceSessionKey: "session-a", OutputLanguage: "zh",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := protocol.GetWorkGraphWorkflowEditorRequest{SourceSessionKey: "session-a", EditorID: editor.EditorID}
	stop := make(chan struct{})
	errorsFound := make(chan error, 3)
	var readers sync.WaitGroup
	for range 3 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					if _, readErr := s.GetMetadataEditor("owner-a", request); readErr != nil {
						errorsFound <- readErr
						return
					}
				}
			}
		}()
	}
	defer func() {
		close(stop)
		readers.Wait()
		close(errorsFound)
		for readErr := range errorsFound {
			t.Error(readErr)
		}
	}()
	for index := range 12 {
		current, readErr := s.GetMetadataEditor("owner-a", request)
		if readErr != nil {
			t.Fatal(readErr)
		}
		revision := editorRevisionRequest(current.Revision, current.Preview)
		revision.Title = "编辑版本 " + strconv.Itoa(index)
		next, reviseErr := s.ReviseEditorPreview(ctx, "owner-a", editor.SessionKey, revision)
		if reviseErr != nil {
			t.Fatal(reviseErr)
		}
		if next.Revision != revision.Revision+1 || next.Preview.Title != revision.Title {
			t.Fatalf("revision receipt regressed during concurrent reads: %#v", next)
		}
	}
}
