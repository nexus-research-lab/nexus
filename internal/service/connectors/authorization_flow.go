// INPUT: 已持久化人工批准、当前 owner-main DM Actor 与 OAuth/Device 启动请求。
// OUTPUT: opaque flow_id、加密绑定本次 OAuth client 的跨 round 状态/取消与安全浏览器跳转。
// POS: Connector 对话授权的持久生命周期编排。
package connectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	connectorstore "github.com/nexus-research-lab/nexus/internal/storage/connectors"
)

type authorizationFlowSecret struct {
	AuthorizationURL  string `json:"authorization_url,omitempty"`
	DeviceCode        string `json:"device_code,omitempty"`
	ClientID          string `json:"client_id,omitempty"`
	ClientSecret      string `json:"client_secret,omitempty"`
	CommitOAuthClient bool   `json:"commit_oauth_client,omitempty"`
}

// Start 执行已经被真实 permission allow 精确批准的 OAuth/Device 启动意图。
func (c *AuthorizationControl) Start(
	ctx context.Context,
	actor AuthorizationActor,
	request AuthorizationStartRequest,
) (*AuthorizationFlowView, error) {
	resolved, err := c.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	request, entry, err := validateAuthorizationStartRequest(request)
	if err != nil {
		return nil, err
	}
	digest, err := authorizationIntentDigest(request)
	if err != nil {
		return nil, err
	}
	flow, err := c.flows.GetByRequest(
		ctx, resolved.OwnerUserID, request.RequestID,
	)
	if err != nil {
		return nil, err
	}
	if flow == nil {
		return nil, errors.New(
			"该 Connector 授权尚未获得当前会话真实人工允许；请确认本次授权",
		)
	}
	if err = validateFlowActor(flow, resolved, request.ConnectorID); err != nil {
		return nil, err
	}
	if flow.RootRoundID != resolved.RootRoundID ||
		flow.RuntimeLeaseSessionKey != resolved.RuntimeLeaseSessionKey ||
		flow.RuntimeLeaseRoundID != resolved.RuntimeLeaseRoundID ||
		flow.IntentDigest != digest ||
		flow.AuthorizationMethod != request.Method ||
		flow.DeviceMode != string(request.DeviceMode) {
		return nil, errors.New(
			"Connector 授权人工批准与当前 round、lease 或启动意图不匹配",
		)
	}
	if flow.Status != AuthorizationStatusApproved &&
		!(flow.Status == AuthorizationStatusPolling && flow.Stage == "") {
		return c.projectFlow(ctx, flow)
	}
	now := c.now()
	if !flow.ExpiresAt.After(now) {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusExpired, "授权启动确认已过期", now,
		)
		flow, _ = c.flows.Get(ctx, flow.OwnerUserID, flow.FlowID)
		return c.projectFlow(ctx, flow)
	}
	state, err := c.connectors.GetConfigurationState(
		ctx, flow.OwnerUserID, flow.ConnectorID,
	)
	if err != nil {
		return nil, err
	}
	if state.ConfigurationVersion != flow.StartConfigurationVersion {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusConflict,
			"Connector 配置已变化，请重新开始授权", now,
		)
		return nil, fmt.Errorf(
			"%w：Connector 授权启动版本已变化",
			ErrConfigurationConflict,
		)
	}
	claimed, err := c.flows.ClaimStart(
		ctx, flow.OwnerUserID, flow.FlowID, now,
		now.Add(authorizationDevicePollClaimLifetime),
	)
	if err != nil {
		return nil, err
	}
	if !claimed {
		current, loadErr := c.flows.Get(
			ctx, flow.OwnerUserID, flow.FlowID,
		)
		if loadErr != nil {
			return nil, loadErr
		}
		if current != nil && current.Stage != "" {
			return c.projectFlow(ctx, current)
		}
		return nil, errors.New(
			"Connector authorization 正由另一进程启动；请稍后用相同 request_id 重试",
		)
	}
	if request.Method == AuthorizationMethodOAuthBrowser {
		// 清理一次崩溃后过期 start claim 遗留、但从未向模型返回的 state。
		if err = c.connectors.deleteOAuthStatesForControlFlow(
			ctx, flow.OwnerUserID, flow.FlowID,
		); err != nil {
			_, _ = c.flows.MarkTerminal(
				ctx, flow.OwnerUserID, flow.FlowID,
				AuthorizationStatusFailed,
				"Connector OAuth state 清理失败", now,
			)
			return nil, err
		}
	}

	var activation connectorstore.AuthorizationFlowActivation
	switch request.Method {
	case AuthorizationMethodOAuthBrowser:
		activation, err = c.startOAuthBrowser(
			ctx, flow, request,
		)
	case AuthorizationMethodDevice:
		activation, err = c.startDevice(
			ctx, flow, entry, request.DeviceMode,
		)
	default:
		err = errors.New("Connector authorization method 无效")
	}
	if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector 授权启动失败", now,
		)
		return nil, authorizationProviderFailure("启动", err)
	}
	activated, err := c.flows.Activate(
		ctx, flow.OwnerUserID, flow.FlowID, activation, now,
	)
	if err != nil {
		if request.Method == AuthorizationMethodOAuthBrowser {
			_ = c.connectors.deleteOAuthStatesForControlFlow(
				ctx, flow.OwnerUserID, flow.FlowID,
			)
		}
		return nil, err
	}
	if !activated {
		if request.Method == AuthorizationMethodOAuthBrowser {
			_ = c.connectors.deleteOAuthStatesForControlFlow(
				ctx, flow.OwnerUserID, flow.FlowID,
			)
		}
	}
	flow, err = c.flows.Get(ctx, flow.OwnerUserID, flow.FlowID)
	if err != nil {
		return nil, err
	}
	if flow == nil {
		return nil, errors.New("Connector authorization flow 启动后丢失")
	}
	return c.projectFlow(ctx, flow)
}

