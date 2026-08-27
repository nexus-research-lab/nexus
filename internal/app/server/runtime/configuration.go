// INPUT: 配置服务、可信 runtime 构造上下文与 loopback HTTP 请求。
// OUTPUT: 绑定当前 Agent/DM/Room round 的 nexuscfg 环境和配置命令响应。
// POS: nexuscfg CLI 与宿主 configuration 服务之间的本机 capability broker。
package runtime

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

const maxConfigurationRequestBytes = 1 << 20

type configurationRequest struct {
	Action    string                         `json:"action"`
	Domains   []string                       `json:"domains,omitempty"`
	Verify    bool                           `json:"verify,omitempty"`
	Change    configurationsvc.ChangeRequest `json:"change,omitempty"`
	Confirmed bool                           `json:"confirmed,omitempty"`
	Domain    string                         `json:"domain,omitempty"`
	Limit     int                            `json:"limit,omitempty"`
}

// NewConfigurationEnvironmentBuilder 为 runtime round 签发配置能力环境。
func NewConfigurationEnvironmentBuilder(
	cfg config.Config,
	svc *configurationsvc.Service,
	agents AgentResolver,
) func(context.Context, *protocol.Agent, string, string, string, string) (map[string]string, error) {
	endpoint := configurationBrokerURL(cfg)
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
		contextKind, _, surfaceOK := mcpSurfaceContext(agentID, sessionKey)
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
		populateTrustedConfigurationActor(
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

func populateTrustedConfigurationActor(
	ctx context.Context,
	actor *configurationsvc.Actor,
	sourceContextType string,
	sourceContextID string,
) {
	if actor == nil || sourceContextType != actor.ContextKind {
		return
	}
	role, authMethod, localSingleUser, ok := trustedPrincipal(ctx, actor.OwnerUserID)
	if !ok {
		return
	}
	conversationID, routeOK := trustedRoute(
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

func configurationBrokerURL(cfg config.Config) string {
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

// NewConfigurationHandler 处理 runtime loopback 配置请求。
func NewConfigurationHandler(
	svc *configurationsvc.Service,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !configurationLoopbackRequest(request) {
			writeConfigurationError(writer, http.StatusForbidden, "nexuscfg broker 只接受本机请求")
			return
		}
		actor, err := svc.ResolveRuntimeCapability(
			request.Header.Get(protocol.NexusConfigCapabilityHeader),
		)
		if err != nil {
			writeConfigurationError(writer, http.StatusUnauthorized, err.Error())
			return
		}
		var command configurationRequest
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxConfigurationRequestBytes))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&command); err != nil {
			writeConfigurationError(writer, http.StatusBadRequest, "请求参数错误")
			return
		}
		var extra any
		if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeConfigurationError(writer, http.StatusBadRequest, "请求参数错误")
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
			writeConfigurationError(writer, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeConfigurationJSON(writer, http.StatusOK, map[string]any{
			"success": true,
			"data":    result,
		})
	}
}

func configurationLoopbackRequest(request *http.Request) bool {
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

func writeConfigurationError(writer http.ResponseWriter, status int, message string) {
	writeConfigurationJSON(writer, status, map[string]any{
		"success": false,
		"error":   map[string]any{"message": strings.TrimSpace(message)},
	})
}

func writeConfigurationJSON(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}
