// INPUT: ephemeral internal login reference and human-only presentation.
// OUTPUT: 带稳定 key identity 的 AES-GCM ciphertext；旧 v1 只经显式 legacy keyring 读取。
// POS: secret/material isolation boundary; terminal transitions scrub both columns.
package channelauthorization

import (
	"encoding/json"
	"errors"
	"strings"
)

type runtimeReference struct {
	LoginID     string `json:"login_id"`
	ChannelType string `json:"channel_type"`
}

func (s *Service) encryptValue(value any) (string, error) {
	if s.keyring == nil || s.keyErr != nil {
		return "", errors.New("Channel 授权加密密钥不可用")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return s.keyring.EncryptEnvelope(payload)
}

func (s *Service) decryptRuntimeReference(ciphertext string) (runtimeReference, error) {
	var result runtimeReference
	if s.keyring == nil || s.keyErr != nil {
		return result, errors.New("Channel 授权加密密钥不可用")
	}
	if strings.TrimSpace(ciphertext) == "" {
		return result, errors.New("Channel 授权缺少 runtime reference")
	}
	payload, err := s.keyring.DecryptEnvelope(ciphertext)
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(payload, &result); err != nil {
		return result, err
	}
	if strings.TrimSpace(result.LoginID) == "" || strings.TrimSpace(result.ChannelType) == "" {
		return runtimeReference{}, errors.New("Channel 授权 runtime reference 无效")
	}
	return result, nil
}

func (s *Service) decryptHumanPresentation(ciphertext string) (HumanPresentation, error) {
	var result HumanPresentation
	if s.keyring == nil || s.keyErr != nil {
		return result, errors.New("Channel 授权加密密钥不可用")
	}
	if strings.TrimSpace(ciphertext) == "" {
		return result, errors.New("Channel 授权缺少人类展示数据")
	}
	payload, err := s.keyring.DecryptEnvelope(ciphertext)
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(payload, &result); err != nil {
		return result, err
	}
	if strings.TrimSpace(result.FlowID) == "" ||
		strings.TrimSpace(result.PresentationToken) == "" {
		return HumanPresentation{}, errors.New("Channel 授权人类展示数据无效")
	}
	return result, nil
}
