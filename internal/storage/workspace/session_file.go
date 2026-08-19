// INPUT: owner-confined Agent workspace、结构化 session_key 与可选 expected configuration_version。
// OUTPUT: 共享资源锁下原子创建、CAS 更新或删除的 session meta。
// POS: 所有 workspace session writer 的并发真相边界；Room ledger 不经过这里。
package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var (
	// ErrSessionConfigurationVersionConflict 表示 session meta 已被其他 writer 推进。
	ErrSessionConfigurationVersionConflict = errors.New("session configuration version conflict")
	// ErrSessionStorageIdentityMismatch 表示两个不同 session_key 命中了同一个历史目录名。
	// legacy 目录编码并非单射；任何读写在发现 meta 身份不一致时都必须 fail closed。
	ErrSessionStorageIdentityMismatch = errors.New("session storage identity mismatch")
	// ErrSessionDeleted 表示 session 已进入删除栅栏或已被持久删除，普通 writer 不得复活。
	ErrSessionDeleted    = errors.New("session is deleting or deleted")
	sessionMutationLocks [256]sync.Mutex
)

// SessionFileStore 负责 workspace 侧会话文件读写。
type SessionFileStore struct {
	paths       *Store
	ownerUserID string
}

// NewSessionFileStore 创建文件存储门面。
func NewSessionFileStore(root string) *SessionFileStore {
	return newSessionFileStore(New(root))
}

func newSessionFileStore(paths *Store) *SessionFileStore {
	return &SessionFileStore{paths: paths}
}

// ForOwner 返回绑定到单个 owner workspace 树的会话文件视图。
func (s *SessionFileStore) ForOwner(ownerUserID string) *SessionFileStore {
	if s == nil {
		return nil
	}
	return &SessionFileStore{
		paths:       s.paths,
		ownerUserID: strings.TrimSpace(ownerUserID),
	}
}

// ListSessions 读取某个 workspace 下的全部文件会话。
func (s *SessionFileStore) ListSessions(workspacePath string) ([]protocol.Session, error) {
	root, err := s.openWorkspaceRoot(workspacePath, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []protocol.Session{}, nil
		}
		return nil, err
	}
	defer root.Close()
	sessionsRoot, err := root.OpenRootNoSymlink(".agents/sessions")
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.Session{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer sessionsRoot.Close()
	entries, err := fs.ReadDir(sessionsRoot.FS(), ".")
	if err != nil {
		return nil, err
	}

	result := make([]protocol.Session, 0, len(entries))
	for _, entry := range entries {
		info, statErr := sessionsRoot.Lstat(entry.Name())
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		sessionRoot, openErr := sessionsRoot.OpenRootNoSymlink(entry.Name())
		if openErr != nil {
			continue
		}
		item, loadErr := readSessionMeta(sessionRoot, "meta.json")
		sessionRoot.Close()
		if errors.Is(loadErr, os.ErrNotExist) {
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		if strings.TrimSpace(item.SessionKey) == "" {
			return nil, fmt.Errorf(
				"%w: directory=%s has empty session_key",
				ErrSessionStorageIdentityMismatch,
				entry.Name(),
			)
		}
		if entry.Name() != encodeSessionDirName(item.SessionKey) {
			return nil, fmt.Errorf(
				"%w: directory=%q stored_session_key=%q expected_directory=%q",
				ErrSessionStorageIdentityMismatch,
				entry.Name(),
				strings.TrimSpace(item.SessionKey),
				encodeSessionDirName(item.SessionKey),
			)
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].LastActivity.After(result[j].LastActivity)
	})
	return result, nil
}

// FindSession 在多个 workspace 中定位单个 session。
func (s *SessionFileStore) FindSession(workspacePaths []string, sessionKey string) (*protocol.Session, string, error) {
	for _, workspacePath := range workspacePaths {
		root, openErr := s.openWorkspaceRoot(workspacePath, false)
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return nil, "", openErr
		}
		sessionRoot, openErr := root.OpenRootNoSymlink(filepath.ToSlash(filepath.Join(
			".agents",
			"sessions",
			encodeSessionDirName(sessionKey),
		)))
		root.Close()
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return nil, "", openErr
		}
		item, err := readSessionMeta(sessionRoot, "meta.json")
		sessionRoot.Close()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(item.SessionKey) != strings.TrimSpace(sessionKey) {
			return nil, "", fmt.Errorf(
				"%w: requested=%q stored=%q directory=%q",
				ErrSessionStorageIdentityMismatch,
				strings.TrimSpace(sessionKey),
				strings.TrimSpace(item.SessionKey),
				encodeSessionDirName(sessionKey),
			)
		}
		return &item, workspacePath, nil
	}
	return nil, "", nil
}

