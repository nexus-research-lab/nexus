package server

import (
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	handlerwebsocket "github.com/nexus-research-lab/nexus/internal/handler/websocket"
)

func newWebSocketHandler(
	api *handlershared.API,
	services *AppServices,
) *handlerwebsocket.Handler {
	return handlerwebsocket.NewHandler(
		api,
		services.Core.Room,
		services.RoomRealtime,
		services.DM,
		services.Permission,
		services.Runtime,
		services.Channels,
		services.Workspace,
		newRuntimeSnapshotProvider(services),
	)
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