// Status 读取状态；Device Flow 在达到 next_poll_at 后由服务端用加密 device_code
// 轮询并在成功时执行 Connector version CAS。
func (c *AuthorizationControl) Status(
	ctx context.Context,
	actor AuthorizationActor,
	ref AuthorizationFlowRef,
) (*AuthorizationFlowView, error) {
	resolved, err := c.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	flow, err := c.flowForActor(ctx, resolved, ref)
	if err != nil {
		return nil, err
	}
	now := c.now()
	if authorizationFlowIsActive(flow.Status) && !flow.ExpiresAt.After(now) {
		_, err = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusExpired, "Connector 授权已过期", now,
		)
		if err != nil {
			return nil, err
		}
		flow, err = c.flows.Get(ctx, flow.OwnerUserID, flow.FlowID)
		if err != nil {
			return nil, err
		}
		return c.projectFlow(ctx, flow)
	}
	if authorizationFlowIsActive(flow.Status) {
		state, stateErr := c.connectors.GetConfigurationState(
			ctx, flow.OwnerUserID, flow.ConnectorID,
		)
		if stateErr != nil {
			return nil, stateErr
		}
		if state.ConfigurationVersion != flow.ExpectedConfigurationVersion {
			_, err = c.flows.MarkTerminal(
				ctx, flow.OwnerUserID, flow.FlowID,
				AuthorizationStatusConflict,
				"Connector 配置已变化，请重新开始授权", now,
			)
			if err != nil {
				return nil, err
			}
			flow, err = c.flows.Get(ctx, flow.OwnerUserID, flow.FlowID)
			if err != nil {
				return nil, err
			}
			return c.projectFlow(ctx, flow)
		}
	}
	if flow.AuthorizationMethod == AuthorizationMethodDevice &&
		(flow.Status == AuthorizationStatusPending ||
			flow.Status == AuthorizationStatusPolling) {
		if err = c.pollDevice(ctx, flow); err != nil {
			return nil, err
		}
		flow, err = c.flows.Get(ctx, flow.OwnerUserID, flow.FlowID)
		if err != nil {
			return nil, err
		}
	}
	return c.projectFlow(ctx, flow)
}

