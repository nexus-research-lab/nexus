package workspace

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func normalizeInputQueueItem(
	location InputQueueLocation,
	item protocol.InputQueueItem,
	now int64,
) protocol.InputQueueItem {
	item.ID = strings.TrimSpace(item.ID)
	// Location 是后端已解析的执行域，调用方携带的 item scope 只能作为无 location
	// 的旧数据兜底，不能反向改变队列归属。
	item.Scope = protocol.NormalizeInputQueueScope(string(firstNonEmpty(string(location.Scope), string(item.Scope))))
	item.SessionKey = firstNonEmpty(item.SessionKey, location.SessionKey)
	item.RoomID = firstNonEmpty(item.RoomID, location.RoomID)
	item.ConversationID = firstNonEmpty(item.ConversationID, location.ConversationID)
	item.AgentID = strings.TrimSpace(item.AgentID)
	item.AgentRoundID = strings.TrimSpace(item.AgentRoundID)
	item.ClientMessageID = strings.TrimSpace(item.ClientMessageID)
	item.SourceAgentID = strings.TrimSpace(item.SourceAgentID)
	item.SourceMessageID = strings.TrimSpace(item.SourceMessageID)
	item.HandoffID = strings.TrimSpace(item.HandoffID)
	item.TargetAgentIDs = normalizeInputQueueTargets(item.TargetAgentIDs)
	item.Source = protocol.NormalizeInputQueueSource(string(item.Source))
	item.Content = strings.TrimSpace(item.Content)
	item.Attachments = protocol.NormalizeChatAttachments(item.Attachments, item.AgentID)
	item.DeliveryPolicy = protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy))
	item.ReplyRoute = normalizeInputQueueReplyRoute(item.ReplyRoute)
	if ownerUserID := strings.TrimSpace(location.OwnerUserID); ownerUserID != "" {
		// 队列文件所在的已解析执行域是 owner 事实源，不能让历史
		// JSON 行里的 owner 字段把宿主恢复指向另一用户。
		item.OwnerUserID = ownerUserID
	} else {
		item.OwnerUserID = strings.TrimSpace(item.OwnerUserID)
	}
	item.RootRoundID = strings.TrimSpace(item.RootRoundID)
	item.GoalCollaborationBinding = protocol.NormalizeGoalCollaborationBinding(item.GoalCollaborationBinding)
	item.WorkBinding = normalizeExecutionWorkBinding(item.WorkBinding)
	item.ReviewBinding = normalizeExecutionReviewBinding(item.ReviewBinding)
	if item.HopIndex < 0 {
		item.HopIndex = 0
	}
	if item.CreatedAt == 0 {
		item.CreatedAt = now
	}
	if item.UpdatedAt == 0 {
		item.UpdatedAt = item.CreatedAt
	}
	if item.QueueOrder == 0 {
		item.QueueOrder = item.CreatedAt
	}
	if item.ExpiresAt < 0 {
		item.ExpiresAt = 0
	}
	return item
}

func normalizeExecutionWorkBinding(source *protocol.ExecutionWorkBinding) *protocol.ExecutionWorkBinding {
	if source == nil {
		return nil
	}
	normalized := source.Normalized()
	return &normalized
}

func inputQueueItemFromAny(value any) (protocol.InputQueueItem, bool) {
	switch typed := value.(type) {
	case protocol.InputQueueItem:
		return typed, true
	case map[string]any:
		return protocol.InputQueueItem{
			ID:                       stringFromAny(typed["id"]),
			Scope:                    protocol.InputQueueScope(stringFromAny(typed["scope"])),
			SessionKey:               stringFromAny(typed["session_key"]),
			RoomID:                   stringFromAny(typed["room_id"]),
			ConversationID:           stringFromAny(typed["conversation_id"]),
			AgentID:                  stringFromAny(typed["agent_id"]),
			AgentRoundID:             stringFromAny(typed["agent_round_id"]),
			ClientMessageID:          stringFromAny(typed["client_message_id"]),
			SourceAgentID:            stringFromAny(typed["source_agent_id"]),
			SourceMessageID:          stringFromAny(typed["source_message_id"]),
			HandoffID:                stringFromAny(typed["handoff_id"]),
			TargetAgentIDs:           stringSliceFromAny(typed["target_agent_ids"]),
			Source:                   protocol.InputQueueSource(stringFromAny(typed["source"])),
			Content:                  stringFromAny(typed["content"]),
			Attachments:              protocol.ChatAttachmentsFromAny(typed["attachments"]),
			DeliveryPolicy:           protocol.ChatDeliveryPolicy(stringFromAny(typed["delivery_policy"])),
			ReplyRoute:               inputQueueReplyRouteFromAny(typed["reply_route"]),
			OwnerUserID:              stringFromAny(typed["owner_user_id"]),
			RootRoundID:              stringFromAny(typed["root_round_id"]),
			HopIndex:                 intFromAny(typed["hop_index"]),
			GoalCollaborationBinding: goalCollaborationBindingFromAny(typed["goal_collaboration_binding"]),
			WorkBinding:              executionWorkBindingFromAny(typed["work_binding"]),
			ReviewBinding:            executionReviewBindingFromAny(typed["review_binding"]),
			QueueOrder:               protocol.Int64FromAny(typed["queue_order"]),
			ExpiresAt:                protocol.Int64FromAny(typed["expires_at"]),
			CreatedAt:                protocol.Int64FromAny(typed["created_at"]),
			UpdatedAt:                protocol.Int64FromAny(typed["updated_at"]),
		}, true
	default:
		return protocol.InputQueueItem{}, false
	}
}

