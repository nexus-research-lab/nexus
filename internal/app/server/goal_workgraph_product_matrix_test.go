// INPUT: DM/Room scope, model-created versus host-created Goal, and explicit current/none Plan binding intent.
// OUTPUT: The eight supported product entry paths converge to standalone Goal, confirmed Goal+WorkGraph, or Goal-free WorkGraph state.
// POS: Cross-domain acceptance matrix for the public Goal/WorkGraph product model.
package server

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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

			var goal *protocol.Goal
			if test.createGoal {
				request := protocol.CreateGoalRequest{
					SessionKey:      sessionKey,
					Objective:       "Complete the product matrix objective",
					CreatedBy:       test.createdBy,
					RoundID:         "round-product-matrix",
					OwnerUserID:     ownerID,
					AgentID:         agentID,
					RoomLeadAgentID: agentID,
				}
				if test.entry == "composer" {
					goal, err = goalService.Create(context.Background(), request)
				} else {
					goal, err = coordinator.Create(context.Background(), request)
				}
				if err != nil {
					t.Fatal(err)
				}
				actor.GoalID = goal.ID
				actor.GoalObjectiveRevision = goal.ObjectiveRevision()
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
				proposal, prepareErr := executionService.PreparePlanExecution(
					context.Background(),
					actor,
					orchestrationsvc.PreparePlanExecutionInput{
						CommandID:    "prepare-" + strings.ReplaceAll(test.name, " ", "-"),
						PlanDocument: productMatrixPlanDocument,
						GoalBinding:  intent,
					},
				)
				if prepareErr != nil {
					t.Fatal(prepareErr)
				}
				result, materializeErr := executionService.MaterializePlanExecution(
					context.Background(),
					actor,
					orchestrationsvc.MaterializePlanExecutionInput{
						ProposalID:     proposal.ID,
						ProposalDigest: proposal.ContentDigest,
					},
				)
				if materializeErr != nil {
					t.Fatal(materializeErr)
				}
				if result.Outcome != orchestrationsvc.MutationApplied || result.Snapshot == nil {
					t.Fatalf("materialize result = %#v", result)
				}
				execution = result.Snapshot
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
