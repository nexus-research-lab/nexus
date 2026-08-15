// INPUT: automation 服务、fresh Agent resolver、业务 source context 与真实 runtime lease。
// OUTPUT: 稳定 Session 工具表，以及按真实来源收紧的逐轮执行权限。
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
	agents runtimeAgentResolver,
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
		stableSurface := stableAutomationMCPSurface(record.AgentID, sessionKey)
		stableContextKind, _, stableNexusSurface := runtimeMCPSurfaceContext(
			record.AgentID,
			sessionKey,
		)
		sctx := automationmcpcontract.ServerContext{
			CurrentAgentID:           strings.TrimSpace(record.AgentID),
			CurrentAgentName:         strings.TrimSpace(record.Name),
			OwnerUserID:              strings.TrimSpace(record.OwnerUserID),
			CurrentSessionKey:        strings.TrimSpace(sessionKey),
			CurrentSessionLabel:      strings.TrimSpace(sourceContextLabel),
			SourceContextType:        sourceContextType,
			SourceContextID:          sourceContextID,
			SourceContextLabel:       strings.TrimSpace(sourceContextLabel),
			DefaultTimezone:          strings.TrimSpace(defaultTimezone),
			StableInteractiveSurface: stableSurface,
			// 高级 schema 属于主智能体 Nexus 私有 DM 的稳定工具面；
			// 真实 owner authority 仍要求当轮 SourceContextType=agent。
			IsMainAgent: record.IsMain && stableNexusSurface &&
				stableContextKind == "agent",
		}
		server := func() map[string]sdkmcp.ServerConfig {
			return map[string]sdkmcp.ServerConfig{
				automationmcpcontract.ServerName: sdkmcp.SDKServerConfig{
					Name:     automationmcpcontract.ServerName,
					Instance: automationmcp.NewServer(svc, sctx),
				},
			}
		}
		downgrade := func() map[string]sdkmcp.ServerConfig {
			if !sctx.StableInteractiveSurface {
				return nil
			}
			sctx.SourceContextType = strings.TrimSuffix(sourceContextType, "_untrusted") + "_untrusted"
			return server()
		}

		// 精确 agent/room 来源会暴露 mutation 工具，因此必须同时验证
		// fresh Agent、owner principal、结构化业务路由和当前 runtime lease。
		if sourceContextType == "agent" || sourceContextType == "room" {
			if _, _, _, ok := trustedRuntimePrincipal(ctx, record.OwnerUserID); !ok {
				return downgrade()
			}
			lease, ok := runtimectx.MCPRoundLeaseFromContext(ctx)
			if !ok {
				return downgrade()
			}
			switch sourceContextType {
			case "agent":
				if sourceContextID != record.AgentID {
					return downgrade()
				}
			case "room":
				if sourceContextID == "" {
					return downgrade()
				}
			}
			if _, ok = trustedRuntimeRoute(
				record.AgentID,
				sourceContextType,
				sessionKey,
				roundID,
				lease.SessionKey,
				lease.RoundID,
			); !ok {
				return downgrade()
			}
		}
		return server()
	}
}

func stableAutomationMCPSurface(agentID string, sessionKey string) bool {
	parsed := protocol.ParseSessionKey(strings.TrimSpace(sessionKey))
	if !parsed.IsStructured {
		return false
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return parsed.IsShared && strings.TrimSpace(parsed.ConversationID) != ""
	}
	return parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.ChatType == protocol.RoomTypeDM &&
		strings.TrimSpace(parsed.AgentID) == strings.TrimSpace(agentID)
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
