package dm

import (
	"context"
	"strings"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) broadcastUserRoundMarker(
	ctx context.Context,
	sessionValue protocol.Session,
	roundID string,
	content string,
	deliveryPolicy protocol.ChatDeliveryPolicy,
) {
	message := dmdomain.BuildUserRoundMarker(sessionValue, roundID, content, deliveryPolicy)
	event := dmdomain.WrapSessionMessageEvent(sessionValue, message, "durable", "")
	s.permission.BroadcastEvent(ctx, sessionValue.SessionKey, event)
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
	s.permission.BroadcastEvent(ctx, sessionValue.SessionKey, event)
}
