package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func feishuDocxSheetSheets(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
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
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
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

func feishuDocxSheetValues(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
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
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
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

func feishuDocxSheetFind(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
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
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true, OpenWorld: true, MaxResultSizeChars: maxResponseBytes},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
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
