// INPUT: runtime 的 exact owner/session 与普通 MCP builder。
// OUTPUT: WorkGraph 编辑 Session 只挂载草图修改工具，其余 Session 保持原 MCP 拓扑。
// POS: 临时编辑 Session 的 MCP 工具面选择边界。
package server

import (
	"context"
	"sync/atomic"

	workgrapheditormcp "github.com/nexus-research-lab/nexus/internal/mcp/workgrapheditor"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type fullMCPBuilder func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig

func workGraphEditorAwareMCPBuilder(
	svc *workgraphworkflowsvc.Service,
	fallback fullMCPBuilder,
) fullMCPBuilder {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		goalObjectiveRevision *atomic.Int64,
		permissionMode sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		if svc != nil && agentValue != nil {
			_, active, err := svc.RuntimeEditorPolicy(agentValue.OwnerUserID, sessionKey)
			if err == nil && active {
				return map[string]sdkmcp.ServerConfig{
					workgrapheditormcp.ServerName: sdkmcp.SDKServerConfig{
						Name:     workgrapheditormcp.ServerName,
						Instance: workgrapheditormcp.NewServer(svc, agentValue.OwnerUserID, sessionKey),
					},
				}
			}
		}
		if fallback == nil {
			return nil
		}
		return fallback(
			ctx,
			agentValue,
			sessionKey,
			roundID,
			sourceContextType,
			sourceContextID,
			sourceContextLabel,
			goalObjectiveRevision,
			permissionMode,
		)
	}
}
