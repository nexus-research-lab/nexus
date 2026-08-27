// INPUT: Composer 可复制的内置 WorkGraph Slash、fresh DM/Room Session 与脚本化 Agent runtime。
// OUTPUT: Slash runtime 展开、Execution CLI 全生命周期和 SQLite 终态的组合验收证据。
// POS: 内置模板从目录命令到真实会话、Plan、责任链、Submission、Acceptance 与自动完成的端到端回归。
package server

import (
	"context"
	"errors"
	"fmt"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"strings"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	serverruntime "github.com/nexus-research-lab/nexus/internal/app/server/runtime"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	workgraphworkflowstore "github.com/nexus-research-lab/nexus/internal/storage/workgraphworkflow"
	"gopkg.in/yaml.v3"
)

const builtinDeepResearchE2ETask = "Compare LangGraph and CrewAI orchestration patterns and deliver a source-backed research brief"

type builtinAdaptiveWorkflowScenario struct {
	slashName        string
	task             string
	targetIterations int
	gateIteration    func(string) (int, bool)
	appendIteration  func(protocol.WorkGraphWorkflow, int) protocol.WorkGraphWorkflow
	parallelKeys     func(int) (string, string, bool)
}

func deepResearchE2EScenario(targetIterations int) builtinAdaptiveWorkflowScenario {
	return builtinAdaptiveWorkflowScenario{
		slashName:        "deep-research",
		task:             builtinDeepResearchE2ETask,
		targetIterations: targetIterations,
		gateIteration:    evidenceEvaluationIteration,
		appendIteration:  appendDeepResearchIteration,
		parallelKeys: func(iteration int) (string, string, bool) {
			if iteration == 1 {
				return "authoritative-evidence-1", "contrasting-evidence-1", true
			}
			return fmt.Sprintf("targeted-authoritative-evidence-%d", iteration),
				fmt.Sprintf("targeted-contrasting-evidence-%d", iteration), true
		},
	}
}

func TestBuiltinDeepResearchCopiedSlashAdaptsIterationsToEvidenceInFreshDMAndRoom(t *testing.T) {
	for _, test := range []struct {
		name             string
		scope            protocol.ExecutionScopeKind
		targetIterations int
	}{
		{name: "fresh DM Session needs two iterations", scope: protocol.ExecutionScopeDM, targetIterations: 2},
		{name: "fresh Room Session needs three iterations", scope: protocol.ExecutionScopeRoom, targetIterations: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			runBuiltinAdaptiveWorkflowSessionE2E(
				t,
				test.scope,
				deepResearchE2EScenario(test.targetIterations),
			)
		})
	}
}

func TestOtherBuiltinWorkflowsCopiedSlashAdaptIterationsInFreshDMAndRoom(t *testing.T) {
	for _, scenario := range otherBuiltinAdaptiveE2EScenarios() {
		for _, scopeTest := range []struct {
			name             string
			scope            protocol.ExecutionScopeKind
			targetIterations int
		}{
			{name: "fresh DM needs two iterations", scope: protocol.ExecutionScopeDM, targetIterations: 2},
			{name: "fresh Room needs three iterations", scope: protocol.ExecutionScopeRoom, targetIterations: 3},
		} {
			scenario := scenario
			scenario.targetIterations = scopeTest.targetIterations
			t.Run(scenario.slashName+"/"+scopeTest.name, func(t *testing.T) {
				runBuiltinAdaptiveWorkflowSessionE2E(t, scopeTest.scope, scenario)
			})
		}
	}
}

func runBuiltinAdaptiveWorkflowSessionE2E(
	t *testing.T,
	scope protocol.ExecutionScopeKind,
	scenario builtinAdaptiveWorkflowScenario,
) {
	t.Helper()
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ownerID := "owner-builtin-workgraph-e2e-" + scenario.slashName
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: ownerID, Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodLocal,
	})
	core := NewCoreServicesWithDB(cfg, db)
	agentValue, err := core.Agent.CreateAgent(ctx, protocol.CreateRequest{Name: "Research Lead"})
	if err != nil {
		t.Fatal(err)
	}
	executionRepository := orchestrationstore.NewRepository(cfg, db)
	executions := orchestrationsvc.NewService(executionRepository)
	workflows := workgraphworkflowsvc.NewService(
		workgraphworkflowstore.NewRepository(cfg, db),
		executions,
	)
	workflow, copiedCommand, err := builtinWorkflowAndCopiedCommand(
		ctx,
		workflows,
		ownerID,
		scenario.slashName,
		scenario.task,
	)
	if err != nil {
		t.Fatal(err)
	}

	roundID := "round-builtin-" + scenario.slashName + "-" + string(scope)
	scopeSessionKey := ""
	runtimeSessionKey := ""
	roomID := ""
	conversationID := ""
	var roomContext *protocol.ConversationContextAggregate
	switch scope {
	case protocol.ExecutionScopeDM:
		scopeSessionKey = protocol.BuildAgentSessionKey(
			agentValue.AgentID,
			protocol.SessionChannelWebSocketSegment,
			protocol.RoomTypeDM,
			"builtin-"+scenario.slashName+"-e2e",
			"",
		)
		runtimeSessionKey = scopeSessionKey
		if _, err = core.Session.CreateSession(ctx, sessionsvc.CreateRequest{
			SessionKey: scopeSessionKey,
			AgentID:    agentValue.AgentID,
			Title:      workflow.Title + " E2E",
		}); err != nil {
			t.Fatalf("create fresh DM Session: %v", err)
		}
	case protocol.ExecutionScopeRoom:
		roomContext, err = core.Room.CreateRoom(ctx, protocol.CreateRoomRequest{
			AgentIDs:             []string{agentValue.AgentID},
			Name:                 workflow.Title + " E2E Room",
			HostAgentID:          agentValue.AgentID,
			HostAutoReplyEnabled: true,
		})
		if err != nil {
			t.Fatalf("create fresh Room Session: %v", err)
		}
		roomID = roomContext.Room.ID
		conversationID = roomContext.Conversation.ID
		scopeSessionKey = protocol.BuildRoomSharedSessionKey(conversationID)
		runtimeSessionKey = protocol.BuildRoomAgentSessionKey(
			conversationID,
			agentValue.AgentID,
			protocol.RoomTypeGroup,
		)
	default:
		t.Fatalf("unsupported scope %q", scope)
	}

	commandActor := builtinWorkflowCommandActor(
		ownerID,
		agentValue,
		scope,
		scopeSessionKey,
		runtimeSessionKey,
		roomID,
		conversationID,
		roundID,
	)
	client := newBuiltinWorkflowE2EClient(func(runCtx context.Context, prompt string) builtinWorkflowE2EOutcome {
		runResult, runErr := runWorkflowThroughRuntimeCommands(
			runCtx,
			executions,
			executionRepository,
			commandActor,
			workflow,
			scenario,
		)
		return builtinWorkflowE2EOutcome{
			prompt: prompt, execution: runResult, err: runErr,
		}
	})
	factory := &builtinWorkflowE2EFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = runtimeManager.CloseOwnerSessions(closeCtx, ownerID)
	})
	permissions := permissionctx.NewContext()

	switch scope {
	case protocol.ExecutionScopeDM:
		chat := dmsvc.NewService(cfg, core.Agent, runtimeManager, permissions)
		chat.SetRuntimeSlashExpander(workflows)
		chat.SetExecutionContextProvider(executions)
		if err = chat.HandleChat(ctx, dmsvc.Request{
			SessionKey: scopeSessionKey,
			AgentID:    agentValue.AgentID,
			Content:    copiedCommand,
			RoundID:    roundID,
		}); err != nil {
			t.Fatalf("run copied command in fresh DM Session: %v", err)
		}
	case protocol.ExecutionScopeRoom:
		chat := roomrealtime.NewServiceWithFactory(
			cfg,
			core.Room,
			core.Agent,
			runtimeManager,
			permissions,
			factory,
		)
		chat.SetRuntimeSlashExpander(workflows)
		chat.SetExecutionContextProvider(executions)
		executions.SetAssignmentTargetAuthorizer(chat)
		if err = chat.HandleChat(ctx, roomrealtime.ChatRequest{
			SessionKey:         scopeSessionKey,
			RoomID:             roomID,
			ConversationID:     conversationID,
			CoordinatorAgentID: agentValue.AgentID,
			Content:            copiedCommand,
			RoundID:            roundID,
		}); err != nil {
			t.Fatalf("run copied command in fresh Room Session: %v", err)
		}
	}

	var outcome builtinWorkflowE2EOutcome
	select {
	case outcome = <-client.outcomes:
	case <-time.After(10 * time.Second):
		t.Fatal("scripted runtime did not finish the copied WorkGraph command")
	}
	if outcome.err != nil {
		t.Fatalf("copied WorkGraph command did not complete: %v", outcome.err)
	}
	waitForBuiltinWorkflowRounds(t, runtimeManager, scopeSessionKey, runtimeSessionKey)
	assertBuiltinWorkflowRuntimePrompt(t, outcome.prompt, workflow, scenario)
	assertCompletedBuiltinWorkflowExecution(
		t,
		outcome.execution,
		scope,
		scopeSessionKey,
		roomID,
		conversationID,
		scenario,
	)
	assertCopiedSlashPersisted(t, core.Session, ctx, scopeSessionKey, copiedCommand)

	switch scope {
	case protocol.ExecutionScopeDM:
		sessionValue, getErr := core.Session.GetSession(ctx, scopeSessionKey)
		if getErr != nil || sessionValue == nil || sessionValue.SessionID == nil ||
			strings.TrimSpace(*sessionValue.SessionID) != client.sessionID {
			t.Fatalf("fresh DM Session runtime identity = %#v, err=%v", sessionValue, getErr)
		}
	case protocol.ExecutionScopeRoom:
		stored, getErr := core.Room.GetConversationContext(ctx, conversationID)
		if getErr != nil || stored == nil || len(stored.Sessions) != 1 ||
			stored.Sessions[0].SDKSessionID != client.sessionID ||
			stored.Room.ID != roomContext.Room.ID {
			t.Fatalf("fresh Room Session runtime identity = %#v, err=%v", stored, getErr)
		}
	}
}

