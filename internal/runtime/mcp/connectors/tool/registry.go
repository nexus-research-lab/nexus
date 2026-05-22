package tool

import (
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
)

// BuildAll 汇集全部 connector MCP 工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdkmcp.Tool {
	return []sdkmcp.Tool{
		list(svc, sctx),
		call(svc, sctx),
		feishuDocxExportMarkdown(svc, sctx),
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
