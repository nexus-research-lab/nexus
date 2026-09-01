package runtime

import (
	"context"
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
