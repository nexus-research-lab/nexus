// workgraph_test.go 覆盖服务端 WorkGraph 应用装配、重启恢复与脱离 Goal 命令测试。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	dm "github.com/nexus-research-lab/nexus/internal/service/dm"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	room "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
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

func TestWorkGraphApplicationAcceptance(t *testing.T) {
	binary := os.Getenv("NEXUS_SMOKE_NXS_BINARY")
	if os.Getenv("NEXUS_WORKGRAPH_APP_ACCEPTANCE") != "1" || binary == "" {
		t.Skip("set NEXUS_WORKGRAPH_APP_ACCEPTANCE=1 and NEXUS_SMOKE_NXS_BINARY for isolated application acceptance")
	}
	t.Setenv("NEXUS_NXS_COMMAND_PATH", binary)
	for _, scope := range []protocol.ExecutionScopeKind{protocol.ExecutionScopeDM, protocol.ExecutionScopeRoom} {
		t.Run(string(scope), func(t *testing.T) {
			var requests atomic.Int64
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request any
				if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&request); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				if appMaintenancePrompt(request) {
					emitAppText(w, "No durable memory to preserve.")
					return
				}
				declaration := appResultFromPrompt(request)
				if declaration == nil {
					http.Error(w, "missing exact bound result contract", 400)
					return
				}
				requests.Add(1)
				emitAppResult(w, *declaration)
			}))
			t.Cleanup(provider.Close)
			cfg := handlertest.NewConfig(t)
			cfg.AppMode = "desktop"
			cfg.MultiplayerDisabled = true
			handlertest.MigrateSQLite(t, cfg.DatabaseURL)
			server, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			handlertest.CloseServer(t, server)
			ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: authctx.SystemUserID, Username: "local", Role: "admin", AuthMethod: "local"})
			record, err := server.services.Provider.Create(ctx, providersvc.CreateInput{Provider: "app-test", PresetKey: "custom", APIFormat: "anthropic_messages", AuthToken: "local-only", BaseURL: provider.URL, Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			yes := true
			_, err = server.services.Provider.UpdateModel(ctx, record.Provider, "claude-app-test", providersvc.UpdateModelInput{Enabled: true, IsDefault: true, CapabilitiesOverride: providersvc.ModelCapabilities{TextOutput: &yes, ToolCalling: &yes}})
			if err != nil {
				t.Fatal(err)
			}
			actor := configureAppGraph(t, ctx, server, scope)
			stop, err := server.startOrchestrationRecovery(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			stopDispatch, err := server.startExecutionDispatches(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer stopDispatch()
			repository := repo.NewSQLRepository("sqlite", server.services.DB)
			deadline := time.NewTimer(30 * time.Second)
			defer deadline.Stop()
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-deadline.C:
					snapshot, _ := repository.GetSnapshot(ctx, actor.ExecutionID)
					raw, _ := json.Marshal(snapshot)
					t.Fatalf("pipeline timed out: %s", raw)
				case <-ticker.C:
					snapshot, err := repository.GetSnapshot(ctx, actor.ExecutionID)
					if err != nil {
						t.Fatal(err)
					}
					if len(snapshot.Acceptances) == 2 && len(server.services.Runtime.GetRunningRoundIDs(actor.SessionKey)) == 0 && (scope != protocol.ExecutionScopeRoom || server.services.RoomRealtime.GetActiveRoundSnapshot(actor.ConversationID) == nil) {
						if len(snapshot.Submissions) != 2 || len(snapshot.Assignments) != 2 || requests.Load() != 4 {
							t.Fatalf("duplicate or missing work: %d requests", requests.Load())
						}
						t.Logf("%s: two dependency-ordered work rounds + two separate review rounds committed through full application queues and nxs", scope)
						return
					}
				}
			}
		})
	}
}
func appGraphActor(t *testing.T, ctx context.Context, s *Server, scope protocol.ExecutionScopeKind) svc.ActorContext {
	t.Helper()
	agent := s.config.DefaultAgentID
	var value *protocol.ConversationContextAggregate
	var err error
	if scope == protocol.ExecutionScopeDM {
		value, err = s.services.Core.Room.EnsureDirectRoom(ctx, agent)
	} else {
		member, createErr := s.services.Core.Agent.CreateAgent(ctx, protocol.CreateRequest{Name: "WorkGraph acceptance worker"})
		if createErr != nil {
			t.Fatal(createErr)
		}
		agent = member.AgentID
		value, err = s.services.Core.Room.CreateRoom(ctx, protocol.CreateRoomRequest{Name: "WorkGraph acceptance", AgentIDs: []string{agent}, HostAgentID: agent})
	}
	if err != nil {
		t.Fatal(err)
	}
	actor := svc.ActorContext{OwnerUserID: authctx.SystemUserID, AgentID: agent, ScopeKind: scope, Role: svc.ExecutionActorCoordinator, RootRoundID: "app-root", RuntimeRoundID: "app-root", AgentRoundID: "app-agent"}
	if scope == protocol.ExecutionScopeDM {
		actor.SessionKey = protocol.BuildRoomAgentSessionKey(value.Conversation.ID, agent, protocol.RoomTypeDM)
	} else {
		actor.RoomID = value.Room.ID
		actor.ConversationID = value.Conversation.ID
		actor.SessionKey = protocol.BuildRoomSharedSessionKey(value.Conversation.ID)
	}
	return actor
}
func appResultFromPrompt(value any) *protocol.ExecutionResultDeclaration {
	switch v := value.(type) {
	case string:
		marker := "<!-- nexus-workgraph-result\n"
		if !strings.Contains(v, "This round supports durable WorkGraph result writeback.") {
			return nil
		}
		_, tail, ok := strings.Cut(v[strings.LastIndex(v, "This round supports durable WorkGraph result writeback."):], marker)
		if !ok {
			return nil
		}
		var declaration protocol.ExecutionResultDeclaration
		if json.NewDecoder(strings.NewReader(tail)).Decode(&declaration) != nil {
			return nil
		}
		if declaration.Delivery != nil {
			declaration.Delivery = &protocol.ExecutionDeliveryResult{ResultSummary: "Synthetic arithmetic checked", ResultRefs: []string{"fixture:arithmetic"}, Evidence: []string{"2+2=4; 4+1=5"}}
		}
		if declaration.Review != nil {
			declaration.Review = &protocol.ExecutionReviewResult{Decision: protocol.WorkAcceptanceAccepted, CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "verified", Passed: true, Evidence: []string{"2+2=4; 4+1=5"}}}}
		}
		return &declaration
	case []any:
		for i := len(v) - 1; i >= 0; i-- {
			if result := appResultFromPrompt(v[i]); result != nil {
				return result
			}
		}
	case map[string]any:
		for _, key := range []string{"system", "messages", "content"} {
			if child, ok := v[key]; ok {
				if result := appResultFromPrompt(child); result != nil {
					return result
				}
			}
		}
		for _, child := range v {
			if result := appResultFromPrompt(child); result != nil {
				return result
			}
		}
	}
	return nil
}
func emitAppResult(w http.ResponseWriter, declaration protocol.ExecutionResultDeclaration) {
	raw, _ := json.Marshal(declaration)
	reply := "Arithmetic verified.\n<!-- nexus-workgraph-result\n" + string(raw) + "\n-->"
	emitAppText(w, reply)
}
func emitAppText(w http.ResponseWriter, reply string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, event := range []map[string]any{
		{"type": "message_start", "message": map[string]any{"id": "app-reply", "model": "claude-app-test", "role": "assistant", "content": []any{}, "usage": map[string]int{"input_tokens": 1}}},
		{"type": "content_block_start", "index": 0, "content_block": map[string]string{"type": "text", "text": ""}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "text_delta", "text": reply}},
		{"type": "content_block_stop", "index": 0}, {"type": "message_delta", "delta": map[string]string{"stop_reason": "end_turn"}, "usage": map[string]int{"output_tokens": 10}}, {"type": "message_stop"},
	} {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
	}
}

