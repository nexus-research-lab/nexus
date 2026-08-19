// INPUT: DM/Room scope, model-created versus host-created Goal, and explicit current/none Plan binding intent.
// OUTPUT: The eight supported product entry paths converge to standalone Goal, confirmed Goal+WorkGraph, or Goal-free WorkGraph state.
// POS: Cross-domain acceptance matrix for the public Goal/WorkGraph product model.
package server

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

const productMatrixPlanDocument = `nexus_plan: 1
operation: create
objective: Complete the product matrix objective
completion_criteria:
  - the matrix result is verified
items:
  - logical_key: verify
    kind: verify
    subject: Verify matrix result
    objective: Verify the requested product path
    deliverable: verified matrix result
    acceptance_criteria:
      - the requested path is verified
    required: true
    terminal: true
    output_scopes:
      - semantic:matrix-result
`

func TestGoalWorkGraphProductMatrix(t *testing.T) {
	tests := []struct {
		name        string
		scope       protocol.ExecutionScopeKind
		entry       string
		createGoal  bool
		createGraph bool
		bindGoal    bool
		createdBy   string
	}{
		{name: "DM dialogue Goal", scope: protocol.ExecutionScopeDM, entry: "dialogue", createGoal: true, createdBy: "model"},
		{name: "DM composer Goal", scope: protocol.ExecutionScopeDM, entry: "composer", createGoal: true, createdBy: "user"},
		{name: "DM dialogue Goal and WorkGraph", scope: protocol.ExecutionScopeDM, entry: "dialogue", createGoal: true, createGraph: true, bindGoal: true, createdBy: "model"},
		{name: "DM dialogue WorkGraph", scope: protocol.ExecutionScopeDM, entry: "dialogue", createGraph: true},
		{name: "Room dialogue Goal", scope: protocol.ExecutionScopeRoom, entry: "dialogue", createGoal: true, createdBy: "model"},
		{name: "Room composer Goal", scope: protocol.ExecutionScopeRoom, entry: "composer", createGoal: true, createdBy: "user"},
		{name: "Room dialogue Goal and WorkGraph", scope: protocol.ExecutionScopeRoom, entry: "dialogue", createGoal: true, createGraph: true, bindGoal: true, createdBy: "model"},
		{name: "Room dialogue WorkGraph", scope: protocol.ExecutionScopeRoom, entry: "dialogue", createGraph: true},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := handlertest.NewConfig(t)
			cfg.GoalEnabled = true
			handlertest.MigrateSQLite(t, cfg.DatabaseURL)
			db, err := OpenDB(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()

			goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
			executionRepo := orchestrationstore.NewRepository(cfg, db)
			executionService := orchestrationsvc.NewService(executionRepo)
			coordinator := newExplicitGoalExecutionCoordinator(goalService, executionService)
			goalService.SetObjectiveRetargetCoordinator(coordinator)
			executionService.SetExplicitGoalBindingGateway(coordinator)
			goalService.SetExecutionGoalCompletionReadiness(executionGoalCompletionReadiness{
				orchestration: executionService,
			})

			ownerID := "owner-product-matrix"
			agentID := "agent-lead"
			conversationID := fmt.Sprintf("product-matrix-%d", index)
			sessionKey := protocol.BuildAgentSessionKey(
				agentID,
				protocol.SessionChannelWebSocketSegment,
				"dm",
				conversationID,
				"",
			)
			actor := orchestrationsvc.ActorContext{
				OwnerUserID: ownerID,
				SessionKey:  sessionKey,
				AgentID:     agentID,
				Role:        orchestrationsvc.ExecutionActorCoordinator,
				ActorKind:   protocol.ExecutionActorAgent,
				ScopeKind:   test.scope,
				RootRoundID: "round-product-matrix",
			}
			if test.scope == protocol.ExecutionScopeRoom {
				actor.RoomID = "room-" + conversationID
				actor.ConversationID = conversationID
				sessionKey = protocol.BuildRoomSharedSessionKey(conversationID)
				actor.SessionKey = sessionKey
			}
			goalAuthority := runtimectx.NewGoalAuthorityState("", 0, "")
			responsibility := runtimectx.NewResponsibilityAuthorityState(
				goalAuthority, "", nil, nil,
			)
			commandActor := productMatrixCommandActor(
				ownerID, agentID, sessionKey, conversationID, test.scope,
				goalAuthority, responsibility,
			)

			var goal *protocol.Goal
			if test.createGoal {
				if test.entry == "composer" {
					goal, err = goalService.Create(context.Background(), protocol.CreateGoalRequest{
						SessionKey:      sessionKey,
						Objective:       "Complete the product matrix objective",
						CreatedBy:       test.createdBy,
						RoundID:         "round-product-matrix",
						OwnerUserID:     ownerID,
						AgentID:         agentID,
						RoomLeadAgentID: agentID,
					})
					if err == nil {
						// Composer acceptance and the runtime command happen in
						// different physical rounds. Model the host-issued exact
						// successor authority instead of mutating through the HTTP
						// request context.
						goalAuthority = runtimectx.NewGoalAuthorityState(
							goal.ID, goal.ObjectiveRevision(), "",
						)
						responsibility = runtimectx.NewResponsibilityAuthorityState(
							goalAuthority, "", nil, nil,
						)
						commandActor = productMatrixCommandActor(
							ownerID, agentID, sessionKey, conversationID, test.scope,
							goalAuthority, responsibility,
						)
					}
				} else {
					commandValue, commandErr := handleGoalRuntimeCommand(
						context.Background(),
						goalService,
						commandActor,
						runtimecommand.Request{
							Domain:    runtimecommand.DomainGoal,
							Action:    runtimecommand.ActionInvoke,
							Operation: runtimecommand.GoalOperationCreate,
							RequestID: "goal-create-matrix",
							Input: map[string]any{
								"objective": "Complete the product matrix objective",
							},
						},
					)
					result := productMatrixCommandResult(t, commandValue, commandErr)
					if result.IsError || result.StructuredContent["outcome"] != string(protocol.MutationResultApplied) {
						t.Fatalf("create_goal command result = %#v", result)
					}
					goal, err = goalService.Current(context.Background(), sessionKey)
				}
				if err != nil {
					t.Fatal(err)
				}
				actor.GoalID = goal.ID
				actor.GoalObjectiveRevision = goal.ObjectiveRevision()
				if test.entry == "dialogue" && test.bindGoal {
					// The immutable launch copy predates create_goal. Execution must
					// consume the Actor's host-owned dynamic authority advanced by
					// the successful command in this same physical round.
					commandActor.Round.CommandContext.GoalAuthority =
						runtimectx.NewGoalAuthorityState("", 0, "")
					commandActor.Round.CommandContext.ResponsibilityAuthority = nil
				}
				if got := protocol.GoalExecutionBindingStateFromGoal(*goal); got !=
					protocol.GoalExecutionBindingStateStandalone {
					t.Fatalf("%s Goal create state = %q, want standalone", test.entry, got)
				}
			}

			var execution *protocol.ExecutionSnapshot
			if test.createGraph {
				intent := orchestrationsvc.PlanGoalBindingNone
				if test.bindGoal {
					intent = orchestrationsvc.PlanGoalBindingCurrent
				}
				inspectValue, inspectErr := handleExecutionRuntimeCommand(
					context.Background(), executionService, commandActor,
					runtimecommand.Request{
						Domain: runtimecommand.DomainExecution,
						Action: runtimecommand.ActionInspect,
					},
				)
				inspect := productMatrixCommandResult(t, inspectValue, inspectErr)
				if inspect.IsError {
					t.Fatalf("execution inspect result = %#v", inspect)
				}
				if _, contractErr := handleExecutionRuntimeCommand(
					context.Background(), executionService, commandActor,
					runtimecommand.Request{
						Domain:    runtimecommand.DomainExecution,
						Action:    runtimecommand.ActionContract,
						Operation: "prepare_plan_execution",
					},
				); contractErr != nil {
					t.Fatal(contractErr)
				}
				preparedValue, preparedErr := handleExecutionRuntimeCommand(
					context.Background(), executionService, commandActor,
					runtimecommand.Request{
						Domain:    runtimecommand.DomainExecution,
						Action:    runtimecommand.ActionInvoke,
						Operation: "prepare_plan_execution",
						RequestID: "prepare-" + strings.ReplaceAll(test.name, " ", "-"),
						Input: map[string]any{
							"plan_document": productMatrixPlanDocument,
							"goal_binding":  string(intent),
						},
					},
				)
				prepared := productMatrixCommandResult(t, preparedValue, preparedErr)
				if prepared.IsError || prepared.StructuredContent["outcome"] != "prepared" {
					t.Fatalf("prepare_plan_execution command result = %#v", prepared)
				}
				proposalID, _ := prepared.StructuredContent["proposal_id"].(string)
				proposalDigest, _ := prepared.StructuredContent["proposal_digest"].(string)
				materializedValue, materializedErr := handleExecutionRuntimeCommand(
					context.Background(), executionService, commandActor,
					runtimecommand.Request{
						Domain:    runtimecommand.DomainExecution,
						Action:    runtimecommand.ActionInvoke,
						Operation: "plan_execution",
						RequestID: "materialize-" + strings.ReplaceAll(test.name, " ", "-"),
						Input: map[string]any{
							"proposal_id":     proposalID,
							"proposal_digest": proposalDigest,
						},
					},
				)
				materialized := productMatrixCommandResult(t, materializedValue, materializedErr)
				if materialized.IsError ||
					materialized.StructuredContent["outcome"] != string(orchestrationsvc.MutationApplied) {
					t.Fatalf("plan_execution command result = %#v", materialized)
				}
				execution, err = executionService.ReadCurrent(context.Background(), actor)
				if err != nil || execution == nil {
					t.Fatalf("read materialized WorkGraph: snapshot=%#v err=%v", execution, err)
				}
			}

			switch {
			case test.createGoal && test.bindGoal:
				current, currentErr := goalService.Current(context.Background(), sessionKey)
				if currentErr != nil {
					t.Fatal(currentErr)
				}
				resolution, resolveErr := executionService.ResolveGoalExecutionBinding(context.Background(), *current)
				if resolveErr != nil {
					t.Fatal(resolveErr)
				}
				if resolution.State != protocol.GoalExecutionBindingStateConfirmed ||
					resolution.ExecutionID != execution.Execution.ID ||
					execution.Execution.GoalID != current.ID {
					t.Fatalf("Goal+WorkGraph state = goal:%#v execution:%#v resolution:%#v", current, execution, resolution)
				}
			case test.createGoal:
				current, currentErr := goalService.Current(context.Background(), sessionKey)
				if currentErr != nil {
					t.Fatal(currentErr)
				}
				resolution, resolveErr := executionService.ResolveGoalExecutionBinding(context.Background(), *current)
				if resolveErr != nil || resolution.State != protocol.GoalExecutionBindingStateStandalone ||
					resolution.ExecutionID != "" || resolution.ReservedExecutionID != "" {
					t.Fatalf("Goal-only state = %#v err=%v", resolution, resolveErr)
				}
			case test.createGraph:
				if execution.Execution.GoalID != "" || execution.Execution.GoalObjectiveRevision != 0 {
					t.Fatalf("WorkGraph-only Execution absorbed Goal authority: %#v", execution.Execution)
				}
				current, currentErr := goalService.CurrentOptional(context.Background(), sessionKey)
				if currentErr != nil || current != nil {
					t.Fatalf("WorkGraph-only path created Goal = %#v err=%v", current, currentErr)
				}
			}
		})
	}
}

