// INPUT: Owned application subprocess, paired nxs, local blocking provider and persistent temporary state.
// OUTPUT: OS kill/reopen, real stop pause, checkpoint durability and exact recovery receipt proofs for DM/Room.
// POS: Current-host lifecycle acceptance; no installed app data or external effects are touched.
package server

import (
	"encoding/json"
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dm "github.com/nexus-research-lab/nexus/internal/service/dm"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	room "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
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
)

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
	for range 2 {
		receipt, err := server.services.Orchestration.RequestWorkRecovery(ctx, state.Actor.OwnerUserID, state.Actor.ExecutionID, input)
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
