// INPUT: 结构化 Agent Session key 与 Session 级模型、权限、Connector 覆盖。
// OUTPUT: DM 文件会话或 Room 成员会话上的同构运行时设置。
// POS: Nexus Session 设置的唯一持久化事务，不修改 Agent 默认值。
package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

// ErrInvalidRuntimeSettings 表示 Session 运行时覆盖不符合协议约束。
var ErrInvalidRuntimeSettings = errors.New("Session 运行时设置无效")

var (
	// ErrLocalDirectoriesUnavailable 表示请求不具备桌面端授权。
	ErrLocalDirectoriesUnavailable = errors.New("本机目录仅可在 Nexus 桌面端使用")
	// ErrInvalidLocalDirectories 表示目录快照不符合本机路径约束。
	ErrInvalidLocalDirectories = errors.New("Session 本机目录无效")
)

const (
	maxSessionLocalDirectories = 32
	maxSessionLocalPathLength  = 4096
)

// GetRuntimeSettings 返回当前 Session 的显式覆盖；空字段表示继承。
func (s *Service) GetRuntimeSettings(
	ctx context.Context,
	rawSessionKey string,
) (protocol.SessionRuntimeSettings, error) {
	item, _, _, err := s.loadRuntimeSettingsSession(ctx, rawSessionKey)
	if err != nil {
		return protocol.SessionRuntimeSettings{}, err
	}
	return protocol.SessionRuntimeSettingsFromOptions(item.Options), nil
}

// UpdateRuntimeSettings 以完整快照更新模型，并统一 Room Conversation 权限。
func (s *Service) UpdateRuntimeSettings(
	ctx context.Context,
	rawSessionKey string,
	settings protocol.SessionRuntimeSettings,
) (protocol.SessionRuntimeSettings, error) {
	normalized, err := normalizeRuntimeSettings(settings)
	if err != nil {
		return protocol.SessionRuntimeSettings{}, err
	}
	item, workspacePath, isRoomSession, err := s.loadRuntimeSettingsSession(
		ctx,
		rawSessionKey,
	)
	if err != nil {
		return protocol.SessionRuntimeSettings{}, err
	}
	item.Options = protocol.WithSessionRuntimeSettings(item.Options, normalized)
	if isRoomSession {
		if item.RoomSessionID == nil || strings.TrimSpace(*item.RoomSessionID) == "" {
			return protocol.SessionRuntimeSettings{}, ErrSessionNotFound
		}
		updatedSessions, updateErr := s.repository.UpdateRoomConversationRuntimeSettings(
			ctx,
			strings.TrimSpace(*item.RoomSessionID),
			item.Options,
			normalized.PermissionMode,
		)
		if updateErr != nil {
			return protocol.SessionRuntimeSettings{}, updateErr
		}
		for _, updatedSession := range updatedSessions {
			s.notifyDirectoryChanged(
				ctx,
				"session_runtime_settings_updated",
				updatedSession,
			)
		}
	} else if _, err = s.ownerFiles(ctx).UpsertSession(workspacePath, *item); err != nil {
		return protocol.SessionRuntimeSettings{}, err
	} else {
		s.notifyDirectoryChanged(ctx, "session_runtime_settings_updated", *item)
	}
	return normalized, nil
}

// GetLocalDirectories 返回当前 Session 的额外挂载目录。
func (s *Service) GetLocalDirectories(
	ctx context.Context,
	rawSessionKey string,
) (protocol.SessionLocalDirectories, error) {
	if err := s.requireLocalDirectoryAccess(ctx); err != nil {
		return protocol.SessionLocalDirectories{}, err
	}
	item, _, _, err := s.loadRuntimeSettingsSession(ctx, rawSessionKey)
	if err != nil {
		return protocol.SessionLocalDirectories{}, err
	}
	if protocol.NormalizeSessionChatType(item.ChatType) != protocol.RoomTypeDM {
		return protocol.SessionLocalDirectories{}, ErrSessionMutationUnsupported
	}
	directories := protocol.SessionAdditionalDirectoriesFromOptions(item.Options)
	if directories == nil {
		directories = []string{}
	}
	return protocol.SessionLocalDirectories{Directories: directories}, nil
}

