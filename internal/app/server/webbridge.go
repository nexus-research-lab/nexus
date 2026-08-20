// INPUT: WebBridge 服务与当前 Agent runtime Session identity。
// OUTPUT: 启用时稳定注入的 nexus_browser SDK MCP server。
// POS: 浏览器扩展能力在 DM/Room runtime 的组合根适配器。
package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	webbridgemcp "github.com/nexus-research-lab/nexus/internal/mcp/webbridge"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	webbridgesvc "github.com/nexus-research-lab/nexus/internal/service/webbridge"
)

func newWebBridgeMCPBuilder(
	service *webbridgesvc.Service,
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
			webbridgemcp.ServerName: sdkmcp.SDKServerConfig{
				Name: webbridgemcp.ServerName,
				Instance: webbridgemcp.NewServer(
					service,
					sessionKey,
					sourceContextLabel,
					resolveCDPAccess,
				),
			},
		}
	}
}
