// INPUT: 加密 device_code/本次 OAuth client、callback state 与启动时 Connector version。
// OUTPUT: provider 轮询/交换、client+连接的单事务 CAS 切换、多阶段飞书推进及 durable 终态。
// POS: Connector 对话授权的秘密消费和最终提交边界。
package connectors

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
	"github.com/nexus-research-lab/nexus/internal/connectors/providers"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	connectorstore "github.com/nexus-research-lab/nexus/internal/storage/connectors"
)

const authorizationDevicePollClaimLifetime = 30 * time.Second

func (c *AuthorizationControl) pollDevice(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
) error {
	now := c.now()
	claimed, err := c.flows.ClaimDevicePoll(
		ctx, flow.OwnerUserID, flow.FlowID,
		now, now.Add(authorizationDevicePollClaimLifetime),
	)
	if err != nil || !claimed {
		return err
	}
	secret, err := c.decryptFlowSecret(flow)
	if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"加密临时授权凭据无法恢复", now,
		)
		return err
	}
	if strings.TrimSpace(secret.DeviceCode) == "" {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"临时授权凭据缺失", now,
		)
		return errors.New("Connector authorization device flow 缺少临时凭据")
	}
	entry, ok := getConnector(flow.ConnectorID)
	if !ok || entry.Status != "available" || !entry.DeviceAuth {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector 已不可用", now,
		)
		return errors.New("Connector authorization target 已不可用")
	}
	if flow.Stage == deviceAuthStageAppSelection {
		return c.pollAppRegistration(ctx, flow, entry, secret)
	}
	return c.pollUserDeviceAuthorization(
		ctx, flow, entry, secret,
	)
}

func (c *AuthorizationControl) pollAppRegistration(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
	entry CatalogEntry,
	secret authorizationFlowSecret,
) error {
	result, err := c.connectors.feishuRegistrationClient(entry).Poll(
		ctx, strings.TrimSpace(secret.DeviceCode),
	)
	if err != nil {
		_ = c.releaseDevicePoll(
			ctx, flow, "等待 Connector 应用配置确认", false,
		)
		return authorizationProviderFailure("查询应用配置", err)
	}
	switch result.Status {
	case appregistration.StatusPending:
		return c.releaseDevicePoll(
			ctx, flow, "等待 Connector 应用配置确认", false,
		)
	case appregistration.StatusSlowDown:
		return c.releaseDevicePoll(
			ctx, flow, "Connector 要求降低轮询频率", true,
		)
	case appregistration.StatusExpired:
		_, err = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusExpired, "Connector 应用配置已过期", c.now(),
		)
		return err
	case appregistration.StatusFailed:
		_, err = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusDenied, "Connector 应用配置未获批准", c.now(),
		)
		return err
	case appregistration.StatusSucceeded:
	default:
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed, "Connector 返回未知应用配置状态", c.now(),
		)
		return errors.New("Connector authorization app registration 状态无效")
	}

	clientID := strings.TrimSpace(result.Credentials["client_id"])
	clientSecret := strings.TrimSpace(result.Credentials["client_secret"])
	if clientID == "" || clientSecret == "" {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed, "Connector 应用配置结果不完整", c.now(),
		)
		return errors.New("Connector authorization app credentials 缺失")
	}
	started, err := c.connectors.startOAuthDeviceAuth(
		ctx, entry, clientID, clientSecret,
	)
	if err != nil {
		_ = c.releaseDevicePoll(
			ctx, flow, "应用已创建，但用户授权启动失败；可稍后重试", false,
		)
		return authorizationProviderFailure("启动用户授权", err)
	}
	encrypted, err := c.encryptFlowSecret(authorizationFlowSecret{
		DeviceCode:        strings.TrimSpace(started.DeviceCode),
		ClientID:          clientID,
		ClientSecret:      clientSecret,
		CommitOAuthClient: true,
	})
	if err != nil {
		return err
	}
	interval := normalizedDevicePollInterval(started.Interval)
	now := c.now()
	expiresAt := now.Add(
		time.Duration(normalizedDeviceExpiry(started.ExpiresIn)) * time.Second,
	)
	err = c.flows.AdvanceDeviceStage(
		ctx, flow.OwnerUserID, flow.FlowID,
		connectorstore.AuthorizationFlowDeviceStage{
			ExpectedConfigurationVersion: flow.ExpectedConfigurationVersion,
			Stage:                        deviceAuthStageUserAuthorization,
			SecretEncrypted:              encrypted,
			PublicUserCode:               strings.TrimSpace(started.UserCode),
			PublicVerificationURI: strings.TrimSpace(
				started.VerificationURI,
			),
			PublicVerificationURIComplete: strings.TrimSpace(
				started.VerificationURIComplete,
			),
			PollIntervalSeconds: interval,
			NextPollAt:          now.Add(time.Duration(interval) * time.Second),
			ExpiresAt:           expiresAt,
		},
		now,
	)
	if err != nil {
		// 未能证明阶段更新未落库时不擦除流程秘密；
		// 释放成功则可安全重试，并发胜者已推进时该释放是 no-op。
		_ = c.releaseDevicePoll(
			ctx, flow, "Connector 授权阶段尚未确认，请稍后重试", false,
		)
	}
	return err
}