// Cancel 撤销未完成流程并立即擦除加密 provider 临时凭据。
func (c *AuthorizationControl) Cancel(
	ctx context.Context,
	actor AuthorizationActor,
	ref AuthorizationFlowRef,
) (*AuthorizationFlowView, error) {
	resolved, err := c.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	flow, err := c.flowForActor(ctx, resolved, ref)
	if err != nil {
		return nil, err
	}
	switch flow.Status {
	case AuthorizationStatusCanceled:
		return c.projectFlow(ctx, flow)
	case AuthorizationStatusConnected, AuthorizationStatusExpired,
		AuthorizationStatusDenied, AuthorizationStatusConflict,
		AuthorizationStatusFailed:
		return nil, errors.New("已结束的 Connector 授权流程不能取消")
	}
	_, err = c.flows.Cancel(
		ctx, flow.OwnerUserID, flow.FlowID, c.now(),
	)
	if err != nil {
		return nil, err
	}
	_ = c.connectors.deleteOAuthStatesForControlFlow(
		ctx, flow.OwnerUserID, flow.FlowID,
	)
	flow, err = c.flows.Get(ctx, flow.OwnerUserID, flow.FlowID)
	if err != nil {
		return nil, err
	}
	return c.projectFlow(ctx, flow)
}

// ResolveAuthorizationRedirectActor 只从已认证 human principal 和 durable
// flow 绑定恢复浏览器打开身份。HTTP 路径只需携带 opaque flow_id，不能覆盖
// owner、Agent、业务 session、root round 或 runtime lease。
func (c *AuthorizationControl) ResolveAuthorizationRedirectActor(
	ctx context.Context,
	flowID string,
) (AuthorizationActor, error) {
	actor, _, err := c.resolveAuthorizationRedirectBinding(ctx, flowID)
	return actor, err
}

// GetAuthorizationRedirect 供受保护的宿主 GET 端点使用。它再次核对由 flow
// 恢复的完整 actor、真实 human principal 与当前主 Agent，记录 opened_at，
// 并只向浏览器返回 provider URL。
func (c *AuthorizationControl) GetAuthorizationRedirect(
	ctx context.Context,
	actor AuthorizationActor,
	flowID string,
) (string, error) {
	expectedActor, flow, err := c.resolveAuthorizationRedirectBinding(
		ctx, flowID,
	)
	if err != nil {
		return "", err
	}
	normalizeAuthorizationActor(&actor)
	if actor != expectedActor {
		return "", errors.New(
			"Connector authorization redirect 身份绑定不匹配",
		)
	}
	opened, err := c.flows.MarkOpened(
		ctx, expectedActor.OwnerUserID, flow.FlowID, c.now(),
	)
	if err != nil {
		return "", err
	}
	if !opened {
		return "", errors.New("Connector authorization redirect 已失效")
	}
	flow, err = c.flows.Get(
		ctx, expectedActor.OwnerUserID, flow.FlowID,
	)
	if err != nil {
		return "", err
	}
	if flow == nil ||
		flow.AuthorizationMethod != AuthorizationMethodOAuthBrowser ||
		flow.Status != AuthorizationStatusPending ||
		!flow.ExpiresAt.After(c.now()) {
		return "", errors.New("Connector authorization redirect 无效或已过期")
	}
	secret, err := c.decryptFlowSecret(flow)
	if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector 授权跳转凭据无法恢复", c.now(),
		)
		return "", err
	}
	parsed, err := url.Parse(strings.TrimSpace(secret.AuthorizationURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector 授权 provider URL 无效", c.now(),
		)
		return "", errors.New("Connector authorization provider URL 无效")
	}
	return parsed.String(), nil
}

