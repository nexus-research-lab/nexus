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
	"unicode/utf8"

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

func newRoomActionCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "action",
		Short: "创建 Room 内部协作动作",
	}
	command.AddCommand(newRoomActionListCommand(services))
	command.AddCommand(newRoomActionCursorsCommand(services))
	command.AddCommand(newRoomPrivateMessageCommand())
	command.AddCommand(newRoomRequestReplyCommand())
	command.AddCommand(newRoomPrivateNoteCommand())
	command.AddCommand(newRoomMarkerCommand())
	return command
}

func newRoomActionListCommand(services *cliServiceProvider) *cobra.Command {
	options := roomActionQueryOptions{}
	command := &cobra.Command{
		Use:   "list",
		Short: "读取 Room action JSONL",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoomActionListCommand(cmd, services, options)
		},
	}
	bindRoomActionQueryFlags(command, &options)
	command.Flags().BoolVar(&options.includeContent, "include-content", false, "显式输出 action 正文")
	command.Flags().IntVar(&options.limit, "limit", 50, "最多返回条数")
	return command
}

func newRoomActionCursorsCommand(services *cliServiceProvider) *cobra.Command {
	options := roomActionQueryOptions{}
	command := &cobra.Command{
		Use:   "cursors",
		Short: "读取 Room action 消费游标",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoomActionCursorsCommand(cmd, services, options)
		},
	}
	bindRoomActionQueryFlags(command, &options)
	return command
}

func newRoomPrivateMessageCommand() *cobra.Command {
	options := roomActionCLIOptions{actionType: protocol.RoomActionTypePrivateMessage}
	command := &cobra.Command{
		Use:   "private-message",
		Short: "发送 Room 内私域消息",
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.replyTarget == "" {
				options.replyTarget = protocol.RoomReplyTargetTargetPrivate
			}
			return runRoomActionCommand(cmd, options)
		},
	}
	bindRoomActionCommonFlags(command, &options)
	command.Flags().StringVar(&options.targetAgentID, "target-agent-id", "", "target room agent id")
	_ = command.MarkFlagRequired("target-agent-id")
	_ = command.MarkFlagRequired("content")
	return command
}

func newRoomRequestReplyCommand() *cobra.Command {
	options := roomActionCLIOptions{
		actionType:  protocol.RoomActionTypeRequestReply,
		replyTarget: protocol.RoomReplyTargetPublicFeed,
		wakePolicy:  protocol.RoomWakePolicyImmediate,
	}
	command := &cobra.Command{
		Use:   "request-reply",
		Short: "请求 Room 成员回复",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoomActionCommand(cmd, options)
		},
	}
	bindRoomActionCommonFlags(command, &options)
	command.Flags().StringVar(&options.targetAgentID, "target-agent-id", "", "target room agent id")
	command.Flags().StringVar((*string)(&options.wakePolicy), "wake-policy", string(protocol.RoomWakePolicyImmediate), "none|immediate")
	_ = command.MarkFlagRequired("target-agent-id")
	_ = command.MarkFlagRequired("content")
	return command
}

func newRoomPrivateNoteCommand() *cobra.Command {
	options := roomActionCLIOptions{actionType: protocol.RoomActionTypePrivateNote}
	command := &cobra.Command{
		Use:   "private-note",
		Short: "记录当前 Room agent 私有上下文",
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.replyTarget == "" {
				options.replyTarget = protocol.RoomReplyTargetSenderPrivate
			}
			return runRoomActionCommand(cmd, options)
		},
	}
	bindRoomActionCommonFlags(command, &options)
	_ = command.MarkFlagRequired("content")
	return command
}

func newRoomMarkerCommand() *cobra.Command {
	options := roomActionCLIOptions{
		actionType: protocol.RoomActionTypeMarker,
		visibility: protocol.RoomActionVisibilityPrivate,
	}
	command := &cobra.Command{
		Use:   "marker",
		Short: "创建 Room 协作标记",
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.replyTarget != "" {
				return runRoomActionCommand(cmd, options)
			}
			if options.visibility == protocol.RoomActionVisibilityPublic {
				options.replyTarget = protocol.RoomReplyTargetPublicFeed
			} else {
				options.replyTarget = protocol.RoomReplyTargetSenderPrivate
			}
			return runRoomActionCommand(cmd, options)
		},
	}
	bindRoomActionCommonFlags(command, &options)
	command.Flags().StringVar(&options.targetAgentID, "target-agent-id", "", "target room agent id")
	command.Flags().StringVar(&options.visibility, "visibility", protocol.RoomActionVisibilityPrivate, "public|private")
	_ = command.MarkFlagRequired("content")
	return command
}

