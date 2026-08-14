package agent_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	agenthandler "github.com/nexus-research-lab/nexus/internal/handler/agent"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	roomhandler "github.com/nexus-research-lab/nexus/internal/handler/room"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	subagentSecurityVictimID   = "subagent-security-victim"
	subagentSecurityAttackerID = "subagent-security-attacker"
	subagentSecurityTaskID     = "task-victim"
)

type subagentSecurityRuntimeClient struct {
	mu    sync.Mutex
	calls []string
}

func (c *subagentSecurityRuntimeClient) record(call string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *subagentSecurityRuntimeClient) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = nil
}

func (c *subagentSecurityRuntimeClient) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

func (c *subagentSecurityRuntimeClient) Connect(context.Context) error {
	c.record("connect")
	return nil
}

func (c *subagentSecurityRuntimeClient) Query(context.Context, string) error {
	c.record("query")
	return nil
}

func (c *subagentSecurityRuntimeClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	c.record("receive_messages")
	result := make(chan sdkprotocol.ReceivedMessage)
	close(result)
	return result
}

func (c *subagentSecurityRuntimeClient) Interrupt(context.Context) error {
	c.record("interrupt")
	return nil
}

func (c *subagentSecurityRuntimeClient) StopTask(context.Context, string) error {
	c.record("stop_task")
	return nil
}

func (c *subagentSecurityRuntimeClient) SendTaskMessage(context.Context, string, string, string) error {
	c.record("send_task_message")
	return nil
}

func (c *subagentSecurityRuntimeClient) RemoveMessages(context.Context, []string) error {
	c.record("remove_messages")
	return nil
}

func (c *subagentSecurityRuntimeClient) SetPermissionMode(context.Context, sdkpermission.Mode) error {
	c.record("set_permission_mode")
	return nil
}

func (c *subagentSecurityRuntimeClient) Retire() {
	c.record("retire")
}

func (c *subagentSecurityRuntimeClient) Disconnect(context.Context) error {
	c.record("disconnect")
	return nil
}

func (c *subagentSecurityRuntimeClient) Reconfigure(context.Context, agentclient.Options) error {
	c.record("reconfigure")
	return nil
}

func (c *subagentSecurityRuntimeClient) SessionID() string {
	c.record("session_id")
	return ""
}

type subagentSecurityRuntimeFactory struct {
	client runtimectx.Client
}

func (f subagentSecurityRuntimeFactory) New(agentclient.Options) runtimectx.Client {
	return f.client
}

func TestSubagentTaskHTTPControlsHideCrossOwnerResources(t *testing.T) {
	fixture := newSubagentSecurityFixture(t)

	assertCasesReturnNotFound(t, fixture.attackerContext, []subagentSecurityHTTPCase{
		{name: "session list", handler: fixture.agent.HandleSessionSubagentTasks, params: map[string]string{"session_key": fixture.sessionKey}},
		{name: "session transcript", handler: fixture.agent.HandleSessionSubagentTaskMessages, params: map[string]string{"session_key": fixture.sessionKey, "task_id": subagentSecurityTaskID}},
		{name: "session stop", method: http.MethodPost, handler: fixture.agent.HandleStopSessionSubagentTask, params: map[string]string{"session_key": fixture.sessionKey, "task_id": subagentSecurityTaskID}},
		{name: "session send", method: http.MethodPost, body: []byte(`{"message":"continue"}`), handler: fixture.agent.HandleSendSessionSubagentTaskMessage, params: map[string]string{"session_key": fixture.sessionKey, "task_id": subagentSecurityTaskID}},
		{name: "room list", handler: fixture.room.HandleConversationSubagentTasks, params: fixture.roomParams(subagentSecurityTaskID, false)},
		{name: "room transcript", handler: fixture.room.HandleConversationSubagentTaskMessages, params: fixture.roomParams(subagentSecurityTaskID, true)},
		{name: "room stop", method: http.MethodPost, handler: fixture.room.HandleStopConversationSubagentTask, params: fixture.roomParams(subagentSecurityTaskID, true)},
		{name: "room send", method: http.MethodPost, body: []byte(`{"message":"continue"}`), handler: fixture.room.HandleSendConversationSubagentTaskMessage, params: fixture.roomParams(subagentSecurityTaskID, true)},
	})

	if calls := fixture.runtimeClient.snapshot(); len(calls) != 0 {
		t.Fatalf("cross-owner HTTP requests reached runtime client: %v", calls)
	}
}

