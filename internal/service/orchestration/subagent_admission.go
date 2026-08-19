// INPUT: 当前 actor 的权威 Execution snapshot 与 SDK Agent hook identity。
// OUTPUT: 可选的 tool_use_id child Attempt 绑定、runtime-only 放行、生命周期校验、durable parent-exit deadline、coordinator 唤醒、终态持久化与对应 session 失效事实。
// POS: 原生 Agent 工具的运行准入与可选 managed WorkGraph 记账边界；策略选择不由状态机强制。
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
	subagentAdmissionMutationAttempts = 3
	subagentAdmissionRetryDelay       = 10 * time.Millisecond
)

// SubagentLaunchInput 是 PreToolUse 提供的宿主可信执行 identity。
//
// ToolUseID 是 launch binding；原生 Agent tool schema 不承载 Nexus token，
// 因此不能从模型输入读取或回填业务 ID。
type SubagentLaunchInput struct {
	ToolUseID         string
	RuntimeSessionKey string
	RoomSessionID     string
	SDKSessionID      string
}

// SubagentLifecycleInput 是 SubagentStart/Stop 或 Agent tool failure 的可用 SDK 证据。
type SubagentLifecycleInput struct {
	ToolUseID            string
	SDKSessionID         string
	SDKTaskID            string
	SDKAgentID           string
	ChildSessionID       string
	AgentType            string
	AgentTranscriptPath  string
	LastAssistantMessage string
	Interrupted          bool
	Error                string
}

// SubagentParentRoundExitInput 是 runtime manager 观察到的 parent physical
// round exit；deadline 由 runtime policy 计算，模型不能提供。
type SubagentParentRoundExitInput struct {
	ToolUseID           string
	SDKSessionID        string
	SDKAgentID          string
	SDKTaskID           string
	ParentRoundExitedAt time.Time
	ReconcileAfter      time.Time
}

type subagentReconciliationRepository interface {
	ScheduleSubagentReconciliation(
		context.Context,
		orchestrationstore.ScheduleSubagentReconciliationCommand,
	) (*protocol.ExecutionSnapshot, error)
	ListExpiredSubagentAttempts(
		context.Context,
		time.Time,
		int,
	) ([]protocol.WorkAttempt, error)
	ListOrphanedSubagentAttempts(
		context.Context,
		time.Time,
		int,
	) ([]protocol.WorkAttempt, error)
}

// SubagentReconciliationResult 汇总 durable parent-exit deadline 的恢复结果。
type SubagentReconciliationResult struct {
	Scanned    int
	Reconciled int
	Deferred   int
}

// SubagentAttemptBinding 是后端生成、不会交给模型修改的 launch identity。
type SubagentAttemptBinding struct {
	ExecutionID     string `json:"execution_id"`
	PlanID          string `json:"plan_id"`
	WorkItemID      string `json:"work_item_id"`
	SpecID          string `json:"spec_id"`
	AssignmentID    string `json:"assignment_id"`
	ParentAttemptID string `json:"parent_attempt_id"`
	AttemptID       string `json:"attempt_id"`
	ToolUseID       string `json:"tool_use_id"`
}

// SubagentAdmissionMode 区分受管 WorkGraph 记账与仅由 Bridge 观测的本地执行。
type SubagentAdmissionMode string

const (
	SubagentAdmissionManaged     SubagentAdmissionMode = "managed"
	SubagentAdmissionRuntimeOnly SubagentAdmissionMode = "runtime_only"
)

// SubagentAdmissionResult 为 Hook 提供稳定、结构化的 allow/deny 原因。
type SubagentAdmissionResult struct {
	Allowed    bool                    `json:"allowed"`
	Mode       SubagentAdmissionMode   `json:"mode,omitempty"`
	ReasonCode ErrorCode               `json:"reason_code,omitempty"`
	Message    string                  `json:"message,omitempty"`
	Binding    *SubagentAttemptBinding `json:"binding,omitempty"`
}

