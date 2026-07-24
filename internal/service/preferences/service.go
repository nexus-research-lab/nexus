package preferences

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
)

// Service 负责读写用户级偏好 JSON。
type Service struct {
	config config.Config
}

// storedWebSearchCredential 是 WebSearch 凭据文件的唯一存储格式。
// provider 与 api_key 必须成对存在，避免不同 provider 复用同一份密钥。
type storedWebSearchCredential struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
}

// NewService 创建偏好服务。
func NewService(cfg config.Config) *Service {
	return &Service{config: cfg}
}

// Get 读取用户偏好，不存在时返回默认值。
func (s *Service) Get(_ context.Context, ownerUserID string) (Preferences, error) {
	root, err := s.openOwnerRoot(ownerUserID, false)
	if errors.Is(err, os.ErrNotExist) {
		return s.withWebSearchAPIKey(ownerUserID, DefaultPreferences()), nil
	}
	if err != nil {
		return Preferences{}, err
	}
	defer root.Close()
	content, err := root.ReadFile(".settings/preferences.json")
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
	webSearchAPIKeyChanged := request.WebSearchAPIKey != nil
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
		previousProvider := current.WebSearch.Provider
		apiKey := current.WebSearchAPIKey()
		current.WebSearch = *request.WebSearch
		current.WebSearch = normalizeWebSearchSettings(current.WebSearch)
		if current.WebSearch.Provider != previousProvider || !webSearchProviderAcceptsAPIKey(current.WebSearch.Provider) {
			apiKey = ""
			webSearchAPIKeyChanged = true
		}
		current.WebSearch = current.WebSearch.WithWebSearchAPIKey(apiKey)
	}
	if request.WebSearchAPIKey != nil {
		apiKey := strings.TrimSpace(*request.WebSearchAPIKey)
		current.WebSearch = current.WebSearch.WithWebSearchAPIKey(apiKey)
		if apiKey == "" && webSearchProviderRequiresAPIKey(current.WebSearch.Provider) {
			current.WebSearch.Enabled = false
		}
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
	if err = validateWebSearchSettings(current.WebSearch); err != nil {
		return Preferences{}, err
	}
	if err = s.write(ownerUserID, current); err != nil {
		return Preferences{}, err
	}
	if webSearchAPIKeyChanged {
		if err = s.writeWebSearchCredential(
			ownerUserID,
			current.WebSearch.Provider,
			current.WebSearchAPIKey(),
		); err != nil {
			return Preferences{}, err
		}
	}
	return current, nil
}

func (s *Service) write(ownerUserID string, item Preferences) error {
	root, err := s.openOwnerRoot(ownerUserID, true)
	if err != nil {
		return err
	}
	defer root.Close()
	payload, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err = root.MkdirAll(".settings", 0o700); err != nil {
		return err
	}
	if err = root.WriteFileAtomic(".settings/preferences.json", payload, 0o600); err != nil {
		return err
	}
	return root.Chmod(".settings/preferences.json", 0o600)
}

func (s *Service) preferencesPath(ownerUserID string) string {
	return filepath.Join(
		agentpkg.UserWorkspaceBasePath(s.config, ownerUserID),
		".settings",
		"preferences.json",
	)
}

func (s *Service) webSearchCredentialPath(ownerUserID string) string {
	return filepath.Join(
		agentpkg.UserWorkspaceBasePath(s.config, ownerUserID),
		".settings",
		"web-search-api-key",
	)
}

func (s *Service) withWebSearchAPIKey(ownerUserID string, item Preferences) Preferences {
	if !webSearchProviderAcceptsAPIKey(item.WebSearch.Provider) {
		return item
	}
	credential, ok := s.readWebSearchCredential(ownerUserID)
	if !ok || credential.Provider != strings.ToLower(strings.TrimSpace(item.WebSearch.Provider)) {
		return item
	}
	item.WebSearch = item.WebSearch.WithWebSearchAPIKey(credential.APIKey)
	if item.WebSearch.APIKeyConfigured {
		item.WebSearch.Enabled = true
	}
	return item
}

func (s *Service) readWebSearchCredential(ownerUserID string) (storedWebSearchCredential, bool) {
	root, err := s.openOwnerRoot(ownerUserID, false)
	if err != nil {
		return storedWebSearchCredential{}, false
	}
	defer root.Close()
	content, err := root.ReadFile(".settings/web-search-api-key")
	if err != nil {
		return storedWebSearchCredential{}, false
	}
	// 旧版裸 key 没有 provider 归属，无法安全推断，按无效凭据处理。
	var credential storedWebSearchCredential
	if err = json.Unmarshal(content, &credential); err != nil {
		return storedWebSearchCredential{}, false
	}
	credential.Provider = strings.ToLower(strings.TrimSpace(credential.Provider))
	credential.APIKey = strings.TrimSpace(credential.APIKey)
	if credential.Provider == "" || credential.APIKey == "" {
		return storedWebSearchCredential{}, false
	}
	return credential, true
}

func (s *Service) writeWebSearchCredential(ownerUserID string, provider string, apiKey string) error {
	root, err := s.openOwnerRoot(ownerUserID, true)
	if err != nil {
		return err
	}
	defer root.Close()
	provider = strings.ToLower(strings.TrimSpace(provider))
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		if err := root.Remove(".settings/web-search-api-key"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := root.MkdirAll(".settings", 0o700); err != nil {
		return err
	}
	payload, err := json.Marshal(storedWebSearchCredential{
		APIKey:   apiKey,
		Provider: provider,
	})
	if err != nil {
		return err
	}
	if err = root.WriteFileAtomic(".settings/web-search-api-key", append(payload, '\n'), 0o600); err != nil {
		return err
	}
	return root.Chmod(".settings/web-search-api-key", 0o600)
}

func (s *Service) openOwnerRoot(ownerUserID string, create bool) (*confinedfs.Root, error) {
	rootPath := agentpkg.UserWorkspaceBasePath(s.config, ownerUserID)
	if create {
		if err := os.MkdirAll(rootPath, appfs.RuntimeCollaborativeDirectoryMode(0o700)); err != nil {
			return nil, err
		}
	}
	return confinedfs.Open(rootPath)
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
