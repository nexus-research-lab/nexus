// INPUT: Host startup or an exact DM/Room root round terminal outcome.
// OUTPUT: CAS-protected orphan settlement and durable coordinator recovery input.
// POS: Root Attempt recovery never retries tools or recreates accepted work.
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type rootRecoveryRepository interface {
	ListRecoveryExecutions(context.Context, string, int) ([]protocol.Execution, error)
}

// ExecutionRecoveryConsumer durably queues an inspect/reconcile coordination turn.
type ExecutionRecoveryConsumer interface {
	QueueExecutionRecovery(context.Context, protocol.Execution, string) error
}

// SetExecutionRecoveryConsumer binds the host queue adapters during assembly.
func (s *Service) SetExecutionRecoveryConsumer(consumer ExecutionRecoveryConsumer) {
	s.rootRecoveryConsumer = consumer
}

func (s *Service) queueRootRecovery(ctx context.Context, snapshot *protocol.ExecutionSnapshot, key string) error {
	if snapshot == nil || snapshot.Execution.Status != protocol.ExecutionStatusActive || snapshot.Execution.GoalID != "" || s.rootRecoveryConsumer == nil {
		return nil
	}
	return s.rootRecoveryConsumer.QueueExecutionRecovery(ctx, snapshot.Execution, key)
}

// ErrExecutionRecoveryTargetUnavailable reports that the recovery coordinator's
// conversation, Room membership or Agent no longer exists. Recovery skips such
// an Execution instead of treating it as a host failure.
var ErrExecutionRecoveryTargetUnavailable = errors.New("execution recovery target is unavailable")

// RecoverRootAttempts runs before runtime admissions at process startup. Pending
// attempts still belong to their outbox; only previously running roots are orphaned.
// Each Execution is isolated: one unreadable or unrecoverable graph is reported in
// the joined error but never stops the scan or the remaining executions.
func (s *Service) RecoverRootAttempts(ctx context.Context) error {
	repository, ok := s.repository.(rootRecoveryRepository)
	if !ok {
		return errors.New("root recovery repository unavailable")
	}
	var failures []error
	after := ""
	for {
		executions, err := repository.ListRecoveryExecutions(ctx, after, 64)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, execution := range executions {
			if err := ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}
			if err := s.recoverRootExecution(ctx, execution); err != nil {
				failures = append(failures, fmt.Errorf("recover Execution %s: %w", execution.ID, err))
			}
			after = execution.ID
		}
		if len(executions) < 64 {
			return errors.Join(failures...)
		}
	}
}

func (s *Service) recoverRootExecution(ctx context.Context, execution protocol.Execution) error {
	snapshot, err := s.repository.GetSnapshot(ctx, execution.ID)
	if err != nil || snapshot == nil {
		return err
	}
	actor := ActorContext{OwnerUserID: execution.OwnerUserID, SessionKey: execution.SessionKey, AgentID: execution.CoordinatorAgentID, ExecutionID: execution.ID, ScopeKind: execution.ScopeKind, RoomID: execution.RoomID, ConversationID: execution.ConversationID}
	versionBefore := snapshot.Execution.Version
	snapshot, err = s.finishRootAttempts(ctx, actor, snapshot, "", hostRestartAttemptStatus, hostRestartAttemptReason)
	if err != nil {
		return err
	}
	if rootRecoveryStoppedByUser(snapshot) {
		return nil
	}
	// Only wake the coordinator for work the host actually left in flight: an
	// orphan settled just now, or a root the host itself interrupted. A graph
	// idle since long before this start (waiting for input, or abandoned) is
	// not revived merely by restarting.
	if snapshot.Execution.Version == versionBefore && !executionHasHostInterruptedRoot(snapshot) {
		return nil
	}
	return skipUnavailableRecoveryTarget(s.queueRootRecovery(ctx, snapshot, fmt.Sprintf("startup:%s:%d", execution.ID, snapshot.Execution.Version)))
}

const (
	hostRestartAttemptStatus = protocol.WorkAttemptStatusInterrupted
	hostRestartAttemptReason = "host_restart: reconcile unknown side effects before takeover"
	// hostShutdownAttemptReason mirrors runtime.InterruptHostShutdown, which DM
	// and Room record as the failure reason of roots cancelled by host exit.
	hostShutdownAttemptReason = "host_shutdown"
)

// executionHasHostInterruptedRoot reports a current Assignment whose latest root
// Attempt ended because of the host (shutdown, restart) or was explicitly marked
// for host recovery. User and host control stops never qualify.
func executionHasHostInterruptedRoot(snapshot *protocol.ExecutionSnapshot) bool {
	for _, assignment := range snapshot.Assignments {
		if !currentAssignment(assignment) {
			continue
		}
		latest := latestRootAttemptForAssignment(snapshot, assignment.ID)
		if latest == nil || (latest.Status != protocol.WorkAttemptStatusFailed && latest.Status != protocol.WorkAttemptStatusInterrupted) {
			continue
		}
		required, _ := latest.Metadata["host_recovery_required"].(bool)
		if required || latest.FailureReason == hostShutdownAttemptReason || latest.FailureReason == hostRestartAttemptReason {
			return true
		}
	}
	return false
}