func builtinWorkflowAndCopiedCommand(
	ctx context.Context,
	service *workgraphworkflowsvc.Service,
	ownerID string,
	slashName string,
	task string,
) (protocol.WorkGraphWorkflow, string, error) {
	commands, err := service.CommandDescriptors(ctx, ownerID)
	if err != nil {
		return protocol.WorkGraphWorkflow{}, "", err
	}
	commandName := ""
	for _, command := range commands {
		if command.Name == slashName {
			commandName = command.Name
			break
		}
	}
	if commandName == "" {
		return protocol.WorkGraphWorkflow{}, "", fmt.Errorf("%s is missing from the copied command catalog", slashName)
	}
	items, err := service.ListLocalized(ctx, ownerID, "en")
	if err != nil {
		return protocol.WorkGraphWorkflow{}, "", err
	}
	for _, item := range items {
		if item.SlashName == commandName {
			return item, "/" + commandName + " " + strings.TrimSpace(task), nil
		}
	}
	return protocol.WorkGraphWorkflow{}, "", fmt.Errorf("%s template is missing from the workflow directory", slashName)
}

func builtinWorkflowCommandActor(
	ownerID string,
	agentValue *protocol.Agent,
	scope protocol.ExecutionScopeKind,
	scopeSessionKey string,
	runtimeSessionKey string,
	roomID string,
	conversationID string,
	roundID string,
) command.Actor {
	goalAuthority := runtimectx.NewGoalAuthorityState("", 0, "")
	responsibility := runtimectx.NewResponsibilityAuthorityState(
		goalAuthority,
		"",
		nil,
		nil,
	)
	sourceContextType := "agent"
	if scope == protocol.ExecutionScopeRoom {
		sourceContextType = "room"
	}
	agentRoundID := roundID + ":" + agentValue.AgentID
	return command.Actor{
		OwnerUserID:             ownerID,
		AgentID:                 agentValue.AgentID,
		SessionKey:              scopeSessionKey,
		RoundID:                 roundID,
		LeaseSessionKey:         runtimeSessionKey,
		LeaseRoundID:            agentRoundID,
		SourceContextType:       sourceContextType,
		SourceContextID:         roomID,
		GoalMutationAuthority:   goalAuthority,
		GoalResponsibilityState: responsibility,
		Round: nexusmcp.RoundContext{
			SessionKey:      scopeSessionKey,
			RoundID:         roundID,
			CommandReceipts: nexusmcp.NewCommandReceiptState(),
			CommandContext: runtimectx.RuntimeCommandContext{
				Agent:                   agentValue,
				ScopeSessionKey:         scopeSessionKey,
				RuntimeSessionKey:       runtimeSessionKey,
				GoalAuthority:           goalAuthority,
				ResponsibilityAuthority: responsibility,
				RootRoundID:             roundID,
				AgentRoundID:            agentRoundID,
				SourceContextType:       sourceContextType,
				RoomID:                  roomID,
				ConversationID:          conversationID,
				CoordinatorAgentID:      agentValue.AgentID,
			},
		},
	}
}

