// INPUT: structured Room slot 的 trusted WorkBinding、物理 runtime identity 与终态证据。
// OUTPUT: root Attempt 的幂等、CAS 保护终态写入及 session 失效事实；已收口 predecessor 的迟到回调为 no-op，不隐式创建 Submission 或 Acceptance。
// POS: Room runtime 生命周期到 Execution Attempt 状态机的原子终态桥。
package orchestration

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

const roomAttemptTerminalMutationAttempts = 3

// RoomAttemptTerminalInput 只接受 Room host 从 slot 观测到的物理终态。
type RoomAttemptTerminalInput struct {
	Binding           protocol.ExecutionWorkBinding
	Status            protocol.WorkAttemptStatus
	FailureReason     string
	RuntimeSessionKey string
	RoomSessionID     string
	SDKSessionID      string
}

// FinishRoomAttempt 终结 structured Room WorkBinding 的 root Attempt。正常 finished
// 只产生 Attempt succeeded；Submission 与 Acceptance 仍必须由模型语义工具创建。
func (s *Service) FinishRoomAttempt(
	ctx context.Context,
	actor ActorContext,
	input RoomAttemptTerminalInput,
) error {
	if err := validateActor(actor); err != nil {
		return err
	}
	switch input.Status {
	case protocol.WorkAttemptStatusSucceeded,
		protocol.WorkAttemptStatusFailed,
		protocol.WorkAttemptStatusInterrupted:
	default:
		return domainError(
			ErrorCodeInvalidInput,
			"Room root Attempt terminal status must be succeeded, failed or interrupted",
		)
	}
	if s.repository == nil {
		return errors.New("orchestration repository is nil")
	}
	binding := normalizeExecutionWorkBinding(&input.Binding)
	if !completeExecutionWorkBinding(binding) {
		return workBindingMismatch("Room root Attempt terminal binding is incomplete")
	}
	actor.ExecutionID = binding.ExecutionID
	for range roomAttemptTerminalMutationAttempts {
		snapshot, err := s.repository.GetSnapshot(ctx, binding.ExecutionID)
		if err != nil {
			return err
		}
		if snapshot == nil {
			return domainError(ErrorCodeNoCurrentExecution, "bound Room Execution was not found")
		}
		if err = authorizeSnapshot(actor, snapshot); err != nil {
			return err
		}
		// Retarget 会在取消通知抵达旧 Room slot 前原子终结 predecessor。
		// 身份已由 owner/session/Room 与 exact ExecutionID 校验；terminal aggregate
		// 不再可变，因此迟到的物理终态只能幂等收口，不能因 active Plan 已移除
		// 再制造一次 work_binding_mismatch。
		if !isCurrentExecutionStatus(snapshot.Execution.Status) {
			s.invalidateSnapshot(ctx, snapshot)
			return nil
		}
		attempt, authErr := authorizeRoomAttemptTerminalBinding(
			snapshot,
			actor.AgentID,
			binding,
		)
		if authErr != nil {
			return authErr
		}
		if attempt.Status != protocol.WorkAttemptStatusPending &&
			attempt.Status != protocol.WorkAttemptStatusRunning {
			s.invalidateSnapshot(ctx, snapshot)
			return nil
		}

		terminal := *attempt
		terminal.Status = input.Status
		terminal.FailureReason = strings.TrimSpace(input.FailureReason)
		switch input.Status {
		case protocol.WorkAttemptStatusSucceeded:
			terminal.FailureReason = ""
		case protocol.WorkAttemptStatusInterrupted:
			if terminal.FailureReason == "" {
				terminal.FailureReason = "Room runtime interrupted"
			}
		case protocol.WorkAttemptStatusFailed:
			if terminal.FailureReason == "" {
				terminal.FailureReason = "Room runtime failed"
			}
		}
		terminal.RuntimeSessionKey = firstNonEmpty(
			strings.TrimSpace(input.RuntimeSessionKey),
			terminal.RuntimeSessionKey,
		)
		terminal.RoomSessionID = firstNonEmpty(
			strings.TrimSpace(input.RoomSessionID),
			terminal.RoomSessionID,
		)
		terminal.SDKSessionID = firstNonEmpty(
			strings.TrimSpace(input.SDKSessionID),
			terminal.SDKSessionID,
		)
		terminal.RuntimeRoundID = firstNonEmpty(
			strings.TrimSpace(actor.RuntimeRoundID),
			terminal.RuntimeRoundID,
		)
		terminal.RootRoundID = firstNonEmpty(
			strings.TrimSpace(actor.RootRoundID),
			terminal.RootRoundID,
		)
		terminal.AgentRoundID = firstNonEmpty(
			strings.TrimSpace(actor.AgentRoundID),
			terminal.AgentRoundID,
		)
		updated, finishErr := s.repository.FinishAttempt(
			ctx,
			orchestrationstore.FinishAttemptCommand{
				ExpectedExecutionVersion: snapshot.Execution.Version,
				ExpectedAttemptVersion:   attempt.Version,
				Attempt:                  terminal,
				Meta: s.commandMeta(
					actor,
					"room-attempt-terminal:"+
						firstNonEmpty(binding.DispatchID, binding.AttemptID)+":"+
						string(input.Status),
					"room-attempt-terminal",
				),
			},
		)
		if errors.Is(finishErr, orchestrationstore.ErrVersionConflict) ||
			errors.Is(finishErr, orchestrationstore.ErrInvariant) {
			continue
		}
		if finishErr == nil {
			s.invalidateSnapshot(ctx, updated)
		}
		return finishErr
	}
	return domainError(
		ErrorCodeStaleExecution,
		"Room root Attempt state changed concurrently; retry terminal settlement",
	)
}

