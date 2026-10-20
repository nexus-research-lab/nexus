// INPUT: A real WebSocket set_goal command, a blocking objective rewriter, and
// an originating connection that closes while normalization is in flight.
// OUTPUT: Proof that Goal/control-record persistence and continuation dispatch
// survive transport cancellation.
// POS: Application composition regression for the detached Goal command boundary.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type blockingGoalObjectiveRewriter struct {
	started chan context.Context
	release chan struct{}
}

func (r *blockingGoalObjectiveRewriter) RewriteGoalObjective(
	ctx context.Context,
	_ string,
	_ string,
	objective string,
) (string, error) {
	r.started <- ctx
	<-r.release
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return strings.TrimSpace(objective) + " (normalized)", nil
}

type detachedGoalContinuationRecorder struct {
	plans chan protocol.GoalContinuation
}

func (*detachedGoalContinuationRecorder) ShouldDeferGoalContinuation(context.Context, string) bool {
	return false
}

func (r *detachedGoalContinuationRecorder) DispatchGoalContinuation(
	_ context.Context,
	plan protocol.GoalContinuation,
) error {
	r.plans <- plan
	return nil
}

func TestWebSocketSetGoalSurvivesDisconnectDuringObjectiveRewrite(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	cfg.GoalAutoContinueEnabled = true
	cfg.GoalMaxContinuationsPerRun = 5
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handlertest.CloseServer(t, server)
	rewriter := &blockingGoalObjectiveRewriter{
		started: make(chan context.Context, 1),
		release: make(chan struct{}),
	}
	continuations := &detachedGoalContinuationRecorder{
		plans: make(chan protocol.GoalContinuation, 1),
	}
	server.services.Goal.SetObjectiveRewriter(rewriter)
	server.services.Goal.SetContinuationDispatcher(continuations)

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()
	conversationID := ensureDetachedGoalDMConversation(t, server)
	sessionKey := protocol.BuildRoomAgentSessionKey(
		conversationID,
		cfg.DefaultAgentID,
		protocol.RoomTypeDM,
	)

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDial()
	connection, _, err := websocket.Dial(
		dialCtx,
		"ws"+strings.TrimPrefix(httpServer.URL, "http")+"/nexus/v1/chat/ws",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = wsjson.Write(dialCtx, connection, map[string]any{
		"type":              "set_goal",
		"session_key":       sessionKey,
		"agent_id":          cfg.DefaultAgentID,
		"objective":         "Keep Goal submission alive across navigation",
		"client_request_id": "request-detached-goal",
		"client_message_id": "message-detached-goal",
		"goal_options": map[string]any{
			"replace_existing": true,
			"token_budget":     nil,
		},
	}); err != nil {
		t.Fatal(err)
	}

	var rewriteCtx context.Context
	select {
	case rewriteCtx = <-rewriter.started:
	case <-time.After(3 * time.Second):
		t.Fatal("set_goal did not reach objective rewrite")
	}
	if err = connection.CloseNow(); err != nil {
		t.Fatal(err)
	}
	// Give the server read loop enough time to observe the closed transport.
	time.Sleep(50 * time.Millisecond)
	if err = rewriteCtx.Err(); err != nil {
		t.Fatalf("objective rewrite inherited WebSocket cancellation: %v", err)
	}
	close(rewriter.release)

	var plan protocol.GoalContinuation
	select {
	case plan = <-continuations.plans:
	case <-time.After(5 * time.Second):
		t.Fatal("detached set_goal did not dispatch continuation after sender closed")
	}
	if plan.Goal.SessionKey != sessionKey || plan.Goal.Objective !=
		"Keep Goal submission alive across navigation (normalized)" {
		t.Fatalf("continuation Goal = %#v", plan.Goal)
	}

	current, err := server.services.Goal.CurrentOptional(context.Background(), sessionKey)
	if err != nil || current == nil {
		t.Fatalf("current Goal = %#v, error = %v", current, err)
	}
	if current.ID != plan.Goal.ID {
		t.Fatalf("persisted Goal = %q, continuation Goal = %q", current.ID, plan.Goal.ID)
	}
	assertDetachedGoalControlRecord(t, cfg, sessionKey)
}

func TestWebSocketRoomSetGoalSurvivesDisconnectDuringObjectiveRewrite(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	cfg.GoalAutoContinueEnabled = true
	cfg.GoalMaxContinuationsPerRun = 5
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handlertest.CloseServer(t, server)
	rewriter := &blockingGoalObjectiveRewriter{
		started: make(chan context.Context, 1),
		release: make(chan struct{}),
	}
	continuations := &detachedGoalContinuationRecorder{
		plans: make(chan protocol.GoalContinuation, 1),
	}
	server.services.Goal.SetObjectiveRewriter(rewriter)
	server.services.Goal.SetContinuationDispatcher(continuations)

	lead, err := server.services.Core.Agent.CreateAgent(
		context.Background(),
		protocol.CreateRequest{Name: "Detached Goal Lead"},
	)
	if err != nil {
		t.Fatal(err)
	}
	roomContext, err := server.services.Core.Room.CreateRoom(
		context.Background(),
		protocol.CreateRoomRequest{
			AgentIDs: []string{lead.AgentID},
			Name:     "Detached Goal Room",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	conversationID := roomContext.Conversation.ID
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDial()
	connection, _, err := websocket.Dial(
		dialCtx,
		"ws"+strings.TrimPrefix(httpServer.URL, "http")+"/nexus/v1/chat/ws",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = wsjson.Write(dialCtx, connection, map[string]any{
		"type":              "set_goal",
		"session_key":       sessionKey,
		"agent_id":          lead.AgentID,
		"target_agent_ids":  []string{lead.AgentID},
		"objective":         "Keep Room Goal submission alive across navigation",
		"client_request_id": "request-detached-room-goal",
		"client_message_id": "message-detached-room-goal",
		"goal_options": map[string]any{
			"replace_existing": true,
			"token_budget":     nil,
		},
	}); err != nil {
		t.Fatal(err)
	}

	var rewriteCtx context.Context
	select {
	case rewriteCtx = <-rewriter.started:
	case <-time.After(3 * time.Second):
		t.Fatal("Room set_goal did not reach objective rewrite")
	}
	if err = connection.CloseNow(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err = rewriteCtx.Err(); err != nil {
		t.Fatalf("Room objective rewrite inherited WebSocket cancellation: %v", err)
	}
	close(rewriter.release)

	var plan protocol.GoalContinuation
	select {
	case plan = <-continuations.plans:
	case <-time.After(5 * time.Second):
		t.Fatal("detached Room set_goal did not dispatch continuation after sender closed")
	}
	if plan.Goal.SessionKey != sessionKey || plan.Goal.Objective !=
		"Keep Room Goal submission alive across navigation (normalized)" {
		t.Fatalf("Room continuation Goal = %#v", plan.Goal)
	}
	current, err := server.services.Goal.CurrentOptional(context.Background(), sessionKey)
	if err != nil || current == nil {
		t.Fatalf("current Room Goal = %#v, error = %v", current, err)
	}
	if current.ID != plan.Goal.ID {
		t.Fatalf("persisted Room Goal = %q, continuation Goal = %q", current.ID, plan.Goal.ID)
	}
	assertDetachedRoomGoalControlRecord(t, cfg, conversationID)
}

func ensureDetachedGoalDMConversation(t *testing.T, server *Server) string {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/rooms/dm/"+server.config.DefaultAgentID,
		nil,
	)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create DM conversation: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data protocol.ConversationContextAggregate `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Conversation.ID == "" {
		t.Fatalf("create DM conversation returned no ID: %s", recorder.Body.String())
	}
	return response.Data.Conversation.ID
}

func assertDetachedGoalControlRecord(
	t *testing.T,
	cfg config.Config,
	sessionKey string,
) {
	t.Helper()
	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	sessions := workspacestore.NewSessionFileStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	workspacePath := agentsvc.ResolveWorkspacePath(
		cfg,
		authctx.SystemUserID,
		cfg.DefaultAgentID,
	)
	item, _, err := sessions.FindSession([]string{workspacePath}, sessionKey)
	if err != nil || item == nil {
		t.Fatalf("detached Goal control session = %#v, error = %v", item, err)
	}
	rows, err := history.ReadMessages(workspacePath, *item, nil)
	if err != nil {
		t.Fatalf("read detached Goal control history: %v", err)
	}
	for _, row := range rows {
		metadataSubtype := ""
		switch metadata := row["metadata"].(type) {
		case map[string]string:
			metadataSubtype = metadata["subtype"]
		case map[string]any:
			metadataSubtype, _ = metadata["subtype"].(string)
		}
		if row["role"] == "user" &&
			row["content"] == "/goal Keep Goal submission alive across navigation" &&
			row["client_message_id"] == "message-detached-goal" &&
			metadataSubtype == "goal_set" &&
			row["control_only"] == true {
			return
		}
	}
	t.Fatalf("detached Goal control history = %#v, want durable goal_set record", rows)
}

func assertDetachedRoomGoalControlRecord(
	t *testing.T,
	cfg config.Config,
	conversationID string,
) {
	t.Helper()
	rows, err := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath).ReadMessages(
		authctx.SystemUserID,
		conversationID,
		nil,
	)
	if err != nil {
		t.Fatalf("read detached Room Goal control history: %v", err)
	}
	for _, row := range rows {
		metadata, _ := row["metadata"].(map[string]any)
		if row["role"] == "user" &&
			row["content"] == "/goal Keep Room Goal submission alive across navigation" &&
			row["client_message_id"] == "message-detached-room-goal" &&
			metadata["subtype"] == "goal_set" &&
			row["control_only"] == true {
			return
		}
	}
	t.Fatalf("detached Room Goal control history = %#v, want durable goal_set record", rows)
}