func appMaintenancePrompt(request any) bool {
	root, ok := request.(map[string]any)
	if !ok {
		return false
	}
	messages, ok := root["messages"].([]any)
	if !ok || len(messages) == 0 {
		return false
	}
	raw, _ := json.Marshal(messages[len(messages)-1])
	return strings.Contains(string(raw), "You are now acting as the memory extraction subagent")
}

func configureAppGraph(t *testing.T, ctx context.Context, server *Server, scope protocol.ExecutionScopeKind) svc.ActorContext {
	t.Helper()
	actor := appGraphActor(t, ctx, server, scope)
	orchestration := server.services.Orchestration
	planned, err := orchestration.PlanExecution(ctx, actor, svc.PlanExecutionInput{CommandID: "app-plan", Objective: "verify a two-step pipeline", CompletionCriteria: []string{"verified"}, Draft: svc.PlanDraft{Items: []svc.PlanWorkItemDraft{
		{LogicalKey: "sum", Kind: protocol.WorkItemKindProduce, Subject: "Sum", Objective: "Calculate 2+2", Deliverable: "fixture:sum = 4", AcceptanceCriteria: []string{"verified"}, Required: true, OutputScopes: []protocol.WorkOutputScope{{Scope: "semantic:sum", Mode: protocol.WorkOutputScopeExclusive}}},
		{LogicalKey: "report", Kind: protocol.WorkItemKindProduce, Subject: "Report", Objective: "Use the accepted sum to report 4+1", Deliverable: "fixture:report = 5", AcceptanceCriteria: []string{"verified"}, Required: true, Terminal: true, DependsOn: []svc.PlanDependencyDraft{{LogicalKey: "sum", Kind: protocol.WorkDependencyHard}}, OutputScopes: []protocol.WorkOutputScope{{Scope: "semantic:report", Mode: protocol.WorkOutputScopeExclusive}}},
	}}})
	if err != nil || planned.Outcome != svc.MutationApplied {
		t.Fatalf("plan %+v %v", planned, err)
	}
	actor.ExecutionID = planned.Snapshot.Execution.ID
	input := svc.ConfigureDispatchInput{ExecutionID: actor.ExecutionID, SnapshotRevision: planned.Snapshot.Execution.Version, CommandID: "app-policy"}
	for _, work := range planned.Snapshot.WorkItems {
		input.Policies = append(input.Policies, svc.DispatchPolicyInput{WorkItemID: work.ID, TargetAgentID: actor.AgentID, ReturnToAgentID: actor.AgentID, Instruction: "Use only the synthetic arithmetic fixture and deliver explicit evidence."})
	}
	configured, err := orchestration.ConfigureDispatch(ctx, actor, input)
	if err != nil || configured.Outcome != svc.MutationApplied {
		t.Fatalf("configure %+v %v", configured, err)
	}
	return actor
}

