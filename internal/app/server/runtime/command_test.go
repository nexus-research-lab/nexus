package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

type runtimeAutomationPermissionSender struct {
	events chan protocol.EventMessage
}

func TestServerBuilderBindsRoundHumanContextToBridgeCall(t *testing.T) {
	roundContext := authctx.WithPrincipal(t.Context(), &authctx.Principal{
		UserID: authctx.SystemUserID, Role: authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
	roundContext = authctx.WithInteractiveHumanEvidence(
		roundContext,
		"desktop_session_token",
	)
	var (
		calledPrincipal *authctx.Principal
		calledEvidence  authctx.InteractiveHumanEvidence
	)
	builder := NewServerBuilder(
		config.Config{}, nil, nil, nil, nil, nil,
		func(context.Context, nexusmcp.RoundContext) []sdktool.Tool {
			return []sdktool.Tool{{
				Name: "check_round_identity", Description: "test round identity",
				InputSchema: map[string]any{"type": "object"},
				Handler: func(ctx context.Context, _ map[string]any) (sdktool.ToolResult, error) {
					calledPrincipal = authctx.PrincipalFromContext(ctx)
					calledEvidence, _ = authctx.InteractiveHumanEvidenceFromContext(ctx)
					return sdktool.ToolResult{}, nil
				},
			}}
		},
	)
	servers, err := builder(roundContext, nexusmcp.RoundContext{})
	if err != nil {
		t.Fatal(err)
	}
	server := servers[nexusMCPServerName].(sdkmcp.SDKServerConfig).Instance
	_, err = server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name": "check_round_identity", "arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calledPrincipal == nil || calledPrincipal.UserID != authctx.SystemUserID ||
		calledPrincipal.AuthMethod != authctx.AuthMethodLocal {
		t.Fatalf("bridge call principal = %+v", calledPrincipal)
	}
	if calledEvidence.Source != "desktop_session_token" {
		t.Fatalf("bridge call human evidence = %+v", calledEvidence)
	}
}

func TestCommandMCPUsesStructuredInputWithoutStagingFile(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	agent := &protocol.Agent{AgentID: "worker", OwnerUserID: "owner"}
	sessionKey := protocol.BuildAgentSessionKey(
		agent.AgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"main",
		"",
	)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})
	ctx = runtimectx.WithRuntimeRoundLease(ctx, sessionKey, "round-1")
	builder := NewServerBuilder(
		config.Config{},
		stubRuntimeAgentResolver{record: agent},
		automationsvc.NewService(config.Config{}, nil, nil, nil, nil, nil, nil, nil),
		nil,
		nil,
		nil,
		func(context.Context, nexusmcp.RoundContext) []sdktool.Tool {
			return []sdktool.Tool{{
				Name: "show_widget", Description: "test built-in tool",
				InputSchema: map[string]any{"type": "object"},
				Handler: func(context.Context, map[string]any) (sdktool.ToolResult, error) {
					return sdktool.ToolResult{}, nil
				},
			}}
		},
	)
	servers, err := builder(ctx, nexusmcp.RoundContext{
		SessionKey:         sessionKey,
		RoundID:            "round-1",
		SourceContextType:  "agent",
		SourceContextID:    agent.AgentID,
		SourceContextLabel: "Worker",
		CommandReceipts:    nexusmcp.NewCommandReceiptState(),
		CommandContext: runtimectx.RuntimeCommandContext{
			Agent: agent, ScopeSessionKey: sessionKey, RuntimeSessionKey: sessionKey,
			RootRoundID: "round-1", SourceContextType: "agent",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	configValue, ok := servers["nexus"].(sdkmcp.SDKServerConfig)
	if !ok || configValue.Instance == nil {
		t.Fatalf("runtime MCP server = %#v", servers)
	}
	listResponse, err := configValue.Instance.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0", "id": 0, "method": "tools/list",
	})
	if err != nil {
		t.Fatal(err)
	}
	listResult, _ := listResponse["result"].(map[string]any)
	listedTools, _ := listResult["tools"].([]map[string]any)
	if len(listedTools) != 2 || listedTools[0]["name"] != "command" || listedTools[1]["name"] != "show_widget" {
		t.Fatalf("nexus tools = %#v", listedTools)
	}
	isolatedServers, err := builder(ctx, nexusmcp.RoundContext{
		SessionKey: sessionKey, RoundID: "round-1",
		SourceContextType: protocol.SessionPurposeWorkGraphEditor,
		SourceContextID:   agent.AgentID, CommandReceipts: nexusmcp.NewCommandReceiptState(),
		CommandContext: runtimectx.RuntimeCommandContext{
			Agent: agent, ScopeSessionKey: sessionKey, RuntimeSessionKey: sessionKey,
			RootRoundID: "round-1", SourceContextType: protocol.SessionPurposeWorkGraphEditor,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	isolatedConfig := isolatedServers[nexusMCPServerName].(sdkmcp.SDKServerConfig)
	isolatedList, err := isolatedConfig.Instance.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0", "id": "isolated", "method": "tools/list",
	})
	if err != nil {
		t.Fatal(err)
	}
	isolatedResult, _ := isolatedList["result"].(map[string]any)
	isolatedTools, _ := isolatedResult["tools"].([]map[string]any)
	if len(isolatedTools) != 1 || isolatedTools[0]["name"] != "command" {
		t.Fatalf("isolated WorkGraph tools = %#v", isolatedTools)
	}
	response, err := configValue.Instance.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": command.ToolName,
			"arguments": map[string]any{
				"domain": command.DomainAutomation,
				"action": command.ActionContract,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, _ := response["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	if structured["mutation_allowed"] != true {
		t.Fatalf("runtime command result = %#v", result)
	}
	stagingRoot := filepath.Join(stateRoot, "users", "owner", "runtime", "tmp", "runtime-command-inputs")
	if _, err = os.Stat(stagingRoot); !os.IsNotExist(err) {
		t.Fatalf("runtime command created staging path: %v", err)
	}
}

func (s *runtimeAutomationPermissionSender) Key() string    { return "runtime-automation-test" }
func (s *runtimeAutomationPermissionSender) IsClosed() bool { return false }
func (s *runtimeAutomationPermissionSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func TestRuntimeAutomationApplyRequiresNativeSessionPermission(t *testing.T) {
	permissions := permissionctx.NewContext()
	sender := &runtimeAutomationPermissionSender{events: make(chan protocol.EventMessage, 2)}
	const sessionKey = "agent:worker:websocket:dm:owner:"
	permissions.BindSession(sessionKey, sender)
	result := make(chan error, 1)
	go func() {
		_, err := requireAutomationConfirmation(
			context.Background(),
			permissions,
			command.Actor{LeaseSessionKey: sessionKey},
			automationPlanForPermissionTest(),
		)
		result <- err
	}()
	var event protocol.EventMessage
	select {
	case event = <-sender.events:
	case <-time.After(time.Second):
		t.Fatal("native Automation permission event was not emitted")
	}
	requestID, _ := event.Data["request_id"].(string)
	if requestID == "" || event.Data["tool_name"] != "nexus_automation_apply" {
		t.Fatalf("permission event = %+v", event)
	}
	if !permissions.HandlePermissionResponse(context.Background(), sessionKey, map[string]any{
		"request_id": requestID, "decision": "allow",
	}) {
		t.Fatal("native Automation permission response was not accepted")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("confirmation error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Automation confirmation did not resume")
	}
}

func TestRuntimeAutomationConfirmationShowsNormalizedChangesWithoutRouteSecrets(t *testing.T) {
	deliver := true
	plan := automationdomain.AutomationCommandPlan{
		Operation:       automationdomain.AutomationCommandOperationCreate,
		Target:          "agent-1",
		Summary:         "创建定时任务",
		Risk:            "write",
		CurrentRevision: "new:agent-1",
		PlanDigest:      "digest-1",
		Input: automationdomain.AutomationCommandInput{
			Name:                    "日报",
			Instruction:             "生成日报",
			ContextMode:             "isolated",
			DeliverResult:           &deliver,
			SelectedSessionKey:      "agent:secret:websocket:dm:owner:",
			SelectedReplySessionKey: "agent:secret:weixin:dm:account:target:",
		},
	}
	input := automationConfirmationInput(plan)
	changes, ok := input["changes"].(map[string]any)
	if !ok || changes["name"] != "日报" || changes["deliver_result"] != &deliver {
		t.Fatalf("confirmation changes = %#v", input["changes"])
	}
	for _, routeField := range []string{
		"selected_session_key", "named_session_key", "selected_reply_session_key", "reply_session_key",
	} {
		if _, leaked := changes[routeField]; leaked {
			t.Fatalf("confirmation leaked host route field %q: %#v", routeField, changes)
		}
	}
}

func TestTrustedAutomationRuntimeActorLimitsMainAuthorityToWebSocketDM(t *testing.T) {
	agent := &protocol.Agent{AgentID: "main", OwnerUserID: "owner", IsMain: true}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: "session",
	})
	actor := command.Actor{
		OwnerUserID: "owner", AgentID: "main",
		SessionKey: protocol.BuildAgentSessionKey("main", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "owner", ""), RoundID: "round-1",
		LeaseSessionKey: protocol.BuildAgentSessionKey("main", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "owner", ""), LeaseRoundID: "round-1",
		SourceContextType: "agent", SourceContextID: "main",
	}
	if !trustedCommandActor(ctx, agent, actor) || !trustedMainCommandActor(actor) {
		t.Fatal("trusted main WebSocket DM was rejected")
	}
	actor.SourceContextType = "agent_paired"
	actor.SessionKey = protocol.BuildAgentSessionKey("main", protocol.SessionChannelFeishuSegment, protocol.RoomTypeDM, "open-id", "")
	actor.LeaseSessionKey = actor.SessionKey
	if !trustedCommandActor(context.Background(), agent, actor) {
		t.Fatal("trusted paired external DM was rejected")
	}
	if trustedMainCommandActor(actor) {
		t.Fatal("paired external DM received owner-main cross-Agent authority")
	}
	actor.LeaseRoundID = "swapped"
	if trustedCommandActor(context.Background(), agent, actor) {
		t.Fatal("swapped round lease was trusted")
	}
}

func TestTrustedWorkGraphEditorActorRequiresExactWebSocketDM(t *testing.T) {
	agent := &protocol.Agent{AgentID: "worker", OwnerUserID: "owner"}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: "session",
	})
	sessionKey := protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "editor-id", "",
	)
	actor := command.Actor{
		OwnerUserID: "owner", AgentID: "worker",
		SessionKey: sessionKey, RoundID: "round-1",
		LeaseSessionKey: sessionKey, LeaseRoundID: "round-1",
		SourceContextType: protocol.SessionPurposeWorkGraphEditor,
		SourceContextID:   "worker",
	}
	if !trustedCommandActor(ctx, agent, actor) {
		t.Fatal("exact WorkGraph editor WebSocket DM was rejected")
	}
	actor.SourceContextID = "other-agent"
	if trustedCommandActor(ctx, agent, actor) {
		t.Fatal("WorkGraph editor accepted a mismatched Agent identity")
	}
	actor.SourceContextID = "worker"
	actor.SessionKey = protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelFeishuSegment, protocol.RoomTypeDM, "editor-id", "",
	)
	actor.LeaseSessionKey = actor.SessionKey
	if trustedCommandActor(ctx, agent, actor) {
		t.Fatal("WorkGraph editor accepted a non-WebSocket DM")
	}
}