func goalCollaborationBindingFromAny(value any) *protocol.GoalCollaborationBinding {
	switch typed := value.(type) {
	case protocol.GoalCollaborationBinding:
		return protocol.NormalizeGoalCollaborationBinding(&typed)
	case *protocol.GoalCollaborationBinding:
		return protocol.NormalizeGoalCollaborationBinding(typed)
	case map[string]any:
		return protocol.NormalizeGoalCollaborationBinding(&protocol.GoalCollaborationBinding{
			GoalID:            stringFromAny(typed["goal_id"]),
			ObjectiveRevision: protocol.Int64FromAny(typed["objective_revision"]),
		})
	default:
		return nil
	}
}

func executionWorkBindingFromAny(value any) *protocol.ExecutionWorkBinding {
	switch typed := value.(type) {
	case protocol.ExecutionWorkBinding:
		result := typed
		return &result
	case *protocol.ExecutionWorkBinding:
		if typed == nil {
			return nil
		}
		result := *typed
		return &result
	case map[string]any:
		return &protocol.ExecutionWorkBinding{
			ExecutionID:  stringFromAny(typed["execution_id"]),
			PlanID:       stringFromAny(typed["plan_id"]),
			WorkItemID:   stringFromAny(typed["work_item_id"]),
			SpecID:       stringFromAny(typed["spec_id"]),
			AssignmentID: stringFromAny(typed["assignment_id"]),
			AttemptID:    stringFromAny(typed["attempt_id"]),
			DispatchID:   stringFromAny(typed["dispatch_id"]),
		}
	default:
		return nil
	}
}

func normalizeExecutionReviewBinding(
	source *protocol.ExecutionReviewBinding,
) *protocol.ExecutionReviewBinding {
	if source == nil {
		return nil
	}
	normalized := source.Normalized()
	return &normalized
}

func executionReviewBindingFromAny(value any) *protocol.ExecutionReviewBinding {
	switch typed := value.(type) {
	case protocol.ExecutionReviewBinding:
		result := typed
		return &result
	case *protocol.ExecutionReviewBinding:
		if typed == nil {
			return nil
		}
		result := *typed
		return &result
	case map[string]any:
		return &protocol.ExecutionReviewBinding{
			ExecutionID:      stringFromAny(typed["execution_id"]),
			PlanID:           stringFromAny(typed["plan_id"]),
			WorkItemID:       stringFromAny(typed["work_item_id"]),
			SpecID:           stringFromAny(typed["spec_id"]),
			AssignmentID:     stringFromAny(typed["assignment_id"]),
			SubmissionID:     stringFromAny(typed["submission_id"]),
			ReviewDispatchID: stringFromAny(typed["review_dispatch_id"]),
			TargetAgentID:    stringFromAny(typed["target_agent_id"]),
		}
	default:
		return nil
	}
}

func normalizeInputQueueTargets(values []string) []string {
	if len(values) == 0 {
		return nil
	}
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

func normalizeInputQueueTimestamp(value any) int64 {
	timestamp := protocol.Int64FromAny(value)
	if timestamp > 0 {
		return timestamp
	}
	return time.Now().UnixMilli()
}

func stringSliceFromAny(value any) []string {
	rawItems, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return typed
		}
		return nil
	}
	result := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		text := stringFromAny(item)
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func normalizeInputQueueReplyRoute(route protocol.RoomReplyRoute) protocol.RoomReplyRoute {
	switch route.Mode {
	case protocol.RoomReplyRoutePublic:
		return protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePublic}
	case protocol.RoomReplyRoutePrivate:
		normalized := protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: normalizeInputQueueTargets(route.Recipients),
			WakePolicy: route.WakePolicy,
		}
		if route.NextReplyRoute != nil {
			next := normalizeInputQueueReplyRoute(*route.NextReplyRoute)
			normalized.NextReplyRoute = &next
		}
		return normalized
	case protocol.RoomReplyRouteNone:
		return protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone}
	default:
		return protocol.RoomReplyRoute{}
	}
}

func inputQueueReplyRouteFromAny(value any) protocol.RoomReplyRoute {
	typed, ok := value.(map[string]any)
	if !ok {
		return protocol.RoomReplyRoute{}
	}
	route := protocol.RoomReplyRoute{
		Mode:       protocol.RoomReplyRouteMode(stringFromAny(typed["mode"])),
		Recipients: stringSliceFromAny(typed["recipients"]),
		WakePolicy: protocol.RoomWakePolicy(stringFromAny(typed["wake_policy"])),
	}
	next := inputQueueReplyRouteFromAny(typed["next_reply_route"])
	if next.Mode != "" {
		route.NextReplyRoute = &next
	}
	return route
}
