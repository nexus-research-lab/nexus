package workgraphworkflow

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRepositoryPersistsWorkflowAggregateAndOwnerScope(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "workgraph-workflows.db")
	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ensureGooseSQLiteDialect(t)
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	now := time.Date(2026, 8, 20, 4, 5, 6, 0, time.UTC)
	created, err := repository.Create(context.Background(), protocol.WorkGraphWorkflow{
		ID: "workflow-a", OwnerUserID: "owner-a", SlashName: "deep-research",
		Title: "Deep research", SourceExecutionID: "execution-a",
		SourceSessionKey: "session-a", Objective: "Research", Version: 1,
		CreatedAt: now, UpdatedAt: now,
		Nodes: []protocol.WorkGraphWorkflowNode{
			{LogicalKey: "review", SourceWorkItemID: "work-review", Role: protocol.WorkGraphWorkflowNodeCollaboration, Kind: protocol.WorkItemKindReview, Subject: "Review", Objective: "Review findings", Deliverable: "Reviewed brief", Required: true, Terminal: true, ParentLogicalKey: "research", Position: 2},
			{LogicalKey: "research", SourceWorkItemID: "work-research", Role: protocol.WorkGraphWorkflowNodeKey, Kind: protocol.WorkItemKindProduce, Subject: "Research", Objective: "Collect findings", Deliverable: "Brief", Required: true, Position: 1},
		},
		Dependencies: []protocol.WorkGraphWorkflowDependency{{
			LogicalKey: "review", DependsOnLogicalKey: "research", Kind: protocol.WorkDependencyHard,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Nodes) != 2 || created.Nodes[1].ParentLogicalKey != "research" || len(created.Dependencies) != 1 {
		t.Fatalf("created aggregate = %#v", created)
	}
	updatedInput := *created
	updatedInput.Title = "Reviewed research"
	updatedInput.Version = 2
	updatedInput.UpdatedAt = now.Add(time.Hour)
	updatedInput.Nodes = append([]protocol.WorkGraphWorkflowNode(nil), created.Nodes...)
	updatedInput.Nodes[0].Subject = "Reviewed evidence"
	updated, err := repository.Update(context.Background(), updatedInput)
	if err != nil || updated.Version != 2 || updated.Title != "Reviewed research" ||
		updated.Nodes[0].Subject != "Reviewed evidence" || len(updated.Dependencies) != 1 {
		t.Fatalf("updated aggregate = %#v, err=%v", updated, err)
	}
	if hidden, getErr := repository.GetByID(context.Background(), "owner-b", created.ID); getErr != nil || hidden != nil {
		t.Fatalf("cross-owner read = %#v, err=%v", hidden, getErr)
	}
	listCtx, cancelList := context.WithTimeout(context.Background(), time.Second)
	defer cancelList()
	listed, err := repository.List(listCtx, "owner-a")
	if err != nil || len(listed) != 1 || listed[0].SlashName != "deep-research" {
		t.Fatalf("list = %#v, err=%v", listed, err)
	}
	deleted, err := repository.Delete(context.Background(), "owner-a", created.ID)
	if err != nil || !deleted {
		t.Fatalf("delete = %v, err=%v", deleted, err)
	}
}

func TestRepositoryPersistsRecoverableDraftVersionsAndSelection(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "workgraph-drafts.db")
	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ensureGooseSQLiteDialect(t)
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 1, 2, 3, 0, time.UTC)
	preview := protocol.WorkGraphWorkflowPreview{
		PreviewID: "preview-a", SlashName: "research", Title: "Research",
		Description: "Reusable research", SourceExecutionID: "execution-a",
		SourceSessionKey: "session-a", Objective: "Research a topic",
		Nodes: []protocol.WorkGraphWorkflowNode{{
			LogicalKey: "report", Role: protocol.WorkGraphWorkflowNodeKey,
			Kind: protocol.WorkItemKindProduce, Subject: "Report", Objective: "Write",
			Deliverable: "Report", Required: true, Terminal: true,
		}},
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	created, err := repository.CreateDraft(ctx, protocol.WorkGraphWorkflowDraft{
		PreviewID: "preview-a", OwnerUserID: "owner-a",
		SourceExecutionID: "execution-a", SourceSessionKey: "session-a",
		SourceAgentID: "agent-source", OutputLanguage: "en",
		HeadRevision: 1, SelectedRevision: 1, Preview: preview,
		Versions: []protocol.WorkGraphWorkflowPreviewVersion{{
			Revision: 1, Preview: preview, CreatedAt: now,
		}},
		ExpiresAt: preview.ExpiresAt, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil || created.HeadRevision != 1 || len(created.Versions) != 1 {
		t.Fatalf("create Draft = %#v, err=%v", created, err)
	}
	bound, err := repository.BindDraftEditor(
		ctx, "owner-a", "preview-a", "editor-a", "agent-main",
		"agent:agent-main:websocket:dm:editor-a", now.UnixMilli(), now.Add(time.Minute),
	)
	if err != nil || bound.EditorAgentID != "agent-main" {
		t.Fatalf("bind editor = %#v, err=%v", bound, err)
	}
	preview.Title = "Research with review"
	updated, err := repository.AppendDraftVersion(
		ctx, "owner-a", "preview-a", 1, preview,
		now.Add(2*time.Minute), preview.ExpiresAt,
	)
	if err != nil || updated.HeadRevision != 2 || updated.SelectedRevision != 2 ||
		len(updated.Versions) != 2 || updated.Preview.Title != "Research with review" {
		t.Fatalf("append version = %#v, err=%v", updated, err)
	}
	selected, err := repository.SelectDraftVersion(
		ctx, "owner-a", "preview-a", 2, 1, now.Add(3*time.Minute),
	)
	if err != nil || selected.HeadRevision != 2 || selected.SelectedRevision != 1 ||
		selected.Preview.Title != "Research" {
		t.Fatalf("select version = %#v, err=%v", selected, err)
	}
	recovered, err := repository.GetDraftByEditorSession(
		ctx, "owner-a", "agent:agent-main:websocket:dm:editor-a",
	)
	if err != nil || recovered == nil || recovered.SelectedRevision != 1 || len(recovered.Versions) != 2 {
		t.Fatalf("recover editor Draft = %#v, err=%v", recovered, err)
	}
	renewedExpiry := now.Add(60 * 24 * time.Hour)
	if err = repository.RenewDraftLease(ctx, "owner-a", "preview-a", renewedExpiry, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	renewed, err := repository.GetDraftByID(ctx, "owner-a", "preview-a")
	if err != nil || renewed == nil || !renewed.ExpiresAt.Equal(renewedExpiry) {
		t.Fatalf("renew Draft lease = %#v, err=%v", renewed, err)
	}
	if err = repository.SetDraftSaveState(ctx, "owner-a", "preview-a", false, "workflow-a", 1, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	linked, err := repository.GetDraftBySavedWorkflowID(ctx, "owner-a", "workflow-a")
	if err != nil || linked == nil || linked.SavedRevision != 1 || linked.SavedWorkflowID != "workflow-a" {
		t.Fatalf("saved workflow Draft = %#v, err=%v", linked, err)
	}
	if hidden, getErr := repository.GetDraftByID(ctx, "owner-b", "preview-a"); getErr != nil || hidden != nil {
		t.Fatalf("cross-owner Draft read = %#v, err=%v", hidden, getErr)
	}
}
