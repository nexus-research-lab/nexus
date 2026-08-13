// INPUT: automation 服务与服务端签发的 runtime 来源上下文。
// OUTPUT: 可信 DM/Room 的完整工具集，或外部/后台来源的只读诊断工具集。
// POS: nexus_automation 的 capability 注册边界。
package tool

import (
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
)

// BuildAll 汇集全部工具，供 mcp.NewServer 注册。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	readTools := []sdktool.Tool{
		find(svc, sctx),
		inspectTask(svc, sctx),
		report(svc, sctx),
		getHeartbeat(svc, sctx),
	}
	if !isTrustedInteractiveSource(sctx) {
		return readTools
	}
	return []sdktool.Tool{
		create(svc, sctx),
		readTools[0],
		update(svc, sctx),
		del(svc, sctx),
		readTools[1],
		readTools[2],
		readTools[3],
		runNow(svc, sctx),
		repair(svc, sctx),
		updateHeartbeat(svc, sctx),
		wakeHeartbeat(svc, sctx),
	}
}

func isTrustedInteractiveSource(sctx contract.ServerContext) bool {
	switch strings.TrimSpace(sctx.SourceContextType) {
	case "agent", "agent_paired", "room":
		return true
	default:
		return false
	}
}
