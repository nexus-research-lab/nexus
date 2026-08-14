package workspace

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Store 负责生成 workspace 侧存储路径。
type Store struct {
	WorkspaceRoot string
	StateRoot     string
	HomeRoot      string
}

// New 创建 workspace store。
func New(root string) *Store {
	workspaceRoot := strings.TrimSpace(root)
	if workspaceRoot == "" {
		workspaceRoot = appfs.UsersRoot()
	}
	return &Store{
		WorkspaceRoot: workspaceRoot,
		StateRoot:     appfs.StateRoot(),
		HomeRoot:      appfs.AppDir(),
	}
}

// SessionDir 返回 session 目录。
func (s *Store) SessionDir(workspacePath string, sessionKey string) string {
	return filepath.Join(workspacePath, ".agents", "sessions", encodeSessionDirName(sessionKey))
}

// SessionRoot 返回某个 workspace 下的 session 根目录。
func (s *Store) SessionRoot(workspacePath string) string {
	return filepath.Join(workspacePath, ".agents", "sessions")
}

// SessionMetaPath 返回 meta.json 路径。
func (s *Store) SessionMetaPath(workspacePath string, sessionKey string) string {
	return filepath.Join(s.SessionDir(workspacePath, sessionKey), "meta.json")
}

// SessionOverlayPath 返回 overlay.jsonl 路径。
func (s *Store) SessionOverlayPath(workspacePath string, sessionKey string) string {
	return filepath.Join(s.SessionDir(workspacePath, sessionKey), "overlay.jsonl")
}

// SessionInputQueuePath 返回 DM/agent 会话待发送队列路径。
func (s *Store) SessionInputQueuePath(workspacePath string, sessionKey string) string {
	return filepath.Join(s.SessionDir(workspacePath, sessionKey), "input_queue.jsonl")
}

// RoomConversationDir 返回指定用户的 Room 对话目录。
func (s *Store) RoomConversationDir(ownerUserID string, conversationID string) string {
	return filepath.Join(s.RoomConversationRoot(ownerUserID), encodeConversationDirName(conversationID))
}

// RoomConversationRoot 返回指定用户的 Room 状态根。
func (s *Store) RoomConversationRoot(ownerUserID string) string {
	return appfs.UserRoomRootAt(s.StateRoot, ownerUserID)
}

// RoomConversationAssetDir 返回 runtime 可读取的 Room 对话公共资产目录。
func (s *Store) RoomConversationAssetDir(ownerUserID string, conversationID string) string {
	return filepath.Join(
		appfs.UserRoomAssetsRootAt(s.StateRoot, ownerUserID),
		encodeConversationDirName(conversationID),
	)
}

// RoomConversationOverlayPath 返回指定用户 Room 对话的共享 overlay 路径。
func (s *Store) RoomConversationOverlayPath(ownerUserID string, conversationID string) string {
	return filepath.Join(s.RoomConversationDir(ownerUserID, conversationID), "overlay.jsonl")
}

// RoomConversationMessagesPath 返回指定用户 Room 对话的 directed message 日志路径。
func (s *Store) RoomConversationMessagesPath(ownerUserID string, conversationID string) string {
	return filepath.Join(s.RoomConversationDir(ownerUserID, conversationID), "directed_messages.jsonl")
}

// RoomConversationMessageCursorsPath 返回指定用户 Room directed message 消费游标路径。
func (s *Store) RoomConversationMessageCursorsPath(ownerUserID string, conversationID string) string {
	return filepath.Join(s.RoomConversationDir(ownerUserID, conversationID), "directed_message_cursors.jsonl")
}

// RoomPublicHandoffsPath 返回指定用户 Room handoff ledger 路径。
func (s *Store) RoomPublicHandoffsPath(ownerUserID string, conversationID string) string {
	return filepath.Join(s.RoomConversationDir(ownerUserID, conversationID), "public_handoffs.jsonl")
}

// RoomDirectedMessageWakesPath 返回指定用户的 Room immediate/delayed 唤醒日志路径。
func (s *Store) RoomDirectedMessageWakesPath(ownerUserID string) string {
	return filepath.Join(s.RoomConversationRoot(ownerUserID), "directed_message_wakes.jsonl")
}

func encodeSessionDirName(value string) string {
	return protocol.LegacySessionDirectoryIdentity(value)
}

func encodeConversationDirName(conversationID string) string {
	return joinSessionPathSegments("room", escapePathAtom(conversationID))
}

func joinSessionPathSegments(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "-")
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "-")
}

func escapePathAtom(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	for _, character := range value {
		isLetter := character >= 'a' && character <= 'z'
		isUpper := character >= 'A' && character <= 'Z'
		isDigit := character >= '0' && character <= '9'
		switch {
		case isLetter || isUpper || isDigit:
			builder.WriteRune(character)
		case character == '-' || character == '_' || character == '.':
			builder.WriteRune(character)
		default:
			builder.WriteString("_")
			builder.WriteString(strconv.FormatInt(int64(character), 16))
		}
	}
	return strings.Trim(builder.String(), "-")
}
