package workgraphworkflow

import (
	"context"
	"errors"
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

type workflowAbstractor func(context.Context, string, AbstractionInput) (AbstractionOutput, error)

func (f workflowAbstractor) Abstract(ctx context.Context, owner string, input AbstractionInput) (AbstractionOutput, error) {
	return f(ctx, owner, input)
}

type workflowEditorSessionManager struct {
	created []EditorSessionCreateRequest
	deleted []string
}

func (m *workflowEditorSessionManager) CreateWorkGraphEditorSession(_ context.Context, request EditorSessionCreateRequest) (*protocol.Session, error) {
	m.created = append(m.created, request)
	return &protocol.Session{AgentID: request.AgentID, SessionKey: request.TargetSessionKey}, nil
}

func (m *workflowEditorSessionManager) DeleteWorkGraphEditorSession(_ context.Context, sessionKey string) error {
	m.deleted = append(m.deleted, sessionKey)
	return nil
}

type workflowSaveRoundRecorder struct {
	requests []SaveRoundRequest
	err      error
}

func (r *workflowSaveRoundRecorder) DispatchWorkGraphSave(_ context.Context, request SaveRoundRequest) error {
	r.requests = append(r.requests, request)
	return r.err
}

func reusableTestAbstractor(_ context.Context, _ string, input AbstractionInput) (AbstractionOutput, error) {
	selected := map[string]protocol.WorkGraphWorkflowNodeRole{
		"research": protocol.WorkGraphWorkflowNodeKey,
		"review":   protocol.WorkGraphWorkflowNodeCollaboration,
	}
	nodes := make([]AbstractedNode, 0, len(selected))
	for _, node := range input.Nodes {
		role, keep := selected[node.LogicalKey]
		if !keep {
			continue
		}
		nodes = append(nodes, AbstractedNode{
			LogicalKey: node.LogicalKey, Role: role,
			Subject:            "Reusable " + node.LogicalKey,
			Objective:          "Complete the reusable responsibility",
			Deliverable:        "Reusable deliverable",
			AcceptanceCriteria: []string{"Meets the reusable acceptance contract"},
		})
	}
	return AbstractionOutput{
		SlashName: "deep-research", Title: "Reusable research",
		Description:        "Reusable across related requests",
		Objective:          "Produce and verify a reusable result",
		CompletionCriteria: []string{"The reusable result is complete and reviewed"},
		Nodes:              nodes,
	}, nil
}

func TestPreviewThenSaveKeepsExactModelExtractedSketch(t *testing.T) {
	repository := &workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)}
	service := NewService(repository, workflowExecutionViewer{view: workflowSourceView()})
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))
	changeCount := 0
	service.SetChangeNotifier(func(context.Context, string) { changeCount++ })
	service.now = func() time.Time { return time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC) }

	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.createCount != 0 || len(repository.items) != 0 {
		t.Fatalf("preview persisted data: creates=%d items=%d", repository.createCount, len(repository.items))
	}
	if preview.SlashName != "research" || len(preview.Nodes) != 2 || len(preview.Dependencies) != 1 {
		t.Fatalf("preview = %#v", preview)
	}
	if preview.Nodes[0].LogicalKey != "research" || preview.Nodes[1].Role != protocol.WorkGraphWorkflowNodeCollaboration {
		t.Fatalf("nodes = %#v", preview.Nodes)
	}
	if preview.Dependencies[0].LogicalKey != "review" || preview.Dependencies[0].DependsOnLogicalKey != "research" {
		t.Fatalf("projected dependencies = %#v", preview.Dependencies)
	}
	if strings.Contains(preview.Description, "tool-secret") {
		t.Fatalf("runtime tool fact leaked into preview: %q", preview.Description)
	}

	created, err := service.SavePreview(context.Background(), "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		CommandID: "workflow-distill-request-1", SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Objective != preview.Objective || len(created.Nodes) != len(preview.Nodes) || repository.createCount != 1 {
		t.Fatalf("created = %#v, creates=%d", created, repository.createCount)
	}
	replayed, err := service.SavePreview(context.Background(), "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		CommandID: "workflow-distill-request-1", SourceSessionKey: "session-a", PreviewID: "expired-or-missing",
	})
	if err != nil || replayed.ID != created.ID || repository.createCount != 1 {
		t.Fatalf("idempotent replay = %#v, err=%v, creates=%d", replayed, err, repository.createCount)
	}
	if changeCount != 1 {
		t.Fatalf("directory change count = %d, want 1", changeCount)
	}
}