func TestSubagentTaskHTTPControlsRejectMismatchedRoomAndUnknownTasks(t *testing.T) {
	fixture := newSubagentSecurityFixture(t)

	assertCasesReturnNotFound(t, fixture.victimContext, []subagentSecurityHTTPCase{
		{
			name:    "room and conversation mismatch",
			handler: fixture.room.HandleConversationSubagentTasks,
			params: map[string]string{
				"room_id":         fixture.otherRoomID,
				"conversation_id": fixture.conversationID,
			},
		},
		{name: "unknown session transcript", handler: fixture.agent.HandleSessionSubagentTaskMessages, params: map[string]string{"session_key": fixture.sessionKey, "task_id": "task-missing"}},
		{name: "unknown session stop", method: http.MethodPost, handler: fixture.agent.HandleStopSessionSubagentTask, params: map[string]string{"session_key": fixture.sessionKey, "task_id": "task-missing"}},
		{name: "unknown session send", method: http.MethodPost, body: []byte(`{"message":"continue"}`), handler: fixture.agent.HandleSendSessionSubagentTaskMessage, params: map[string]string{"session_key": fixture.sessionKey, "task_id": "task-missing"}},
		{name: "unknown room transcript", handler: fixture.room.HandleConversationSubagentTaskMessages, params: fixture.roomParams("task-missing", true)},
		{name: "unknown room stop", method: http.MethodPost, handler: fixture.room.HandleStopConversationSubagentTask, params: fixture.roomParams("task-missing", true)},
		{name: "unknown room send", method: http.MethodPost, body: []byte(`{"message":"continue"}`), handler: fixture.room.HandleSendConversationSubagentTaskMessage, params: fixture.roomParams("task-missing", true)},
	})

	if calls := fixture.runtimeClient.snapshot(); len(calls) != 0 {
		t.Fatalf("invalid HTTP requests reached runtime client: %v", calls)
	}
}

type subagentSecurityFixture struct {
	agent           *agenthandler.Handlers
	room            *roomhandler.Handlers
	runtimeClient   *subagentSecurityRuntimeClient
	victimContext   context.Context
	attackerContext context.Context
	sessionKey      string
	roomID          string
	otherRoomID     string
	conversationID  string
}

