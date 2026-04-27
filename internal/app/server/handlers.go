package server

import (
	agenthandler "github.com/nexus-research-lab/nexus/internal/handler/agent"
	authhandler "github.com/nexus-research-lab/nexus/internal/handler/auth"
	automationhandler "github.com/nexus-research-lab/nexus/internal/handler/automation"
	capabilityhandler "github.com/nexus-research-lab/nexus/internal/handler/capability"
	channelhandler "github.com/nexus-research-lab/nexus/internal/handler/channel"
	connectorhandler "github.com/nexus-research-lab/nexus/internal/handler/connector"
	corehandler "github.com/nexus-research-lab/nexus/internal/handler/core"
	launcherhandler "github.com/nexus-research-lab/nexus/internal/handler/launcher"
	roomhandler "github.com/nexus-research-lab/nexus/internal/handler/room"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	skillhandler "github.com/nexus-research-lab/nexus/internal/handler/skill"
	handlerwebsocket "github.com/nexus-research-lab/nexus/internal/handler/websocket"
	workspacehandler "github.com/nexus-research-lab/nexus/internal/handler/workspace"
)

type handlerSet struct {
	auth       *authhandler.Handlers
	core       *corehandler.Handlers
	agent      *agenthandler.Handlers
	room       *roomhandler.Handlers
	capability *capabilityhandler.Handlers
	skill      *skillhandler.Handlers
	connector  *connectorhandler.Handlers
	channel    *channelhandler.Handlers
	automation *automationhandler.Handlers
	launcher   *launcherhandler.Handlers
	workspace  *workspacehandler.Handlers
	websocket  *handlerwebsocket.Handler
}

func newHandlerSet(
	api *handlershared.API,
	services *AppServices,
	websocketHandler *handlerwebsocket.Handler,
) handlerSet {
	return handlerSet{
		auth: authhandler.New(api, services.Auth, services.Usage),
		core: corehandler.New(
			api,
			services.Core.Agent,
			services.Provider,
			services.Preferences,
		),
		agent: agenthandler.New(
			api,
			services.Core.Agent,
			services.Core.Session,
			services.Runtime,
			services.RoomRealtime,
			services.Preferences,
		),
		room: roomhandler.New(
			api,
			services.Core.Room,
			services.RoomRealtime,
			services.Core.Session,
			websocketHandler.BroadcastRoomEvent,
			websocketHandler.BroadcastRoomResyncRequired,
			websocketHandler.RemoveRoom,
		),
		capability: capabilityhandler.New(api, services.Skills, services.Connectors, services.Automation),
		skill:      skillhandler.New(api, services.Skills),
		connector:  connectorhandler.New(api, services.Connectors),
		channel:    channelhandler.New(api, services.Ingress),
		automation: automationhandler.New(api, services.Automation),
		launcher:   launcherhandler.New(api, services.Launcher),
		workspace:  workspacehandler.New(api, services.Workspace),
		websocket:  websocketHandler,
	}
}
