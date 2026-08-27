// INPUT: Room/DM 持久消息与 Agent public handoff 宿主注解。
// OUTPUT: 区分正文 mention action 与非动作 reply lineage 的跨 HTTP/WS 消息契约。
// POS: Room/DM 消息注解 wire shape 的唯一协议真相源。
package protocol

import "strings"

// AgentMention 是消息正文中经过服务端目录解析的 Agent mention span。
type AgentMention struct {
	AgentID           string `json:"agent_id"`
	Label             string `json:"label"`
	ContentBlockIndex int    `json:"content_block_index"`
	StartRune         int    `json:"start_rune"`
	EndRune           int    `json:"end_rune"`
	HandoffID         string `json:"handoff_id,omitempty"`
}

// PublicHandoffReply 是宿主从可信 public mention slot 派生的回复因果注解。
// 它只说明当前公开终态在回应哪条 handoff，不是 @ action，也不授予任何权限。
type PublicHandoffReply struct {
	HandoffID       string `json:"handoff_id"`
	SourceMessageID string `json:"source_message_id"`
	SourceAgentID   string `json:"source_agent_id"`
}

// NormalizePublicHandoffReply 将进程内类型或 JSON object 收口为
// 一份完整的不可变回复因果；缺任一身份时 fail closed。
func NormalizePublicHandoffReply(value any) *PublicHandoffReply {
	var result PublicHandoffReply
	switch typed := value.(type) {
	case PublicHandoffReply:
		result = typed
	case *PublicHandoffReply:
		if typed == nil {
			return nil
		}
		result = *typed
	case map[string]any:
		result = PublicHandoffReply{
			HandoffID:       publicHandoffReplyString(typed["handoff_id"]),
			SourceMessageID: publicHandoffReplyString(typed["source_message_id"]),
			SourceAgentID:   publicHandoffReplyString(typed["source_agent_id"]),
		}
	default:
		return nil
	}
	result.HandoffID = strings.TrimSpace(result.HandoffID)
	result.SourceMessageID = strings.TrimSpace(result.SourceMessageID)
	result.SourceAgentID = strings.TrimSpace(result.SourceAgentID)
	if result.HandoffID == "" || result.SourceMessageID == "" || result.SourceAgentID == "" {
		return nil
	}
	return &result
}

func publicHandoffReplyString(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}
