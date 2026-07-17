// INPUT: automation 服务、Agent 身份与 runtime source context。
// OUTPUT: DM/Room 共用的 nexus_automation MCP builder。
// POS: automation MCP 的应用装配入口。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	automationmcp "github.com/nexus-research-lab/nexus/internal/mcp/automation"
	automationmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/service/agent"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

// newAutomationMCPBuilder 返回 DM/Room 实时链路所需的 MCPServerBuilder。
//
// 每次新建会话时按当前 (agentID, sessionKey, sourceContextType) 构造一个
// nexus_automation 进程内 MCP server，让主智能体可以通过工具自助管理定时任务。
// 在 dm 与 chat 包外部完成绑定，避免它们反向依赖 automation 子包导致 import cycle。
func newAutomationMCPBuilder(
	svc automationmcpcontract.Service,
	agents *agent.Service,
	defaultTimezone string,
) func(string, string, string, string, string, string, *atomic.Int64) map[string]sdkmcp.ServerConfig {
	return func(
		agentID string,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		_ *atomic.Int64,
	) map[string]sdkmcp.ServerConfig {
		sctx := automationmcpcontract.ServerContext{
			CurrentAgentID:      agentID,
			CurrentSessionKey:   sessionKey,
			CurrentSessionLabel: strings.TrimSpace(sourceContextLabel),
			SourceContextType:   sourceContextType,
			SourceContextID:     sourceContextID,
			SourceContextLabel:  sourceContextLabel,
			DefaultTimezone:     strings.TrimSpace(defaultTimezone),
		}
		if agents != nil && agentID != "" {
			if record, err := agents.GetAgent(context.Background(), agentID); err == nil && record != nil {
				sctx.CurrentAgentName = record.Name
				sctx.OwnerUserID = record.OwnerUserID
				sctx.IsMainAgent = record.IsMain
			}
		}
		return map[string]sdkmcp.ServerConfig{
			automationmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     automationmcpcontract.ServerName,
				Instance: automationmcp.NewServer(svc, sctx),
			},
		}
	}
}
