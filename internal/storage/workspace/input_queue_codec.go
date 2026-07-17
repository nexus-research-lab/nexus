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
	item.Scope = protocol.NormalizeInputQueueScope(string(firstNonEmpty(string(item.Scope), string(location.Scope))))
	item.SessionKey = strings.TrimSpace(firstNonEmpty(item.SessionKey, location.SessionKey))
	item.RoomID = strings.TrimSpace(firstNonEmpty(item.RoomID, location.RoomID))
	item.ConversationID = strings.TrimSpace(firstNonEmpty(item.ConversationID, location.ConversationID))
	item.AgentID = strings.TrimSpace(item.AgentID)
	item.SourceAgentID = strings.TrimSpace(item.SourceAgentID)
	item.SourceMessageID = strings.TrimSpace(item.SourceMessageID)
	item.HandoffID = strings.TrimSpace(item.HandoffID)
	item.TargetAgentIDs = normalizeInputQueueTargets(item.TargetAgentIDs)
	item.Source = protocol.NormalizeInputQueueSource(string(item.Source))
	item.Content = strings.TrimSpace(item.Content)
	item.Attachments = protocol.NormalizeChatAttachments(item.Attachments, item.AgentID)
	item.DeliveryPolicy = protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy))
	item.ReplyRoute = normalizeInputQueueReplyRoute(item.ReplyRoute)
	item.OwnerUserID = strings.TrimSpace(item.OwnerUserID)
	item.RootRoundID = strings.TrimSpace(item.RootRoundID)
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

func inputQueueItemFromAny(value any) (protocol.InputQueueItem, bool) {
	switch typed := value.(type) {
	case protocol.InputQueueItem:
		return typed, true
	case map[string]any:
		return protocol.InputQueueItem{
			ID:              stringFromAny(typed["id"]),
			Scope:           protocol.InputQueueScope(stringFromAny(typed["scope"])),
			SessionKey:      stringFromAny(typed["session_key"]),
			RoomID:          stringFromAny(typed["room_id"]),
			ConversationID:  stringFromAny(typed["conversation_id"]),
			AgentID:         stringFromAny(typed["agent_id"]),
			SourceAgentID:   stringFromAny(typed["source_agent_id"]),
			SourceMessageID: stringFromAny(typed["source_message_id"]),
			HandoffID:       stringFromAny(typed["handoff_id"]),
			TargetAgentIDs:  stringSliceFromAny(typed["target_agent_ids"]),
			Source:          protocol.InputQueueSource(stringFromAny(typed["source"])),
			Content:         stringFromAny(typed["content"]),
			Attachments:     protocol.ChatAttachmentsFromAny(typed["attachments"]),
			DeliveryPolicy:  protocol.ChatDeliveryPolicy(stringFromAny(typed["delivery_policy"])),
			ReplyRoute:      inputQueueReplyRouteFromAny(typed["reply_route"]),
			OwnerUserID:     stringFromAny(typed["owner_user_id"]),
			RootRoundID:     stringFromAny(typed["root_round_id"]),
			HopIndex:        intFromAny(typed["hop_index"]),
			QueueOrder:      protocol.Int64FromAny(typed["queue_order"]),
			ExpiresAt:       protocol.Int64FromAny(typed["expires_at"]),
			CreatedAt:       protocol.Int64FromAny(typed["created_at"]),
			UpdatedAt:       protocol.Int64FromAny(typed["updated_at"]),
		}, true
	default:
		return protocol.InputQueueItem{}, false
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
