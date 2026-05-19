package dm

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
)

func (s *Service) normalizeChatAttachments(
	attachments []protocol.ChatAttachment,
	fallbackAgentID string,
) []protocol.ChatAttachment {
	return protocol.NormalizeChatAttachments(attachments, strings.TrimSpace(fallbackAgentID))
}

func (s *Service) renderRuntimeContentWithAttachments(
	ctx context.Context,
	content string,
	attachments []protocol.ChatAttachment,
) (string, error) {
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
) (string, error) {
	agentID := strings.TrimSpace(attachment.WorkspaceAgentID)
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return "", err
	}
	return conversationsvc.ResolveWorkspaceAttachmentPath(agentValue.WorkspacePath, attachment.WorkspacePath)
}
