package server

import (
	"context"
	"net/url"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	connectormcp "github.com/nexus-research-lab/nexus/internal/mcp/connectors"
	connectormcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
	"github.com/nexus-research-lab/nexus/internal/service/agent"
)

// newConnectorMCPBuilder 返回 DM/Room 实时链路所需的 connector MCPServerBuilder。
func newConnectorMCPBuilder(
	svc connectormcpcontract.Service,
	agents *agent.Service,
) func(string, string, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		agentID string,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || agents == nil || strings.TrimSpace(agentID) == "" {
			return nil
		}
		record, err := agents.GetAgent(context.Background(), agentID)
		if err != nil || record == nil || strings.TrimSpace(record.OwnerUserID) == "" {
			return nil
		}
		sctx := connectormcpcontract.ServerContext{
			OwnerUserID:        record.OwnerUserID,
			CurrentAgentID:     agentID,
			CurrentSessionKey:  sessionKey,
			SourceContextType:  sourceContextType,
			SourceContextID:    sourceContextID,
			SourceContextLabel: sourceContextLabel,
			IsMainAgent:        record.IsMain,
		}
		servers := map[string]sdkmcp.ServerConfig{
			connectormcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     connectormcpcontract.ServerName,
				Instance: connectormcp.NewServer(svc, sctx),
			},
		}
		appendAmapMCPServer(context.Background(), servers, svc, record.OwnerUserID)
		appendDidiMCPServer(context.Background(), servers, svc, record.OwnerUserID)
		appendDingTalkAITableMCPServer(context.Background(), servers, svc, record.OwnerUserID)
		appendTencentDocsMCPServer(context.Background(), servers, svc, record.OwnerUserID)
		appendYuqueMCPServer(context.Background(), servers, svc, record.OwnerUserID)
		return servers
	}
}

func appendAmapMCPServer(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
) {
	appendAPIKeyMCPServer(ctx, servers, svc, ownerUserID, "amap", "amap_maps", "https://mcp.amap.com/mcp")
}

func appendDidiMCPServer(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
) {
	appendAPIKeyMCPServer(ctx, servers, svc, ownerUserID, "didi", "didi_ride", "https://mcp.didichuxing.com/mcp-servers")
}

func appendDingTalkAITableMCPServer(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
) {
	appendUserURLMCPServer(ctx, servers, svc, ownerUserID, "dingtalk-ai-table", "dingtalk_ai_table")
}

func appendTencentDocsMCPServer(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
) {
	appendHeaderTokenMCPServer(ctx, servers, svc, ownerUserID, "tencent-docs", "tencent_docs", "https://docs.qq.com/openapi/mcp", "Authorization")
}

func appendYuqueMCPServer(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
) {
	snapshot := loadConnectorMCPSnapshot(ctx, servers, svc, ownerUserID, "yuque")
	if snapshot == nil {
		return
	}
	servers["yuque"] = sdkmcp.StdioServerConfig{
		Command: "npx",
		Args:    []string{"-y", "yuque-mcp"},
		Env: map[string]string{
			"YUQUE_PERSONAL_TOKEN": strings.TrimSpace(snapshot.AccessToken),
		},
	}
}

func appendAPIKeyMCPServer(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
	connectorID string,
	serverName string,
	baseURL string,
) {
	snapshot := loadConnectorMCPSnapshot(ctx, servers, svc, ownerUserID, connectorID)
	if snapshot == nil {
		return
	}
	servers[serverName] = sdkmcp.HTTPServerConfig{
		URL: strings.TrimSpace(baseURL) + "?key=" + url.QueryEscape(snapshot.AccessToken),
	}
}

func appendUserURLMCPServer(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
	connectorID string,
	serverName string,
) {
	snapshot := loadConnectorMCPSnapshot(ctx, servers, svc, ownerUserID, connectorID)
	if snapshot == nil {
		return
	}
	serverURL := strings.TrimSpace(snapshot.AccessToken)
	parsedURL, err := url.Parse(serverURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
		return
	}
	servers[serverName] = sdkmcp.HTTPServerConfig{URL: serverURL}
}

func appendHeaderTokenMCPServer(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
	connectorID string,
	serverName string,
	serverURL string,
	headerName string,
) {
	snapshot := loadConnectorMCPSnapshot(ctx, servers, svc, ownerUserID, connectorID)
	if snapshot == nil {
		return
	}
	servers[serverName] = sdkmcp.HTTPServerConfig{
		URL: strings.TrimSpace(serverURL),
		Headers: map[string]string{
			headerName: strings.TrimSpace(snapshot.AccessToken),
		},
	}
}

func loadConnectorMCPSnapshot(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	svc connectormcpcontract.Service,
	ownerUserID string,
	connectorID string,
) *connectordomain.ConnectionSnapshot {
	if len(servers) == 0 || svc == nil || strings.TrimSpace(ownerUserID) == "" {
		return nil
	}
	snapshot, err := svc.LoadActiveConnection(ctx, ownerUserID, connectorID)
	if err != nil || snapshot == nil || strings.TrimSpace(snapshot.AccessToken) == "" {
		return nil
	}
	return snapshot
}

func combinedMCPBuilder(
	builders ...func(string, string, string, string, string, string) map[string]sdkmcp.ServerConfig,
) func(string, string, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		agentID string,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		merged := map[string]sdkmcp.ServerConfig{}
		for _, builder := range builders {
			if builder == nil {
				continue
			}
			for name, server := range builder(agentID, sessionKey, roundID, sourceContextType, sourceContextID, sourceContextLabel) {
				merged[name] = server
			}
		}
		return merged
	}
}

func contextOnlyMCPBuilder(
	builder func(string, string, string, string, string) map[string]sdkmcp.ServerConfig,
) func(string, string, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		agentID string,
		sessionKey string,
		_ string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		if builder == nil {
			return nil
		}
		return builder(agentID, sessionKey, sourceContextType, sourceContextID, sourceContextLabel)
	}
}
