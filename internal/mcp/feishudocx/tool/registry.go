package tool

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/feishudocx/contract"
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
