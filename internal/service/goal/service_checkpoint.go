package goal

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CreateCheckpointByModel 允许模型保存当前 Goal 的长程执行摘要。
func (s *Service) CreateCheckpointByModel(ctx context.Context, goalID string, request protocol.CreateGoalCheckpointRequest) (*protocol.GoalCheckpoint, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	summary := strings.TrimSpace(request.Summary)
	if summary == "" {
		return nil, ErrGoalInvalidInput
	}
	now := s.nowFn()
	checkpoint := protocol.GoalCheckpoint{
		ID:                s.idFactory("goal_checkpoint"),
		GoalID:            item.ID,
		SessionKey:        item.SessionKey,
		Summary:           summary,
		ContinuationCount: item.ContinuationCount,
		Usage:             item.Usage,
		CreatedAt:         now,
	}
	created, err := s.repo.CreateCheckpoint(ctx, checkpoint)
	if err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, *item, "checkpoint_created", protocol.GoalUpdateSourceModel, request.RoundID, map[string]any{
		"checkpoint_id": created.ID,
		"summary":       created.Summary,
	}); err != nil {
		return nil, err
	}
	return created, nil
}
