// INPUT: owner/Connector、provider device_code、本次 OAuth client 与启动时配置版本。
// OUTPUT: 客户端可回传但不可读取的加密 Device Flow attempt，以及严格的 owner/scope/expiry 校验。
// POS: 普通 UI Device Flow 的短期能力封装；不写 canonical OAuth client、connection 或业务身份。
package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
)

const (
	deviceAuthAttemptPrefix         = "nexus-device-auth:"
	deviceAuthAttemptVersion        = 1
	maxDeviceAuthAttemptTokenLength = 64 * 1024
)

// deviceAuthAttempt 把 provider device_code 与本次实际使用的 OAuth client
// 固定在同一个加密短期能力中。ExpectedConfigurationVersion 只用于最终 CAS，
// 不能被当作 owner、Connector 或 provider 的身份替代品。
type deviceAuthAttempt struct {
	Version                      int       `json:"version"`
	OwnerUserID                  string    `json:"owner_user_id"`
	ConnectorID                  string    `json:"connector_id"`
	Stage                        string    `json:"stage"`
	DeviceCode                   string    `json:"device_code"`
	ClientID                     string    `json:"client_id,omitempty"`
	ClientSecret                 string    `json:"client_secret,omitempty"`
	ExpectedConfigurationVersion int64     `json:"expected_configuration_version"`
	CommitOAuthClient            bool      `json:"commit_oauth_client,omitempty"`
	ExpiresAt                    time.Time `json:"expires_at"`
}

func (s *Service) bindDeviceAuthAttempt(
	ctx context.Context,
	ownerUserID string,
	entry CatalogEntry,
	started *DeviceAuthStartResult,
	clientID string,
	clientSecret string,
	expectedConfigurationVersion int64,
	commitOAuthClient bool,
) (*DeviceAuthStartResult, error) {
	if started == nil || strings.TrimSpace(started.DeviceCode) == "" {
		return nil, errors.New("Device provider 未返回临时授权凭据")
	}
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	now := time.Now().UTC()
	attempt := deviceAuthAttempt{
		Version:     deviceAuthAttemptVersion,
		OwnerUserID: ownerUserID,
		ConnectorID: strings.TrimSpace(entry.ConnectorID),
		Stage: connectorFirstNonEmpty(
			started.Stage,
			deviceAuthStageUserAuthorization,
		),
		DeviceCode:                   strings.TrimSpace(started.DeviceCode),
		ClientID:                     strings.TrimSpace(clientID),
		ClientSecret:                 strings.TrimSpace(clientSecret),
		ExpectedConfigurationVersion: expectedConfigurationVersion,
		CommitOAuthClient:            commitOAuthClient,
		ExpiresAt: now.Add(
			time.Duration(normalizedDeviceExpiry(started.ExpiresIn)) * time.Second,
		),
	}
	if err := validateDeviceAuthAttempt(attempt, entry, now); err != nil {
		return nil, err
	}
	token, err := s.encryptDeviceAuthAttempt(attempt)
	if err != nil {
		return nil, err
	}
	bound := *started
	prefix := deviceAuthAttemptPrefix
	if attempt.Stage == deviceAuthStageAppSelection {
		prefix = feishuAppRegistrationDevicePrefix
	}
	bound.DeviceCode = prefix + token
	return &bound, nil
}

func (s *Service) openDeviceAuthAttempt(
	ctx context.Context,
	ownerUserID string,
	entry CatalogEntry,
	token string,
) (deviceAuthAttempt, error) {
	token = strings.TrimSpace(token)
	stage := deviceAuthStageUserAuthorization
	switch {
	case strings.HasPrefix(token, feishuAppRegistrationDevicePrefix):
		stage = deviceAuthStageAppSelection
		token = strings.TrimPrefix(token, feishuAppRegistrationDevicePrefix)
	case strings.HasPrefix(token, deviceAuthAttemptPrefix):
		token = strings.TrimPrefix(token, deviceAuthAttemptPrefix)
	default:
		return deviceAuthAttempt{}, errors.New("Device Flow 授权会话无效，请重新开始")
	}
	if token == "" || len(token) > maxDeviceAuthAttemptTokenLength {
		return deviceAuthAttempt{}, errors.New("Device Flow 授权会话格式不正确")
	}
	key, err := credentials.DecodeKey(s.config.ConnectorCredentialsKey)
	if err != nil {
		return deviceAuthAttempt{}, err
	}
	payload, err := credentials.DecryptPayload(key, token)
	if err != nil {
		return deviceAuthAttempt{}, errors.New("Device Flow 授权会话无效，请重新开始")
	}
	var attempt deviceAuthAttempt
	if err = json.Unmarshal(payload, &attempt); err != nil {
		return deviceAuthAttempt{}, errors.New("Device Flow 授权会话格式不正确")
	}
	now := time.Now().UTC()
	if err = validateDeviceAuthAttempt(attempt, entry, now); err != nil {
		return deviceAuthAttempt{}, err
	}
	if attempt.Stage != stage ||
		attempt.OwnerUserID != normalizeConnectorOwnerUserID(ctx, ownerUserID) ||
		attempt.ConnectorID != strings.TrimSpace(entry.ConnectorID) {
		return deviceAuthAttempt{}, errors.New("Device Flow 授权会话与当前用户或连接器不匹配")
	}
	return attempt, nil
}

func (s *Service) encryptDeviceAuthAttempt(attempt deviceAuthAttempt) (string, error) {
	payload, err := json.Marshal(attempt)
	if err != nil {
		return "", err
	}
	key, err := credentials.DecodeKey(s.config.ConnectorCredentialsKey)
	if err != nil {
		return "", err
	}
	return credentials.EncryptPayload(key, payload)
}

func validateDeviceAuthAttempt(
	attempt deviceAuthAttempt,
	entry CatalogEntry,
	now time.Time,
) error {
	if attempt.Version != deviceAuthAttemptVersion ||
		strings.TrimSpace(attempt.OwnerUserID) == "" ||
		strings.TrimSpace(attempt.ConnectorID) == "" ||
		strings.TrimSpace(attempt.DeviceCode) == "" ||
		attempt.ExpectedConfigurationVersion < 1 {
		return errors.New("Device Flow 授权会话缺少必要绑定")
	}
	if !attempt.ExpiresAt.After(now) {
		return errors.New("Device Flow 授权会话已过期，请重新开始")
	}
	switch attempt.Stage {
	case deviceAuthStageAppSelection:
		if !entry.AutoOAuthClient || attempt.CommitOAuthClient ||
			strings.TrimSpace(attempt.ClientID) != "" ||
			strings.TrimSpace(attempt.ClientSecret) != "" {
			return errors.New("Device Flow 应用配置阶段绑定无效")
		}
	case deviceAuthStageUserAuthorization:
		if strings.TrimSpace(attempt.ClientID) == "" {
			return errors.New("Device Flow 授权会话缺少 OAuth Client ID")
		}
		if attempt.CommitOAuthClient && (!entry.UserOAuthClient ||
			strings.TrimSpace(attempt.ClientSecret) == "") {
			return errors.New("Device Flow OAuth 应用切换绑定无效")
		}
	default:
		return errors.New("Device Flow 授权阶段无效")
	}
	return nil
}
