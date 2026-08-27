// INPUT: 当前 runtime 的 Agent/scope/session/round/permission identity 与统一动态 Responsibility authority。
// OUTPUT: 每次 command 原子读取 Goal/Execution/Work/Review identity 的权威 Execution context。
// POS: DM/Room runtime 到 Execution command 的不可伪造应用装配边界。
package execution

import (
	"context"
	"strings"

	executioncontract "github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// SnapshotReader 读取当前或指定 Execution 快照。
type SnapshotReader interface {
	ReadCurrent(context.Context, orchestrationsvc.ActorContext) (*protocol.ExecutionSnapshot, error)
	ReadSnapshot(context.Context, orchestrationsvc.ActorContext, string) (*protocol.ExecutionSnapshot, error)
}

func ResolveCommandContext(
	ctx context.Context,
	reader SnapshotReader,
	runtimeContext runtimectx.RuntimeCommandContext,
) (executioncontract.Context, bool) {
	agentValue := runtimeContext.Agent
	if reader == nil || agentValue == nil {
		return executioncontract.Context{}, false
	}
	ownerUserID := strings.TrimSpace(agentValue.OwnerUserID)
	agentID := strings.TrimSpace(agentValue.AgentID)
	scopeSessionKey := strings.TrimSpace(runtimeContext.ScopeSessionKey)
	if ownerUserID == "" || agentID == "" || scopeSessionKey == "" {
		return executioncontract.Context{}, false
	}
	sourceContextType := strings.TrimSpace(runtimeContext.SourceContextType)
	scopeKind := protocol.ExecutionScopeDM
	role := orchestrationsvc.ExecutionActorCoordinator
	runtimeRoundID := strings.TrimSpace(runtimeContext.RootRoundID)
	switch sourceContextType {
	case "agent":
	case "room":
		scopeKind = protocol.ExecutionScopeRoom
		runtimeRoundID = strings.TrimSpace(runtimeContext.AgentRoundID)
	default:
		return executioncontract.Context{}, false
	}
	sctx := executioncontract.Context{
		OwnerUserID: ownerUserID, AgentID: agentID, Role: role,
		ActorKind: protocol.ExecutionActorAgent, ScopeKind: scopeKind,
		ScopeSessionKey:         scopeSessionKey,
		RuntimeSessionKey:       strings.TrimSpace(runtimeContext.RuntimeSessionKey),
		ExecutionID:             strings.TrimSpace(runtimeContext.ExecutionID),
		WorkBinding:             cloneWorkBinding(runtimeContext.WorkBinding),
		WorkBindingState:        runtimeContext.WorkBindingState,
		ReviewBinding:           cloneReviewBinding(runtimeContext.ReviewBinding),
		GoalAuthority:           runtimeContext.GoalAuthority,
		ResponsibilityAuthority: runtimeContext.ResponsibilityAuthority,
		RootRoundID:             strings.TrimSpace(runtimeContext.RootRoundID),
		RuntimeRoundID:          runtimeRoundID,
		AgentRoundID:            strings.TrimSpace(runtimeContext.AgentRoundID),
		RoomID:                  strings.TrimSpace(runtimeContext.RoomID),
		ConversationID:          strings.TrimSpace(runtimeContext.ConversationID),
		PlanMode:                runtimepermission.NormalizeMode(runtimeContext.PermissionMode) == sdkpermission.ModePlan,
	}
	if scopeKind == protocol.ExecutionScopeRoom {
		if sctx.RoomID == "" || sctx.ConversationID == "" {
			return executioncontract.Context{}, false
		}
		lookupActor := sctx.Actor()
		lookupActor.Role = ""
		var snapshot *protocol.ExecutionSnapshot
		var err error
		if lookupActor.ExecutionID != "" {
			snapshot, err = reader.ReadSnapshot(ctx, lookupActor, lookupActor.ExecutionID)
		} else {
			snapshot, err = reader.ReadCurrent(ctx, lookupActor)
		}
		if err != nil {
			sctx.Role = orchestrationsvc.ExecutionActorMember
			return sctx, true
		}
		switch {
		case snapshot != nil && strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) == agentID:
			sctx.Role = orchestrationsvc.ExecutionActorCoordinator
		case snapshot != nil:
			sctx.Role = orchestrationsvc.ExecutionActorMember
		case strings.TrimSpace(runtimeContext.CoordinatorAgentID) == agentID:
			sctx.Role = orchestrationsvc.ExecutionActorCoordinator
		default:
			sctx.Role = orchestrationsvc.ExecutionActorMember
		}
	}
	return sctx, true
}

func cloneReviewBinding(binding *protocol.ExecutionReviewBinding) *protocol.ExecutionReviewBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}

func cloneWorkBinding(binding *protocol.ExecutionWorkBinding) *protocol.ExecutionWorkBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}
