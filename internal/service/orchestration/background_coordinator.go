// INPUT: durable outbox/recovery deadlines, process-local mutation wakes and bounded domain reconcilers.
// OUTPUT: event-driven Execution dispatch, Subagent recovery and orchestration saga lifecycles.
// POS: orchestration-owned background control plane; app only supplies lifecycle and observation hooks.
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// BackgroundErrorObserver receives recoverable coordinator errors. The loop
// keeps running with bounded backoff after the callback returns.
type BackgroundErrorObserver func(kind string, err error)

// DispatchCoordinatorObserver keeps operational logging outside business
// decisions while preserving the three independent result shapes.
type DispatchCoordinatorObserver struct {
	OnError        BackgroundErrorObserver
	OnCancellation func(CancellationDispatchRunResult)
	OnRoom         func(DispatchRunResult)
	OnReview       func(DispatchRunResult)
}

// RecoveryCoordinatorObserver projects bounded saga recovery results.
type RecoveryCoordinatorObserver struct {
	OnError            BackgroundErrorObserver
	OnCompletionAudit  func(CompletionAuditRecoveryResult)
	OnGoalConfirmation func(GoalConfirmationRecoveryResult)
	OnPlanProposal     func(PlanProposalRecoveryResult)
}

type dispatchDeadlineRepository interface {
	ExecutionDispatchDeadlines(context.Context) (orchestrationstore.DispatchDeadlines, error)
}

type subagentDeadlineRepository interface {
	NextSubagentReconciliationAt(context.Context) (*time.Time, error)
}

type recoveryDeadlineRepository interface {
	OrchestrationRecoveryDeadlines(context.Context) (orchestrationstore.RecoveryDeadlines, error)
}

// WakeExecutionDispatch records a post-commit hint. Duplicate hints are
// coalesced; startup and audit recovery do not depend on its delivery.
func (s *Service) WakeExecutionDispatch() {
	if s != nil && s.dispatchLoop != nil {
		s.dispatchLoop.Notify()
	}
}

// WakeSubagentReconciliation records a newly committed parent-exit deadline.
func (s *Service) WakeSubagentReconciliation() {
	if s != nil && s.subagentLoop != nil {
		s.subagentLoop.Notify()
	}
}

// WakeOrchestrationRecovery records a newly committed saga receipt/retry.
func (s *Service) WakeOrchestrationRecovery() {
	if s != nil && s.recoveryLoop != nil {
		s.recoveryLoop.Notify()
	}
}

