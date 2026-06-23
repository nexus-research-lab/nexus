package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

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
	sessionRoot := filepath.Join(workspacePath, ".agents", "sessions")
	entries, err := os.ReadDir(sessionRoot)
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
		metaPath := filepath.Join(sessionRoot, entry.Name(), "meta.json")
		item, loadErr := s.readSessionMeta(metaPath)
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
		metaPath := s.paths.SessionMetaPath(workspacePath, sessionKey)
		item, err := s.readSessionMeta(metaPath)
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
	metaPath := s.paths.SessionMetaPath(workspacePath, item.SessionKey)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return nil, err
	}

	// 这里直接以 Go 模型作为 meta 真相源，避免再复制一套弱类型结构。
	payload, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return nil, err
	}
	tempFile, err := os.CreateTemp(filepath.Dir(metaPath), ".meta-*.tmp")
	if err != nil {
		return nil, err
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if _, err = tempFile.Write(payload); err != nil {
		_ = tempFile.Close()
		return nil, err
	}
	if err = tempFile.Chmod(0o644); err != nil {
		_ = tempFile.Close()
		return nil, err
	}
	if err = tempFile.Close(); err != nil {
		return nil, err
	}
	// 先写临时文件再 rename，避免并发 meta 刷新时读到半截 JSON。
	if err = os.Rename(tempPath, metaPath); err != nil {
		return nil, err
	}
	created, _, err := s.FindSession([]string{workspacePath}, item.SessionKey)
	return created, err
}

// DeleteSession 删除整个 session 目录。
func (s *SessionFileStore) DeleteSession(workspacePath string, sessionKey string) (bool, error) {
	sessionDir := s.paths.SessionDir(workspacePath, sessionKey)
	if _, err := os.Stat(sessionDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := os.RemoveAll(sessionDir); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteRoomConversation 删除 Room 对话共享目录。
func (s *SessionFileStore) DeleteRoomConversation(conversationID string) (bool, error) {
	conversationDir := s.paths.RoomConversationDir(conversationID)
	if _, err := os.Stat(conversationDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := os.RemoveAll(conversationDir); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SessionFileStore) readSessionMeta(metaPath string) (protocol.Session, error) {
	payload, err := os.ReadFile(metaPath)
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
