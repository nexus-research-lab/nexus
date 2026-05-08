package room

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// IsFinalPublicAssistantMessage 判断消息是否可作为 Room 公区最终 assistant 输出。
func IsFinalPublicAssistantMessage(message protocol.Message) bool {
	if protocol.MessageRole(message) != "assistant" {
		return false
	}
	if message["is_complete"] == true {
		return true
	}
	_, hasResultSummary := message["result_summary"]
	return hasResultSummary
}

// IsMemberAgent 判断 agent_id 是否属于 Room 成员。
func IsMemberAgent(members []protocol.MemberRecord, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return false
	}
	for _, member := range members {
		if member.MemberType == protocol.MemberTypeAgent && strings.TrimSpace(member.MemberAgentID) == agentID {
			return true
		}
	}
	return false
}
