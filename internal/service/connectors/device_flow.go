// INPUT: Connector 目录、显式飞书连接方式、owner 级配置版本与官方 Device Flow 响应。
// OUTPUT: 精确绑定本次 OAuth client 的 GitHub/飞书授权会话；成功时原子切换 client 与连接。
// POS: connectors 服务的设备授权编排，协议细节与持久化分别下沉到 provider/store。
package connectors

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
	"github.com/nexus-research-lab/nexus/internal/connectors/providers"
	connectorstore "github.com/nexus-research-lab/nexus/internal/storage/connectors"
)

const (
	feishuAppRegistrationDevicePrefix = "feishu-app-registration:"
	deviceAuthStageAppSelection       = "app_selection"
	deviceAuthStageUserAuthorization  = "user_authorization"
)

// StartDeviceAuth 启动 Device Flow；飞书必须显式选择官方扫码或手工凭据兜底。
func (s *Service) StartDeviceAuth(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
	mode DeviceAuthStartMode,
) (*DeviceAuthStartResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	state, err := s.GetConfigurationState(
		ctx, ownerUserID, entry.ConnectorID,
	)
	if err != nil {
		return nil, err
	}
	if entry.AutoOAuthClient {
		switch mode {
		case DeviceAuthStartModeOfficialQR:
			started, startErr := s.startFeishuAppRegistration(ctx, entry)
			if startErr != nil {
				return nil, startErr
			}
			return s.bindDeviceAuthAttempt(
				ctx, ownerUserID, entry, started, "", "",
				state.ConfigurationVersion, false,
			)
		case DeviceAuthStartModeManualCredentials:
			clientID, clientSecret, credentialsErr := s.oauthCredentials(ctx, ownerUserID, entry.ConnectorID)
			if credentialsErr != nil {
				return nil, credentialsErr
			}
			started, startErr := s.startOAuthDeviceAuth(
				ctx, entry, clientID, clientSecret,
			)
			if startErr != nil {
				return nil, startErr
			}
			return s.bindDeviceAuthAttempt(
				ctx, ownerUserID, entry, started, clientID, clientSecret,
				state.ConfigurationVersion, false,
			)
		default:
			return nil, errors.New("请选择官方扫码连接或手工应用凭据兜底")
		}
	}
	if mode != "" {
		return nil, errors.New("当前连接器不支持飞书连接方式选项")
	}
	provider, err := s.deviceProvider(entry)
	if err != nil {
		return nil, err
	}
	clientID, err := s.oauthPublicClientID(ctx, ownerUserID, entry.ConnectorID, entry.Title)
	if err != nil {
		return nil, err
	}
	started, err := s.startOAuthDeviceAuthWithProvider(
		ctx, entry, provider, clientID, "",
	)
	if err != nil {
		return nil, err
	}
	return s.bindDeviceAuthAttempt(
		ctx, ownerUserID, entry, started, clientID, "",
		state.ConfigurationVersion, false,
	)
}

func (s *Service) startOAuthDeviceAuth(
	ctx context.Context,
	entry CatalogEntry,
	clientID string,
	clientSecret string,
) (*DeviceAuthStartResult, error) {
	provider, err := s.deviceProvider(entry)
	if err != nil {
		return nil, err
	}
	return s.startOAuthDeviceAuthWithProvider(ctx, entry, provider, clientID, clientSecret)
}

func (s *Service) startOAuthDeviceAuthWithProvider(
	ctx context.Context,
	entry CatalogEntry,
	provider providers.DeviceProvider,
	clientID string,
	clientSecret string,
) (*DeviceAuthStartResult, error) {
	response, err := provider.RequestDeviceCode(ctx, s.httpClient, providers.DeviceCodeRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       entry.Scopes,
	})
	if err != nil {
		return nil, friendlyDeviceAuthError(err)
	}
	result := &DeviceAuthStartResult{
		ConnectorID:             entry.ConnectorID,
		DeviceCode:              response.DeviceCode,
		UserCode:                response.UserCode,
		VerificationURI:         response.VerificationURI,
		VerificationURIComplete: response.VerificationURIComplete,
		ExpiresIn:               response.ExpiresIn,
		Interval:                response.Interval,
	}
	if entry.AutoOAuthClient {
		result.Stage = deviceAuthStageUserAuthorization
	}
	return result, nil
}

