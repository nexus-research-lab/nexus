package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

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
