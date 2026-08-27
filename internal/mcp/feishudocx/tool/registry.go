package tool

import (
	"encoding/json"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/feishudocx/contract"
)

const (
	searchHintFeishuDocxRead           = "飞书 文档 docx wiki read export markdown 读取 导出 文档 内容"
	searchHintFeishuDocxSearch         = "飞书 云文档 search 搜索 doc docx wiki sheet bitable 文档"
	searchHintFeishuDocxSheetList      = "飞书 表格 sheet sheets list 工作表 sheet_id 元数据"
	searchHintFeishuDocxSheetValues    = "飞书 表格 sheet values read range 单元格 读取"
	searchHintFeishuDocxSheetFind      = "飞书 表格 sheet find search cell 查找 单元格"
	searchHintFeishuDocxBitableTables  = "飞书 多维表格 bitable tables list 数据表 table_id"
	searchHintFeishuDocxBitableFields  = "飞书 多维表格 bitable fields 字段 schema"
	searchHintFeishuDocxBitableRecords = "飞书 多维表格 bitable records 记录 查询 筛选"
	searchHintFeishuDocxCreate         = "飞书 文档 docx create 创建 markdown 写入"
	searchHintFeishuDocxAppendMarkdown = "飞书 文档 docx append markdown 追加 写入"
	searchHintFeishuDocxUpdateBlock    = "飞书 文档 docx update block 修改 block_id"
	searchHintFeishuDocxDriveList      = "飞书 云空间 drive files list 文件夹 浏览 文档"
	searchHintFeishuDocxWikiSpaces     = "飞书 知识库 wiki spaces list 空间"
	searchHintFeishuDocxWikiSpace      = "飞书 知识库 wiki space detail 详情 space_id"
	searchHintFeishuDocxWikiNodes      = "飞书 知识库 wiki nodes list 子节点 目录"
	searchHintFeishuDocxWikiNode       = "飞书 知识库 wiki node resolve URL node_token 解析"
)

// BuildAll 汇集全部飞书云文档 MCP 工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		feishuDocxRead(svc, sctx),
		feishuDocxSearch(svc, sctx),
		feishuDocxSheetList(svc, sctx),
		feishuDocxSheetValues(svc, sctx),
		feishuDocxSheetFind(svc, sctx),
		feishuDocxBitableTables(svc, sctx),
		feishuDocxBitableFields(svc, sctx),
		feishuDocxBitableRecords(svc, sctx),
		feishuDocxCreateDocument(svc, sctx),
		feishuDocxAppendMarkdown(svc, sctx),
		feishuDocxUpdateBlock(svc, sctx),
		feishuDocxDriveList(svc, sctx),
		feishuDocxWikiSpaces(svc, sctx),
		feishuDocxWikiSpace(svc, sctx),
		feishuDocxWikiNodes(svc, sctx),
		feishuDocxWikiNode(svc, sctx),
	}
}

func jsonResult(payload any) sdktool.ToolResult {
	data, err := json.Marshal(payload)
	if err != nil {
		return errorResult(err)
	}
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": string(data)}},
	}
}

func errorResult(err error) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
