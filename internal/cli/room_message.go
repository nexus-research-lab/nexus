package cli

import (
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/spf13/cobra"
)

const (
	nexusRoomConversationIDEnvName = "NEXUS_ROOM_CONVERSATION_ID"
)

func newRoomMessageCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "message",
		Short: "读取 Room 私域消息",
	}
	command.AddCommand(newRoomMessageListCommand(services))
	command.AddCommand(newRoomMessageCursorsCommand(services))
	return command
}

func newRoomMessageListCommand(services *cliServiceProvider) *cobra.Command {
	options := roomMessageQueryOptions{}
	command := &cobra.Command{
		Use:   "list",
		Short: "读取 Room directed message JSONL",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoomMessageListCommand(cmd, services, options)
		},
	}
	bindRoomMessageQueryFlags(command, &options)
	command.Flags().BoolVar(&options.includeContent, "include-content", false, "显式输出 message 正文")
	command.Flags().BoolVar(&options.afterCursor, "after-cursor", false, "只返回目标 agent 最新消费游标之后可见的 message")
	command.Flags().IntVar(&options.limit, "limit", 50, "最多返回条数")
	return command
}

func newRoomMessageCursorsCommand(services *cliServiceProvider) *cobra.Command {
	options := roomMessageQueryOptions{}
	command := &cobra.Command{
		Use:   "cursors",
		Short: "读取 Room directed message 消费游标",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoomMessageCursorsCommand(cmd, services, options)
		},
	}
	bindRoomMessageQueryFlags(command, &options)
	return command
}

type roomMessageQueryOptions struct {
	conversationID string
	agentID        string
	includeContent bool
	afterCursor    bool
	limit          int
}

func bindRoomMessageQueryFlags(command *cobra.Command, options *roomMessageQueryOptions) {
	command.Flags().StringVar(&options.conversationID, "conversation-id", "", "conversation id，默认读取 NEXUS_ROOM_CONVERSATION_ID")
	command.Flags().StringVar(&options.agentID, "agent-id", "", "只读取投影给指定 agent 的 message 或游标")
}

func runRoomMessageListCommand(
	_ *cobra.Command,
	services *cliServiceProvider,
	options roomMessageQueryOptions,
) error {
	conversationID := strings.TrimSpace(options.conversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(os.Getenv(nexusRoomConversationIDEnvName))
	}
	if conversationID == "" {
		return usageErrorf("room message list requires --conversation-id or %s", nexusRoomConversationIDEnvName)
	}
	store := workspacestore.NewRoomDirectedMessageStore(services.cfg.WorkspacePath)
	agentID := strings.TrimSpace(options.agentID)
	cursorFound := false
	var cursor workspacestore.RoomDirectedMessageCursor
	var (
		messages []protocol.RoomDirectedMessageRecord
		err      error
	)
	if agentID == "" {
		if options.afterCursor {
			return usageErrorf("room message list --after-cursor requires --agent-id")
		}
		messages, err = store.ReadMessages(conversationID)
	} else if options.afterCursor {
		cursor, cursorFound, err = store.ReadMessageCursor(conversationID, agentID)
		if err == nil {
			messages, err = store.ReadContextMessagesAfterCursor(conversationID, agentID, cursor)
		}
	} else {
		messages, err = store.ReadVisibleMessages(conversationID, agentID)
	}
	if err != nil {
		return err
	}
	if options.limit > 0 && len(messages) > options.limit {
		messages = messages[len(messages)-options.limit:]
	}
	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		item := roomMessageCLIOutputItem(&message, options.includeContent)
		items = append(items, item)
	}
	output := map[string]any{
		"domain":          "room",
		"action":          "room_message_list",
		"conversation_id": conversationID,
		"agent_id":        agentID,
		"count":           len(items),
		"items":           items,
	}
	if options.afterCursor {
		output["after_cursor"] = true
		output["cursor_found"] = cursorFound
		if cursorFound {
			output["cursor"] = roomMessageCursorCLIOutputItem(cursor)
		}
	}
	return emitJSON(output)
}

func runRoomMessageCursorsCommand(
	_ *cobra.Command,
	services *cliServiceProvider,
	options roomMessageQueryOptions,
) error {
	conversationID := strings.TrimSpace(options.conversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(os.Getenv(nexusRoomConversationIDEnvName))
	}
	if conversationID == "" {
		return usageErrorf("room message cursors requires --conversation-id or %s", nexusRoomConversationIDEnvName)
	}
	store := workspacestore.NewRoomDirectedMessageStore(services.cfg.WorkspacePath)
	cursors, err := store.ReadMessageCursors(conversationID, strings.TrimSpace(options.agentID))
	if err != nil {
		return err
	}
	items := make([]map[string]any, 0, len(cursors))
	for _, cursor := range cursors {
		items = append(items, roomMessageCursorCLIOutputItem(cursor))
	}
	return emitJSON(map[string]any{
		"domain":          "room",
		"action":          "room_message_cursors",
		"conversation_id": conversationID,
		"agent_id":        strings.TrimSpace(options.agentID),
		"count":           len(items),
		"items":           items,
	})
}

func roomMessageCLIOutputItem(message *protocol.RoomDirectedMessageRecord, includeContent ...bool) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	item := map[string]any{
		"message_id":      message.MessageID,
		"room_id":         message.RoomID,
		"conversation_id": message.ConversationID,
		"source_agent_id": message.SourceAgentID,
		"recipients":      message.Recipients,
		"wake_policy":     message.WakePolicy,
		"reply_route":     message.ReplyRoute,
		"delay_seconds":   message.DelaySeconds,
		"correlation_id":  message.CorrelationID,
		"timestamp":       message.Timestamp,
		"content_chars":   len([]rune(message.Content)),
	}
	if len(includeContent) > 0 && includeContent[0] {
		item["content"] = message.Content
	}
	return item
}

func roomMessageCursorCLIOutputItem(cursor workspacestore.RoomDirectedMessageCursor) map[string]any {
	return map[string]any{
		"room_id":                cursor.RoomID,
		"conversation_id":        cursor.ConversationID,
		"agent_id":               cursor.AgentID,
		"round_id":               cursor.RoundID,
		"last_message_id":        cursor.LastMessageID,
		"last_message_timestamp": cursor.LastMessageTimestamp,
		"timestamp":              cursor.Timestamp,
	}
}
