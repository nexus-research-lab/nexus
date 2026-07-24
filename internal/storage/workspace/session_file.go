package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SessionFileStore 负责 workspace 侧会话文件读写。
type SessionFileStore struct {
	paths *Store
}

// NewSessionFileStore 创建文件存储门面。
func NewSessionFileStore(root string) *SessionFileStore {
	return &SessionFileStore{
		paths: New(root),
	}
}

// ListSessions 读取某个 workspace 下的全部文件会话。
func (s *SessionFileStore) ListSessions(workspacePath string) ([]protocol.Session, error) {
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []protocol.Session{}, nil
		}
		return nil, err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".agents/sessions")
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.Session{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make([]protocol.Session, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.ToSlash(filepath.Join(".agents", "sessions", entry.Name(), "meta.json"))
		item, loadErr := readSessionMeta(root, metaPath)
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
		root, openErr := confinedfs.Open(workspacePath)
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return nil, "", openErr
		}
		relative := filepath.ToSlash(filepath.Join(
			".agents",
			"sessions",
			encodeSessionDirName(sessionKey),
			"meta.json",
		))
		item, err := readSessionMeta(root, relative)
		root.Close()
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
	parent, relative, err := s.openStorePath(workspacePath, true)
	if err != nil {
		return nil, err
	}
	if err = parent.MkdirAll(relative, storageDirectoryMode()); err != nil {
		parent.Close()
		return nil, err
	}
	parent.Close()
	return confinedfs.Open(workspacePath)
}

// DeleteSession 删除整个 session 目录。
func (s *SessionFileStore) DeleteSession(workspacePath string, sessionKey string) (bool, error) {
	root, err := confinedfs.Open(workspacePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()
	sessionDir := filepath.ToSlash(filepath.Join(".agents", "sessions", encodeSessionDirName(sessionKey)))
	if _, err := root.Lstat(sessionDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := root.RemoveAll(sessionDir); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteRoomConversation 删除 Room 对话共享目录。
func (s *SessionFileStore) DeleteRoomConversation(conversationID string) (bool, error) {
	conversationDir := s.paths.RoomConversationDir(conversationID)
	root, relative, err := s.openStorePath(conversationDir, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()
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
