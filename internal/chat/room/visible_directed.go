package room

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func buildRoomDirectedMessageContext(
	messages []protocol.RoomDirectedMessageRecord,
	agentNameByID map[string]string,
	targetAgentID string,
) string {
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		if line := formatRoomDirectedMessageLine(message, agentNameByID); line != "" {
			lines = append(lines, line)
		}
	}
	return wrapRoomDirectedMessageContext(lines, agentNameByID, targetAgentID)
}

func formatRoomDirectedMessageLine(
	message protocol.RoomDirectedMessageRecord,
	agentNameByID map[string]string,
) string {
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return ""
	}
	sourceName := displayAgentName(message.SourceAgentID, agentNameByID)
	recipients := formatReplyRecipients(message.Recipients, agentNameByID)
	if recipients == "" {
		recipients = "specified recipients"
	}
	return fmt.Sprintf(
		"[directed_message recipients=%s reply_route=%s] %s: %s",
		recipients,
		formatReplyRoute(message.ReplyRoute, message.SourceAgentID, agentNameByID),
		sourceName,
		content,
	)
}

func wrapRoomDirectedMessageContext(
	lines []string,
	agentNameByID map[string]string,
	targetAgentID string,
) string {
	if len(lines) == 0 {
		return ""
	}
	header := "These Room directed messages are projected to you and are not part of public_feed. Reveal them only when the task explicitly requires it."
	if strings.TrimSpace(targetAgentID) != "" {
		header = fmt.Sprintf("These Room directed messages are projected to %s and are not part of public_feed. Reveal them only when the task explicitly requires it.", displayAgentName(targetAgentID, agentNameByID))
	}
	return fmt.Sprintf(
		"%s\n\n<room_directed_messages>\n%s\n</room_directed_messages>",
		header,
		strings.Join(lines, "\n"),
	)
}
