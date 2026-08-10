// INPUT: authorization service and server-fixed owner-main DM context.
// OUTPUT: exactly start/status/cancel/submit-code-card tools for allowed contexts.
// POS: capability visibility fence; the service still dynamically revalidates every call.
package tool

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func BuildAll(
	svc contract.Service,
	sctx contract.ServerContext,
) []sdktool.Tool {
	if !sctx.IsMainAgent ||
		strings.ToLower(strings.TrimSpace(sctx.ContextKind)) != configurationsvc.ContextKindAgent ||
		strings.TrimSpace(sctx.ContextID) != strings.TrimSpace(sctx.CurrentAgentID) {
		return nil
	}
	return []sdktool.Tool{
		start(svc, sctx),
		status(svc, sctx),
		cancel(svc, sctx),
		submitCode(svc, sctx),
	}
}
