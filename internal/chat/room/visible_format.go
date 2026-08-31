// INPUT: Room trigger、成员目录与回复路由。
// OUTPUT: 供单个成员消费的动态唤醒正文与紧凑回复路由；稳定行为契约由 visible_prompt.go 提供。
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
		return "Continue the active Room Goal from the hidden Goal context."
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
	if projection := formatRoomReplyProjection(trigger, agentNameByID); projection != "" {
		line += "\n" + projection
	}
	return line
}

func formatRoomReplyProjection(trigger Trigger, agentNameByID map[string]string) string {
	if trigger.ReplyRoute.Mode == "" {
		return ""
	}
	return fmt.Sprintf("reply_route=%s", formatReplyRoute(trigger.ReplyRoute, agentNameByID))
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

func formatReplyRoute(route protocol.RoomReplyRoute, agentNameByID map[string]string) string {
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
		nextRoute := ""
		if route.NextReplyRoute != nil {
			nextRoute = fmt.Sprintf(" next_reply_route=%s", formatReplyRoute(*route.NextReplyRoute, agentNameByID))
		}
		return fmt.Sprintf("private recipients=%s wake=%s%s", recipients, wake, nextRoute)
	case protocol.RoomReplyRouteNone:
		return "none"
	default:
		return ""
	}
}
