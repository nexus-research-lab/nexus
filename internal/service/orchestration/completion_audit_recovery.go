// INPUT: accepted Review 的 durable completion receipt、最新 Execution snapshot 与有界 scan limit。
// OUTPUT: blocker-aware Complete CAS、deferred/discarded receipt 或跨进程恢复计数。
// POS: foreground review-to-complete 之外的 authoritative background recovery；不替代可见 objective-alignment audit。
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

const (
	completionAuditRetryMinDelay = 15 * time.Second
	completionAuditRetryMaxDelay = 15 * time.Minute
)

// CompletionAuditRepository is separate from the aggregate Repository so
// focused command tests do not need to implement background recovery methods.
type CompletionAuditRepository interface {
	ListRecoverableCompletionAudits(
		context.Context,
		orchestrationstore.ListRecoverableCompletionAuditsQuery,
	) ([]orchestrationstore.CompletionAuditReceipt, error)
	ScheduleCompletionAuditRetry(
		context.Context,
		orchestrationstore.TransitionCompletionAuditCommand,
	) (*orchestrationstore.CompletionAuditReceipt, error)
	MarkCompletionAuditCompleted(
		context.Context,
		orchestrationstore.TransitionCompletionAuditCommand,
	) (*orchestrationstore.CompletionAuditReceipt, error)
	MarkCompletionAuditDiscarded(
		context.Context,
		orchestrationstore.TransitionCompletionAuditCommand,
	) (*orchestrationstore.CompletionAuditReceipt, error)
}

// CompletionAuditRecoveryResult summarizes one bounded durable receipt pass.
type CompletionAuditRecoveryResult struct {
	Scanned   int
	Completed int
	Deferred  int
	Discarded int
	Failed    int
}

// ReconcileCompletionAudits re-derives completion readiness from the latest
// snapshot and lets Repository.Complete enforce the final Execution CAS. Two
// workers may list the same receipt; only one can commit the terminal event.
func (s *Service) ReconcileCompletionAudits(
	ctx context.Context,
	limit int,
) (CompletionAuditRecoveryResult, error) {
	result := CompletionAuditRecoveryResult{}
	if s == nil || s.completionAudits == nil {
		return result, errors.New("Execution completion audit repository is unavailable")
	}
	if s.repository == nil {
		return result, errors.New("Execution orchestration repository is unavailable")
	}
	if limit <= 0 {
		return result, errors.New("positive completion audit recovery limit is required")
	}
	receipts, err := s.completionAudits.ListRecoverableCompletionAudits(
		ctx,
		orchestrationstore.ListRecoverableCompletionAuditsQuery{
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
				snapshotErr = errors.New("completion audit Execution disappeared")
			}
			if retryErr := s.deferCompletionAudit(ctx, receipt, snapshotErr.Error()); retryErr != nil &&
				!errors.Is(retryErr, orchestrationstore.ErrVersionConflict) {
				itemErrors = append(itemErrors, retryErr)
			}
			result.Failed++
			itemErrors = append(itemErrors, fmt.Errorf(
				"reconcile completion audit %s: %w",
				receipt.ExecutionID,
				snapshotErr,
			))
			continue
		}

		switch snapshot.Execution.Status {
		case protocol.ExecutionStatusCompleted:
			if settleErr := s.settleCompletedAudit(ctx, receipt); settleErr != nil {
				if errors.Is(settleErr, orchestrationstore.ErrVersionConflict) {
					result.Deferred++
					continue
				}
				result.Failed++
				itemErrors = append(itemErrors, settleErr)
				continue
			}
			result.Completed++
			continue
		case protocol.ExecutionStatusFailed,
			protocol.ExecutionStatusCancelled,
			protocol.ExecutionStatusSuperseded:
			if discardErr := s.discardCompletionAudit(
				ctx,
				receipt,
				"Execution is terminal with status "+string(snapshot.Execution.Status),
			); discardErr != nil {
				if errors.Is(discardErr, orchestrationstore.ErrVersionConflict) {
					result.Deferred++
					continue
				}
				result.Failed++
				itemErrors = append(itemErrors, discardErr)
				continue
			}
			result.Discarded++
			continue
		case protocol.ExecutionStatusPaused:
			if deferErr := s.deferCompletionAudit(ctx, receipt, "Execution is paused"); deferErr != nil &&
				!errors.Is(deferErr, orchestrationstore.ErrVersionConflict) {
				result.Failed++
				itemErrors = append(itemErrors, deferErr)
				continue
			}
			result.Deferred++
			continue
		case protocol.ExecutionStatusActive, protocol.ExecutionStatusWaiting:
		default:
			if deferErr := s.deferCompletionAudit(
				ctx,
				receipt,
				"Execution has unsupported completion state "+string(snapshot.Execution.Status),
			); deferErr != nil && !errors.Is(deferErr, orchestrationstore.ErrVersionConflict) {
				result.Failed++
				itemErrors = append(itemErrors, deferErr)
				continue
			}
			result.Deferred++
			continue
		}

		if len(snapshot.CompletionBlockers) > 0 {
			reason := "completion blockers: " + strings.Join(snapshot.CompletionBlockers, ", ")
			if deferErr := s.deferCompletionAudit(ctx, receipt, reason); deferErr != nil &&
				!errors.Is(deferErr, orchestrationstore.ErrVersionConflict) {
				result.Failed++
				itemErrors = append(itemErrors, deferErr)
				continue
			}
			result.Deferred++
			continue
		}

		completed, completeErr := s.repository.Complete(
			ctx,
			orchestrationstore.CompleteCommand{
				ExecutionID:              snapshot.Execution.ID,
				ExpectedExecutionVersion: snapshot.Execution.Version,
				CompletedAt:              s.currentTime(),
				Meta: orchestrationstore.CommandMeta{
					CommandID: "completion-audit:" + receipt.TriggerAcceptanceID,
					EventID:   s.id("event"),
					ActorKind: protocol.ExecutionActorSystem,
					ActorID:   "completion-audit-reconciler",
				},
			},
		)
		if completeErr != nil {
			if errors.Is(completeErr, orchestrationstore.ErrVersionConflict) {
				latest, latestErr := s.repository.GetSnapshot(ctx, receipt.ExecutionID)
				if latestErr == nil && latest != nil &&
					latest.Execution.Status == protocol.ExecutionStatusCompleted {
					if settleErr := s.settleCompletedAudit(ctx, receipt); settleErr == nil ||
						errors.Is(settleErr, orchestrationstore.ErrVersionConflict) {
						result.Completed++
						continue
					}
				}
			}
			if errors.Is(completeErr, orchestrationstore.ErrVersionConflict) ||
				errors.Is(completeErr, orchestrationstore.ErrCompletionBlocked) ||
				orchestrationstore.IsTransientMutationError(completeErr) {
				if deferErr := s.deferCompletionAudit(ctx, receipt, completeErr.Error()); deferErr != nil &&
					!errors.Is(deferErr, orchestrationstore.ErrVersionConflict) {
					result.Failed++
					itemErrors = append(itemErrors, deferErr)
					continue
				}
				result.Deferred++
				continue
			}
			result.Failed++
			itemErrors = append(itemErrors, fmt.Errorf(
				"complete Execution %s from durable audit: %w",
				receipt.ExecutionID,
				completeErr,
			))
			continue
		}
		result.Completed++
		s.invalidateSnapshot(ctx, completed)
	}
	return result, errors.Join(itemErrors...)
}

