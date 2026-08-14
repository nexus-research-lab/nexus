// INPUT: Room/DM 持久消息、Agent public handoff 宿主注解与 round 投影。
// OUTPUT: 区分正文 mention action 与非动作 reply lineage 的跨 HTTP/WS 消息契约。
// POS: 对话 turn/message wire shape 的唯一协议真相源。
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

// ConversationMessage 是投影后的对话消息，round_id 永远是 root round。
type ConversationMessage struct {
	MessageID     string              `json:"message_id"`
	SessionKey    string              `json:"session_key,omitempty"`
	Role          string              `json:"role"`
	RoundID       string              `json:"round_id"`
	AgentRoundID  string              `json:"agent_round_id,omitempty"`
	AgentID       string              `json:"agent_id,omitempty"`
	ParentID      string              `json:"parent_id,omitempty"`
	Content       any                 `json:"content"`
	Timestamp     int64               `json:"timestamp"`
	DisplayOrder  int64               `json:"display_order,omitempty"`
	StreamStatus  string              `json:"stream_status,omitempty"`
	ResultSummary map[string]any      `json:"result_summary,omitempty"`
	AgentMentions []AgentMention      `json:"agent_mentions,omitempty"`
	HandoffReply  *PublicHandoffReply `json:"handoff_reply,omitempty"`
}

// TurnPendingPermission 是投影输出中挂在 slot 上的待确认权限。
type TurnPendingPermission struct {
	RequestID string `json:"request_id"`
	MessageID string `json:"message_id,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
}

// AgentTurnSlot 表示一个 agent 在某个 turn 中的执行槽位。
type AgentTurnSlot struct {
	AgentID            string                  `json:"agent_id"`
	AgentRoundID       string                  `json:"agent_round_id"`
	MsgID              string                  `json:"msg_id,omitempty"`
	Status             string                  `json:"status"`
	AssistantMessages  []ConversationMessage   `json:"assistant_messages"`
	PendingPermissions []TurnPendingPermission `json:"pending_permissions"`
	ResultSummary      map[string]any          `json:"result_summary,omitempty"`
	StartedAt          *int64                  `json:"started_at,omitempty"`
	FinishedAt         *int64                  `json:"finished_at,omitempty"`
}

// ConversationTurn 是前端时间线主对象，也是历史分页单位。
type ConversationTurn struct {
	RoundID      string                `json:"round_id"`
	Status       string                `json:"status"`
	CreatedAt    int64                 `json:"created_at"`
	UpdatedAt    int64                 `json:"updated_at"`
	UserMessage  *ConversationMessage  `json:"user_message"`
	AgentSlots   []AgentTurnSlot       `json:"agent_slots"`
	SystemEvents []ConversationMessage `json:"system_events"`
	IsLoaded     bool                  `json:"is_loaded"`
}

// ConversationTurnIndexItem 是 navigator / 虚拟列表占位用的 turn 索引项。
type ConversationTurnIndexItem struct {
	RoundID     string   `json:"round_id"`
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
	Status      string   `json:"status"`
	UserPreview string   `json:"user_preview"`
	AgentIDs    []string `json:"agent_ids"`
	Loaded      bool     `json:"loaded"`
}

// TurnPage 是 /turns 历史 API 的分页响应。
type TurnPage struct {
	Turns                 []ConversationTurn `json:"turns"`
	NextBeforeRoundID     string             `json:"next_before_round_id,omitempty"`
	BackwardsAfterRoundID string             `json:"backwards_after_round_id,omitempty"`
}
