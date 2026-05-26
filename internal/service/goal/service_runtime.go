package goal

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type externalMutationAccountant interface {
	FlushGoalAccounting(context.Context, string) ([]string, error)
	ClearGoalAccounting(string) []string
	ActivateGoalAccounting(context.Context, string) ([]string, error)
}

// SetExternalMutationAccountant 注入运行时 accounting flush，用于外部 Goal 状态变化前结算进度。
func (s *Service) SetExternalMutationAccountant(accountant externalMutationAccountant) {
	s.externalMutation = accountant
}

func (s *Service) prepareExternalMutation(ctx context.Context, goalID string) {
	goalID = strings.TrimSpace(goalID)
	if s.repo == nil || goalID == "" {
		return
	}
	if err := s.ensureEnabled(); err != nil {
		return
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil || item == nil {
		return
	}
	flushed := []string(nil)
	if s.externalMutation != nil {
		flushed, _ = s.externalMutation.FlushGoalAccounting(ctx, item.SessionKey)
	}
	if len(flushed) == 0 {
		_, _ = s.accountActiveWallClockUsage(ctx, *item)
	}
}

func (s *Service) clearExternalGoalAccounting(item protocol.Goal) {
	if !shouldClearRuntimeAccounting(item.Status) {
		return
	}
	s.clearWallClockGoal(item)
	if s.externalMutation == nil {
		return
	}
	_ = s.externalMutation.ClearGoalAccounting(item.SessionKey)
}

func (s *Service) clearDeletedGoalRuntimeAccounting(item protocol.Goal) {
	s.clearWallClockGoal(item)
	if s.externalMutation == nil {
		return
	}
	_ = s.externalMutation.ClearGoalAccounting(item.SessionKey)
}

func (s *Service) activateExternalGoalAccounting(ctx context.Context, item protocol.Goal) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return
	}
	s.markWallClockGoalActive(item)
	if s.externalMutation == nil {
		return
	}
	_, _ = s.externalMutation.ActivateGoalAccounting(ctx, item.SessionKey)
}

func shouldClearRuntimeAccounting(status protocol.GoalStatus) bool {
	switch protocol.NormalizeGoalStatus(status) {
	case protocol.GoalStatusPaused,
		protocol.GoalStatusComplete,
		protocol.GoalStatusBlocked,
		protocol.GoalStatusUsageLimited,
		protocol.GoalStatusCleared:
		return true
	default:
		return false
	}
}

func shouldClearAccountingAfterMutation(source protocol.GoalUpdateSource) bool {
	return source != protocol.GoalUpdateSourceModel
}