// UpdateLocalDirectories 以完整快照更新当前 DM Session 的本机目录。
func (s *Service) UpdateLocalDirectories(
	ctx context.Context,
	rawSessionKey string,
	payload protocol.SessionLocalDirectories,
) (protocol.SessionLocalDirectories, error) {
	if err := s.requireLocalDirectoryAccess(ctx); err != nil {
		return protocol.SessionLocalDirectories{}, err
	}
	directories, err := normalizeSessionLocalDirectories(payload.Directories)
	if err != nil {
		return protocol.SessionLocalDirectories{}, err
	}
	item, workspacePath, isRoomSession, err := s.loadRuntimeSettingsSession(
		ctx,
		rawSessionKey,
	)
	if err != nil {
		return protocol.SessionLocalDirectories{}, err
	}
	if protocol.NormalizeSessionChatType(item.ChatType) != protocol.RoomTypeDM {
		return protocol.SessionLocalDirectories{}, ErrSessionMutationUnsupported
	}
	item.Options = protocol.WithSessionAdditionalDirectories(
		item.Options,
		directories,
	)
	if isRoomSession {
		if item.RoomSessionID == nil || strings.TrimSpace(*item.RoomSessionID) == "" {
			return protocol.SessionLocalDirectories{}, ErrSessionNotFound
		}
		settings := protocol.SessionRuntimeSettingsFromOptions(item.Options)
		updatedSessions, updateErr := s.repository.UpdateRoomConversationRuntimeSettings(
			ctx,
			strings.TrimSpace(*item.RoomSessionID),
			item.Options,
			settings.PermissionMode,
		)
		if updateErr != nil {
			return protocol.SessionLocalDirectories{}, updateErr
		}
		for _, updatedSession := range updatedSessions {
			s.notifyDirectoryChanged(
				ctx,
				"session_runtime_settings_updated",
				updatedSession,
			)
		}
	} else if _, err = s.ownerFiles(ctx).UpsertSession(workspacePath, *item); err != nil {
		return protocol.SessionLocalDirectories{}, err
	} else {
		s.notifyDirectoryChanged(ctx, "session_runtime_settings_updated", *item)
	}
	return protocol.SessionLocalDirectories{Directories: directories}, nil
}

func (s *Service) requireLocalDirectoryAccess(ctx context.Context) error {
	evidence, ok := authctx.InteractiveHumanEvidenceFromContext(ctx)
	if !ok || evidence.Source != "desktop_session_token" {
		return ErrLocalDirectoriesUnavailable
	}
	return nil
}

func normalizeSessionLocalDirectories(values []string) ([]string, error) {
	if len(values) > maxSessionLocalDirectories {
		return nil, fmt.Errorf(
			"%w：最多挂载 %d 个目录",
			ErrInvalidLocalDirectories,
			maxSessionLocalDirectories,
		)
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > maxSessionLocalPathLength || !filepath.IsAbs(value) {
			return nil, fmt.Errorf("%w：目录必须是绝对路径", ErrInvalidLocalDirectories)
		}
		value = filepath.Clean(value)
		info, err := os.Stat(value)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("%w：目录不存在或不可访问", ErrInvalidLocalDirectories)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func (s *Service) loadRuntimeSettingsSession(
	ctx context.Context,
	rawSessionKey string,
) (*protocol.Session, string, bool, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, "", false, err
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return nil, "", false, ErrSessionMutationUnsupported
	}

	roomSession, err := s.repository.GetRoomSessionByKey(
		ctx,
		authctx.OwnerUserID(ctx),
		parsed,
	)
	if err != nil {
		return nil, "", false, err
	}
	if roomSession != nil {
		return roomSession, "", true, nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, "", false, err
	}
	item, workspacePath, err := s.ownerFiles(ctx).FindSession(
		workspacePaths,
		sessionKey,
	)
	if err != nil {
		return nil, "", false, err
	}
	if item == nil {
		return nil, "", false, ErrSessionNotFound
	}
	return item, workspacePath, false, nil
}

func normalizeRuntimeSettings(
	settings protocol.SessionRuntimeSettings,
) (protocol.SessionRuntimeSettings, error) {
	settings.Provider = strings.TrimSpace(settings.Provider)
	settings.Model = strings.TrimSpace(settings.Model)
	settings.PermissionMode = strings.TrimSpace(settings.PermissionMode)
	if settings.ConnectorIDs != nil {
		normalized := normalizeRuntimeConnectorIDs(*settings.ConnectorIDs)
		settings.ConnectorIDs = &normalized
	}
	if (settings.Provider == "") != (settings.Model == "") {
		return protocol.SessionRuntimeSettings{}, fmt.Errorf(
			"%w：provider 与 model 必须同时设置或同时清除",
			ErrInvalidRuntimeSettings,
		)
	}
	if settings.PermissionMode == "" {
		return settings, nil
	}
	normalizedMode := runtimepermission.NormalizeMode(
		sdkpermission.Mode(settings.PermissionMode),
	)
	if string(normalizedMode) != settings.PermissionMode ||
		normalizedMode == sdkpermission.ModeAuto {
		return protocol.SessionRuntimeSettings{}, fmt.Errorf(
			"%w：permission_mode 不受支持",
			ErrInvalidRuntimeSettings,
		)
	}
	settings.PermissionMode = string(normalizedMode)
	return settings, nil
}

func normalizeRuntimeConnectorIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
