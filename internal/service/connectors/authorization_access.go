// INPUT: runtime/permission 固化身份、当前 Agent/round 数据与授权启动参数。
// OUTPUT: owner-main 私有 DM 动态鉴权、durable 人工批准与精确意图绑定。
// POS: Connector 对话授权的唯一身份边界；flow 工具参数不能覆盖 owner/Agent/session。
package connectors

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	connectorstore "github.com/nexus-research-lab/nexus/internal/storage/connectors"
)

const maximumAuthorizationApprovalLifetime = 2 * time.Minute

var authorizationRequestIDPattern = regexp.MustCompile(
	`^[a-zA-Z0-9][a-zA-Z0-9._:-]{7,127}$`,
)

type resolvedAuthorizationActor struct {
	AuthorizationActor
	Agent *protocol.Agent
}

// IsConnectorAuthorizationStartTool 判断 permission 请求是否为授权启动叶工具。
func IsConnectorAuthorizationStartTool(toolName string) bool {
	toolName = strings.TrimSpace(toolName)
	if toolName == StartConnectorAuthorizationToolName {
		return true
	}
	for _, separator := range []string{"__", ".", "/"} {
		if strings.HasSuffix(
			toolName,
			separator+StartConnectorAuthorizationToolName,
		) {
			return true
		}
	}
	return false
}

// RecordHumanToolApproval 在真实 WebSocket permission allow 后持久保存完整意图。
// runtime 必须在执行 start_connector_authorization 之前调用本方法。
func (c *AuthorizationControl) RecordHumanToolApproval(
	ctx context.Context,
	approval permissionctx.HumanToolApproval,
) error {
	if err := c.requireReady(); err != nil {
		return err
	}
	if !IsConnectorAuthorizationStartTool(approval.ToolName) {
		return errors.New("人工批准工具与 Connector authorization start 不匹配")
	}
	if strings.TrimSpace(approval.PermissionRequestID) == "" {
		return errors.New("Connector authorization 人工批准缺少 permission request_id")
	}
	if c.humanVerifier == nil {
		return errors.New("Connector authorization 缺少 human principal verifier")
	}
	principal, err := c.humanVerifier.VerifyInteractiveHuman(
		ctx, authctx.PrincipalFromContext(ctx),
	)
	if err != nil {
		return err
	}
	ctx = authctx.WithPrincipal(ctx, principal)
	request, err := authorizationStartRequestFromToolInput(approval.ToolInput)
	if err != nil {
		return err
	}
	request, _, err = validateAuthorizationStartRequest(request)
	if err != nil {
		return err
	}
	actor, err := authorizationActorFromApproval(principal, approval)
	if err != nil {
		return err
	}
	resolved, err := c.resolveActor(ctx, actor)
	if err != nil {
		return err
	}
	digest, err := authorizationIntentDigest(request)
	if err != nil {
		return err
	}
	state, err := c.connectors.GetConfigurationState(
		ctx, resolved.OwnerUserID, request.ConnectorID,
	)
	if err != nil {
		return err
	}

	now := c.now()
	expiresAt := approval.ExpiresAt.UTC()
	maximumExpiry := now.Add(maximumAuthorizationApprovalLifetime)
	if expiresAt.IsZero() || expiresAt.After(maximumExpiry) {
		expiresAt = maximumExpiry
	}
	if !expiresAt.After(now) {
		return errors.New("Connector authorization 人工批准已过期")
	}
	sessionID := sql.NullString{}
	if principal.SessionID != nil && strings.TrimSpace(*principal.SessionID) != "" {
		sessionID = sql.NullString{
			String: strings.TrimSpace(*principal.SessionID), Valid: true,
		}
	}
	flowID, err := newAuthorizationFlowID()
	if err != nil {
		return err
	}
	record := connectorstore.AuthorizationFlow{
		FlowID: flowID, OwnerUserID: resolved.OwnerUserID,
		HumanPrincipalUserID: resolved.PrincipalUserID,
		HumanPrincipalRole:   resolved.PrincipalRole,
		HumanAuthMethod:      resolved.AuthMethod,
		HumanAuthSessionID:   sessionID,
		PermissionRequestID:  strings.TrimSpace(approval.PermissionRequestID),
		RequestID:            request.RequestID, AgentID: resolved.AgentID,
		BusinessSessionKey:           resolved.BusinessSessionKey,
		RootRoundID:                  resolved.RootRoundID,
		RuntimeLeaseSessionKey:       resolved.RuntimeLeaseSessionKey,
		RuntimeLeaseRoundID:          resolved.RuntimeLeaseRoundID,
		ConnectorID:                  request.ConnectorID,
		AuthorizationMethod:          request.Method,
		DeviceMode:                   string(request.DeviceMode),
		IntentDigest:                 digest,
		StartConfigurationVersion:    state.ConfigurationVersion,
		ExpectedConfigurationVersion: state.ConfigurationVersion,
		Status:                       AuthorizationStatusApproved,
		HumanApprovedAt:              now, ExpiresAt: expiresAt,
	}
	stored, err := c.flows.CreateApproved(ctx, record)
	if err != nil {
		return err
	}
	if !sameAuthorizationApproval(stored, &record) {
		return errors.New(
			"request_id 已绑定另一项 Connector 授权；请生成新的 request_id",
		)
	}
	return nil
}

