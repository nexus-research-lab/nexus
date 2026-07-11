package runtime

import (
	"errors"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

const managedGoalMCPServerName = "nexus_goal"

var errManagedGoalMCPServerSetChanged = errors.New("runtime client restart required: managed goal mcp server set changed")

func shouldRestartForManagedGoalMCPServerSetChange(
	currentOptions agentclient.Options,
	nextOptions agentclient.Options,
) bool {
	return hasMCPServer(currentOptions, managedGoalMCPServerName) !=
		hasMCPServer(nextOptions, managedGoalMCPServerName)
}

func hasMCPServer(options agentclient.Options, name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	servers := resolvedMCPServersForManagedGoalCheck(options)
	_, ok := servers[name]
	return ok
}

func resolvedMCPServersForManagedGoalCheck(options agentclient.Options) map[string]sdkmcp.ServerConfig {
	if len(options.MCP.Servers) == 0 && len(options.MCP.SDKServers) == 0 {
		return nil
	}
	servers := make(map[string]sdkmcp.ServerConfig, len(options.MCP.Servers)+len(options.MCP.SDKServers))
	for name, config := range options.MCP.Servers {
		if strings.TrimSpace(name) == "" || config == nil {
			continue
		}
		servers[name] = config
	}
	for name, server := range options.MCP.SDKServers {
		if strings.TrimSpace(name) == "" || server == nil {
			continue
		}
		if _, exists := servers[name]; exists {
			continue
		}
		servers[name] = sdkmcp.SDKServerConfig{
			Name:     name,
			Instance: server,
		}
	}
	if len(servers) == 0 {
		return nil
	}
	return servers
}
