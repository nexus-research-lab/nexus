package operation

import (
	"context"
	"strings"
	"testing"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type workflowCommandService struct {
	owner   string
	request protocol.SaveWorkGraphWorkflowRequest
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
	})
	result, err := definition.Invoke(context.Background(), map[string]any{
		"preview_id": "workgraph_preview_a",
	}, &runtimecommand.CallContext{RequestID: "workflow-request-1"})
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