func (c *AuthorizationControl) pollUserDeviceAuthorization(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
	entry CatalogEntry,
	secret authorizationFlowSecret,
) error {
	provider, err := c.connectors.deviceProvider(entry)
	if err != nil {
		return c.failClaimedDeviceFlow(
			ctx, flow, "Connector 不再支持 Device Flow", err,
		)
	}
	clientID := strings.TrimSpace(secret.ClientID)
	clientSecret := strings.TrimSpace(secret.ClientSecret)
	if clientID == "" {
		return c.failClaimedDeviceFlow(
			ctx, flow, "Connector OAuth 应用绑定无法恢复",
			errors.New("Connector authorization OAuth client 绑定缺失"),
		)
	}
	payload, err := provider.ExchangeDeviceToken(
		ctx, c.connectors.httpClient, providers.DeviceTokenRequest{
			ClientID: clientID, ClientSecret: clientSecret,
			DeviceCode: strings.TrimSpace(secret.DeviceCode),
		},
	)
	if err != nil {
		switch deviceAuthStatusFromError(err) {
		case deviceAuthStatusPending:
			return c.releaseDevicePoll(
				ctx, flow, "等待 Connector 授权确认", false,
			)
		case deviceAuthStatusSlowDown:
			return c.releaseDevicePoll(
				ctx, flow, "Connector 要求降低轮询频率", true,
			)
		case deviceAuthStatusExpired:
			_, markErr := c.flows.MarkTerminal(
				ctx, flow.OwnerUserID, flow.FlowID,
				AuthorizationStatusExpired,
				"Connector 用户授权已过期", c.now(),
			)
			return markErr
		case deviceAuthStatusDenied:
			_, markErr := c.flows.MarkTerminal(
				ctx, flow.OwnerUserID, flow.FlowID,
				AuthorizationStatusDenied,
				"Connector 用户授权未获批准", c.now(),
			)
			return markErr
		default:
			_ = c.releaseDevicePoll(
				ctx, flow, "Connector 暂时无法确认授权状态", false,
			)
			return authorizationProviderFailure("查询用户授权", err)
		}
	}
	credentials := normalizeOAuthPayload(payload)
	_, err = c.connectFlowAtVersion(
		ctx, flow, entry, credentials, secret,
	)
	return err
}

func (c *AuthorizationControl) releaseDevicePoll(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
	message string,
	slowDown bool,
) error {
	interval := normalizedDevicePollInterval(flow.PollIntervalSeconds)
	if slowDown {
		interval += 5
		if interval > 60 {
			interval = 60
		}
	}
	now := c.now()
	_, err := c.flows.ReleaseDevicePoll(
		ctx, flow.OwnerUserID, flow.FlowID,
		now.Add(time.Duration(interval)*time.Second),
		message, now,
	)
	return err
}

