package server

import (
	agenthandler "github.com/nexus-research-lab/nexus/internal/handler/agent"
	authhandler "github.com/nexus-research-lab/nexus/internal/handler/auth"
	automationhandler "github.com/nexus-research-lab/nexus/internal/handler/automation"
	capabilityhandler "github.com/nexus-research-lab/nexus/internal/handler/capability"
	channelhandler "github.com/nexus-research-lab/nexus/internal/handler/channel"
	connectorhandler "github.com/nexus-research-lab/nexus/internal/handler/connector"
	corehandler "github.com/nexus-research-lab/nexus/internal/handler/core"
	executionhandler "github.com/nexus-research-lab/nexus/internal/handler/execution"
	goalhandler "github.com/nexus-research-lab/nexus/internal/handler/goal"
	launcherhandler "github.com/nexus-research-lab/nexus/internal/handler/launcher"
	loophandler "github.com/nexus-research-lab/nexus/internal/handler/loop"
	projectpermissionhandler "github.com/nexus-research-lab/nexus/internal/handler/projectpermission"
	providerhandler "github.com/nexus-research-lab/nexus/internal/handler/provider"
	roomhandler "github.com/nexus-research-lab/nexus/internal/handler/room"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	skillhandler "github.com/nexus-research-lab/nexus/internal/handler/skill"
	subscriptionhandler "github.com/nexus-research-lab/nexus/internal/handler/subscription"
	handlerwebsocket "github.com/nexus-research-lab/nexus/internal/handler/websocket"
	workspacehandler "github.com/nexus-research-lab/nexus/internal/handler/workspace"
)

type handlerSet struct {
	auth         *authhandler.Handlers
	core         *corehandler.Handlers
	agent        *agenthandler.Handlers
	room         *roomhandler.Handlers
	capability   *capabilityhandler.Handlers
	skill        *skillhandler.Handlers
	connector    *connectorhandler.Handlers
	channel      *channelhandler.Handlers
	automation   *automationhandler.Handlers
	provider     *providerhandler.Handlers
	subscription *subscriptionhandler.Handlers
	goal         *goalhandler.Handlers
	execution    *executionhandler.Handlers
	launcher     *launcherhandler.Handlers
	loop         *loophandler.Handlers
	workspace    *workspacehandler.Handlers
	project      *projectpermissionhandler.Handlers
	websocket    *handlerwebsocket.Handler
}

func newHandlerSet(
	api *handlershared.API,
	services *AppServices,
	websocketHandler *handlerwebsocket.Handler,
) handlerSet {
	core := corehandler.New(
		api,
		services.Core.Agent,
		services.Provider,
		services.Preferences,
	)
	core.SetRuntimeManager(services.Runtime)
	return handlerSet{
		auth: authhandler.New(api, services.Auth, services.Usage, services.Subscription),
		core: core,
		agent: agenthandler.New(
			api,
			services.Core.Agent,
			services.Core.Session,
			services.Runtime,
			services.RoomRealtime,
			services.Communication,
			websocketHandler.BroadcastDirectoryChanged,
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
			websocketHandler.BroadcastDirectoryChanged,
		),
		capability:   capabilityhandler.New(api, services.Skills, services.Connectors, services.Automation, services.ChannelControl),
		skill:        skillhandler.New(api, services.Skills),
		connector:    connectorhandler.New(api, services.Connectors),
		channel:      channelhandler.New(api, services.Ingress, services.ChannelControl),
		automation:   automationhandler.New(api, services.Automation),
		provider:     providerhandler.New(api, services.Provider, services.Preferences),
		subscription: subscriptionhandler.New(api, services.Subscription),
		goal:         goalhandler.New(api, services.Goal),
		execution:    executionhandler.New(api, services.Orchestration),
		launcher:     launcherhandler.New(api, services.Launcher),
		loop:         loophandler.New(api, services.Loops),
		workspace:    workspacehandler.New(api, services.Workspace),
		project:      projectpermissionhandler.New(api, services.ProjectPermission),
		websocket:    websocketHandler,
	}
}