// AdmitSubagentLaunch 在 Agent tool 真正执行前按唯一 tool_use_id 预留 child Attempt。
// 能精确命中 current Assignment 时写入 managed binding；否则允许 runtime-only
// 执行，由 Bridge 记录运行图，但不把它伪装成共享 WorkGraph 交付证据。
// 同一 parent 可以并行多个 child；重复 managed launch 幂等复用原 binding。
func (s *Service) AdmitSubagentLaunch(
	ctx context.Context,
	actor ActorContext,
	input SubagentLaunchInput,
) (SubagentAdmissionResult, error) {
	if err := validateActor(actor); err != nil {
		return rejectedSubagentAdmission(err), nil
	}
	if actor.PlanMode {
		return rejectedSubagentAdmission(planModeError()), nil
	}
	toolUseID := strings.TrimSpace(input.ToolUseID)
	for attempt := range subagentAdmissionMutationAttempts {
		snapshot, err := s.optionalSubagentSnapshot(ctx, actor)
		if err != nil {
			var domainErr *DomainError
			if errors.As(err, &domainErr) {
				return rejectedSubagentAdmission(err), nil
			}
			if orchestrationstore.IsTransientMutationError(err) {
				if attempt+1 < subagentAdmissionMutationAttempts {
					if waitErr := waitForSubagentAdmissionRetry(ctx, attempt); waitErr != nil {
						return SubagentAdmissionResult{}, waitErr
					}
				}
				continue
			}
			return SubagentAdmissionResult{}, err
		}
		if snapshot == nil {
			return allowedRuntimeOnlySubagentAdmission(), nil
		}
		if toolUseID != "" {
			if existing := activeSubagentAttempt(snapshot, actor.AgentID, toolUseID); existing != nil {
				s.invalidateSnapshot(ctx, snapshot)
				return allowedSubagentAdmission(bindingFromAttempt(*existing)), nil
			}
		}
		assignment, parent, resolveErr := resolveSubagentLaunchCandidate(snapshot, actor.AgentID)
		if resolveErr != nil || toolUseID == "" {
			// Missing or ambiguous responsibility only means this launch cannot be
			// counted as a managed child Attempt. It is not a reason to prevent the
			// Agent from using its native local delegation capability.
			return allowedRuntimeOnlySubagentAdmission(), nil
		}
		if parent.Status == protocol.WorkAttemptStatusPending {
			updated, startErr := s.repository.StartAttempt(ctx, orchestrationstore.StartAttemptCommand{
				ExpectedExecutionVersion:  snapshot.Execution.Version,
				ExpectedAssignmentVersion: assignment.Version,
				ExpectedAttemptVersion:    parent.Version,
				Attempt:                   mergeSubagentRuntime(*parent, actor, input),
				Meta: s.commandMeta(
					actor,
					"subagent-launch:"+toolUseID,
					"parent-attempt-start",
				),
			})
			if subagentAdmissionRetryable(startErr) {
				if attempt+1 < subagentAdmissionMutationAttempts {
					if waitErr := waitForSubagentAdmissionRetry(ctx, attempt); waitErr != nil {
						return SubagentAdmissionResult{}, waitErr
					}
				}
				continue
			}
			if startErr != nil {
				return SubagentAdmissionResult{}, startErr
			}
			s.invalidateSnapshot(ctx, updated)
			snapshot = updated
			assignment = findAssignmentByID(snapshot, assignment.ID)
			parent = findAttemptByID(snapshot, parent.ID)
			if assignment == nil || parent == nil {
				return SubagentAdmissionResult{}, errors.New(
					"repository returned an incomplete subagent parent binding",
				)
			}
		}
		child := protocol.WorkAttempt{
			ID:              s.id("attempt"),
			ExecutionID:     assignment.ExecutionID,
			PlanID:          assignment.PlanID,
			WorkItemID:      assignment.WorkItemID,
			SpecID:          assignment.SpecID,
			AssignmentID:    assignment.ID,
			ParentAttemptID: parent.ID,
			ExecutorKind:    protocol.AttemptExecutorSubagent,
			ParentAgentID:   strings.TrimSpace(actor.AgentID),
			RuntimeSessionKey: firstNonEmpty(
				strings.TrimSpace(input.RuntimeSessionKey),
				strings.TrimSpace(actor.SessionKey),
			),
			RoomSessionID:  strings.TrimSpace(input.RoomSessionID),
			SDKSessionID:   strings.TrimSpace(input.SDKSessionID),
			RuntimeRoundID: strings.TrimSpace(actor.RuntimeRoundID),
			RootRoundID:    strings.TrimSpace(actor.RootRoundID),
			AgentRoundID:   strings.TrimSpace(actor.AgentRoundID),
			ToolUseID:      toolUseID,
			Status:         protocol.WorkAttemptStatusRunning,
			Metadata: map[string]any{
				"binding_kind": "pre_tool_use",
			},
		}
		updated, startErr := s.repository.StartAttempt(ctx, orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    0,
			Attempt:                   child,
			Meta: s.commandMeta(
				actor,
				"subagent-launch:"+toolUseID,
				"child-attempt-start",
			),
		})
		if subagentAdmissionRetryable(startErr) {
			if attempt+1 < subagentAdmissionMutationAttempts {
				if waitErr := waitForSubagentAdmissionRetry(ctx, attempt); waitErr != nil {
					return SubagentAdmissionResult{}, waitErr
				}
			}
			continue
		}
		if startErr != nil {
			return SubagentAdmissionResult{}, startErr
		}
		s.invalidateSnapshot(ctx, updated)
		return allowedSubagentAdmission(bindingForAttempt(updated, child.ID)), nil
	}
	snapshot, err := s.optionalSubagentSnapshot(ctx, actor)
	if err != nil {
		return SubagentAdmissionResult{}, err
	}
	if snapshot == nil {
		return allowedRuntimeOnlySubagentAdmission(), nil
	}
	if child := activeSubagentAttempt(snapshot, actor.AgentID, toolUseID); child != nil {
		s.invalidateSnapshot(ctx, snapshot)
		return allowedSubagentAdmission(bindingFromAttempt(*child)), nil
	}
	return rejectedSubagentAdmission(domainError(
		ErrorCodeStaleExecution,
		"execution state changed concurrently; reload before launching a subagent",
	)), nil
}

