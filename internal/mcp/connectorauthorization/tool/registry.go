// INPUT: 服务与 server 固化的主智能体私有 DM 快照。
// OUTPUT: owner-main DM 三工具或空工具集。
// POS: nexus_connector_auth capability 可见性第一道边界；service 每次仍动态重验。
package tool

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// BuildAll 只给主智能体的 Agent 私有 DM 暴露授权工具。
func BuildAll(
	svc contract.Service,
	sctx contract.ServerContext,
) []sdktool.Tool {
	if svc == nil ||
		!sctx.IsMainAgent ||
		strings.ToLower(strings.TrimSpace(sctx.ContextKind)) != "agent" ||
		strings.TrimSpace(sctx.OwnerUserID) == "" ||
		strings.TrimSpace(sctx.CurrentAgentID) == "" {
		return nil
	}
	return []sdktool.Tool{
		start(svc, sctx),
		status(svc, sctx),
		cancel(svc, sctx),
	}
}
