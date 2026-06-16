package channels

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

type FeishuIngressPreparation = channeladapters.FeishuIngressPreparation

type FeishuIngressCallback = channeladapters.FeishuIngressCallback

var ErrFeishuCallbackUnauthorized = channeladapters.ErrFeishuCallbackUnauthorized

func (s *ControlService) ResolveChannelOwnerByConfig(ctx context.Context, channelType string, key string, value string) (string, error) {
	channelType = normalizeIMChannelType(channelType)
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if channelType == "" || key == "" || value == "" {
		return "", nil
	}
	rows, err := s.listAllChannelConfigRows(ctx)
	if err != nil {
		return "", err
	}
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled || row.ChannelType != channelType {
			continue
		}
		publicConfig, decodeErr := decodeStringMap(row.ConfigJSON)
		if decodeErr != nil {
			return "", decodeErr
		}
		if strings.TrimSpace(publicConfig[key]) == value {
			return row.OwnerUserID, nil
		}
	}
	return "", nil
}

func (s *ControlService) PrepareFeishuIngress(ctx context.Context, raw []byte, header http.Header) (FeishuIngressPreparation, error) {
	encryptValue, encrypted, err := channeladapters.FeishuEncryptEnvelope(raw)
	if err != nil {
		return FeishuIngressPreparation{}, err
	}
	configs, err := s.listFeishuIngressConfigs(ctx)
	if err != nil {
		return FeishuIngressPreparation{}, err
	}
	if encrypted {
		return s.prepareEncryptedFeishuIngress(raw, header, encryptValue, configs)
	}
	callback, err := channeladapters.DecodeFeishuIngressCallback(raw)
	if err != nil {
		return FeishuIngressPreparation{}, err
	}
	config := matchFeishuIngressConfig(configs, callback)
	if config == nil {
		// 飞书 URL 校验 payload 通常没有 app_id。开发态只保存 App ID/App Secret 时，
		// 这里不能因为飞书侧携带了 token 就拒绝 challenge；真实消息事件仍按 app_id 绑定配置。
		if strings.TrimSpace(callback.Challenge) != "" && strings.TrimSpace(callback.AppID) == "" {
			return FeishuIngressPreparation{Body: raw}, nil
		}
		if strings.TrimSpace(callback.AppID) == "" && strings.TrimSpace(callback.Token) == "" {
			return FeishuIngressPreparation{Body: raw}, nil
		}
		return FeishuIngressPreparation{}, fmt.Errorf("%w: unknown feishu app", ErrFeishuCallbackUnauthorized)
	}
	if strings.TrimSpace(config.EncryptKey) != "" {
		return FeishuIngressPreparation{}, fmt.Errorf("%w: encrypted feishu callback expected", ErrFeishuCallbackUnauthorized)
	}
	if err = channeladapters.VerifyFeishuCallbackToken(callback, config.VerificationToken); err != nil {
		return FeishuIngressPreparation{}, err
	}
	return FeishuIngressPreparation{
		Body:        raw,
		OwnerUserID: config.OwnerUserID,
		AppID:       config.AppID,
	}, nil
}

func (s *ControlService) prepareEncryptedFeishuIngress(
	raw []byte,
	header http.Header,
	encryptValue string,
	configs []feishuIngressConfig,
) (FeishuIngressPreparation, error) {
	for _, config := range configs {
		if strings.TrimSpace(config.EncryptKey) == "" {
			continue
		}
		plain, decryptErr := channeladapters.DecryptFeishuEncryptedPayload(encryptValue, config.EncryptKey)
		if decryptErr != nil {
			continue
		}
		callback, decodeErr := channeladapters.DecodeFeishuIngressCallback(plain)
		if decodeErr != nil {
			continue
		}
		if strings.TrimSpace(callback.AppID) != "" && strings.TrimSpace(callback.AppID) != strings.TrimSpace(config.AppID) {
			continue
		}
		if err := channeladapters.VerifyFeishuCallbackSignature(raw, header, config.EncryptKey); err != nil {
			return FeishuIngressPreparation{}, err
		}
		if err := channeladapters.VerifyFeishuCallbackToken(callback, config.VerificationToken); err != nil {
			return FeishuIngressPreparation{}, err
		}
		return FeishuIngressPreparation{
			Body:        plain,
			OwnerUserID: config.OwnerUserID,
			AppID:       config.AppID,
		}, nil
	}
	return FeishuIngressPreparation{}, fmt.Errorf("%w: encrypted feishu callback did not match configured apps", ErrFeishuCallbackUnauthorized)
}

func (s *ControlService) listFeishuIngressConfigs(ctx context.Context) ([]feishuIngressConfig, error) {
	rows, err := s.listAllChannelConfigRows(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]feishuIngressConfig, 0, len(rows))
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled || row.ChannelType != ChannelTypeFeishu {
			continue
		}
		publicConfig, decodeErr := decodeStringMap(row.ConfigJSON)
		if decodeErr != nil {
			return nil, decodeErr
		}
		secrets, decryptErr := s.decryptCredentials(row.CredentialsEncrypted)
		if decryptErr != nil {
			return nil, decryptErr
		}
		result = append(result, feishuIngressConfig{
			OwnerUserID:       strings.TrimSpace(row.OwnerUserID),
			AppID:             strings.TrimSpace(publicConfig["app_id"]),
			VerificationToken: strings.TrimSpace(secrets["verification_token"]),
			EncryptKey:        strings.TrimSpace(secrets["encrypt_key"]),
		})
	}
	return result, nil
}

func matchFeishuIngressConfig(configs []feishuIngressConfig, callback FeishuIngressCallback) *feishuIngressConfig {
	appID := strings.TrimSpace(callback.AppID)
	token := strings.TrimSpace(callback.Token)
	for index := range configs {
		config := &configs[index]
		if appID != "" && strings.TrimSpace(config.AppID) == appID {
			return config
		}
	}
	if appID != "" {
		return nil
	}
	for index := range configs {
		config := &configs[index]
		if token != "" && strings.TrimSpace(config.VerificationToken) == token {
			return config
		}
	}
	return nil
}