func runWorkflowThroughRuntimeCommands(
	ctx context.Context,
	service *orchestrationsvc.Service,
	repository *orchestrationstore.Repository,
	actor command.Actor,
	workflow protocol.WorkGraphWorkflow,
	scenario builtinAdaptiveWorkflowScenario,
) (builtinWorkflowExecutionResult, error) {
	if _, err := invokeBuiltinWorkflowCommand(ctx, service, actor, command.Request{
		Domain: command.DomainExecution,
		Action: command.ActionInspect,
	}); err != nil {
		return builtinWorkflowExecutionResult{}, fmt.Errorf("inspect fresh Execution: %w", err)
	}
	planDocument, err := builtinWorkflowPlanDocument(workflow, scenario.task)
	if err != nil {
		return builtinWorkflowExecutionResult{}, err
	}
	prepared, err := invokeBuiltinWorkflowCommand(ctx, service, actor, command.Request{
		Domain:    command.DomainExecution,
		Action:    command.ActionInvoke,
		Operation: "prepare_plan_execution",
		RequestID: "prepare-builtin-" + scenario.slashName,
		Input: map[string]any{
			"plan_document": planDocument,
			"goal_binding":  "none",
		},
	})
	if err != nil || prepared.StructuredContent["outcome"] != "prepared" {
		return builtinWorkflowExecutionResult{}, fmt.Errorf("prepare template Plan: result=%#v err=%v", prepared, err)
	}
	materialized, err := invokeBuiltinWorkflowCommand(ctx, service, actor, command.Request{
		Domain:    command.DomainExecution,
		Action:    command.ActionInvoke,
		Operation: "plan_execution",
		RequestID: "materialize-builtin-" + scenario.slashName,
		Input:     map[string]any{},
	})
	if err != nil || materialized.StructuredContent["outcome"] != string(orchestrationsvc.MutationApplied) {
		return builtinWorkflowExecutionResult{}, fmt.Errorf("materialize template Plan: result=%#v err=%v", materialized, err)
	}
	executionID, _ := materialized.StructuredContent["execution_id"].(string)
	if strings.TrimSpace(executionID) == "" {
		return builtinWorkflowExecutionResult{}, errors.New("materialized template Plan returned no execution_id")
	}

	activeWorkflow := workflow
	attemptByLogicalKey := make(map[string]int)
	parallelReadyIterations := make(map[int]bool)
	maxReady := 0
	for step := 0; step < scenario.targetIterations*12+len(workflow.Nodes); step++ {
		snapshot, getErr := repository.GetSnapshot(ctx, executionID)
		if getErr != nil {
			return builtinWorkflowExecutionResult{}, getErr
		}
		if snapshot.Execution.Status == protocol.ExecutionStatusCompleted {
			stateSnapshot, history, stateErr := repository.GetWorkGraphState(ctx, executionID)
			return builtinWorkflowExecutionResult{
				Snapshot:                stateSnapshot,
				History:                 history,
				Workflow:                activeWorkflow,
				ParallelReadyIterations: parallelReadyIterations,
				MaxReady:                maxReady,
			}, stateErr
		}
		if len(snapshot.ReadyWorkItemIDs) > maxReady {
			maxReady = len(snapshot.ReadyWorkItemIDs)
		}
		markParallelWorkflowIterations(snapshot, scenario, parallelReadyIterations)
		logicalKey := firstReadyWorkflowLogicalKey(snapshot, activeWorkflow)
		if logicalKey == "" {
			return builtinWorkflowExecutionResult{Snapshot: snapshot}, fmt.Errorf(
				"Execution stalled before completion: status=%s ready=%v blockers=%v",
				snapshot.Execution.Status,
				snapshot.ReadyWorkItemIDs,
				snapshot.CompletionBlockers,
			)
		}
		attemptByLogicalKey[logicalKey]++
		attemptNumber := attemptByLogicalKey[logicalKey]
		requestSuffix := fmt.Sprintf("%s-%d", logicalKey, attemptNumber)
		assigned, assignErr := invokeBuiltinWorkflowCommand(ctx, service, actor, command.Request{
			Domain:    command.DomainExecution,
			Action:    command.ActionInvoke,
			Operation: "assign_work",
			RequestID: "assign-builtin-" + requestSuffix,
			Input: map[string]any{
				"logical_key":     logicalKey,
				"target_agent_id": actor.AgentID,
				"strategy":        string(protocol.AssignmentStrategySelf),
			},
		})
		if assignErr != nil || assigned.StructuredContent["outcome"] != string(orchestrationsvc.MutationApplied) {
			return builtinWorkflowExecutionResult{Snapshot: snapshot}, fmt.Errorf("assign %s: result=%#v err=%v", logicalKey, assigned, assignErr)
		}
		iteration, isEvaluation := scenario.gateIteration(logicalKey)
		insufficient := isEvaluation && iteration < scenario.targetIterations
		submitInput := map[string]any{
			"result_summary": "Completed " + logicalKey + " for: " + scenario.task,
			"result_refs":    []string{"semantic:builtin-e2e/" + logicalKey},
			"evidence":       []string{"fixture-evidence:" + logicalKey},
		}
		if insufficient {
			submitInput["result_summary"] = fmt.Sprintf("Iteration %d %s gate verdict: insufficient; a targeted iteration %d graph change is required.", iteration, scenario.slashName, iteration+1)
			submitInput["evidence"] = []string{fmt.Sprintf("fixture-gap:%s-iteration-%d-insufficient", scenario.slashName, iteration)}
		} else if isEvaluation {
			submitInput["result_summary"] = fmt.Sprintf("Iteration %d %s gate verdict: sufficient; terminal integration may proceed.", iteration, scenario.slashName)
			submitInput["evidence"] = []string{fmt.Sprintf("fixture-gate:%s-iteration-%d-sufficient", scenario.slashName, iteration)}
		}
		if workflowScope(actor) == protocol.ExecutionScopeDM {
			submitInput["logical_key"] = logicalKey
		}
		submitted, submitErr := invokeBuiltinWorkflowCommand(ctx, service, actor, command.Request{
			Domain:    command.DomainExecution,
			Action:    command.ActionInvoke,
			Operation: "submit_work",
			RequestID: "submit-builtin-" + requestSuffix,
			Input:     submitInput,
		})
		if submitErr != nil || submitted.StructuredContent["outcome"] != string(orchestrationsvc.MutationApplied) {
			return builtinWorkflowExecutionResult{Snapshot: snapshot}, fmt.Errorf("submit %s: result=%#v err=%v", logicalKey, submitted, submitErr)
		}
		node := workflowNodeByLogicalKey(activeWorkflow, logicalKey)
		criteriaResults := make([]any, 0, len(node.AcceptanceCriteria))
		for _, criterion := range node.AcceptanceCriteria {
			criteriaResults = append(criteriaResults, map[string]any{
				"criterion": criterion,
				"passed":    true,
				"evidence":  []string{"fixture-review:" + logicalKey},
			})
		}
		reviewInput := map[string]any{
			"decision":         string(protocol.WorkAcceptanceAccepted),
			"criteria_results": criteriaResults,
			"feedback":         "The work result accurately satisfies this iteration stage's acceptance contract.",
		}
		if workflowScope(actor) == protocol.ExecutionScopeDM {
			reviewInput["logical_key"] = logicalKey
		}
		reviewed, reviewErr := invokeBuiltinWorkflowCommand(ctx, service, actor, command.Request{
			Domain:    command.DomainExecution,
			Action:    command.ActionInvoke,
			Operation: "review_work",
			RequestID: "review-builtin-" + requestSuffix,
			Input:     reviewInput,
		})
		if reviewErr != nil || reviewed.StructuredContent["outcome"] != string(orchestrationsvc.MutationApplied) {
			return builtinWorkflowExecutionResult{Snapshot: snapshot}, fmt.Errorf("review %s: result=%#v err=%v", logicalKey, reviewed, reviewErr)
		}
		if insufficient {
			activeWorkflow = scenario.appendIteration(activeWorkflow, iteration+1)
			replanDocument, documentErr := workflowPlanDocument(
				activeWorkflow,
				scenario.task,
				protocol.ExecutionPlanProposalReplan,
				fmt.Sprintf("Iteration %d %s gate was insufficient; append a targeted iteration", iteration, scenario.slashName),
				true,
			)
			if documentErr != nil {
				return builtinWorkflowExecutionResult{Snapshot: snapshot}, documentErr
			}
			iterationSuffix := fmt.Sprintf("iteration-%d", iteration+1)
			prepared, err = invokeBuiltinWorkflowCommand(ctx, service, actor, command.Request{
				Domain:    command.DomainExecution,
				Action:    command.ActionInvoke,
				Operation: "prepare_plan_execution",
				RequestID: "prepare-builtin-" + scenario.slashName + "-" + iterationSuffix,
				Input: map[string]any{
					"plan_document": replanDocument,
					"goal_binding":  "inherit",
				},
			})
			if err != nil || prepared.StructuredContent["outcome"] != "prepared" {
				return builtinWorkflowExecutionResult{Snapshot: snapshot}, fmt.Errorf("prepare adaptive %s: result=%#v err=%v", iterationSuffix, prepared, err)
			}
			materialized, err = invokeBuiltinWorkflowCommand(ctx, service, actor, command.Request{
				Domain:    command.DomainExecution,
				Action:    command.ActionInvoke,
				Operation: "plan_execution",
				RequestID: "materialize-builtin-" + scenario.slashName + "-" + iterationSuffix,
				Input:     map[string]any{},
			})
			if err != nil || materialized.StructuredContent["outcome"] != string(orchestrationsvc.MutationApplied) ||
				materialized.StructuredContent["execution_id"] != executionID {
				return builtinWorkflowExecutionResult{Snapshot: snapshot}, fmt.Errorf("materialize adaptive %s in the same Execution: result=%#v err=%v", iterationSuffix, materialized, err)
			}
		}
	}
	snapshot, err := repository.GetSnapshot(ctx, executionID)
	return builtinWorkflowExecutionResult{Snapshot: snapshot}, err
}