func TestPreviewPrefersAvailableSingleWordSlashName(t *testing.T) {
	repository := &workflowMemoryRepository{items: map[string]protocol.WorkGraphWorkflow{
		"existing-research": {ID: "existing-research", OwnerUserID: "owner-a", SlashName: "research"},
	}}
	service := NewService(repository, workflowExecutionViewer{view: workflowSourceView()})
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))

	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.SlashName != "deep" {
		t.Fatalf("slash name = %q, want next available single word", preview.SlashName)
	}
}

func TestPreviewUsesTwoWordSlashNameOnlyAfterSingleWordConflicts(t *testing.T) {
	repository := &workflowMemoryRepository{items: map[string]protocol.WorkGraphWorkflow{
		"existing-research": {ID: "existing-research", OwnerUserID: "owner-a", SlashName: "research"},
		"existing-deep":     {ID: "existing-deep", OwnerUserID: "owner-a", SlashName: "deep"},
	}}
	service := NewService(repository, workflowExecutionViewer{view: workflowSourceView()})
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))

	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.SlashName != "deep-research" {
		t.Fatalf("slash name = %q, want two-word fallback", preview.SlashName)
	}
}

func TestPreviewPassesRequestedOutputLanguageToAbstraction(t *testing.T) {
	repository := &workflowMemoryRepository{items: map[string]protocol.WorkGraphWorkflow{
		"existing": {ID: "existing", OwnerUserID: "owner-a", SlashName: "component-integrate-verify"},
	}}
	service := NewService(
		repository,
		workflowExecutionViewer{view: workflowSourceView()},
	)
	gotLanguage := ""
	gotExistingSlashNames := []string(nil)
	service.SetAbstractor(workflowAbstractor(func(ctx context.Context, owner string, input AbstractionInput) (AbstractionOutput, error) {
		gotLanguage = input.OutputLanguage
		gotExistingSlashNames = input.ExistingSlashNames
		return reusableTestAbstractor(ctx, owner, input)
	}))
	_, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a", OutputLanguage: "en-US",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotLanguage != "en" {
		t.Fatalf("output language = %q, want en", gotLanguage)
	}
	joinedNames := strings.Join(gotExistingSlashNames, ",")
	if !strings.Contains(joinedNames, "component-integrate-verify") || !strings.Contains(joinedNames, "workgraph") {
		t.Fatalf("existing slash names = %q", joinedNames)
	}
}

func TestSavePreviewRejectsWrongSessionAndExpiredPreview(t *testing.T) {
	currentTime := time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC)
	service := NewService(
		&workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)},
		workflowExecutionViewer{view: workflowSourceView()},
	)
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))
	service.now = func() time.Time { return currentTime }
	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.SavePreview(context.Background(), "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		SourceSessionKey: "session-b", PreviewID: preview.PreviewID,
	})
	if err != ErrNotFound {
		t.Fatalf("wrong-session error = %v, want ErrNotFound", err)
	}
	currentTime = currentTime.Add(workflowPreviewTTL + time.Second)
	_, err = service.SavePreview(context.Background(), "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	})
	if err != ErrNotFound {
		t.Fatalf("expired-preview error = %v, want ErrNotFound", err)
	}
}

func TestScheduleSaveDispatchesOneHiddenPromptWithoutGraphContent(t *testing.T) {
	service := NewService(
		&workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)},
		workflowExecutionViewer{view: workflowSourceView()},
	)
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))
	dispatcher := &workflowSaveRoundRecorder{}
	service.SetSaveRoundDispatcher(dispatcher)
	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := protocol.ScheduleWorkGraphWorkflowSaveRequest{
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
		SlashName: "evidence-review", Title: "证据审查", Description: "整理证据并完成独立复核",
	}
	receipt, err := service.ScheduleSave(context.Background(), "owner-a", request)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "scheduled" || receipt.PreviewID != preview.PreviewID || len(dispatcher.requests) != 1 {
		t.Fatalf("receipt=%#v requests=%#v", receipt, dispatcher.requests)
	}
	dispatched := dispatcher.requests[0]
	if dispatched.OwnerUserID != "owner-a" || dispatched.SessionKey != "session-a" || dispatched.PreviewID != preview.PreviewID {
		t.Fatalf("dispatch request = %#v", dispatched)
	}
	for _, expected := range []string{"execution-orchestrator Skill", "distill_workgraph", preview.PreviewID, "/evidence-review"} {
		if !strings.Contains(dispatched.Prompt, expected) {
			t.Fatalf("background prompt missing %q: %s", expected, dispatched.Prompt)
		}
	}
	for _, forbidden := range []string{"work-research", "Reusable research", "tool-secret", "attempt-secret"} {
		if strings.Contains(dispatched.Prompt, forbidden) {
			t.Fatalf("background prompt leaked graph content %q: %s", forbidden, dispatched.Prompt)
		}
	}
	if _, err = service.ScheduleSave(context.Background(), "owner-a", request); err != nil || len(dispatcher.requests) != 1 {
		t.Fatalf("repeated schedule err=%v requests=%d, want idempotent acceptance", err, len(dispatcher.requests))
	}
	created, err := service.SavePreview(context.Background(), "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.SlashName != request.SlashName || created.Title != request.Title || created.Description != request.Description {
		t.Fatalf("saved metadata = (%q, %q, %q)", created.SlashName, created.Title, created.Description)
	}
	if _, err = service.ScheduleSave(context.Background(), "owner-a", request); err != nil || len(dispatcher.requests) != 1 {
		t.Fatalf("post-save replay err=%v requests=%d, want idempotent receipt", err, len(dispatcher.requests))
	}
}

