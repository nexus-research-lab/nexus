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
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
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
	if hidden, getErr := repository.GetByID(context.Background(), "owner-b", created.ID); getErr != nil || hidden != nil {
		t.Fatalf("cross-owner read = %#v, err=%v", hidden, getErr)
	}
	listed, err := repository.List(context.Background(), "owner-a")
	if err != nil || len(listed) != 1 || listed[0].SlashName != "deep-research" {
		t.Fatalf("list = %#v, err=%v", listed, err)
	}
	deleted, err := repository.Delete(context.Background(), "owner-a", created.ID)
	if err != nil || !deleted {
		t.Fatalf("delete = %v, err=%v", deleted, err)
	}
}
