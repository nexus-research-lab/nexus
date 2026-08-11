// INPUT: exact Goal-bound Execution snapshot、durable pending confirmation receipt 与到期 scan limit。
// OUTPUT: Goal reverse-binding confirmation、confirmed receipt 或带重试期限的 pending receipt，以及跨进程恢复计数。
// POS: 所有 Execution -> Goal SQL mutation 共用的 confirmation saga；Plan proposal 只保留其自身 materialization receipt。
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

const goalConfirmationRetryDelay = 15 * time.Second

// GoalConfirmationRepository is kept separate from the aggregate Repository so
// focused service fakes do not need to implement the background recovery port.
type GoalConfirmationRepository interface {
	EnsureGoalConfirmationReceipt(
		context.Context,
		string,
	) (*orchestrationstore.GoalConfirmationReceipt, error)
	GetGoalConfirmationReceipt(
		context.Context,
		string,
	) (*orchestrationstore.GoalConfirmationReceipt, error)
	ListRecoverableGoalConfirmations(
		context.Context,
		orchestrationstore.ListRecoverableGoalConfirmationsQuery,
	) ([]orchestrationstore.GoalConfirmationReceipt, error)
	MarkGoalConfirmationConfirmed(
		context.Context,
		orchestrationstore.MarkGoalConfirmationCommand,
	) (*orchestrationstore.GoalConfirmationReceipt, error)
	ScheduleGoalConfirmationRetry(
		context.Context,
		orchestrationstore.MarkGoalConfirmationCommand,
	) (*orchestrationstore.GoalConfirmationReceipt, error)
}

// GoalConfirmationRecoveryResult summarizes one bounded durable receipt pass.
type GoalConfirmationRecoveryResult struct {
	Scanned   int
	Confirmed int
	Pending   int
	Failed    int
}

// ReconcileGoalConfirmations retries Goal-side reverse binding independently
// from the request and from ExecutionPlanProposal recovery.
func (s *Service) ReconcileGoalConfirmations(
	ctx context.Context,
	limit int,
) (GoalConfirmationRecoveryResult, error) {
	result := GoalConfirmationRecoveryResult{}
	if s == nil || s.goalConfirmations == nil {
		return result, errors.New("Execution Goal confirmation repository is unavailable")
	}
	if limit <= 0 {
		return result, errors.New("positive Goal confirmation recovery limit is required")
	}
	receipts, err := s.goalConfirmations.ListRecoverableGoalConfirmations(
		ctx,
		orchestrationstore.ListRecoverableGoalConfirmationsQuery{
			Now:   s.currentTime(),
			Limit: limit,
		},
	)
	if err != nil {
		return result, err
	}
	result.Scanned = len(receipts)
	var itemErrors []error
	for index := range receipts {
		receipt := receipts[index]
		snapshot, snapshotErr := s.repository.GetSnapshot(ctx, receipt.ExecutionID)
		if snapshotErr != nil || snapshot == nil {
			if snapshotErr == nil {
				snapshotErr = errors.New("Goal confirmation Execution disappeared")
			}
			s.scheduleGoalConfirmationRetry(ctx, receipt, snapshotErr)
			result.Failed++
			itemErrors = append(itemErrors, fmt.Errorf(
				"reconcile Goal confirmation %s: %w",
				receipt.ExecutionID,
				snapshotErr,
			))
			continue
		}
		if validationErr := goalConfirmationReceiptMatchesSnapshot(receipt, snapshot); validationErr != nil {
			s.scheduleGoalConfirmationRetry(ctx, receipt, validationErr)
			result.Failed++
			itemErrors = append(itemErrors, fmt.Errorf(
				"reconcile Goal confirmation %s: %w",
				receipt.ExecutionID,
				validationErr,
			))
			continue
		}
		if confirmErr := s.confirmGoalExecutionBinding(ctx, snapshot); confirmErr != nil {
			result.Pending++
			itemErrors = append(itemErrors, fmt.Errorf(
				"reconcile Goal confirmation %s: %w",
				receipt.ExecutionID,
				confirmErr,
			))
			continue
		}
		result.Confirmed++
		s.invalidateSnapshot(ctx, snapshot)
	}
	return result, errors.Join(itemErrors...)
}

