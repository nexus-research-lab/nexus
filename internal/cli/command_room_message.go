package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/spf13/cobra"
)

const (
	nexusRoomIDEnvName              = "NEXUS_ROOM_ID"
	nexusRoomConversationIDEnvName  = "NEXUS_ROOM_CONVERSATION_ID"
	nexusRoomAgentIDEnvName         = "NEXUS_ROOM_AGENT_ID"
	nexusRoomInternalAPIBaseEnvName = "NEXUS_ROOM_INTERNAL_API_BASE"
	nexusRoomInternalTokenEnvName   = "NEXUS_ROOM_INTERNAL_TOKEN"
	nexusInternalTokenHeader        = "X-Nexus-Internal-Token"
	nexusInternalScopeUserIDHeader  = "X-Nexus-Scope-User-ID"
	nexusInternalRoomAgentIDHeader  = "X-Nexus-Room-Agent-ID"
)

func newRoomMessageCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "message",
		Short: "创建和读取 Room 消息",
	}
	command.AddCommand(newRoomMessageListCommand(services))
	command.AddCommand(newRoomMessageCursorsCommand(services))
	command.AddCommand(newRoomMessageSendCommand())
	command.AddCommand(newRoomMessagePublishCommand())
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

func newRoomMessageSendCommand() *cobra.Command {
	options := roomMessageCLIOptions{
		wakePolicy:          protocol.RoomWakePolicyNone,
		replyRouteMode:      protocol.RoomReplyRouteNone,
		replyWakePolicy:     protocol.RoomWakePolicyNone,
		replyNextWakePolicy: protocol.RoomWakePolicyNone,
	}
	command := &cobra.Command{
		Use:   "send",
		Short: "发送 Room directed message",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoomMessageSendCommand(cmd, options)
		},
	}
	command.Flags().StringVar(&options.roomID, "room-id", "", "room id，默认读取 NEXUS_ROOM_ID")
	command.Flags().StringVar(&options.conversationID, "conversation-id", "", "conversation id，默认读取 NEXUS_ROOM_CONVERSATION_ID")
	command.Flags().StringArrayVar(&options.recipientAgentIDs, "recipient-agent-id", nil, "recipient room agent id，可重复")
	command.Flags().StringVar(&options.content, "content", "", "message content")
	command.Flags().StringVar((*string)(&options.wakePolicy), "wake-policy", string(protocol.RoomWakePolicyNone), "none|immediate|delayed")
	command.Flags().IntVar(&options.delaySeconds, "delay-seconds", 0, "wake-policy=delayed 时延迟唤醒秒数")
	command.Flags().StringVar((*string)(&options.replyRouteMode), "reply-route", string(protocol.RoomReplyRouteNone), "public|private|none")
	command.Flags().StringArrayVar(&options.replyRecipientAgentIDs, "reply-recipient-agent-id", nil, "reply_route=private 的 recipient room agent id，可重复")
	command.Flags().StringVar((*string)(&options.replyWakePolicy), "reply-wake-policy", string(protocol.RoomWakePolicyNone), "none|immediate")
	command.Flags().StringVar((*string)(&options.replyNextRouteMode), "reply-next-route", "", "private handback 唤醒后 route recipient 的 reply route: public|private|none")
	command.Flags().StringArrayVar(&options.replyNextRecipientAgentIDs, "reply-next-recipient-agent-id", nil, "reply_next_route=private 的 recipient room agent id，可重复")
	command.Flags().StringVar((*string)(&options.replyNextWakePolicy), "reply-next-wake-policy", string(protocol.RoomWakePolicyNone), "reply_next_route=private 时的 none|immediate")
	command.Flags().StringVar(&options.correlationID, "correlation-id", "", "可选关联 id，仅用于观测分组")
	_ = command.MarkFlagRequired("recipient-agent-id")
	_ = command.MarkFlagRequired("content")
	return command
}

