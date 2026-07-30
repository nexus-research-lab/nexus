package dm

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) normalizeChatAttachments(
	attachments []protocol.ChatAttachment,
	defaultAgentID string,
) []protocol.ChatAttachment {
	return protocol.NormalizeChatAttachments(attachments, strings.TrimSpace(defaultAgentID))
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

func (s *Service) resolveRuntimeAttachmentPath(
	ctx context.Context,
	attachment protocol.ChatAttachment,
) (conversationsvc.ResolvedAttachment, error) {
	agentID := strings.TrimSpace(attachment.WorkspaceAgentID)
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return conversationsvc.ResolvedAttachment{}, err
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	if agentOwner := strings.TrimSpace(agentValue.OwnerUserID); agentOwner != "" {
		if currentUserID, ok := authctx.CurrentUserID(ctx); ok &&
			strings.TrimSpace(currentUserID) != agentOwner {
			return conversationsvc.ResolvedAttachment{}, errors.New("附件 agent 不属于当前用户")
		}
		ownerUserID = agentOwner
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return conversationsvc.ResolvedAttachment{}, errors.New("附件 agent 不属于当前用户")
	}
	absolutePath, file, err := workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspaceFile(
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