func skipUnavailableRecoveryTarget(err error) error {
	if errors.Is(err, ErrExecutionRecoveryTargetUnavailable) {
		return nil
	}
	return err
}

// FinishDMRoundAttempts closes only root attempts bound to this physical DM turn.
func (s *Service) FinishDMRoundAttempts(ctx context.Context, actor ActorContext, status protocol.WorkAttemptStatus, reason string) error {
	if actor.ScopeKind != protocol.ExecutionScopeDM || actor.RuntimeRoundID == "" {
		return nil
	}
	execution, err := s.repository.FindCurrent(ctx, actor.OwnerUserID, actor.SessionKey)
	if err != nil || execution == nil {
		return err
	}
	snapshot, err := s.repository.GetSnapshot(ctx, execution.ID)
	if err != nil {
		return err
	}
	if err := authorizeSnapshot(actor, snapshot); err != nil {
		return err
	}
	_, err = s.finishRootAttempts(ctx, actor, snapshot, actor.RuntimeRoundID, status, reason)
	return err
}

func (s *Service) finishRootAttempts(ctx context.Context, actor ActorContext, snapshot *protocol.ExecutionSnapshot, roundID string, status protocol.WorkAttemptStatus, reason string) (*protocol.ExecutionSnapshot, error) {
	if snapshot == nil {
		return nil, nil
	}
	candidates := append([]protocol.WorkAttempt(nil), snapshot.Attempts...)
	for _, candidate := range candidates {
		if candidate.ParentAttemptID != "" || !recoverableRootAttempt(snapshot, candidate, roundID == "") {
			continue
		}
		if roundID != "" && candidate.RuntimeRoundID != roundID && candidate.AgentRoundID != actor.AgentRoundID {
			continue
		}
		for retry := 0; retry < 3; retry++ {
			current := findAttemptByID(snapshot, candidate.ID)
			if current == nil || !recoverableRootAttempt(snapshot, *current, roundID == "") {
				break
			}
			terminal := *current
			terminal.Status = status
			terminal.FailureReason = reason
			updated, err := s.repository.FinishAttempt(ctx, orchestrationstore.FinishAttemptCommand{
				ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAttemptVersion: current.Version, Attempt: terminal,
				Meta: s.commandMeta(actor, "root-terminal:"+candidate.ID+":"+string(status), "root-terminal"),
			})
			if errors.Is(err, orchestrationstore.ErrVersionConflict) {
				snapshot, err = s.repository.GetSnapshot(ctx, snapshot.Execution.ID)
				if err != nil {
					return nil, err
				}
				if retry == 2 {
					return nil, orchestrationstore.ErrVersionConflict
				}
				continue
			}
			if err != nil {
				return nil, err
			}
			snapshot = updated
			s.invalidateSnapshot(ctx, snapshot)
			break
		}
	}
	return snapshot, nil
}

// RecoveryExecutionCurrent fences queued recovery against replacement and pause.
func (s *Service) RecoveryExecutionCurrent(ctx context.Context, owner, session, id string) (bool, error) {
	execution, err := s.repository.FindCurrent(ctx, owner, session)
	return execution != nil && execution.ID == id && execution.Status == protocol.ExecutionStatusActive, err
}

// ResumeMemberWork queues a fresh coordinator inspection after explicit member
// participation resumes. It does not revive an old Attempt or bypass pause gates.
func (s *Service) ResumeMemberWork(ctx context.Context, owner, session, member string) error {
	execution, err := s.repository.FindCurrent(ctx, owner, session)
	if err != nil || execution == nil {
		return err
	}
	snapshot, err := s.repository.GetSnapshot(ctx, execution.ID)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return nil
	}
	for _, assignment := range snapshot.Assignments {
		if !currentAssignment(assignment) || assignment.OwnerAgentID != member {
			continue
		}
		for _, attempt := range snapshot.Attempts {
			if attempt.AssignmentID == assignment.ID && attempt.ParentAttemptID == "" && attempt.Status == protocol.WorkAttemptStatusInterrupted {
				return skipUnavailableRecoveryTarget(s.queueRootRecovery(ctx, snapshot, fmt.Sprintf("member-resume:%s:%s:%d", execution.ID, member, s.currentTime().UnixNano())))
			}
		}
	}
	return nil
}