func newRoomMessagePublishCommand() *cobra.Command {
	options := roomPublicMessageCLIOptions{}
	command := &cobra.Command{
		Use:   "publish",
		Short: "发布 Room 公区消息",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoomMessagePublishCommand(cmd, options)
		},
	}
	command.Flags().StringVar(&options.roomID, "room-id", "", "room id，默认读取 NEXUS_ROOM_ID")
	command.Flags().StringVar(&options.conversationID, "conversation-id", "", "conversation id，默认读取 NEXUS_ROOM_CONVERSATION_ID")
	command.Flags().StringVar(&options.content, "content", "", "public message content")
	command.Flags().StringVar(&options.correlationID, "correlation-id", "", "可选关联 id，仅用于观测分组")
	_ = command.MarkFlagRequired("content")
	return command
}

type roomMessageCLIOptions struct {
	roomID                     string
	conversationID             string
	sourceAgentID              string
	recipientAgentIDs          []string
	content                    string
	wakePolicy                 protocol.RoomWakePolicy
	delaySeconds               int
	replyRouteMode             protocol.RoomReplyRouteMode
	replyRecipientAgentIDs     []string
	replyWakePolicy            protocol.RoomWakePolicy
	replyNextRouteMode         protocol.RoomReplyRouteMode
	replyNextRecipientAgentIDs []string
	replyNextWakePolicy        protocol.RoomWakePolicy
	correlationID              string
	internalAPIBase            string
	internalToken              string
}

