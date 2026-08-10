// INPUT: owner-main 的 Room/conversation 目标，或服务端固定的当前 Room 上下文。
// OUTPUT: 脱敏 Room/conversation 查询结果。
// POS: nexus_manager Room 只读查询工具。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func listRooms(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "list_nexus_rooms",
		Description: "列出当前 owner 最近的 Room 及 Agent 成员、群主和配置版本。",
		SearchHint:  "Nexus list rooms groups members host",
		InputSchema: listSchema(false),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.ListRooms(ctx, sctx.Actor(), intArg(args, "limit"))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func getRoom(svc contract.Service, sctx contract.ServerContext, currentOnly bool) sdktool.Tool {
	description := "读取当前 owner 下指定 Room 的脱敏状态。"
	schema := idSchema("room_id", "当前 owner 下的 room_id")
	if currentOnly {
		description = "读取服务端固定的当前 Room；不接受 room_id，不能切换到其他 Room。"
		schema = emptySchema()
	}
	return sdktool.Tool{
		Name: "get_nexus_room", Description: description,
		SearchHint:  "Nexus get current room inspect members host settings",
		InputSchema: schema,
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			roomID := ""
			if !currentOnly {
				roomID = stringArg(args, "room_id")
			}
			result, err := svc.GetRoom(ctx, sctx.Actor(), roomID)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func listConversations(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "list_nexus_conversations",
		Description: "列出当前 owner 下指定 Room 的 conversation 上下文；结果会移除 SDK/runtime session 标识和 Agent options。",
		SearchHint:  "Nexus list room conversations topics contexts",
		InputSchema: listRoomContextsSchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.ListRoomContexts(
				ctx, sctx.Actor(), stringArg(args, "room_id"), intArg(args, "limit"),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func getConversation(svc contract.Service, sctx contract.ServerContext, currentOnly bool) sdktool.Tool {
	description := "读取当前 owner 下指定 conversation 的脱敏上下文。"
	schema := idSchema("conversation_id", "当前 owner 下的 conversation_id")
	if currentOnly {
		description = "读取服务端固定的当前 Room conversation；不接受目标参数，不能读取其他话题。"
		schema = emptySchema()
	}
	return sdktool.Tool{
		Name: "get_nexus_conversation", Description: description,
		SearchHint:  "Nexus get current room conversation context members sessions",
		InputSchema: schema,
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			conversationID := ""
			if !currentOnly {
				conversationID = stringArg(args, "conversation_id")
			}
			result, err := svc.GetConversation(ctx, sctx.Actor(), conversationID)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
