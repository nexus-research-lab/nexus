// =====================================================
// @File   ：store.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package workspace

import (
	"encoding/base64"
	"hash/fnv"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Store 负责生成 workspace 侧存储路径。
type Store struct {
	WorkspaceRoot string
	HomeRoot      string
}

// New 创建 workspace store。
func New(root string) *Store {
	workspaceRoot := strings.TrimSpace(root)
	homeRoot := transcriptConfigHomeDir()
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(homeRoot, "workspace")
	}
	return &Store{
		WorkspaceRoot: workspaceRoot,
		HomeRoot:      homeRoot,
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

// LegacySessionDir 返回最旧版 base64 session 目录。
func (s *Store) LegacySessionDir(workspacePath string, sessionKey string) string {
	return filepath.Join(workspacePath, ".agents", "sessions", legacyEncodeSessionKey(sessionKey))
}

// CompactSessionDir 返回上一版短名 + hash session 目录。
func (s *Store) CompactSessionDir(workspacePath string, sessionKey string) string {
	return filepath.Join(workspacePath, ".agents", "sessions", legacyCompactSessionDirName(sessionKey))
}

// SessionMetaPath 返回 meta.json 路径。
func (s *Store) SessionMetaPath(workspacePath string, sessionKey string) string {
	return filepath.Join(s.SessionDir(workspacePath, sessionKey), "meta.json")
}

// SessionOverlayPath 返回 overlay.jsonl 路径。
func (s *Store) SessionOverlayPath(workspacePath string, sessionKey string) string {
	return filepath.Join(s.SessionDir(workspacePath, sessionKey), "overlay.jsonl")
}

// RoomConversationDir 返回 Room 对话目录。
func (s *Store) RoomConversationDir(conversationID string) string {
	return filepath.Join(s.HomeRoot, "rooms", encodeConversationDirName(conversationID))
}

// RoomConversationRoot 返回 Room 共享目录根。
func (s *Store) RoomConversationRoot() string {
	return filepath.Join(s.HomeRoot, "rooms")
}

// LegacyRoomConversationDir 返回最旧版 base64 Room 对话目录。
func (s *Store) LegacyRoomConversationDir(conversationID string) string {
	return filepath.Join(s.HomeRoot, "rooms", legacyEncodeSessionKey(conversationID))
}

// CompactRoomConversationDir 返回上一版短名 + hash Room 对话目录。
func (s *Store) CompactRoomConversationDir(conversationID string) string {
	return filepath.Join(s.HomeRoot, "rooms", legacyCompactConversationDirName(conversationID))
}

// RoomConversationOverlayPath 返回 Room 对话共享 overlay 路径。
func (s *Store) RoomConversationOverlayPath(conversationID string) string {
	return filepath.Join(s.RoomConversationDir(conversationID), "overlay.jsonl")
}

func legacyEncodeSessionKey(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func encodeSessionDirName(value string) string {
	parsed := protocol.ParseSessionKey(value)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		return joinSessionPathSegments("room", escapePathAtom(parsed.ConversationID))
	case protocol.SessionKeyKindAgent:
		switch strings.TrimSpace(parsed.ChatType) {
		case "dm":
			parts := []string{"dm"}
			if channel := escapePathAtom(parsed.Channel); channel != "" {
				parts = append(parts, channel)
			}
			if ref := escapePathAtom(parsed.Ref); ref != "" {
				parts = append(parts, ref)
			}
			if threadID := escapePathAtom(parsed.ThreadID); threadID != "" {
				parts = append(parts, "topic", threadID)
			}
			return joinSessionPathSegments(parts...)
		case "group":
			parts := []string{"room"}
			if channel := strings.TrimSpace(parsed.Channel); channel != "" && channel != protocol.SessionChannelWebSocketSegment {
				parts = append(parts, escapePathAtom(channel))
			}
			if ref := escapePathAtom(parsed.Ref); ref != "" {
				parts = append(parts, ref)
			}
			if threadID := escapePathAtom(parsed.ThreadID); threadID != "" {
				parts = append(parts, "topic", threadID)
			}
			return joinSessionPathSegments(parts...)
		default:
			return joinSessionPathSegments(
				"session",
				escapePathAtom(parsed.Channel),
				escapePathAtom(parsed.Ref),
				escapePathAtom(parsed.ThreadID),
			)
		}
	default:
		return joinSessionPathSegments("session", escapePathAtom(value))
	}
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

func shortPathSlug(value string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(lower))
	previousHyphen := false
	for _, character := range lower {
		isLetter := character >= 'a' && character <= 'z'
		isDigit := character >= '0' && character <= '9'
		if isLetter || isDigit {
			builder.WriteRune(character)
			previousHyphen = false
			continue
		}
		if previousHyphen || builder.Len() == 0 {
			continue
		}
		builder.WriteByte('-')
		previousHyphen = true
	}

	slug := strings.Trim(builder.String(), "-")
	if len(slug) <= maxLength {
		return slug
	}
	return strings.Trim(slug[:maxLength], "-")
}

func shortPathHash(value string) string {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(value))
	return strconv.FormatUint(uint64(hasher.Sum32()), 36)
}

func legacyCompactSessionDirName(value string) string {
	parsed := protocol.ParseSessionKey(value)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		return joinSessionPathSegments(
			"room",
			shortPathSlug(parsed.ConversationID, 12),
			shortPathHash(value),
		)
	case protocol.SessionKeyKindAgent:
		prefix := "session"
		switch strings.TrimSpace(parsed.ChatType) {
		case "dm":
			prefix = "dm"
		case "group":
			prefix = "room"
		}
		return joinSessionPathSegments(
			prefix,
			shortPathSlug(parsed.Ref, 16),
			"a",
			shortPathSlug(parsed.AgentID, 8),
			"t",
			shortPathSlug(parsed.ThreadID, 8),
			shortPathHash(value),
		)
	default:
		return joinSessionPathSegments(
			"session",
			shortPathSlug(value, 24),
			shortPathHash(value),
		)
	}
}

func legacyCompactConversationDirName(conversationID string) string {
	return joinSessionPathSegments(
		"room",
		shortPathSlug(conversationID, 12),
		shortPathHash(conversationID),
	)
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
