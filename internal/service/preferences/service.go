package preferences

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
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
		return s.withWebSearchAPIKey(ownerUserID, DefaultPreferences()), nil
	}
	if err != nil {
		return Preferences{}, err
	}
	item, err := decodePreferences(content)
	if err != nil {
		return Preferences{}, err
	}
	return s.withWebSearchAPIKey(ownerUserID, item), nil
}

// Update 合并并写入用户偏好。
func (s *Service) Update(ctx context.Context, ownerUserID string, request UpdateRequest) (Preferences, error) {
	current, err := s.Get(ctx, ownerUserID)
	if err != nil {
		return Preferences{}, err
	}
	if request.ChatDefaultDeliveryPolicy != nil {
		current.ChatDefaultDeliveryPolicy = *request.ChatDefaultDeliveryPolicy
	}
	if request.AgentRuntimeKind != nil {
		current.AgentRuntimeKind = *request.AgentRuntimeKind
	}
	if request.AgentSDKDiagnosticsEnabled != nil {
		current.AgentSDKDiagnosticsEnabled = *request.AgentSDKDiagnosticsEnabled
	}
	if request.RuntimeSettings != nil {
		current.RuntimeSettings = *request.RuntimeSettings
	}
	if request.WebSearch != nil {
		apiKey := current.WebSearchAPIKey()
		current.WebSearch = *request.WebSearch
		current.WebSearch.APIKeyConfigured = apiKey != ""
		current.WebSearch = normalizeWebSearchSettings(current.WebSearch)
		current.WebSearch.apiKey = apiKey
		current.WebSearch.APIKeyConfigured = apiKey != ""
	}
	if request.WebSearchAPIKey != nil {
		current.WebSearch.apiKey = strings.TrimSpace(*request.WebSearchAPIKey)
		current.WebSearch.APIKeyConfigured = current.WebSearch.apiKey != ""
	}
	if request.DefaultAgentOptions != nil {
		current.DefaultAgentOptions = *request.DefaultAgentOptions
	}
	if request.DefaultImageModelSelection != nil {
		current.DefaultImageModelSelection = *request.DefaultImageModelSelection
	}
	if request.DefaultVisionModelSelection != nil {
		current.DefaultVisionModelSelection = *request.DefaultVisionModelSelection
	}
	if request.DefaultBackgroundModelSelection != nil {
		current.DefaultBackgroundModelSelection = *request.DefaultBackgroundModelSelection
	}
	current.UpdatedAt = nowRFC3339()
	current = normalizePreferences(current)
	if err = s.write(ownerUserID, current); err != nil {
		return Preferences{}, err
	}
	if request.WebSearchAPIKey != nil {
		if err = s.writeWebSearchAPIKey(ownerUserID, current.WebSearchAPIKey()); err != nil {
			return Preferences{}, err
		}
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

func (s *Service) webSearchAPIKeyPath(ownerUserID string) string {
	return filepath.Join(
		agentpkg.UserWorkspaceBasePath(s.config, ownerUserID),
		".settings",
		"web-search-api-key",
	)
}

func (s *Service) withWebSearchAPIKey(ownerUserID string, item Preferences) Preferences {
	apiKey, err := os.ReadFile(s.webSearchAPIKeyPath(ownerUserID))
	if err != nil {
		return item
	}
	item.WebSearch.apiKey = strings.TrimSpace(string(apiKey))
	item.WebSearch.APIKeyConfigured = item.WebSearch.apiKey != ""
	return item
}

func (s *Service) writeWebSearchAPIKey(ownerUserID string, apiKey string) error {
	path := s.webSearchAPIKeyPath(ownerUserID)
	if apiKey == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(apiKey+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
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
