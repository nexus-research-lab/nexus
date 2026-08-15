package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	automationmcp "github.com/nexus-research-lab/nexus/internal/mcp/automation"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var automationMCPRequestSequence atomic.Uint64

type automationMCPFixture struct {
	WorkspacePath string
	Permission    *permissionctx.Context
	DM            *fakeDMRunner
	Router        *channels.Router
	Service       *Service
	ServerContext contract.ServerContext
}

type allowAutomationDeliveryGrant struct{}

func (allowAutomationDeliveryGrant) ValidateAutomationDeliveryGrant(
	context.Context,
	string,
	string,
	string,
) error {
	return nil
}

func newAutomationMCPFixture(t *testing.T, resultText string) automationMCPFixture {
	t.Helper()
	workspacePath := newAutomationOwnerWorkspace(t, "user-1", "agent-1")
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
	service.SetDeliveryGrantResolver(allowAutomationDeliveryGrant{})
	currentSessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"operator",
		"",
	)
	prepareAutomationDeliverySession(t, workspacePath, "user-1", "agent-1", currentSessionKey)
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
			CurrentSessionKey:   currentSessionKey,
			CurrentSessionLabel: "用户对话",
			SourceContextType:   "agent",
			SourceContextID:     "agent-1",
			SourceContextLabel:  "新闻智能体",
			DefaultTimezone:     "Asia/Shanghai",
		},
	}
}

func prepareAutomationDeliverySession(
	t *testing.T,
	workspacePath string,
	ownerUserID string,
	agentID string,
	sessionKey string,
) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := workspacestore.NewSessionFileStore(workspacePath).
		ForOwner(ownerUserID).
		UpsertSession(workspacePath, protocol.Session{
			SessionKey:   sessionKey,
			AgentID:      agentID,
			ChannelType:  protocol.NormalizeStoredChannelType(protocol.ParseSessionKey(sessionKey).Channel),
			ChatType:     protocol.RoomTypeDM,
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Title:        "用户对话",
			Options:      map[string]any{},
			IsActive:     true,
		}); err != nil {
		t.Fatalf("准备真实接收会话失败: %v", err)
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
	name, args = automationMCPTestRoute(name, args)
	if args["operation"] == "create" {
		requestID, hasRequestID := args["request_id"]
		if !hasRequestID || strings.TrimSpace(fmt.Sprint(requestID)) == "" {
			args["request_id"] = fmt.Sprintf(
				"test-%s-%d",
				strings.ReplaceAll(t.Name(), "/", "-"),
				automationMCPRequestSequence.Add(1),
			)
		}
	}
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

func automationMCPTestRoute(name string, args map[string]any) (string, map[string]any) {
	routed := make(map[string]any, len(args)+1)
	for key, value := range args {
		routed[key] = value
	}
	operation := ""
	switch name {
	case "create_scheduled_task":
		operation = "create"
	case "find_scheduled_tasks":
		operation = "list"
	case "inspect_scheduled_task":
		operation, _ = routed["view"].(string)
		if operation == "" || operation == "status" {
			operation = "get"
		}
	case "update_scheduled_task":
		operation = "update"
	case "delete_scheduled_task":
		operation = "delete"
	case "get_scheduled_task_report":
		operation = "report"
	case "get_heartbeat":
		operation = "heartbeat"
	case "run_scheduled_task":
		operation = "run"
	case "repair_scheduled_task":
		operation = "retry_delivery"
		delete(routed, "action")
	case "update_heartbeat":
		operation = "set_heartbeat"
	case "wake_heartbeat":
		operation = "wake"
	case "automation_query", "automation_update":
		return name, routed
	}
	routed["operation"] = operation
	if operation == "list" || operation == "get" || operation == "runs" || operation == "events" || operation == "report" || operation == "heartbeat" {
		return "automation_query", routed
	}
	return "automation_update", routed
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