func TestScheduleSaveReleasesClaimWhenDispatchFails(t *testing.T) {
	service := NewService(
		&workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)},
		workflowExecutionViewer{view: workflowSourceView()},
	)
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))
	dispatcher := &workflowSaveRoundRecorder{err: errors.New("dispatch failed")}
	service.SetSaveRoundDispatcher(dispatcher)
	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := protocol.ScheduleWorkGraphWorkflowSaveRequest{SourceSessionKey: "session-a", PreviewID: preview.PreviewID}
	if _, err = service.ScheduleSave(context.Background(), "owner-a", request); err == nil {
		t.Fatal("dispatch failure was accepted")
	}
	dispatcher.err = nil
	if _, err = service.ScheduleSave(context.Background(), "owner-a", request); err != nil || len(dispatcher.requests) != 2 {
		t.Fatalf("retry err=%v requests=%d, want released claim", err, len(dispatcher.requests))
	}
}

func TestMetadataEditorAppliesValidatedGraphRevisionAndDiscardsTransientSession(t *testing.T) {
	service := NewService(
		&workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)},
		workflowExecutionViewer{view: workflowSourceView()},
	)
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))
	sessions := &workflowEditorSessionManager{}
	service.SetEditorSessionManager(sessions)
	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a", OutputLanguage: "zh",
	})
	if err != nil {
		t.Fatal(err)
	}
	editor, err := service.StartMetadataEditor(context.Background(), "owner-a", protocol.StartWorkGraphWorkflowEditorRequest{
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID, OutputLanguage: "zh",
		SlashName: "research-review", Title: "研究审查", Description: "整理材料并复核",
	})
	if err != nil {
		t.Fatal(err)
	}
	if editor.Revision != 1 || editor.Preview.Title != "研究审查" || editor.SessionKey == "" || len(sessions.created) != 1 {
		t.Fatalf("started editor = %#v", editor)
	}
	if sessions.created[0].SourceSessionKey != "session-a" {
		t.Fatalf("transient fork request = %#v", sessions.created[0])
	}
	nodes := cloneWorkflowNodes(editor.Preview.Nodes)
	for index := range nodes {
		if nodes[index].LogicalKey == "review" {
			nodes[index].Terminal = false
		}
	}
	nodes = append(nodes, protocol.WorkGraphWorkflowNode{
		LogicalKey: "publish", Role: protocol.WorkGraphWorkflowNodeKey,
		Kind: protocol.WorkItemKindProduce, Subject: "发布简报", Objective: "形成可交付简报",
		Deliverable: "最终简报", AcceptanceCriteria: []string{"内容完整"}, Required: true, Terminal: true,
	})
	dependencies := append([]protocol.WorkGraphWorkflowDependency(nil), editor.Preview.Dependencies...)
	dependencies = append(dependencies, protocol.WorkGraphWorkflowDependency{
		LogicalKey: "publish", DependsOnLogicalKey: "review", Kind: protocol.WorkDependencyHard,
	})
	revised, err := service.ReviseEditorPreview(context.Background(), "owner-a", editor.SessionKey, protocol.ReviseWorkGraphWorkflowPreviewRequest{
		Revision: editor.Revision, SlashName: "evidence-brief", Title: "证据简报",
		Description: "整合证据并进行独立复核", Objective: editor.Preview.Objective,
		CompletionCriteria: editor.Preview.CompletionCriteria, Nodes: nodes, Dependencies: dependencies,
	})
	if err != nil {
		t.Fatal(err)
	}
	if revised.Revision != 2 || revised.Preview.Title != "证据简报" || len(revised.Preview.Nodes) != 3 {
		t.Fatalf("revised editor = %#v", revised)
	}
	unchanged, err := service.getPreview("owner-a", "session-a", preview.PreviewID)
	if err != nil || len(unchanged.Nodes) != 2 {
		t.Fatalf("unapplied preview = %#v, err=%v", unchanged, err)
	}
	if _, err = service.ReviseEditorPreview(context.Background(), "owner-a", editor.SessionKey, protocol.ReviseWorkGraphWorkflowPreviewRequest{
		Revision: 1, SlashName: "stale", Title: "过期", Description: "过期版本",
		Objective: editor.Preview.Objective, Nodes: nodes, Dependencies: dependencies,
	}); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("stale revision error = %v", err)
	}
	applied, err := service.ApplyMetadataEditor("owner-a", protocol.ApplyWorkGraphWorkflowEditorRequest{
		SourceSessionKey: "session-a", EditorID: editor.EditorID, Revision: revised.Revision,
	})
	if err != nil || len(applied.Nodes) != 3 || applied.Nodes[2].LogicalKey != "publish" {
		t.Fatalf("applied preview = %#v, err=%v", applied, err)
	}
	closed, err := service.CloseMetadataEditor(context.Background(), "owner-a", "session-a", editor.EditorID)
	if err != nil || !closed || len(sessions.deleted) != 1 || sessions.deleted[0] != editor.SessionKey {
		t.Fatal("editor was not closed")
	}
}

