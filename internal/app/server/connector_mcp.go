// INPUT: connector 服务、Agent 身份与 runtime source context。
// OUTPUT: DM/Room 共用的 connector MCP builder。
// POS: connector MCP 的应用装配入口。
package server

import (
	"context"
	"net/url"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	connectormcp "github.com/nexus-research-lab/nexus/internal/mcp/connectors"
	connectormcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// newConnectorMCPBuilder 返回 DM/Room 实时链路所需的 connector MCPServerBuilder。
func newConnectorMCPBuilder(
	svc connectormcpcontract.Service,
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
		if svc == nil || agentValue == nil ||
			strings.TrimSpace(agentValue.AgentID) == "" ||
			strings.TrimSpace(agentValue.OwnerUserID) == "" {
			return nil
		}
		enabledConnectorIDs := normalizedConnectorIDs(runtimectx.EnabledConnectorIDs(ctx))
		if len(enabledConnectorIDs) == 0 {
			return nil
		}
		sctx := connectormcpcontract.ServerContext{
			OwnerUserID:         agentValue.OwnerUserID,
			CurrentAgentID:      agentValue.AgentID,
			CurrentSessionKey:   sessionKey,
			SourceContextType:   sourceContextType,
			SourceContextID:     sourceContextID,
			SourceContextLabel:  sourceContextLabel,
			IsMainAgent:         agentValue.IsMain,
			EnabledConnectorIDs: enabledConnectorIDs,
		}
		servers := map[string]sdkmcp.ServerConfig{
			connectormcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     connectormcpcontract.ServerName,
				Instance: connectormcp.NewServer(svc, sctx),
			},
		}
		for _, connectorID := range enabledConnectorIDs {
			switch connectorID {
			case "amap":
				appendAmapMCPServer(ctx, servers, svc, agentValue.OwnerUserID)
			case "didi":
				appendDidiMCPServer(ctx, servers, svc, agentValue.OwnerUserID)
			case "dingtalk-ai-table":
				appendDingTalkAITableMCPServer(ctx, servers, svc, agentValue.OwnerUserID)
			case "tencent-docs":
				appendTencentDocsMCPServer(ctx, servers, svc, agentValue.OwnerUserID)
			case "yuque":
				appendYuqueMCPServer(ctx, servers, svc, agentValue.OwnerUserID)
			}
		}
		return servers
	}
}

func normalizedConnectorIDs(requested []string) []string {
	if len(requested) == 0 {
		return nil
	}
	result := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, connectorID := range requested {
		connectorID = strings.TrimSpace(connectorID)
		if connectorID == "" {
			continue
		}
		if _, duplicate := seen[connectorID]; duplicate {
			continue
		}
		seen[connectorID] = struct{}{}
		result = append(result, connectorID)
	}
	return result
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
	builders ...func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig,
) func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		goalObjectiveRevision *atomic.Int64,
		permissionMode sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		merged := map[string]sdkmcp.ServerConfig{}
		for _, builder := range builders {
			if builder == nil {
				continue
			}
			for name, server := range builder(
				ctx,
				agentValue,
				sessionKey,
				roundID,
				sourceContextType,
				sourceContextID,
				sourceContextLabel,
				goalObjectiveRevision,
				permissionMode,
			) {
				merged[name] = server
			}
		}
		return merged
	}
}

func contextOnlyMCPBuilder(
	builder func(context.Context, *protocol.Agent, string, string, string, string) map[string]sdkmcp.ServerConfig,
) func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		_ string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		if builder == nil {
			return nil
		}
		return builder(
			ctx,
			agentValue,
			sessionKey,
			sourceContextType,
			sourceContextID,
			sourceContextLabel,
		)
	}
}

// roundContextMCPBuilder 适配不消费 Goal revision 的会话级 MCP builder。
func roundContextMCPBuilder(
	builder func(context.Context, *protocol.Agent, string, string, string, string, string) map[string]sdkmcp.ServerConfig,
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
		if builder == nil {
			return nil
		}
		return builder(
			ctx,
			agentValue,
			sessionKey,
			roundID,
			sourceContextType,
			sourceContextID,
			sourceContextLabel,
		)
	}
}
