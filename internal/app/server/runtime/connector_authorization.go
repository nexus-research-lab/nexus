// INPUT: Connector authorization control、当前主 Agent 记录、认证 principal 与 runtime round lease。
// OUTPUT: owner-main WebSocket 私有 DM 稳定授权工具，以及只接收 opaque flow_id 的安全浏览器跳转路由。
// POS: Connector 对话授权的应用层接线边界；模型和 URL 都不能构造或覆盖授权身份。
package runtime

import (
	"context"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"net/http"
	"net/url"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	connectorauthorizationmcp "github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization"
	connectorauthorizationcontract "github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"

	"github.com/go-chi/chi/v5"
)

const connectorAuthorizationOpenRoute = "/connectors/authorization-flows/{flow_id}/open"

// AuthorizationRedirectControl 解析已绑定身份的 Connector 授权跳转。
type AuthorizationRedirectControl interface {
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

// NewConnectorAuthorizationToolBuilder 只向当前 owner 主智能体的 WebSocket
// 私有 DM 注入授权工具。全部身份字段来自 fresh Agent/principal/lease 记录。
func NewConnectorAuthorizationToolBuilder(
	svc connectorauthorizationcontract.Service,
	agents AgentResolver,
) ToolBuilder {
	return func(ctx context.Context, round nexusmcp.RoundContext) []sdktool.Tool {
		agentValue := round.CommandContext.Agent
		sessionKey := round.SessionKey
		roundID := round.RoundID
		sourceContextType := round.SourceContextType
		sourceContextID := round.SourceContextID
		if svc == nil || agents == nil || agentValue == nil {
			return nil
		}
		agentID := strings.TrimSpace(agentValue.AgentID)
		sessionKey = strings.TrimSpace(sessionKey)
		roundID = strings.TrimSpace(roundID)
		sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
		sourceContextID = strings.TrimSpace(sourceContextID)
		contextKind, _, surfaceOK := mcpSurfaceContext(agentID, sessionKey)
		if agentID == "" || sessionKey == "" || roundID == "" ||
			!surfaceOK || contextKind != "agent" {
			return nil
		}
		lease, ok := runtimectx.RuntimeRoundLeaseFromContext(ctx)
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
		tools := func() []sdktool.Tool {
			return connectorauthorizationmcp.BuildTools(svc, sctx)
		}
		if sourceContextType != contextKind || sourceContextID != agentID {
			return tools()
		}
		role, authMethod, localSingleUser, ok := trustedPrincipal(
			ctx, record.OwnerUserID,
		)
		if !ok {
			return tools()
		}
		principal := authctx.PrincipalFromContext(ctx)
		if principal == nil ||
			strings.TrimSpace(principal.UserID) != record.OwnerUserID {
			return tools()
		}
		authSessionID := ""
		if principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
		switch authMethod {
		case authctx.AuthMethodPassword:
			if authSessionID == "" {
				return tools()
			}
		case authctx.AuthMethodLocal:
			evidence, hasEvidence := authctx.InteractiveHumanEvidenceFromContext(ctx)
			if !localSingleUser ||
				!hasEvidence ||
				evidence.Source != "desktop_session_token" {
				return tools()
			}
		default:
			// Bearer/runtime principals cannot satisfy the human-presence
			// requirement of Connector authorization.
			return tools()
		}
		if _, routeOK := trustedRoute(
			record.AgentID,
			sourceContextType,
			sessionKey,
			roundID,
			lease.SessionKey,
			lease.RoundID,
		); !routeOK {
			return tools()
		}
		sctx.PrincipalUserID = strings.TrimSpace(principal.UserID)
		sctx.PrincipalRole = role
		sctx.AuthMethod = authMethod
		sctx.AuthSessionID = authSessionID
		return tools()
	}
}

// MountConnectorAuthorizationRoutes 挂载受认证浏览器打开端点。prefixPath
// 由 Server 提供，因此该组件不依赖全局 routes/app_services 装配细节。
func MountConnectorAuthorizationRoutes(
	router chi.Router,
	prefixPath func(string) string,
	control AuthorizationRedirectControl,
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
	control AuthorizationRedirectControl,
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
