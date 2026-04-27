package server

import (
	"context"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	connectormcp "github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors"
	connectormcpcontract "github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
	"github.com/nexus-research-lab/nexus/internal/service/agent"
)

// newConnectorMCPBuilder 返回 DM/Room 实时链路所需的 connector MCPServerBuilder。
func newConnectorMCPBuilder(svc connectormcpcontract.Service, agents *agent.Service) func(string, string, string) map[string]agentclient.SDKMCPServer {
	return func(agentID, sessionKey, sourceContextType string) map[string]agentclient.SDKMCPServer {
		if svc == nil || agents == nil || strings.TrimSpace(agentID) == "" {
			return nil
		}
		record, err := agents.GetAgent(context.Background(), agentID)
		if err != nil || record == nil || strings.TrimSpace(record.OwnerUserID) == "" {
			return nil
		}
		sctx := connectormcpcontract.ServerContext{
			OwnerUserID:       record.OwnerUserID,
			CurrentAgentID:    agentID,
			CurrentSessionKey: sessionKey,
			SourceContextType: sourceContextType,
			IsMainAgent:       record.IsMain,
		}
		return map[string]agentclient.SDKMCPServer{
			connectormcpcontract.ServerName: connectormcp.NewServer(svc, sctx),
		}
	}
}

func combinedMCPBuilder(builders ...func(string, string, string) map[string]agentclient.SDKMCPServer) func(string, string, string) map[string]agentclient.SDKMCPServer {
	return func(agentID, sessionKey, sourceContextType string) map[string]agentclient.SDKMCPServer {
		merged := map[string]agentclient.SDKMCPServer{}
		for _, builder := range builders {
			if builder == nil {
				continue
			}
			for name, server := range builder(agentID, sessionKey, sourceContextType) {
				merged[name] = server
			}
		}
		return merged
	}
}
