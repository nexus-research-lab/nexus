package goal

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type previewFiller interface {
	FillEmptyPreviewFromGoal(context.Context, string, string) error
	ScheduleGoalTitleFromGoal(context.Context, protocol.Goal, string, string)
}

// SetPreviewFiller 注入会话预览填充器，用于对齐 Codex create_goal 的空 thread preview 语义。
func (s *Service) SetPreviewFiller(filler previewFiller) {
	if s == nil {
		return
	}
	s.preview = filler
}

func (s *Service) updatePreviewFromGoal(ctx context.Context, item protocol.Goal, ownerUserID string) {
	if s == nil || s.preview == nil {
		return
	}
	sessionKey := strings.TrimSpace(item.SessionKey)
	fallbackTitle := goalPreviewTitle(item)
	if sessionKey == "" || fallbackTitle == "" {
		return
	}
	_ = s.preview.FillEmptyPreviewFromGoal(ctx, sessionKey, fallbackTitle)
	s.preview.ScheduleGoalTitleFromGoal(ctx, item, ownerUserID, fallbackTitle)
}

func goalPreviewTitle(item protocol.Goal) string {
	if title := protocol.GoalMetadataString(item.Metadata, protocol.GoalMetadataRoomGoalLoopTitle); title != "" {
		return title
	}
	return strings.TrimSpace(item.Objective)
}
