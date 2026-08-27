// INPUT: exact owner/job/run script identity and its cancellable execution context.
// OUTPUT: process-local cancel-and-drain fence used by delete and manual recovery.
// POS: Automation script lifecycle registry; durable cancellation cannot outrun a local script process.
package automation

import (
	"context"
	"errors"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

type scriptAttempt struct {
	ownerUserID    string
	jobID          string
	cancel         context.CancelFunc
	done           chan struct{}
	terminationErr error
}

// ErrExecutionAttemptOwnershipUnconfirmed 表示删除请求所在实例不能证明活跃执行已经停止。
var ErrExecutionAttemptOwnershipUnconfirmed = errors.New("active execution is owned by another instance or has not registered locally")

func (s *Service) launchScriptObservation(
	job automationdomain.ScheduledTask,
	runID string,
	scheduledFor time.Time,
) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return errors.New("script run identity is required")
	}
	runCtx, cancel := context.WithCancel(backgroundContextForJobOwner(job))
	attempt := &scriptAttempt{
		ownerUserID: strings.TrimSpace(job.OwnerUserID),
		jobID:       strings.TrimSpace(job.JobID),
		cancel:      cancel,
		done:        make(chan struct{}),
	}
	s.scriptAttemptMu.Lock()
	if s.scriptAttempts[runID] != nil {
		s.scriptAttemptMu.Unlock()
		cancel()
		return errors.New("script run is already registered")
	}
	s.scriptAttempts[runID] = attempt
	s.scriptAttemptMu.Unlock()

	go func() {
		defer func() {
			s.scriptAttemptMu.Lock()
			if s.scriptAttempts[runID] == attempt {
				delete(s.scriptAttempts, runID)
			}
			s.scriptAttemptMu.Unlock()
			close(attempt.done)
			cancel()
		}()
		attempt.terminationErr = s.observeScriptJob(runCtx, job, runID, scheduledFor)
	}()
	return nil
}

func (s *Service) cancelScriptAttemptAndWait(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
) error {
	runID := strings.TrimSpace(run.RunID)
	s.scriptAttemptMu.Lock()
	attempt := s.scriptAttempts[runID]
	s.scriptAttemptMu.Unlock()
	if attempt == nil {
		if automationdomain.NormalizeExecutionKind(job.ExecutionKind) == automationdomain.ExecutionKindScript &&
			run.EffectStarted && strings.TrimSpace(run.Status) == automationdomain.RunStatusRunning {
			// Absence from this process is not proof that another service instance
			// has stopped its child process. Keep the durable deletion claim and let
			// the execution owner/scheduler leader resume cleanup.
			return ErrExecutionAttemptOwnershipUnconfirmed
		}
		return nil
	}
	if attempt.ownerUserID != strings.TrimSpace(job.OwnerUserID) ||
		attempt.jobID != strings.TrimSpace(job.JobID) {
		return errors.New("script cancellation identity does not match task")
	}
	attempt.cancel()
	select {
	case <-attempt.done:
		if attempt.terminationErr != nil {
			return errors.Join(ErrExecutionAttemptOwnershipUnconfirmed, attempt.terminationErr)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