func (s *Service) confirmGoalExecutionBinding(
	ctx context.Context,
	snapshot *protocol.ExecutionSnapshot,
) error {
	if s == nil || snapshot == nil || strings.TrimSpace(snapshot.Execution.GoalID) == "" {
		return nil
	}
	if s.goalConfirmations != nil {
		receipt, err := s.goalConfirmations.GetGoalConfirmationReceipt(
			ctx,
			snapshot.Execution.ID,
		)
		if err == nil && receipt == nil {
			receipt, err = s.goalConfirmations.EnsureGoalConfirmationReceipt(
				ctx,
				snapshot.Execution.ID,
			)
		}
		if err != nil {
			return fmt.Errorf("ensure durable Goal confirmation receipt: %w", err)
		}
		if receipt == nil {
			return errors.New("durable Goal confirmation receipt disappeared")
		}
		if receipt.State == orchestrationstore.GoalConfirmationConfirmed {
			return nil
		}
		if err = goalConfirmationReceiptMatchesSnapshot(*receipt, snapshot); err != nil {
			return err
		}
	}

	confirmer, ok := s.explicitGoalGateway.(goalExecutionBindingConfirmer)
	if !ok {
		err := errors.New("Goal execution binding confirmation is unavailable")
		return s.recordGoalConfirmationFailure(ctx, snapshot, err)
	}
	confirmErr := confirmer.ConfirmGoalExecutionBinding(ctx, GoalExecutionBindingConfirmation{
		GoalID:                snapshot.Execution.GoalID,
		GoalObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
		ExecutionID:           snapshot.Execution.ID,
		CompletionCriteria:    append([]string(nil), snapshot.Execution.CompletionCriteria...),
	})
	if confirmErr != nil {
		return s.recordGoalConfirmationFailure(ctx, snapshot, confirmErr)
	}
	if s.goalConfirmations == nil {
		return nil
	}
	_, err := s.goalConfirmations.MarkGoalConfirmationConfirmed(
		ctx,
		goalConfirmationCommand(snapshot),
	)
	if err != nil {
		return fmt.Errorf("Goal binding is confirmed but durable receipt finalization is pending: %w", err)
	}
	return nil
}

func (s *Service) recordGoalConfirmationFailure(
	ctx context.Context,
	snapshot *protocol.ExecutionSnapshot,
	cause error,
) error {
	if s == nil || s.goalConfirmations == nil || snapshot == nil {
		return cause
	}
	command := goalConfirmationCommand(snapshot)
	nextAttemptAt := s.currentTime().Add(goalConfirmationRetryDelay)
	command.NextAttemptAt = &nextAttemptAt
	command.LastError = cause.Error()
	receipt, err := s.goalConfirmations.ScheduleGoalConfirmationRetry(ctx, command)
	if err == nil && receipt != nil &&
		receipt.State == orchestrationstore.GoalConfirmationConfirmed {
		// Another worker completed the idempotent Goal confirmation while this
		// attempt was failing; the durable terminal receipt wins.
		return nil
	}
	return errors.Join(cause, err)
}

func (s *Service) scheduleGoalConfirmationRetry(
	ctx context.Context,
	receipt orchestrationstore.GoalConfirmationReceipt,
	cause error,
) {
	if s == nil || s.goalConfirmations == nil || cause == nil {
		return
	}
	nextAttemptAt := s.currentTime().Add(goalConfirmationRetryDelay)
	_, _ = s.goalConfirmations.ScheduleGoalConfirmationRetry(
		ctx,
		orchestrationstore.MarkGoalConfirmationCommand{
			ExecutionID:           receipt.ExecutionID,
			GoalID:                receipt.GoalID,
			GoalObjectiveRevision: receipt.GoalObjectiveRevision,
			NextAttemptAt:         &nextAttemptAt,
			LastError:             cause.Error(),
		},
	)
}

func goalConfirmationCommand(
	snapshot *protocol.ExecutionSnapshot,
) orchestrationstore.MarkGoalConfirmationCommand {
	return orchestrationstore.MarkGoalConfirmationCommand{
		ExecutionID:           strings.TrimSpace(snapshot.Execution.ID),
		GoalID:                strings.TrimSpace(snapshot.Execution.GoalID),
		GoalObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
	}
}

func goalConfirmationReceiptMatchesSnapshot(
	receipt orchestrationstore.GoalConfirmationReceipt,
	snapshot *protocol.ExecutionSnapshot,
) error {
	if snapshot == nil ||
		strings.TrimSpace(snapshot.Execution.ID) != strings.TrimSpace(receipt.ExecutionID) ||
		strings.TrimSpace(snapshot.Execution.GoalID) != strings.TrimSpace(receipt.GoalID) ||
		snapshot.Execution.GoalObjectiveRevision != receipt.GoalObjectiveRevision ||
		!slices.Equal(snapshot.Execution.CompletionCriteria, receipt.CompletionCriteria) {
		return errors.New("durable Goal confirmation receipt no longer matches Execution truth")
	}
	return nil
}
