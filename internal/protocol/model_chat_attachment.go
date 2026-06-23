package protocol

import "strings"

// ChatAttachmentKind 表示聊天附件的粗粒度类型。
type ChatAttachmentKind string
type ChatAttachmentScope string

const (
	ChatAttachmentKindText  ChatAttachmentKind = "text"
	ChatAttachmentKindImage ChatAttachmentKind = "image"
	ChatAttachmentKindFile  ChatAttachmentKind = "file"

	ChatAttachmentScopeAgentWorkspace   ChatAttachmentScope = "agent_workspace"
	ChatAttachmentScopeRoomConversation ChatAttachmentScope = "room_conversation"
)

// ChatAttachment 表示一次用户输入绑定的应用层附件。
type ChatAttachment struct {
	FileName         string              `json:"file_name"`
	WorkspacePath    string              `json:"workspace_path"`
	WorkspaceAgentID string              `json:"workspace_agent_id,omitempty"`
	RoomID           string              `json:"room_id,omitempty"`
	ConversationID   string              `json:"conversation_id,omitempty"`
	Scope            ChatAttachmentScope `json:"scope,omitempty"`
	Kind             ChatAttachmentKind  `json:"kind"`
	MIMEType         string              `json:"mime_type,omitempty"`
	Size             int64               `json:"size,omitempty"`
}

// NormalizeChatAttachmentKind 归一化附件类型。
func NormalizeChatAttachmentKind(value string) ChatAttachmentKind {
	switch ChatAttachmentKind(strings.ToLower(strings.TrimSpace(value))) {
	case ChatAttachmentKindImage:
		return ChatAttachmentKindImage
	case ChatAttachmentKindFile:
		return ChatAttachmentKindFile
	default:
		return ChatAttachmentKindText
	}
}

// NormalizeChatAttachmentScope 归一化附件存储作用域。
func NormalizeChatAttachmentScope(value string, conversationID string) ChatAttachmentScope {
	switch ChatAttachmentScope(strings.ToLower(strings.TrimSpace(value))) {
	case ChatAttachmentScopeRoomConversation:
		return ChatAttachmentScopeRoomConversation
	case ChatAttachmentScopeAgentWorkspace:
		return ChatAttachmentScopeAgentWorkspace
	default:
		if strings.TrimSpace(conversationID) != "" {
			return ChatAttachmentScopeRoomConversation
		}
		return ChatAttachmentScopeAgentWorkspace
	}
}

// NormalizeChatAttachment 归一化单个聊天附件。
func NormalizeChatAttachment(value ChatAttachment, defaultAgentID string) ChatAttachment {
	value.FileName = strings.TrimSpace(value.FileName)
	value.WorkspacePath = strings.TrimSpace(strings.ReplaceAll(value.WorkspacePath, "\\", "/"))
	value.WorkspaceAgentID = strings.TrimSpace(firstNonEmptyChatAttachment(value.WorkspaceAgentID, defaultAgentID))
	value.RoomID = strings.TrimSpace(value.RoomID)
	value.ConversationID = strings.TrimSpace(value.ConversationID)
	value.Scope = NormalizeChatAttachmentScope(string(value.Scope), value.ConversationID)
	if value.Scope == ChatAttachmentScopeRoomConversation {
		value.WorkspaceAgentID = ""
	}
	value.Kind = NormalizeChatAttachmentKind(string(value.Kind))
	value.MIMEType = strings.TrimSpace(value.MIMEType)
	if value.Size < 0 {
		value.Size = 0
	}
	return value
}

func firstNonEmptyChatAttachment(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// NormalizeChatAttachments 归一化并过滤无效附件。
func NormalizeChatAttachments(values []ChatAttachment, defaultAgentID string) []ChatAttachment {
	if len(values) == 0 {
		return nil
	}
	result := make([]ChatAttachment, 0, len(values))
	for _, value := range values {
		normalized := NormalizeChatAttachment(value, defaultAgentID)
		if normalized.WorkspacePath == "" {
			continue
		}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// HasChatInput 判断一次用户输入是否包含正文或附件。
func HasChatInput(content string, attachments []ChatAttachment) bool {
	return strings.TrimSpace(content) != "" || len(NormalizeChatAttachments(attachments, "")) > 0
}

// ChatAttachmentsFromAny 从弱类型 payload 中解析附件列表。
func ChatAttachmentsFromAny(value any) []ChatAttachment {
	switch typed := value.(type) {
	case []ChatAttachment:
		return NormalizeChatAttachments(typed, "")
	case []map[string]any:
		items := make([]ChatAttachment, 0, len(typed))
		for _, item := range typed {
			items = append(items, ChatAttachmentFromMap(item))
		}
		return NormalizeChatAttachments(items, "")
	case []any:
		items := make([]ChatAttachment, 0, len(typed))
		for _, item := range typed {
			if payload, ok := item.(map[string]any); ok {
				items = append(items, ChatAttachmentFromMap(payload))
			}
		}
		return NormalizeChatAttachments(items, "")
	default:
		return nil
	}
}

// ChatAttachmentFromMap 从 map payload 中解析单个附件。
func ChatAttachmentFromMap(value map[string]any) ChatAttachment {
	if value == nil {
		return ChatAttachment{}
	}
	return ChatAttachment{
		FileName:         chatAttachmentStringFromAny(value["file_name"]),
		WorkspacePath:    chatAttachmentStringFromAny(value["workspace_path"]),
		WorkspaceAgentID: chatAttachmentStringFromAny(value["workspace_agent_id"]),
		RoomID:           chatAttachmentStringFromAny(value["room_id"]),
		ConversationID:   chatAttachmentStringFromAny(value["conversation_id"]),
		Scope:            ChatAttachmentScope(chatAttachmentStringFromAny(value["scope"])),
		Kind:             ChatAttachmentKind(chatAttachmentStringFromAny(value["kind"])),
		MIMEType:         chatAttachmentStringFromAny(value["mime_type"]),
		Size:             chatAttachmentInt64FromAny(value["size"]),
	}
}

func chatAttachmentStringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return ""
	}
}

func chatAttachmentInt64FromAny(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
