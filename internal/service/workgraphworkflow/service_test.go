package workgraphworkflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type workflowMemoryRepository struct {
	items       map[string]protocol.WorkGraphWorkflow
	createCount int
}

func (r *workflowMemoryRepository) Create(_ context.Context, workflow protocol.WorkGraphWorkflow) (*protocol.WorkGraphWorkflow, error) {
	r.createCount++
	if r.items == nil {
		r.items = make(map[string]protocol.WorkGraphWorkflow)
	}
	r.items[workflow.ID] = workflow
	result := workflow
	return &result, nil
}

func (r *workflowMemoryRepository) List(_ context.Context, owner string) ([]protocol.WorkGraphWorkflow, error) {
	result := make([]protocol.WorkGraphWorkflow, 0)
	for _, item := range r.items {
		if item.OwnerUserID == owner {
			result = append(result, item)
		}
	}
	return result, nil
}

func (r *workflowMemoryRepository) GetByID(_ context.Context, owner, id string) (*protocol.WorkGraphWorkflow, error) {
	item, ok := r.items[id]
	if !ok || item.OwnerUserID != owner {
		return nil, nil
	}
	return &item, nil
}

func (r *workflowMemoryRepository) GetBySlashName(_ context.Context, owner, name string) (*protocol.WorkGraphWorkflow, error) {
	for _, item := range r.items {
		if item.OwnerUserID == owner && item.SlashName == name {
			return &item, nil
		}
	}
	return nil, nil
}

func (r *workflowMemoryRepository) Delete(_ context.Context, owner, id string) (bool, error) {
	item, ok := r.items[id]
	if !ok || item.OwnerUserID != owner {
		return false, nil
	}
	delete(r.items, id)
	return true, nil
}

type workflowExecutionViewer struct {
	view *protocol.ExecutionView
}

func (v workflowExecutionViewer) GetView(_ context.Context, owner, session, execution string) (*protocol.ExecutionView, error) {
	if v.view == nil || owner != "owner-a" || v.view.SessionKey != session || v.view.ID != execution {
		return nil, ErrNotFound
	}
	return v.view, nil
}

func TestCreateFromExecutionKeepsOnlySelectedSemanticGraph(t *testing.T) {
	repository := &workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)}
	view := workflowSourceView()
	service := NewService(repository, workflowExecutionViewer{view: view})
	service.now = func() time.Time { return time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC) }
	created, err := service.CreateFromExecution(context.Background(), "owner-a", protocol.CreateWorkGraphWorkflowRequest{
		CommandID: "workflow-distill-request-1", SourceSessionKey: "session-a",
		SourceExecutionID: "execution-a", SlashName: "/deep-research",
		Title: "Deep research", Nodes: []protocol.WorkGraphWorkflowNodeSelection{
			{WorkItemID: "work-research", Role: protocol.WorkGraphWorkflowNodeKey},
			{WorkItemID: "work-review", Role: protocol.WorkGraphWorkflowNodeCollaboration},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.SlashName != "deep-research" || len(created.Nodes) != 2 || len(created.Dependencies) != 1 {
		t.Fatalf("workflow = %#v", created)
	}
	if created.Nodes[0].SourceWorkItemID != "work-research" || created.Nodes[1].Role != protocol.WorkGraphWorkflowNodeCollaboration {
		t.Fatalf("nodes = %#v", created.Nodes)
	}
	if created.Dependencies[0].LogicalKey != "review" || created.Dependencies[0].DependsOnLogicalKey != "research" {
		t.Fatalf("dependencies = %#v", created.Dependencies)
	}
	if strings.Contains(created.Description, "tool-secret") {
		t.Fatalf("runtime tool fact leaked into workflow: %q", created.Description)
	}

	replayed, err := service.CreateFromExecution(context.Background(), "owner-a", protocol.CreateWorkGraphWorkflowRequest{
		CommandID: "workflow-distill-request-1", SourceSessionKey: "session-a",
		SourceExecutionID: "execution-a", SlashName: "deep-research",
		Title: "Deep research", Nodes: []protocol.WorkGraphWorkflowNodeSelection{
			{WorkItemID: "work-research", Role: protocol.WorkGraphWorkflowNodeKey},
		},
	})
	if err != nil || replayed.ID != created.ID || repository.createCount != 1 {
		t.Fatalf("idempotent replay = %#v, err=%v, creates=%d", replayed, err, repository.createCount)
	}
}

func TestExpandRuntimePromptMaterializesFreshGraphWithoutRunFacts(t *testing.T) {
	repository := &workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)}
	service := NewService(repository, workflowExecutionViewer{view: workflowSourceView()})
	created, err := service.CreateFromExecution(context.Background(), "owner-a", protocol.CreateWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
		SlashName: "deep-research", Title: "Deep research",
		Nodes: []protocol.WorkGraphWorkflowNodeSelection{{WorkItemID: "work-research", Role: protocol.WorkGraphWorkflowNodeKey}},
	})
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := service.ExpandRuntimePrompt(context.Background(), "owner-a", "/deep-research compare storage engines")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"execution-orchestrator", "fresh managed WorkGraph", "compare storage engines", created.Nodes[0].LogicalKey} {
		if !strings.Contains(expanded, expected) {
			t.Fatalf("expanded prompt missing %q: %s", expected, expanded)
		}
	}
	for _, forbidden := range []string{"work-research", "tool-secret", "attempt-secret"} {
		if strings.Contains(expanded, forbidden) {
			t.Fatalf("expanded prompt leaked %q: %s", forbidden, expanded)
		}
	}
}

func workflowSourceView() *protocol.ExecutionView {
	return &protocol.ExecutionView{
		ID: "execution-a", SessionKey: "session-a",
		Objective: "Research and review", Plan: &protocol.ExecutionPlanView{ID: "plan-a"},
		WorkItems: []protocol.ExecutionWorkItemView{
			{ID: "work-research", LogicalKey: "research", Kind: protocol.WorkItemKindProduce, Subject: "Research", Objective: "Collect evidence", Deliverable: "Evidence brief", Required: true, Position: 1},
			{ID: "work-review", LogicalKey: "review", Kind: protocol.WorkItemKindReview, Subject: "Independent review", Objective: "Challenge evidence", Deliverable: "Reviewed brief", Required: true, Terminal: true, Position: 2, DependencyIDs: []string{"work-research"}},
			{ID: "work-incidental", LogicalKey: "incidental", Kind: protocol.WorkItemKindVerify, Subject: "Incidental", Objective: "Not selected", Deliverable: "No copy", Position: 3},
		},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{{
			ID: "tool-secret", Kind: protocol.ExecutionGraphNodeTool,
			Runs: []protocol.ExecutionGraphNodeRunView{{ID: "attempt-secret"}},
		}}},
	}
}