func authorizeRoomAttemptTerminalBinding(
	snapshot *protocol.ExecutionSnapshot,
	agentID string,
	binding protocol.ExecutionWorkBinding,
) (*protocol.WorkAttempt, error) {
	agentID = strings.TrimSpace(agentID)
	if snapshot == nil || snapshot.Plan == nil ||
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		snapshot.Execution.ID != binding.ExecutionID ||
		snapshot.Plan.ID != binding.PlanID {
		return nil, workBindingMismatch("Room root Attempt is outside its bound Execution Plan")
	}
	assignment := findAssignmentByID(snapshot, binding.AssignmentID)
	if assignment == nil ||
		assignment.ExecutionID != binding.ExecutionID ||
		assignment.PlanID != binding.PlanID ||
		assignment.WorkItemID != binding.WorkItemID ||
		assignment.SpecID != binding.SpecID ||
		strings.TrimSpace(assignment.OwnerAgentID) != agentID {
		return nil, workBindingMismatch("Room root Attempt Assignment binding is stale")
	}
	if assignment.Strategy == protocol.AssignmentStrategySelf {
		if binding.DispatchID != "" {
			return nil, workBindingMismatch("self Room root Attempt must not carry a Dispatch")
		}
	} else {
		if binding.DispatchID == "" {
			return nil, workBindingMismatch("dispatched Room root Attempt is missing its Dispatch")
		}
		dispatchMatched := false
		for _, dispatch := range snapshot.Dispatches {
			if dispatch.ID == binding.DispatchID &&
				dispatch.ExecutionID == binding.ExecutionID &&
				dispatch.PlanID == binding.PlanID &&
				dispatch.WorkItemID == binding.WorkItemID &&
				dispatch.SpecID == binding.SpecID &&
				dispatch.AssignmentID == binding.AssignmentID &&
				strings.TrimSpace(dispatch.TargetAgentID) == agentID {
				dispatchMatched = true
				break
			}
		}
		if !dispatchMatched {
			return nil, workBindingMismatch("Room root Attempt Dispatch binding is stale")
		}
	}
	attempt := findAttemptByID(snapshot, binding.AttemptID)
	if attempt == nil ||
		attempt.ExecutionID != binding.ExecutionID ||
		attempt.PlanID != binding.PlanID ||
		attempt.WorkItemID != binding.WorkItemID ||
		attempt.SpecID != binding.SpecID ||
		attempt.AssignmentID != binding.AssignmentID ||
		attempt.DispatchID != binding.DispatchID ||
		attempt.ParentAttemptID != "" ||
		attempt.ExecutorKind != protocol.AttemptExecutorAgent ||
		strings.TrimSpace(attempt.ExecutorAgentID) != agentID {
		return nil, workBindingMismatch("Room root Attempt binding is stale")
	}
	return attempt, nil
}