type roomPublicMessageCLIOptions struct {
	roomID          string
	conversationID  string
	sourceAgentID   string
	content         string
	correlationID   string
	internalAPIBase string
	internalToken   string
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

func runRoomMessageSendCommand(
	cmd *cobra.Command,
	options roomMessageCLIOptions,
) error {
	resolved := options.withRoomRuntimeDefaults()
	if err := resolved.validate(); err != nil {
		return err
	}
	scopeUserID, ok := authsvc.CurrentUserID(commandContext(cmd))
	if !ok || strings.TrimSpace(scopeUserID) == "" {
		return usageErrorf("room message requires %s from Room runtime or --scope-user-id", nexusRoomAgentIDEnvName)
	}
	item, err := createRoomDirectedMessage(commandContext(cmd), resolved, scopeUserID)
	if err != nil {
		return err
	}
	return emitJSON(map[string]any{
		"domain": "room",
		"action": "room_message_send",
		"item":   roomMessageCLIOutputItem(item),
	})
}

func runRoomMessagePublishCommand(
	cmd *cobra.Command,
	options roomPublicMessageCLIOptions,
) error {
	resolved := options.withRoomRuntimeDefaults()
	if err := resolved.validate(); err != nil {
		return err
	}
	scopeUserID, ok := authsvc.CurrentUserID(commandContext(cmd))
	if !ok || strings.TrimSpace(scopeUserID) == "" {
		return usageErrorf("room message publish requires %s from Room runtime or --scope-user-id", nexusRoomAgentIDEnvName)
	}
	item, err := createRoomPublicMessage(commandContext(cmd), resolved, scopeUserID)
	if err != nil {
		return err
	}
	return emitJSON(map[string]any{
		"domain": "room",
		"action": "room_message_publish",
		"item":   roomPublicMessageCLIOutputItem(item),
	})
}

func (o roomMessageCLIOptions) withRoomRuntimeDefaults() roomMessageCLIOptions {
	if strings.TrimSpace(o.roomID) == "" {
		o.roomID = strings.TrimSpace(os.Getenv(nexusRoomIDEnvName))
	}
	if strings.TrimSpace(o.conversationID) == "" {
		o.conversationID = strings.TrimSpace(os.Getenv(nexusRoomConversationIDEnvName))
	}
	if strings.TrimSpace(o.sourceAgentID) == "" {
		o.sourceAgentID = strings.TrimSpace(os.Getenv(nexusRoomAgentIDEnvName))
	}
	if strings.TrimSpace(o.internalAPIBase) == "" {
		o.internalAPIBase = strings.TrimSpace(os.Getenv(nexusRoomInternalAPIBaseEnvName))
	}
	if strings.TrimSpace(o.internalToken) == "" {
		o.internalToken = strings.TrimSpace(os.Getenv(nexusRoomInternalTokenEnvName))
	}
	return o
}

func (o roomPublicMessageCLIOptions) withRoomRuntimeDefaults() roomPublicMessageCLIOptions {
	if strings.TrimSpace(o.roomID) == "" {
		o.roomID = strings.TrimSpace(os.Getenv(nexusRoomIDEnvName))
	}
	if strings.TrimSpace(o.conversationID) == "" {
		o.conversationID = strings.TrimSpace(os.Getenv(nexusRoomConversationIDEnvName))
	}
	if strings.TrimSpace(o.sourceAgentID) == "" {
		o.sourceAgentID = strings.TrimSpace(os.Getenv(nexusRoomAgentIDEnvName))
	}
	if strings.TrimSpace(o.internalAPIBase) == "" {
		o.internalAPIBase = strings.TrimSpace(os.Getenv(nexusRoomInternalAPIBaseEnvName))
	}
	if strings.TrimSpace(o.internalToken) == "" {
		o.internalToken = strings.TrimSpace(os.Getenv(nexusRoomInternalTokenEnvName))
	}
	return o
}

func (o roomMessageCLIOptions) validate() error {
	if strings.TrimSpace(o.roomID) == "" {
		return usageErrorf("room message requires --room-id or %s", nexusRoomIDEnvName)
	}
	if strings.TrimSpace(o.conversationID) == "" {
		return usageErrorf("room message requires --conversation-id or %s", nexusRoomConversationIDEnvName)
	}
	if strings.TrimSpace(o.sourceAgentID) == "" {
		return usageErrorf("room message requires %s from Room runtime", nexusRoomAgentIDEnvName)
	}
	if strings.TrimSpace(o.content) == "" {
		return usageErrorf("room message requires --content")
	}
	if len(normalizeRoomMessageCLIIDs(o.recipientAgentIDs)) == 0 {
		return usageErrorf("room message requires --recipient-agent-id")
	}
	if strings.TrimSpace(o.internalAPIBase) == "" {
		return usageErrorf("room message requires %s from Room runtime", nexusRoomInternalAPIBaseEnvName)
	}
	if strings.TrimSpace(o.internalToken) == "" {
		return usageErrorf("room message requires %s from Room runtime", nexusRoomInternalTokenEnvName)
	}
	if o.wakePolicy == protocol.RoomWakePolicyDelayed {
		if o.delaySeconds <= 0 {
			return usageErrorf("wake-policy=delayed requires --delay-seconds")
		}
	} else if o.delaySeconds != 0 {
		return usageErrorf("--delay-seconds requires --wake-policy delayed")
	}
	switch o.replyRouteMode {
	case protocol.RoomReplyRoutePublic, protocol.RoomReplyRouteNone:
	case protocol.RoomReplyRoutePrivate:
		if len(normalizeRoomMessageCLIIDs(o.replyRecipientAgentIDs)) == 0 {
			return usageErrorf("reply-route=private requires --reply-recipient-agent-id")
		}
		if o.replyWakePolicy != protocol.RoomWakePolicyNone && o.replyWakePolicy != protocol.RoomWakePolicyImmediate {
			return usageErrorf("--reply-wake-policy must be none or immediate")
		}
	default:
		return usageErrorf("--reply-route must be public, private, or none")
	}
	if err := o.validateNextReplyRoute(); err != nil {
		return err
	}
	return nil
}

func (o roomMessageCLIOptions) validateNextReplyRoute() error {
	nextRecipients := normalizeRoomMessageCLIIDs(o.replyNextRecipientAgentIDs)
	hasNextRoute := o.replyNextRouteMode != ""
	hasNextDetails := len(nextRecipients) > 0 || o.replyNextWakePolicy != protocol.RoomWakePolicyNone
	if !hasNextRoute {
		if hasNextDetails {
			return usageErrorf("--reply-next-route is required before reply-next route details")
		}
		return nil
	}
	if o.replyRouteMode != protocol.RoomReplyRoutePrivate || o.replyWakePolicy != protocol.RoomWakePolicyImmediate {
		return usageErrorf("--reply-next-route requires --reply-route private --reply-wake-policy immediate")
	}
	switch o.replyNextRouteMode {
	case protocol.RoomReplyRoutePublic, protocol.RoomReplyRouteNone:
		if len(nextRecipients) > 0 {
			return usageErrorf("--reply-next-recipient-agent-id requires --reply-next-route private")
		}
		if o.replyNextWakePolicy != protocol.RoomWakePolicyNone {
			return usageErrorf("--reply-next-wake-policy requires --reply-next-route private")
		}
	case protocol.RoomReplyRoutePrivate:
		if len(nextRecipients) == 0 {
			return usageErrorf("reply-next-route=private requires --reply-next-recipient-agent-id")
		}
		if o.replyNextWakePolicy != protocol.RoomWakePolicyNone && o.replyNextWakePolicy != protocol.RoomWakePolicyImmediate {
			return usageErrorf("--reply-next-wake-policy must be none or immediate")
		}
	default:
		return usageErrorf("--reply-next-route must be public, private, or none")
	}
	return nil
}

func (o roomPublicMessageCLIOptions) validate() error {
	if strings.TrimSpace(o.roomID) == "" {
		return usageErrorf("room message publish requires --room-id or %s", nexusRoomIDEnvName)
	}
	if strings.TrimSpace(o.conversationID) == "" {
		return usageErrorf("room message publish requires --conversation-id or %s", nexusRoomConversationIDEnvName)
	}
	if strings.TrimSpace(o.sourceAgentID) == "" {
		return usageErrorf("room message publish requires %s from Room runtime", nexusRoomAgentIDEnvName)
	}
	if strings.TrimSpace(o.content) == "" {
		return usageErrorf("room message publish requires --content")
	}
	if strings.TrimSpace(o.internalAPIBase) == "" {
		return usageErrorf("room message publish requires %s from Room runtime", nexusRoomInternalAPIBaseEnvName)
	}
	if strings.TrimSpace(o.internalToken) == "" {
		return usageErrorf("room message publish requires %s from Room runtime", nexusRoomInternalTokenEnvName)
	}
	return nil
}

func createRoomDirectedMessage(
	ctx context.Context,
	options roomMessageCLIOptions,
	scopeUserID string,
) (*protocol.RoomDirectedMessageRecord, error) {
	scopeUserID = strings.TrimSpace(scopeUserID)
	if scopeUserID == "" {
		return nil, usageErrorf("room message requires current user scope")
	}
	payload := protocol.CreateRoomDirectedMessageRequest{
		Recipients:    normalizeRoomMessageCLIIDs(options.recipientAgentIDs),
		Content:       strings.TrimSpace(options.content),
		WakePolicy:    options.wakePolicy,
		DelaySeconds:  options.delaySeconds,
		CorrelationID: strings.TrimSpace(options.correlationID),
		ReplyRoute:    roomMessageCLIReplyRoute(options),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := roomMessageEndpoint(options)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set(nexusInternalTokenHeader, strings.TrimSpace(options.internalToken))
	httpRequest.Header.Set(nexusInternalScopeUserIDHeader, scopeUserID)
	httpRequest.Header.Set(nexusInternalRoomAgentIDHeader, strings.TrimSpace(options.sourceAgentID))

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("调用 Room internal directed message endpoint 失败: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Room internal directed message endpoint 返回 %d: %s", response.StatusCode, responseBody)
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
		Code    any             `json:"code"`
	}
	if err = json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Success {
		return nil, fmt.Errorf("Room internal directed message endpoint 调用失败: %s", responseBody)
	}
	var item protocol.RoomDirectedMessageRecord
	if err = json.Unmarshal(envelope.Data, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func createRoomPublicMessage(
	ctx context.Context,
	options roomPublicMessageCLIOptions,
	scopeUserID string,
) (protocol.Message, error) {
	scopeUserID = strings.TrimSpace(scopeUserID)
	if scopeUserID == "" {
		return nil, usageErrorf("room message publish requires current user scope")
	}
	payload := protocol.CreateRoomPublicMessageRequest{
		Content:       strings.TrimSpace(options.content),
		CorrelationID: strings.TrimSpace(options.correlationID),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := roomPublicMessageEndpoint(options)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set(nexusInternalTokenHeader, strings.TrimSpace(options.internalToken))
	httpRequest.Header.Set(nexusInternalScopeUserIDHeader, scopeUserID)
	httpRequest.Header.Set(nexusInternalRoomAgentIDHeader, strings.TrimSpace(options.sourceAgentID))

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("调用 Room internal public message endpoint 失败: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Room internal public message endpoint 返回 %d: %s", response.StatusCode, responseBody)
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
		Code    any             `json:"code"`
	}
	if err = json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Success {
		return nil, fmt.Errorf("Room internal public message endpoint 调用失败: %s", responseBody)
	}
	var item protocol.Message
	if err = json.Unmarshal(envelope.Data, &item); err != nil {
		return nil, err
	}
	return item, nil
}

func roomMessageCLIReplyRoute(options roomMessageCLIOptions) protocol.RoomReplyRoute {
	switch options.replyRouteMode {
	case protocol.RoomReplyRoutePublic:
		return protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePublic}
	case protocol.RoomReplyRoutePrivate:
		route := protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: normalizeRoomMessageCLIIDs(options.replyRecipientAgentIDs),
			WakePolicy: options.replyWakePolicy,
		}
		if nextRoute := roomMessageCLINextReplyRoute(options); nextRoute != nil {
			route.NextReplyRoute = nextRoute
		}
		return route
	default:
		return protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone}
	}
}

