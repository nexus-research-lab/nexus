// INPUT: Room member records 与精确 Agent ID。
// OUTPUT: 该成员持久 participation_paused 闸门与去重 Agent 成员规模的权威判断。
// POS: Room 持久成员状态到 realtime 调度及共享 Goal 完成门槛的无状态领域投影。
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

// HasMultipleAgentMembers 判断 Room 当前是否包含至少两个不同 Agent 成员。
func HasMultipleAgentMembers(members []protocol.MemberRecord) bool {
	agentIDs := make(map[string]struct{}, len(members))
	for _, member := range members {
		agentID := strings.TrimSpace(member.MemberAgentID)
		if member.MemberType != protocol.MemberTypeAgent || agentID == "" {
			continue
		}
		agentIDs[agentID] = struct{}{}
		if len(agentIDs) > 1 {
			return true
		}
	}
	return false
}
