// Package tool 定义 nexus_automation MCP 暴露的定时任务工具。
// 每个文件负责一个工具的 schema+handler 装配；registry.go 统一汇总。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package tool

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
)

// BuildAll 汇集全部工具，供 mcp.NewServer 注册。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		list(svc, sctx),
		searchHistory(svc, sctx),
		create(svc, sctx),
		update(svc, sctx),
		del(svc, sctx),
		status(svc, sctx, "enable_scheduled_task", true),
		status(svc, sctx, "disable_scheduled_task", false),
		inspectTask(svc, sctx),
		runNow(svc, sctx),
		runs(svc, sctx),
		taskEvents(svc, sctx),
		dailyReport(svc, sctx),
		redeliver(svc, sctx),
		recover(svc, sctx),
	}
}