func TestTrustedWorkGraphDistillationActorRequiresExactInternalBinding(t *testing.T) {
	agent := &protocol.Agent{AgentID: "worker", OwnerUserID: "owner"}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: "session",
	})
	sessionKey := protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelInternalSegment, protocol.RoomTypeDM, "preview-a", "",
	)
	actor := command.Actor{
		OwnerUserID: "owner", AgentID: "worker",
		SessionKey: sessionKey, RoundID: "round-1",
		LeaseSessionKey: sessionKey, LeaseRoundID: "round-1",
		SourceContextType: protocol.SessionPurposeWorkGraphDistillation,
		SourceContextID:   "worker",
		Round: nexusmcp.RoundContext{CommandContext: runtimectx.RuntimeCommandContext{
			ScopeSessionKey:    "room:group:conversation-a",
			WorkGraphPreviewID: "preview-a",
		}},
	}
	if !trustedCommandActor(ctx, agent, actor) {
		t.Fatal("exact isolated WorkGraph distillation actor was rejected")
	}
	actor.Round.CommandContext.WorkGraphPreviewID = ""
	if trustedCommandActor(ctx, agent, actor) {
		t.Fatal("WorkGraph distillation accepted a missing preview binding")
	}
	actor.Round.CommandContext.WorkGraphPreviewID = "preview-a"
	actor.SessionKey = protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "preview-a", "",
	)
	actor.LeaseSessionKey = actor.SessionKey
	if trustedCommandActor(ctx, agent, actor) {
		t.Fatal("WorkGraph distillation accepted a user WebSocket Session")
	}
}

