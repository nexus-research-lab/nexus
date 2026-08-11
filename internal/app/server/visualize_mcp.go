// INPUT: 当前 Agent 会话上下文。
// OUTPUT: 对所有 Agent 注入的 nexus_visualize SDK MCP server。
// POS: 生成式 UI 在 DM/Room runtime 的组合根适配器。
package server

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	visualizemcp "github.com/nexus-research-lab/nexus/internal/mcp/visualize"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func newVisualizeMCPBuilder() func(
	context.Context,
	*protocol.Agent,
	string,
	string,
	string,
	string,
) map[string]sdkmcp.ServerConfig {
	return func(
		_ context.Context,
		agentValue *protocol.Agent,
		_ string,
		_ string,
		_ string,
		_ string,
	) map[string]sdkmcp.ServerConfig {
		if agentValue == nil {
			return nil
		}
		return map[string]sdkmcp.ServerConfig{
			visualizemcp.ServerName: sdkmcp.SDKServerConfig{
				Name:     visualizemcp.ServerName,
				Instance: visualizemcp.NewServer(),
			},
		}
	}
}
