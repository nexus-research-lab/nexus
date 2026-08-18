package clientopts

import (
	"context"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

type countingMCPRuntimeResolver struct {
	calls int
}

func (r *countingMCPRuntimeResolver) ResolveRuntimeConfig(
	context.Context,
	string,
	string,
) (*RuntimeConfig, error) {
	r.calls++
	return nil, nil
}

func TestMergeAgentMCPServersParsesSupportedTransports(t *testing.T) {
	builtIn := map[string]sdkmcp.ServerConfig{
		"nexus_visualize": sdkmcp.HTTPServerConfig{URL: "https://nexus.invalid/mcp"},
	}
	merged, err := MergeAgentMCPServers(builtIn, map[string]any{
		"local_tools": map[string]any{
			"type":    "stdio",
			"command": "npx",
			"args":    []any{"-y", "local-mcp"},
			"env":     map[string]any{"TOKEN": "secret"},
		},
		"remote_http": map[string]any{
			"type":          "http",
			"url":           "https://mcp.example.com/rpc",
			"headers":       map[string]any{"Authorization": "Bearer token"},
			"headersHelper": "/opt/nexus/header-helper",
			"oauth": map[string]any{
				"clientId":              "client-1",
				"callbackPort":          43123,
				"authServerMetadataUrl": "https://auth.example.com/.well-known/oauth-authorization-server",
				"xaa":                   true,
			},
		},
		"remote_sse": map[string]any{
			"type":    "sse",
			"url":     "http://127.0.0.1:9000/events",
			"headers": map[string]any{"X-Test": "value"},
		},
	})
	if err != nil {
		t.Fatalf("MergeAgentMCPServers() error = %v", err)
	}
	if len(merged) != 4 {
		t.Fatalf("merged servers = %#v, want four entries", merged)
	}
	if len(builtIn) != 1 {
		t.Fatalf("built-in server map was mutated: %#v", builtIn)
	}

	stdio, ok := merged["local_tools"].(sdkmcp.StdioServerConfig)
	if !ok || stdio.Command != "npx" || len(stdio.Args) != 2 || stdio.Env["TOKEN"] != "secret" {
		t.Fatalf("stdio config = %#v", merged["local_tools"])
	}
	httpServer, ok := merged["remote_http"].(sdkmcp.HTTPServerConfig)
	if !ok || httpServer.URL != "https://mcp.example.com/rpc" {
		t.Fatalf("http config = %#v", merged["remote_http"])
	}
	if httpServer.Headers["Authorization"] != "Bearer token" || httpServer.HeadersHelper != "/opt/nexus/header-helper" {
		t.Fatalf("http headers config = %#v", httpServer)
	}
	if httpServer.OAuth == nil || httpServer.OAuth.ClientID != "client-1" || httpServer.OAuth.CallbackPort != 43123 {
		t.Fatalf("http oauth config = %#v", httpServer.OAuth)
	}
	if httpServer.OAuth.XAA == nil || !*httpServer.OAuth.XAA {
		t.Fatalf("http oauth xaa config = %#v", httpServer.OAuth)
	}
	sse, ok := merged["remote_sse"].(sdkmcp.SSEServerConfig)
	if !ok || sse.URL != "http://127.0.0.1:9000/events" || sse.Headers["X-Test"] != "value" {
		t.Fatalf("sse config = %#v", merged["remote_sse"])
	}
}

func TestMergeAgentMCPServersRejectsManagedNames(t *testing.T) {
	tests := []struct {
		name       string
		builtIn    map[string]sdkmcp.ServerConfig
		configured map[string]any
		serverName string
	}{
		{
			name: "active built-in name",
			builtIn: map[string]sdkmcp.ServerConfig{
				"amap_maps": sdkmcp.HTTPServerConfig{URL: "https://mcp.amap.com/mcp"},
			},
			configured: map[string]any{
				"amap_maps": map[string]any{"command": "custom"},
			},
			serverName: "amap_maps",
		},
		{
			name: "reserved nexus namespace",
			configured: map[string]any{
				"nexus_shadow": map[string]any{"command": "custom"},
			},
			serverName: "nexus_shadow",
		},
		{
			name: "reserved connector name",
			configured: map[string]any{
				"amap_maps": map[string]any{"command": "custom"},
			},
			serverName: "amap_maps",
		},
		{
			name: "case insensitive active built-in name",
			builtIn: map[string]sdkmcp.ServerConfig{
				"ManagedTools": sdkmcp.HTTPServerConfig{URL: "https://managed.example.com/mcp"},
			},
			configured: map[string]any{
				"managedtools": map[string]any{"command": "custom"},
			},
			serverName: "managedtools",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := MergeAgentMCPServers(test.builtIn, test.configured)
			if err == nil || !strings.Contains(err.Error(), test.serverName) {
				t.Fatalf("MergeAgentMCPServers() error = %v, want named collision", err)
			}
		})
	}
}

