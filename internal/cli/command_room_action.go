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
)

func newRoomActionCommand(_ *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "action",
		Short: "创建 Room 内部协作动作",
	}
	command.AddCommand(newRoomPrivateMessageCommand())
	command.AddCommand(newRoomPrivateNoteCommand())
	command.AddCommand(newRoomMarkerCommand())
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
	internalAPIBase  string
	internalToken    string
}

func bindRoomActionCommonFlags(command *cobra.Command, options *roomActionCLIOptions) {
	command.Flags().StringVar(&options.roomID, "room-id", "", "room id，默认读取 NEXUS_ROOM_ID")
	command.Flags().StringVar(&options.conversationID, "conversation-id", "", "conversation id，默认读取 NEXUS_ROOM_CONVERSATION_ID")
	command.Flags().StringVar(&options.sourceAgentID, "source-agent-id", "", "source room agent id，默认读取 NEXUS_ROOM_AGENT_ID")
	command.Flags().StringVar(&options.content, "content", "", "action content")
	command.Flags().StringArrayVar(&options.audienceAgentIDs, "audience-agent-id", nil, "audience room agent id，可重复")
	command.Flags().StringVar((*string)(&options.replyTarget), "reply-target", "", "public_feed|sender_private|target_private|audience|none")
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
		"item":   item,
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
		return usageErrorf("room action requires --source-agent-id or %s", nexusRoomAgentIDEnvName)
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
		SourceAgentID:    strings.TrimSpace(options.sourceAgentID),
		TargetAgentID:    strings.TrimSpace(options.targetAgentID),
		AudienceAgentIDs: normalizeRoomActionCLIIDs(options.audienceAgentIDs),
		Content:          strings.TrimSpace(options.content),
		Visibility:       strings.TrimSpace(options.visibility),
		ReplyTarget:      options.replyTarget,
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