func (c *AuthorizationControl) failClaimedDeviceFlow(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
	message string,
	cause error,
) error {
	_, _ = c.flows.MarkTerminal(
		ctx, flow.OwnerUserID, flow.FlowID,
		AuthorizationStatusFailed, message, c.now(),
	)
	return authorizationProviderFailure("完成", cause)
}

func (c *AuthorizationControl) completeOAuthCallback(
	ctx context.Context,
	state stateRow,
	request OAuthCallbackRequest,
) (*Info, error) {
	flow, err := c.flows.GetByID(ctx, state.ControlFlowID)
	if err != nil {
		return nil, err
	}
	if flow == nil ||
		flow.OwnerUserID != strings.TrimSpace(state.OwnerUserID) ||
		flow.ConnectorID != strings.TrimSpace(state.ConnectorID) ||
		flow.AuthorizationMethod != AuthorizationMethodOAuthBrowser ||
		flow.Status != AuthorizationStatusPending {
		return nil, errors.New("Connector authorization callback flow 无效")
	}
	now := c.now()
	if !flow.ExpiresAt.After(now) || !state.ExpiresAt.After(now) {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusExpired, "Connector OAuth 授权已过期", now,
		)
		return nil, errors.New("Connector authorization callback 已过期")
	}
	if !flow.OpenedAt.Valid {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"OAuth callback 未绑定人工打开记录", now,
		)
		return nil, errors.New(
			"Connector authorization callback 缺少人工打开证据",
		)
	}
	if err = c.revalidateCallbackAuthority(ctx, flow); err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector 授权主体或主智能体身份已失效", now,
		)
		return nil, err
	}
	if err = c.connectors.validateOAuthCallbackRedirect(
		state, request.RedirectURI,
	); err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"OAuth callback redirect 不匹配", now,
		)
		return nil, err
	}
	current, err := c.connectors.GetConfigurationState(
		ctx, flow.OwnerUserID, flow.ConnectorID,
	)
	if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector 配置版本无法核验", now,
		)
		return nil, err
	}
	if current.ConfigurationVersion != flow.ExpectedConfigurationVersion {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusConflict,
			"Connector 配置已变化，请重新开始授权", now,
		)
		return nil, ErrConfigurationConflict
	}
	entry, ok := getConnector(flow.ConnectorID)
	if !ok || entry.Status != "available" {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector 已不可用", now,
		)
		return nil, errors.New("Connector authorization target 已不可用")
	}
	extra, err := state.extra()
	if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"OAuth callback 参数状态无效", now,
		)
		return nil, err
	}
	provider, err := providers.Get(
		connectorFirstNonEmpty(entry.Provider, entry.ConnectorID),
	)
	if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector OAuth provider 已不可用", now,
		)
		return nil, err
	}
	clientID, clientSecret, err := c.connectors.oauthCredentials(
		ctx, flow.OwnerUserID, entry.ConnectorID,
	)
	if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector OAuth 应用配置不可用", now,
		)
		return nil, err
	}
	payload, err := provider.ExchangeToken(
		ctx, c.connectors.httpClient, providers.TokenRequest{
			ClientID: clientID, ClientSecret: clientSecret,
			RedirectURI:  state.RedirectURI,
			Code:         strings.TrimSpace(request.Code),
			CodeVerifier: state.CodeVerifier, Extra: extra,
		},
	)
	if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector OAuth token exchange 失败", now,
		)
		return nil, authorizationProviderFailure("完成 OAuth", err)
	}
	credentials := mergeCredentialExtras(
		normalizeOAuthPayload(payload), extra,
	)
	_, err = c.connectFlowAtVersion(
		ctx, flow, entry, credentials, authorizationFlowSecret{},
	)
	if err != nil {
		return nil, err
	}
	info := c.connectors.toInfo(
		ctx, flow.OwnerUserID, entry, "connected",
	)
	return &info, nil
}