func newSubagentSecurityFixture(t *testing.T) subagentSecurityFixture {
	t.Helper()
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	t.Cleanup(func() { _ = db.Close() })
	core := serverapp.NewCoreServicesWithDB(cfg, db)
	runtimeManager := runtimectx.NewManager()
	core.Session.SetRuntimeManager(runtimeManager)

	victimContext := subagentSecurityOwnerContext(subagentSecurityVictimID)
	attackerContext := subagentSecurityOwnerContext(subagentSecurityAttackerID)
	victimAgent, err := core.Agent.CreateAgent(victimContext, protocol.CreateRequest{Name: "Victim worker"})
	if err != nil {
		t.Fatalf("create victim agent: %v", err)
	}
	victimPeer, err := core.Agent.CreateAgent(victimContext, protocol.CreateRequest{Name: "Victim peer"})
	if err != nil {
		t.Fatalf("create victim peer: %v", err)
	}

	sessionKey := protocol.BuildAgentSessionKey(victimAgent.AgentID, "ws", "dm", "victim-session", "")
	if _, err = core.Session.CreateSession(victimContext, sessionsvc.CreateRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("create victim session: %v", err)
	}
	agentHistory := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath).ForOwner(subagentSecurityVictimID)
	if err = agentHistory.AppendOverlayMessage(victimAgent.WorkspacePath, sessionKey, subagentSecurityTaskMessage(victimAgent.AgentID)); err != nil {
		t.Fatalf("seed victim session task: %v", err)
	}

	roomContext, err := core.Room.CreateRoom(victimContext, protocol.CreateRoomRequest{
		AgentIDs: []string{victimAgent.AgentID, victimPeer.AgentID},
		Name:     "Victim room",
	})
	if err != nil {
		t.Fatalf("create victim room: %v", err)
	}
	otherRoomContext, err := core.Room.CreateRoom(victimContext, protocol.CreateRoomRequest{
		AgentIDs: []string{victimAgent.AgentID, victimPeer.AgentID},
		Name:     "Other victim room",
	})
	if err != nil {
		t.Fatalf("create second victim room: %v", err)
	}
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	if err = roomHistory.AppendInlineMessage(
		subagentSecurityVictimID,
		roomContext.Conversation.ID,
		subagentSecurityTaskMessage(victimAgent.AgentID),
	); err != nil {
		t.Fatalf("seed victim room task: %v", err)
	}

	runtimeClient := &subagentSecurityRuntimeClient{}
	factory := subagentSecurityRuntimeFactory{client: runtimeClient}
	for _, runtimeSessionKey := range []string{
		sessionKey,
		protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, victimAgent.AgentID, protocol.RoomTypeGroup),
	} {
		if _, err = runtimeManager.GetOrCreateWithFactory(
			victimContext,
			runtimeSessionKey,
			agentclient.Options{
				Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
				Env:     map[string]string{"NEXUS_RUNTIME_USER_ID": subagentSecurityVictimID},
			},
			factory,
		); err != nil {
			t.Fatalf("register victim runtime %q: %v", runtimeSessionKey, err)
		}
	}
	runtimeClient.reset()

	api := handlershared.NewAPI(nil)
	return subagentSecurityFixture{
		agent: agenthandler.New(
			api,
			core.Agent,
			core.Session,
			runtimeManager,
			nil,
			nil,
			nil,
		),
		room: roomhandler.New(
			api,
			core.Room,
			nil,
			core.Session,
			nil,
			nil,
			nil,
		),
		runtimeClient:   runtimeClient,
		victimContext:   victimContext,
		attackerContext: attackerContext,
		sessionKey:      sessionKey,
		roomID:          roomContext.Room.ID,
		otherRoomID:     otherRoomContext.Room.ID,
		conversationID:  roomContext.Conversation.ID,
	}
}

func subagentSecurityOwnerContext(ownerUserID string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
	})
}

func subagentSecurityTaskMessage(hostAgentID string) protocol.Message {
	return protocol.Message{
		"message_id": "subagent-security-task",
		"agent_id":   hostAgentID,
		"round_id":   "round-victim",
		"role":       "system",
		"timestamp":  int64(1000),
		"metadata": map[string]any{
			"subtype":      "task_started",
			"task_id":      subagentSecurityTaskID,
			"agent_id":     "sdk-subagent-victim",
			"agent_type":   "worker",
			"runtime_kind": "nxs",
			"status":       "running",
		},
	}
}

func (f subagentSecurityFixture) roomParams(taskID string, includeTaskID bool) map[string]string {
	result := map[string]string{
		"room_id":         f.roomID,
		"conversation_id": f.conversationID,
	}
	if includeTaskID {
		result["task_id"] = taskID
	}
	return result
}

type subagentSecurityHTTPCase struct {
	name    string
	method  string
	body    []byte
	handler http.HandlerFunc
	params  map[string]string
}

func assertCasesReturnNotFound(t *testing.T, ctx context.Context, cases []subagentSecurityHTTPCase) {
	t.Helper()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			method := test.method
			if method == "" {
				method = http.MethodGet
			}
			request := httptest.NewRequest(method, "/nexus/v1/subagent-security", bytes.NewReader(test.body))
			routeContext := chi.NewRouteContext()
			for name, value := range test.params {
				routeContext.URLParams.Add(name, value)
			}
			request = request.WithContext(context.WithValue(ctx, chi.RouteCtxKey, routeContext))
			recorder := httptest.NewRecorder()

			test.handler(recorder, request)

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
