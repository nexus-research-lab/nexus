// INPUT: ephemeral internal login reference and human-only presentation.
// OUTPUT: AES-GCM ciphertext persisted only while a flow is active.
// POS: secret/material isolation boundary; terminal transitions scrub both columns.
package channelauthorization

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
)

type runtimeReference struct {
	LoginID     string `json:"login_id"`
	ChannelType string `json:"channel_type"`
}

func (s *Service) encryptValue(value any) (string, error) {
	if len(s.encryptionKey) != 32 {
		return "", errors.New("Channel 授权加密密钥不可用")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return credentials.EncryptPayload(s.encryptionKey, payload)
}

func (s *Service) decryptRuntimeReference(ciphertext string) (runtimeReference, error) {
	var result runtimeReference
	if strings.TrimSpace(ciphertext) == "" {
		return result, errors.New("Channel 授权缺少 runtime reference")
	}
	payload, err := credentials.DecryptPayload(s.encryptionKey, ciphertext)
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
	if strings.TrimSpace(ciphertext) == "" {
		return result, errors.New("Channel 授权缺少人类展示数据")
	}
	payload, err := credentials.DecryptPayload(s.encryptionKey, ciphertext)
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