// PollDeviceAuth 轮询 OAuth Device Flow，并在成功后保存连接凭证。
func (s *Service) PollDeviceAuth(ctx context.Context, ownerUserID string, connectorID string, deviceCode string) (*DeviceAuthPollResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	if strings.TrimSpace(deviceCode) == "" {
		return nil, errors.New("device_code 不能为空")
	}
	attempt, err := s.openDeviceAuthAttempt(
		ctx, ownerUserID, entry, deviceCode,
	)
	if err != nil {
		return nil, err
	}
	state, err := s.GetConfigurationState(
		ctx, ownerUserID, entry.ConnectorID,
	)
	if err != nil {
		return nil, err
	}
	if state.ConfigurationVersion != attempt.ExpectedConfigurationVersion {
		return nil, ErrConfigurationConflict
	}
	if attempt.Stage == deviceAuthStageAppSelection {
		return s.pollFeishuAppRegistration(ctx, ownerUserID, entry, attempt)
	}
	provider, err := s.deviceProvider(entry)
	if err != nil {
		return nil, err
	}
	payload, err := provider.ExchangeDeviceToken(ctx, s.httpClient, providers.DeviceTokenRequest{
		ClientID:     attempt.ClientID,
		ClientSecret: attempt.ClientSecret,
		DeviceCode:   attempt.DeviceCode,
	})
	if err != nil {
		status := deviceAuthStatusFromError(err)
		if status != "" {
			return &DeviceAuthPollResult{
				Status:  status,
				Message: deviceAuthMessage(status, entry.Title),
			}, nil
		}
		return nil, friendlyDeviceAuthError(err)
	}
	credentials := normalizeOAuthPayload(payload)
	if err = s.commitDeviceAuthConnection(
		ctx, ownerUserID, entry, attempt, credentials,
	); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "connected")
	return &DeviceAuthPollResult{
		Status:    deviceAuthStatusConnected,
		Connector: &info,
	}, nil
}

func (s *Service) startFeishuAppRegistration(
	ctx context.Context,
	entry CatalogEntry,
) (*DeviceAuthStartResult, error) {
	started, err := s.feishuRegistrationClient(entry).Start(ctx)
	if err != nil {
		return nil, err
	}
	return &DeviceAuthStartResult{
		ConnectorID:             entry.ConnectorID,
		DeviceCode:              started.DeviceCode,
		UserCode:                started.UserCode,
		VerificationURI:         started.VerificationURI,
		VerificationURIComplete: started.VerificationURIComplete,
		ExpiresIn:               started.ExpiresIn,
		Interval:                started.Interval,
		Stage:                   deviceAuthStageAppSelection,
	}, nil
}

func (s *Service) pollFeishuAppRegistration(
	ctx context.Context,
	ownerUserID string,
	entry CatalogEntry,
	attempt deviceAuthAttempt,
) (*DeviceAuthPollResult, error) {
	result, err := s.feishuRegistrationClient(entry).Poll(
		ctx, attempt.DeviceCode,
	)
	if err != nil {
		return nil, err
	}
	switch result.Status {
	case appregistration.StatusPending:
		return &DeviceAuthPollResult{Status: deviceAuthStatusPending, Message: firstRegistrationMessage(result.Message, "等待飞书确认应用")}, nil
	case appregistration.StatusSlowDown:
		return &DeviceAuthPollResult{Status: deviceAuthStatusSlowDown, Message: firstRegistrationMessage(result.Message, "飞书要求降低轮询频率")}, nil
	case appregistration.StatusExpired:
		return &DeviceAuthPollResult{Status: deviceAuthStatusExpired, Message: firstRegistrationMessage(result.Message, "飞书应用配置二维码已过期")}, nil
	case appregistration.StatusFailed:
		return &DeviceAuthPollResult{Status: deviceAuthStatusDenied, Message: firstRegistrationMessage(result.Message, "飞书应用配置未完成")}, nil
	case appregistration.StatusSucceeded:
		clientID := strings.TrimSpace(result.Credentials["client_id"])
		clientSecret := strings.TrimSpace(result.Credentials["client_secret"])
		if clientID == "" || clientSecret == "" {
			return nil, errors.New("飞书应用配置成功但未返回应用凭据")
		}
		next, startErr := s.startOAuthDeviceAuth(ctx, entry, clientID, clientSecret)
		if startErr != nil {
			return nil, startErr
		}
		next, startErr = s.bindDeviceAuthAttempt(
			ctx, ownerUserID, entry, next, clientID, clientSecret,
			attempt.ExpectedConfigurationVersion, true,
		)
		if startErr != nil {
			return nil, startErr
		}
		return &DeviceAuthPollResult{
			Status:  deviceAuthStatusPending,
			Message: "飞书应用已就绪，请继续完成当前用户的云文档授权",
			Next:    next,
		}, nil
	default:
		return nil, errors.New("未知飞书应用注册状态")
	}
}

