package tool

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
)

// BuildAll 汇集全部工具，供 mcp.NewServer 注册。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		create(svc, sctx),
		find(svc, sctx),
		update(svc, sctx),
		del(svc, sctx),
		inspectTask(svc, sctx),
		report(svc, sctx),
		runNow(svc, sctx),
		repair(svc, sctx),
	}
}
