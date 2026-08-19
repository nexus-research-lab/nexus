// INPUT: 配置服务、可信 runtime 构造上下文与 loopback HTTP 请求。
// OUTPUT: 绑定当前 Agent/DM/Room round 的 nexuscfg 环境和配置命令响应。
// POS: nexuscfg CLI 与宿主 configuration 服务之间的本机 capability broker。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

const maxRuntimeConfigurationRequestBytes = 1 << 20

type runtimeConfigurationRequest struct {
	Action    string                         `json:"action"`
	Domains   []string                       `json:"domains,omitempty"`
	Verify    bool                           `json:"verify,omitempty"`
	Change    configurationsvc.ChangeRequest `json:"change,omitempty"`
	Confirmed bool                           `json:"confirmed,omitempty"`
	Domain    string                         `json:"domain,omitempty"`
	Limit     int                            `json:"limit,omitempty"`
}

func newConfigurationRuntimeEnvironmentBuilder(
	cfg config.Config,
	svc *configurationsvc.Service,
	agents runtimeAgentResolver,
) func(context.Context, *protocol.Agent, string, string, string, string) (map[string]string, error) {
	endpoint := configurationRuntimeBrokerURL(cfg)
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
	) (map[string]string, error) {
		if svc == nil || agents == nil || agentValue == nil {
			return nil, nil
		}
		agentID := strings.TrimSpace(agentValue.AgentID)
		contextKind, _, surfaceOK := runtimeMCPSurfaceContext(agentID, sessionKey)
		lease, hasLease := runtimectx.RuntimeRoundLeaseFromContext(ctx)
		if agentID == "" || !surfaceOK || !hasLease {
			return nil, nil
		}
		record, err := agents.GetAgent(ctx, agentID)
		if err != nil || record == nil || strings.TrimSpace(record.OwnerUserID) == "" {
			return nil, err
		}
		actor := configurationsvc.Actor{
			OwnerUserID: strings.TrimSpace(record.OwnerUserID),
			AgentID:     agentID, SessionKey: strings.TrimSpace(sessionKey),
			RoundID:         strings.TrimSpace(roundID),
			LeaseSessionKey: strings.TrimSpace(lease.SessionKey),
			LeaseRoundID:    strings.TrimSpace(lease.RoundID),
			IsMainAgent:     record.IsMain, ContextKind: contextKind,
			SourceContext:      strings.Trim(strings.ToLower(strings.TrimSpace(sourceContextType))+":"+strings.TrimSpace(sourceContextID), ":"),
			RoundLeaseRequired: true,
		}
		populateTrustedRuntimeConfigurationActor(
			ctx,
			&actor,
			strings.ToLower(strings.TrimSpace(sourceContextType)),
			strings.TrimSpace(sourceContextID),
		)
		token, err := svc.IssueRuntimeCapability(actor)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			protocol.NexusConfigBrokerURLEnvName:       endpoint,
			protocol.NexusConfigCapabilityTokenEnvName: token,
		}, nil
	}
}

func populateTrustedRuntimeConfigurationActor(
	ctx context.Context,
	actor *configurationsvc.Actor,
	sourceContextType string,
	sourceContextID string,
) {
	if actor == nil || sourceContextType != actor.ContextKind {
		return
	}
	role, authMethod, localSingleUser, ok := trustedRuntimePrincipal(ctx, actor.OwnerUserID)
	if !ok {
		return
	}
	conversationID, routeOK := trustedRuntimeRoute(
		actor.AgentID,
		actor.ContextKind,
		actor.SessionKey,
		actor.RoundID,
		actor.LeaseSessionKey,
		actor.LeaseRoundID,
	)
	if !routeOK ||
		(actor.ContextKind == configurationsvc.ContextKindAgent && sourceContextID != actor.AgentID) ||
		(actor.ContextKind == configurationsvc.ContextKindRoom && sourceContextID == "") {
		return
	}
	actor.ContextID = sourceContextID
	actor.ConversationID = conversationID
	actor.PrincipalRole = role
	actor.AuthMethod = authMethod
	actor.LocalSingleUser = localSingleUser
	if principal := authctx.PrincipalFromContext(ctx); principal != nil && principal.SessionID != nil {
		actor.AuthSessionID = strings.TrimSpace(*principal.SessionID)
	}
	if queued, hasQueuedBinding := authctx.QueuedHumanPrincipalBindingFromContext(ctx); hasQueuedBinding {
		if strings.TrimSpace(queued.UserID) != actor.OwnerUserID {
			actor.ContextID = ""
			return
		}
		actor.PrincipalRole = authctx.RoleMember
		actor.AuthMethod = queued.AuthMethod
		actor.AuthSessionID = queued.SessionID
		actor.LocalSingleUser = false
	}
	if actor.ContextKind == configurationsvc.ContextKindRoom {
		actor.RoomID = sourceContextID
	}
}

func configurationRuntimeBrokerURL(cfg config.Config) string {
	prefix := "/" + strings.Trim(strings.TrimSpace(cfg.APIPrefix), "/")
	if prefix == "/" {
		prefix = ""
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port)),
		Path:   prefix + "/internal/runtime/configuration",
	}).String()
}

func newRuntimeConfigurationHandler(
	svc *configurationsvc.Service,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !runtimeConfigurationLoopbackRequest(request) {
			writeRuntimeConfigurationError(writer, http.StatusForbidden, "nexuscfg broker 只接受本机请求")
			return
		}
		actor, err := svc.ResolveRuntimeCapability(
			request.Header.Get(protocol.NexusConfigCapabilityHeader),
		)
		if err != nil {
			writeRuntimeConfigurationError(writer, http.StatusUnauthorized, err.Error())
			return
		}
		var command runtimeConfigurationRequest
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxRuntimeConfigurationRequestBytes))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&command); err != nil {
			writeRuntimeConfigurationError(writer, http.StatusBadRequest, "请求参数错误")
			return
		}
		var extra any
		if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeRuntimeConfigurationError(writer, http.StatusBadRequest, "请求参数错误")
			return
		}
		var result any
		switch strings.ToLower(strings.TrimSpace(command.Action)) {
		case "inspect":
			result, err = svc.Inspect(request.Context(), actor, command.Domains, command.Verify)
		case "plan":
			result, err = svc.PlanChange(request.Context(), actor, command.Change)
		case "apply":
			result, err = svc.ApplyChangeFromCLI(
				request.Context(),
				actor,
				command.Change,
				configurationsvc.CLIApplyOptions{Confirmed: command.Confirmed},
			)
		case "history":
			result, err = svc.ListChanges(request.Context(), actor, command.Domain, command.Limit)
		default:
			err = errors.New("未知 nexuscfg broker action")
		}
		if err != nil {
			writeRuntimeConfigurationError(writer, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeRuntimeConfigurationJSON(writer, http.StatusOK, map[string]any{
			"success": true,
			"data":    result,
		})
	}
}

func runtimeConfigurationLoopbackRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func writeRuntimeConfigurationError(writer http.ResponseWriter, status int, message string) {
	writeRuntimeConfigurationJSON(writer, status, map[string]any{
		"success": false,
		"error":   map[string]any{"message": strings.TrimSpace(message)},
	})
}

func writeRuntimeConfigurationJSON(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}