type roomActionCLIOptions struct {
	actionType       protocol.RoomActionType
	roomID           string
	conversationID   string
	sourceAgentID    string
	targetAgentID    string
	audienceAgentIDs []string
	content          string
	visibility       string
	replyTarget      protocol.RoomReplyTarget
	wakePolicy       protocol.RoomWakePolicy
	internalAPIBase  string
	internalToken    string
}

type roomActionQueryOptions struct {
	conversationID string
	agentID        string
	includeContent bool
	limit          int
}

func bindRoomActionCommonFlags(command *cobra.Command, options *roomActionCLIOptions) {
	command.Flags().StringVar(&options.roomID, "room-id", "", "room id，默认读取 NEXUS_ROOM_ID")
	command.Flags().StringVar(&options.conversationID, "conversation-id", "", "conversation id，默认读取 NEXUS_ROOM_CONVERSATION_ID")
	command.Flags().StringVar(&options.content, "content", "", "action content")
	command.Flags().StringArrayVar(&options.audienceAgentIDs, "audience-agent-id", nil, "audience room agent id，可重复")
	command.Flags().StringVar((*string)(&options.replyTarget), "reply-target", "", "public_feed|sender_private|target_private|audience|none")
}

func bindRoomActionQueryFlags(command *cobra.Command, options *roomActionQueryOptions) {
	command.Flags().StringVar(&options.conversationID, "conversation-id", "", "conversation id，默认读取 NEXUS_ROOM_CONVERSATION_ID")
	command.Flags().StringVar(&options.agentID, "agent-id", "", "只读取投影给指定 agent 的 action 或游标")
}

func runRoomActionListCommand(
	_ *cobra.Command,
	services *cliServiceProvider,
	options roomActionQueryOptions,
) error {
	conversationID := strings.TrimSpace(options.conversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(os.Getenv(nexusRoomConversationIDEnvName))
	}
	if conversationID == "" {
		return usageErrorf("room action list requires --conversation-id or %s", nexusRoomConversationIDEnvName)
	}
	store := workspacestore.NewRoomActionStore(services.cfg.WorkspacePath)
	agentID := strings.TrimSpace(options.agentID)
	var (
		actions []protocol.RoomActionRecord
		err     error
	)
	if agentID == "" {
		actions, err = store.ReadActions(conversationID)
	} else {
		actions, err = store.ReadContextActions(conversationID, agentID)
	}
	if err != nil {
		return err
	}
	if options.limit > 0 && len(actions) > options.limit {
		actions = actions[len(actions)-options.limit:]
	}
	items := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		item := roomActionCLIOutputItem(&action, options.includeContent)
		items = append(items, item)
	}
	return emitJSON(map[string]any{
		"domain":          "room",
		"action":          "room_action_list",
		"conversation_id": conversationID,
		"agent_id":        agentID,
		"count":           len(items),
		"items":           items,
	})
}

func runRoomActionCursorsCommand(
	_ *cobra.Command,
	services *cliServiceProvider,
	options roomActionQueryOptions,
) error {
	conversationID := strings.TrimSpace(options.conversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(os.Getenv(nexusRoomConversationIDEnvName))
	}
	if conversationID == "" {
		return usageErrorf("room action cursors requires --conversation-id or %s", nexusRoomConversationIDEnvName)
	}
	store := workspacestore.NewRoomActionStore(services.cfg.WorkspacePath)
	cursors, err := store.ReadActionCursors(conversationID, strings.TrimSpace(options.agentID))
	if err != nil {
		return err
	}
	items := make([]map[string]any, 0, len(cursors))
	for _, cursor := range cursors {
		items = append(items, roomActionCursorCLIOutputItem(cursor))
	}
	return emitJSON(map[string]any{
		"domain":          "room",
		"action":          "room_action_cursors",
		"conversation_id": conversationID,
		"agent_id":        strings.TrimSpace(options.agentID),
		"count":           len(items),
		"items":           items,
	})
}

func runRoomActionCommand(
	cmd *cobra.Command,
	options roomActionCLIOptions,
) error {
	resolved := options.withRoomRuntimeDefaults()
	if err := resolved.validate(); err != nil {
		return err
	}
	scopeUserID, ok := authsvc.CurrentUserID(commandContext(cmd))
	if !ok || strings.TrimSpace(scopeUserID) == "" {
		return usageErrorf("room action requires %s from Room runtime or --scope-user-id", nexusctlUserIDEnvName)
	}
	item, err := createRoomAction(commandContext(cmd), resolved, scopeUserID)
	if err != nil {
		return err
	}
	return emitJSON(map[string]any{
		"domain": "room",
		"action": "room_action_create",
		"item":   roomActionCLIOutputItem(item),
	})
}