// RunExecutionDispatchCoordinator drains cancellation, Room and review
// outboxes when their durable deadlines become due.
func (s *Service) RunExecutionDispatchCoordinator(
	ctx context.Context,
	workerID string,
	batchSize int,
	observer DispatchCoordinatorObserver,
) error {
	if s == nil || s.dispatchLoop == nil {
		return errors.New("Execution dispatch coordinator is unavailable")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || batchSize <= 0 {
		return errors.New("Execution dispatch coordinator requires worker id and positive batch size")
	}
	return s.dispatchLoop.Run(ctx, func(
		runCtx context.Context,
		now time.Time,
	) (duework.Result, error) {
		result, err := s.reconcileExecutionDispatches(
			runCtx,
			now,
			workerID,
			batchSize,
			observer,
		)
		if err != nil && observer.OnError != nil {
			observer.OnError("execution_dispatch", err)
		}
		return result, err
	})
}

func (s *Service) reconcileExecutionDispatches(
	ctx context.Context,
	now time.Time,
	workerID string,
	batchSize int,
	observer DispatchCoordinatorObserver,
) (duework.Result, error) {
	repository, ok := s.repository.(dispatchDeadlineRepository)
	if !ok {
		return duework.Result{}, errors.New("orchestration repository does not expose dispatch deadlines")
	}
	deadlines, err := repository.ExecutionDispatchDeadlines(ctx)
	if err != nil {
		return duework.Result{}, err
	}
	var runErrors []error
	hasMore := false
	processed := false
	progress := map[string]bool{}
	if deadlineDue(deadlines.Cancellation, now) {
		processed = true
		result, runErr := s.DispatchPendingCancellations(ctx, workerID, batchSize)
		if runErr != nil {
			runErrors = append(runErrors, fmt.Errorf("cancellation: %w", runErr))
		} else {
			progress["cancellation"] = result.Claimed > 0
			hasMore = hasMore || result.Claimed >= batchSize
			if observer.OnCancellation != nil {
				observer.OnCancellation(result)
			}
		}
	}
	if deadlineDue(deadlines.Room, now) {
		processed = true
		result, runErr := s.DispatchPending(ctx, workerID, batchSize)
		if runErr != nil {
			runErrors = append(runErrors, fmt.Errorf("room: %w", runErr))
		} else {
			progress["room"] = result.Claimed > 0
			hasMore = hasMore || result.Claimed >= batchSize
			if observer.OnRoom != nil {
				observer.OnRoom(result)
			}
		}
	}
	if deadlineDue(deadlines.Review, now) {
		processed = true
		result, runErr := s.DispatchPendingReviews(ctx, workerID, batchSize)
		if runErr != nil {
			runErrors = append(runErrors, fmt.Errorf("review: %w", runErr))
		} else {
			progress["review"] = result.Claimed > 0
			hasMore = hasMore || result.Claimed >= batchSize
			if observer.OnReview != nil {
				observer.OnReview(result)
			}
		}
	}
	if len(runErrors) > 0 {
		return duework.Result{}, errors.Join(runErrors...)
	}
	if !processed {
		return duework.Result{NextDueAt: earliestDeadline(
			deadlines.Cancellation,
			deadlines.Room,
			deadlines.Review,
		)}, nil
	}
	refreshed, err := repository.ExecutionDispatchDeadlines(ctx)
	if err != nil {
		return duework.Result{}, err
	}
	// A broad candidate query may contain a stale row that a stricter claim CAS
	// correctly rejects. Do not turn that durable inconsistency into a hot loop;
	// a later mutation wake or low-frequency audit will retry it.
	refreshed.Cancellation = suppressStalledDeadline(
		refreshed.Cancellation,
		now,
		progress["cancellation"],
	)
	refreshed.Room = suppressStalledDeadline(refreshed.Room, now, progress["room"])
	refreshed.Review = suppressStalledDeadline(refreshed.Review, now, progress["review"])
	return duework.Result{
		HasMore: hasMore,
		NextDueAt: earliestDeadline(
			refreshed.Cancellation,
			refreshed.Room,
			refreshed.Review,
		),
	}, nil
}

// RunSubagentReconciliationCoordinator handles durable parent-exit deadlines
// and the one process-generation orphan pass without periodic scans.
func (s *Service) RunSubagentReconciliationCoordinator(
	ctx context.Context,
	processStartedAt time.Time,
	batchSize int,
	onResult func(string, SubagentReconciliationResult),
	onError BackgroundErrorObserver,
) error {
	if s == nil || s.subagentLoop == nil {
		return errors.New("Subagent reconciliation coordinator is unavailable")
	}
	if processStartedAt.IsZero() || batchSize <= 0 {
		return errors.New("Subagent coordinator requires process start and positive batch size")
	}
	orphanDueAt := processStartedAt.UTC().Add(protocol.SubagentReconciliationGrace)
	orphanComplete := false
	return s.subagentLoop.Run(ctx, func(
		runCtx context.Context,
		now time.Time,
	) (duework.Result, error) {
		result, err := s.reconcileSubagents(
			runCtx,
			now,
			processStartedAt.UTC(),
			orphanDueAt,
			&orphanComplete,
			batchSize,
			onResult,
		)
		if err != nil && onError != nil {
			onError("subagent_reconciliation", err)
		}
		return result, err
	})
}

func (s *Service) reconcileSubagents(
	ctx context.Context,
	now time.Time,
	processStartedAt time.Time,
	orphanDueAt time.Time,
	orphanComplete *bool,
	batchSize int,
	onResult func(string, SubagentReconciliationResult),
) (duework.Result, error) {
	repository, ok := s.repository.(subagentDeadlineRepository)
	if !ok {
		return duework.Result{}, errors.New("orchestration repository does not expose subagent deadlines")
	}
	hasMore := false
	processed := false
	orphanStalled := false
	deadlineStalled := false
	if orphanComplete != nil && !*orphanComplete && !orphanDueAt.After(now) {
		processed = true
		result, err := s.ReconcileOrphanedSubagents(ctx, processStartedAt, batchSize)
		if err != nil {
			return duework.Result{}, err
		}
		if onResult != nil {
			onResult("restart_orphan", result)
		}
		if result.Scanned >= batchSize && result.Reconciled > 0 {
			hasMore = true
		} else if result.Scanned >= batchSize {
			orphanStalled = true
		} else {
			*orphanComplete = true
		}
	}
	nextDueAt, err := repository.NextSubagentReconciliationAt(ctx)
	if err != nil {
		return duework.Result{}, err
	}
	if deadlineDue(nextDueAt, now) {
		processed = true
		result, reconcileErr := s.ReconcileExpiredSubagents(ctx, batchSize)
		if reconcileErr != nil {
			return duework.Result{}, reconcileErr
		}
		if onResult != nil {
			onResult("deadline", result)
		}
		if result.Scanned >= batchSize && result.Reconciled > 0 {
			hasMore = true
		} else if result.Reconciled == 0 {
			deadlineStalled = true
		}
	}
	if !processed {
		if orphanComplete != nil && !*orphanComplete {
			nextDueAt = earliestDeadline(nextDueAt, &orphanDueAt)
		}
		return duework.Result{NextDueAt: nextDueAt}, nil
	}
	nextDueAt, err = repository.NextSubagentReconciliationAt(ctx)
	if err != nil {
		return duework.Result{}, err
	}
	if deadlineStalled {
		nextDueAt = suppressStalledDeadline(nextDueAt, now, false)
	}
	if orphanComplete != nil && !*orphanComplete && !orphanStalled {
		nextDueAt = earliestDeadline(nextDueAt, &orphanDueAt)
	}
	return duework.Result{HasMore: hasMore, NextDueAt: nextDueAt}, nil
}

// RunRecoveryCoordinator drives the three durable orchestration sagas from one
// deadline snapshot while preserving their independent state machines.
func (s *Service) RunRecoveryCoordinator(
	ctx context.Context,
	batchSize int,
	observer RecoveryCoordinatorObserver,
) error {
	if s == nil || s.recoveryLoop == nil {
		return errors.New("orchestration recovery coordinator is unavailable")
	}
	if batchSize <= 0 {
		return errors.New("orchestration recovery coordinator requires positive batch size")
	}
	return s.recoveryLoop.Run(ctx, func(
		runCtx context.Context,
		now time.Time,
	) (duework.Result, error) {
		result, err := s.reconcileRecoveries(runCtx, now, batchSize, observer)
		if err != nil && observer.OnError != nil {
			observer.OnError("orchestration_recovery", err)
		}
		return result, err
	})
}

func (s *Service) reconcileRecoveries(
	ctx context.Context,
	now time.Time,
	batchSize int,
	observer RecoveryCoordinatorObserver,
) (duework.Result, error) {
	repository, ok := s.repository.(recoveryDeadlineRepository)
	if !ok {
		return duework.Result{}, errors.New("orchestration repository does not expose recovery deadlines")
	}
	deadlines, err := repository.OrchestrationRecoveryDeadlines(ctx)
	if err != nil {
		return duework.Result{}, err
	}
	hasMore := false
	processed := false
	var runErrors []error
	if deadlineDue(deadlines.CompletionAudit, now) {
		processed = true
		result, runErr := s.ReconcileCompletionAudits(ctx, batchSize)
		if runErr != nil {
			runErrors = append(runErrors, fmt.Errorf("completion audit: %w", runErr))
		}
		hasMore = hasMore || result.Scanned >= batchSize
		if observer.OnCompletionAudit != nil {
			observer.OnCompletionAudit(result)
		}
	}
	if deadlineDue(deadlines.GoalConfirmation, now) {
		processed = true
		result, runErr := s.ReconcileGoalConfirmations(ctx, batchSize)
		if runErr != nil {
			runErrors = append(runErrors, fmt.Errorf("Goal confirmation: %w", runErr))
		}
		hasMore = hasMore || result.Scanned >= batchSize
		if observer.OnGoalConfirmation != nil {
			observer.OnGoalConfirmation(result)
		}
	}
	if deadlineDue(deadlines.PlanProposal, now) {
		processed = true
		result, runErr := s.ReconcilePlanProposals(ctx, batchSize)
		if runErr != nil {
			runErrors = append(runErrors, fmt.Errorf("Plan proposal: %w", runErr))
		}
		hasMore = hasMore || result.Scanned >= batchSize
		if observer.OnPlanProposal != nil {
			observer.OnPlanProposal(result)
		}
	}
	if len(runErrors) > 0 {
		return duework.Result{}, errors.Join(runErrors...)
	}
	if !processed {
		return duework.Result{NextDueAt: earliestDeadline(
			deadlines.CompletionAudit,
			deadlines.GoalConfirmation,
			deadlines.PlanProposal,
		)}, nil
	}
	refreshed, err := repository.OrchestrationRecoveryDeadlines(ctx)
	if err != nil {
		return duework.Result{}, err
	}
	return duework.Result{
		HasMore: hasMore,
		NextDueAt: earliestDeadline(
			refreshed.CompletionAudit,
			refreshed.GoalConfirmation,
			refreshed.PlanProposal,
		),
	}, nil
}

func deadlineDue(deadline *time.Time, now time.Time) bool {
	return deadline != nil && !deadline.UTC().After(now.UTC())
}

func earliestDeadline(deadlines ...*time.Time) *time.Time {
	var earliest *time.Time
	for _, deadline := range deadlines {
		if deadline == nil {
			continue
		}
		value := deadline.UTC()
		if earliest == nil || value.Before(*earliest) {
			copy := value
			earliest = &copy
		}
	}
	return earliest
}

func suppressStalledDeadline(
	deadline *time.Time,
	now time.Time,
	madeProgress bool,
) *time.Time {
	if !madeProgress && deadlineDue(deadline, now) {
		return nil
	}
	return deadline
}