func productMatrixCommandActor(
	ownerID string,
	agentID string,
	sessionKey string,
	conversationID string,
	scope protocol.ExecutionScopeKind,
	goalAuthority *runtimectx.GoalAuthorityState,
	responsibility *runtimectx.ResponsibilityAuthorityState,
) runtimecommand.Actor {
	sourceContextType := "agent"
	roomID := ""
	roundConversationID := ""
	if scope == protocol.ExecutionScopeRoom {
		sourceContextType = "room"
		roomID = "room-" + conversationID
		roundConversationID = conversationID
	}
	round := runtimecommand.RoundContext{
		SessionKey: sessionKey,
		RoundID:    "round-product-matrix",
		Receipts:   runtimecommand.NewReceiptState(),
		Resources:  runtimecommand.NewRoundResources(),
		CommandContext: runtimectx.RuntimeCommandContext{
			Agent:                   &protocol.Agent{AgentID: agentID, OwnerUserID: ownerID},
			ScopeSessionKey:         sessionKey,
			RuntimeSessionKey:       "runtime:" + agentID + ":" + conversationID,
			GoalAuthority:           goalAuthority,
			ResponsibilityAuthority: responsibility,
			RootRoundID:             "round-product-matrix",
			AgentRoundID:            "round-product-matrix:" + agentID,
			SourceContextType:       sourceContextType,
			RoomID:                  roomID,
			ConversationID:          roundConversationID,
			CoordinatorAgentID:      agentID,
		},
	}
	return runtimecommand.Actor{
		OwnerUserID:             ownerID,
		AgentID:                 agentID,
		SessionKey:              sessionKey,
		RoundID:                 round.RoundID,
		SourceContextType:       sourceContextType,
		SourceContextID:         roomID,
		GoalMutationAuthority:   goalAuthority,
		GoalResponsibilityState: responsibility,
		Round:                   round,
	}
}

func productMatrixCommandResult(t *testing.T, value any, err error) runtimecommand.Result {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	result, ok := value.(runtimecommand.Result)
	if !ok {
		t.Fatalf("runtime command result type = %T", value)
	}
	return result
}