type appRestartState struct {
	Actor   svc.ActorContext
	Attempt protocol.WorkAttempt
}

func TestWorkGraphProcessRestartAcceptance(t *testing.T) {
	if os.Getenv("NEXUS_WORKGRAPH_RESTART_CHILD") != "" {
		runWorkGraphRestartChild(t)
		return
	}
	if os.Getenv("NEXUS_WORKGRAPH_APP_ACCEPTANCE") != "1" || os.Getenv("NEXUS_SMOKE_NXS_BINARY") == "" {
		t.Skip("explicit application acceptance opt-in required")
	}
	for _, scope := range []string{"dm", "room"} {
		for _, scenario := range []string{"stop", "crash"} {
			t.Run(scope+"_"+scenario, func(t *testing.T) {
				cfg := handlertest.NewConfig(t)
				cfg.AppMode = "desktop"
				cfg.MultiplayerDisabled = true
				handlertest.MigrateSQLite(t, cfg.DatabaseURL)
				root := filepath.Dir(cfg.DatabaseURL)
				cfgPath := filepath.Join(root, "config.json")
				raw, _ := json.Marshal(cfg)
				if err := os.WriteFile(cfgPath, raw, 0600); err != nil {
					t.Fatal(err)
				}
				var blocked atomic.Bool
				blocked.Store(true)
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 4<<20))
					if blocked.Load() {
						_ = os.WriteFile(filepath.Join(root, "provider-started"), []byte("inflight"), 0600)
						<-r.Context().Done()
						return
					}
					emitAppText(w, "Recovery inspection only. The external effect is unknown; reconcile its durable receipt before resuming. No operation was replayed.")
					_ = os.WriteFile(filepath.Join(root, "inspection-responded"), []byte("responded"), 0600)
				}))
				t.Cleanup(provider.Close)
				child := func(stage string) *exec.Cmd {
					cmd := exec.Command(os.Args[0], "-test.run=^TestWorkGraphProcessRestartAcceptance$", "-test.v", "-test.timeout=75s")
					cmd.Env = append(os.Environ(), "NEXUS_WORKGRAPH_RESTART_CHILD="+stage, "NEXUS_WORKGRAPH_RESTART_CONFIG="+cfgPath, "NEXUS_WORKGRAPH_RESTART_SCOPE="+scope, "NEXUS_WORKGRAPH_RESTART_SCENARIO="+scenario, "NEXUS_WORKGRAPH_RESTART_PROVIDER="+provider.URL, "NEXUS_NXS_COMMAND_PATH="+os.Getenv("NEXUS_SMOKE_NXS_BINARY"))
					return cmd
				}
				seed := child("seed")
				logPath := filepath.Join(root, "seed.log")
				log, err := os.Create(logPath)
				if err != nil {
					t.Fatal(err)
				}
				seed.Stdout, seed.Stderr = log, log
				if err = seed.Start(); err != nil {
					t.Fatal(err)
				}
				defer func() { _ = seed.Process.Kill(); _ = log.Close() }()
				waitAppCondition(t, 45*time.Second, func() bool { _, err := os.Stat(filepath.Join(root, "suspended.json")); return err == nil }, func() string { data, _ := os.ReadFile(logPath); return string(data) })
				if err = seed.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				if err = seed.Wait(); err == nil {
					t.Fatal("child was not killed")
				}
				_ = log.Close()
				blocked.Store(false)
				verify := child("verify")
				output, err := verify.CombinedOutput()
				if err != nil {
					t.Fatalf("reopened application: %v\n%s", err, output)
				}
				t.Logf("%s/%s: killed owned host process; reopened same isolated state; checkpoint and unique recovery queue receipt verified", scope, scenario)
			})
		}
	}
}
func runWorkGraphRestartChild(t *testing.T) {
	cfgPath := os.Getenv("NEXUS_WORKGRAPH_RESTART_CONFIG")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err = json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(cfgPath)
	stage := os.Getenv("NEXUS_WORKGRAPH_RESTART_CHILD")
	scenario := os.Getenv("NEXUS_WORKGRAPH_RESTART_SCENARIO")
	server, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handlertest.CloseServer(t, server)
	ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: authctx.SystemUserID, Username: "local", Role: "admin", AuthMethod: "local"})
	repository := repo.NewSQLRepository("sqlite", server.services.DB)
	if stage == "seed" {
		record, err := server.services.Provider.Create(ctx, providersvc.CreateInput{Provider: "restart-test", PresetKey: "custom", APIFormat: "anthropic_messages", AuthToken: "local-only", BaseURL: os.Getenv("NEXUS_WORKGRAPH_RESTART_PROVIDER"), Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		yes := true
		_, err = server.services.Provider.UpdateModel(ctx, record.Provider, "claude-app-test", providersvc.UpdateModelInput{Enabled: true, IsDefault: true, CapabilitiesOverride: providersvc.ModelCapabilities{TextOutput: &yes, ToolCalling: &yes}})
		if err != nil {
			t.Fatal(err)
		}
		actor := configureAppGraph(t, ctx, server, protocol.ExecutionScopeKind(os.Getenv("NEXUS_WORKGRAPH_RESTART_SCOPE")))
		stop, err := server.startOrchestrationRecovery(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer stop()
		stopDispatch, err := server.startExecutionDispatches(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer stopDispatch()
		waitAppCondition(t, 30*time.Second, func() bool { _, err := os.Stat(filepath.Join(root, "provider-started")); return err == nil }, func() string { return "real provider request never started" })
		snapshot, err := repository.GetSnapshot(ctx, actor.ExecutionID)
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Attempts) != 1 {
			t.Fatalf("root attempts: %d", len(snapshot.Attempts))
		}
		attempt := snapshot.Attempts[0]
		worker := actor
		worker.Role = svc.ExecutionActorMember
		worker.AgentRoundID = attempt.AgentRoundID
		worker.RuntimeRoundID = attempt.RuntimeRoundID
		worker.WorkBinding = &protocol.ExecutionWorkBinding{ExecutionID: actor.ExecutionID, PlanID: attempt.PlanID, WorkItemID: attempt.WorkItemID, SpecID: attempt.SpecID, AssignmentID: attempt.AssignmentID, AttemptID: attempt.ID, DispatchID: attempt.DispatchID}
		checkpoint, err := server.services.Orchestration.RecordCheckpoint(ctx, worker, svc.RecordCheckpointInput{ExecutionID: actor.ExecutionID, CommandID: "app-checkpoint", SnapshotRevision: snapshot.Execution.Version, Data: protocol.WorkCheckpointData{Summary: "staged synthetic artifact", NextStep: "reconcile simulated publish receipt", ArtifactRefs: []string{"fixture:staged"}, Effects: []protocol.WorkCheckpointEffect{{Operation: "synthetic publish (not executed)", Outcome: "unknown"}}}})
		if err != nil || checkpoint.Outcome != svc.MutationApplied {
			t.Fatalf("checkpoint %+v %v", checkpoint, err)
		}
		if scenario == "stop" {
			if actor.ScopeKind == protocol.ExecutionScopeDM {
				err = server.services.DM.HandleInterrupt(ctx, dm.InterruptRequest{SessionKey: actor.SessionKey, RoundID: attempt.RootRoundID, UserStop: true})
			} else {
				err = server.services.RoomRealtime.HandleInterrupt(ctx, room.InterruptRequest{SessionKey: actor.SessionKey, AgentRoundID: attempt.AgentRoundID, UserStop: true})
			}
			if err != nil {
				t.Fatal(err)
			}
			waitAppCondition(t, 10*time.Second, func() bool {
				s, _ := repository.GetSnapshot(ctx, actor.ExecutionID)
				return s != nil && s.Attempts[0].Status == protocol.WorkAttemptStatusInterrupted
			}, func() string { return "user stop did not settle root" })
		}
		raw, _ := json.Marshal(appRestartState{actor, attempt})
		if err = os.WriteFile(filepath.Join(root, "suspended.tmp"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.Rename(filepath.Join(root, "suspended.tmp"), filepath.Join(root, "suspended.json")); err != nil {
			t.Fatal(err)
		}
		<-t.Context().Done()
		return
	}
	raw, err = os.ReadFile(filepath.Join(root, "suspended.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state appRestartState
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	_, err = server.startRootAttemptRecovery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repository.GetSnapshot(ctx, state.Actor.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Checkpoints) != 1 || snapshot.Checkpoints[0].Effects[0].Outcome != "unknown" || len(snapshot.Submissions) != 0 || len(snapshot.Acceptances) != 0 || snapshot.Attempts[0].Status == protocol.WorkAttemptStatusRunning {
		t.Fatalf("restart did not preserve unknown progress: %+v", snapshot)
	}
	if scenario == "stop" {
		var allowed bool
		if state.Actor.ScopeKind == protocol.ExecutionScopeDM {
			allowed, err = server.services.DM.AutomaticDispatchAllowed(ctx, snapshot.Execution, state.Actor.AgentID)
		} else {
			allowed, err = server.services.RoomRealtime.AutomaticDispatchAllowed(ctx, snapshot.Execution, state.Actor.AgentID)
		}
		if err != nil || allowed {
			t.Fatalf("stop pause was lost: %v", err)
		}
	}
	stopRecovery, err := server.startOrchestrationRecovery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stopRecovery()
	stopDispatch, err := server.startExecutionDispatches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stopDispatch()
	input := protocol.RequestExecutionRecovery{SessionKey: state.Actor.SessionKey, RequestID: "app-explicit-recovery", WorkItemID: state.Attempt.WorkItemID, AttemptID: state.Attempt.ID}
	preview, err := server.services.Orchestration.PreviewWorkRetry(ctx, state.Actor.OwnerUserID, state.Actor.ExecutionID, input)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		receipt, err := server.services.Orchestration.RequestWorkRetry(ctx, state.Actor.OwnerUserID, state.Actor.ExecutionID, protocol.RequestWorkRetry{RequestExecutionRecovery: input, SnapshotRevision: preview.SnapshotRevision})
		if err != nil || !receipt.Queued {
			t.Fatalf("exact recovery receipt %+v %v", receipt, err)
		}
	}
	receipt, err := server.services.Orchestration.GetWorkRecoveryReceipt(ctx, state.Actor.OwnerUserID, state.Actor.ExecutionID, state.Actor.SessionKey, input.RequestID)
	if err != nil || !receipt.Queued {
		t.Fatalf("lost ACK reconciliation %+v %v", receipt, err)
	}
	waitAppCondition(t, 12*time.Second, func() bool {
		_, observed := os.Stat(filepath.Join(root, "inspection-responded"))
		return observed == nil && len(server.services.Runtime.GetRunningRoundIDs(state.Actor.SessionKey)) == 0 && (state.Actor.ScopeKind != protocol.ExecutionScopeRoom || server.services.RoomRealtime.GetActiveRoundSnapshot(state.Actor.ConversationID) == nil)
	}, func() string {
		snap, _ := repository.GetSnapshot(ctx, state.Actor.ExecutionID)
		data, _ := json.Marshal(snap)
		event, queueErr := server.services.RoomRealtime.InputQueueSnapshotEvent(ctx, state.Actor.RoomID, state.Actor.ConversationID)
		queue, _ := json.Marshal(event)
		return fmt.Sprintf("recovery inspection did not drain: snapshot=%s queue=%s err=%v", data, queue, queueErr)
	})
	after, _ := repository.GetSnapshot(ctx, state.Actor.ExecutionID)
	if len(after.Assignments) != 1 || after.Assignments[0].ID == state.Attempt.AssignmentID || after.Assignments[0].OwnerAgentID != state.Attempt.ExecutorAgentID || len(after.Attempts) != 2 || len(after.Submissions) != 0 || len(after.Acceptances) != 0 {
		t.Fatal("continuation changed owner, duplicated attempts or fabricated completion")
	}
	t.Log("same-owner continuation reached a real runtime after restart; prior progress preserved; model inspection did not fabricate submission or completion")
}
func waitAppCondition(t *testing.T, timeout time.Duration, ready func() bool, detail func() string) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ready() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal(strings.TrimSpace(fmt.Sprintf("acceptance condition timed out: %s", detail())))
		case <-ticker.C:
		}
	}
}