func subagentAdmissionRetryable(err error) bool {
	return orchestrationstore.IsTransientMutationError(err) ||
		errors.Is(err, orchestrationstore.ErrInvariant)
}

func waitForSubagentAdmissionRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(subagentAdmissionRetryDelay * time.Duration(attempt+1))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ObserveSubagentStart 在可精确命中时返回 managed child Attempt；无法命中时
// 保留 runtime-only 观测，不按最新时间猜测，也不阻断已经开始的原生执行。
func (s *Service) ObserveSubagentStart(
	ctx context.Context,
	actor ActorContext,
	input SubagentLifecycleInput,
) (SubagentAdmissionResult, error) {
	if err := validateActor(actor); err != nil {
		return rejectedSubagentAdmission(err), nil
	}
	if actor.PlanMode {
		return rejectedSubagentAdmission(planModeError()), nil
	}
	snapshot, err := s.optionalSubagentSnapshot(ctx, actor)
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) {
			return rejectedSubagentAdmission(err), nil
		}
		return SubagentAdmissionResult{}, err
	}
	if snapshot == nil {
		return allowedRuntimeOnlySubagentAdmission(), nil
	}
	child, resolveErr := resolveActiveSubagentBinding(snapshot, actor.AgentID, input.ToolUseID)
	if resolveErr != nil {
		return allowedRuntimeOnlySubagentAdmission(), nil
	}
	s.invalidateSnapshot(ctx, snapshot)
	return allowedSubagentAdmission(bindingFromAttempt(*child)), nil
}

// ObserveSubagentStop 终结精确 child Attempt；它不创建 Submission 或 Acceptance。
func (s *Service) ObserveSubagentStop(
	ctx context.Context,
	actor ActorContext,
	input SubagentLifecycleInput,
) (SubagentAdmissionResult, error) {
	if err := validateActor(actor); err != nil {
		return rejectedSubagentAdmission(err), nil
	}
	if actor.PlanMode {
		return rejectedSubagentAdmission(planModeError()), nil
	}
	for range subagentAdmissionMutationAttempts {
		snapshot, err := s.optionalSubagentSnapshot(ctx, actor)
		if err != nil {
			var domainErr *DomainError
			if errors.As(err, &domainErr) {
				return rejectedSubagentAdmission(err), nil
			}
			return SubagentAdmissionResult{}, err
		}
		if snapshot == nil {
			return allowedRuntimeOnlySubagentAdmission(), nil
		}
		child, resolveErr := resolveActiveSubagentBinding(snapshot, actor.AgentID, input.ToolUseID)
		if resolveErr != nil {
			if stopped := matchingTerminalSubagentAttempt(snapshot, actor.AgentID, input); stopped != nil {
				s.invalidateSnapshot(ctx, snapshot)
				return allowedSubagentAdmission(bindingFromAttempt(*stopped)), nil
			}
			return allowedRuntimeOnlySubagentAdmission(), nil
		}
		terminal := *child
		terminal.SDKSessionID = firstNonEmpty(
			strings.TrimSpace(input.SDKSessionID),
			terminal.SDKSessionID,
		)
		terminal.SDKTaskID = firstNonEmpty(
			strings.TrimSpace(input.SDKTaskID),
			terminal.SDKTaskID,
		)
		terminal.ChildSessionID = firstNonEmpty(
			strings.TrimSpace(input.ChildSessionID),
			terminal.ChildSessionID,
		)
		terminal.ExecutorAgentID = firstNonEmpty(
			strings.TrimSpace(input.SDKAgentID),
			terminal.ExecutorAgentID,
		)
		terminal.Status = protocol.WorkAttemptStatusSucceeded
		terminal.FailureReason = ""
		errorMessage := strings.TrimSpace(input.Error)
		if input.Interrupted {
			terminal.Status = protocol.WorkAttemptStatusInterrupted
			terminal.FailureReason = firstNonEmpty(errorMessage, "subagent interrupted")
		} else if errorMessage != "" {
			terminal.Status = protocol.WorkAttemptStatusFailed
			terminal.FailureReason = errorMessage
		}
		terminal.Metadata = mergeSubagentLifecycleMetadata(terminal.Metadata, input)
		updated, finishErr := s.repository.FinishAttempt(ctx, orchestrationstore.FinishAttemptCommand{
			ExpectedExecutionVersion: snapshot.Execution.Version,
			ExpectedAttemptVersion:   child.Version,
			Attempt:                  terminal,
			Meta: s.commandMeta(
				actorForAttempt(actor, *child),
				"subagent-launch:"+child.ToolUseID,
				"child-attempt-terminal",
			),
		})
		if errors.Is(finishErr, orchestrationstore.ErrVersionConflict) ||
			errors.Is(finishErr, orchestrationstore.ErrInvariant) {
			continue
		}
		if finishErr != nil {
			return SubagentAdmissionResult{}, finishErr
		}
		s.invalidateSnapshot(ctx, updated)
		return allowedSubagentAdmission(bindingForAttempt(updated, child.ID)), nil
	}
	return rejectedSubagentAdmission(domainError(
		ErrorCodeStaleExecution,
		"subagent lifecycle changed concurrently; reload before retrying",
	)), nil
}

