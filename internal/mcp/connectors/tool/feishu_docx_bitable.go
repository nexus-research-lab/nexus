package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func feishuDocxBitableTables(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
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
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
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

func feishuDocxBitableFields(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
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
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
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

func feishuDocxBitableRecords(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
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
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
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