// UpsertSession 创建或更新 session meta。
func (s *SessionFileStore) UpsertSession(workspacePath string, item protocol.Session) (*protocol.Session, error) {
	unlock := lockSessionMutation(s.ownerUserID, workspacePath, item.SessionKey)
	defer unlock()
	return s.upsertSessionLocked(workspacePath, item, nil, true)
}

// UpsertSessionAtVersion 仅在 session configuration_version 匹配时写入。
func (s *SessionFileStore) UpsertSessionAtVersion(
	workspacePath string,
	item protocol.Session,
	expectedConfigurationVersion int64,
) (*protocol.Session, error) {
	if expectedConfigurationVersion < 1 {
		return nil, errors.New("expected session configuration_version 必须大于 0")
	}
	unlock := lockSessionMutation(s.ownerUserID, workspacePath, item.SessionKey)
	defer unlock()
	return s.upsertSessionLocked(
		workspacePath,
		item,
		&expectedConfigurationVersion,
		true,
	)
}

// PatchSessionRuntime 合并 runtime 拥有的热态字段，同时保留配置控制面拥有的标题和
// 稳定会话身份。调用方可以携带陈旧投影；合并始终在物理 session 锁内读取最新值。
func (s *SessionFileStore) PatchSessionRuntime(
	workspacePath string,
	item protocol.Session,
) (*protocol.Session, error) {
	unlock := lockSessionMutation(s.ownerUserID, workspacePath, item.SessionKey)
	defer unlock()
	return s.patchSessionRuntimeLocked(workspacePath, item, nil)
}

// PatchSessionRuntimeAtVersion 只在配置版本仍与后台 runtime 预备快照一致时合并热态。
// 成功不会推进 configuration_version；用户配置仍只由控制面 writer 推进。
func (s *SessionFileStore) PatchSessionRuntimeAtVersion(
	workspacePath string,
	item protocol.Session,
	expectedConfigurationVersion int64,
) (*protocol.Session, error) {
	if expectedConfigurationVersion < 1 {
		return nil, errors.New("expected session configuration_version 必须大于 0")
	}
	unlock := lockSessionMutation(s.ownerUserID, workspacePath, item.SessionKey)
	defer unlock()
	return s.patchSessionRuntimeLocked(
		workspacePath,
		item,
		&expectedConfigurationVersion,
	)
}

func (s *SessionFileStore) patchSessionRuntimeLocked(
	workspacePath string,
	item protocol.Session,
	expectedConfigurationVersion *int64,
) (*protocol.Session, error) {
	current, _, err := s.FindSession([]string{workspacePath}, item.SessionKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		if expectedConfigurationVersion != nil {
			return nil, os.ErrNotExist
		}
		return s.upsertSessionLocked(workspacePath, item, nil, false)
	}
	if expectedConfigurationVersion != nil &&
		current.ConfigurationVersion != *expectedConfigurationVersion {
		return nil, fmt.Errorf(
			"%w: expected=%d actual=%d",
			ErrSessionConfigurationVersionConflict,
			*expectedConfigurationVersion,
			current.ConfigurationVersion,
		)
	}
	merged := item
	merged.SessionKey = current.SessionKey
	merged.AgentID = current.AgentID
	merged.RoomSessionID = current.RoomSessionID
	merged.RoomID = current.RoomID
	merged.ConversationID = current.ConversationID
	merged.ChannelType = current.ChannelType
	merged.ChatType = current.ChatType
	merged.CreatedAt = current.CreatedAt
	merged.Title = current.Title
	merged.ConfigurationVersion = current.ConfigurationVersion
	return s.upsertSessionLocked(
		workspacePath,
		merged,
		&current.ConfigurationVersion,
		false,
	)
}