func invokeBuiltinWorkflowCommand(
	ctx context.Context,
	service *orchestrationsvc.Service,
	actor command.Actor,
	request command.Request,
) (command.Result, error) {
	value, err := serverruntime.HandleExecutionCommand(ctx, service, actor, request)
	if err != nil {
		return command.Result{}, err
	}
	result, ok := value.(command.Result)
	if !ok {
		return command.Result{}, fmt.Errorf("runtime command returned %T", value)
	}
	if result.IsError {
		return result, fmt.Errorf("runtime command returned an error: %#v", result.StructuredContent)
	}
	return result, nil
}

type builtinWorkflowPlanYAML struct {
	NexusPlan          int                           `yaml:"nexus_plan"`
	Operation          string                        `yaml:"operation"`
	Objective          string                        `yaml:"objective"`
	CompletionCriteria []string                      `yaml:"completion_criteria"`
	RevisionReason     string                        `yaml:"revision_reason,omitempty"`
	SupersedeActive    bool                          `yaml:"supersede_active_work,omitempty"`
	Items              []builtinWorkflowPlanItemYAML `yaml:"items"`
}

type builtinWorkflowPlanItemYAML struct {
	LogicalKey         string   `yaml:"logical_key"`
	Kind               string   `yaml:"kind"`
	Subject            string   `yaml:"subject"`
	Objective          string   `yaml:"objective"`
	Deliverable        string   `yaml:"deliverable"`
	AcceptanceCriteria []string `yaml:"acceptance_criteria,omitempty"`
	Required           bool     `yaml:"required"`
	Terminal           bool     `yaml:"terminal,omitempty"`
	ParentLogicalKey   string   `yaml:"parent_logical_key,omitempty"`
	DependsOn          []string `yaml:"depends_on,omitempty"`
}

func builtinWorkflowPlanDocument(
	workflow protocol.WorkGraphWorkflow,
	task string,
) (string, error) {
	return workflowPlanDocument(
		workflow,
		task,
		protocol.ExecutionPlanProposalCreate,
		"",
		false,
	)
}

