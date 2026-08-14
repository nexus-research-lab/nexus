// INPUT: Connector authorization control、当前主 Agent 记录、认证 principal 与 runtime round lease。
// OUTPUT: owner-main WebSocket 私有 DM 稳定 MCP builder，以及只接收 opaque flow_id 的安全浏览器跳转路由。
// POS: Connector 对话授权的应用层接线边界；模型和 URL 都不能构造或覆盖授权身份。
package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	connectorauthorizationmcp "github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization"
	connectorauthorizationcontract "github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"

	"github.com/go-chi/chi/v5"
)

const connectorAuthorizationOpenRoute = "/connectors/authorization-flows/{flow_id}/open"

type connectorAuthorizationRedirectControl interface {
	ResolveAuthorizationRedirectActor(
		context.Context,
		string,
	) (connectorsvc.AuthorizationActor, error)
	GetAuthorizationRedirect(
		context.Context,
		connectorsvc.AuthorizationActor,
		string,
	) (string, error)
}

// newConnectorAuthorizationMCPBuilder 只向当前 owner 主智能体的 WebSocket
// 私有 DM 注入授权工具。全部身份字段来自 fresh Agent/principal/lease 记录。
func newConnectorAuthorizationMCPBuilder(
	svc connectorauthorizationcontract.Service,
	agents configurationAgentResolver,
) func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || agents == nil || agentValue == nil {
			return nil
		}
		agentID := strings.TrimSpace(agentValue.AgentID)
		sessionKey = strings.TrimSpace(sessionKey)
		roundID = strings.TrimSpace(roundID)
		sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
		sourceContextID = strings.TrimSpace(sourceContextID)
		contextKind, _, surfaceOK := runtimeMCPSurfaceContext(agentID, sessionKey)
		if agentID == "" || sessionKey == "" || roundID == "" ||
			!surfaceOK || contextKind != "agent" {
			return nil
		}
		lease, ok := runtimectx.MCPRoundLeaseFromContext(ctx)
		if !ok {
			return nil
		}
		record, err := agents.GetAgent(ctx, agentID)
		if err != nil ||
			record == nil ||
			strings.TrimSpace(record.AgentID) != agentID ||
			strings.TrimSpace(record.OwnerUserID) == "" ||
			!record.IsMain {
			return nil
		}
		sctx := connectorauthorizationcontract.ServerContext{
			OwnerUserID:            strings.TrimSpace(record.OwnerUserID),
			CurrentAgentID:         agentID,
			BusinessSessionKey:     sessionKey,
			RootRoundID:            roundID,
			RuntimeLeaseSessionKey: strings.TrimSpace(lease.SessionKey),
			RuntimeLeaseRoundID:    strings.TrimSpace(lease.RoundID),
			ContextKind:            contextKind,
			IsMainAgent:            true,
		}
		server := func() map[string]sdkmcp.ServerConfig {
			return map[string]sdkmcp.ServerConfig{
				connectorauthorizationcontract.ServerName: sdkmcp.SDKServerConfig{
					Name: connectorauthorizationcontract.ServerName,
					Instance: connectorauthorizationmcp.NewServer(
						svc, sctx,
					),
				},
			}
		}
		if sourceContextType != contextKind || sourceContextID != agentID {
			return server()
		}
		role, authMethod, localSingleUser, ok := trustedConfigurationPrincipal(
			ctx, record.OwnerUserID,
		)
		if !ok {
			return server()
		}
		principal := authctx.PrincipalFromContext(ctx)
		if principal == nil ||
			strings.TrimSpace(principal.UserID) != record.OwnerUserID {
			return server()
		}
		authSessionID := ""
		if principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
		switch authMethod {
		case authctx.AuthMethodPassword:
			if authSessionID == "" {
				return server()
			}
		case authctx.AuthMethodLocal:
			evidence, hasEvidence := authctx.InteractiveHumanEvidenceFromContext(ctx)
			if !localSingleUser ||
				!hasEvidence ||
				evidence.Source != "desktop_session_token" {
				return server()
			}
		default:
			// Bearer/runtime principals cannot satisfy the human-presence
			// requirement of Connector authorization.
			return server()
		}
		if _, routeOK := trustedConfigurationRuntimeRoute(
			record.AgentID,
			sourceContextType,
			sessionKey,
			roundID,
			lease.SessionKey,
			lease.RoundID,
		); !routeOK {
			return server()
		}
		sctx.PrincipalUserID = strings.TrimSpace(principal.UserID)
		sctx.PrincipalRole = role
		sctx.AuthMethod = authMethod
		sctx.AuthSessionID = authSessionID
		return server()
	}
}

// mountConnectorAuthorizationRoutes 挂载受认证浏览器打开端点。prefixPath
// 由 Server 提供，因此该组件不依赖全局 routes/app_services 装配细节。
func mountConnectorAuthorizationRoutes(
	router chi.Router,
	prefixPath func(string) string,
	control connectorAuthorizationRedirectControl,
) {
	if router == nil || prefixPath == nil {
		return
	}
	router.Get(
		prefixPath(connectorAuthorizationOpenRoute),
		newConnectorAuthorizationOpenHandler(control),
	)
}

// newConnectorAuthorizationOpenHandler 只消费 path flow_id；owner、Agent、
// session、round 与 lease 均由 control 从认证 principal 和 durable flow 恢复。
func newConnectorAuthorizationOpenHandler(
	control connectorAuthorizationRedirectControl,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		setConnectorAuthorizationOpenHeaders(writer)
		if request == nil ||
			authctx.PrincipalFromContext(request.Context()) == nil {
			http.Error(
				writer,
				"Connector 授权链接需要已认证的人类会话",
				http.StatusUnauthorized,
			)
			return
		}
		flowID := strings.TrimSpace(chi.URLParam(request, "flow_id"))
		if control == nil || flowID == "" || len(flowID) > 128 {
			writeConnectorAuthorizationOpenFailure(writer)
			return
		}
		actor, err := control.ResolveAuthorizationRedirectActor(
			request.Context(), flowID,
		)
		if err != nil {
			writeConnectorAuthorizationOpenFailure(writer)
			return
		}
		redirectURL, err := control.GetAuthorizationRedirect(
			request.Context(), actor, flowID,
		)
		if err != nil {
			writeConnectorAuthorizationOpenFailure(writer)
			return
		}
		parsed, err := url.Parse(strings.TrimSpace(redirectURL))
		if err != nil ||
			parsed.Scheme != "https" ||
			parsed.Host == "" {
			writeConnectorAuthorizationOpenFailure(writer)
			return
		}
		writer.Header().Set("Location", parsed.String())
		writer.WriteHeader(http.StatusSeeOther)
	}
}

func setConnectorAuthorizationOpenHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Pragma", "no-cache")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
}

func writeConnectorAuthorizationOpenFailure(writer http.ResponseWriter) {
	http.Error(
		writer,
		"Connector 授权链接无效或已过期",
		http.StatusNotFound,
	)
}
