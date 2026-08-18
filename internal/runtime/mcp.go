package runtime

import (
	"context"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

type enabledConnectorIDsContextKey struct{}

// WithEnabledConnectorIDs 把当前 Session 的显式 Connector 挂载选择交给 MCP builder。
func WithEnabledConnectorIDs(ctx context.Context, connectorIDs []string) context.Context {
	return context.WithValue(ctx, enabledConnectorIDsContextKey{}, slices.Clone(connectorIDs))
}

// EnabledConnectorIDs 返回当前 Session 允许挂载的 Connector。
func EnabledConnectorIDs(ctx context.Context) []string {
	values, _ := ctx.Value(enabledConnectorIDsContextKey{}).([]string)
	return slices.Clone(values)
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
