// INPUT: owner-scoped Preferences 读取、局部更新、CAS 更新与条件回滚。
// OUTPUT: 进程内按 owner 串行、version 单调且以原子文件替换持久化的偏好和凭据。
// POS: Preferences 文件真相源的唯一读写事务边界；调用方不得自行拼接 RMW 流程。
package preferences

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// Service 负责读写用户级偏好 JSON。
type Service struct {
	config     config.Config
	ownerLocks sync.Map
}

// ErrVersionConflict 表示 Preferences 已被另一条 UI 或对话写流程更新。
var ErrVersionConflict = errors.New("preferences version conflict")

// UpdateBuilder 在 owner 锁内基于最新 Preferences 构造局部更新。
// Builder 不得重入同一 Service 的 Get/Update 方法。
type UpdateBuilder func(Preferences) (UpdateRequest, error)

// NewService 创建偏好服务。
func NewService(cfg config.Config) *Service {
	return &Service{config: cfg}
}

// Get 读取用户偏好，不存在时返回默认值。
func (s *Service) Get(_ context.Context, ownerUserID string) (Preferences, error) {
	unlock := s.lockOwner(ownerUserID)
	defer unlock()
	return s.getLocked(ownerUserID)
}

func (s *Service) getLocked(ownerUserID string) (Preferences, error) {
	root, err := s.openOwnerRoot(ownerUserID, false)
	if errors.Is(err, os.ErrNotExist) {
		return s.withWebSearchAPIKeyConfined(ownerUserID, DefaultPreferences()), nil
	}
	if err != nil {
		return Preferences{}, err
	}
	defer root.Close()
	content, err := root.ReadFile(".settings/preferences.json")
	if errors.Is(err, os.ErrNotExist) {
		return s.withWebSearchAPIKeyConfined(ownerUserID, DefaultPreferences()), nil
	}
	if err != nil {
		return Preferences{}, err
	}
	item, err := decodePreferences(content)
	if err != nil {
		return Preferences{}, err
	}
	return s.withWebSearchAPIKeyConfined(ownerUserID, item), nil
}

// Update 合并并写入用户偏好。
func (s *Service) Update(ctx context.Context, ownerUserID string, request UpdateRequest) (Preferences, error) {
	return s.update(ctx, ownerUserID, nil, func(Preferences) (UpdateRequest, error) {
		return request, nil
	})
}

// SetEchoEnabled 保存主动跟进开关；调用方仍负责关闭后的业务收口。
func (s *Service) SetEchoEnabled(
	ctx context.Context,
	ownerUserID string,
	enabled bool,
) (Preferences, error) {
	return s.update(ctx, ownerUserID, nil, func(Preferences) (UpdateRequest, error) {
		return UpdateRequest{echoEnabled: &enabled}, nil
	})
}

// SetEchoEnabledAtVersion 仅在 Preferences aggregate 仍为调用方读取的版本时保存主动跟进开关。
func (s *Service) SetEchoEnabledAtVersion(
	ctx context.Context,
	ownerUserID string,
	enabled bool,
	expectedVersion int64,
) (Preferences, error) {
	return s.update(ctx, ownerUserID, &expectedVersion, func(Preferences) (UpdateRequest, error) {
		return UpdateRequest{echoEnabled: &enabled}, nil
	})
}

// UpdateAtVersion 仅在当前持久化 version 等于 expectedVersion 时合并并写入偏好。
func (s *Service) UpdateAtVersion(
	ctx context.Context,
	ownerUserID string,
	request UpdateRequest,
	expectedVersion int64,
) (Preferences, error) {
	return s.update(ctx, ownerUserID, &expectedVersion, func(Preferences) (UpdateRequest, error) {
		return request, nil
	})
}

// UpdatePrepared 在 owner 锁内基于最新值构造普通非 CAS 更新。
func (s *Service) UpdatePrepared(
	ctx context.Context,
	ownerUserID string,
	builder UpdateBuilder,
) (Preferences, error) {
	if builder == nil {
		return Preferences{}, errors.New("preferences update builder 不能为空")
	}
	return s.update(ctx, ownerUserID, nil, builder)
}

