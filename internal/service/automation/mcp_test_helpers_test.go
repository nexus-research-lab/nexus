package automation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	automationmcp "github.com/nexus-research-lab/nexus/internal/mcp/automation"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

type automationMCPFixture struct {
	WorkspacePath string
	Permission    *permissionctx.Context
	DM            *fakeDMRunner
	Router        *channels.Router
	Service       *Service
	ServerContext contract.ServerContext
}

func newAutomationMCPFixture(t *testing.T, resultText string) automationMCPFixture {
	t.Helper()
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: firstNonEmptyString(resultText, "ok"),
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)
	return automationMCPFixture{
		WorkspacePath: workspacePath,
		Permission:    permission,
		DM:            dm,
		Router:        router,
		Service:       service,
		ServerContext: contract.ServerContext{
			CurrentAgentID:      "agent-1",
			CurrentAgentName:    "新闻智能体",
			OwnerUserID:         "user-1",
			CurrentSessionKey:   protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "operator", ""),
			CurrentSessionLabel: "用户对话",
			SourceContextType:   "agent",
			SourceContextID:     "agent-1",
			SourceContextLabel:  "新闻智能体",
			DefaultTimezone:     "Asia/Shanghai",
		},
	}
}

func callAutomationMCPTool(
	t *testing.T,
	service *Service,
	sctx contract.ServerContext,
	name string,
	args map[string]any,
) (map[string]any, bool) {
	t.Helper()
	server := automationmcp.NewServer(service, sctx)
	resp, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result, got %+v", resp)
	}
	isError, _ := result["isError"].(bool)
	return result, isError
}

func decodeAutomationMCPJSON[T any](t *testing.T, result map[string]any) T {
	t.Helper()
	var payload T
	if err := json.Unmarshal([]byte(automationMCPToolText(t, result)), &payload); err != nil {
		t.Fatalf("解析 MCP 工具 JSON 失败: %v", err)
	}
	return payload
}

func automationMCPToolText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("MCP 工具返回 content 异常: %+v", result)
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("MCP 工具返回 text 异常: %+v", content[0])
	}
	return text
}

func automationMCPTestOwnerContext(ownerUserID string) context.Context {
	if strings.TrimSpace(ownerUserID) == "" {
		return context.Background()
	}
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "mcp_test",
	})
}
