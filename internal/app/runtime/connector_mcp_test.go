package runtime

import (
	"context"
	"errors"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type connectorMCPServiceStub struct {
	snapshot *connectordomain.ConnectionSnapshot
}

func (stub connectorMCPServiceStub) LoadActiveConnection(
	context.Context,
	string,
	string,
) (*connectordomain.ConnectionSnapshot, error) {
	return stub.snapshot, nil
}

func TestConnectorBuilderMountsRichMailWithBearerToken(t *testing.T) {
	ctx := runtimectx.WithEnabledConnectorIDs(
		context.Background(),
		[]string{"richmail"},
	)
	builder := NewConnectorBuilder(connectorMCPServiceStub{
		snapshot: &connectordomain.ConnectionSnapshot{
			ConnectorID: "richmail",
			AccessToken: "rich-token",
		},
	})
	servers := builder(
		ctx,
		&protocol.Agent{AgentID: "agent-1", OwnerUserID: "owner-1"},
		"", "", "", "", "", nil, sdkpermission.Mode(""),
	)
	configValue, ok := servers["richmail"]
	if !ok {
		t.Fatal("已选择并连接的 RichMail 未挂载")
	}
	config, ok := configValue.(sdkmcp.HTTPServerConfig)
	if !ok {
		t.Fatalf("RichMail MCP 配置类型错误: %T", configValue)
	}
	if config.URL != "http://127.0.0.1:3100/mcp" ||
		config.Headers["Authorization"] != "Bearer rich-token" {
		t.Fatalf("RichMail MCP 配置错误: %+v", config)
	}
}

// The shared builder is used by both DM and Room runtimes.
func TestConnectorBuilderGitHubSelectionAndCredentials(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selected bool
		snapshot *connectordomain.ConnectionSnapshot
		err      error
		mounted  bool
	}{
		{name: "selected with OAuth token", selected: true, snapshot: &connectordomain.ConnectionSnapshot{AccessToken: "  gho-test-token  "}, mounted: true},
		{name: "not selected", snapshot: &connectordomain.ConnectionSnapshot{AccessToken: "gho-test-token"}},
		{name: "disconnected", selected: true},
		{name: "empty token", selected: true, snapshot: &connectordomain.ConnectionSnapshot{AccessToken: "  "}},
		{name: "load failed", selected: true, err: errors.New("connection unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &githubMCPServiceStub{snapshot: tc.snapshot, err: tc.err}
			ctx := context.Background()
			if tc.selected {
				ctx = runtimectx.WithEnabledConnectorIDs(ctx, []string{"github"})
			}
			servers := NewConnectorBuilder(svc)(ctx, &protocol.Agent{AgentID: "agent-1", OwnerUserID: "owner-1"}, "", "", "", "", "", nil, sdkpermission.Mode(""))
			value, mounted := servers["github"]
			if mounted != tc.mounted {
				t.Fatalf("mounted = %v, want %v", mounted, tc.mounted)
			}
			if tc.selected && (svc.owner != "owner-1" || svc.connector != "github") {
				t.Fatalf("connection lookup scope = %q/%q", svc.owner, svc.connector)
			}
			if !tc.selected && svc.owner != "" {
				t.Fatal("unselected connector loaded credentials")
			}
			if mounted {
				config, ok := value.(sdkmcp.HTTPServerConfig)
				if !ok {
					t.Fatalf("GitHub config type = %T", value)
				}
				if config.URL != "https://api.githubcopilot.com/mcp/" {
					t.Fatalf("URL = %q", config.URL)
				}
				if config.Headers["Authorization"] != "Bearer gho-test-token" {
					t.Fatal("GitHub bearer token not propagated")
				}
				if config.OAuth != nil {
					t.Fatal("existing authorization must not start a second OAuth flow")
				}
			}
		})
	}
}

type githubMCPServiceStub struct {
	snapshot         *connectordomain.ConnectionSnapshot
	err              error
	owner, connector string
}

