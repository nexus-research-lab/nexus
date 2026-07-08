package tool

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func feishuDocxDriveList(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_drive_list",
		Description: "列出飞书云空间文件，可按 folder_token 分页浏览文档、知识库节点、文件夹等资源。",
		SearchHint:  searchHintFeishuDocxDriveList,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"folder_token": map[string]any{"type": "string"},
				"page_token":   map[string]any{"type": "string"},
				"page_size":    map[string]any{"type": "number"},
				"order_by":     map[string]any{"type": "string"},
				"direction":    map[string]any{"type": "string"},
				"option":       map[string]any{"type": "string", "description": "飞书 drive list option，可选"},
				"file_type":    map[string]any{"type": "string", "description": "客户端过滤类型，如 docx / doc / sheet / bitable / wiki / folder / file"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ListDriveFiles(
				ctx,
				stringValue(args["folder_token"]),
				stringValue(args["page_token"]),
				intValue(args["page_size"]),
				stringValue(args["order_by"]),
				stringValue(args["direction"]),
				stringValue(args["option"]),
			)
			if err != nil {
				return errorResult(err), nil
			}
			result.Files = filterDriveFilesByType(result.Files, stringValue(args["file_type"]))
			return jsonResult(result), nil
		},
	}
}

func feishuDocxWikiSpaces(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_wiki_spaces",
		Description: "列出当前授权账号或应用可访问的飞书知识库空间，返回 space_id、名称和分页信息。",
		SearchHint:  searchHintFeishuDocxWikiSpaces,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"page_token": map[string]any{"type": "string"},
				"page_size":  map[string]any{"type": "number"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ListWikiSpaces(ctx, stringValue(args["page_token"]), intValue(args["page_size"]))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxWikiSpace(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_wiki_space",
		Description: "获取指定飞书知识库空间详情，用于确认知识库名称、描述和 space_id。",
		SearchHint:  searchHintFeishuDocxWikiSpace,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"space_id"},
			"properties": map[string]any{
				"space_id": map[string]any{"type": "string"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			spaceID := strings.TrimSpace(stringValue(args["space_id"]))
			if spaceID == "" {
				return errorResult(errors.New("space_id 不能为空")), nil
			}
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.GetWikiSpace(ctx, spaceID)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxWikiNodes(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_wiki_nodes",
		Description: "分页列出飞书知识库空间中的子节点；不传 parent_node_token 时列出顶层节点，可用于逐层浏览操作文档。",
		SearchHint:  searchHintFeishuDocxWikiNodes,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"space_id"},
			"properties": map[string]any{
				"space_id":          map[string]any{"type": "string"},
				"parent_node_token": map[string]any{"type": "string", "description": "可传 Wiki node_token 或 wiki URL"},
				"page_token":        map[string]any{"type": "string"},
				"page_size":         map[string]any{"type": "number"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			spaceID := strings.TrimSpace(stringValue(args["space_id"]))
			if spaceID == "" {
				return errorResult(errors.New("space_id 不能为空")), nil
			}
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ListWikiNodes(
				ctx,
				spaceID,
				stringValue(args["parent_node_token"]),
				stringValue(args["page_token"]),
				intValue(args["page_size"]),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxWikiNode(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_wiki_node",
		Description: "通过飞书 Wiki URL 或 node_token 解析知识库节点，返回真实 obj_token、obj_type、父节点、标题和是否有子节点。",
		SearchHint:  searchHintFeishuDocxWikiNode,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"token"},
			"properties": map[string]any{
				"token":    map[string]any{"type": "string", "description": "Wiki node_token 或 wiki URL"},
				"obj_type": map[string]any{"type": "string", "description": "可选，默认 wiki"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			token := strings.TrimSpace(stringValue(args["token"]))
			if token == "" {
				return errorResult(errors.New("token 不能为空")), nil
			}
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.GetWikiNodeByToken(ctx, token, stringValue(args["obj_type"]))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
