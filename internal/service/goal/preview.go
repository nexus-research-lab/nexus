package goal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type previewFiller interface {
	FillEmptyPreviewFromGoal(context.Context, string, string) error
	ScheduleGoalTitleFromGoal(context.Context, protocol.Goal, string, string)
}

type previewRecoveryFiller interface {
	RepairGoalTitleFromGoal(context.Context, protocol.Goal, string) error
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

// RepairCurrentGoalPreviews replays the idempotent title projection for Goals
// that survived a restart or whose original control response was lost.
func (s *Service) RepairCurrentGoalPreviews(ctx context.Context) error {
	if s == nil || !s.config.GoalEnabled || s.preview == nil {
		return nil
	}
	repairer, ok := s.preview.(previewRecoveryFiller)
	if !ok {
		return nil
	}
	items, err := s.repo.ListCurrentGoals(ctx)
	if err != nil {
		return err
	}
	var repairErrors []error
	for _, item := range items {
		ownerUserID := protocol.GoalMetadataString(item.Metadata, protocol.GoalMetadataOwnerUserID)
		if ownerUserID == "" {
			continue
		}
		if err := repairer.RepairGoalTitleFromGoal(ctx, item, ownerUserID); err != nil {
			repairErrors = append(repairErrors, fmt.Errorf("repair Goal %s preview: %w", item.ID, err))
		}
	}
	return errors.Join(repairErrors...)
}

func goalPreviewTitle(item protocol.Goal) string {
	if title := protocol.GoalMetadataString(item.Metadata, protocol.GoalMetadataRoomGoalLoopTitle); title != "" {
		return title
	}
	return strings.TrimSpace(item.Objective)
}
