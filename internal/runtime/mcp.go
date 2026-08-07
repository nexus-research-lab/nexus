package runtime

import (
	"errors"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
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
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if options.MCP.Servers[name] != nil {
		return true
	}
	return options.MCP.SDKServers[name] != nil
}
