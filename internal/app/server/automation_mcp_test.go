package server

import (
	"context"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	automationmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

func TestAutomationMCPBuilderInjectsHostToolServer(t *testing.T) {
	builder := newAutomationMCPBuilder(nil, "Asia/Shanghai")
	servers := builder(
		context.Background(),
		&protocol.Agent{AgentID: "agent-1", OwnerUserID: "user-1"},
		"agent:agent-1:dm:main",
		"round-1",
		"agent",
		"agent-1",
		"主会话",
		nil,
		sdkpermission.ModeDefault,
	)
	config, ok := servers[automationmcpcontract.ServerName]
	if !ok {
		t.Fatalf("未注入 %s: %+v", automationmcpcontract.ServerName, servers)
	}
	sdkConfig, ok := config.(sdkmcp.SDKServerConfig)
	if !ok || sdkConfig.Instance == nil {
		t.Fatalf("automation 必须作为进程内 MCP server 注入: %+v", config)
	}
}