// ObserveSubagentParentRoundExit 在 parent round 释放 callback 前持久化 child
// reconciliation deadline。进程内 timer 只优化延迟；SQL deadline 支持重启恢复。
func (s *Service) ObserveSubagentParentRoundExit(
	ctx context.Context,
	actor ActorContext,
	input SubagentParentRoundExitInput,
) (SubagentAdmissionResult, error) {
	if err := validateActor(actor); err != nil {
		return rejectedSubagentAdmission(err), nil
	}
	if actor.PlanMode {
		return rejectedSubagentAdmission(planModeError()), nil
	}
	for range subagentAdmissionMutationAttempts {
		snapshot, err := s.optionalSubagentSnapshot(ctx, actor)
		if err != nil {
			var domainErr *DomainError
			if errors.As(err, &domainErr) {
				return rejectedSubagentAdmission(err), nil
			}
			return SubagentAdmissionResult{}, err
		}
		if snapshot == nil {
			return allowedRuntimeOnlySubagentAdmission(), nil
		}
		lifecycle := SubagentLifecycleInput{
			ToolUseID:    strings.TrimSpace(input.ToolUseID),
			SDKSessionID: strings.TrimSpace(input.SDKSessionID),
			SDKAgentID:   strings.TrimSpace(input.SDKAgentID),
		}
		child, resolveErr := resolveActiveSubagentBinding(
			snapshot,
			actor.AgentID,
			input.ToolUseID,
		)
		if resolveErr != nil {
			if stopped := matchingTerminalSubagentAttempt(
				snapshot,
				actor.AgentID,
				lifecycle,
			); stopped != nil {
				s.invalidateSnapshot(ctx, snapshot)
				return allowedSubagentAdmission(bindingFromAttempt(*stopped)), nil
			}
			return allowedRuntimeOnlySubagentAdmission(), nil
		}
		if !protocol.ValidSubagentReconciliationDeadline(
			input.ParentRoundExitedAt,
			input.ReconcileAfter,
		) {
			return rejectedSubagentAdmission(domainError(
				ErrorCodeInvalidInput,
				"subagent reconciliation deadline must equal parent round exit plus 30 seconds",
			)), nil
		}
		repository, ok := s.repository.(subagentReconciliationRepository)
		if !ok {
			return SubagentAdmissionResult{}, fmt.Errorf(
				"orchestration repository does not support durable subagent reconciliation",
			)
		}
		updated, scheduleErr := repository.ScheduleSubagentReconciliation(
			ctx,
			orchestrationstore.ScheduleSubagentReconciliationCommand{
				ExecutionID:              child.ExecutionID,
				ExpectedExecutionVersion: snapshot.Execution.Version,
				ExpectedAttemptVersion:   child.Version,
				AttemptID:                child.ID,
				ParentRoundExitedAt:      input.ParentRoundExitedAt,
				ReconcileAfter:           input.ReconcileAfter,
				Meta: s.commandMeta(
					actorForAttempt(actor, *child),
					"subagent-launch:"+child.ToolUseID,
					"child-reconciliation-scheduled",
				),
			},
		)
		if errors.Is(scheduleErr, orchestrationstore.ErrVersionConflict) ||
			errors.Is(scheduleErr, orchestrationstore.ErrInvariant) {
			continue
		}
		if scheduleErr != nil {
			return SubagentAdmissionResult{}, scheduleErr
		}
		s.invalidateSnapshot(ctx, updated)
		s.WakeSubagentReconciliation()
		return allowedSubagentAdmission(bindingForAttempt(updated, child.ID)), nil
	}
	return rejectedSubagentAdmission(domainError(
		ErrorCodeStaleExecution,
		"subagent reconciliation schedule changed concurrently; reload before retrying",
	)), nil
}