func (c *AuthorizationControl) resolveAuthorizationRedirectBinding(
	ctx context.Context,
	flowID string,
) (AuthorizationActor, *connectorstore.AuthorizationFlow, error) {
	if err := c.requireReady(); err != nil {
		return AuthorizationActor{}, nil, err
	}
	if c.humanVerifier == nil {
		return AuthorizationActor{}, nil, errors.New(
			"Connector authorization 缺少 human verifier",
		)
	}
	principal, err := c.humanVerifier.VerifyInteractiveHuman(
		ctx, authctx.PrincipalFromContext(ctx),
	)
	if err != nil {
		return AuthorizationActor{}, nil, err
	}
	ownerUserID := strings.TrimSpace(principal.UserID)
	flowID = strings.TrimSpace(flowID)
	if ownerUserID == "" || flowID == "" {
		return AuthorizationActor{}, nil, errors.New(
			"Connector authorization redirect 身份或 flow 缺失",
		)
	}
	flow, err := c.flows.Get(ctx, ownerUserID, flowID)
	if err != nil {
		return AuthorizationActor{}, nil, err
	}
	if flow == nil ||
		flow.AuthorizationMethod != AuthorizationMethodOAuthBrowser ||
		flow.Status != AuthorizationStatusPending ||
		!flow.ExpiresAt.After(c.now()) {
		return AuthorizationActor{}, nil, errors.New(
			"Connector authorization redirect 无效或已过期",
		)
	}
	if err = validateFlowHumanPrincipal(c, ctx, flow, principal); err != nil {
		return AuthorizationActor{}, nil, err
	}
	authSessionID := ""
	if flow.HumanAuthSessionID.Valid {
		authSessionID = strings.TrimSpace(flow.HumanAuthSessionID.String)
	}
	actor := AuthorizationActor{
		OwnerUserID:            flow.OwnerUserID,
		AgentID:                flow.AgentID,
		BusinessSessionKey:     flow.BusinessSessionKey,
		RootRoundID:            flow.RootRoundID,
		RuntimeLeaseSessionKey: flow.RuntimeLeaseSessionKey,
		RuntimeLeaseRoundID:    flow.RuntimeLeaseRoundID,
		PrincipalUserID:        flow.HumanPrincipalUserID,
		PrincipalRole:          flow.HumanPrincipalRole,
		AuthMethod:             flow.HumanAuthMethod,
		AuthSessionID:          authSessionID,
	}
	normalizeAuthorizationActor(&actor)
	if actor.OwnerUserID == "" ||
		actor.PrincipalUserID != actor.OwnerUserID ||
		actor.BusinessSessionKey == "" ||
		actor.BusinessSessionKey != actor.RuntimeLeaseSessionKey ||
		actor.RootRoundID == "" ||
		actor.RootRoundID != actor.RuntimeLeaseRoundID {
		return AuthorizationActor{}, nil, errors.New(
			"Connector authorization redirect flow 绑定无效",
		)
	}
	parsed := protocol.ParseSessionKey(actor.BusinessSessionKey)
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		parsed.Channel != protocol.SessionChannelWebSocketSegment ||
		parsed.ChatType != protocol.RoomTypeDM ||
		strings.TrimSpace(parsed.AgentID) != actor.AgentID {
		return AuthorizationActor{}, nil, errors.New(
			"Connector authorization redirect 只允许主智能体 WebSocket 私有 DM",
		)
	}
	agentValue, err := c.agents.GetAgent(ctx, actor.AgentID)
	if err != nil {
		return AuthorizationActor{}, nil, fmt.Errorf(
			"重新验证 Connector authorization redirect Agent: %w",
			err,
		)
	}
	if agentValue == nil ||
		strings.TrimSpace(agentValue.OwnerUserID) != actor.OwnerUserID ||
		!agentValue.IsMain {
		return AuthorizationActor{}, nil, errors.New(
			"Connector authorization redirect 主智能体权限已变化",
		)
	}
	return actor, flow, nil
}

// CompletionRecord 返回统一审计层可持久引用的完成记录，不含流程秘密。
func (c *AuthorizationControl) CompletionRecord(
	ctx context.Context,
	ownerUserID string,
	flowID string,
) (*AuthorizationCompletionRecord, error) {
	if err := c.requireReady(); err != nil {
		return nil, err
	}
	flow, err := c.flows.Get(
		ctx, strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID),
	)
	if err != nil || flow == nil {
		return nil, err
	}
	result := &AuthorizationCompletionRecord{
		FlowID: flow.FlowID, OwnerUserID: flow.OwnerUserID,
		HumanPrincipalUserID: flow.HumanPrincipalUserID,
		HumanPrincipalRole:   flow.HumanPrincipalRole,
		HumanAuthMethod:      flow.HumanAuthMethod,
		PermissionRequestID:  flow.PermissionRequestID,
		RequestID:            flow.RequestID, AgentID: flow.AgentID,
		BusinessSessionKey:     flow.BusinessSessionKey,
		RootRoundID:            flow.RootRoundID,
		RuntimeLeaseSessionKey: flow.RuntimeLeaseSessionKey,
		RuntimeLeaseRoundID:    flow.RuntimeLeaseRoundID,
		ConnectorID:            flow.ConnectorID, Method: flow.AuthorizationMethod,
		StartConfigurationVersion: flow.StartConfigurationVersion,
		Status:                    flow.Status, HumanApprovedAt: flow.HumanApprovedAt,
	}
	if flow.CompletedConfigurationVersion.Valid {
		value := flow.CompletedConfigurationVersion.Int64
		result.CompletedConfigurationVersion = &value
	}
	if flow.CompletedAt.Valid {
		value := flow.CompletedAt.Time
		result.CompletedAt = &value
	}
	return result, nil
}

