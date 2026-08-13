// INPUT: automation 服务、fresh Agent resolver、业务 source context 与真实 runtime lease。
// OUTPUT: 可信 DM/Room 的写能力或后台/外部来源的当前 Agent 只读能力。
// POS: automation MCP 的应用鉴权与装配入口。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	automationmcp "github.com/nexus-research-lab/nexus/internal/mcp/automation"
	automationmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// newAutomationMCPBuilder 返回 DM/Room 实时链路所需的 MCPServerBuilder。
//
// 每次构造都从数据库重新读取 Agent。只有带同 owner 认证主体、结构化业务路由
// 和当前 runtime lease 的精确 agent/room 来源获得写工具；其他来源只保留当前
// Agent 的只读诊断工具。跨 Agent authority 只签发给主智能体自己的私有 DM。
func newAutomationMCPBuilder(
	svc automationmcpcontract.Service,
	agents configurationAgentResolver,
	defaultTimezone string,
) func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || agents == nil || agentValue == nil ||
			strings.TrimSpace(agentValue.AgentID) == "" {
			return nil
		}
		requestedAgentID := strings.TrimSpace(agentValue.AgentID)
		record, err := agents.GetAgent(ctx, requestedAgentID)
		if err != nil || record == nil ||
			strings.TrimSpace(record.AgentID) != requestedAgentID ||
			strings.TrimSpace(record.OwnerUserID) == "" {
			return nil
		}
		sessionKey = strings.TrimSpace(sessionKey)
		roundID = strings.TrimSpace(roundID)
		sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
		sourceContextID = strings.TrimSpace(sourceContextID)
		sctx := automationmcpcontract.ServerContext{
			CurrentAgentID:      strings.TrimSpace(record.AgentID),
			CurrentAgentName:    strings.TrimSpace(record.Name),
			OwnerUserID:         strings.TrimSpace(record.OwnerUserID),
			CurrentSessionKey:   strings.TrimSpace(sessionKey),
			CurrentSessionLabel: strings.TrimSpace(sourceContextLabel),
			SourceContextType:   sourceContextType,
			SourceContextID:     sourceContextID,
			SourceContextLabel:  strings.TrimSpace(sourceContextLabel),
			DefaultTimezone:     strings.TrimSpace(defaultTimezone),
		}

		// 精确 agent/room 来源会暴露 mutation 工具，因此必须同时验证
		// fresh Agent、owner principal、结构化业务路由和当前 runtime lease。
		if sourceContextType == "agent" || sourceContextType == "room" {
			if _, _, _, ok := trustedConfigurationPrincipal(ctx, record.OwnerUserID); !ok {
				return nil
			}
			lease, ok := runtimectx.MCPRoundLeaseFromContext(ctx)
			if !ok {
				return nil
			}
			switch sourceContextType {
			case "agent":
				if sourceContextID != record.AgentID {
					return nil
				}
			case "room":
				if sourceContextID == "" {
					return nil
				}
			}
			if _, ok = trustedConfigurationRuntimeRoute(
				record.AgentID,
				sourceContextType,
				sessionKey,
				roundID,
				lease.SessionKey,
				lease.RoundID,
			); !ok {
				return nil
			}
			// Room 中即使运行的是主智能体，也只能管理自身 Automation。
			// owner scope 的跨 Agent 能力只存在于主智能体自己的私有 DM。
			sctx.IsMainAgent = record.IsMain && sourceContextType == "agent"
		}
		return map[string]sdkmcp.ServerConfig{
			automationmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     automationmcpcontract.ServerName,
				Instance: automationmcp.NewServer(svc, sctx),
			},
		}
	}
}

// newAutomationExecutionMCPBuilder 为后台 run 覆盖通用 Automation MCP。
// 任务身份来自服务端签发的 ExecutionToolContext，不解析提示词或 session_key。
func newAutomationExecutionMCPBuilder(
	svc automationmcpcontract.Service,
	defaultTimezone string,
) runtimectx.ExecutionMCPServerBuilder {
	return func(
		_ context.Context,
		runtimeContext runtimectx.ExecutionToolContext,
	) map[string]sdkmcp.ServerConfig {
		binding := runtimeContext.AutomationRun
		agentValue := runtimeContext.Agent
		if svc == nil || binding == nil || agentValue == nil {
			return nil
		}
		normalized := binding.Normalized()
		if !normalized.Valid() ||
			strings.TrimSpace(agentValue.AgentID) == "" ||
			strings.TrimSpace(agentValue.OwnerUserID) == "" {
			return nil
		}
		sctx := automationmcpcontract.ServerContext{
			CurrentAgentID:     strings.TrimSpace(agentValue.AgentID),
			CurrentAgentName:   strings.TrimSpace(agentValue.Name),
			OwnerUserID:        strings.TrimSpace(agentValue.OwnerUserID),
			CurrentSessionKey:  strings.TrimSpace(runtimeContext.RuntimeSessionKey),
			SourceContextType:  "automation_run",
			SourceContextID:    normalized.JobID,
			SourceContextLabel: normalized.JobName,
			DefaultTimezone:    strings.TrimSpace(defaultTimezone),
			CurrentJobID:       normalized.JobID,
			CurrentRunID:       normalized.RunID,
		}
		return map[string]sdkmcp.ServerConfig{
			automationmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     automationmcpcontract.ServerName,
				Instance: automationmcp.NewServer(svc, sctx),
			},
		}
	}
}