// ReconcileExpiredSubagents terminalizes running children whose durable grace
// deadline has elapsed. Repeated workers race through Execution/Attempt CAS.
func (s *Service) ReconcileExpiredSubagents(
	ctx context.Context,
	limit int,
) (SubagentReconciliationResult, error) {
	var result SubagentReconciliationResult
	if limit <= 0 {
		return result, fmt.Errorf("positive subagent reconciliation limit is required")
	}
	repository, ok := s.repository.(subagentReconciliationRepository)
	if !ok {
		return result, fmt.Errorf(
			"orchestration repository does not support durable subagent reconciliation",
		)
	}
	expired, err := repository.ListExpiredSubagentAttempts(ctx, s.now().UTC(), limit)
	if err != nil {
		return result, err
	}
	result.Scanned = len(expired)
	for _, candidate := range expired {
		snapshot, getErr := s.repository.GetSnapshot(ctx, candidate.ExecutionID)
		if getErr != nil {
			return result, getErr
		}
		current := findAttemptByID(snapshot, candidate.ID)
		if current == nil ||
			current.Status != protocol.WorkAttemptStatusRunning ||
			current.ReconcileAfter == nil ||
			current.ReconcileAfter.After(s.now().UTC()) {
			continue
		}
		terminal := *current
		terminal.Status = protocol.WorkAttemptStatusInterrupted
		terminal.FailureReason =
			"parent runtime round ended before subagent lifecycle completed"
		updated, finishErr := s.repository.FinishAttempt(
			ctx,
			orchestrationstore.FinishAttemptCommand{
				ExpectedExecutionVersion: snapshot.Execution.Version,
				ExpectedAttemptVersion:   current.Version,
				Attempt:                  terminal,
				Meta: s.commandMeta(
					ActorContext{
						AgentID:        "subagent-reconciler",
						ActorKind:      protocol.ExecutionActorSystem,
						RootRoundID:    current.RootRoundID,
						RuntimeRoundID: current.RuntimeRoundID,
						AgentRoundID:   current.AgentRoundID,
					},
					"subagent-reconcile:"+current.ID,
					"child-attempt-terminal",
				),
			},
		)
		if errors.Is(finishErr, orchestrationstore.ErrVersionConflict) ||
			errors.Is(finishErr, orchestrationstore.ErrInvariant) {
			result.Deferred++
			continue
		}
		if finishErr != nil {
			return result, finishErr
		}
		s.invalidateSnapshot(ctx, updated)
		result.Reconciled++
	}
	return result, nil
}

// ReconcileOrphanedSubagents closes the only child-Attempt class that cannot
// carry a durable deadline: a previous process observed parent exit but died
// before that scheduling transaction committed. The immutable process-start
// cutoff prevents this recovery pass from ever selecting children launched by
// the current server.
func (s *Service) ReconcileOrphanedSubagents(
	ctx context.Context,
	processStartedAt time.Time,
	limit int,
) (SubagentReconciliationResult, error) {
	var result SubagentReconciliationResult
	if processStartedAt.IsZero() || limit <= 0 {
		return result, fmt.Errorf(
			"process start and positive subagent reconciliation limit are required",
		)
	}
	repository, ok := s.repository.(subagentReconciliationRepository)
	if !ok {
		return result, fmt.Errorf(
			"orchestration repository does not support durable subagent reconciliation",
		)
	}
	orphans, err := repository.ListOrphanedSubagentAttempts(
		ctx,
		processStartedAt.UTC(),
		limit,
	)
	if err != nil {
		return result, err
	}
	result.Scanned = len(orphans)
	for _, candidate := range orphans {
		snapshot, getErr := s.repository.GetSnapshot(ctx, candidate.ExecutionID)
		if getErr != nil {
			return result, getErr
		}
		current := findAttemptByID(snapshot, candidate.ID)
		if current == nil ||
			current.Status != protocol.WorkAttemptStatusRunning ||
			current.ExecutorKind != protocol.AttemptExecutorSubagent ||
			strings.TrimSpace(current.ParentAttemptID) == "" ||
			current.ReconcileAfter != nil ||
			!current.CreatedAt.Before(processStartedAt.UTC()) {
			continue
		}
		terminal := *current
		terminal.Status = protocol.WorkAttemptStatusInterrupted
		terminal.FailureReason =
			"server restarted before parent round exit reconciliation could be persisted"
		updated, finishErr := s.repository.FinishAttempt(
			ctx,
			orchestrationstore.FinishAttemptCommand{
				ExpectedExecutionVersion: snapshot.Execution.Version,
				ExpectedAttemptVersion:   current.Version,
				Attempt:                  terminal,
				Meta: s.commandMeta(
					ActorContext{
						AgentID:        "subagent-reconciler",
						ActorKind:      protocol.ExecutionActorSystem,
						RootRoundID:    current.RootRoundID,
						RuntimeRoundID: current.RuntimeRoundID,
						AgentRoundID:   current.AgentRoundID,
					},
					"subagent-orphan-reconcile:"+current.ID,
					"child-attempt-terminal",
				),
			},
		)
		if errors.Is(finishErr, orchestrationstore.ErrVersionConflict) ||
			errors.Is(finishErr, orchestrationstore.ErrInvariant) {
			result.Deferred++
			continue
		}
		if finishErr != nil {
			return result, finishErr
		}
		s.invalidateSnapshot(ctx, updated)
		result.Reconciled++
	}
	return result, nil
}

