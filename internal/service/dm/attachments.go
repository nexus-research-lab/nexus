// INPUT: DM 附件、workspace 与 owner-scoped 固定/动态 Slash 原始文本。
// OUTPUT: 附件归一化及只在 runtime 投递边界展开的消息内容。
// POS: DM 用户时间线原文到 runtime 可读消息的附件与产品提示适配层。
package dm

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
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
	agentValue, err := s.Agents.GetAgent(ctx, strings.TrimSpace(attachment.WorkspaceAgentID))
	if err != nil {
		return conversationsvc.ResolvedAttachment{}, err
	}
	return conversationsvc.OpenAgentWorkspaceAttachment(ctx, s.Config.WorkspacePath, *agentValue, attachment.WorkspacePath)
}
