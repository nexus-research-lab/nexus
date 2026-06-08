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
		// RoomBroadcaster 面向房间 WebSocket，不经过 permission.Context；后台自动化需要这条内部镜像。
		s.notifyRoomEventObserver(ctx, sessionKey, event)
		return
	}
	s.permission.BroadcastEvent(ctx, sessionKey, event)
}

func (s *RealtimeService) notifyRoomEventObserver(ctx context.Context, sessionKey string, event protocol.EventMessage) {
	if s == nil {
		return
	}
	roundID := eventRoundID(event)
	if strings.TrimSpace(roundID) == "" {
		return
	}
	s.mu.Lock()
	roundValue := s.activeRounds[roomActiveRoundKey(sessionKey, roundID)]
	var observer RoomEventObserver
	if roundValue != nil {
		observer = roundValue.EventObserver
	}
	s.mu.Unlock()
	if observer == nil {
		return
	}
	observer(ctx, event)
}

func eventRoundID(event protocol.EventMessage) string {
	if roundID := strings.TrimSpace(anyString(event.Data["round_id"])); roundID != "" {
		return roundID
	}
	return strings.TrimSpace(event.CausedBy)
}