func TestExpandRuntimePromptMaterializesFreshGraphWithoutRunFacts(t *testing.T) {
	repository := &workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)}
	service := NewService(repository, workflowExecutionViewer{view: workflowSourceView()})
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))
	preview, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.SavePreview(context.Background(), "owner-a", protocol.SaveWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", PreviewID: preview.PreviewID,
	})
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := service.ExpandRuntimePrompt(context.Background(), "owner-a", "/research compare storage engines")
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

func TestPreviewFailsClosedWithoutBackgroundAbstractionOrCompletedGraph(t *testing.T) {
	view := workflowSourceView()
	service := NewService(
		&workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)},
		workflowExecutionViewer{view: view},
	)
	_, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected unavailable abstractor error, got %v", err)
	}
	service.SetAbstractor(workflowAbstractor(reusableTestAbstractor))
	view.Status = protocol.ExecutionStatusActive
	_, err = service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err == nil || !strings.Contains(err.Error(), "only completed") {
		t.Fatalf("expected completed-only error, got %v", err)
	}
}

func TestPreviewRejectsInventedModelNode(t *testing.T) {
	service := NewService(
		&workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)},
		workflowExecutionViewer{view: workflowSourceView()},
	)
	service.SetAbstractor(workflowAbstractor(func(context.Context, string, AbstractionInput) (AbstractionOutput, error) {
		return AbstractionOutput{
			SlashName: "invented-graph", Title: "Invented", Description: "Invalid", Objective: "Invalid",
			Nodes: []AbstractedNode{{
				LogicalKey: "invented", Role: protocol.WorkGraphWorkflowNodeKey,
				Subject: "Invented", Objective: "Invented", Deliverable: "Invented",
			}},
		}, nil
	}))
	_, err := service.PreviewFromExecution(context.Background(), "owner-a", protocol.PreviewWorkGraphWorkflowRequest{
		SourceSessionKey: "session-a", SourceExecutionID: "execution-a",
	})
	if err == nil || !strings.Contains(err.Error(), "invented") {
		t.Fatalf("expected invented-node error, got %v", err)
	}
}

func workflowSourceView() *protocol.ExecutionView {
	return &protocol.ExecutionView{
		ID: "execution-a", SessionKey: "session-a", Status: protocol.ExecutionStatusCompleted,
		CoordinatorAgentID: "agent-a",
		Objective:          "Research and review", Plan: &protocol.ExecutionPlanView{ID: "plan-a"},
		WorkItems: []protocol.ExecutionWorkItemView{
			{ID: "work-research", LogicalKey: "research", Kind: protocol.WorkItemKindProduce, Subject: "Research", Objective: "Collect evidence", Deliverable: "Evidence brief", Required: true, Position: 1, Status: protocol.ExecutionWorkItemViewAccepted},
			{ID: "work-incidental", LogicalKey: "incidental", Kind: protocol.WorkItemKindVerify, Subject: "One-off formatting", Objective: "Task-specific cleanup", Deliverable: "Temporary formatting", Position: 2, DependencyIDs: []string{"work-research"}, Status: protocol.ExecutionWorkItemViewAccepted},
			{ID: "work-review", LogicalKey: "review", Kind: protocol.WorkItemKindReview, Subject: "Independent review", Objective: "Challenge evidence", Deliverable: "Reviewed brief", Required: true, Terminal: true, Position: 3, DependencyIDs: []string{"work-incidental"}, AssignmentStrategy: protocol.AssignmentStrategyRoomMember, ReviewStatus: "accepted", Status: protocol.ExecutionWorkItemViewAccepted},
		},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{{
			ID: "tool-secret", Kind: protocol.ExecutionGraphNodeTool,
			Runs: []protocol.ExecutionGraphNodeRunView{{ID: "attempt-secret"}},
		}}},
	}
}