func workflowPlanDocument(
	workflow protocol.WorkGraphWorkflow,
	task string,
	operation protocol.ExecutionPlanProposalOperation,
	revisionReason string,
	supersedeActive bool,
) (string, error) {
	dependencies := make(map[string][]string)
	for _, dependency := range workflow.Dependencies {
		if dependency.Kind != "" && dependency.Kind != protocol.WorkDependencyHard {
			return "", fmt.Errorf("unsupported built-in dependency kind %q", dependency.Kind)
		}
		dependencies[dependency.LogicalKey] = append(
			dependencies[dependency.LogicalKey],
			dependency.DependsOnLogicalKey,
		)
	}
	document := builtinWorkflowPlanYAML{
		NexusPlan:          1,
		Operation:          string(operation),
		Objective:          workflow.Objective + " Requested task: " + strings.TrimSpace(task),
		CompletionCriteria: append([]string(nil), workflow.CompletionCriteria...),
		RevisionReason:     revisionReason,
		SupersedeActive:    supersedeActive,
		Items:              make([]builtinWorkflowPlanItemYAML, 0, len(workflow.Nodes)),
	}
	for _, node := range workflow.Nodes {
		document.Items = append(document.Items, builtinWorkflowPlanItemYAML{
			LogicalKey:         node.LogicalKey,
			Kind:               string(node.Kind),
			Subject:            node.Subject,
			Objective:          node.Objective + " Requested task: " + strings.TrimSpace(task),
			Deliverable:        node.Deliverable,
			AcceptanceCriteria: append([]string(nil), node.AcceptanceCriteria...),
			Required:           node.Required,
			Terminal:           node.Terminal,
			ParentLogicalKey:   node.ParentLogicalKey,
			DependsOn:          append([]string(nil), dependencies[node.LogicalKey]...),
		})
	}
	payload, err := yaml.Marshal(document)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func firstReadyWorkflowLogicalKey(
	snapshot *protocol.ExecutionSnapshot,
	workflow protocol.WorkGraphWorkflow,
) string {
	ready := make(map[string]struct{}, len(snapshot.ReadyWorkItemIDs))
	for _, workItemID := range snapshot.ReadyWorkItemIDs {
		ready[workItemID] = struct{}{}
	}
	for _, node := range workflow.Nodes {
		for _, workItem := range snapshot.WorkItems {
			if workItem.LogicalKey == node.LogicalKey {
				if _, ok := ready[workItem.ID]; ok {
					return node.LogicalKey
				}
			}
		}
	}
	return ""
}

func workflowNodeByLogicalKey(
	workflow protocol.WorkGraphWorkflow,
	logicalKey string,
) protocol.WorkGraphWorkflowNode {
	for _, node := range workflow.Nodes {
		if node.LogicalKey == logicalKey {
			return node
		}
	}
	return protocol.WorkGraphWorkflowNode{}
}

func evidenceEvaluationIteration(logicalKey string) (int, bool) {
	var iteration int
	if _, err := fmt.Sscanf(logicalKey, "evidence-evaluation-%d", &iteration); err != nil ||
		iteration < 1 || logicalKey != fmt.Sprintf("evidence-evaluation-%d", iteration) {
		return 0, false
	}
	return iteration, true
}

func appendDeepResearchIteration(
	workflow protocol.WorkGraphWorkflow,
	iteration int,
) protocol.WorkGraphWorkflow {
	next := workflow
	next.Nodes = append([]protocol.WorkGraphWorkflowNode(nil), workflow.Nodes...)
	next.Dependencies = make([]protocol.WorkGraphWorkflowDependency, 0, len(workflow.Dependencies)+6)
	previousEvaluation := fmt.Sprintf("evidence-evaluation-%d", iteration-1)
	for _, dependency := range workflow.Dependencies {
		if dependency.LogicalKey == "synthesize" &&
			dependency.DependsOnLogicalKey == previousEvaluation {
			continue
		}
		next.Dependencies = append(next.Dependencies, dependency)
	}

	position := len(next.Nodes)
	iterationLabel := fmt.Sprintf("Iteration %d", iteration)
	gapKey := fmt.Sprintf("gap-diagnosis-%d", iteration)
	strategyKey := fmt.Sprintf("research-strategy-%d", iteration)
	authoritativeKey := fmt.Sprintf("targeted-authoritative-evidence-%d", iteration)
	contrastingKey := fmt.Sprintf("targeted-contrasting-evidence-%d", iteration)
	evaluationKey := fmt.Sprintf("evidence-evaluation-%d", iteration)
	next.Nodes = append(next.Nodes,
		protocol.WorkGraphWorkflowNode{
			LogicalKey: gapKey, Role: protocol.WorkGraphWorkflowNodeCollaboration, Kind: protocol.WorkItemKindVerify,
			Subject:            iterationLabel + " · Diagnose collection failures",
			Objective:          "Explain why the previous evidence gate remained insufficient, distinguishing weak queries, unavailable primary sources, poor source quality, stale evidence, insufficient independence, contradictions, and overly broad decomposition.",
			Deliverable:        "A causal gap diagnosis mapped to the previous iteration's findings.",
			AcceptanceCriteria: []string{"Every targeted gap has an evidence-based failure diagnosis.", "Strategy problems are distinguished from genuinely unavailable evidence."},
			Required:           true, Position: position,
		},
		protocol.WorkGraphWorkflowNode{
			LogicalKey: strategyKey, Role: protocol.WorkGraphWorkflowNodeKey, Kind: protocol.WorkItemKindProduce,
			Subject:            iterationLabel + " · Adjust the search strategy",
			Objective:          "Design materially different queries, sources, methods, time ranges, languages, or subquestion boundaries for the diagnosed gaps, with testable success and stop conditions.",
			Deliverable:        "A targeted strategy tied to every diagnosed gap.",
			AcceptanceCriteria: []string{"The strategy changes the failed approach instead of adding more of the same search.", "Every branch has testable success and stop conditions."},
			Required:           true, Position: position + 1,
		},
		protocol.WorkGraphWorkflowNode{
			LogicalKey: authoritativeKey, Role: protocol.WorkGraphWorkflowNodeCollaboration, Kind: protocol.WorkItemKindProduce,
			Subject:            iterationLabel + " · Target authoritative gaps",
			Objective:          "Execute the adjusted strategy for missing or weak authoritative evidence and record how new sources change coverage and confidence.",
			Deliverable:        "A targeted authoritative evidence packet mapped to the diagnosed gaps.",
			AcceptanceCriteria: []string{"The evidence follows the adjusted strategy and is source-traceable.", "Coverage changes and remaining gaps are explicit."},
			Required:           true, Position: position + 2,
		},
		protocol.WorkGraphWorkflowNode{
			LogicalKey: contrastingKey, Role: protocol.WorkGraphWorkflowNodeCollaboration, Kind: protocol.WorkItemKindProduce,
			Subject:            iterationLabel + " · Target counterevidence and conflicts",
			Objective:          "Execute the adjusted strategy for unresolved counterexamples, conflicting claims, or independence gaps and record whether the leading interpretation changes.",
			Deliverable:        "A targeted counterevidence packet with resolved and unresolved conflicts.",
			AcceptanceCriteria: []string{"The strongest unresolved counterevidence was actively tested.", "Interpretation changes and persistent conflicts are explicit."},
			Required:           true, Position: position + 3,
		},
		protocol.WorkGraphWorkflowNode{
			LogicalKey: evaluationKey, Role: protocol.WorkGraphWorkflowNodeCollaboration, Kind: protocol.WorkItemKindVerify,
			Subject:            iterationLabel + " · Re-evaluate evidence sufficiency",
			Objective:          "Re-evaluate every subquestion cumulatively and produce an explicit sufficient/insufficient verdict. If still insufficient, append another numbered diagnosis-strategy-parallel-collection-evaluation segment to this same Execution and WorkGraph before synthesis.",
			Deliverable:        "A cumulative sufficiency decision with iteration changes, remaining gaps, and the next-iteration decision.",
			AcceptanceCriteria: []string{"Every material gap is closed or remains an explicit next-iteration blocker.", "The verdict records the effect of the adjusted strategy and distinguishes evidence from inference."},
			Required:           true, Position: position + 4,
		},
	)
	next.Dependencies = append(next.Dependencies,
		protocol.WorkGraphWorkflowDependency{LogicalKey: gapKey, DependsOnLogicalKey: previousEvaluation, Kind: protocol.WorkDependencyHard},
		protocol.WorkGraphWorkflowDependency{LogicalKey: strategyKey, DependsOnLogicalKey: gapKey, Kind: protocol.WorkDependencyHard},
		protocol.WorkGraphWorkflowDependency{LogicalKey: authoritativeKey, DependsOnLogicalKey: strategyKey, Kind: protocol.WorkDependencyHard},
		protocol.WorkGraphWorkflowDependency{LogicalKey: contrastingKey, DependsOnLogicalKey: strategyKey, Kind: protocol.WorkDependencyHard},
		protocol.WorkGraphWorkflowDependency{LogicalKey: evaluationKey, DependsOnLogicalKey: authoritativeKey, Kind: protocol.WorkDependencyHard},
		protocol.WorkGraphWorkflowDependency{LogicalKey: evaluationKey, DependsOnLogicalKey: contrastingKey, Kind: protocol.WorkDependencyHard},
		protocol.WorkGraphWorkflowDependency{LogicalKey: "synthesize", DependsOnLogicalKey: evaluationKey, Kind: protocol.WorkDependencyHard},
	)
	return next
}

func otherBuiltinAdaptiveE2EScenarios() []builtinAdaptiveWorkflowScenario {
	return []builtinAdaptiveWorkflowScenario{
		{
			slashName:       "build-ship",
			task:            "Add a health status endpoint with regression tests and release notes",
			gateIteration:   buildQualityGateIteration,
			appendIteration: appendBuildShipIteration,
			parallelKeys: func(iteration int) (string, string, bool) {
				if iteration == 1 {
					return "validate", "review", true
				}
				return fmt.Sprintf("validate-%d", iteration), fmt.Sprintf("review-%d", iteration), true
			},
		},
		{
			slashName:       "decision-brief",
			task:            "Choose between synchronous and queue-backed report generation and deliver a decision brief",
			gateIteration:   decisionChallengeIteration,
			appendIteration: appendDecisionBriefIteration,
			parallelKeys: func(iteration int) (string, string, bool) {
				if iteration == 1 {
					return "evidence", "options", true
				}
				return fmt.Sprintf("evidence-update-%d", iteration), fmt.Sprintf("options-update-%d", iteration), true
			},
		},
		{
			slashName:       "review-improve",
			task:            "Review and improve the first-time workspace onboarding guide with regression-aware verification",
			gateIteration:   reviewImproveVerificationIteration,
			appendIteration: appendReviewImproveIteration,
			parallelKeys: func(iteration int) (string, string, bool) {
				if iteration == 1 {
					return "quality-audit", "experience-audit", true
				}
				return "", "", false
			},
		},
	}
}

func buildQualityGateIteration(logicalKey string) (int, bool) {
	return numberedWorkflowGateIteration(logicalKey, "quality-gate-1", "quality-gate")
}

func decisionChallengeIteration(logicalKey string) (int, bool) {
	return numberedWorkflowGateIteration(logicalKey, "challenge", "challenge")
}

func reviewImproveVerificationIteration(logicalKey string) (int, bool) {
	return numberedWorkflowGateIteration(logicalKey, "verify", "verify")
}

func numberedWorkflowGateIteration(
	logicalKey string,
	initialKey string,
	prefix string,
) (int, bool) {
	if logicalKey == initialKey {
		return 1, true
	}
	var iteration int
	if _, err := fmt.Sscanf(logicalKey, prefix+"-%d", &iteration); err != nil ||
		iteration < 2 || logicalKey != fmt.Sprintf("%s-%d", prefix, iteration) {
		return 0, false
	}
	return iteration, true
}

func appendBuildShipIteration(
	workflow protocol.WorkGraphWorkflow,
	iteration int,
) protocol.WorkGraphWorkflow {
	previousGate := "quality-gate-1"
	if iteration > 2 {
		previousGate = fmt.Sprintf("quality-gate-%d", iteration-1)
	}
	diagnosisKey := fmt.Sprintf("remediation-diagnosis-%d", iteration)
	remediateKey := fmt.Sprintf("remediate-%d", iteration)
	validateKey := fmt.Sprintf("validate-%d", iteration)
	reviewKey := fmt.Sprintf("review-%d", iteration)
	gateKey := fmt.Sprintf("quality-gate-%d", iteration)
	next := cloneWorkflowWithoutDependency(workflow, "deliver", previousGate, 5, 6)
	position := len(next.Nodes)
	label := fmt.Sprintf("Iteration %d", iteration)
	next.Nodes = append(next.Nodes,
		adaptiveWorkflowNode(diagnosisKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindVerify, label+" · Diagnose delivery blockers", "Classify every failed validation or review finding as scope, design, implementation, test, documentation, or external dependency work and choose the narrowest safe remediation boundary.", "A classified blocker diagnosis and remediation route.", position),
		adaptiveWorkflowNode(remediateKey, protocol.WorkGraphWorkflowNodeKey, protocol.WorkItemKindProduce, label+" · Remediate the change", "Apply the targeted scope, design, implementation, test, or documentation corrections selected by the blocker diagnosis.", "A corrected implementation and focused change record.", position+1),
		adaptiveWorkflowNode(validateKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindVerify, label+" · Revalidate behavior", "Rerun affected acceptance and regression checks against the remediated change with reproducible evidence.", "A refreshed validation record.", position+2),
		adaptiveWorkflowNode(reviewKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindReview, label+" · Rereview the change", "Independently rereview the material remediation for correctness, maintainability, safety, and hidden regression risk.", "A refreshed independent review decision.", position+3),
		adaptiveWorkflowNode(gateKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindVerify, label+" · Re-evaluate release readiness", "Evaluate cumulative validation and review evidence and produce an explicit sufficient/insufficient verdict. If insufficient, append another targeted remediation iteration to this same Execution and WorkGraph.", "A cumulative release-readiness verdict and next-iteration decision.", position+4),
	)
	next.Dependencies = append(next.Dependencies,
		hardWorkflowDependency(diagnosisKey, previousGate),
		hardWorkflowDependency(remediateKey, diagnosisKey),
		hardWorkflowDependency(validateKey, remediateKey),
		hardWorkflowDependency(reviewKey, remediateKey),
		hardWorkflowDependency(gateKey, validateKey),
		hardWorkflowDependency(gateKey, reviewKey),
		hardWorkflowDependency("deliver", gateKey),
	)
	return next
}

func appendDecisionBriefIteration(
	workflow protocol.WorkGraphWorkflow,
	iteration int,
) protocol.WorkGraphWorkflow {
	previousChallenge := "challenge"
	if iteration > 2 {
		previousChallenge = fmt.Sprintf("challenge-%d", iteration-1)
	}
	diagnosisKey := fmt.Sprintf("decision-gap-diagnosis-%d", iteration)
	evidenceKey := fmt.Sprintf("evidence-update-%d", iteration)
	optionsKey := fmt.Sprintf("options-update-%d", iteration)
	evaluateKey := fmt.Sprintf("evaluate-%d", iteration)
	challengeKey := fmt.Sprintf("challenge-%d", iteration)
	next := cloneWorkflowWithoutDependency(workflow, "recommend", previousChallenge, 5, 6)
	position := len(next.Nodes)
	label := fmt.Sprintf("Iteration %d", iteration)
	next.Nodes = append(next.Nodes,
		adaptiveWorkflowNode(diagnosisKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindVerify, label+" · Diagnose decision gaps", "Classify the failed challenge as missing evidence, flawed criteria, missing options, or decision-critical uncertainty and select only the necessary next branches.", "A classified decision-gap diagnosis and bounded revision route.", position),
		adaptiveWorkflowNode(evidenceKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindProduce, label+" · Update decision evidence", "Collect only the evidence or bounded experiment results needed to resolve the diagnosed decision-critical uncertainty.", "A targeted evidence update with confidence changes.", position+1),
		adaptiveWorkflowNode(optionsKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindProduce, label+" · Update viable options", "Revise missing or weak option profiles and any affected criteria without reopening settled decision facts.", "Targeted option and criteria updates.", position+2),
		adaptiveWorkflowNode(evaluateKey, protocol.WorkGraphWorkflowNodeKey, protocol.WorkItemKindIntegrate, label+" · Re-evaluate tradeoffs", "Recompare the revised options against the corrected rubric and cumulative evidence.", "An updated option comparison and provisional conclusion.", position+3),
		adaptiveWorkflowNode(challengeKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindReview, label+" · Rechallenge the analysis", "Red-team the updated analysis and produce an explicit sufficient/insufficient verdict. If insufficient, append only the next necessary decision branches to this same Execution and WorkGraph.", "A cumulative challenge verdict and next-iteration decision.", position+4),
	)
	next.Dependencies = append(next.Dependencies,
		hardWorkflowDependency(diagnosisKey, previousChallenge),
		hardWorkflowDependency(evidenceKey, diagnosisKey),
		hardWorkflowDependency(optionsKey, diagnosisKey),
		hardWorkflowDependency(evaluateKey, evidenceKey),
		hardWorkflowDependency(evaluateKey, optionsKey),
		hardWorkflowDependency(challengeKey, evaluateKey),
		hardWorkflowDependency("recommend", challengeKey),
	)
	return next
}

func appendReviewImproveIteration(
	workflow protocol.WorkGraphWorkflow,
	iteration int,
) protocol.WorkGraphWorkflow {
	previousVerification := "verify"
	if iteration > 2 {
		previousVerification = fmt.Sprintf("verify-%d", iteration-1)
	}
	diagnosisKey := fmt.Sprintf("revision-diagnosis-%d", iteration)
	reviseKey := fmt.Sprintf("revise-%d", iteration)
	verifyKey := fmt.Sprintf("verify-%d", iteration)
	next := cloneWorkflowWithoutDependency(workflow, "deliver", previousVerification, 3, 3)
	position := len(next.Nodes)
	label := fmt.Sprintf("Iteration %d", iteration)
	next.Nodes = append(next.Nodes,
		adaptiveWorkflowNode(diagnosisKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindVerify, label+" · Diagnose failed improvement checks", "Classify failures as unresolved findings, regressions, no measurable improvement, or an invalid baseline/rubric, and choose targeted revision, renewed audit, or rebaseline work.", "A classified failure diagnosis and bounded revision route.", position),
		adaptiveWorkflowNode(reviseKey, protocol.WorkGraphWorkflowNodeKey, protocol.WorkItemKindProduce, label+" · Revise the artifact again", "Apply the targeted revision while preserving accepted behavior; update the baseline or affected audit findings only when the diagnosis requires it.", "A targeted revised artifact and change record.", position+1),
		adaptiveWorkflowNode(verifyKey, protocol.WorkGraphWorkflowNodeCollaboration, protocol.WorkItemKindVerify, label+" · Reverify the improvement", "Recheck affected rubric criteria and regression surfaces and produce an explicit sufficient/insufficient verdict. If insufficient, append another targeted revision iteration to this same Execution and WorkGraph.", "A cumulative verification verdict and next-iteration decision.", position+2),
	)
	next.Dependencies = append(next.Dependencies,
		hardWorkflowDependency(diagnosisKey, previousVerification),
		hardWorkflowDependency(reviseKey, diagnosisKey),
		hardWorkflowDependency(verifyKey, reviseKey),
		hardWorkflowDependency("deliver", verifyKey),
	)
	return next
}

func cloneWorkflowWithoutDependency(
	workflow protocol.WorkGraphWorkflow,
	logicalKey string,
	dependsOn string,
	additionalNodes int,
	additionalDependencies int,
) protocol.WorkGraphWorkflow {
	next := workflow
	next.Nodes = make([]protocol.WorkGraphWorkflowNode, len(workflow.Nodes), len(workflow.Nodes)+additionalNodes)
	copy(next.Nodes, workflow.Nodes)
	next.Dependencies = make([]protocol.WorkGraphWorkflowDependency, 0, len(workflow.Dependencies)+additionalDependencies)
	for _, dependency := range workflow.Dependencies {
		if dependency.LogicalKey == logicalKey && dependency.DependsOnLogicalKey == dependsOn {
			continue
		}
		next.Dependencies = append(next.Dependencies, dependency)
	}
	return next
}

func adaptiveWorkflowNode(
	logicalKey string,
	role protocol.WorkGraphWorkflowNodeRole,
	kind protocol.WorkItemKind,
	subject string,
	objective string,
	deliverable string,
	position int,
) protocol.WorkGraphWorkflowNode {
	return protocol.WorkGraphWorkflowNode{
		LogicalKey: logicalKey, Role: role, Kind: kind, Subject: subject,
		Objective: objective, Deliverable: deliverable,
		AcceptanceCriteria: []string{
			"The result is specific, evidence-based, and complete for this iteration boundary.",
			"Remaining blockers and the next action are explicit.",
		},
		Required: true, Position: position,
	}
}

func hardWorkflowDependency(logicalKey string, dependsOn string) protocol.WorkGraphWorkflowDependency {
	return protocol.WorkGraphWorkflowDependency{
		LogicalKey: logicalKey, DependsOnLogicalKey: dependsOn, Kind: protocol.WorkDependencyHard,
	}
}

func markParallelWorkflowIterations(
	snapshot *protocol.ExecutionSnapshot,
	scenario builtinAdaptiveWorkflowScenario,
	observed map[int]bool,
) {
	readyIDs := make(map[string]struct{}, len(snapshot.ReadyWorkItemIDs))
	for _, workItemID := range snapshot.ReadyWorkItemIDs {
		readyIDs[workItemID] = struct{}{}
	}
	readyKeys := make(map[string]bool, len(readyIDs))
	for _, workItem := range snapshot.WorkItems {
		if _, ok := readyIDs[workItem.ID]; ok {
			readyKeys[workItem.LogicalKey] = true
		}
	}
	for iteration := 1; iteration <= scenario.targetIterations; iteration++ {
		leftKey, rightKey, required := scenario.parallelKeys(iteration)
		if required && readyKeys[leftKey] && readyKeys[rightKey] {
			observed[iteration] = true
		}
	}
}

func workflowScope(actor command.Actor) protocol.ExecutionScopeKind {
	if actor.SourceContextType == "room" {
		return protocol.ExecutionScopeRoom
	}
	return protocol.ExecutionScopeDM
}

func assertBuiltinWorkflowRuntimePrompt(
	t *testing.T,
	prompt string,
	workflow protocol.WorkGraphWorkflow,
	scenario builtinAdaptiveWorkflowScenario,
) {
	t.Helper()
	for _, expected := range []string{
		"execution-orchestrator",
		"fresh managed WorkGraph",
		strings.TrimSpace(scenario.task),
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expanded runtime prompt is missing %q:\n%s", expected, prompt)
		}
	}
	for _, node := range workflow.Nodes {
		if !strings.Contains(prompt, node.LogicalKey) {
			t.Fatalf("expanded runtime prompt is missing node %q", node.LogicalKey)
		}
	}
}