func roomMessageCLINextReplyRoute(options roomMessageCLIOptions) *protocol.RoomReplyRoute {
	switch options.replyNextRouteMode {
	case protocol.RoomReplyRoutePublic:
		return &protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePublic}
	case protocol.RoomReplyRoutePrivate:
		return &protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: normalizeRoomMessageCLIIDs(options.replyNextRecipientAgentIDs),
			WakePolicy: options.replyNextWakePolicy,
		}
	case protocol.RoomReplyRouteNone:
		return &protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone}
	default:
		return nil
	}
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

func roomPublicMessageCLIOutputItem(message protocol.Message) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	content := strings.TrimSpace(roomPublicMessageText(message))
	return map[string]any{
		"message_id":      message["message_id"],
		"room_id":         message["room_id"],
		"conversation_id": message["conversation_id"],
		"source_agent_id": message["agent_id"],
		"timestamp":       message["timestamp"],
		"correlation_id":  message["correlation_id"],
		"content_chars":   len([]rune(content)),
	}
}

func roomPublicMessageText(message protocol.Message) string {
	content, ok := message["content"].([]any)
	if !ok {
		if text, textOK := message["content"].(string); textOK {
			return strings.TrimSpace(text)
		}
		return ""
	}
	parts := make([]string, 0, len(content))
	for _, item := range content {
		block, blockOK := item.(map[string]any)
		if !blockOK {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(block["type"])) != "text" {
			continue
		}
		if text := strings.TrimSpace(fmt.Sprint(block["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
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

func normalizeRoomMessageCLIIDs(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" || containsRoomMessageCLIID(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func containsRoomMessageCLIID(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func roomMessageEndpoint(options roomMessageCLIOptions) string {
	base := strings.TrimRight(strings.TrimSpace(options.internalAPIBase), "/")
	roomID := url.PathEscape(strings.TrimSpace(options.roomID))
	conversationID := url.PathEscape(strings.TrimSpace(options.conversationID))
	return base + "/internal/rooms/" + roomID +
		"/conversations/" + conversationID +
		"/directed-messages"
}

func roomPublicMessageEndpoint(options roomPublicMessageCLIOptions) string {
	base := strings.TrimRight(strings.TrimSpace(options.internalAPIBase), "/")
	roomID := url.PathEscape(strings.TrimSpace(options.roomID))
	conversationID := url.PathEscape(strings.TrimSpace(options.conversationID))
	return base + "/internal/rooms/" + roomID +
		"/conversations/" + conversationID +
		"/public-messages"
}
