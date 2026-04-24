package bootstrap

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/agent"
	automationmcp "github.com/nexus-research-lab/nexus/internal/automation/mcp"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

// newAutomationMCPBuilder 返回 chat/room 实时链路所需的 MCPServerBuilder。
//
// 每次新建会话时按当前 (agentID, sessionKey, sourceContextType) 构造一个
// nexus_automation 进程内 MCP server，让主智能体可以通过工具自助管理定时任务。
// 在 chat 与 room 包外部完成绑定，避免它们反向依赖 automation 子包导致 import cycle。
func newAutomationMCPBuilder(svc automationmcp.Service, agents *agent.Service, defaultTimezone string) func(string, string, string) map[string]agentclient.SDKMCPServer {
	return func(agentID, sessionKey, sourceContextType string) map[string]agentclient.SDKMCPServer {
		sctx := automationmcp.ServerContext{
			CurrentAgentID:    agentID,
			CurrentSessionKey: sessionKey,
			SourceContextType: sourceContextType,
			DefaultTimezone:   strings.TrimSpace(defaultTimezone),
		}
		if agents != nil && agentID != "" {
			if record, err := agents.GetAgent(context.Background(), agentID); err == nil && record != nil {
				sctx.CurrentAgentName = record.Name
				sctx.IsMainAgent = record.IsMain
			}
		}
		return map[string]agentclient.SDKMCPServer{
			automationmcp.ServerName: automationmcp.NewServer(svc, sctx),
		}
	}
}