func (c *AuthorizationControl) resolveActor(
	ctx context.Context,
	actor AuthorizationActor,
) (*resolvedAuthorizationActor, error) {
	if err := c.requireReady(); err != nil {
		return nil, err
	}
	normalizeAuthorizationActor(&actor)
	if actor.OwnerUserID == "" || actor.AgentID == "" ||
		actor.PrincipalUserID == "" {
		return nil, errors.New("Connector authorization 缺少可信 owner/Agent/principal")
	}
	principal := authctx.PrincipalFromContext(ctx)
	if principal == nil ||
		strings.TrimSpace(principal.UserID) != actor.OwnerUserID ||
		strings.TrimSpace(principal.UserID) != actor.PrincipalUserID ||
		strings.TrimSpace(principal.AuthMethod) != actor.AuthMethod {
		return nil, errors.New(
			"Connector authorization 当前认证 principal 与 runtime 身份不匹配",
		)
	}
	currentSessionID := ""
	if principal.SessionID != nil {
		currentSessionID = strings.TrimSpace(*principal.SessionID)
	}
	if currentSessionID != actor.AuthSessionID {
		return nil, errors.New(
			"Connector authorization 当前认证 session 与批准主体不匹配",
		)
	}
	if c.roleResolver != nil {
		role, err := c.roleResolver.ResolveActivePrincipalRole(
			ctx, actor.PrincipalUserID,
		)
		if err != nil {
			return nil, fmt.Errorf("重新验证 Connector authorization principal: %w", err)
		}
		if strings.TrimSpace(role) != actor.PrincipalRole {
			return nil, errors.New(
				"Connector authorization principal 角色已变化；请重新开始授权",
			)
		}
	}
	if !authorizationRoundIsActive(
		c.runtime,
		actor.RuntimeLeaseSessionKey,
		actor.RuntimeLeaseRoundID,
	) {
		return nil, errors.New(
			"Connector authorization 调用所属 runtime round 已结束或不再可信",
		)
	}
	if actor.BusinessSessionKey != actor.RuntimeLeaseSessionKey ||
		actor.RootRoundID != actor.RuntimeLeaseRoundID {
		return nil, errors.New(
			"Connector authorization 私有 DM 业务身份与 runtime lease 不匹配",
		)
	}
	parsed := protocol.ParseSessionKey(actor.BusinessSessionKey)
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		parsed.Channel != protocol.SessionChannelWebSocketSegment ||
		parsed.ChatType != protocol.RoomTypeDM ||
		strings.TrimSpace(parsed.AgentID) != actor.AgentID {
		return nil, errors.New(
			"Connector authorization 只允许主智能体的 WebSocket 私有 DM",
		)
	}
	agentValue, err := c.agents.GetAgent(ctx, actor.AgentID)
	if err != nil {
		return nil, fmt.Errorf("重新验证 Connector authorization Agent: %w", err)
	}
	if agentValue == nil ||
		strings.TrimSpace(agentValue.OwnerUserID) != actor.OwnerUserID ||
		!agentValue.IsMain {
		return nil, errors.New(
			"Connector authorization 只允许当前 owner 的主智能体私有 DM",
		)
	}
	return &resolvedAuthorizationActor{
		AuthorizationActor: actor, Agent: agentValue,
	}, nil
}

func authorizationActorFromApproval(
	principal *authctx.Principal,
	approval permissionctx.HumanToolApproval,
) (AuthorizationActor, error) {
	if principal == nil {
		return AuthorizationActor{}, errors.New(
			"Connector authorization 人工批准缺少认证 principal",
		)
	}
	if strings.TrimSpace(approval.Route.RoomID) != "" ||
		strings.TrimSpace(approval.Route.ConversationID) != "" {
		return AuthorizationActor{}, errors.New(
			"Connector authorization 不允许从 Room 获得人工批准",
		)
	}
	agentID := strings.TrimSpace(approval.Route.AgentID)
	if agentID == "" {
		agentID = protocol.ParseSessionKey(approval.RuntimeSessionKey).AgentID
	}
	authorizationActor := AuthorizationActor{
		OwnerUserID:            principal.UserID,
		AgentID:                agentID,
		BusinessSessionKey:     approval.DispatchSessionKey,
		RootRoundID:            approval.Route.RoundID,
		RuntimeLeaseSessionKey: approval.RuntimeSessionKey,
		RuntimeLeaseRoundID:    approval.Route.RoundID,
		PrincipalUserID:        principal.UserID,
		PrincipalRole:          principal.Role,
		AuthMethod:             principal.AuthMethod,
	}
	if principal.SessionID != nil {
		authorizationActor.AuthSessionID = *principal.SessionID
	}
	normalizeAuthorizationActor(&authorizationActor)
	if authorizationActor.OwnerUserID == "" {
		return AuthorizationActor{}, errors.New(
			"Connector authorization 人工批准缺少认证 principal",
		)
	}
	if authorizationActor.AgentID == "" ||
		authorizationActor.BusinessSessionKey == "" ||
		authorizationActor.RootRoundID == "" ||
		authorizationActor.RuntimeLeaseSessionKey == "" {
		return AuthorizationActor{}, errors.New(
			"Connector authorization 人工批准缺少可信 DM session/round",
		)
	}
	return authorizationActor, nil
}

