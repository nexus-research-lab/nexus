// INPUT: DM domain 消息、round/session 状态与广播上下文。
// OUTPUT: 带超时边界的 DM WebSocket 事件。
// POS: DM service 到实时事件总线的唯一广播出口。
package dm

import (
	"context"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var dmBroadcastTimeout = 5 * time.Second

func (s *Service) withBroadcastTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, dmBroadcastTimeout)
}

func (s *Service) broadcastEventWithTimeout(ctx context.Context, sessionKey string, event protocol.EventMessage) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	s.permission.BroadcastEvent(broadcastCtx, sessionKey, event)
}

func (s *Service) broadcastUserRoundMarker(
	ctx context.Context,
	sessionValue protocol.Session,
	roundID string,
	sourceRoundID string,
	userMessageID string,
	content string,
	deliveryPolicy protocol.ChatDeliveryPolicy,
	attachments []protocol.ChatAttachment,
) {
	message := dmdomain.BuildUserRoundMarker(sessionValue, roundID, sourceRoundID, userMessageID, content, deliveryPolicy, attachments)
	event := dmdomain.WrapSessionMessageEvent(sessionValue, message, "durable", roundID)
	s.broadcastEventWithTimeout(ctx, sessionValue.SessionKey, event)
}
