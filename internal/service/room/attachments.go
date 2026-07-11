package room

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// UploadConversationAttachment 上传 Room conversation 级公共附件。
func (s *Service) UploadConversationAttachment(
	ctx context.Context,
	roomID string,
	conversationID string,
	filename string,
	destination string,
	reader io.Reader,
) (*workspacepkg.UploadResult, error) {
	contextValue, err := s.GetConversationContext(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(contextValue.Room.ID) != strings.TrimSpace(roomID) {
		return nil, ErrConversationNotFound
	}
	if contextValue.Room.RoomType == protocol.RoomTypeDM {
		return nil, errors.New("DM conversation does not support room attachments")
	}

	root := workspacestore.New(s.config.WorkspacePath).RoomConversationDir(conversationID)
	return workspacepkg.UploadFileToRoot(root, filename, destination, reader)
}

func (s *RealtimeService) normalizeChatAttachments(
	attachments []protocol.ChatAttachment,
	defaultAgentID string,
	defaultRoomID string,
	defaultConversationID string,
) []protocol.ChatAttachment {
	normalized := protocol.NormalizeChatAttachments(attachments, strings.TrimSpace(defaultAgentID))
	for index := range normalized {
		if normalized[index].Scope != protocol.ChatAttachmentScopeRoomConversation {
			continue
		}
		if strings.TrimSpace(normalized[index].RoomID) == "" {
			normalized[index].RoomID = strings.TrimSpace(defaultRoomID)
		}
		if strings.TrimSpace(normalized[index].ConversationID) == "" {
			normalized[index].ConversationID = strings.TrimSpace(defaultConversationID)
		}
		normalized[index].WorkspaceAgentID = ""
	}
	return normalized
}

func (s *RealtimeService) renderRuntimeContentWithAttachments(
	ctx context.Context,
	content string,
	attachments []protocol.ChatAttachment,
) (conversationsvc.RuntimeContent, error) {
	return conversationsvc.RenderRuntimeContentWithAttachments(
		ctx,
		content,
		attachments,
		s.resolveRuntimeAttachmentPath,
	)
}

func (s *RealtimeService) appendRuntimeUserContext(
	ctx context.Context,
	conversationID string,
	agentValue *protocol.Agent,
	runtimeContent conversationsvc.RuntimeContent,
) conversationsvc.RuntimeContent {
	if agentValue == nil || runtimeContent.IsEmpty() {
		return runtimeContent
	}
	if s.agents == nil {
		return runtimeContent
	}
	return runtimeContent.AppendText(s.agents.BuildRuntimeUserMessageSuffixForContext(ctx, agentValue, "room:"+strings.TrimSpace(conversationID)))
}

func (s *RealtimeService) resolveRuntimeAttachmentPath(
	ctx context.Context,
	attachment protocol.ChatAttachment,
) (string, error) {
	if attachment.Scope == protocol.ChatAttachmentScopeRoomConversation {
		conversationID := strings.TrimSpace(attachment.ConversationID)
		if conversationID == "" {
			return "", errors.New("room attachment conversation_id is required")
		}
		root := workspacestore.New(s.config.WorkspacePath).RoomConversationDir(conversationID)
		return conversationsvc.ResolveWorkspaceAttachmentPath(root, attachment.WorkspacePath)
	}

	agentID := strings.TrimSpace(attachment.WorkspaceAgentID)
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return "", err
	}
	return conversationsvc.ResolveWorkspaceAttachmentPath(agentValue.WorkspacePath, attachment.WorkspacePath)
}

func (s *RealtimeService) renderRuntimeAttachmentMessages(
	ctx context.Context,
	messages []protocol.Message,
) ([]protocol.Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}
	result := make([]protocol.Message, 0, len(messages))
	for _, message := range messages {
		attachments := protocol.ChatAttachmentsFromAny(message["attachments"])
		if len(attachments) == 0 {
			result = append(result, message)
			continue
		}
		content, _ := message["content"].(string)
		runtimeContent, err := s.renderRuntimeContentWithAttachments(ctx, content, attachments)
		if err != nil {
			return nil, err
		}
		next := protocol.Clone(message)
		next["content"] = runtimeContent.PlainText()
		result = append(result, next)
	}
	return result, nil
}
