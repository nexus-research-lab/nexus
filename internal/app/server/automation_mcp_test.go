package server

import (
	"testing"

	automationmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

func TestAutomationMCPBuilderInjectsHostToolServer(t *testing.T) {
	builder := newAutomationMCPBuilder(nil, nil, "Asia/Shanghai")
	servers := builder(
		"agent-1",
		"agent:agent-1:dm:main",
		"round-1",
		"agent",
		"agent-1",
		"主会话",
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
