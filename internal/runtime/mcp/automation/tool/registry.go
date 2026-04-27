// Package tool 定义 nexus_automation MCP 暴露的 8 个工具。
// 每个文件负责一个工具的 schema+handler 装配；registry.go 统一汇总。
package tool

import (
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
)

// BuildAll 汇集全部工具，供 mcp.NewServer 注册。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []agentclient.MCPTool {
	return []agentclient.MCPTool{
		list(svc, sctx),
		create(svc, sctx),
		update(svc, sctx),
		del(svc, sctx),
		status(svc, sctx, "enable_scheduled_task", true),
		status(svc, sctx, "disable_scheduled_task", false),
		runNow(svc, sctx),
		runs(svc, sctx),
	}
}
