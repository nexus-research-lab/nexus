package preferences

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
)

// Service 负责读写用户级偏好 JSON。
type Service struct {
	config config.Config
}

// NewService 创建偏好服务。
func NewService(cfg config.Config) *Service {
	return &Service{config: cfg}
}

// Get 读取用户偏好，不存在时返回默认值。
func (s *Service) Get(_ context.Context, ownerUserID string) (Preferences, error) {
	path := s.preferencesPath(ownerUserID)
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultPreferences(), nil
	}
	if err != nil {
		return Preferences{}, err
	}
	return decodePreferences(content)
}

// Update 合并并写入用户偏好。
func (s *Service) Update(ctx context.Context, ownerUserID string, request UpdateRequest) (Preferences, error) {
	current, err := s.Get(ctx, ownerUserID)
	if err != nil {
		return Preferences{}, err
	}
	if request.ChatDefaultDeliveryPolicy != nil {
		current.ChatDefaultDeliveryPolicy = protocol.NormalizeChatDeliveryPolicy(*request.ChatDefaultDeliveryPolicy)
	}
	if request.DefaultAgentOptions != nil {
		current.DefaultAgentOptions = *request.DefaultAgentOptions
	}
	current.UpdatedAt = nowRFC3339()
	current = normalizePreferences(current)
	if err = s.write(ownerUserID, current); err != nil {
		return Preferences{}, err
	}
	return current, nil
}

func (s *Service) write(ownerUserID string, item Preferences) error {
	path := s.preferencesPath(ownerUserID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	tmpPath := path + ".tmp"
	if err = os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (s *Service) preferencesPath(ownerUserID string) string {
	return filepath.Join(
		agentpkg.UserWorkspaceBasePath(s.config, ownerUserID),
		".settings",
		"preferences.json",
	)
}

func decodePreferences(content []byte) (Preferences, error) {
	var item Preferences
	if err := json.Unmarshal(content, &item); err != nil {
		return Preferences{}, err
	}
	normalized := normalizePreferences(item)
	if normalized.UpdatedAt == "" {
		normalized.UpdatedAt = nowRFC3339()
	}
	return normalized, nil
}