func (c *AuthorizationControl) flowForActor(
	ctx context.Context,
	actor *resolvedAuthorizationActor,
	ref AuthorizationFlowRef,
) (*connectorstore.AuthorizationFlow, error) {
	ref.FlowID = strings.TrimSpace(ref.FlowID)
	ref.ConnectorID = strings.TrimSpace(ref.ConnectorID)
	if ref.FlowID == "" || ref.ConnectorID == "" {
		return nil, errors.New(
			"Connector authorization status/cancel 需要 flow_id 和 connector_id",
		)
	}
	flow, err := c.flows.Get(ctx, actor.OwnerUserID, ref.FlowID)
	if err != nil {
		return nil, err
	}
	if flow == nil {
		return nil, errors.New("Connector authorization flow 不存在")
	}
	if err = validateFlowActor(flow, actor, ref.ConnectorID); err != nil {
		return nil, err
	}
	return flow, nil
}

func validateFlowActor(
	flow *connectorstore.AuthorizationFlow,
	actor *resolvedAuthorizationActor,
	connectorID string,
) error {
	if flow == nil || actor == nil {
		return errors.New("Connector authorization flow/actor 缺失")
	}
	authSessionID := ""
	if flow.HumanAuthSessionID.Valid {
		authSessionID = strings.TrimSpace(flow.HumanAuthSessionID.String)
	}
	// Start 会额外核对原始 root round/runtime lease，确保人工允许只消费一次。
	// Status/Cancel 则允许同一真人登录会话在同一主智能体私聊的后续
	// active round 恢复 durable flow，不能要求已经结束的原 round 仍存活。
	if flow.OwnerUserID != actor.OwnerUserID ||
		flow.AgentID != actor.AgentID ||
		flow.BusinessSessionKey != actor.BusinessSessionKey ||
		flow.RuntimeLeaseSessionKey != actor.RuntimeLeaseSessionKey ||
		flow.HumanPrincipalUserID != actor.PrincipalUserID ||
		flow.HumanPrincipalRole != actor.PrincipalRole ||
		flow.HumanAuthMethod != actor.AuthMethod ||
		authSessionID != actor.AuthSessionID {
		return errors.New(
			"Connector authorization flow 不属于当前 principal、主智能体或私有 DM",
		)
	}
	if flow.ConnectorID != strings.TrimSpace(connectorID) {
		return errors.New("Connector authorization flow 与 connector_id 不匹配")
	}
	return nil
}

func validateFlowHumanPrincipal(
	control *AuthorizationControl,
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
	principal *authctx.Principal,
) error {
	if control == nil || flow == nil || principal == nil {
		return errors.New("Connector authorization human principal 缺失")
	}
	sessionID := ""
	if principal.SessionID != nil {
		sessionID = strings.TrimSpace(*principal.SessionID)
	}
	storedSessionID := ""
	if flow.HumanAuthSessionID.Valid {
		storedSessionID = strings.TrimSpace(flow.HumanAuthSessionID.String)
	}
	if strings.TrimSpace(principal.UserID) != flow.HumanPrincipalUserID ||
		strings.TrimSpace(principal.Role) != flow.HumanPrincipalRole ||
		strings.TrimSpace(principal.AuthMethod) != flow.HumanAuthMethod ||
		sessionID != storedSessionID {
		return errors.New(
			"Connector authorization redirect principal 与人工批准不匹配",
		)
	}
	if control.roleResolver != nil {
		role, err := control.roleResolver.ResolveActivePrincipalRole(
			ctx, flow.HumanPrincipalUserID,
		)
		if err != nil {
			return err
		}
		if strings.TrimSpace(role) != flow.HumanPrincipalRole {
			return errors.New(
				"Connector authorization principal 角色已变化",
			)
		}
	}
	return nil
}

