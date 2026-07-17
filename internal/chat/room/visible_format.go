// INPUT: Room trigger、成员目录与回复路由。
// OUTPUT: 供单个成员消费的动态唤醒文本；公区提及明确区分已公开 source 与新增交付。
// POS: Room 可见上下文中 latest_trigger 的唯一格式化入口。
package room

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func displayAgentName(agentID string, agentNameByID map[string]string) string {
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedAgentID == "" {
		return "unknown"
	}
	if name := strings.TrimSpace(agentNameByID[normalizedAgentID]); name != "" {
		return name
	}
	return normalizedAgentID
}

func formatRoomTrigger(trigger Trigger, agentNameByID map[string]string) string {
	triggerType := strings.TrimSpace(trigger.TriggerType)
	content := strings.TrimSpace(trigger.Content)
	if triggerType == "goal_continuation" {
		return "Goal continuation: continue the active Room goal using this turn's hidden internal goal context. Do not treat this as a new public user message. If this is a multi-member Room Goal and room-visible collaborator evidence is still missing, the lead's public reply should @ exactly one collaborator with a concrete deliverable before attempting completion."
	}
	if triggerType == "" && content == "" {
		return "(No trigger message.)"
	}
	sourceName := firstNonEmpty(agentNameByID[trigger.SourceAgentID], trigger.SourceAgentID)
	if sourceName == "" {
		sourceName = "User"
	}
	var line string
	if content != "" {
		line = sourceName + ": " + content
	} else {
		line = sourceName + ": (No content.)"
	}
	if triggerType == "room_host_default" {
		line += "\nroom host default takeover: the user did not @ any member, and Room settings require you as host to handle this turn. You may answer directly or @ exactly one member to delegate."
	}
	if triggerType == "public_mention" {
		line += "\nThis source message is already published in the Room. Do not repeat, quote, paraphrase, summarize, acknowledge, or confirm it. Output only the new deliverable concretely assigned to you. If it assigns no concrete new work, output exactly <nexus_room_no_reply/>."
	}
	if projection := formatRoomReplyProjection(trigger, agentNameByID); projection != "" {
		line += "\n" + projection
	}
	return line
}

func formatRoomReplyProjection(trigger Trigger, agentNameByID map[string]string) string {
	if trigger.ReplyRoute.Mode == "" {
		return ""
	}
	return fmt.Sprintf("reply_route=%s", formatReplyRoute(trigger.ReplyRoute, trigger.SourceAgentID, agentNameByID))
}

func formatReplyRecipients(agentIDs []string, agentNameByID map[string]string) string {
	if len(agentIDs) == 0 {
		return ""
	}
	items := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		normalizedAgentID := strings.TrimSpace(agentID)
		if normalizedAgentID == "" {
			continue
		}
		items = append(items, fmt.Sprintf("%s(%s)", displayAgentName(normalizedAgentID, agentNameByID), normalizedAgentID))
	}
	return strings.Join(items, ",")
}

func formatReplyRoute(route protocol.RoomReplyRoute, sourceAgentID string, agentNameByID map[string]string) string {
	switch route.Mode {
	case protocol.RoomReplyRoutePublic:
		return "public (this turn's final reply will enter public_feed)"
	case protocol.RoomReplyRoutePrivate:
		recipients := formatReplyRecipients(route.Recipients, agentNameByID)
		if recipients == "" {
			recipients = "specified recipients"
		}
		wake := route.WakePolicy
		if wake == "" {
			wake = protocol.RoomWakePolicyNone
		}
		nextRoute := ""
		if route.NextReplyRoute != nil {
			nextRoute = fmt.Sprintf(" next_reply_route=%s", formatReplyRouteCompact(*route.NextReplyRoute, agentNameByID))
		}
		return fmt.Sprintf("private recipients=%s wake=%s%s (this turn's final reply will not enter public_feed)", recipients, wake, nextRoute)
	case protocol.RoomReplyRouteNone:
		return "none (this turn's final reply only ends this run; it is not projected to any member and will not enter public_feed)"
	default:
		return ""
	}
}

func formatReplyRouteCompact(route protocol.RoomReplyRoute, agentNameByID map[string]string) string {
	switch route.Mode {
	case protocol.RoomReplyRoutePublic:
		return "public"
	case protocol.RoomReplyRoutePrivate:
		recipients := formatReplyRecipients(route.Recipients, agentNameByID)
		if recipients == "" {
			recipients = "specified recipients"
		}
		wake := route.WakePolicy
		if wake == "" {
			wake = protocol.RoomWakePolicyNone
		}
		return fmt.Sprintf("private recipients=%s wake=%s", recipients, wake)
	case protocol.RoomReplyRouteNone:
		return "none"
	default:
		return ""
	}
}
