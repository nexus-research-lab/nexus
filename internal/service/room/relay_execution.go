// INPUT: 宿主验证后的在线会话绑定与本机 Agent。
// OUTPUT: 同 owner/在线 Room/Agent 的确定性本地执行 Room。
// POS: 在线任务复用本机 Room 历史和审批，不将远程 ID 冒充本地 Room ID。
package room

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) EnsureRelayExecutionRoom(ctx context.Context, binding, agentID string) (*protocol.ConversationContextAggregate, error) {
	if binding == "" || agentID == "" {
		return nil, errors.New("缺少在线执行绑定")
	}
	hash := sha256.Sum256([]byte(authctx.OwnerUserID(ctx) + "\x00" + binding + "\x00" + agentID))
	id := hex.EncodeToString(hash[:16])
	roomID, conversationID := "relay_"+id, "relay_conversation_"+id
	existing, err := s.repository.GetConversationContext(ctx, authctx.OwnerUserID(ctx), conversationID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return s.createRoomWithIDs(ctx, protocol.CreateRoomRequest{AgentIDs: []string{agentID}, Name: "在线任务 · " + agentID}, protocol.RoomTypeGroup, roomID, conversationID)
	}
	count := 0
	for _, member := range existing.Members {
		if member.MemberType == protocol.MemberTypeAgent {
			count++
			if member.MemberAgentID != agentID {
				return nil, errors.New("在线执行 Room 成员已改变")
			}
		}
	}
	if count != 1 || existing.Room.ID != roomID || existing.Room.PrivateMessagesEnabled || existing.Room.HostAutoReplyEnabled {
		return nil, errors.New("在线执行 Room 配置已改变")
	}
	return existing, nil
}
