package connectors

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/providers"
)

// StartDeviceAuth 启动支持桌面公共客户端的 OAuth Device Flow。
func (s *Service) StartDeviceAuth(ctx context.Context, ownerUserID string, connectorID string) (*DeviceAuthStartResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	provider, err := s.deviceProvider(entry)
	if err != nil {
		return nil, err
	}
	clientID, err := s.oauthPublicClientID(ctx, ownerUserID, entry.ConnectorID, entry.Title)
	if err != nil {
		return nil, err
	}
	response, err := provider.RequestDeviceCode(ctx, s.httpClient, providers.DeviceCodeRequest{
		ClientID: clientID,
		Scopes:   entry.Scopes,
	})
	if err != nil {
		return nil, friendlyDeviceAuthError(err)
	}
	return &DeviceAuthStartResult{
		ConnectorID:             entry.ConnectorID,
		DeviceCode:              response.DeviceCode,
		UserCode:                response.UserCode,
		VerificationURI:         response.VerificationURI,
		VerificationURIComplete: response.VerificationURIComplete,
		ExpiresIn:               response.ExpiresIn,
		Interval:                response.Interval,
	}, nil
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
	provider, err := s.deviceProvider(entry)
	if err != nil {
		return nil, err
	}
	clientID, err := s.oauthPublicClientID(ctx, ownerUserID, entry.ConnectorID, entry.Title)
	if err != nil {
		return nil, err
	}
	payload, err := provider.ExchangeDeviceToken(ctx, s.httpClient, providers.DeviceTokenRequest{
		ClientID:   clientID,
		DeviceCode: deviceCode,
	})
	if err != nil {
		status := deviceAuthStatusFromError(err)
		if status != "" {
			return &DeviceAuthPollResult{
				Status:  status,
				Message: deviceAuthMessage(status),
			}, nil
		}
		return nil, friendlyDeviceAuthError(err)
	}
	credentials := normalizeOAuthPayload(payload)
	if err = s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: credentials,
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "connected")
	return &DeviceAuthPollResult{
		Status:    deviceAuthStatusConnected,
		Connector: &info,
	}, nil
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

func deviceAuthMessage(status string) string {
	switch status {
	case deviceAuthStatusPending:
		return "等待 GitHub 授权确认"
	case deviceAuthStatusSlowDown:
		return "GitHub 要求降低轮询频率"
	case deviceAuthStatusExpired:
		return "GitHub 授权码已过期"
	case deviceAuthStatusDenied:
		return "用户取消了 GitHub 授权"
	default:
		return ""
	}
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
