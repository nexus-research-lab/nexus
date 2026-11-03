// INPUT: Opt-in paired nxs binary, isolated complete AppServices and scripted local provider.
// OUTPUT: Real DM/Room queues, runtime hooks and dependency dispatch through authoritative SQL.
// POS: Application acceptance; scripted provider isolates host behavior from model quality.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

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
