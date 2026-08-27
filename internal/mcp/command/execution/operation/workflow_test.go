package operation

import (
	"context"
	"strings"
	"testing"

	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type workflowCommandService struct {
	owner   string
	request protocol.SaveWorkGraphWorkflowRequest
}

type workflowEditorCommandService struct {
	active     bool
	owner      string
	sessionKey string
	request    protocol.ReviseWorkGraphWorkflowPreviewRequest
}

func (s *workflowEditorCommandService) RuntimeEditorActive(owner, sessionKey string) bool {
	return s.active && owner == "owner-a" && sessionKey == "editor-session-a"
}

func (s *workflowEditorCommandService) ReviseEditorPreview(
	_ context.Context,
	owner string,
	sessionKey string,
	request protocol.ReviseWorkGraphWorkflowPreviewRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	s.owner = owner
	s.sessionKey = sessionKey
	s.request = request
	return &protocol.WorkGraphWorkflowEditorSession{
		EditorID: "editor-a",
		Revision: request.Revision + 1,
		Preview: protocol.WorkGraphWorkflowPreview{
			Title: request.Title, Nodes: request.Nodes, Dependencies: request.Dependencies,
		},
	}, nil
}

func (s *workflowEditorCommandService) SelectEditorVersionBySession(
	_ context.Context,
	owner string,
	sessionKey string,
	headRevision int64,
	selectedRevision int64,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	s.owner = owner
	s.sessionKey = sessionKey
	return &protocol.WorkGraphWorkflowEditorSession{
		EditorID: "editor-a", Revision: headRevision,
		SelectedRevision: selectedRevision,
	}, nil
}

func (s *workflowCommandService) SavePreview(
	_ context.Context,
	owner string,
	request protocol.SaveWorkGraphWorkflowRequest,
) (*protocol.WorkGraphWorkflow, error) {
	s.owner = owner
	s.request = request
	return &protocol.WorkGraphWorkflow{
		ID: "workflow-a", SlashName: "deep-research",
		Nodes: []protocol.WorkGraphWorkflowNode{{LogicalKey: "research"}},
	}, nil
}

func TestDistillWorkGraphUsesTrustedOwnerSessionAndExactPreview(t *testing.T) {
	service := &workflowCommandService{}
	definition := distillWorkGraphWorkflow(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "session-a",
		RootRoundID: "round-a", RuntimeRoundID: "round-a", AgentID: "agent-a",
		WorkGraphPreviewID: "workgraph_preview_a",
	})
	result, err := definition.Invoke(context.Background(), map[string]any{
		"preview_id": "workgraph_preview_a",
	}, &command.CallContext{RequestID: "workflow-request-1"})
	if err != nil || result.IsError {
		t.Fatalf("invoke result = %#v, err=%v", result, err)
	}
	if service.owner != "owner-a" || service.request.SourceSessionKey != "session-a" || service.request.CommandID != "workflow-request-1" || service.request.PreviewID != "workgraph_preview_a" {
		t.Fatalf("trusted request = owner %q, %#v", service.owner, service.request)
	}
	if result.StructuredContent["command"] != "/deep-research" || result.StructuredContent["outcome"] != "applied" {
		t.Fatalf("result = %#v", result.StructuredContent)
	}
	if !strings.Contains(definition.Description, "原样持久化") ||
		strings.Contains(definition.Description, "Persist the exact") ||
		result.StructuredContent["message"] != "WorkGraph 命令已保存，可以在其他会话中复用。" {
		t.Fatalf("non-Chinese workflow contract or receipt: description=%q result=%#v", definition.Description, result.StructuredContent)
	}
}

func TestDistillWorkGraphRejectsPreviewOutsideHostBinding(t *testing.T) {
	service := &workflowCommandService{}
	operations := BuildWorkGraphDistillation(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "session-a",
		RuntimeSessionKey: "isolated-save-session-a",
		RootRoundID:       "round-a", RuntimeRoundID: "round-a", AgentID: "agent-a",
		WorkGraphPreviewID: "workgraph_preview_a",
	})
	if len(operations) != 1 || operations[0].Name != "distill_workgraph" {
		t.Fatalf("distillation operations = %#v", operations)
	}
	result, err := operations[0].Invoke(context.Background(), map[string]any{
		"preview_id": "workgraph_preview_b",
	}, &command.CallContext{RequestID: "workflow-request-1"})
	if err != nil || !result.IsError || service.request.PreviewID != "" {
		t.Fatalf("mismatched preview result = %#v, request=%#v, err=%v", result, service.request, err)
	}
}

func TestDistillWorkGraphSchemaAcceptsOnlyPreviewID(t *testing.T) {
	schema := distillWorkflowSchema()
	properties := schema["properties"].(map[string]any)
	if len(properties) != 1 || properties["preview_id"] == nil {
		t.Fatalf("properties = %#v", properties)
	}
	required := schema["required"].([]string)
	if len(required) != 1 || required[0] != "preview_id" {
		t.Fatalf("required = %#v", required)
	}
	description := properties["preview_id"].(map[string]any)["description"].(string)
	if !strings.Contains(description, "用户已确认") || strings.Contains(description, "Exact opaque") {
		t.Fatalf("preview_id description = %q", description)
	}
}

