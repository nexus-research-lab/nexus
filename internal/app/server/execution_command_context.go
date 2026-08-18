// INPUT: 当前 runtime 的 Agent/scope/session/round/permission identity 与统一动态 Responsibility authority。
// OUTPUT: 每次 command 原子读取 Goal/Execution/Work/Review identity 的权威 Execution context。
// POS: DM/Room runtime 到 Execution command 的不可伪造应用装配边界。
package server

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	executioncontract "github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/contract"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type executionSnapshotReader interface {
	GetCurrent(context.Context, orchestrationsvc.ActorContext) (*protocol.ExecutionSnapshot, error)
	GetSnapshot(context.Context, orchestrationsvc.ActorContext, string) (*protocol.ExecutionSnapshot, error)
}

func resolveExecutionCommandContext(
	ctx context.Context,
	reader executionSnapshotReader,
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
		WorkBinding:             cloneExecutionCommandWorkBinding(runtimeContext.WorkBinding),
		WorkBindingState:        runtimeContext.WorkBindingState,
		ReviewBinding:           cloneExecutionCommandReviewBinding(runtimeContext.ReviewBinding),
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
			snapshot, err = reader.GetSnapshot(ctx, lookupActor, lookupActor.ExecutionID)
		} else {
			snapshot, err = reader.GetCurrent(ctx, lookupActor)
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

func cloneExecutionCommandReviewBinding(binding *protocol.ExecutionReviewBinding) *protocol.ExecutionReviewBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}

func cloneExecutionCommandWorkBinding(binding *protocol.ExecutionWorkBinding) *protocol.ExecutionWorkBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}
