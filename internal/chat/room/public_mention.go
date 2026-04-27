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

// BuildPublicMentionTriggerMetadata 构建公区 @ fanout 唤醒的目标元数据。
func BuildPublicMentionTriggerMetadata(targetAgentIDs []string, targetIndex int, agentNameByID map[string]string) map[string]any {
	targets := make([]string, 0, len(targetAgentIDs))
	targetNames := make([]string, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		targetAgentID = strings.TrimSpace(targetAgentID)
		if targetAgentID == "" {
			continue
		}
		targets = append(targets, targetAgentID)
		targetNames = append(targetNames, firstNonEmpty(agentNameByID[targetAgentID], targetAgentID))
	}
	if len(targets) == 0 {
		return nil
	}
	return map[string]any{
		"public_mention_target_count": len(targets),
		"public_mention_target_ids":   targets,
		"public_mention_target_index": targetIndex,
		"public_mention_target_names": targetNames,
	}
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
