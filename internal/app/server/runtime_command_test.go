package server

import (
	"context"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

type runtimeAutomationPermissionSender struct {
	events chan protocol.EventMessage
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
		_, err := requireRuntimeAutomationConfirmation(
			context.Background(),
			permissions,
			runtimecommand.Actor{LeaseSessionKey: sessionKey},
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
	input := runtimeAutomationConfirmationInput(plan)
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
	actor := runtimecommand.Actor{
		OwnerUserID: "owner", AgentID: "main",
		SessionKey: protocol.BuildAgentSessionKey("main", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "owner", ""), RoundID: "round-1",
		LeaseSessionKey: protocol.BuildAgentSessionKey("main", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "owner", ""), LeaseRoundID: "round-1",
		SourceContextType: "agent", SourceContextID: "main",
	}
	if !trustedRuntimeCommandActor(ctx, agent, actor) || !trustedMainRuntimeCommandActor(actor) {
		t.Fatal("trusted main WebSocket DM was rejected")
	}
	actor.SourceContextType = "agent_paired"
	actor.SessionKey = protocol.BuildAgentSessionKey("main", protocol.SessionChannelFeishuSegment, protocol.RoomTypeDM, "open-id", "")
	actor.LeaseSessionKey = actor.SessionKey
	if !trustedRuntimeCommandActor(context.Background(), agent, actor) {
		t.Fatal("trusted paired external DM was rejected")
	}
	if trustedMainRuntimeCommandActor(actor) {
		t.Fatal("paired external DM received owner-main cross-Agent authority")
	}
	actor.LeaseRoundID = "swapped"
	if trustedRuntimeCommandActor(context.Background(), agent, actor) {
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
	actor := runtimecommand.Actor{
		OwnerUserID: "owner", AgentID: "worker",
		SessionKey: sessionKey, RoundID: "round-1",
		LeaseSessionKey: sessionKey, LeaseRoundID: "round-1",
		SourceContextType: protocol.SessionPurposeWorkGraphEditor,
		SourceContextID:   "worker",
	}
	if !trustedRuntimeCommandActor(ctx, agent, actor) {
		t.Fatal("exact WorkGraph editor WebSocket DM was rejected")
	}
	actor.SourceContextID = "other-agent"
	if trustedRuntimeCommandActor(ctx, agent, actor) {
		t.Fatal("WorkGraph editor accepted a mismatched Agent identity")
	}
	actor.SourceContextID = "worker"
	actor.SessionKey = protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelFeishuSegment, protocol.RoomTypeDM, "editor-id", "",
	)
	actor.LeaseSessionKey = actor.SessionKey
	if trustedRuntimeCommandActor(ctx, agent, actor) {
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
	actor := runtimecommand.Actor{
		OwnerUserID: "owner", AgentID: "worker",
		SessionKey: sessionKey, RoundID: "round-1",
		LeaseSessionKey: sessionKey, LeaseRoundID: "round-1",
		SourceContextType: protocol.SessionPurposeWorkGraphDistillation,
		SourceContextID:   "worker",
		Round: runtimecommand.RoundContext{CommandContext: runtimectx.RuntimeCommandContext{
			ScopeSessionKey:    "room:group:conversation-a",
			WorkGraphPreviewID: "preview-a",
		}},
	}
	if !trustedRuntimeCommandActor(ctx, agent, actor) {
		t.Fatal("exact isolated WorkGraph distillation actor was rejected")
	}
	actor.Round.CommandContext.WorkGraphPreviewID = ""
	if trustedRuntimeCommandActor(ctx, agent, actor) {
		t.Fatal("WorkGraph distillation accepted a missing preview binding")
	}
	actor.Round.CommandContext.WorkGraphPreviewID = "preview-a"
	actor.SessionKey = protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "preview-a", "",
	)
	actor.LeaseSessionKey = actor.SessionKey
	if trustedRuntimeCommandActor(ctx, agent, actor) {
		t.Fatal("WorkGraph distillation accepted a user WebSocket Session")
	}
}

func TestSemanticRuntimeCommandRecordsHostTypedExecutionReceipt(t *testing.T) {
	workBinding := &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	responsibility := runtimectx.NewResponsibilityAuthorityState(nil, "execution-1", workBinding, nil)
	receipts := runtimecommand.NewReceiptState()
	sdkSessionIdentity := runtimectx.NewSDKSessionIdentityState("sdk-session-current")
	actor := runtimecommand.Actor{Round: runtimecommand.RoundContext{
		Receipts: receipts,
		CommandContext: runtimectx.RuntimeCommandContext{
			ResponsibilityAuthority: responsibility,
			SDKSessionIdentity:      sdkSessionIdentity,
		},
	}}
	var capturedSessionID string
	operations := []runtimecommand.Operation{{
		Name: "submit_work",
		ContextHandler: func(_ context.Context, _ map[string]any, call *runtimecommand.CallContext) (runtimecommand.Result, error) {
			capturedSessionID = call.SessionID
			return runtimecommand.Result{StructuredContent: map[string]any{
				"outcome": "applied", "execution_id": "execution-1", "message": "submitted",
				"changed": []any{"submission:submission-1", "attempt:attempt-1"},
			}}, nil
		},
	}}
	_, err := handleSemanticRuntimeCommand(
		context.Background(), actor, runtimecommand.DomainExecution, "get_execution", operations,
		runtimecommand.Request{
			Domain: runtimecommand.DomainExecution, Action: runtimecommand.ActionInvoke,
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
	if receipt.RequestID != "submit-request-1" || receipt.Domain != runtimecommand.DomainExecution ||
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