func resolveSubagentLaunchCandidate(
	snapshot *protocol.ExecutionSnapshot,
	agentID string,
) (*protocol.WorkAssignment, *protocol.WorkAttempt, error) {
	agentID = strings.TrimSpace(agentID)
	if snapshot == nil || snapshot.Plan == nil ||
		snapshot.Plan.Status != protocol.PlanRevisionStatusActive ||
		(snapshot.Execution.Status != protocol.ExecutionStatusActive &&
			snapshot.Execution.Status != protocol.ExecutionStatusWaiting) {
		return nil, nil, domainError(
			ErrorCodeNoDelegableAssignment,
			"the current Execution has no active Plan that permits subagent work",
		)
	}
	candidates := subagentLaunchCandidates(snapshot, agentID)
	switch len(candidates) {
	case 0:
		return nil, nil, domainError(
			ErrorCodeNoDelegableAssignment,
			"the current Agent has no bounded, incomplete Assignment to delegate",
		)
	case 1:
	default:
		return nil, nil, domainError(
			ErrorCodeAmbiguousAssignment,
			"the current Agent has multiple delegable Assignments; select one through the WorkGraph before launching a subagent",
		)
	}
	assignment := candidates[0]
	parents := make([]protocol.WorkAttempt, 0, 2)
	for _, attempt := range snapshot.Attempts {
		if attempt.AssignmentID == assignment.ID &&
			attempt.ParentAttemptID == "" &&
			attempt.ExecutorKind == protocol.AttemptExecutorAgent &&
			attempt.ExecutionID == assignment.ExecutionID &&
			attempt.PlanID == assignment.PlanID &&
			attempt.WorkItemID == assignment.WorkItemID &&
			attempt.SpecID == assignment.SpecID &&
			strings.TrimSpace(attempt.ExecutorAgentID) ==
				strings.TrimSpace(assignment.OwnerAgentID) &&
			(attempt.Status == protocol.WorkAttemptStatusPending ||
				attempt.Status == protocol.WorkAttemptStatusRunning) {
			parents = append(parents, attempt)
		}
	}
	if len(parents) != 1 {
		return nil, nil, domainError(
			ErrorCodeSubagentBindingMissing,
			"the Assignment must have exactly one pending or running parent Attempt",
		)
	}
	return &assignment, &parents[0], nil
}

func subagentLaunchCandidates(
	snapshot *protocol.ExecutionSnapshot,
	agentID string,
) []protocol.WorkAssignment {
	if snapshot == nil || snapshot.Plan == nil ||
		snapshot.Plan.Status != protocol.PlanRevisionStatusActive ||
		(snapshot.Execution.Status != protocol.ExecutionStatusActive &&
			snapshot.Execution.Status != protocol.ExecutionStatusWaiting) {
		return nil
	}
	agentID = strings.TrimSpace(agentID)
	candidates := make([]protocol.WorkAssignment, 0, 2)
	for _, assignment := range snapshot.Assignments {
		if strings.TrimSpace(assignment.OwnerAgentID) != agentID ||
			!currentAssignment(assignment) ||
			!delegableWorkItem(snapshot, assignment) {
			continue
		}
		candidates = append(candidates, assignment)
	}
	return candidates
}

func (s *Service) subagentSnapshot(
	ctx context.Context,
	actor ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	if executionID := strings.TrimSpace(actor.ExecutionID); executionID != "" {
		return s.GetSnapshot(ctx, actor, executionID)
	}
	return s.GetCurrent(ctx, actor)
}

// optionalSubagentSnapshot 把“当前 round 没有可用 managed WorkGraph”解释为
// runtime-only 能力，而不弱化错误 owner、stale binding 等真实授权失败。
func (s *Service) optionalSubagentSnapshot(
	ctx context.Context,
	actor ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	snapshot, err := s.subagentSnapshot(ctx, actor)
	if err == nil {
		return snapshot, nil
	}
	var domainErr *DomainError
	if errors.As(err, &domainErr) &&
		(domainErr.Code == ErrorCodeConversationOnly ||
			domainErr.Code == ErrorCodeNoCurrentExecution) {
		return nil, nil
	}
	return nil, err
}

