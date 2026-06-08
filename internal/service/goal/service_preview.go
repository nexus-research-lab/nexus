package goal

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type previewFiller interface {
	FillEmptyPreviewFromGoal(context.Context, string, string) error
}

// SetPreviewFiller 注入会话预览填充器，用于对齐 Codex create_goal 的空 thread preview 语义。
func (s *Service) SetPreviewFiller(filler previewFiller) {
	if s == nil {
		return
	}
	s.preview = filler
}

func (s *Service) fillEmptyPreviewFromGoal(ctx context.Context, item protocol.Goal) {
	if s == nil || s.preview == nil {
		return
	}
	sessionKey := strings.TrimSpace(item.SessionKey)
	objective := strings.TrimSpace(item.Objective)
	if sessionKey == "" || objective == "" {
		return
	}
	_ = s.preview.FillEmptyPreviewFromGoal(ctx, sessionKey, objective)
}
