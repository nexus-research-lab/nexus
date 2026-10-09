// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package realtime

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) enqueueForActiveAgentSlots(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	targetAgentIDs []string,
	content string,
	attachments []protocol.ChatAttachment,
	roundID string,
	userMessageID string,
	ownerUserID string,
) (map[string]struct{}, error) {
	return s.enqueueForActiveAgentSlotsWithTrust(
		ctx,
		sessionKey,
		roomID,
		conversationID,
		targetAgentIDs,
		content,
		attachments,
		roundID,
		userMessageID,
		ownerUserID,
		false,
	)
}
