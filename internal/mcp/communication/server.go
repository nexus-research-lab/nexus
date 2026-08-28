// INPUT: 平台通讯服务、Room realtime 服务与宿主固化的当前 runtime 身份。
// OUTPUT: list_targets 与按 DM/Room 上下文路由的统一 send_message 工具。
// POS: nexus MCP 唯一通讯工具组；模型只表达目标和正文，宿主决定真实会话边界。
package communication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

const (
	destinationContact     = "contact"
	destinationRoom        = "room"
	destinationCurrentRoom = "current_room"
	visibilityPrivate      = "private"
	visibilityPublic       = "public"
)

// RoomService 是统一消息工具调用当前 Room transport 所需的最小接口。
type RoomService interface {
	HandleDirectedMessage(
		context.Context,
		string,
		string,
		protocol.CreateRoomDirectedMessageRequest,
	) (*protocol.RoomDirectedMessageRecord, error)
	HandlePublicMessage(
		context.Context,
		string,
		string,
		protocol.CreateRoomPublicMessageRequest,
	) (protocol.Message, error)
	MarkPublicMessagePublished(context.Context, string, string, string) error
}

// RuntimeContext 承载宿主固定的通讯身份和当前 Room 工具上下文。
type RuntimeContext struct {
	Actor                communicationsvc.Actor
	CurrentAgentRoundID  string
	CurrentRoomAvailable bool
}

// BuildTools 构建当前 runtime 唯一的通讯工具面。
func BuildTools(
	svc *communicationsvc.Service,
	room RoomService,
	sctx RuntimeContext,
) []sdktool.Tool {
	return []sdktool.Tool{
		listTargetsTool(svc, sctx.Actor),
		sendMessageTool(svc, room, sctx),
	}
}

