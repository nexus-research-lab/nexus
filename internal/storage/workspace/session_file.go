package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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
		return &item, workspacePath, nil
	}
	return nil, "", nil
}

// UpsertSession 创建或更新 session meta。
func (s *SessionFileStore) UpsertSession(workspacePath string, item protocol.Session) (*protocol.Session, error) {
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
	return item, nil
}
