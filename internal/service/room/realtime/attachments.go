// INPUT: Room 聊天附件、conversation/owner 身份、Agent workspace 与固定/动态 Slash 原文。
// OUTPUT: 归一化附件和只在 runtime 投递边界展开的消息内容。
// POS: Room 公共附件进入实时 runtime 的路径解析边界。
package realtime

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) normalizeChatAttachments(
	attachments []protocol.ChatAttachment,
	defaultAgentID string,
	defaultRoomID string,
	defaultConversationID string,
) []protocol.ChatAttachment {
	normalized := protocol.NormalizeChatAttachments(attachments, defaultAgentID)
	for index := range normalized {
		if normalized[index].Scope != protocol.ChatAttachmentScopeRoomConversation {
			continue
		}
		if normalized[index].RoomID == "" {
			normalized[index].RoomID = strings.TrimSpace(defaultRoomID)
		}
		if normalized[index].ConversationID == "" {
			normalized[index].ConversationID = strings.TrimSpace(defaultConversationID)
		}
		normalized[index].WorkspaceAgentID = ""
	}
	return normalized
}

func (s *Service) renderRuntimeContentWithAttachments(
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

func (s *Service) expandRuntimeSlashPrompt(
	ctx context.Context,
	content string,
) (string, error) {
	if s.runtimeSlashExpander != nil {
		return s.runtimeSlashExpander.ExpandRuntimePrompt(
			ctx,
			authctx.OwnerUserID(ctx),
			content,
		)
	}
	return slashcommandsvc.ExpandProductPrompt(content), nil
}

func (s *Service) appendRuntimeUserContext(
	ctx context.Context,
	conversationID string,
	agentValue *protocol.Agent,
	runtimeContent conversationsvc.RuntimeContent,
	emotionEnabled bool,
) conversationsvc.RuntimeContent {
	if agentValue == nil || runtimeContent.IsEmpty() || s.agents == nil || !emotionEnabled {
		return runtimeContent
	}
	return runtimeContent.AppendText(s.agents.BuildRuntimeUserMessageSuffixForContext(
		ctx,
		agentValue,
		"room:"+strings.TrimSpace(conversationID),
		emotionEnabled,
	))
}

func (s *Service) resolveRuntimeAttachmentPath(
	ctx context.Context,
	attachment protocol.ChatAttachment,
) (conversationsvc.ResolvedAttachment, error) {
	pathStore := workspacestore.New(s.config.WorkspacePath)
	ownerUserID := authctx.OwnerUserID(ctx)
	if attachment.Scope == protocol.ChatAttachmentScopeRoomConversation {
		conversationID := strings.TrimSpace(attachment.ConversationID)
		if conversationID == "" {
			return conversationsvc.ResolvedAttachment{}, errors.New("room attachment conversation_id is required")
		}
		absolutePath, file, err := pathStore.OpenRoomConversationAssetFile(
			ownerUserID,
			conversationID,
			attachment.WorkspacePath,
		)
		if err != nil {
			return conversationsvc.ResolvedAttachment{}, err
		}
		return conversationsvc.ResolvedAttachment{
			AbsolutePath: absolutePath,
			File:         file,
		}, nil
	}

	agentID := strings.TrimSpace(attachment.WorkspaceAgentID)
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return conversationsvc.ResolvedAttachment{}, err
	}
	ownerUserID = authctx.OwnerUserID(ctx)
	if agentOwner := strings.TrimSpace(agentValue.OwnerUserID); agentOwner != "" {
		if currentUserID, ok := authctx.CurrentUserID(ctx); ok &&
			strings.TrimSpace(currentUserID) != agentOwner {
			return conversationsvc.ResolvedAttachment{}, errors.New("附件 agent 不属于当前用户")
		}
		ownerUserID = agentOwner
	}
	absolutePath, file, err := pathStore.OpenOwnerWorkspaceFile(
		ownerUserID,
		agentValue.WorkspacePath,
		attachment.WorkspacePath,
	)
	if err != nil {
		return conversationsvc.ResolvedAttachment{}, err
	}
	return conversationsvc.ResolvedAttachment{
		AbsolutePath: absolutePath,
		File:         file,
	}, nil
}

func (s *Service) renderRuntimeAttachmentMessages(
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
