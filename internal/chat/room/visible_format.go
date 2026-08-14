// INPUT: Room trigger、成员目录与回复路由。
// OUTPUT: 供单个成员消费的动态唤醒文本；房主按任务结构判断持久协作，公区提及的 final reply 由宿主关联回 source，新 @ 仅表示独立后续 handoff。
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
		return "Goal continuation: continue the active Room goal using this turn's hidden internal goal context. Do not treat this as a new public user message. The current Goal lead may complete it when the objective is satisfied; collaboration evidence is audit context, not a completion requirement. If a conversational contribution would help, a Goal-attributed @ remains conversation-only and creates no WorkBinding, while its substantive public reply may be recorded as collaboration evidence. If accountable collaborator work is needed, create a distinct Ready Work Item and use assign_work so the target receives a WorkBinding."
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
		line += "\nroom host default takeover: the user did not @ any member, and Room settings require you as host to handle this turn. Before substantial execution, assess the task's actual structure: atomicity, separable subproblems, specialist fit, local-subagent value, and whether persistent Room ownership or topology should be recoverable. You may invite one or many members with @ to chat, debate, vote, brainstorm, or provide untracked one-off contributions even when this Room has a background Execution; every raw @ remains conversation-only and creates no Work Item. A substantive public reply to a Goal-attributed @ may be recorded as collaboration evidence, but that evidence is audit context and never a Goal completion gate. When the task needs separately accountable member deliverables or durable dependency, parallel, synthesis, verification, acceptance, recovery, or continuity handoffs, first prepare the complete managed Plan and materialize its exact sealed proposal to create distinct Ready Work Items, then use assign_work for each selected member. assign_work is intentionally unavailable until that bootstrap completes; follow the refreshed context for structured assignment. Never use raw @ as a fallback for planned responsibility. Only the resulting WorkBinding defines responsibility. Once work is assigned, do not duplicate those deliverables yourself; focus on coordination, unblocking, integration, and verification. Direct ownership remains valid when one member can deliver coherently, including by using native subagents inside that responsibility."
	}
	if triggerType == "public_mention" {
		line += "\nThis source message is already published in the Room. The host already associates and returns your final public reply through this source handoff. Do not @ the source merely to address them, confirm delivery, or tell them to continue or close the current step; @ the source only when you genuinely require a distinct new conversational contribution. Do not repeat, quote, paraphrase, summarize, acknowledge, or confirm the source message. A public mention is conversation-only and never carries or activates a managed Assignment; output only the newly requested contribution or untracked one-off result. If it requests no new contribution, output exactly <nexus_room_no_reply/>."
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
