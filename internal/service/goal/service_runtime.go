package goal

import (
	"context"
	"strings"
)

type externalMutationAccountant interface {
	FlushGoalAccounting(context.Context, string) ([]string, error)
}

// SetExternalMutationAccountant 注入运行时 accounting flush，用于外部 Goal 状态变化前结算进度。
func (s *Service) SetExternalMutationAccountant(accountant externalMutationAccountant) {
	s.externalMutation = accountant
}

func (s *Service) prepareExternalMutation(ctx context.Context, goalID string) {
	goalID = strings.TrimSpace(goalID)
	if s.externalMutation == nil || s.repo == nil || goalID == "" {
		return
	}
	if err := s.ensureEnabled(); err != nil {
		return
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil || item == nil {
		return
	}
	_, _ = s.externalMutation.FlushGoalAccounting(ctx, item.SessionKey)
}
