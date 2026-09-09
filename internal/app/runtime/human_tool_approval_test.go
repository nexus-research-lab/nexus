// INPUT: configuration/Connector 两类高风险工具批准与 recorder stub。
// OUTPUT: 精确域路由、缺失依赖和未知工具的 fail-closed 断言。
// POS: 应用层人工批准组合器单元测试。
package runtime

import (
	"context"
	"errors"
	"testing"

	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

type humanToolApprovalRecorderStub struct {
	approvals []permissionctx.HumanToolApproval
	err       error
}

func (s *humanToolApprovalRecorderStub) RecordHumanToolApproval(
	_ context.Context,
	approval permissionctx.HumanToolApproval,
) error {
	s.approvals = append(s.approvals, approval)
	return s.err
}

func TestHumanToolApprovalRouterUsesExactToolDomain(t *testing.T) {
	configuration := &humanToolApprovalRecorderStub{}
	connector := &humanToolApprovalRecorderStub{}
	router := humanToolApprovalRouter{
		configuration: configuration,
		connector:     connector,
	}

	configApproval := permissionctx.HumanToolApproval{
		ToolName: "mcp__nexus_config__apply_nexus_configuration_change",
	}
	if err := router.RecordHumanToolApproval(
		t.Context(),
		configApproval,
	); err != nil {
		t.Fatal(err)
	}
	connectorApproval := permissionctx.HumanToolApproval{
		ToolName:  "mcp__nexus__connector_authorization",
		ToolInput: map[string]any{"action": "start"},
	}
	if err := router.RecordHumanToolApproval(
		t.Context(),
		connectorApproval,
	); err != nil {
		t.Fatal(err)
	}

	if len(configuration.approvals) != 1 ||
		configuration.approvals[0].ToolName != configApproval.ToolName {
		t.Fatalf("configuration recorder = %+v", configuration.approvals)
	}
	if len(connector.approvals) != 1 ||
		connector.approvals[0].ToolName != connectorApproval.ToolName {
		t.Fatalf("connector recorder = %+v", connector.approvals)
	}
}

func TestHumanToolApprovalRouterFailsClosed(t *testing.T) {
	expected := errors.New("connector recorder failed")
	router := humanToolApprovalRouter{
		configuration: &humanToolApprovalRecorderStub{},
		connector:     &humanToolApprovalRecorderStub{err: expected},
	}
	err := router.RecordHumanToolApproval(
		t.Context(),
		permissionctx.HumanToolApproval{
			ToolName:  "connector_authorization",
			ToolInput: map[string]any{"action": "start"},
		},
	)
	if !errors.Is(err, expected) {
		t.Fatalf("connector recorder error = %v, want %v", err, expected)
	}
	if err = router.RecordHumanToolApproval(
		t.Context(),
		permissionctx.HumanToolApproval{
			ToolName:  "connector_authorization",
			ToolInput: map[string]any{"action": "status"},
		},
	); err == nil {
		t.Fatal("Connector status 不能进入启动批准记录器")
	}
	if err = router.RecordHumanToolApproval(
		t.Context(),
		permissionctx.HumanToolApproval{ToolName: "unknown_tool"},
	); err == nil {
		t.Fatal("unknown approval tool must fail closed")
	}
}
