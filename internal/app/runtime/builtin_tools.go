// INPUT: 当前 Agent round 与 Browser、Imagegen、Visualize 服务。
// OUTPUT: 按当前配置动态可见的 Nexus 内建工具。
// POS: nexus MCP 的通用内建工具装配入口。
package runtime

import (
	"context"
	"strings"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	browsermcp "github.com/nexus-research-lab/nexus/internal/mcp/browser"
	imagegenmcp "github.com/nexus-research-lab/nexus/internal/mcp/imagegen"
	imagegenmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	visualizemcp "github.com/nexus-research-lab/nexus/internal/mcp/visualize"
	browsersvc "github.com/nexus-research-lab/nexus/internal/service/browser"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

// ImagegenConfigResolver 解析当前可用的图片生成配置。
type ImagegenConfigResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

// NewBrowserToolBuilder 构建浏览器内建工具。
func NewBrowserToolBuilder(
	service *browsersvc.Service,
	preferences *preferencessvc.Service,
) ToolBuilder {
	return func(_ context.Context, round nexusmcp.RoundContext) []sdktool.Tool {
		agentValue := round.CommandContext.Agent
		sessionKey := round.SessionKey
		roundID := round.RoundID
		if service == nil || agentValue == nil || strings.TrimSpace(sessionKey) == "" || strings.TrimSpace(roundID) == "" {
			return nil
		}
		resolveCDPAccess := func(ctx context.Context) (bool, error) {
			if preferences == nil {
				return false, nil
			}
			item, err := preferences.Get(ctx, agentValue.OwnerUserID)
			return item.BrowserCDPEnabled, err
		}
		return browsermcp.BuildTools(
			service,
			sessionKey,
			roundID,
			round.SourceContextLabel,
			resolveCDPAccess,
		)
	}
}

// NewImagegenToolBuilder 返回 DM/Room 实时链路所需的图片生成工具构造器。
func NewImagegenToolBuilder(
	svc imagegenmcpcontract.Service,
	configResolver ImagegenConfigResolver,
) ToolBuilder {
	return func(ctx context.Context, round nexusmcp.RoundContext) []sdktool.Tool {
		agentValue := round.CommandContext.Agent
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
		return imagegenmcp.BuildTools(svc, sctx)
	}
}

// NewVisualizeToolBuilder 构建可视化内建工具。
func NewVisualizeToolBuilder() ToolBuilder {
	return func(_ context.Context, round nexusmcp.RoundContext) []sdktool.Tool {
		if round.CommandContext.Agent == nil {
			return nil
		}
		return visualizemcp.BuildTools()
	}
}
