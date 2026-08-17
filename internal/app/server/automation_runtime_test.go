package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

type runtimeAutomationRoundResolver struct {
	sessionKey string
	roundID    string
}

func (r runtimeAutomationRoundResolver) GetRunningRoundIDs(sessionKey string) []string {
	if sessionKey == r.sessionKey {
		return []string{r.roundID}
	}
	return nil
}

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
			automationsvc.RuntimeCommandActor{LeaseSessionKey: sessionKey},
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

func TestRuntimeAutomationHandlerResolvesCapabilityWithoutMCP(t *testing.T) {
	service := automationsvc.NewService(config.Config{}, nil, nil, nil, nil, nil, nil, nil)
	actor := automationsvc.RuntimeCommandActor{
		OwnerUserID: "owner", AgentID: "worker",
		SessionKey: "agent:worker:websocket:dm:owner:", RoundID: "round-1",
		LeaseSessionKey: "runtime-session", LeaseRoundID: "round-1",
		SourceContextType: "agent", SourceContextID: "worker",
	}
	service.SetRuntimeCommandRoundResolver(runtimeAutomationRoundResolver{
		sessionKey: actor.LeaseSessionKey, roundID: actor.LeaseRoundID,
	})
	token, err := service.IssueRuntimeCommandCapability(actor)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(automationdomain.AutomationCommandRequest{
		Action: automationdomain.AutomationCommandActionContract,
	})
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/internal/runtime/automation", bytes.NewReader(payload))
	request.RemoteAddr = "127.0.0.1:43210"
	request.Header.Set(protocol.NexusCommandCapabilityHeader, token)
	recorder := httptest.NewRecorder()
	newRuntimeAutomationHandler(service, nil).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"mutation_allowed":true`) {
		t.Fatalf("contract response status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "automation_query") || strings.Contains(recorder.Body.String(), "automation_update") {
		t.Fatalf("runtime contract leaked retired MCP names: %s", recorder.Body.String())
	}
}

func TestTrustedAutomationRuntimeActorLimitsMainAuthorityToWebSocketDM(t *testing.T) {
	agent := &protocol.Agent{AgentID: "main", OwnerUserID: "owner", IsMain: true}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: "session",
	})
	actor := automationsvc.RuntimeCommandActor{
		OwnerUserID: "owner", AgentID: "main",
		SessionKey: protocol.BuildAgentSessionKey("main", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "owner", ""), RoundID: "round-1",
		LeaseSessionKey: protocol.BuildAgentSessionKey("main", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "owner", ""), LeaseRoundID: "round-1",
		SourceContextType: "agent", SourceContextID: "main",
	}
	if !trustedAutomationRuntimeActor(ctx, agent, actor) || !trustedMainAutomationRuntime(actor) {
		t.Fatal("trusted main WebSocket DM was rejected")
	}
	actor.SourceContextType = "agent_paired"
	actor.SessionKey = protocol.BuildAgentSessionKey("main", protocol.SessionChannelFeishuSegment, protocol.RoomTypeDM, "open-id", "")
	actor.LeaseSessionKey = actor.SessionKey
	if !trustedAutomationRuntimeActor(context.Background(), agent, actor) {
		t.Fatal("trusted paired external DM was rejected")
	}
	if trustedMainAutomationRuntime(actor) {
		t.Fatal("paired external DM received owner-main cross-Agent authority")
	}
	actor.LeaseRoundID = "swapped"
	if trustedAutomationRuntimeActor(context.Background(), agent, actor) {
		t.Fatal("swapped round lease was trusted")
	}
}

func automationPlanForPermissionTest() automationdomain.AutomationCommandPlan {
	return automationdomain.AutomationCommandPlan{
		Operation: "delete", Target: "task-1", Summary: "删除任务", Risk: "destructive",
		RequiresConfirmation: true, CurrentRevision: "task:task-1:1", PlanDigest: strings.Repeat("a", 64),
	}
}