// A pending root with a delivered outbox has already crossed admission. Pending
// undelivered work remains owned by the ordinary outbox and must not be stolen.
func recoverableRootAttempt(snapshot *protocol.ExecutionSnapshot, attempt protocol.WorkAttempt, startup bool) bool {
	if attempt.Status == protocol.WorkAttemptStatusRunning {
		return true
	}
	if !startup || attempt.Status != protocol.WorkAttemptStatusPending {
		return false
	}
	for _, dispatch := range snapshot.Dispatches {
		if dispatch.ID == attempt.DispatchID && dispatch.Status == protocol.ExecutionDispatchStatusDelivered {
			return true
		}
	}
	return false
}

// A stopped current assignment is not revived merely by restarting the host.
func rootRecoveryStoppedByUser(snapshot *protocol.ExecutionSnapshot) bool {
	for _, assignment := range snapshot.Assignments {
		if !currentAssignment(assignment) {
			continue
		}
		latest := latestRootAttemptForAssignment(snapshot, assignment.ID)
		if latest != nil && latest.Status == protocol.WorkAttemptStatusInterrupted && latest.FailureReason == message.InterruptWithoutMessage {
			return true
		}
	}
	return false
}

// reconcileRootRecoveryInputs repairs the settlement-to-queue crash window from
// the terminal Attempt receipt. Its queue key survives dispatch and restart.
// The recovery coordinator owns cursor access and audits one bounded page.
func (s *Service) reconcileRootRecoveryInputs(ctx context.Context, cursor *string, limit int) error {
	repository, ok := s.repository.(rootRecoveryRepository)
	if !ok || s.rootRecoveryConsumer == nil {
		return nil
	}
	executions, err := repository.ListRecoveryExecutions(ctx, *cursor, limit)
	if err != nil {
		return err
	}
	// Advance past a failing Execution so one bad graph cannot pin the audit
	// cursor and starve every later Execution; the next audit cycle retries it.
	var failures []error
	for _, execution := range executions {
		if err := s.reconcileRootRecoveryInput(ctx, execution); err != nil {
			failures = append(failures, fmt.Errorf("audit Execution %s: %w", execution.ID, err))
		}
		*cursor = execution.ID
	}
	if len(executions) < limit {
		*cursor = ""
	}
	return errors.Join(failures...)
}

func (s *Service) reconcileRootRecoveryInput(ctx context.Context, execution protocol.Execution) error {
	snapshot, err := s.repository.GetSnapshot(ctx, execution.ID)
	if err != nil || snapshot == nil || rootRecoveryStoppedByUser(snapshot) {
		return err
	}
	for _, assignment := range snapshot.Assignments {
		if !currentAssignment(assignment) || assignment.OwnerAgentID == execution.CoordinatorAgentID {
			continue
		}
		latest := latestRootAttemptForAssignment(snapshot, assignment.ID)
		if latest == nil {
			continue
		}
		required, _ := latest.Metadata["host_recovery_required"].(bool)
		if required && (latest.Status == protocol.WorkAttemptStatusFailed || latest.Status == protocol.WorkAttemptStatusInterrupted) {
			if err := skipUnavailableRecoveryTarget(s.queueRootRecovery(ctx, snapshot, "failed:"+latest.ID)); err != nil {
				return err
			}
		}
	}
	return nil
}

func latestRootAttemptForAssignment(snapshot *protocol.ExecutionSnapshot, assignmentID string) *protocol.WorkAttempt {
	var latest *protocol.WorkAttempt
	for i := range snapshot.Attempts {
		attempt := &snapshot.Attempts[i]
		if attempt.AssignmentID == assignmentID && attempt.ParentAttemptID == "" && (latest == nil || attempt.CreatedAt.After(latest.CreatedAt) || attempt.CreatedAt.Equal(latest.CreatedAt) && attempt.ID > latest.ID) {
			latest = attempt
		}
	}
	return latest
}

// RecoveryWorkInputCurrent retires a surviving queue claim for an Attempt that
// startup already settled. Otherwise that obsolete head could block its recovery.
func (s *Service) RecoveryWorkInputCurrent(ctx context.Context, owner, session string, binding protocol.ExecutionWorkBinding) (bool, error) {
	execution, err := s.repository.FindCurrent(ctx, owner, session)
	if err != nil || execution == nil {
		return false, err
	}
	if execution.ID != binding.ExecutionID {
		return false, nil
	}
	snapshot, err := s.repository.GetSnapshot(ctx, execution.ID)
	if err != nil || snapshot == nil {
		return false, err
	}
	if snapshot.Execution.OwnerUserID != owner || snapshot.Execution.SessionKey != session {
		return false, errors.New("recovery work owner or session mismatch")
	}
	attempt := findAttemptByID(snapshot, binding.AttemptID)
	if attempt == nil {
		return false, nil
	}
	return attempt.Status == protocol.WorkAttemptStatusPending || attempt.Status == protocol.WorkAttemptStatusRunning, nil
}