func delegableWorkItem(
	snapshot *protocol.ExecutionSnapshot,
	assignment protocol.WorkAssignment,
) bool {
	if snapshot == nil || snapshot.Plan == nil ||
		assignment.ExecutionID != snapshot.Execution.ID ||
		assignment.PlanID != snapshot.Plan.ID ||
		strings.TrimSpace(assignment.WorkItemID) == "" ||
		strings.TrimSpace(assignment.SpecID) == "" {
		return false
	}
	itemBound := false
	for _, item := range snapshot.WorkItems {
		if item.ID == assignment.WorkItemID &&
			item.ExecutionID == assignment.ExecutionID {
			itemBound = true
			break
		}
	}
	if !itemBound {
		return false
	}
	planItemBound := false
	for _, item := range snapshot.PlanItems {
		if item.PlanID == assignment.PlanID &&
			item.ExecutionID == assignment.ExecutionID &&
			item.WorkItemID == assignment.WorkItemID &&
			item.SpecID == assignment.SpecID {
			planItemBound = true
			break
		}
	}
	if !planItemBound {
		return false
	}
	var state *protocol.WorkItemState
	for index := range snapshot.WorkItemStates {
		if snapshot.WorkItemStates[index].WorkItemID == assignment.WorkItemID &&
			snapshot.WorkItemStates[index].ExecutionID == assignment.ExecutionID {
			state = &snapshot.WorkItemStates[index]
			break
		}
	}
	if state == nil ||
		state.Status != protocol.WorkItemStatusOpen ||
		state.CurrentSpecID != assignment.SpecID {
		return false
	}
	for _, acceptance := range snapshot.Acceptances {
		if acceptance.WorkItemID == assignment.WorkItemID &&
			acceptance.SpecID == assignment.SpecID &&
			acceptance.Decision == protocol.WorkAcceptanceAccepted {
			return false
		}
	}
	for _, submission := range snapshot.Submissions {
		if submission.AssignmentID != assignment.ID ||
			submission.WorkItemID != assignment.WorkItemID ||
			submission.SpecID != assignment.SpecID {
			continue
		}
		reviewed := false
		for _, acceptance := range snapshot.Acceptances {
			if acceptance.SubmissionID == submission.ID {
				reviewed = true
				break
			}
		}
		if !reviewed {
			return false
		}
	}
	for _, spec := range snapshot.WorkItemSpecs {
		if spec.ID != assignment.SpecID ||
			spec.WorkItemID != assignment.WorkItemID ||
			spec.ExecutionID != assignment.ExecutionID {
			continue
		}
		return strings.TrimSpace(spec.Objective) != "" &&
			strings.TrimSpace(spec.Deliverable) != ""
	}
	return false
}

func resolveActiveSubagentBinding(
	snapshot *protocol.ExecutionSnapshot,
	agentID string,
	toolUseID string,
) (*protocol.WorkAttempt, error) {
	matches := make([]protocol.WorkAttempt, 0, 2)
	agentID = strings.TrimSpace(agentID)
	toolUseID = strings.TrimSpace(toolUseID)
	for _, attempt := range snapshot.Attempts {
		if attempt.ExecutorKind != protocol.AttemptExecutorSubagent ||
			attempt.ParentAgentID != agentID ||
			(attempt.Status != protocol.WorkAttemptStatusPending &&
				attempt.Status != protocol.WorkAttemptStatusRunning) {
			continue
		}
		if toolUseID != "" && attempt.ToolUseID != toolUseID {
			continue
		}
		matches = append(matches, attempt)
	}
	if len(matches) != 1 {
		return nil, domainError(
			ErrorCodeSubagentBindingMissing,
			"the hook event does not match exactly one active subagent Attempt",
		)
	}
	return &matches[0], nil
}

func activeSubagentAttempt(
	snapshot *protocol.ExecutionSnapshot,
	agentID string,
	toolUseID string,
) *protocol.WorkAttempt {
	attempt, _ := resolveActiveSubagentBinding(snapshot, agentID, toolUseID)
	return attempt
}

func mergeSubagentRuntime(
	attempt protocol.WorkAttempt,
	actor ActorContext,
	input SubagentLaunchInput,
) protocol.WorkAttempt {
	attempt.RuntimeSessionKey = firstNonEmpty(
		strings.TrimSpace(input.RuntimeSessionKey),
		strings.TrimSpace(actor.SessionKey),
	)
	attempt.RoomSessionID = strings.TrimSpace(input.RoomSessionID)
	attempt.SDKSessionID = strings.TrimSpace(input.SDKSessionID)
	attempt.RuntimeRoundID = strings.TrimSpace(actor.RuntimeRoundID)
	attempt.RootRoundID = strings.TrimSpace(actor.RootRoundID)
	attempt.AgentRoundID = strings.TrimSpace(actor.AgentRoundID)
	return attempt
}