func (s *githubMCPServiceStub) LoadActiveConnection(_ context.Context, owner, connector string) (*connectordomain.ConnectionSnapshot, error) {
	s.owner, s.connector = owner, connector
	return s.snapshot, s.err
}

func TestConnectorBuilderMountsAllBuiltInConnectors(t *testing.T) {
	for _, tc := range []struct{ id, alias, token, transport string }{
		{"github", "github", "oauth-token", "http"},
		{"richmail", "richmail", "paired-token", "http"},
		{"amap", "amap_maps", "map-key", "http"},
		{"didi", "didi_ride", "ride-key", "http"},
		{"dingtalk-ai-table", "dingtalk_ai_table", "https://mcp.dingtalk.com/private-test", "http"},
		{"tencent-docs", "tencent_docs", "docs-token", "http"},
		{"yuque", "yuque", "yuque-token", "stdio"},
		{"feishu-docx", "nexus_feishu_docx", "feishu-token", "sdk"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			ctx := runtimectx.WithEnabledConnectorIDs(context.Background(), []string{tc.id})
			svc := &githubMCPServiceStub{snapshot: &connectordomain.ConnectionSnapshot{ConnectorID: tc.id, AccessToken: tc.token}}
			servers := NewConnectorBuilder(svc)(ctx, &protocol.Agent{AgentID: "agent-1", OwnerUserID: "owner-1"}, "", "", "", "", "", nil, sdkpermission.Mode(""))
			if len(servers) != 1 {
				t.Fatalf("mounted server count = %d", len(servers))
			}
			value, ok := servers[tc.alias]
			if !ok {
				t.Fatalf("missing server %q", tc.alias)
			}
			switch tc.transport {
			case "http":
				config, ok := value.(sdkmcp.HTTPServerConfig)
				if !ok || config.URL == "" {
					t.Fatalf("invalid HTTP server: %T", value)
				}
			case "stdio":
				config, ok := value.(sdkmcp.StdioServerConfig)
				if !ok || config.Command == "" || config.Env["YUQUE_PERSONAL_TOKEN"] != tc.token {
					t.Fatal("invalid stdio server")
				}
			case "sdk":
				config, ok := value.(sdkmcp.SDKServerConfig)
				if !ok || config.Instance == nil {
					t.Fatal("invalid SDK server")
				}
			}
			if tc.transport != "sdk" && (svc.owner != "owner-1" || svc.connector != tc.id) {
				t.Fatal("incorrect credential scope")
			}
		})
	}
}

type customConnectorMCPStub struct {
	connectorMCPServiceStub
	owner, id string
}

func (s *customConnectorMCPStub) LoadActiveCustomMCPServer(_ context.Context, owner, id string) (string, map[string]any, error) {
	s.owner, s.id = owner, id
	if id != "custom-mcp:test" {
		return "", nil, nil
	}
	return "my_tools", map[string]any{"type": "http", "url": "https://example.invalid/mcp", "headers": map[string]any{"Authorization": "Bearer custom-token"}}, nil
}
func TestConnectorBuilderMountsSelectedCustomMCP(t *testing.T) {
	svc := &customConnectorMCPStub{}
	builder := NewConnectorBuilder(svc)
	agent := &protocol.Agent{AgentID: "agent-1", OwnerUserID: "owner-1"}
	ctx := runtimectx.WithEnabledConnectorIDs(context.Background(), []string{"custom-mcp:test"})
	servers := builder(ctx, agent, "", "", "", "", "", nil, sdkpermission.Mode(""))
	config, ok := servers["my_tools"].(sdkmcp.HTTPServerConfig)
	if !ok || config.Headers["Authorization"] != "Bearer custom-token" {
		t.Fatal("selected custom MCP not mounted with credentials")
	}
	if svc.owner != "owner-1" || svc.id != "custom-mcp:test" {
		t.Fatal("incorrect custom MCP scope")
	}
	servers = builder(context.Background(), agent, "", "", "", "", "", nil, sdkpermission.Mode(""))
	if len(servers) != 0 {
		t.Fatal("unselected custom MCP mounted")
	}
}
