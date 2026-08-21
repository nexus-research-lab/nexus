// INPUT: Browser 服务与当前 Agent runtime Session identity。
// OUTPUT: 启用时稳定注入的 nexus_browser SDK MCP server。
// POS: 浏览器扩展能力在 DM/Room runtime 的组合根适配器。
package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	browsermcp "github.com/nexus-research-lab/nexus/internal/mcp/browser"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	browsersvc "github.com/nexus-research-lab/nexus/internal/service/browser"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

func newBrowserMCPBuilder(
	service *browsersvc.Service,
	preferences *preferencessvc.Service,
) func(
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
		sessionKey string,
		_ string,
		_ string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		if service == nil || agentValue == nil || strings.TrimSpace(sessionKey) == "" {
			return nil
		}
		resolveCDPAccess := func(ctx context.Context) (bool, error) {
			if preferences == nil {
				return false, nil
			}
			item, err := preferences.Get(ctx, agentValue.OwnerUserID)
			return item.BrowserCDPEnabled, err
		}
		return map[string]sdkmcp.ServerConfig{
			browsermcp.ServerName: sdkmcp.SDKServerConfig{
				Name: browsermcp.ServerName,
				Instance: browsermcp.NewServer(
					service,
					sessionKey,
					sourceContextLabel,
					resolveCDPAccess,
				),
			},
		}
	}
}
