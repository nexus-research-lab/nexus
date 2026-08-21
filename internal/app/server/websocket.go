// INPUT: AppServices 单例、WebSocket transport 依赖与 runtime kind resolver。
// OUTPUT: 已注入 Room/Goal broadcaster、owner Workflow command provider 及 ExecutionInvalidationSink 的 WebSocket handler。
// POS: service 到 handler 的组合根；业务 service 不反向依赖 WebSocket。
package server

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	handlerwebsocket "github.com/nexus-research-lab/nexus/internal/handler/websocket"
	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func newWebSocketHandler(
	api *handlershared.API,
	services *AppServices,
	cfg config.Config,
) *handlerwebsocket.Handler {
	handler := handlerwebsocket.NewHandler(
		api,
		services.Core.Room,
		services.RoomRealtime,
		services.DM,
		services.Goal,
		services.Permission,
		services.Runtime,
		services.Core.Session,
		services.Channels,
		services.Workspace,
		newRuntimeSnapshotProvider(services),
		cfg.AllowedWebSocketOrigins,
		services.SlashRegistry,
		services.SlashCatalog,
		newRuntimeKindResolver(services),
	)
	if services != nil && services.Orchestration != nil {
		services.Orchestration.SetExecutionInvalidationSink(handler)
	}
	if services != nil && services.WorkGraphWorkflow != nil {
		handler.SetWorkGraphWorkflowProvider(services.WorkGraphWorkflow)
	}
	if services != nil && services.Automation != nil {
		handler.SetAutomationPermissionService(services.Automation)
	}
	return handler
}

func newRuntimeKindResolver(
	services *AppServices,
) func(context.Context, string) (agentclient.RuntimeKind, error) {
	return func(ctx context.Context, agentID string) (agentclient.RuntimeKind, error) {
		if services == nil || services.Core == nil || services.Core.Agent == nil {
			return agentclient.RuntimeNXS, nil
		}
		agent, err := services.Core.Agent.GetAgent(ctx, agentID)
		if err != nil {
			return "", err
		}
		selection, err := runtimeselectionsvc.NewService(services.Preferences).Resolve(
			ctx,
			runtimeselectionsvc.Request{Agent: agent},
		)
		if err != nil {
			return "", err
		}
		return agentclient.RuntimeKind(
			runtimeprovider.NormalizeRuntimeKind(selection.RuntimeKind),
		), nil
	}
}

func newRuntimeSnapshotProvider(services *AppServices) func(string) handlerwebsocket.RuntimeSnapshot {
	return func(agentID string) handlerwebsocket.RuntimeSnapshot {
		runningCount := services.Runtime.CountRunningRounds(agentID)
		if services.RoomRealtime != nil {
			runningCount += services.RoomRealtime.CountRunningTasks(agentID)
		}
		status := "idle"
		if runningCount > 0 {
			status = "running"
		}
		return handlerwebsocket.RuntimeSnapshot{
			AgentID:          agentID,
			RunningTaskCount: runningCount,
			Status:           status,
		}
	}
}