// UpdatePreparedAtVersion 在 owner 锁和 version CAS 边界内，让调用方基于最新值构造更新。
func (s *Service) UpdatePreparedAtVersion(
	ctx context.Context,
	ownerUserID string,
	expectedVersion int64,
	builder UpdateBuilder,
) (Preferences, error) {
	if builder == nil {
		return Preferences{}, errors.New("preferences update builder 不能为空")
	}
	return s.update(ctx, ownerUserID, &expectedVersion, builder)
}

func (s *Service) update(
	_ context.Context,
	ownerUserID string,
	expectedVersion *int64,
	builder UpdateBuilder,
) (Preferences, error) {
	unlock := s.lockOwner(ownerUserID)
	defer unlock()

	current, err := s.getLocked(ownerUserID)
	if err != nil {
		return Preferences{}, err
	}
	if expectedVersion != nil && current.Version != *expectedVersion {
		return Preferences{}, fmt.Errorf(
			"%w: expected=%d current=%d",
			ErrVersionConflict,
			*expectedVersion,
			current.Version,
		)
	}
	request, err := builder(current)
	if err != nil {
		return Preferences{}, err
	}
	return s.updateLocked(ownerUserID, current, request)
}

func (s *Service) updateLocked(
	ownerUserID string,
	current Preferences,
	request UpdateRequest,
) (Preferences, error) {
	previous := current
	if request.ChatDefaultDeliveryPolicy != nil {
		current.ChatDefaultDeliveryPolicy = *request.ChatDefaultDeliveryPolicy
	}
	if request.AgentRuntimeKind != nil {
		current.AgentRuntimeKind = *request.AgentRuntimeKind
	}
	if request.AgentSDKDiagnosticsEnabled != nil {
		current.AgentSDKDiagnosticsEnabled = *request.AgentSDKDiagnosticsEnabled
	}
	if request.EmotionEnabled != nil {
		current.EmotionEnabled = *request.EmotionEnabled
	}
	if request.BrowserCDPEnabled != nil {
		current.BrowserCDPEnabled = *request.BrowserCDPEnabled
	}
	if request.echoEnabled != nil {
		current.EchoEnabled = *request.echoEnabled
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
	if current.Version == math.MaxInt64 {
		return Preferences{}, errors.New("preferences version 已达到上限")
	}
	current.Version++
	current.UpdatedAt = nowRFC3339()
	current = normalizePreferences(current)
	if err := validateWebSearchSettings(current.WebSearch); err != nil {
		return Preferences{}, err
	}
	if err := s.commitPreferencesLocked(ownerUserID, previous, current); err != nil {
		return Preferences{}, err
	}
	return current, nil
}

// RestoreIfVersion 在没有后续写入时恢复旧值；恢复本身仍推进 version。
func (s *Service) RestoreIfVersion(
	_ context.Context,
	ownerUserID string,
	expectedVersion int64,
	previous Preferences,
) (Preferences, bool, error) {
	unlock := s.lockOwner(ownerUserID)
	defer unlock()

	current, err := s.getLocked(ownerUserID)
	if err != nil {
		return Preferences{}, false, err
	}
	if current.Version != expectedVersion {
		return current, false, nil
	}
	if current.Version == math.MaxInt64 {
		return Preferences{}, false, errors.New("preferences version 已达到上限")
	}
	restored := previous
	restored.Version = current.Version + 1
	restored.UpdatedAt = nowRFC3339()
	restored = normalizePreferences(restored)
	if err = validateWebSearchSettings(restored.WebSearch); err != nil {
		return Preferences{}, false, err
	}
	if err = s.commitPreferencesLocked(ownerUserID, current, restored); err != nil {
		return Preferences{}, false, err
	}
	return restored, true, nil
}

// commitPreferencesLocked 用 version 作为两个原子文件之间的发布指针：
// 先持久化旧、新双代凭据，再发布 Preferences，最后清理旧代。任一崩溃点
// 都只会让读取方看到与已发布 Preferences.version 精确匹配的凭据。
func (s *Service) commitPreferencesLocked(
	ownerUserID string,
	previous Preferences,
	next Preferences,
) error {
	before := s.readWebSearchCredentialBundleConfined(ownerUserID)
	staged := credentialBundleForTransition(previous, next)
	if err := s.writeWebSearchCredentialBundleConfined(ownerUserID, staged); err != nil {
		return fmt.Errorf("暂存 Preferences 凭据版本: %w", err)
	}
	if err := s.writePreferencesConfined(ownerUserID, next); err != nil {
		restoreErr := s.writeWebSearchCredentialBundleConfined(ownerUserID, before)
		if restoreErr != nil {
			restoreErr = fmt.Errorf("恢复 Preferences 凭据版本: %w", restoreErr)
		}
		return errors.Join(err, restoreErr)
	}
	if err := s.writeWebSearchCredentialBundleConfined(
		ownerUserID,
		credentialBundleForCurrent(next),
	); err != nil {
		return fmt.Errorf(
			"Preferences version=%d 已发布，但旧凭据代清理失败: %w",
			next.Version,
			err,
		)
	}
	return nil
}

func (s *Service) writePreferencesConfined(ownerUserID string, item Preferences) error {
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
	if err := root.MkdirAll(
		".settings",
		appfs.RuntimeCollaborativeDirectoryMode(0o700),
	); err != nil {
		return err
	}
	return root.WriteFileAtomic(
		".settings/preferences.json",
		payload,
		appfs.RuntimeCollaborativeFileMode(0o600),
	)
}

func (s *Service) withWebSearchAPIKeyConfined(
	ownerUserID string,
	item Preferences,
) Preferences {
	if !webSearchProviderAcceptsAPIKey(item.WebSearch.Provider) {
		return item
	}
	credential := s.readWebSearchCredentialBundleConfined(ownerUserID).
		credentialForVersion(item.Version)
	if credential.Provider != strings.ToLower(strings.TrimSpace(item.WebSearch.Provider)) {
		return item
	}
	item.WebSearch = item.WebSearch.WithWebSearchAPIKey(credential.APIKey)
	if item.WebSearch.APIKeyConfigured {
		item.WebSearch.Enabled = true
	}
	return item
}

func (s *Service) readWebSearchCredentialBundleConfined(
	ownerUserID string,
) storedWebSearchCredentialBundle {
	root, err := s.openOwnerRoot(ownerUserID, false)
	if err != nil {
		return storedWebSearchCredentialBundle{}
	}
	defer root.Close()
	settingsRoot, err := root.OpenRootNoSymlink(".settings")
	if err != nil {
		return storedWebSearchCredentialBundle{}
	}
	defer settingsRoot.Close()
	file, err := settingsRoot.OpenFileNoSymlink("web-search-api-key", os.O_RDONLY, 0)
	if err != nil {
		return storedWebSearchCredentialBundle{}
	}
	content, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		return storedWebSearchCredentialBundle{}
	}
	return decodeWebSearchCredentialBundle(content)
}

func (s *Service) writeWebSearchCredentialBundleConfined(
	ownerUserID string,
	bundle storedWebSearchCredentialBundle,
) error {
	root, err := s.openOwnerRoot(ownerUserID, true)
	if err != nil {
		return err
	}
	defer root.Close()
	bundle = normalizeWebSearchCredentialBundle(bundle)
	if credentialBundleEmpty(bundle) {
		if err = root.Remove(".settings/web-search-api-key"); err != nil &&
			!errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err = root.MkdirAll(
		".settings",
		appfs.RuntimeCollaborativeDirectoryMode(0o700),
	); err != nil {
		return err
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	return root.WriteFileAtomic(
		".settings/web-search-api-key",
		append(payload, '\n'),
		appfs.RuntimeCollaborativeFileMode(0o600),
	)
}

func (s *Service) openOwnerRoot(ownerUserID string, create bool) (*confinedfs.Root, error) {
	rootPath := agentpkg.UserWorkspaceBasePath(s.config, ownerUserID)
	return workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		rootPath,
		create,
	)
}

func (s *Service) lockOwner(ownerUserID string) func() {
	key := strings.TrimSpace(ownerUserID)
	value, _ := s.ownerLocks.LoadOrStore(key, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}
