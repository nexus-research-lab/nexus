// INPUT: manager 服务与 runtime 固化的初始上下文。
// OUTPUT: owner-main、agent-self、room-member 各自的最小工具集合。
// POS: nexus_manager capability 可见性第一道边界；业务服务仍在每次调用重校验。
package tool

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	managersvc "github.com/nexus-research-lab/nexus/internal/service/nexusmanager"
)

// BuildAll 返回当前 server 快照下最小可见工具；任何陈旧权限仍由服务调用拒绝。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	tools := []sdktool.Tool{inspectCapabilities(svc, sctx)}
	switch strings.ToLower(strings.TrimSpace(sctx.ContextKind)) {
	case managersvc.ContextKindRoom:
		return append(tools,
			getRoom(svc, sctx, true),
			getConversation(svc, sctx, true),
		)
	case managersvc.ContextKindAgent:
		if !sctx.IsMainAgent {
			return append(tools,
				listWorkspace(svc, sctx, false),
				readWorkspaceFile(svc, sctx, false),
			)
		}
		return append(tools,
			listAgents(svc, sctx),
			getAgent(svc, sctx),
			listRooms(svc, sctx),
			getRoom(svc, sctx, false),
			listConversations(svc, sctx),
			getConversation(svc, sctx, false),
			listSessions(svc, sctx),
			getSession(svc, sctx),
			listWorkspace(svc, sctx, true),
			readWorkspaceFile(svc, sctx, true),
		)
	default:
		return tools
	}
}
