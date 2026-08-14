// INPUT: 外部 Goal mutation、活跃 runtime accounting hook 与 wall-clock checkpoint。
// OUTPUT: mutation 前可失败的 usage flush，以及状态切换后的 clear/activate/interrupt。
// POS: Goal 状态机与运行中 parent accounting 的顺序边界。
package goal

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type externalMutationAccountant interface {
	FlushGoalAccounting(context.Context, string) ([]string, error)
	ClearGoalAccounting(string) []string
	ActivateGoalAccounting(context.Context, string, string) ([]string, error)
}

type externalMutationFinalizer interface {
	BeginGoalAccountingFinalizing(string) []string
}

type externalGoalCreatePreflighter interface {
	GoalAccountingCreateConflicts(sessionKey string, scopeRoundID string) []string
}

type externalGoalActivationRollback interface {
	ClearGoalAccountingRounds(sessionKey string, roundIDs []string) []string
}

type goalAccountingRoundReader interface {
	GoalAccountingRoundIDs(sessionKey string, goalID string) []string
}

type runtimeInterrupter interface {
	InterruptGoalRuntime(context.Context, string, []string) error
}

type runtimeUsageSettlementBoundaryKey struct{}

// SetExternalMutationAccountant 注入运行时 accounting flush，用于外部 Goal 状态变化前结算进度。
func (s *Service) SetExternalMutationAccountant(accountant externalMutationAccountant) {
	s.externalMutation = accountant
}

// SetRuntimeInterrupter 注入用户暂停 Goal 时的运行中输出中断器。
func (s *Service) SetRuntimeInterrupter(interrupter runtimeInterrupter) {
	s.runtimeInterrupt = interrupter
}

func (s *Service) prepareExternalMutation(ctx context.Context, goalID string) error {
	goalID = strings.TrimSpace(goalID)
	if s.repo == nil || goalID == "" {
		return nil
	}
	if err := s.ensureEnabled(); err != nil {
		return err
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil || item == nil {
		return err
	}
	flushed := []string(nil)
	if s.externalMutation != nil {
		flushed, err = s.externalMutation.FlushGoalAccounting(ctx, item.SessionKey)
		if err != nil {
			return err
		}
	}
	if len(flushed) == 0 {
		if _, err = s.accountActiveWallClockUsage(ctx, *item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) prepareExternalMutationAtSettlementBoundary(
	ctx context.Context,
	goalID string,
) error {
	return s.prepareExternalMutation(
		context.WithValue(ctx, runtimeUsageSettlementBoundaryKey{}, true),
		goalID,
	)
}

// RuntimeUsageSettlementBoundary 判断当前 flush 后是否会停止或换绑 Goal。
// runtime consumer 仅在该边界结算 deferred actual；普通 mid-round checkpoint
// 仍等待 provider terminal 做最终校准。
func RuntimeUsageSettlementBoundary(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(runtimeUsageSettlementBoundaryKey{}).(bool)
	return value
}

func (s *Service) preflightGoalCreate(sessionKey string, scopeRoundID string) error {
	preflighter, ok := s.externalMutation.(externalGoalCreatePreflighter)
	if !ok {
		return nil
	}
	conflicts := preflighter.GoalAccountingCreateConflicts(
		strings.TrimSpace(sessionKey),
		strings.TrimSpace(scopeRoundID),
	)
	if len(conflicts) == 0 {
		return nil
	}
	conflicts = slices.Sorted(slices.Values(conflicts))
	return fmt.Errorf(
		"%w: live runtime scope already consumed a Goal in round(s) %s",
		ErrGoalConflict,
		strings.Join(conflicts, ", "),
	)
}

func (s *Service) clearExternalGoalAccounting(
	ctx context.Context,
	item protocol.Goal,
) error {
	if !shouldClearRuntimeAccounting(item.Status) {
		return nil
	}
	s.clearWallClockGoal(item)
	switch protocol.NormalizeGoalStatus(item.Status) {
	case protocol.GoalStatusComplete, protocol.GoalStatusPaused:
		if finalizer, ok := s.externalMutation.(externalMutationFinalizer); ok {
			if rounds := finalizer.BeginGoalAccountingFinalizing(item.SessionKey); len(rounds) > 0 {
				return nil
			}
		}
		if protocol.NormalizeGoalStatus(item.Status) == protocol.GoalStatusComplete {
			_, err := s.FinalizeUsageForGoal(ctx, item.ID, protocol.GoalUsage{}, "")
			return err
		}
	}
	if s.externalMutation != nil {
		_ = s.externalMutation.ClearGoalAccounting(item.SessionKey)
	}
	return nil
}

func (s *Service) clearDeletedGoalRuntimeAccounting(item protocol.Goal) {
	s.clearWallClockGoal(item)
	if s.externalMutation == nil {
		return
	}
	_ = s.externalMutation.ClearGoalAccounting(item.SessionKey)
}

func (s *Service) activateExternalGoalAccounting(ctx context.Context, item protocol.Goal) error {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return nil
	}
	if s.externalMutation != nil {
		activated, err := s.externalMutation.ActivateGoalAccounting(ctx, item.SessionKey, item.ID)
		if err != nil {
			if rollback, ok := s.externalMutation.(externalGoalActivationRollback); ok {
				_ = rollback.ClearGoalAccountingRounds(item.SessionKey, activated)
			}
			return err
		}
	}
	s.markWallClockGoalActive(item)
	return nil
}

func (s *Service) rollbackFailedGoalCreate(
	ctx context.Context,
	item protocol.Goal,
	activationErr error,
) error {
	if activationErr == nil {
		return nil
	}
	deleted, err := s.repo.DeleteGoal(context.WithoutCancel(ctx), item.ID)
	if err != nil {
		return errors.Join(
			activationErr,
			fmt.Errorf("rollback Goal %q after runtime activation failed: %w", item.ID, err),
		)
	}
	if !deleted {
		return errors.Join(
			activationErr,
			fmt.Errorf("rollback Goal %q after runtime activation failed: %w", item.ID, ErrGoalInvalidState),
		)
	}
	return activationErr
}

func (s *Service) interruptGoalRuntimeAfterPause(ctx context.Context, item protocol.Goal) {
	if s.runtimeInterrupt == nil {
		return
	}
	roundIDs := []string(nil)
	if reader, ok := s.externalMutation.(goalAccountingRoundReader); ok {
		roundIDs = reader.GoalAccountingRoundIDs(item.SessionKey, item.ID)
	}
	_ = s.runtimeInterrupt.InterruptGoalRuntime(ctx, item.SessionKey, roundIDs)
}

func shouldClearRuntimeAccounting(status protocol.GoalStatus) bool {
	switch protocol.NormalizeGoalStatus(status) {
	case protocol.GoalStatusPaused,
		protocol.GoalStatusComplete,
		protocol.GoalStatusBlocked,
		protocol.GoalStatusUsageLimited:
		return true
	default:
		return false
	}
}