func (c *AuthorizationControl) startOAuthBrowser(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
	request AuthorizationStartRequest,
) (connectorstore.AuthorizationFlowActivation, error) {
	result, err := c.connectors.getAuthURL(
		ctx, flow.OwnerUserID, flow.ConnectorID, "",
		request.Extras, flow.FlowID,
	)
	if err != nil {
		return connectorstore.AuthorizationFlowActivation{}, err
	}
	encrypted, err := c.encryptFlowSecret(authorizationFlowSecret{
		AuthorizationURL: result.AuthURL,
	})
	if err != nil {
		_ = c.connectors.deleteOAuthStatesForControlFlow(
			ctx, flow.OwnerUserID, flow.FlowID,
		)
		return connectorstore.AuthorizationFlowActivation{}, err
	}
	apiPrefix := "/" + strings.Trim(
		c.connectors.config.APIPrefix, "/",
	)
	if apiPrefix == "/" {
		apiPrefix = ""
	}
	return connectorstore.AuthorizationFlowActivation{
		ExpectedConfigurationVersion: flow.StartConfigurationVersion,
		Stage:                        "awaiting_browser",
		SecretEncrypted:              encrypted,
		PublicOpenPath: apiPrefix +
			"/connectors/authorization-flows/" +
			url.PathEscape(flow.FlowID) + "/open",
		ExpiresAt: c.now().Add(c.connectors.oauthStateTTL()),
	}, nil
}

