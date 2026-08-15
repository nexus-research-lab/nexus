// INPUT: 图片生成服务、Provider 配置解析器与当前 Agent 会话上下文。
// OUTPUT: 仅在生图模型可用时注入的 nexus_imagegen SDK MCP server。
// POS: 图片生成能力在 DM/Room runtime 的组合根适配器。
package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	imagegenmcp "github.com/nexus-research-lab/nexus/internal/mcp/imagegen"
	imagegenmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

type imagegenMCPConfigResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

// newImagegenMCPBuilder 返回 DM/Room 实时链路所需的图片生成 MCPServerBuilder。
func newImagegenMCPBuilder(
	svc imagegenmcpcontract.Service,
	configResolver imagegenMCPConfigResolver,
) func(context.Context, *protocol.Agent, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || configResolver == nil || agentValue == nil ||
			strings.TrimSpace(agentValue.AgentID) == "" ||
			strings.TrimSpace(agentValue.WorkspacePath) == "" {
			return nil
		}
		if _, err := configResolver.ResolveImageConfig(ctx, ""); err != nil {
			return nil
		}
		sctx := imagegenmcpcontract.ServerContext{
			OwnerUserID:   strings.TrimSpace(agentValue.OwnerUserID),
			WorkspacePath: strings.TrimSpace(agentValue.WorkspacePath),
		}
		return map[string]sdkmcp.ServerConfig{
			imagegenmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     imagegenmcpcontract.ServerName,
				Instance: imagegenmcp.NewServer(svc, sctx),
			},
		}
	}
}