func normalizeAuthorizationActor(actor *AuthorizationActor) {
	if actor == nil {
		return
	}
	actor.OwnerUserID = strings.TrimSpace(actor.OwnerUserID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	actor.BusinessSessionKey = strings.TrimSpace(actor.BusinessSessionKey)
	actor.RootRoundID = strings.TrimSpace(actor.RootRoundID)
	actor.RuntimeLeaseSessionKey = strings.TrimSpace(
		actor.RuntimeLeaseSessionKey,
	)
	actor.RuntimeLeaseRoundID = strings.TrimSpace(actor.RuntimeLeaseRoundID)
	actor.PrincipalUserID = strings.TrimSpace(actor.PrincipalUserID)
	actor.PrincipalRole = strings.TrimSpace(actor.PrincipalRole)
	actor.AuthMethod = strings.TrimSpace(actor.AuthMethod)
	actor.AuthSessionID = strings.TrimSpace(actor.AuthSessionID)
}

func authorizationRoundIsActive(
	runtime authorizationRoundVerifier,
	sessionKey string,
	roundID string,
) bool {
	if runtime == nil || strings.TrimSpace(sessionKey) == "" ||
		strings.TrimSpace(roundID) == "" {
		return false
	}
	for _, running := range runtime.GetRunningRoundIDs(sessionKey) {
		if running == roundID {
			return true
		}
	}
	return false
}

func validateAuthorizationStartRequest(
	request AuthorizationStartRequest,
) (AuthorizationStartRequest, CatalogEntry, error) {
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.ConnectorID = strings.TrimSpace(request.ConnectorID)
	request.Method = strings.ToLower(strings.TrimSpace(request.Method))
	request.DeviceMode = DeviceAuthStartMode(
		strings.ToLower(strings.TrimSpace(string(request.DeviceMode))),
	)
	if !authorizationRequestIDPattern.MatchString(request.RequestID) {
		return AuthorizationStartRequest{}, CatalogEntry{}, errors.New(
			"Connector authorization request_id 必须为 8-128 位稳定幂等 ID",
		)
	}
	entry, ok := getConnector(request.ConnectorID)
	if !ok || entry.Status != "available" || entry.AuthType != "oauth2" {
		return AuthorizationStartRequest{}, CatalogEntry{}, errors.New(
			"目标 Connector 不存在、不可用或不支持 OAuth",
		)
	}
	switch request.Method {
	case AuthorizationMethodOAuthBrowser:
		if request.DeviceMode != "" {
			return AuthorizationStartRequest{}, CatalogEntry{}, errors.New(
				"浏览器 OAuth 不接受 device_mode",
			)
		}
		_, provider, err := availableOAuthProvider(request.ConnectorID)
		if err != nil {
			return AuthorizationStartRequest{}, CatalogEntry{}, err
		}
		request.Extras, err = validatedAuthorizationExtras(provider, request.Extras)
		if err != nil {
			return AuthorizationStartRequest{}, CatalogEntry{}, err
		}
	case AuthorizationMethodDevice:
		if !entry.DeviceAuth {
			return AuthorizationStartRequest{}, CatalogEntry{}, errors.New(
				"目标 Connector 不支持 Device Flow",
			)
		}
		if len(request.Extras) != 0 {
			return AuthorizationStartRequest{}, CatalogEntry{}, errors.New(
				"Device Flow 不接受 extras",
			)
		}
		request.Extras = nil
		if entry.AutoOAuthClient {
			switch request.DeviceMode {
			case DeviceAuthStartModeOfficialQR,
				DeviceAuthStartModeManualCredentials:
			default:
				return AuthorizationStartRequest{}, CatalogEntry{}, errors.New(
					"该 Connector 必须明确选择 official_qr 或 manual_credentials",
				)
			}
		} else if request.DeviceMode != "" {
			return AuthorizationStartRequest{}, CatalogEntry{}, errors.New(
				"该 Connector 不接受 device_mode",
			)
		}
	default:
		return AuthorizationStartRequest{}, CatalogEntry{}, errors.New(
			"Connector authorization method 只允许 oauth_browser 或 device",
		)
	}
	return request, entry, nil
}

func validatedAuthorizationExtras(
	provider interface {
		RequiredExtraKeys() []string
	},
	extras map[string]string,
) (map[string]string, error) {
	if len(extras) > 16 {
		return nil, errors.New("Connector authorization extras 过多")
	}
	normalized := make(map[string]string, len(extras))
	for rawKey, rawValue := range extras {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		value := strings.TrimSpace(rawValue)
		if key == "" || len(key) > 64 || len(value) > 512 {
			return nil, errors.New("Connector authorization extra 格式无效")
		}
		for _, forbidden := range []string{
			"secret", "token", "password", "credential",
			"device_code", "auth_code", "state", "verifier",
		} {
			if strings.Contains(key, forbidden) {
				return nil, errors.New(
					"Connector authorization extras 不接受秘密字段",
				)
			}
		}
		normalized[key] = value
	}
	for _, required := range provider.RequiredExtraKeys() {
		if strings.TrimSpace(normalized[required]) == "" {
			return nil, fmt.Errorf("%s 参数缺失", required)
		}
	}
	return normalized, nil
}

func authorizationStartRequestFromToolInput(
	input map[string]any,
) (AuthorizationStartRequest, error) {
	request := AuthorizationStartRequest{
		RequestID:   stringToolInput(input, "request_id"),
		ConnectorID: stringToolInput(input, "connector_id"),
		Method:      stringToolInput(input, "method"),
		DeviceMode:  DeviceAuthStartMode(stringToolInput(input, "device_mode")),
	}
	rawExtras, exists := input["extras"]
	if !exists || rawExtras == nil {
		return request, nil
	}
	extrasMap, ok := rawExtras.(map[string]any)
	if !ok {
		if typed, typedOK := rawExtras.(map[string]string); typedOK {
			request.Extras = typed
			return request, nil
		}
		return AuthorizationStartRequest{}, errors.New(
			"Connector authorization extras 必须是字符串对象",
		)
	}
	request.Extras = make(map[string]string, len(extrasMap))
	for key, raw := range extrasMap {
		value, valueOK := raw.(string)
		if !valueOK {
			return AuthorizationStartRequest{}, errors.New(
				"Connector authorization extra 值必须是字符串",
			)
		}
		request.Extras[key] = value
	}
	return request, nil
}

func stringToolInput(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	value, _ := input[key].(string)
	return strings.TrimSpace(value)
}

func authorizationIntentDigest(request AuthorizationStartRequest) (string, error) {
	payload, err := json.Marshal(struct {
		RequestID   string              `json:"request_id"`
		ConnectorID string              `json:"connector_id"`
		Method      string              `json:"method"`
		DeviceMode  DeviceAuthStartMode `json:"device_mode,omitempty"`
		Extras      map[string]string   `json:"extras,omitempty"`
	}{
		RequestID:   request.RequestID,
		ConnectorID: request.ConnectorID,
		Method:      request.Method,
		DeviceMode:  request.DeviceMode,
		Extras:      request.Extras,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func newAuthorizationFlowID() (string, error) {
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "caf_" + base64.RawURLEncoding.EncodeToString(random), nil
}

func sameAuthorizationApproval(
	left *connectorstore.AuthorizationFlow,
	right *connectorstore.AuthorizationFlow,
) bool {
	if left == nil || right == nil {
		return false
	}
	return left.OwnerUserID == right.OwnerUserID &&
		left.HumanPrincipalUserID == right.HumanPrincipalUserID &&
		left.HumanPrincipalRole == right.HumanPrincipalRole &&
		left.HumanAuthMethod == right.HumanAuthMethod &&
		left.HumanAuthSessionID == right.HumanAuthSessionID &&
		left.PermissionRequestID == right.PermissionRequestID &&
		left.RequestID == right.RequestID &&
		left.AgentID == right.AgentID &&
		left.BusinessSessionKey == right.BusinessSessionKey &&
		left.RootRoundID == right.RootRoundID &&
		left.RuntimeLeaseSessionKey == right.RuntimeLeaseSessionKey &&
		left.RuntimeLeaseRoundID == right.RuntimeLeaseRoundID &&
		left.ConnectorID == right.ConnectorID &&
		left.AuthorizationMethod == right.AuthorizationMethod &&
		left.DeviceMode == right.DeviceMode &&
		left.IntentDigest == right.IntentDigest &&
		left.StartConfigurationVersion == right.StartConfigurationVersion &&
		left.ExpectedConfigurationVersion == right.ExpectedConfigurationVersion &&
		left.Status == right.Status
}
