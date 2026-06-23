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
	if len(messages) == 0 {
		return ""
	}
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		sourceName := displayAgentName(message.SourceAgentID, agentNameByID)
		recipients := formatReplyRecipients(message.Recipients, agentNameByID)
		if recipients == "" {
			recipients = "specified recipients"
		}
		lines = append(lines, fmt.Sprintf(
			"[directed_message recipients=%s reply_route=%s] %s: %s",
			recipients,
			formatReplyRoute(message.ReplyRoute, message.SourceAgentID, agentNameByID),
			sourceName,
			content,
		))
	}
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
