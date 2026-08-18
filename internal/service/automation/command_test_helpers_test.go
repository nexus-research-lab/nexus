package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var automationCommandRequestSequence atomic.Uint64

type automationCommandFixture struct {
	WorkspacePath string
	Permission    *permissionctx.Context
	DM            *fakeDMRunner
	Router        *channels.Router
	Service       *Service
	ServerContext runtimecommand.Actor
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

func newAutomationCommandFixture(t *testing.T, resultText string) automationCommandFixture {
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
	return automationCommandFixture{
		WorkspacePath: workspacePath,
		Permission:    permission,
		DM:            dm,
		Router:        router,
		Service:       service,
		ServerContext: runtimecommand.Actor{
			AgentID:            "agent-1",
			AgentName:          "新闻智能体",
			OwnerUserID:        "user-1",
			SessionKey:         currentSessionKey,
			SessionLabel:       "用户对话",
			SourceContextType:  "agent",
			SourceContextID:    "agent-1",
			SourceContextLabel: "新闻智能体",
			DefaultTimezone:    "Asia/Shanghai",
			RoundID:            "round-test",
			LeaseSessionKey:    currentSessionKey,
			LeaseRoundID:       "round-test",
			Round: runtimecommand.RoundContext{
				Receipts: runtimecommand.NewReceiptState(),
			},
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

func callAutomationCommand(
	t *testing.T,
	service *Service,
	sctx runtimecommand.Actor,
	name string,
	args map[string]any,
) (map[string]any, bool) {
	t.Helper()
	if strings.TrimSpace(sctx.LeaseSessionKey) == "" {
		sctx.LeaseSessionKey = firstNonEmptyString(sctx.SessionKey, "agent:test:dm:websocket:test:")
	}
	if strings.TrimSpace(sctx.LeaseRoundID) == "" {
		sctx.LeaseRoundID = firstNonEmptyString(sctx.RoundID, "round-test")
	}
	if strings.TrimSpace(sctx.RoundID) == "" {
		sctx.RoundID = sctx.LeaseRoundID
	}
	if strings.TrimSpace(sctx.SessionKey) == "" {
		sctx.SessionKey = sctx.LeaseSessionKey
	}
	if sctx.Round.Receipts == nil {
		sctx.Round.Receipts = runtimecommand.NewReceiptState()
	}
	name, args = automationCommandTestRoute(name, args)
	if !sctx.CrossAgentAllowed() {
		if rawMode, ok := args["execution_mode"].(string); ok && strings.TrimSpace(rawMode) != "" {
			mode := strings.TrimSpace(rawMode)
			contextMode := "isolated"
			if mode == "existing" {
				contextMode = "current"
			}
			args["context_mode"] = contextMode
		}
		if rawMode, ok := args["reply_mode"].(string); ok && strings.TrimSpace(rawMode) != "" {
			mode := strings.TrimSpace(rawMode)
			args["deliver_result"] = mode != "none"
		}
		for _, field := range []string{
			"execution_mode", "reply_mode", "selected_session_key", "named_session_key",
			"selected_reply_session_key", "reply_session_key", "reply_channel", "reply_to",
			"reply_account_id", "reply_thread_id",
		} {
			delete(args, field)
		}
	}
	operation := strings.TrimSpace(fmt.Sprint(args["operation"]))
	requestID, _ := args["request_id"].(string)
	requestID = strings.TrimSpace(requestID)
	if operation == "create" {
		_, hasRequestID := args["request_id"]
		if !hasRequestID || requestID == "" {
			requestID = fmt.Sprintf(
				"test-%s-%d",
				strings.ReplaceAll(t.Name(), "/", "-"),
				automationCommandRequestSequence.Add(1),
			)
			args["request_id"] = requestID
		}
	}
	delete(args, "operation")
	delete(args, "request_id")
	delete(args, "view")
	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal command input: %v", err)
	}
	var input automationdomain.AutomationCommandInput
	if err = json.Unmarshal(encoded, &input); err != nil {
		t.Fatalf("decode command input: %v", err)
	}
	ctx := context.Background()
	var payload any
	if name == "automation_query" {
		payload, err = service.InspectRuntimeCommand(ctx, sctx, operation, input)
	} else {
		plan, planErr := service.PlanRuntimeCommand(ctx, sctx, operation, input)
		if planErr != nil {
			err = planErr
		} else {
			if requestID == "" {
				requestID = fmt.Sprintf("test-%s-%d", strings.ReplaceAll(t.Name(), "/", "-"), automationCommandRequestSequence.Add(1))
			}
			var applied *automationdomain.AutomationCommandApplyResult
			applied, err = service.ApplyRuntimeCommand(ctx, sctx, automationdomain.AutomationCommandRequest{
				Action: automationdomain.AutomationCommandActionApply, Operation: operation,
				Input: input, RequestID: requestID,
				ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
			}, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"})
			if applied != nil {
				payload = applied.Data
			}
		}
	}
	if err != nil {
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}, true
	}
	text, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal command result: %v", err)
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
	}, false
}

func automationCommandTestRoute(name string, args map[string]any) (string, map[string]any) {
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
		operation = strings.TrimSpace(fmt.Sprint(routed["action"]))
		if operation == "recover" {
			operation = "update"
			routed["enabled"] = false
			routed["cancel_active_run"] = true
		} else {
			operation = "retry_delivery"
		}
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

func decodeAutomationCommandJSON[T any](t *testing.T, result map[string]any) T {
	t.Helper()
	var payload T
	if err := json.Unmarshal([]byte(automationCommandText(t, result)), &payload); err != nil {
		t.Fatalf("解析 Automation command JSON 失败: %v", err)
	}
	return payload
}

func automationCommandText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("Automation command 返回 content 异常: %+v", result)
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("Automation command 返回 text 异常: %+v", content[0])
	}
	return text
}

func automationCommandTestOwnerContext(ownerUserID string) context.Context {
	if strings.TrimSpace(ownerUserID) == "" {
		return context.Background()
	}
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "command_test",
	})
}
