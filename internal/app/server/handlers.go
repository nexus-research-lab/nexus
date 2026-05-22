package server

import (
	"github.com/nexus-research-lab/nexus/internal/config"
	agenthandler "github.com/nexus-research-lab/nexus/internal/handler/agent"
	authhandler "github.com/nexus-research-lab/nexus/internal/handler/auth"
	automationhandler "github.com/nexus-research-lab/nexus/internal/handler/automation"
	capabilityhandler "github.com/nexus-research-lab/nexus/internal/handler/capability"
	channelhandler "github.com/nexus-research-lab/nexus/internal/handler/channel"
	connectorhandler "github.com/nexus-research-lab/nexus/internal/handler/connector"
	corehandler "github.com/nexus-research-lab/nexus/internal/handler/core"
	goalhandler "github.com/nexus-research-lab/nexus/internal/handler/goal"
	launcherhandler "github.com/nexus-research-lab/nexus/internal/handler/launcher"
	memoryhandler "github.com/nexus-research-lab/nexus/internal/handler/memory"
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
	goal       *goalhandler.Handlers
	launcher   *launcherhandler.Handlers
	memory     *memoryhandler.Handlers
	workspace  *workspacehandler.Handlers
	websocket  *handlerwebsocket.Handler
}

func newHandlerSet(
	api *handlershared.API,
	services *AppServices,
	websocketHandler *handlerwebsocket.Handler,
	internalControlToken string,
	cfg config.Config,
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
			internalControlToken,
		),
		capability: capabilityhandler.New(api, services.Skills, services.Connectors, services.Automation, services.ChannelControl),
		skill:      skillhandler.New(api, services.Skills),
		connector:  connectorhandler.New(api, services.Connectors),
		channel:    channelhandler.New(api, services.Ingress, services.ChannelControl),
		automation: automationhandler.New(api, services.Automation),
		goal:       goalhandler.New(api, services.Goal),
		launcher:   launcherhandler.New(api, services.Launcher),
		memory:     memoryhandler.New(api, cfg, services.Core.Agent),
		workspace:  workspacehandler.New(api, services.Workspace),
		websocket:  websocketHandler,
	}
}
