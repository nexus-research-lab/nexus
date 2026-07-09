package server

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/handler/websocket"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

func configureRealtimeInvalidation(
	services *AppServices,
	broadcaster *websocket.Handler,
) {
	if services == nil || broadcaster == nil {
		return
	}
	if services.Core != nil && services.Core.Session != nil {
		services.Core.Session.SetDirectoryNotifier(sessionsvc.DirectoryNotifierFunc(
			func(ctx context.Context, reason string, session protocol.Session) {
				broadcaster.BroadcastDirectoryChanged(ctx, reason, map[string]any{
					"agent_id":    strings.TrimSpace(session.AgentID),
					"session_key": strings.TrimSpace(session.SessionKey),
				})
			},
		))
	}
	if services.Automation != nil {
		services.Automation.SetTaskEventNotifier(automationsvc.TaskEventNotifierFunc(
			func(ctx context.Context, event automationdomain.CronTaskEvent) {
				broadcaster.BroadcastScheduledTaskChanged(ctx, event)
			},
		))
	}
}
