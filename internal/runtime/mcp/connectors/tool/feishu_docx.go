package tool

import (
	"context"
	"errors"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	feishudocxapi "github.com/nexus-research-lab/nexus/internal/connectors/feishudocx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
)

func feishuDocxRead(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxSearch(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxSheetSheets(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "feishu_docx_sheet_sheets",
		Description: "列出飞书电子表格内的工作表，返回 sheet_id、标题、行列信息等元数据。",
		SearchHint:  searchHintFeishuDocxSheetSheets,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url"},
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "飞书 Sheet URL 或 spreadsheet_token"},
			},
		},
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ListSheets(ctx, stringValue(args["url"]))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxSheetValues(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "feishu_docx_sheet_values",
		Description: "读取飞书电子表格指定范围的具体单元格内容，适合查看表格正文。",
		SearchHint:  searchHintFeishuDocxSheetValues,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url", "range"},
			"properties": map[string]any{
				"url":   map[string]any{"type": "string", "description": "飞书 Sheet URL 或 spreadsheet_token"},
				"range": map[string]any{"type": "string", "description": "读取范围，例如 Sheet1!A1:D20"},
			},
		},
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ReadSheetValues(ctx, stringValue(args["url"]), stringValue(args["range"]))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxSheetFind(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "feishu_docx_sheet_find",
		Description: "在飞书电子表格指定工作表内查找单元格内容，返回匹配单元格位置。",
		SearchHint:  searchHintFeishuDocxSheetFind,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url", "query"},
			"properties": map[string]any{
				"url":               map[string]any{"type": "string", "description": "飞书 Sheet URL 或 spreadsheet_token；URL 可携带 sheet 参数"},
				"sheet_id":          map[string]any{"type": "string", "description": "工作表 ID，URL 已携带时可省略"},
				"query":             map[string]any{"type": "string", "description": "查找文本或正则表达式"},
				"range":             map[string]any{"type": "string", "description": "可选查找范围，例如 Sheet1!A1:D20"},
				"match_case":        map[string]any{"type": "boolean"},
				"match_entire_cell": map[string]any{"type": "boolean"},
				"search_by_regex":   map[string]any{"type": "boolean"},
				"include_formulas":  map[string]any{"type": "boolean"},
			},
		},
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.FindSheet(
				ctx,
				stringValue(args["url"]),
				stringValue(args["sheet_id"]),
				stringValue(args["query"]),
				stringValue(args["range"]),
				boolValue(args["match_case"]),
				boolValue(args["match_entire_cell"]),
				boolValue(args["search_by_regex"]),
				boolValue(args["include_formulas"]),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxBitableTables(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "feishu_docx_bitable_tables",
		Description: "列出飞书多维表格应用内的数据表，返回 table_id、名称和分页信息。",
		SearchHint:  searchHintFeishuDocxBitableTables,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url"},
			"properties": map[string]any{
				"url":        map[string]any{"type": "string", "description": "飞书 Bitable URL 或 app_token"},
				"page_token": map[string]any{"type": "string"},
				"page_size":  map[string]any{"type": "number"},
			},
		},
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ListBitableTables(ctx, stringValue(args["url"]), stringValue(args["page_token"]), intValue(args["page_size"]))
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxBitableFields(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "feishu_docx_bitable_fields",
		Description: "列出飞书多维表格指定数据表的字段，返回字段名称、类型、属性和说明。",
		SearchHint:  searchHintFeishuDocxBitableFields,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url"},
			"properties": map[string]any{
				"url":        map[string]any{"type": "string", "description": "飞书 Bitable URL 或 app_token；URL 可携带 table 参数"},
				"table_id":   map[string]any{"type": "string", "description": "数据表 ID，URL 已携带时可省略"},
				"view_id":    map[string]any{"type": "string"},
				"page_token": map[string]any{"type": "string"},
				"page_size":  map[string]any{"type": "number"},
			},
		},
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ListBitableFields(
				ctx,
				stringValue(args["url"]),
				stringValue(args["table_id"]),
				stringValue(args["view_id"]),
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

func feishuDocxBitableRecords(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "feishu_docx_bitable_records",
		Description: "读取飞书多维表格指定数据表的记录内容，支持字段选择、视图、筛选、排序和分页。",
		SearchHint:  searchHintFeishuDocxBitableRecords,
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url"},
			"properties": map[string]any{
				"url":              map[string]any{"type": "string", "description": "飞书 Bitable URL 或 app_token；URL 可携带 table 参数"},
				"table_id":         map[string]any{"type": "string", "description": "数据表 ID，URL 已携带时可省略"},
				"view_id":          map[string]any{"type": "string"},
				"field_names":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"filter":           map[string]any{"type": "string", "description": "飞书 Bitable 公式筛选条件"},
				"sort":             map[string]any{"type": "string", "description": "排序 JSON 字符串，例如 [\"字段1 DESC\"]"},
				"page_token":       map[string]any{"type": "string"},
				"page_size":        map[string]any{"type": "number"},
				"automatic_fields": map[string]any{"type": "boolean", "description": "是否返回 created_by/created_time 等自动字段"},
			},
		},
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			client, err := loadFeishuDocxClient(ctx, svc, sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, err := client.ListBitableRecords(
				ctx,
				stringValue(args["url"]),
				stringValue(args["table_id"]),
				stringValue(args["view_id"]),
				stringSliceValue(args["field_names"]),
				stringValue(args["filter"]),
				stringValue(args["sort"]),
				stringValue(args["page_token"]),
				intValue(args["page_size"]),
				boolValue(args["automatic_fields"]),
			)
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}

func feishuDocxCreateDocument(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxAppendMarkdown(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxUpdateBlock(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxDriveList(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxWikiSpaces(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxWikiSpace(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxWikiNodes(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func feishuDocxWikiNode(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
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
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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

func loadFeishuDocxClient(ctx context.Context, svc contract.Service, sctx contract.ServerContext) (*feishudocxapi.Client, error) {
	snapshot, err := svc.LoadActiveConnection(ctx, sctx.OwnerUserID, "feishu-docx")
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, errors.New("飞书云文档连接器未连接")
	}
	return feishuDocxClientFromSnapshot(snapshot), nil
}

func feishuDocxClientFromSnapshot(snapshot *connectordomain.ConnectionSnapshot) *feishudocxapi.Client {
	return feishudocxapi.NewClient(snapshot.APIBaseURL, snapshot.AccessToken, connectorCallHTTPClient)
}

func filterDriveFilesByType(files []map[string]any, fileType string) []map[string]any {
	fileType = strings.TrimSpace(fileType)
	if fileType == "" {
		return files
	}
	result := make([]map[string]any, 0, len(files))
	for _, item := range files {
		if strings.TrimSpace(stringValue(item["type"])) == fileType {
			result = append(result, item)
		}
	}
	return result
}

func boolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				result = append(result, strings.TrimSpace(text))
			}
		}
		return result
	default:
		return nil
	}
}
