// INPUT: 平台通讯服务与 server 固化、不可由模型覆盖的 runtime Actor。
// OUTPUT: 始终加载的通讯录查询和 Agent/Room 消息发送工具。
// POS: nexus_comms MCP transport 边界。
package communication

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	managersvc "github.com/nexus-research-lab/nexus/internal/service/nexusmanager"
)

// ServerName 是 Agent 平台通讯 MCP server 注册名。
const ServerName = "nexus_comms"

// NewServer 创建通讯录与消息发送 MCP server。
func NewServer(
	svc *communicationsvc.Service,
	actor managersvc.Actor,
) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(ServerName, "1.0.0", []sdktool.Tool{
		{
			Name:        "list_address_book",
			Description: "列出你自己的 Nexus 通讯录，包括好友 Agent 与你当前所在的群。返回稳定 target_id，发送前目标不明确时先调用。",
			SearchHint:  "Nexus 通讯录 联系人 好友 群 address book contacts rooms",
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
		},
		{
			Name: "send_message",
			Description: "给通讯录好友或所在群发消息。target_type=agent 会私下投递并立即唤醒好友，回复私下回到并唤醒你；" +
				"target_type=room 会发布到公区，只有正文里明确 @ 的成员会被唤醒。当前 Room 内省略 conversation_id 时发送到当前 conversation；跨 Room 必须显式指定。不会传播当前 workspace、Goal mutation authority 或 WorkBinding。",
			SearchHint: "Nexus 发消息 联系好友 群聊 私信 send message contact room",
			AlwaysLoad: true,
			InputSchema: objectSchema(map[string]any{
				"target_type": map[string]any{
					"type": "string", "enum": []string{communicationsvc.TargetTypeAgent, communicationsvc.TargetTypeRoom},
				},
				"target_id": map[string]any{
					"type": "string", "description": "好友的 agent_id 或群的 room_id",
				},
				"conversation_id": map[string]any{
					"type": "string", "description": "仅群消息可选；当前 Room 内省略时使用当前 conversation，跨 Room 必须显式提供",
				},
				"content": map[string]any{"type": "string", "minLength": 1},
			}, []string{"target_type", "target_id", "content"}),
			Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
				if svc == nil {
					return errorResult(errors.New("平台通讯服务未装配")), nil
				}
				result, err := svc.SendMessage(ctx, actor, communicationsvc.SendRequest{
					TargetType:     stringArg(args, "target_type"),
					TargetID:       stringArg(args, "target_id"),
					ConversationID: stringArg(args, "conversation_id"),
					Content:        stringArg(args, "content"),
				})
				if err != nil {
					return errorResult(err), nil
				}
				return jsonResult(result), nil
			},
		},
	})
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

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
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
