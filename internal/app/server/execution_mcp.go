// INPUT: 当前 runtime 的权威 Agent/scope/session/round/permission identity、统一动态 Responsibility authority 与 Execution 服务。
// OUTPUT: 绑定当前 round 权限角色，并在每次 Actor 投影时原子读取 Goal/Execution/Work/Review identity 的 nexus_execution MCP overlay。
// POS: DM/Room runtime 到 Execution Orchestration 模型工具的不可伪造应用装配边界。
package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	executionmcp "github.com/nexus-research-lab/nexus/internal/mcp/execution"
	executionmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionSnapshotReader interface {
	GetCurrent(context.Context, orchestrationsvc.ActorContext) (*protocol.ExecutionSnapshot, error)
	GetSnapshot(context.Context, orchestrationsvc.ActorContext, string) (*protocol.ExecutionSnapshot, error)
}

func newExecutionMCPBuilder(
	svc executionmcpcontract.Service,
) runtimectx.ExecutionMCPServerBuilder {
	return func(
		ctx context.Context,
		runtimeContext runtimectx.ExecutionToolContext,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil {
			return nil
		}
		serverContext, ok := resolveExecutionMCPServerContext(ctx, svc, runtimeContext)
		if !ok {
			return nil
		}
		return map[string]sdkmcp.ServerConfig{
			executionmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     executionmcpcontract.ServerName,
				Instance: executionmcp.NewServer(svc, serverContext),
			},
		}
	}
}

func combinedExecutionMCPBuilder(
	builders ...runtimectx.ExecutionMCPServerBuilder,
) runtimectx.ExecutionMCPServerBuilder {
	return func(
		ctx context.Context,
		runtimeContext runtimectx.ExecutionToolContext,
	) map[string]sdkmcp.ServerConfig {
		merged := map[string]sdkmcp.ServerConfig{}
		for _, builder := range builders {
			if builder == nil {
				continue
			}
			for name, server := range builder(ctx, runtimeContext) {
				merged[name] = server
			}
		}
		return merged
	}
}

func resolveExecutionMCPServerContext(
	ctx context.Context,
	reader executionSnapshotReader,
	runtimeContext runtimectx.ExecutionToolContext,
) (executionmcpcontract.ServerContext, bool) {
	agentValue := runtimeContext.Agent
	if reader == nil || agentValue == nil {
		return executionmcpcontract.ServerContext{}, false
	}

	ownerUserID := strings.TrimSpace(agentValue.OwnerUserID)
	agentID := strings.TrimSpace(agentValue.AgentID)
	scopeSessionKey := strings.TrimSpace(runtimeContext.ScopeSessionKey)
	if ownerUserID == "" || agentID == "" || scopeSessionKey == "" {
		return executionmcpcontract.ServerContext{}, false
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
		return executionmcpcontract.ServerContext{}, false
	}

	serverContext := executionmcpcontract.ServerContext{
		OwnerUserID:             ownerUserID,
		AgentID:                 agentID,
		Role:                    role,
		ActorKind:               protocol.ExecutionActorAgent,
		ScopeKind:               scopeKind,
		ScopeSessionKey:         scopeSessionKey,
		RuntimeSessionKey:       strings.TrimSpace(runtimeContext.RuntimeSessionKey),
		ExecutionID:             strings.TrimSpace(runtimeContext.ExecutionID),
		WorkBinding:             cloneExecutionMCPWorkBinding(runtimeContext.WorkBinding),
		WorkBindingState:        runtimeContext.WorkBindingState,
		ReviewBinding:           cloneExecutionMCPReviewBinding(runtimeContext.ReviewBinding),
		GoalAuthority:           runtimeContext.GoalAuthority,
		ResponsibilityAuthority: runtimeContext.ResponsibilityAuthority,
		RootRoundID:             strings.TrimSpace(runtimeContext.RootRoundID),
		RuntimeRoundID:          runtimeRoundID,
		AgentRoundID:            strings.TrimSpace(runtimeContext.AgentRoundID),
		RoomID:                  strings.TrimSpace(runtimeContext.RoomID),
		ConversationID:          strings.TrimSpace(runtimeContext.ConversationID),
		PlanMode: runtimepermission.NormalizeMode(runtimeContext.PermissionMode) ==
			sdkpermission.ModePlan,
	}
	if scopeKind == protocol.ExecutionScopeRoom {
		if serverContext.RoomID == "" || serverContext.ConversationID == "" {
			return executionmcpcontract.ServerContext{}, false
		}
		lookupActor := serverContext.Actor()
		lookupActor.Role = ""
		var snapshot *protocol.ExecutionSnapshot
		var err error
		if lookupActor.ExecutionID != "" {
			snapshot, err = reader.GetSnapshot(ctx, lookupActor, lookupActor.ExecutionID)
		} else {
			snapshot, err = reader.GetCurrent(ctx, lookupActor)
		}
		if err != nil {
			// 快照故障不得改变 Session 工具表；服务层仍会在
			// 每次执行时重读真相源并拒绝未绑定权限。
			serverContext.Role = orchestrationsvc.ExecutionActorMember
			return serverContext, true
		}
		switch {
		case snapshot != nil &&
			strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) == agentID:
			serverContext.Role = orchestrationsvc.ExecutionActorCoordinator
		case snapshot != nil:
			serverContext.Role = orchestrationsvc.ExecutionActorMember
		case strings.TrimSpace(runtimeContext.CoordinatorAgentID) == agentID:
			serverContext.Role = orchestrationsvc.ExecutionActorCoordinator
		default:
			serverContext.Role = orchestrationsvc.ExecutionActorMember
		}
		// 裸 @ / 用户定向消息只创建 conversation round。未绑定 member
		// 保留稳定工具面，由逐轮 Responsibility authority 在服务层拒绝越权。
	}
	return serverContext, true
}

func cloneExecutionMCPReviewBinding(
	binding *protocol.ExecutionReviewBinding,
) *protocol.ExecutionReviewBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}

func cloneExecutionMCPWorkBinding(
	binding *protocol.ExecutionWorkBinding,
) *protocol.ExecutionWorkBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}
