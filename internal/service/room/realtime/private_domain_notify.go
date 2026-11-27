// INPUT: 已持久化的 Room directed message。
// OUTPUT: 房间共享事件，以及涉及 Agent 的私域投影失效通知。
// POS: directed message 创建后的唯一发布出口；私域通知只含 Agent ID，不携带正文。
package realtime

import (
	"context"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// PrivateDomainNotifier 接收私域投影失效通知，参数为受影响的 Agent ID。
type PrivateDomainNotifier func(ctx context.Context, agentIDs []string)

// SetPrivateDomainNotifier 注入私域投影失效通知器。
func (s *Service) SetPrivateDomainNotifier(notifier PrivateDomainNotifier) {
	s.privateDomainNotifier = notifier
}

// publishDirectedMessageCreated 广播房间事件，并令发送方与接收方的私域投影失效。
func (s *Service) publishDirectedMessageCreated(
	ctx context.Context,
	sessionKey string,
	message protocol.RoomDirectedMessageRecord,
) {
	s.broadcastSharedEventWithTimeout(ctx, sessionKey, message.RoomID, newRoomDirectedMessageEvent(message))
	if s.privateDomainNotifier == nil {
		return
	}
	if agentIDs := directedMessageAgentIDs(message); len(agentIDs) > 0 {
		s.privateDomainNotifier(ctx, agentIDs)
	}
}

func directedMessageAgentIDs(message protocol.RoomDirectedMessageRecord) []string {
	agentIDs := make([]string, 0, len(message.Recipients)+1)
	for _, agentID := range append([]string{message.SourceAgentID}, message.Recipients...) {
		if agentID = strings.TrimSpace(agentID); agentID != "" {
			agentIDs = append(agentIDs, agentID)
		}
	}
	slices.Sort(agentIDs)
	return slices.Compact(agentIDs)
}