func (s *SessionFileStore) upsertSessionLocked(
	workspacePath string,
	item protocol.Session,
	expectedConfigurationVersion *int64,
	advanceConfigurationVersion bool,
) (*protocol.Session, error) {
	current, _, err := s.FindSession([]string{workspacePath}, item.SessionKey)
	if err != nil {
		return nil, err
	}
	if err = s.requireSessionWritableLocked(workspacePath, item.SessionKey); err != nil {
		return nil, err
	}
	switch {
	case current == nil:
		if expectedConfigurationVersion != nil {
			return nil, os.ErrNotExist
		}
		item.ConfigurationVersion = 1
	case expectedConfigurationVersion != nil &&
		current.ConfigurationVersion != *expectedConfigurationVersion:
		return nil, fmt.Errorf(
			"%w: expected=%d actual=%d",
			ErrSessionConfigurationVersionConflict,
			*expectedConfigurationVersion,
			current.ConfigurationVersion,
		)
	default:
		if item.MessageCount < current.MessageCount {
			item.MessageCount = current.MessageCount
		}
		if item.LastActivity.Before(current.LastActivity) {
			item.LastActivity = current.LastActivity
		}
		if !current.CreatedAt.IsZero() {
			item.CreatedAt = current.CreatedAt
		}
		// 非配置 runtime writer 也必须在同一锁内读取当前版本并单调推进，
		// 但不把调用方携带的陈旧投影误当成 CAS token。对话配置只使用
		// UpsertSessionAtVersion，因此 inspect/plan/apply 仍是严格 CAS。
		item.ConfigurationVersion = current.ConfigurationVersion
		if advanceConfigurationVersion {
			item.ConfigurationVersion++
		}
	}
	root, err := s.openOrCreateWorkspaceRoot(workspacePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	relative := filepath.ToSlash(filepath.Join(
		".agents",
		"sessions",
		encodeSessionDirName(item.SessionKey),
		"meta.json",
	))
	if err := root.MkdirAll(filepath.Dir(relative), storageDirectoryMode()); err != nil {
		return nil, err
	}

	// 这里直接以 Go 模型作为 meta 真相源，避免再复制一套弱类型结构。
	payload, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return nil, err
	}
	// 先写临时文件再 rename，避免并发 meta 刷新时读到半截 JSON。
	if err = root.WriteFileAtomic(relative, payload, storageFileMode(0o644)); err != nil {
		return nil, err
	}
	created, _, err := s.FindSession([]string{workspacePath}, item.SessionKey)
	return created, err
}

func (s *SessionFileStore) openOrCreateWorkspaceRoot(workspacePath string) (*confinedfs.Root, error) {
	return s.openWorkspaceRoot(workspacePath, true)
}

func (s *SessionFileStore) openWorkspaceRoot(
	workspacePath string,
	create bool,
) (*confinedfs.Root, error) {
	if strings.TrimSpace(s.ownerUserID) != "" {
		return s.paths.OpenOwnerWorkspacePath(
			s.ownerUserID,
			workspacePath,
			create,
		)
	}
	parent, relative, err := s.openStorePath(workspacePath, create)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if create {
		return parent.OpenOrCreateRootNoSymlink(relative, storageDirectoryMode())
	}
	return parent.OpenRootNoSymlink(relative)
}

// DeleteSession 删除整个 session 目录。
func (s *SessionFileStore) DeleteSession(workspacePath string, sessionKey string) (bool, error) {
	unlock := lockSessionMutation(s.ownerUserID, workspacePath, sessionKey)
	defer unlock()
	return s.deleteSessionLocked(workspacePath, sessionKey, nil)
}

// DeleteSessionAtVersion 仅在 session configuration_version 匹配时删除。
func (s *SessionFileStore) DeleteSessionAtVersion(
	workspacePath string,
	sessionKey string,
	expectedConfigurationVersion int64,
) (bool, error) {
	if expectedConfigurationVersion < 1 {
		return false, errors.New("expected session configuration_version 必须大于 0")
	}
	unlock := lockSessionMutation(s.ownerUserID, workspacePath, sessionKey)
	defer unlock()
	return s.deleteSessionLocked(
		workspacePath,
		sessionKey,
		&expectedConfigurationVersion,
	)
}

