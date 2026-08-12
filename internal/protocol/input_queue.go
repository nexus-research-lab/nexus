// INPUT: 输入队列项、host-only Goal collaboration attribution、Execution WorkBinding/ReviewBinding、变更结果与客户端请求关联 ID。
// OUTPUT: 对外提供带非授权 Goal continuation provenance 或 trusted work/review capability 的输入队列协议模型、快照事件与持久接受 ACK。
// POS: protocol 包的输入队列跨边界真相源。
package protocol

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidInputQueueCapabilityEnvelope 表示跨 workspace queue 的责任能力
// envelope 形状不可信。调用方可以据此丢弃历史坏记录，而不是把它恢复成普通消息。
var ErrInvalidInputQueueCapabilityEnvelope = errors.New("invalid input queue capability envelope")

// InputQueueScope 表示待发送队列所在的会话面。
type InputQueueScope string

const (
	InputQueueScopeDM   InputQueueScope = "dm"
	InputQueueScopeRoom InputQueueScope = "room"
)

// InputQueueSource 表示队列项来源。
type InputQueueSource string

const (
	InputQueueSourceUser               InputQueueSource = "user"
	InputQueueSourceAgentPublicMention InputQueueSource = "agent_public_mention"
	InputQueueSourceAgentRoomMessage   InputQueueSource = "agent_room_directed_message"
)

// InputQueueItem 表示后端同步的待发送队列项。
type InputQueueItem struct {
	ID              string             `json:"id"`
	Scope           InputQueueScope    `json:"scope"`
	SessionKey      string             `json:"session_key"`
	RoomID          string             `json:"room_id,omitempty"`
	ConversationID  string             `json:"conversation_id,omitempty"`
	AgentID         string             `json:"agent_id,omitempty"`
	AgentRoundID    string             `json:"agent_round_id,omitempty"`
	ClientMessageID string             `json:"client_message_id,omitempty"`
	SourceAgentID   string             `json:"source_agent_id,omitempty"`
	SourceMessageID string             `json:"source_message_id,omitempty"`
	HandoffID       string             `json:"handoff_id,omitempty"`
	TargetAgentIDs  []string           `json:"target_agent_ids,omitempty"`
	Source          InputQueueSource   `json:"source"`
	Content         string             `json:"content"`
	Attachments     []ChatAttachment   `json:"attachments,omitempty"`
	DeliveryPolicy  ChatDeliveryPolicy `json:"delivery_policy"`
	ReplyRoute      RoomReplyRoute     `json:"reply_route,omitempty"`
	OwnerUserID     string             `json:"owner_user_id,omitempty"`
	RootRoundID     string             `json:"root_round_id,omitempty"`
	HopIndex        int                `json:"hop_index,omitempty"`
	// GoalCollaborationBinding is host-only continuation provenance. It lets a
	// completed Room handoff wake a fresh authorized Goal continuation without
	// granting this queued Agent round Goal mutation capability.
	GoalCollaborationBinding *GoalCollaborationBinding `json:"goal_collaboration_binding,omitempty"`
	WorkBinding              *ExecutionWorkBinding     `json:"work_binding,omitempty"`
	ReviewBinding            *ExecutionReviewBinding   `json:"review_binding,omitempty"`
	QueueOrder               int64                     `json:"queue_order,omitempty"`
	ExpiresAt                int64                     `json:"expires_at,omitempty"`
	CreatedAt                int64                     `json:"created_at"`
	UpdatedAt                int64                     `json:"updated_at"`
}