func listTargetsTool(svc *communicationsvc.Service, actor communicationsvc.Actor) sdktool.Tool {
	return sdktool.Tool{
		Name:        "list_targets",
		Description: "列出当前 Agent 可联系的好友与群。返回稳定 ID；发送目标不明确时先调用。",
		SearchHint:  "Nexus 通讯目标 联系人 好友 群 contacts rooms list targets",
		AlwaysLoad:  true,
		InputSchema: objectSchema(map[string]any{}, nil),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, _ map[string]any) (sdktool.ToolResult, error) {
			if svc == nil {
				return errorResult(errors.New("平台通讯服务未装配")), nil
			}
			result, err := svc.ListAddressBook(ctx, actor)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func sendMessageTool(
	svc *communicationsvc.Service,
	room RoomService,
	sctx RuntimeContext,
) sdktool.Tool {
	description := "给通讯录联系人或群发消息。contact 进入好友私聊，room 发布到目标群。"
	if sctx.CurrentRoomAvailable {
		description = "发送消息。current_room 使用当前 Room：private 发给指定成员，public 只用于私域或工具流程额外广播；" +
			"contact 和 room 用于跨会话通讯录目标。普通当前 Room 公区回复直接使用 final reply。"
	}
	return sdktool.Tool{
		Name:        "send_message",
		Description: description,
		SearchHint:  "Nexus 发消息 联系人 群聊 Room 私信 公开 广播 send message",
		AlwaysLoad:  true,
		InputSchema: sendMessageSchema(sctx.CurrentRoomAvailable),
		ContextHandler: func(
			ctx context.Context,
			args map[string]any,
			callContext *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			result, err := sendMessage(ctx, svc, room, sctx, args, callContext)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func sendMessage(
	ctx context.Context,
	svc *communicationsvc.Service,
	room RoomService,
	sctx RuntimeContext,
	args map[string]any,
	callContext *sdktool.CallContext,
) (any, error) {
	switch destination := stringArg(args, "destination"); destination {
	case destinationContact:
		if err := allowOnly(args, "destination", "target_id", "content"); err != nil {
			return nil, err
		}
		return sendToAddressBook(ctx, svc, sctx.Actor, communicationsvc.TargetTypeAgent, args)
	case destinationRoom:
		if err := allowOnly(args, "destination", "target_id", "conversation_id", "content"); err != nil {
			return nil, err
		}
		if sctx.CurrentRoomAvailable &&
			stringArg(args, "target_id") == strings.TrimSpace(sctx.Actor.RoomID) {
			return nil, errors.New("当前 Room 必须使用 destination=current_room")
		}
		return sendToAddressBook(ctx, svc, sctx.Actor, communicationsvc.TargetTypeRoom, args)
	case destinationCurrentRoom:
		if !sctx.CurrentRoomAvailable {
			return nil, errors.New("current_room 只支持 Room runtime")
		}
		return sendToCurrentRoom(ctx, room, sctx, args, callContext)
	default:
		return nil, errors.New("destination 只支持 contact、room 或 current_room")
	}
}

func sendToAddressBook(
	ctx context.Context,
	svc *communicationsvc.Service,
	actor communicationsvc.Actor,
	targetType string,
	args map[string]any,
) (any, error) {
	if svc == nil {
		return nil, errors.New("平台通讯服务未装配")
	}
	return svc.SendMessage(ctx, actor, communicationsvc.SendRequest{
		TargetType:     targetType,
		TargetID:       stringArg(args, "target_id"),
		ConversationID: stringArg(args, "conversation_id"),
		Content:        stringArg(args, "content"),
	})
}

func sendToCurrentRoom(
	ctx context.Context,
	room RoomService,
	sctx RuntimeContext,
	args map[string]any,
	callContext *sdktool.CallContext,
) (any, error) {
	if room == nil {
		return nil, errors.New("Room 通讯服务未装配")
	}
	if err := requireRoomScope(sctx); err != nil {
		return nil, err
	}
	switch stringArg(args, "visibility") {
	case visibilityPrivate:
		if err := allowOnly(
			args,
			"destination", "visibility", "recipients", "wake_targets", "content",
			"wake_policy", "delay_seconds", "reply_route", "correlation_id",
		); err != nil {
			return nil, err
		}
		return sendPrivateRoomMessage(ctx, room, sctx, args, callContext)
	case visibilityPublic:
		if err := allowOnly(args, "destination", "visibility", "content", "correlation_id"); err != nil {
			return nil, err
		}
		return sendPublicRoomMessage(ctx, room, sctx, args)
	default:
		return nil, errors.New("current_room visibility 只支持 private 或 public")
	}
}

func sendPrivateRoomMessage(
	ctx context.Context,
	room RoomService,
	sctx RuntimeContext,
	args map[string]any,
	callContext *sdktool.CallContext,
) (any, error) {
	commandID, err := roomCommandID(sctx, callContext, args)
	if err != nil {
		return nil, err
	}
	request := protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID:      sctx.Actor.AgentID,
		SourceAgentRoundID: strings.TrimSpace(sctx.CurrentAgentRoundID),
		RootRoundID:        sctx.Actor.RoundID,
		CommandID:          commandID,
		Recipients:         stringListArg(args, "recipients"),
		WakeTargets:        stringListArg(args, "wake_targets"),
		Content:            stringArg(args, "content"),
		WakePolicy:         protocol.RoomWakePolicy(stringArg(args, "wake_policy")),
		ReplyRoute:         roomReplyRouteArg(objectArg(args, "reply_route")),
		DelaySeconds:       intArg(args, "delay_seconds"),
		CorrelationID:      stringArg(args, "correlation_id"),
		GoalCollaborationBinding: currentGoalCollaborationBinding(
			sctx.Actor,
		),
	}
	item, err := room.HandleDirectedMessage(
		scopedToolContext(ctx, sctx.Actor),
		sctx.Actor.RoomID,
		sctx.Actor.ConversationID,
		request,
	)
	if errors.Is(err, roomsvc.ErrDirectedReplyAutoRouted) {
		return map[string]any{
			"status":      "reply_via_final",
			"instruction": "Return the intended reply as this turn's final reply; runtime will route it.",
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return directedMessageOutput(item), nil
}

func sendPublicRoomMessage(
	ctx context.Context,
	room RoomService,
	sctx RuntimeContext,
	args map[string]any,
) (any, error) {
	toolCtx := scopedToolContext(ctx, sctx.Actor)
	item, err := room.HandlePublicMessage(
		toolCtx,
		sctx.Actor.RoomID,
		sctx.Actor.ConversationID,
		protocol.CreateRoomPublicMessageRequest{
			SourceAgentID:      sctx.Actor.AgentID,
			SourceAgentRoundID: strings.TrimSpace(sctx.CurrentAgentRoundID),
			RootRoundID:        sctx.Actor.RoundID,
			Content:            stringArg(args, "content"),
			CorrelationID:      stringArg(args, "correlation_id"),
			GoalCollaborationBinding: currentGoalCollaborationBinding(
				sctx.Actor,
			),
		},
	)
	if err != nil {
		return nil, err
	}
	if err = room.MarkPublicMessagePublished(
		toolCtx,
		sctx.Actor.SessionKey,
		sctx.Actor.RoundID,
		sctx.Actor.AgentID,
	); err != nil {
		return nil, err
	}
	return publicMessageOutput(item), nil
}

func currentGoalCollaborationBinding(actor communicationsvc.Actor) *protocol.GoalCollaborationBinding {
	if actor.GoalCollaborationBinding == nil {
		return nil
	}
	return protocol.NormalizeGoalCollaborationBinding(actor.GoalCollaborationBinding())
}

func requireRoomScope(sctx RuntimeContext) error {
	if sctx.Actor.ContextKind != communicationsvc.ContextKindRoom ||
		strings.TrimSpace(sctx.Actor.AgentID) == "" ||
		strings.TrimSpace(sctx.Actor.RoomID) == "" ||
		strings.TrimSpace(sctx.Actor.ConversationID) == "" {
		return errors.New("当前 Room runtime 身份不完整")
	}
	return nil
}

func scopedToolContext(ctx context.Context, actor communicationsvc.Actor) context.Context {
	ownerUserID := strings.TrimSpace(actor.OwnerUserID)
	if ownerUserID == "" {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "nexus_communication_runtime",
	})
}

func roomCommandID(
	sctx RuntimeContext,
	callContext *sdktool.CallContext,
	input map[string]any,
) (string, error) {
	if callContext != nil {
		if toolUseID := strings.TrimSpace(callContext.ToolUseID); toolUseID != "" {
			return toolUseID, nil
		}
	}
	canonical, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("canonicalize communication tool input: %w", err)
	}
	parts := []string{
		sctx.Actor.SessionKey,
		sctx.Actor.ConversationID,
		sctx.Actor.AgentID,
		sctx.CurrentAgentRoundID,
		sctx.Actor.RoundID,
		"send_message",
		string(canonical),
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "room-" + hex.EncodeToString(digest[:]), nil
}

func roomReplyRouteArg(raw map[string]any) protocol.RoomReplyRoute {
	route := protocol.RoomReplyRoute{
		Mode:       protocol.RoomReplyRouteMode(stringArg(raw, "mode")),
		Recipients: stringListArg(raw, "recipients"),
		WakePolicy: protocol.RoomWakePolicy(stringArg(raw, "wake_policy")),
	}
	if next := objectArg(raw, "next_reply_route"); next != nil {
		nextRoute := roomReplyRouteArg(next)
		route.NextReplyRoute = &nextRoute
	}
	return route
}

func directedMessageOutput(message *protocol.RoomDirectedMessageRecord) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	status := "recorded"
	if message.WakePolicy == protocol.RoomWakePolicyImmediate {
		status = "queued"
	} else if message.WakePolicy == protocol.RoomWakePolicyDelayed {
		status = "scheduled"
	}
	return map[string]any{"message_id": message.MessageID, "status": status}
}

func publicMessageOutput(message protocol.Message) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	return map[string]any{"message_id": message["message_id"], "status": "published"}
}

func allowOnly(args map[string]any, allowed ...string) error {
	for key := range args {
		if !slices.Contains(allowed, key) {
			return fmt.Errorf("%s 不适用于当前消息目标", key)
		}
	}
	return nil
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	result := map[string]any{
		"type": "object", "properties": properties, "additionalProperties": false,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func sendMessageSchema(currentRoomAvailable bool) map[string]any {
	destinations := []string{destinationContact, destinationRoom}
	destinationDescription := "contact=好友私聊；room=目标群公区"
	if currentRoomAvailable {
		destinations = append([]string{destinationCurrentRoom}, destinations...)
		destinationDescription += "；current_room=当前 Room"
	}
	properties := map[string]any{
		"destination": map[string]any{
			"type": "string", "enum": destinations,
			"description": destinationDescription,
		},
		"target_id": map[string]any{
			"type": "string", "description": "contact 的 agent_id 或 room 的 room_id",
		},
		"conversation_id": map[string]any{
			"type": "string", "description": "仅跨 Room 指定 conversation；当前 Room 由宿主固定",
		},
		"content": map[string]any{"type": "string", "minLength": 1},
	}
	alternatives := []any{
		messageSchemaAlternative(destinationContact, []string{"destination", "target_id", "content"}),
		messageSchemaAlternative(destinationRoom, []string{"destination", "target_id", "content"}),
	}
	if currentRoomAvailable {
		properties["visibility"] = map[string]any{
			"type": "string", "enum": []string{visibilityPrivate, visibilityPublic},
			"description": "仅 current_room 使用",
		}
		properties["recipients"] = map[string]any{
			"type": "array", "items": map[string]any{"type": "string"},
			"description": "当前 Room 私域消息的可见成员 agent_id",
		}
		properties["wake_targets"] = map[string]any{
			"type": "array", "items": map[string]any{"type": "string"},
			"description": "需要唤醒的 recipients 子集；省略时唤醒全部 recipients",
		}
		properties["wake_policy"] = map[string]any{
			"type": "string", "enum": []string{"none", "immediate", "delayed"},
		}
		properties["delay_seconds"] = map[string]any{"type": "integer", "minimum": 1}
		properties["reply_route"] = replyRouteSchema()
		properties["correlation_id"] = map[string]any{
			"type": "string", "description": "可选，仅用于日志/UI 关联",
		}
		alternatives = append([]any{
			currentRoomMessageSchemaAlternative(
				visibilityPrivate,
				[]string{"destination", "visibility", "recipients", "content"},
			),
			currentRoomMessageSchemaAlternative(
				visibilityPublic,
				[]string{"destination", "visibility", "content"},
			),
		}, alternatives...)
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             []string{"destination", "content"},
		"anyOf":                alternatives,
		"additionalProperties": false,
	}
}

func messageSchemaAlternative(destination string, required []string) map[string]any {
	return map[string]any{
		"properties": map[string]any{
			"destination": map[string]any{"enum": []string{destination}},
		},
		"required": required,
	}
}

func currentRoomMessageSchemaAlternative(visibility string, required []string) map[string]any {
	return map[string]any{
		"properties": map[string]any{
			"destination": map[string]any{"enum": []string{destinationCurrentRoom}},
			"visibility":  map[string]any{"enum": []string{visibility}},
		},
		"required": required,
	}
}

func replyRouteSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type": "string", "enum": []string{"public", "private", "none"},
			},
			"recipients": map[string]any{
				"type": "array", "items": map[string]any{"type": "string"},
			},
			"wake_policy": map[string]any{
				"type": "string", "enum": []string{"none", "immediate"},
			},
			"next_reply_route": map[string]any{
				"type": "object", "description": "private immediate 回复后的下一跳路线",
			},
		},
		"required":             []string{"mode"},
		"additionalProperties": false,
	}
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func stringListArg(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	raw, ok := args[key]
	if !ok {
		return nil
	}
	values := []string{}
	switch typed := raw.(type) {
	case []string:
		values = typed
	case []any:
		for _, item := range typed {
			if value, ok := item.(string); ok {
				values = append(values, value)
			}
		}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func objectArg(args map[string]any, key string) map[string]any {
	value, _ := args[key].(map[string]any)
	return value
}

func intArg(args map[string]any, key string) int {
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		if math.Trunc(value) == value {
			return int(value)
		}
	}
	return 0
}

func jsonResult(value any) sdktool.ToolResult {
	payload, err := json.Marshal(value)
	if err != nil {
		return errorResult(err)
	}
	return sdktool.ToolResult{Content: []map[string]any{{"type": "text", "text": string(payload)}}}
}

func errorResult(err error) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}}, IsError: true,
	}
}