func mergeSubagentLifecycleMetadata(
	current map[string]any,
	input SubagentLifecycleInput,
) map[string]any {
	result := cloneMap(current)
	if result == nil {
		result = make(map[string]any)
	}
	if value := strings.TrimSpace(input.SDKAgentID); value != "" {
		result["sdk_agent_id"] = value
	}
	if value := strings.TrimSpace(input.AgentType); value != "" {
		result["sdk_agent_type"] = value
	}
	if value := strings.TrimSpace(input.AgentTranscriptPath); value != "" {
		result["agent_transcript_path"] = value
	}
	if strings.TrimSpace(input.LastAssistantMessage) != "" {
		result["has_last_assistant_message"] = true
	}
	result["binding_kind"] = "sdk_hook"
	return result
}

func matchingTerminalSubagentAttempt(
	snapshot *protocol.ExecutionSnapshot,
	agentID string,
	input SubagentLifecycleInput,
) *protocol.WorkAttempt {
	agentID = strings.TrimSpace(agentID)
	toolUseID := strings.TrimSpace(input.ToolUseID)
	sdkAgentID := strings.TrimSpace(input.SDKAgentID)
	for index := len(snapshot.Attempts) - 1; index >= 0; index-- {
		attempt := snapshot.Attempts[index]
		if attempt.ExecutorKind != protocol.AttemptExecutorSubagent ||
			attempt.ParentAgentID != agentID ||
			attempt.Status == protocol.WorkAttemptStatusPending ||
			attempt.Status == protocol.WorkAttemptStatusRunning {
			continue
		}
		if toolUseID != "" && attempt.ToolUseID == toolUseID {
			return &attempt
		}
		if sdkAgentID != "" &&
			strings.TrimSpace(stringMetadata(attempt.Metadata, "sdk_agent_id")) == sdkAgentID {
			return &attempt
		}
	}
	return nil
}

func actorForAttempt(actor ActorContext, attempt protocol.WorkAttempt) ActorContext {
	result := actor
	result.RootRoundID = firstNonEmpty(attempt.RootRoundID, actor.RootRoundID)
	result.RuntimeRoundID = firstNonEmpty(attempt.RuntimeRoundID, actor.RuntimeRoundID)
	result.AgentRoundID = firstNonEmpty(attempt.AgentRoundID, actor.AgentRoundID)
	return result
}

func bindingForAttempt(
	snapshot *protocol.ExecutionSnapshot,
	attemptID string,
) *SubagentAttemptBinding {
	if snapshot == nil {
		return nil
	}
	attempt := findAttemptByID(snapshot, attemptID)
	if attempt == nil {
		return nil
	}
	return bindingFromAttempt(*attempt)
}

func bindingFromAttempt(attempt protocol.WorkAttempt) *SubagentAttemptBinding {
	return &SubagentAttemptBinding{
		ExecutionID:     attempt.ExecutionID,
		PlanID:          attempt.PlanID,
		WorkItemID:      attempt.WorkItemID,
		SpecID:          attempt.SpecID,
		AssignmentID:    attempt.AssignmentID,
		ParentAttemptID: attempt.ParentAttemptID,
		AttemptID:       attempt.ID,
		ToolUseID:       attempt.ToolUseID,
	}
}

func allowedSubagentAdmission(binding *SubagentAttemptBinding) SubagentAdmissionResult {
	if binding == nil {
		return rejectedSubagentAdmission(domainError(
			ErrorCodeSubagentBindingMissing,
			"managed subagent state did not return its durable Attempt binding",
		))
	}
	return SubagentAdmissionResult{
		Allowed: true,
		Mode:    SubagentAdmissionManaged,
		Binding: binding,
	}
}

func allowedRuntimeOnlySubagentAdmission() SubagentAdmissionResult {
	return SubagentAdmissionResult{
		Allowed: true,
		Mode:    SubagentAdmissionRuntimeOnly,
		Message: "native subagent execution is allowed without managed WorkGraph evidence",
	}
}

func rejectedSubagentAdmission(err error) SubagentAdmissionResult {
	result := SubagentAdmissionResult{Allowed: false}
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		result.ReasonCode = domainErr.Code
		result.Message = domainErr.Message
		return result
	}
	result.ReasonCode = ErrorCodeInvalidInput
	if err != nil {
		result.Message = strings.TrimSpace(err.Error())
	}
	return result
}

func stringMetadata(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return value
}
