package tool

import (
	"context"
	"errors"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
)

func feishuDocxRead(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_read",
		Description: "阅读已授权飞书 Docx 或 Wiki 文档，返回 Markdown，可选择保留 block_id 注释用于后续精准更新。",
		SearchHint:  searchHintFeishuDocxRead,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url"},
			"properties": map[string]any{
				"url":            map[string]any{"type": "string", "description": "飞书 docx/wiki URL，或直接传 document_id"},
				"with_block_ids": map[string]any{"type": "boolean", "description": "是否在 Markdown 中输出 feishu-docx:block_id 注释"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ExportMarkdown(ctx, strings.TrimSpace(stringValue(args["url"])), boolValue(args["with_block_ids"]))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxSearch(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_search",
		Description: "全文搜索当前授权账号可访问的飞书云文档，返回匹配文档 token、类型、标题和分页信息。",
		SearchHint:  searchHintFeishuDocxSearch,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"query"},
			"properties": map[string]any{
				"query":      map[string]any{"type": "string", "description": "搜索关键词"},
				"docs_types": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "可选：doc / docx / wiki / sheet / slides / bitable / mindnote / file"},
				"owner_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "可选，限定文件所有者 open_id"},
				"chat_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "可选，限定文件所在群 ID"},
				"offset":     map[string]any{"type": "number", "description": "搜索偏移量，默认 0"},
				"count":      map[string]any{"type": "number", "description": "返回数量，默认 10，最大 50"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.SearchDocuments(
				ctx,
				stringValue(args["query"]),
				stringSliceValue(args["docs_types"]),
				stringSliceValue(args["owner_ids"]),
				stringSliceValue(args["chat_ids"]),
				intValue(args["offset"]),
				intValue(args["count"]),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxCreateDocument(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_create",
		Description: "创建飞书 Docx 文档，并可直接把 Markdown 内容写入文档。",
		SearchHint:  searchHintFeishuDocxCreate,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"title"},
			"properties": map[string]any{
				"title":        map[string]any{"type": "string"},
				"markdown":     map[string]any{"type": "string"},
				"folder_token": map[string]any{"type": "string", "description": "可选，目标云空间文件夹 token"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			title := strings.TrimSpace(stringValue(args["title"]))
			if title == "" {
				return errorResult(errors.New("title 不能为空")), nil
			}
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.CreateDocument(ctx, title, stringValue(args["markdown"]), stringValue(args["folder_token"]))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxAppendMarkdown(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_append_markdown",
		Description: "向飞书 Docx 或 Wiki 文档末尾追加 Markdown 内容。",
		SearchHint:  searchHintFeishuDocxAppendMarkdown,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url", "markdown"},
			"properties": map[string]any{
				"url":      map[string]any{"type": "string", "description": "飞书 docx/wiki URL，或直接传 document_id"},
				"markdown": map[string]any{"type": "string"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			markdown := strings.TrimSpace(stringValue(args["markdown"]))
			if markdown == "" {
				return errorResult(errors.New("markdown 不能为空")), nil
			}
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			target, err := client.ResolveDocument(ctx, stringValue(args["url"]))
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.AppendMarkdown(ctx, target.DocumentID, markdown)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(map[string]any{
				"document_id":    target.DocumentID,
				"source_type":    target.SourceType,
				"created_blocks": result.CreatedBlocks,
				"children":       result.Children,
			}), nil
		},
	}
}

func feishuDocxUpdateBlock(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "feishu_docx_update_block",
		Description: "更新飞书 Docx 文档中指定文本 Block 的内容。",
		SearchHint:  searchHintFeishuDocxUpdateBlock,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url", "block_id", "content"},
			"properties": map[string]any{
				"url":      map[string]any{"type": "string", "description": "飞书 docx/wiki URL，或直接传 document_id"},
				"block_id": map[string]any{"type": "string"},
				"content":  map[string]any{"type": "string"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			blockID := strings.TrimSpace(stringValue(args["block_id"]))
			if blockID == "" {
				return errorResult(errors.New("block_id 不能为空")), nil
			}
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			target, err := client.ResolveDocument(ctx, stringValue(args["url"]))
			if err != nil {
				return errorResult(err), nil
			}
			block, err := client.UpdateTextBlock(ctx, target.DocumentID, blockID, stringValue(args["content"]))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(map[string]any{
				"document_id": target.DocumentID,
				"block":       block,
			}), nil
		},
	}
}
