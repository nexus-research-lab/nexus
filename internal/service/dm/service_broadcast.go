package dm

import (
	"context"
	"strings"
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
	content string,
	deliveryPolicy protocol.ChatDeliveryPolicy,
	attachments []protocol.ChatAttachment,
) {
	message := dmdomain.BuildUserRoundMarker(sessionValue, roundID, content, deliveryPolicy, attachments)
	event := dmdomain.WrapSessionMessageEvent(sessionValue, message, "durable", "")
	s.broadcastEventWithTimeout(ctx, sessionValue.SessionKey, event)
}

func (s *Service) broadcastGuidanceMessage(
	ctx context.Context,
	sessionValue protocol.Session,
	targetRoundID string,
	sourceRoundID string,
	content string,
) {
	message := dmdomain.BuildGuidanceMessage(sessionValue, targetRoundID, sourceRoundID, content, time.Now().UnixMilli())
	event := dmdomain.WrapSessionMessageEvent(sessionValue, message, "ephemeral", strings.TrimSpace(sourceRoundID))
	s.broadcastEventWithTimeout(ctx, sessionValue.SessionKey, event)
}