func (s *Service) commitDeviceAuthConnection(
	ctx context.Context,
	ownerUserID string,
	entry CatalogEntry,
	attempt deviceAuthAttempt,
	credentials string,
) error {
	record := connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: credentials,
		AuthType:    entry.AuthType,
	}
	if err := s.encryptConnectionCredentials(&record); err != nil {
		return err
	}
	var oauthStore *connectorstore.OAuthClientStore
	var err error
	if attempt.CommitOAuthClient {
		oauthStore, err = s.oauthClientStore()
		if err != nil {
			return err
		}
	}
	_, err = s.mutateConnector(
		ctx, ownerUserID, entry.ConnectorID,
		&attempt.ExpectedConfigurationVersion,
		func(tx *sql.Tx) error {
			if oauthStore != nil {
				if upsertErr := oauthStore.UpsertTx(
					ctx, tx, connectorstore.OAuthClient{
						OwnerUserID:  ownerUserID,
						ConnectorID:  entry.ConnectorID,
						ClientID:     attempt.ClientID,
						ClientSecret: attempt.ClientSecret,
					},
				); upsertErr != nil {
					return upsertErr
				}
			}
			return s.writeConnection(ctx, tx, record)
		},
	)
	return err
}

func (s *Service) feishuRegistrationClient(entry CatalogEntry) appregistration.Client {
	if s.registrationClientFactory != nil {
		return s.registrationClientFactory()
	}
	return appregistration.NewFeishuClient(s.httpClient, appregistration.FeishuOptions{
		Name:        "Nexus 云文档",
		Description: "连接 Nexus 后用于在用户授权范围内读写飞书云文档。",
		UserScopes:  entry.Scopes,
	})
}

func (s *Service) deviceProvider(entry CatalogEntry) (providers.DeviceProvider, error) {
	providerID := connectorFirstNonEmpty(entry.Provider, entry.ConnectorID)
	provider, err := providers.Get(providerID)
	if err != nil {
		return nil, err
	}
	deviceProvider, ok := provider.(providers.DeviceProvider)
	if !ok {
		return nil, errors.New("连接器不支持 Device Flow")
	}
	return deviceProvider, nil
}

func (s *Service) oauthPublicClientID(ctx context.Context, ownerUserID string, connectorID string, _ string) (string, error) {
	if connectorID == "github" && s.isDesktopMode() {
		return requireOAuthClientID(s.config.ConnectorGitHubClientID, "GitHub")
	}
	clientID, _, err := s.oauthCredentials(ctx, ownerUserID, connectorID)
	if err == nil {
		return clientID, nil
	}
	return "", err
}

func deviceAuthStatusFromError(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "authorization_pending"):
		return deviceAuthStatusPending
	case strings.Contains(message, "slow_down"):
		return deviceAuthStatusSlowDown
	case strings.Contains(message, "expired_token"), strings.Contains(message, "token_expired"):
		return deviceAuthStatusExpired
	case strings.Contains(message, "access_denied"):
		return deviceAuthStatusDenied
	default:
		return ""
	}
}

func deviceAuthMessage(status string, title string) string {
	switch status {
	case deviceAuthStatusPending:
		return "等待 " + title + " 授权确认"
	case deviceAuthStatusSlowDown:
		return title + " 要求降低轮询频率"
	case deviceAuthStatusExpired:
		return title + " 授权码已过期"
	case deviceAuthStatusDenied:
		return "用户取消了 " + title + " 授权"
	default:
		return ""
	}
}

func firstRegistrationMessage(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func friendlyDeviceAuthError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "device_flow_disabled") {
		return errors.New("GitHub OAuth App 未启用 Device Flow，请在 GitHub Developer settings 的 OAuth App 设置中启用 Device Flow 后重试")
	}
	return err
}