func (o roomActionCLIOptions) withRoomRuntimeDefaults() roomActionCLIOptions {
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

func (o roomActionCLIOptions) validate() error {
	if strings.TrimSpace(o.roomID) == "" {
		return usageErrorf("room action requires --room-id or %s", nexusRoomIDEnvName)
	}
	if strings.TrimSpace(o.conversationID) == "" {
		return usageErrorf("room action requires --conversation-id or %s", nexusRoomConversationIDEnvName)
	}
	if strings.TrimSpace(o.sourceAgentID) == "" {
		return usageErrorf("room action requires %s from Room runtime", nexusRoomAgentIDEnvName)
	}
	if strings.TrimSpace(o.content) == "" {
		return usageErrorf("room action requires --content")
	}
	if strings.TrimSpace(o.internalAPIBase) == "" {
		return usageErrorf("room action requires %s from Room runtime", nexusRoomInternalAPIBaseEnvName)
	}
	if strings.TrimSpace(o.internalToken) == "" {
		return usageErrorf("room action requires %s from Room runtime", nexusRoomInternalTokenEnvName)
	}
	if o.actionType == protocol.RoomActionTypePrivateMessage && strings.TrimSpace(o.targetAgentID) == "" {
		return usageErrorf("private-message requires --target-agent-id")
	}
	if o.actionType == protocol.RoomActionTypeRequestReply && strings.TrimSpace(o.targetAgentID) == "" {
		return usageErrorf("request-reply requires --target-agent-id")
	}
	return nil
}

func createRoomAction(
	ctx context.Context,
	options roomActionCLIOptions,
	scopeUserID string,
) (*protocol.RoomActionRecord, error) {
	scopeUserID = strings.TrimSpace(scopeUserID)
	if scopeUserID == "" {
		return nil, usageErrorf("room action requires current user scope")
	}
	payload := protocol.CreateRoomActionRequest{
		ActionType:       options.actionType,
		TargetAgentID:    strings.TrimSpace(options.targetAgentID),
		AudienceAgentIDs: normalizeRoomActionCLIIDs(options.audienceAgentIDs),
		Content:          strings.TrimSpace(options.content),
		Visibility:       strings.TrimSpace(options.visibility),
		ReplyTarget:      options.replyTarget,
		WakePolicy:       options.wakePolicy,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := roomActionEndpoint(options)
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
		return nil, fmt.Errorf("调用 Room internal action endpoint 失败: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Room internal action endpoint 返回 %d: %s", response.StatusCode, responseBody)
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
		return nil, fmt.Errorf("Room internal action endpoint 调用失败: %s", responseBody)
	}
	var item protocol.RoomActionRecord
	if err = json.Unmarshal(envelope.Data, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func roomActionCLIOutputItem(action *protocol.RoomActionRecord, includeContent ...bool) map[string]any {
	if action == nil {
		return map[string]any{}
	}
	item := map[string]any{
		"action_id":       action.ActionID,
		"room_id":         action.RoomID,
		"conversation_id": action.ConversationID,
		"action_type":     string(action.ActionType),
		"source_agent_id": action.SourceAgentID,
		"visibility":      action.Visibility,
		"reply_target":    string(action.ReplyTarget),
		"content_chars":   utf8.RuneCountInString(action.Content),
		"timestamp":       action.Timestamp,
	}
	if strings.TrimSpace(action.RequestID) != "" {
		item["request_id"] = action.RequestID
	}
	if action.WakePolicy != "" {
		item["wake_policy"] = string(action.WakePolicy)
	}
	if strings.TrimSpace(action.TargetAgentID) != "" {
		item["target_agent_id"] = action.TargetAgentID
	}
	if len(action.AudienceAgentIDs) > 0 {
		item["audience_agent_ids"] = append([]string(nil), action.AudienceAgentIDs...)
	}
	if len(includeContent) > 0 && includeContent[0] {
		item["content"] = action.Content
	}
	return item
}

func roomActionCursorCLIOutputItem(cursor workspacestore.RoomActionCursor) map[string]any {
	return map[string]any{
		"room_id":               cursor.RoomID,
		"conversation_id":       cursor.ConversationID,
		"agent_id":              cursor.AgentID,
		"round_id":              cursor.RoundID,
		"last_action_id":        cursor.LastActionID,
		"last_action_timestamp": cursor.LastActionTimestamp,
		"timestamp":             cursor.Timestamp,
	}
}

func normalizeRoomActionCLIIDs(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" || containsRoomActionCLIID(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func containsRoomActionCLIID(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func roomActionEndpoint(options roomActionCLIOptions) string {
	base := strings.TrimRight(strings.TrimSpace(options.internalAPIBase), "/")
	return base + "/internal/rooms/" +
		url.PathEscape(strings.TrimSpace(options.roomID)) +
		"/conversations/" +
		url.PathEscape(strings.TrimSpace(options.conversationID)) +
		"/actions"
}
