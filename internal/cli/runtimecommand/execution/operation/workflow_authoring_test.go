package operation

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