func TestReviseWorkGraphPreviewUsesOnlyTrustedEditorIdentity(t *testing.T) {
	service := &workflowEditorCommandService{active: true}
	operations := BuildWorkGraphEditor(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "source-session-a",
		RuntimeSessionKey: "editor-session-a", RootRoundID: "round-a",
	})
	if len(operations) != 2 || operations[0].Name != "revise_workgraph_preview" ||
		operations[1].Name != "select_workgraph_preview_revision" {
		t.Fatalf("editor operations = %#v", operations)
	}
	input := map[string]any{
		"revision": float64(3), "slash_name": "review", "title": "复核流程",
		"description": "先产出，再复核。", "objective": "形成经过复核的交付。",
		"nodes": []any{map[string]any{
			"logical_key": "report", "role": "key", "kind": "integrate",
			"subject": "整合报告", "objective": "汇总内容", "deliverable": "报告",
			"required": true, "terminal": true, "position": float64(0),
		}},
		"dependencies": []any{},
	}
	result, err := operations[0].Invoke(
		context.Background(), input, &command.CallContext{RequestID: "editor-request-1"},
	)
	if err != nil || result.IsError {
		t.Fatalf("invoke result = %#v, err=%v", result, err)
	}
	if service.owner != "owner-a" || service.sessionKey != "editor-session-a" ||
		service.request.Revision != 3 || service.request.SlashName != "review" {
		t.Fatalf("trusted editor mutation = owner %q session %q request %#v", service.owner, service.sessionKey, service.request)
	}
	if result.StructuredContent["outcome"] != "applied" ||
		result.StructuredContent["revision"] != float64(4) {
		t.Fatalf("result = %#v", result.StructuredContent)
	}
}

func TestReviseWorkGraphPreviewSchemaIsClosedAndComplete(t *testing.T) {
	schema := reviseWorkflowPreviewSchema()
	if schema["additionalProperties"] != false {
		t.Fatalf("schema must be closed: %#v", schema)
	}
	properties := schema["properties"].(map[string]any)
	for _, name := range []string{
		"revision", "slash_name", "title", "description", "objective", "completion_criteria", "nodes", "dependencies",
	} {
		if properties[name] == nil {
			t.Fatalf("missing property %q: %#v", name, properties)
		}
	}
	node := properties["nodes"].(map[string]any)["items"].(map[string]any)
	if node["additionalProperties"] != false {
		t.Fatalf("node schema must be closed: %#v", node)
	}
}

type workflowAuthoringCommandService struct {
	owner      string
	sessionKey string
	execution  string
}

func (s *workflowAuthoringCommandService) InspectLibrary(_ context.Context, owner, session string) (*protocol.WorkGraphWorkflowLibrary, error) {
	s.owner, s.sessionKey = owner, session
	return &protocol.WorkGraphWorkflowLibrary{}, nil
}

func (s *workflowAuthoringCommandService) PreviewFromExecution(_ context.Context, owner string, request protocol.PreviewWorkGraphWorkflowRequest) (*protocol.WorkGraphWorkflowPreview, error) {
	s.owner, s.sessionKey, s.execution = owner, request.SourceSessionKey, request.SourceExecutionID
	return &protocol.WorkGraphWorkflowPreview{PreviewID: "preview-a", SourceExecutionID: request.SourceExecutionID}, nil
}

func (s *workflowAuthoringCommandService) GetDraft(_ context.Context, owner, session, previewID string) (*protocol.WorkGraphWorkflowDraft, error) {
	s.owner, s.sessionKey = owner, session
	return &protocol.WorkGraphWorkflowDraft{PreviewID: previewID, HeadRevision: 1, SelectedRevision: 1}, nil
}

func (s *workflowAuthoringCommandService) ReviseDraftPreview(_ context.Context, owner, session string, request protocol.ReviseWorkGraphWorkflowDraftRequest) (*protocol.WorkGraphWorkflowDraft, error) {
	s.owner, s.sessionKey = owner, session
	return &protocol.WorkGraphWorkflowDraft{PreviewID: request.PreviewID, HeadRevision: request.Revision + 1, SelectedRevision: request.Revision + 1}, nil
}

func (s *workflowAuthoringCommandService) SelectDraftRevision(_ context.Context, owner, session, previewID string, head, selected int64) (*protocol.WorkGraphWorkflowDraft, error) {
	s.owner, s.sessionKey = owner, session
	return &protocol.WorkGraphWorkflowDraft{PreviewID: previewID, HeadRevision: head, SelectedRevision: selected}, nil
}

func (s *workflowAuthoringCommandService) SavePreview(_ context.Context, owner string, request protocol.SaveWorkGraphWorkflowRequest) (*protocol.WorkGraphWorkflow, error) {
	s.owner, s.sessionKey = owner, request.SourceSessionKey
	return &protocol.WorkGraphWorkflow{ID: "workflow-a", SlashName: "research"}, nil
}

func TestWorkGraphAuthoringOperationsUseTrustedSessionWithoutActiveExecution(t *testing.T) {
	service := &workflowAuthoringCommandService{}
	operations := BuildWorkGraphAuthoring(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "session-a",
		RuntimeSessionKey: "runtime-a", RootRoundID: "round-a",
	})
	want := []string{
		"inspect_workgraph_library", "extract_workgraph_preview", "get_workgraph_preview",
		"revise_workgraph_preview", "select_workgraph_preview_revision", "save_workgraph_preview",
	}
	if len(operations) != len(want) {
		t.Fatalf("operations = %#v", operations)
	}
	for index, name := range want {
		if operations[index].Name != name {
			t.Fatalf("operation[%d] = %q, want %q", index, operations[index].Name, name)
		}
	}
	result, err := operations[1].Invoke(context.Background(), map[string]any{
		"source_execution_id": "execution-a", "output_language": "zh",
	}, nil)
	if err != nil || result.IsError || service.owner != "owner-a" ||
		service.sessionKey != "session-a" || service.execution != "execution-a" {
		t.Fatalf("extract = %#v, service=%#v, err=%v", result, service, err)
	}
}
