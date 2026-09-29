package runtime

import (
	"context"
	"testing"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	executioncontract "github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// Embedding the service interface is enough for contract generation: the test
// never invokes a business mutation, while ResolveCommandContext still sees a
// non-nil complete Execution service.
type executionContractServiceStub struct {
	executioncontract.Service
}

func TestDMGoalContinuationBuildsFullExecutionContract(t *testing.T) {
	sessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"conversation-1",
		"",
	)
	goalAuthority := runtimectx.NewGoalAuthorityState("goal-1", 2, "execution-1")
	responsibility := runtimectx.NewResponsibilityAuthorityState(goalAuthority, "execution-1", nil, nil)
	round := nexusRoundForExecutionAuthority(
		sessionKey,
		"round-1",
		&protocol.Agent{OwnerUserID: "owner-1", AgentID: "agent-1"},
		goalAuthority,
		responsibility,
	)
	actor := command.Actor{
		OwnerUserID: "owner-1", AgentID: "agent-1", SessionKey: sessionKey,
		RoundID: "round-1", LeaseSessionKey: sessionKey, LeaseRoundID: "round-1",
		SourceContextType: runtimectx.SourceContextGoalContinuation,
		SourceContextID:   "agent-1", Round: round,
		GoalMutationAuthority: goalAuthority, GoalResponsibilityState: responsibility,
	}
	if !trustedCommandActor(context.Background(), actor.Round.CommandContext.Agent, actor) {
		t.Fatal("exact DM Goal continuation actor was rejected by runtime admission")
	}

	value, err := HandleExecutionCommand(
		context.Background(),
		&executionContractServiceStub{},
		actor,
		command.Request{Action: command.ActionContract, Operation: "assign_work"},
	)
	if err != nil {
		t.Fatal(err)
	}
	contract, ok := value.(command.Contract)
	if !ok {
		t.Fatalf("contract type = %T, value=%+v", value, value)
	}
	if len(contract.Operations) != 1 || contract.Operations[0].Name != "assign_work" {
		t.Fatalf("assign_work contract = %+v", contract)
	}
	for _, required := range []string{"get_execution", "assign_work", "submit_work", "review_work"} {
		value, err := HandleExecutionCommand(
			context.Background(), &executionContractServiceStub{}, actor,
			command.Request{Action: command.ActionContract, Operation: required},
		)
		if err != nil {
			t.Fatalf("contract %s: %v", required, err)
		}
		operationContract, ok := value.(command.Contract)
		if !ok || len(operationContract.Operations) != 1 || operationContract.Operations[0].Name != required {
			t.Fatalf("contract %s = %#v", required, value)
		}
	}
}

func TestDMGoalContinuationDoesNotFallBackToAuthoringOnIdentityFailure(t *testing.T) {
	sessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"conversation-1",
		"",
	)
	goalAuthority := runtimectx.NewGoalAuthorityState("goal-1", 2, "execution-1")
	responsibility := runtimectx.NewResponsibilityAuthorityState(goalAuthority, "execution-1", nil, nil)
	round := nexusRoundForExecutionAuthority(sessionKey, "round-1", &protocol.Agent{OwnerUserID: "owner-1", AgentID: "agent-1"}, goalAuthority, responsibility)
	round.CommandContext.ExecutionID = "execution-other"
	actor := command.Actor{
		OwnerUserID: "owner-1", AgentID: "agent-1", SessionKey: sessionKey,
		RoundID: "round-1", LeaseSessionKey: sessionKey, LeaseRoundID: "round-1",
		SourceContextType: runtimectx.SourceContextGoalContinuation,
		SourceContextID:   "agent-1", Round: round,
		GoalMutationAuthority: goalAuthority, GoalResponsibilityState: responsibility,
	}
	if _, err := HandleExecutionCommand(context.Background(), &executionContractServiceStub{}, actor, command.Request{
		Action: command.ActionContract, Operation: "assign_work",
	}); err == nil {
		t.Fatal("mismatched continuation identity must not fall back to authoring")
	}
}

func nexusRoundForExecutionAuthority(
	sessionKey string,
	roundID string,
	agent *protocol.Agent,
	goal *runtimectx.GoalAuthorityState,
	responsibility *runtimectx.ResponsibilityAuthorityState,
) nexusmcp.RoundContext {
	return nexusmcp.RoundContext{
		SessionKey: sessionKey, RoundID: roundID,
		SourceContextType: runtimectx.SourceContextGoalContinuation,
		SourceContextID:   agent.AgentID,
		CommandReceipts:   nexusmcp.NewCommandReceiptState(),
		CommandContext: runtimectx.RuntimeCommandContext{
			Agent: agent, ScopeSessionKey: sessionKey, RuntimeSessionKey: sessionKey,
			ExecutionID: "execution-1", RootRoundID: roundID,
			SourceContextType: runtimectx.SourceContextGoalContinuation,
			SourceContextID:   agent.AgentID,
			GoalAuthority:     goal, ResponsibilityAuthority: responsibility,
			GoalContinuationAuthority: &runtimectx.GoalContinuationAuthority{
				OwnerUserID: agent.OwnerUserID, AgentID: agent.AgentID, ScopeSessionKey: sessionKey,
				GoalID: "goal-1", ObjectiveRevision: 2, ExecutionID: "execution-1", RootRoundID: roundID,
			},
		},
	}
}