func (s *SessionFileStore) deleteSessionLocked(
	workspacePath string,
	sessionKey string,
	expectedConfigurationVersion *int64,
) (bool, error) {
	if expectedConfigurationVersion != nil {
		current, _, err := s.FindSession([]string{workspacePath}, sessionKey)
		if err != nil {
			return false, err
		}
		if current == nil {
			return false, nil
		}
		if current.ConfigurationVersion != *expectedConfigurationVersion {
			return false, fmt.Errorf(
				"%w: expected=%d actual=%d",
				ErrSessionConfigurationVersionConflict,
				*expectedConfigurationVersion,
				current.ConfigurationVersion,
			)
		}
	} else {
		current, _, err := s.FindSession([]string{workspacePath}, sessionKey)
		if err != nil {
			return false, err
		}
		if current == nil {
			return false, nil
		}
	}
	root, err := s.openWorkspaceRoot(workspacePath, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()
	sessionsRoot, err := root.OpenRootNoSymlink(".agents/sessions")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer sessionsRoot.Close()
	sessionDir := encodeSessionDirName(sessionKey)
	if _, err := sessionsRoot.Lstat(sessionDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := sessionsRoot.RemoveAll(sessionDir); err != nil {
		return false, err
	}
	return true, nil
}

func lockSessionMutation(
	ownerUserID string,
	workspacePath string,
	sessionKey string,
) func() {
	key := strings.Join([]string{
		strings.TrimSpace(ownerUserID),
		filepath.Clean(strings.TrimSpace(workspacePath)),
		// legacy session directory names are not injective. Lock the physical
		// identity so aliases cannot race two CAS writers against one directory.
		encodeSessionDirName(strings.TrimSpace(sessionKey)),
	}, "\x00")
	index := sessionMutationLockIndex(key)
	mutex := &sessionMutationLocks[index]
	mutex.Lock()
	return mutex.Unlock
}

func sessionMutationLockIndex(value string) byte {
	var hash uint32 = 2166136261
	for index := 0; index < len(value); index++ {
		hash ^= uint32(value[index])
		hash *= 16777619
	}
	return byte(hash)
}

// DeleteRoomConversation 删除指定用户的 Room ledger 与公共资产目录。
func (s *SessionFileStore) DeleteRoomConversation(ownerUserID string, conversationID string) (bool, error) {
	deletedState, err := s.deleteRoomConversationState(ownerUserID, conversationID)
	if err != nil {
		return false, err
	}
	deletedAssets, err := s.deleteRoomConversationAssets(ownerUserID, conversationID)
	if err != nil {
		return false, err
	}
	return deletedState || deletedAssets, nil
}

func (s *SessionFileStore) deleteRoomConversationState(
	ownerUserID string,
	conversationID string,
) (bool, error) {
	root, err := s.openRoomRoot(ownerUserID, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()
	return deleteConfinedDirectoryAtRoot(
		root,
		filepath.Base(s.paths.RoomConversationDir(ownerUserID, conversationID)),
	)
}

func (s *SessionFileStore) deleteRoomConversationAssets(
	ownerUserID string,
	conversationID string,
) (bool, error) {
	workspaceRoot, err := s.paths.openOwnerWorkspaceRoot(ownerUserID, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	assetsRoot, err := workspaceRoot.OpenRootNoSymlink(".rooms")
	workspaceRoot.Close()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer assetsRoot.Close()
	return deleteConfinedDirectoryAtRoot(
		assetsRoot,
		filepath.Base(s.paths.RoomConversationAssetDir(ownerUserID, conversationID)),
	)
}

func deleteConfinedDirectoryAtRoot(root *confinedfs.Root, relative string) (bool, error) {
	if _, err := root.Lstat(relative); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := root.RemoveAll(relative); err != nil {
		return false, err
	}
	return true, nil
}

func readSessionMeta(root *confinedfs.Root, metaPath string) (protocol.Session, error) {
	payload, err := root.ReadFile(metaPath)
	if err != nil {
		return protocol.Session{}, err
	}
	var item protocol.Session
	if err = json.Unmarshal(payload, &item); err != nil {
		return protocol.Session{}, err
	}
	if item.Options == nil {
		item.Options = map[string]any{}
	}
	if item.Title == "" {
		item.Title = "New Chat"
	}
	if item.ChannelType == "" {
		item.ChannelType = "websocket"
	}
	if item.ChatType == "" {
		item.ChatType = "dm"
	}
	item.IsActive = item.Status == "" || item.Status == "active"
	if item.Status == "" {
		item.Status = "active"
	}
	if item.LastActivity.IsZero() {
		item.LastActivity = item.CreatedAt
	}
	if item.ConfigurationVersion < 1 {
		item.ConfigurationVersion = 1
	}
	return item, nil
}