func (c *AuthorizationControl) revalidateCallbackAuthority(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
) error {
	if c == nil || flow == nil || c.roleResolver == nil || c.agents == nil {
		return errors.New("Connector authorization callback 缺少身份校验器")
	}
	role, err := c.roleResolver.ResolveActivePrincipalRole(
		ctx, flow.HumanPrincipalUserID,
	)
	if err != nil {
		return err
	}
	if strings.TrimSpace(role) != flow.HumanPrincipalRole {
		return errors.New(
			"Connector authorization callback principal 角色已变化",
		)
	}
	scoped := authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     flow.HumanPrincipalUserID,
		Username:   flow.AgentID,
		Role:       flow.HumanPrincipalRole,
		AuthMethod: flow.HumanAuthMethod,
	})
	agentValue, err := c.agents.GetAgent(scoped, flow.AgentID)
	if err != nil {
		return err
	}
	if agentValue == nil || !agentValue.IsMain ||
		strings.TrimSpace(agentValue.OwnerUserID) != flow.OwnerUserID {
		return errors.New(
			"Connector authorization callback 主智能体身份已失效",
		)
	}
	return nil
}

func (c *AuthorizationControl) connectFlowAtVersion(
	ctx context.Context,
	flow *connectorstore.AuthorizationFlow,
	entry CatalogEntry,
	credentials string,
	secret authorizationFlowSecret,
) (int64, error) {
	record := connectionRecord{
		OwnerUserID: flow.OwnerUserID,
		ConnectorID: flow.ConnectorID,
		State:       "connected", Credentials: credentials,
		AuthType: entry.AuthType,
	}
	if err := c.connectors.encryptConnectionCredentials(&record); err != nil {
		return 0, err
	}
	completedVersion := flow.ExpectedConfigurationVersion + 1
	now := c.now()
	var oauthStore *connectorstore.OAuthClientStore
	var err error
	if secret.CommitOAuthClient {
		clientID := strings.TrimSpace(secret.ClientID)
		clientSecret := strings.TrimSpace(secret.ClientSecret)
		if !entry.UserOAuthClient || clientID == "" || clientSecret == "" {
			return 0, errors.New(
				"Connector authorization OAuth client 切换绑定无效",
			)
		}
		oauthStore, err = c.connectors.oauthClientStore()
		if err != nil {
			return 0, err
		}
	}
	version, err := c.connectors.mutateConnector(
		ctx, flow.OwnerUserID, flow.ConnectorID,
		&flow.ExpectedConfigurationVersion,
		func(tx *sql.Tx) error {
			if oauthStore != nil {
				if upsertErr := oauthStore.UpsertTx(
					ctx, tx, connectorstore.OAuthClient{
						OwnerUserID:  flow.OwnerUserID,
						ConnectorID:  flow.ConnectorID,
						ClientID:     strings.TrimSpace(secret.ClientID),
						ClientSecret: strings.TrimSpace(secret.ClientSecret),
					},
				); upsertErr != nil {
					return upsertErr
				}
			}
			if writeErr := c.connectors.writeConnection(
				ctx, tx, record,
			); writeErr != nil {
				return writeErr
			}
			return c.flows.MarkConnectedTx(
				ctx, tx, flow.OwnerUserID, flow.FlowID,
				completedVersion, now,
			)
		},
	)
	if errors.Is(err, ErrConfigurationConflict) {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusConflict,
			"Connector 配置已变化，请重新开始授权", now,
		)
	} else if err != nil {
		_, _ = c.flows.MarkTerminal(
			ctx, flow.OwnerUserID, flow.FlowID,
			AuthorizationStatusFailed,
			"Connector 连接提交失败", now,
		)
	}
	return version, err
}
