// INPUT: Room member records 与精确 Agent ID。
// OUTPUT: 该成员持久 participation_paused 闸门的权威判断。
// POS: Room 持久成员状态到 realtime 调度策略的无状态领域投影。
package room

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// IsMemberParticipationPaused 判断目标是否是已暂停参与的 Room Agent。
func IsMemberParticipationPaused(
	members []protocol.MemberRecord,
	agentID string,
) bool {
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedAgentID == "" {
		return false
	}
	for _, member := range members {
		if member.MemberType == protocol.MemberTypeAgent &&
			strings.TrimSpace(member.MemberAgentID) == normalizedAgentID {
			return member.ParticipationPaused
		}
	}
	return false
}
