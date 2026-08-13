// INPUT: 已装配的核心/automation/configuration 服务与 websocket broadcaster。
// OUTPUT: Session、conversation 标题、定时任务、Agent 与 Room 配置变更的实时失效通知绑定。
// POS: 各服务领域事件到前端实时刷新投影的统一应用装配点。
package server

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
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
	if services.Title != nil {
		services.Title.SetRoomResyncBroadcaster(broadcaster)
	}
	if services.Automation != nil {
		services.Automation.SetTaskEventNotifier(automationsvc.TaskEventNotifierFunc(
			func(ctx context.Context, event automationdomain.ScheduledTaskEvent) {
				broadcaster.BroadcastScheduledTaskChanged(ctx, event)
			},
		))
	}
	if services.Configuration != nil {
		services.Configuration.SetNotifier(configurationRealtimeNotifier{broadcaster: broadcaster})
	}
}
