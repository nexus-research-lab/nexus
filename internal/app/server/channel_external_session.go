package server

import (
	"context"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

type externalSessionRoomResyncBroadcaster interface {
	BroadcastRoomResyncRequired(context.Context, string, string, string)
}

func configureExternalSessionNotifier(
	services *AppServices,
	broadcaster externalSessionRoomResyncBroadcaster,
	logger *slog.Logger,
) {
	if services == nil || services.Ingress == nil || services.Core == nil || services.Core.Room == nil || broadcaster == nil {
		return
	}
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	services.Ingress.SetExternalSessionNotifier(channels.ExternalSessionNotifierFunc(
		func(ctx context.Context, agentID string, sessionKey string) {
			normalizedAgentID := strings.TrimSpace(agentID)
			if normalizedAgentID == "" {
				return
			}
			contextValue, err := services.Core.Room.EnsureDirectRoom(ctx, normalizedAgentID)
			if err != nil {
				logger.Warn("外部 IM session 更新后刷新 DM room 失败",
					"agent_id", normalizedAgentID,
					"session_key", strings.TrimSpace(sessionKey),
					"err", err,
				)
				return
			}
			broadcaster.BroadcastRoomResyncRequired(
				ctx,
				contextValue.Room.ID,
				contextValue.Conversation.ID,
				"external_session_updated",
			)
		},
	))
}