func (s *Service) deferCompletionAudit(
	ctx context.Context,
	receipt orchestrationstore.CompletionAuditReceipt,
	reason string,
) error {
	nextAttemptAt := s.currentTime().Add(completionAuditRetryBackoff(receipt.AttemptCount))
	_, err := s.completionAudits.ScheduleCompletionAuditRetry(
		ctx,
		orchestrationstore.TransitionCompletionAuditCommand{
			ExecutionID:         receipt.ExecutionID,
			TriggerAcceptanceID: receipt.TriggerAcceptanceID,
			ExpectedVersion:     receipt.Version,
			NextAttemptAt:       &nextAttemptAt,
			LastError:           reason,
		},
	)
	return err
}

func completionAuditRetryBackoff(attemptCount int) time.Duration {
	delay := completionAuditRetryMinDelay
	for attemptCount > 0 && delay < completionAuditRetryMaxDelay {
		delay *= 2
		attemptCount--
	}
	if delay > completionAuditRetryMaxDelay {
		return completionAuditRetryMaxDelay
	}
	return delay
}

func (s *Service) settleCompletedAudit(
	ctx context.Context,
	receipt orchestrationstore.CompletionAuditReceipt,
) error {
	_, err := s.completionAudits.MarkCompletionAuditCompleted(
		ctx,
		completionAuditTransition(receipt),
	)
	return err
}

func (s *Service) discardCompletionAudit(
	ctx context.Context,
	receipt orchestrationstore.CompletionAuditReceipt,
	reason string,
) error {
	command := completionAuditTransition(receipt)
	command.LastError = reason
	_, err := s.completionAudits.MarkCompletionAuditDiscarded(ctx, command)
	return err
}

func completionAuditTransition(
	receipt orchestrationstore.CompletionAuditReceipt,
) orchestrationstore.TransitionCompletionAuditCommand {
	return orchestrationstore.TransitionCompletionAuditCommand{
		ExecutionID:         receipt.ExecutionID,
		TriggerAcceptanceID: receipt.TriggerAcceptanceID,
		ExpectedVersion:     receipt.Version,
	}
}
