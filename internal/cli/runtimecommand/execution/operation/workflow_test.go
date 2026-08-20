package operation

import (
	"context"
	"testing"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type workflowCommandService struct {
	owner   string
	request protocol.CreateWorkGraphWorkflowRequest
}

func (s *workflowCommandService) CreateFromExecution(
	_ context.Context,
	owner string,
	request protocol.CreateWorkGraphWorkflowRequest,
) (*protocol.WorkGraphWorkflow, error) {
	s.owner = owner
	s.request = request
	return &protocol.WorkGraphWorkflow{
		ID: "workflow-a", SlashName: request.SlashName,
		Nodes: []protocol.WorkGraphWorkflowNode{{LogicalKey: "research"}},
	}, nil
}

func TestDistillWorkflowUsesTrustedOwnerAndSession(t *testing.T) {
	service := &workflowCommandService{}
	definition := distillWorkGraphWorkflow(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "session-a",
		RootRoundID: "round-a", RuntimeRoundID: "round-a", AgentID: "agent-a",
	})
	result, err := definition.Invoke(context.Background(), map[string]any{
		"execution_id": "execution-a", "slash_name": "deep-research", "title": "Deep research",
		"nodes": []any{map[string]any{"work_item_id": "work-a", "role": "key"}},
	}, &runtimecommand.CallContext{RequestID: "workflow-request-1"})
	if err != nil || result.IsError {
		t.Fatalf("invoke result = %#v, err=%v", result, err)
	}
	if service.owner != "owner-a" || service.request.SourceSessionKey != "session-a" || service.request.CommandID != "workflow-request-1" {
		t.Fatalf("trusted request = owner %q, %#v", service.owner, service.request)
	}
	if result.StructuredContent["command"] != "/deep-research" || result.StructuredContent["outcome"] != "applied" {
		t.Fatalf("result = %#v", result.StructuredContent)
	}
}
