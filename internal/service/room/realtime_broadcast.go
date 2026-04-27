package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) withBroadcastTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, roomBroadcastTimeout)
}

func (s *RealtimeService) broadcastSharedEventWithTimeout(
	ctx context.Context,
	sessionKey string,
	roomID string,
	event protocol.EventMessage,
) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	s.broadcastSharedEvent(broadcastCtx, sessionKey, roomID, event)
}

func (s *RealtimeService) broadcastSessionStatus(ctx context.Context, sessionKey string) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	if errs := s.permission.BroadcastSessionStatus(
		broadcastCtx,
		sessionKey,
		s.runtime.GetRunningRoundIDs(sessionKey),
	); len(errs) > 0 {
		s.loggerFor(broadcastCtx).Warn("广播 Room session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

func (s *RealtimeService) broadcastSharedEvent(ctx context.Context, sessionKey string, roomID string, event protocol.EventMessage) {
	if s.broadcaster != nil && strings.TrimSpace(roomID) != "" {
		s.broadcaster.Broadcast(ctx, roomID, event)
		return
	}
	s.permission.BroadcastEvent(ctx, sessionKey, event)
}