func TestSemanticRuntimeCommandRecordsHostTypedExecutionReceipt(t *testing.T) {
	workBinding := &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	responsibility := runtimectx.NewResponsibilityAuthorityState(nil, "execution-1", workBinding, nil)
	receipts := nexusmcp.NewCommandReceiptState()
	sdkSessionIdentity := runtimectx.NewSDKSessionIdentityState("sdk-session-current")
	actor := command.Actor{Round: nexusmcp.RoundContext{
		CommandReceipts: receipts,
		CommandContext: runtimectx.RuntimeCommandContext{
			ResponsibilityAuthority: responsibility,
			SDKSessionIdentity:      sdkSessionIdentity,
		},
	}}
	var capturedSessionID string
	operations := []command.Operation{{
		Name: "submit_work",
		ContextHandler: func(_ context.Context, _ map[string]any, call *command.CallContext) (command.Result, error) {
			capturedSessionID = call.SessionID
			return command.Result{StructuredContent: map[string]any{
				"outcome": "applied", "execution_id": "execution-1", "message": "submitted",
				"changed": []any{"submission:submission-1", "attempt:attempt-1"},
			}}, nil
		},
	}}
	_, err := command.HandleSemantic(
		context.Background(), actor, command.DomainExecution, "get_execution", operations,
		command.Request{
			Domain: command.DomainExecution, Action: command.ActionInvoke,
			Operation: "submit_work", RequestID: "submit-request-1",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if capturedSessionID != "sdk-session-current" {
		t.Fatalf("semantic command SDK Session = %q", capturedSessionID)
	}
	items, sequence := receipts.Since(0)
	if sequence != 1 || len(items) != 1 {
		t.Fatalf("receipt state = sequence %d items %+v", sequence, items)
	}
	receipt := items[0]
	if receipt.RequestID != "submit-request-1" || receipt.Domain != command.DomainExecution ||
		receipt.Operation != "submit_work" || !receipt.Applied() ||
		receipt.ExecutionID != "execution-1" || receipt.WorkItemID != "work-1" ||
		receipt.AssignmentID != "assignment-1" || receipt.AttemptID != "attempt-1" ||
		len(receipt.Changed) != 2 || receipt.Changed[0] != "submission:submission-1" {
		t.Fatalf("typed execution receipt = %+v", receipt)
	}
}

func automationPlanForPermissionTest() automationdomain.AutomationCommandPlan {
	return automationdomain.AutomationCommandPlan{
		Operation: "delete", Target: "task-1", Summary: "删除任务", Risk: "destructive",
		RequiresConfirmation: true, CurrentRevision: "task:task-1:1", PlanDigest: strings.Repeat("a", 64),
	}
}
