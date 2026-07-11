package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
)

func (s *ControlService) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (s *ControlService) bindList(count int) string {
	items := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		items = append(items, s.bind(index))
	}
	return strings.Join(items, ",")
}

func (s *ControlService) ensureAgent(ctx context.Context, agentID string) error {
	if s.agents == nil {
		return nil
	}
	_, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	return err
}

func (s *ControlService) agentName(ctx context.Context, agentID string) string {
	if s.agents == nil || strings.TrimSpace(agentID) == "" {
		return ""
	}
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil || agentValue == nil {
		return ""
	}
	return strings.TrimSpace(agentValue.Name)
}

func (s *ControlService) defaultAgentForChannel(ctx context.Context, ownerUserID string, channelType string) (string, error) {
	row, err := s.getChannelConfigRow(ctx, ownerUserID, normalizeIMChannelType(channelType))
	if err != nil || row == nil {
		return "", err
	}
	return row.AgentID, nil
}

func (s *ControlService) encryptCredentials(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	if s.keyErr != nil && strings.TrimSpace(s.config.ConnectorCredentialsKey) != "" {
		return "", fmt.Errorf("CONNECTOR_CREDENTIALS_KEY 解析失败: %w", s.keyErr)
	}
	if len(s.key) == 0 {
		return "", errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法保存 IM 通道凭据")
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return credentials.EncryptPayload(s.key, payload)
}

func (s *ControlService) decryptCredentials(encrypted sql.NullString) (map[string]string, error) {
	if !encrypted.Valid || strings.TrimSpace(encrypted.String) == "" {
		return nil, nil
	}
	if s.keyErr != nil && strings.TrimSpace(s.config.ConnectorCredentialsKey) != "" {
		return nil, fmt.Errorf("CONNECTOR_CREDENTIALS_KEY 解析失败: %w", s.keyErr)
	}
	if len(s.key) == 0 {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法读取 IM 通道凭据")
	}
	payload, err := credentials.DecryptPayload(s.key, encrypted.String)
	if err != nil {
		return nil, err
	}
	var result map[string]string
	if err = json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return normalizeStringMap(result), nil
}