func TestMergeAgentMCPServersRejectsUnstableNames(t *testing.T) {
	for _, name := range []string{" white-space", "with space", "line\nbreak", strings.Repeat("a", 65)} {
		_, err := MergeAgentMCPServers(nil, map[string]any{
			name: map[string]any{"command": "custom"},
		})
		if err == nil {
			t.Fatalf("server name %q should be rejected", name)
		}
	}
}

func TestMergeAgentMCPServersRejectsInvalidConfigWithServerName(t *testing.T) {
	tests := []struct {
		name       string
		configured any
	}{
		{name: "not an object", configured: []any{"npx"}},
		{name: "unknown field", configured: map[string]any{"command": "npx", "scope": "user"}},
		{name: "wrong args", configured: map[string]any{"command": "npx", "args": []any{1}}},
		{name: "unsupported sdk", configured: map[string]any{"type": "sdk", "name": "internal"}},
		{name: "invalid URL", configured: map[string]any{"type": "http", "url": "not-a-url"}},
		{name: "invalid OAuth metadata URL", configured: map[string]any{
			"type": "sse",
			"url":  "https://mcp.example.com/events",
			"oauth": map[string]any{
				"authServerMetadataUrl": "http://auth.example.com/metadata",
			},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := MergeAgentMCPServers(nil, map[string]any{"broken_server": test.configured})
			if err == nil {
				t.Fatal("MergeAgentMCPServers() error = nil")
			}
			if !strings.Contains(err.Error(), "broken_server") {
				t.Fatalf("error does not identify server: %v", err)
			}
		})
	}
}

func TestBuildAgentClientOptionsMergesAgentMCPServersBeforeRuntimeResolution(t *testing.T) {
	resolver := &countingMCPRuntimeResolver{}
	options, err := BuildAgentClientOptions(context.Background(), resolver, AgentClientOptionsInput{
		MCPServers: map[string]sdkmcp.ServerConfig{
			"managed": sdkmcp.HTTPServerConfig{URL: "https://managed.example.com/mcp"},
		},
		AgentMCPServers: map[string]any{
			"custom": map[string]any{"command": "custom-mcp"},
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions() error = %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("runtime resolver calls = %d, want 1", resolver.calls)
	}
	if len(options.MCP.Servers) != 2 {
		t.Fatalf("MCP servers = %#v, want merged servers", options.MCP.Servers)
	}
	if _, ok := options.MCP.Servers["custom"].(sdkmcp.StdioServerConfig); !ok {
		t.Fatalf("custom MCP server = %#v", options.MCP.Servers["custom"])
	}

	resolver.calls = 0
	_, err = BuildAgentClientOptions(context.Background(), resolver, AgentClientOptionsInput{
		AgentMCPServers: map[string]any{
			"broken": map[string]any{"type": "sdk"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("invalid Agent MCP error = %v", err)
	}
	if resolver.calls != 0 {
		t.Fatalf("invalid MCP config reached runtime resolver: calls=%d", resolver.calls)
	}
}