// ValidateInputQueueCapabilityEnvelope 保证普通通信与受管 responsibility
// 使用互斥 envelope。它不验证数据库状态，只验证跨 workspace queue 的形状。
func ValidateInputQueueCapabilityEnvelope(item InputQueueItem) error {
	if item.GoalCollaborationBinding != nil &&
		NormalizeGoalCollaborationBinding(item.GoalCollaborationBinding) == nil {
		return invalidInputQueueCapabilityEnvelope(
			"goal_collaboration_binding requires goal_id and objective_revision",
		)
	}
	if item.WorkBinding != nil && item.ReviewBinding != nil {
		return invalidInputQueueCapabilityEnvelope("work_binding and review_binding are mutually exclusive")
	}
	if item.WorkBinding == nil && item.ReviewBinding == nil {
		return nil
	}
	if NormalizeInputQueueScope(string(item.Scope)) != InputQueueScopeRoom {
		return invalidInputQueueCapabilityEnvelope("execution-bound input queue item must use Room scope")
	}
	if NormalizeInputQueueSource(string(item.Source)) != InputQueueSourceAgentRoomMessage {
		return invalidInputQueueCapabilityEnvelope("execution-bound input queue item must come from structured Room dispatch")
	}
	if NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)) != ChatDeliveryPolicyQueue {
		return invalidInputQueueCapabilityEnvelope("execution-bound input queue item must start a separate queued round")
	}
	targets := normalizedCapabilityTargets(item.TargetAgentIDs)
	agentID := strings.TrimSpace(item.AgentID)
	if len(targets) != 1 || agentID == "" || targets[0] != agentID {
		return invalidInputQueueCapabilityEnvelope("execution-bound input queue item requires one exact target Agent")
	}
	if item.WorkBinding != nil {
		if missing := firstMissingCapabilityField(map[string]string{
			"execution_id":  item.WorkBinding.ExecutionID,
			"plan_id":       item.WorkBinding.PlanID,
			"work_item_id":  item.WorkBinding.WorkItemID,
			"spec_id":       item.WorkBinding.SpecID,
			"assignment_id": item.WorkBinding.AssignmentID,
			"attempt_id":    item.WorkBinding.AttemptID,
			"dispatch_id":   item.WorkBinding.DispatchID,
		}); missing != "" {
			return invalidInputQueueCapabilityEnvelope("work_binding %s is required", missing)
		}
		return nil
	}
	if missing := firstMissingCapabilityField(map[string]string{
		"execution_id":       item.ReviewBinding.ExecutionID,
		"plan_id":            item.ReviewBinding.PlanID,
		"work_item_id":       item.ReviewBinding.WorkItemID,
		"spec_id":            item.ReviewBinding.SpecID,
		"assignment_id":      item.ReviewBinding.AssignmentID,
		"submission_id":      item.ReviewBinding.SubmissionID,
		"review_dispatch_id": item.ReviewBinding.ReviewDispatchID,
		"target_agent_id":    item.ReviewBinding.TargetAgentID,
	}); missing != "" {
		return invalidInputQueueCapabilityEnvelope("review_binding %s is required", missing)
	}
	if strings.TrimSpace(item.ReviewBinding.TargetAgentID) != agentID {
		return invalidInputQueueCapabilityEnvelope("review_binding target_agent_id conflicts with queue target")
	}
	return nil
}

func invalidInputQueueCapabilityEnvelope(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInputQueueCapabilityEnvelope, fmt.Sprintf(format, args...))
}

func normalizedCapabilityTargets(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func firstMissingCapabilityField(values map[string]string) string {
	for _, key := range []string{
		"execution_id",
		"plan_id",
		"work_item_id",
		"spec_id",
		"assignment_id",
		"attempt_id",
		"dispatch_id",
		"submission_id",
		"review_dispatch_id",
		"target_agent_id",
	} {
		value, exists := values[key]
		if exists && strings.TrimSpace(value) == "" {
			return key
		}
	}
	return ""
}

// InputQueueMutationResult 表示一次已被服务端持久接受的输入队列变更。
type InputQueueMutationResult struct {
	Action    string `json:"action"`
	ItemID    string `json:"item_id,omitempty"`
	Duplicate bool   `json:"duplicate"`
}

// NormalizeInputQueueScope 归一化队列作用域。
func NormalizeInputQueueScope(value string) InputQueueScope {
	switch InputQueueScope(strings.ToLower(strings.TrimSpace(value))) {
	case InputQueueScopeRoom:
		return InputQueueScopeRoom
	default:
		return InputQueueScopeDM
	}
}

// NormalizeInputQueueSource 归一化队列来源。
func NormalizeInputQueueSource(value string) InputQueueSource {
	normalized := InputQueueSource(strings.ToLower(strings.TrimSpace(value)))
	switch normalized {
	case InputQueueSourceAgentPublicMention, InputQueueSourceAgentRoomMessage:
		return normalized
	default:
		return InputQueueSourceUser
	}
}

// NewInputQueueEvent 构造 input_queue 快照事件。
func NewInputQueueEvent(sessionKey string, items []InputQueueItem) EventMessage {
	if items == nil {
		items = []InputQueueItem{}
	}
	scope := string(InputQueueScopeDM)
	roomID := ""
	conversationID := ""
	if len(items) > 0 {
		scope = string(items[0].Scope)
		roomID = strings.TrimSpace(items[0].RoomID)
		conversationID = strings.TrimSpace(items[0].ConversationID)
	}
	event := NewEvent(EventTypeInputQueue, map[string]any{
		"scope": scope,
		"items": items,
	})
	event.SessionKey = strings.TrimSpace(sessionKey)
	event.RoomID = roomID
	event.ConversationID = conversationID
	return event
}

// NewInputQueueAckEvent 构造 input_queue_ack 事件。
// client_request_id / client_message_id 原样回传；duplicate 表示同一幂等请求此前已持久接受。
func NewInputQueueAckEvent(
	sessionKey string,
	clientRequestID string,
	clientMessageID string,
	result InputQueueMutationResult,
) EventMessage {
	event := NewEvent(EventTypeInputQueueAck, map[string]any{
		"accepted":          true,
		"duplicate":         result.Duplicate,
		"action":            strings.TrimSpace(result.Action),
		"item_id":           strings.TrimSpace(result.ItemID),
		"client_request_id": clientRequestID,
		"client_message_id": clientMessageID,
		"ack_timeout_ms":    RequestAckTimeoutMS,
	})
	event.SessionKey = strings.TrimSpace(sessionKey)
	return event
}