func (c *AuthorizationControl) startDevice(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
	entry CatalogEntry,
	mode DeviceAuthStartMode,
) (connectorstore.AuthorizationFlowActivation, error) {
	var (
		started      *DeviceAuthStartResult
		clientID     string
		clientSecret string
		err          error
	)
	if entry.AutoOAuthClient {
		switch mode {
		case DeviceAuthStartModeOfficialQR:
			started, err = c.connectors.startFeishuAppRegistration(ctx, entry)
		case DeviceAuthStartModeManualCredentials:
			clientID, clientSecret, err = c.connectors.oauthCredentials(
				ctx, flow.OwnerUserID, entry.ConnectorID,
			)
			if err == nil {
				started, err = c.connectors.startOAuthDeviceAuth(
					ctx, entry, clientID, clientSecret,
				)
			}
		default:
			err = errors.New("Device Flow mode 无效")
		}
	} else {
		clientID, err = c.connectors.oauthPublicClientID(
			ctx, flow.OwnerUserID, entry.ConnectorID, entry.Title,
		)
		if err == nil {
			started, err = c.connectors.startOAuthDeviceAuthWithPublicClient(
				ctx, entry, clientID,
			)
		}
	}
	if err != nil {
		return connectorstore.AuthorizationFlowActivation{}, err
	}
	if started == nil || strings.TrimSpace(started.DeviceCode) == "" {
		return connectorstore.AuthorizationFlowActivation{}, errors.New(
			"Device provider 未返回临时授权凭据",
		)
	}
	encrypted, err := c.encryptFlowSecret(authorizationFlowSecret{
		DeviceCode:   strings.TrimSpace(started.DeviceCode),
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
	})
	if err != nil {
		return connectorstore.AuthorizationFlowActivation{}, err
	}
	interval := normalizedDevicePollInterval(started.Interval)
	expiresIn := normalizedDeviceExpiry(started.ExpiresIn)
	now := c.now()
	return connectorstore.AuthorizationFlowActivation{
		ExpectedConfigurationVersion: flow.StartConfigurationVersion,
		Stage: connectorFirstNonEmpty(
			started.Stage, deviceAuthStageUserAuthorization,
		),
		SecretEncrypted:       encrypted,
		PublicUserCode:        strings.TrimSpace(started.UserCode),
		PublicVerificationURI: strings.TrimSpace(started.VerificationURI),
		PublicVerificationURIComplete: strings.TrimSpace(
			started.VerificationURIComplete,
		),
		PollIntervalSeconds: interval,
		NextPollAt: sql.NullTime{
			Time: now.Add(time.Duration(interval) * time.Second), Valid: true,
		},
		ExpiresAt: now.Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func (s *Service) startOAuthDeviceAuthWithPublicClient(
	ctx context.Context,
	entry CatalogEntry,
	clientID string,
) (*DeviceAuthStartResult, error) {
	provider, err := s.deviceProvider(entry)
	if err != nil {
		return nil, err
	}
	return s.startOAuthDeviceAuthWithProvider(
		ctx, entry, provider, clientID, "",
	)
}

func (c *AuthorizationControl) projectFlow(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
) (*AuthorizationFlowView, error) {
	if flow == nil {
		return nil, errors.New("Connector authorization flow 不存在")
	}
	state, err := c.connectors.GetConfigurationState(
		ctx, flow.OwnerUserID, flow.ConnectorID,
	)
	if err != nil {
		return nil, err
	}
	status := flow.Status
	if status == AuthorizationStatusPolling {
		status = AuthorizationStatusPending
	}
	view := &AuthorizationFlowView{
		FlowID: flow.FlowID, ConnectorID: flow.ConnectorID,
		Method: flow.AuthorizationMethod, Status: status,
		Stage:                       flow.Stage,
		Message:                     flow.ResultMessage,
		StartConfigurationVersion:   flow.StartConfigurationVersion,
		CurrentConfigurationVersion: state.ConfigurationVersion,
		ExpiresAt:                   flow.ExpiresAt,
	}
	if authorizationFlowIsActive(flow.Status) {
		view.UserCode = flow.PublicUserCode
		view.VerificationURI = flow.PublicVerificationURI
		view.VerificationURIComplete = flow.PublicVerificationURIComplete
		view.PollAfterSeconds = flow.PollIntervalSeconds
	}
	if flow.AuthorizationMethod == AuthorizationMethodOAuthBrowser &&
		flow.Status == AuthorizationStatusPending {
		view.AuthorizationURL = flow.PublicOpenPath
	}
	if flow.CompletedAt.Valid {
		value := flow.CompletedAt.Time
		view.CompletedAt = &value
	}
	return view, nil
}

func (c *AuthorizationControl) encryptFlowSecret(
	secret authorizationFlowSecret,
) (string, error) {
	payload, err := json.Marshal(secret)
	if err != nil {
		return "", err
	}
	return c.flows.EncryptSecret(payload)
}

func (c *AuthorizationControl) decryptFlowSecret(
	flow *connectorstore.AuthorizationFlow,
) (authorizationFlowSecret, error) {
	if flow == nil {
		return authorizationFlowSecret{}, errors.New(
			"Connector authorization flow 缺失",
		)
	}
	payload, err := c.flows.DecryptSecret(flow.SecretEncrypted)
	if err != nil {
		return authorizationFlowSecret{}, err
	}
	var secret authorizationFlowSecret
	if err = json.Unmarshal(payload, &secret); err != nil {
		return authorizationFlowSecret{}, errors.New(
			"Connector authorization 加密临时凭据无效",
		)
	}
	return secret, nil
}

func authorizationFlowIsActive(status string) bool {
	return status == AuthorizationStatusApproved ||
		status == AuthorizationStatusPending ||
		status == AuthorizationStatusPolling
}

func normalizedDevicePollInterval(interval int) int {
	if interval < 5 {
		return 5
	}
	if interval > 60 {
		return 60
	}
	return interval
}

func normalizedDeviceExpiry(expiresIn int) int {
	if expiresIn <= 0 {
		return 600
	}
	if expiresIn > 3600 {
		return 3600
	}
	return expiresIn
}

func authorizationProviderFailure(action string, _ error) error {
	return fmt.Errorf(
		"Connector authorization %s失败；provider 秘密和响应未写入模型或日志",
		strings.TrimSpace(action),
	)
}