func assertCompletedBuiltinWorkflowExecution(
	t *testing.T,
	result builtinWorkflowExecutionResult,
	scope protocol.ExecutionScopeKind,
	sessionKey string,
	roomID string,
	conversationID string,
	scenario builtinAdaptiveWorkflowScenario,
) {
	t.Helper()
	snapshot := result.Snapshot
	workflow := result.Workflow
	if snapshot == nil {
		t.Fatal("completed WorkGraph snapshot is nil")
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusCompleted ||
		snapshot.Execution.ScopeKind != scope ||
		snapshot.Execution.SessionKey != sessionKey ||
		snapshot.Execution.RoomID != roomID ||
		snapshot.Execution.ConversationID != conversationID ||
		!strings.Contains(snapshot.Execution.Objective, scenario.task) {
		t.Fatalf("completed Execution identity/status = %#v", snapshot.Execution)
	}
	if snapshot.Plan == nil || snapshot.Plan.Revision != int64(scenario.targetIterations) ||
		(scenario.targetIterations > 1 && strings.TrimSpace(snapshot.Plan.BasePlanID) == "") {
		t.Fatalf("adaptive %s Plan lineage = %#v, want revision %d in one Execution", scenario.slashName, snapshot.Plan, scenario.targetIterations)
	}
	expectedNodes := len(workflow.Nodes)
	expectedDependencies := len(workflow.Dependencies)
	if len(workflow.Nodes) != expectedNodes || len(workflow.Dependencies) != expectedDependencies ||
		len(snapshot.WorkItems) != expectedNodes || len(snapshot.Dependencies) != expectedDependencies ||
		len(snapshot.Submissions) != expectedNodes || len(snapshot.Acceptances) != expectedNodes {
		t.Fatalf(
			"completed adaptive graph counts: work=%d dependencies=%d submissions=%d acceptances=%d workflow=%d/%d want=%d/%d",
			len(snapshot.WorkItems),
			len(snapshot.Dependencies),
			len(snapshot.Submissions),
			len(snapshot.Acceptances),
			len(workflow.Nodes),
			len(workflow.Dependencies),
			expectedNodes,
			expectedDependencies,
		)
	}
	if len(result.History.Assignments) != expectedNodes ||
		len(result.History.Attempts) != expectedNodes ||
		len(result.History.Submissions) != expectedNodes ||
		len(result.History.Acceptances) != expectedNodes {
		t.Fatalf(
			"cross-revision lifecycle history: assignments=%d attempts=%d submissions=%d acceptances=%d want=%d each",
			len(result.History.Assignments),
			len(result.History.Attempts),
			len(result.History.Submissions),
			len(result.History.Acceptances),
			expectedNodes,
		)
	}
	if result.MaxReady < 2 {
		t.Fatalf("parallel evidence fork was never ready together: max_ready=%d", result.MaxReady)
	}
	for iteration := 1; iteration <= scenario.targetIterations; iteration++ {
		_, _, required := scenario.parallelKeys(iteration)
		if required && !result.ParallelReadyIterations[iteration] {
			t.Fatalf("iteration %d parallel evidence branches were never ready together: observed=%v", iteration, result.ParallelReadyIterations)
		}
	}

	logicalKeyByWorkID := make(map[string]string, len(snapshot.WorkItems))
	for _, workItem := range snapshot.WorkItems {
		logicalKeyByWorkID[workItem.ID] = workItem.LogicalKey
	}
	acceptedByWork := make(map[string]bool, len(result.History.Acceptances))
	for _, acceptance := range result.History.Acceptances {
		if acceptance.Decision == protocol.WorkAcceptanceAccepted {
			acceptedByWork[acceptance.WorkItemID] = true
		}
	}
	verdictByIteration := make(map[int]string, scenario.targetIterations)
	for _, submission := range result.History.Submissions {
		iteration, ok := scenario.gateIteration(logicalKeyByWorkID[submission.WorkItemID])
		if !ok {
			continue
		}
		summary := strings.ToLower(submission.ResultSummary)
		switch {
		case strings.Contains(summary, "insufficient"):
			verdictByIteration[iteration] = "insufficient"
		case strings.Contains(summary, "sufficient"):
			verdictByIteration[iteration] = "sufficient"
		}
	}
	for iteration := 1; iteration <= scenario.targetIterations; iteration++ {
		expectedVerdict := "insufficient"
		if iteration == scenario.targetIterations {
			expectedVerdict = "sufficient"
		}
		if verdictByIteration[iteration] != expectedVerdict {
			t.Fatalf("iteration %d verdict = %q, want %q; all=%v", iteration, verdictByIteration[iteration], expectedVerdict, verdictByIteration)
		}
	}
	for _, node := range workflow.Nodes {
		found := false
		for _, workItem := range snapshot.WorkItems {
			if workItem.LogicalKey != node.LogicalKey {
				continue
			}
			found = true
			if !acceptedByWork[workItem.ID] {
				t.Fatalf("template node %q did not reach accepted state", node.LogicalKey)
			}
		}
		if !found {
			t.Fatalf("template node %q was not materialized", node.LogicalKey)
		}
	}
	t.Logf(
		"completed adaptive %s %s WorkGraph: iterations=%d plan_revision=%d nodes=%d dependencies=%d assignments=%d attempts=%d submissions=%d acceptances=%d verdicts=%v parallel_iterations=%v max_ready=%d",
		scenario.slashName,
		scope,
		scenario.targetIterations,
		snapshot.Plan.Revision,
		len(snapshot.WorkItems),
		len(snapshot.Dependencies),
		len(result.History.Assignments),
		len(result.History.Attempts),
		len(result.History.Submissions),
		len(result.History.Acceptances),
		verdictByIteration,
		result.ParallelReadyIterations,
		result.MaxReady,
	)
}

func assertCopiedSlashPersisted(
	t *testing.T,
	sessions *sessionsvc.Service,
	ctx context.Context,
	sessionKey string,
	copiedCommand string,
) {
	t.Helper()
	messages, err := sessions.GetSessionMessages(ctx, sessionKey)
	if err != nil {
		t.Fatalf("read fresh Session history: %v", err)
	}
	for _, message := range messages {
		if protocol.MessageRole(message) == "user" && messageContainsText(message, copiedCommand) {
			return
		}
	}
	t.Fatalf("fresh Session history did not preserve copied Slash %q: %#v", copiedCommand, messages)
}

func messageContainsText(message protocol.Message, expected string) bool {
	var contains func(any) bool
	contains = func(value any) bool {
		switch typed := value.(type) {
		case string:
			return strings.Contains(typed, expected)
		case []any:
			for _, item := range typed {
				if contains(item) {
					return true
				}
			}
		case []map[string]any:
			for _, item := range typed {
				if contains(item) {
					return true
				}
			}
		case map[string]any:
			for _, item := range typed {
				if contains(item) {
					return true
				}
			}
		}
		return false
	}
	return contains(message["content"])
}

func waitForBuiltinWorkflowRounds(
	t *testing.T,
	manager *runtimectx.Manager,
	scopeSessionKey string,
	runtimeSessionKey string,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(manager.GetRunningRoundIDs(scopeSessionKey)) == 0 &&
			len(manager.GetRunningRoundIDs(runtimeSessionKey)) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf(
		"runtime rounds did not finish: scope=%v runtime=%v",
		manager.GetRunningRoundIDs(scopeSessionKey),
		manager.GetRunningRoundIDs(runtimeSessionKey),
	)
}

type builtinWorkflowE2EOutcome struct {
	prompt    string
	execution builtinWorkflowExecutionResult
	err       error
}

type builtinWorkflowExecutionResult struct {
	Snapshot                *protocol.ExecutionSnapshot
	History                 protocol.ExecutionWorkGraphHistory
	Workflow                protocol.WorkGraphWorkflow
	ParallelReadyIterations map[int]bool
	MaxReady                int
}

type builtinWorkflowE2EClient struct {
	mu        sync.Mutex
	sessionID string
	messages  chan sdkprotocol.ReceivedMessage
	run       func(context.Context, string) builtinWorkflowE2EOutcome
	outcomes  chan builtinWorkflowE2EOutcome
}

func newBuiltinWorkflowE2EClient(
	run func(context.Context, string) builtinWorkflowE2EOutcome,
) *builtinWorkflowE2EClient {
	return &builtinWorkflowE2EClient{
		sessionID: "sdk-session-builtin-workgraph-e2e",
		messages:  make(chan sdkprotocol.ReceivedMessage, 4),
		run:       run,
		outcomes:  make(chan builtinWorkflowE2EOutcome, 1),
	}
}

func (c *builtinWorkflowE2EClient) Connect(context.Context) error { return nil }

func (c *builtinWorkflowE2EClient) Query(ctx context.Context, prompt string) error {
	outcome := c.run(ctx, prompt)
	c.outcomes <- outcome
	text := "Built-in WorkGraph completed."
	if outcome.err != nil {
		text = "Built-in WorkGraph failed: " + outcome.err.Error()
	}
	c.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: c.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:    "assistant-builtin-workgraph-e2e",
			Model: "scripted-e2e-runtime",
			Content: []sdkprotocol.ContentBlock{
				sdkprotocol.TextBlock{Text: text},
			},
		}},
	}
	c.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: c.sessionID,
		UUID:      "result-builtin-workgraph-e2e",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
			Result:     "done",
		},
	}
	return nil
}

func (c *builtinWorkflowE2EClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *builtinWorkflowE2EClient) Interrupt(context.Context) error { return nil }

func (c *builtinWorkflowE2EClient) StopTask(context.Context, string) error { return nil }

func (c *builtinWorkflowE2EClient) SendTaskMessage(context.Context, string, string, string) error {
	return nil
}

func (c *builtinWorkflowE2EClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *builtinWorkflowE2EClient) SetPermissionMode(context.Context, sdkpermission.Mode) error {
	return nil
}

func (c *builtinWorkflowE2EClient) Retire() {}

func (c *builtinWorkflowE2EClient) Disconnect(context.Context) error { return nil }

func (c *builtinWorkflowE2EClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *builtinWorkflowE2EClient) SessionID() string { return c.sessionID }

type builtinWorkflowE2EFactory struct {
	mu      sync.Mutex
	client  *builtinWorkflowE2EClient
	options []agentclient.Options
}

func (f *builtinWorkflowE2EFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options = append(f.options, options)
	return f.client
}
