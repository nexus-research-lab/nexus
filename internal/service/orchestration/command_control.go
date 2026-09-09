// INPUT: 阻塞、恢复与完成控制。
// OUTPUT: 原子命令结果与后续责任动作。
// POS: Orchestration 命令的业务阶段，复用同包授权、版本栅栏与结果投影。
package orchestration

import (
	"context"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// AbandonExecutionInput 明确取消一个 transient Execution，不创建 successor。
type AbandonExecutionInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	Reason           string
}

// BlockWorkInput 只记录确定的外部输入阻塞，不复制依赖阻塞。
type BlockWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	WorkItemID       string
	LogicalKey       string
	Reason           string
	NeededInput      string
}

// ResumeWorkInput 以 resolution/evidence 关闭显式 waiting_input，并重新开放当前 spec。
type ResumeWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	WorkItemID       string
	LogicalKey       string
	Resolution       string
	Evidence         []string
}

// CompleteExecutionInput 触发一次权威 completion audit。
type CompleteExecutionInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
}

// BlockWork 标记一个 Work Item 正等待确定的外部输入。
func (s *Service) BlockWork(
	ctx context.Context,
	actor ActorContext,
	input BlockWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	input.CommandID = strings.TrimSpace(input.CommandID)
	input.Reason = strings.TrimSpace(input.Reason)
	input.NeededInput = strings.TrimSpace(input.NeededInput)
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		false,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if input.CommandID == "" || input.Reason == "" || input.NeededInput == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id, reason and needed_input are required",
		), nil), nil
	}
	input.WorkItemID, input.LogicalKey, err = bindWorkReferenceFromTrustedResponsibility(
		actor,
		snapshot,
		input.WorkItemID,
		input.LogicalKey,
	)
	if err != nil {
		return RejectedResult(snapshot, err, nextActions(snapshot, actor)), nil
	}
	work, spec, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	assignment := activeAssignmentForWork(snapshot, work.ID)
	actorAgentID := strings.TrimSpace(actor.AgentID)
	isCoordinator := snapshot.Execution.CoordinatorAgentID == actorAgentID
	if !isCoordinator && (assignment == nil || assignment.OwnerAgentID != actorAgentID) {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeWrongOwner,
			"only the current Assignment owner or coordinator may block work",
			work.LogicalKey,
			"",
		), nil), nil
	}
	if submission := latestUnreviewedSubmissionForSpec(snapshot, work.ID, spec.ID); submission != nil {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeCompletionBlocked,
			"review the pending Submission before blocking this Work Item",
			work.LogicalKey,
			submission.ID,
		), nextActions(snapshot, actor)), nil
	}
	state := findStateByWorkID(snapshot, work.ID)
	if state == nil {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"Work Item state is missing",
		), nil), nil
	}
	if state.Status == protocol.WorkItemStatusWaitingInput &&
		state.BlockReason == input.Reason &&
		state.NeededInput == input.NeededInput {
		return NoOpResult(snapshot, "work is already blocked on the same input"), nil
	}
	updated, blockErr := s.repository.Block(ctx, orchestrationstore.BlockCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedStateVersion:     state.Version,
		State: protocol.WorkItemState{
			WorkItemID:    work.ID,
			ExecutionID:   snapshot.Execution.ID,
			CurrentSpecID: spec.ID,
			Status:        protocol.WorkItemStatusWaitingInput,
			BlockReason:   input.Reason,
			NeededInput:   input.NeededInput,
			Metadata:      cloneMap(state.Metadata),
		},
		Meta: s.commandMeta(actor, input.CommandID, "block"),
	})
	if blockErr != nil {
		return s.storageMutationResult(snapshot, blockErr, nextActions(snapshot, actor))
	}
	return AppliedResult(updated, []string{"work_item_state:" + work.ID}, nextActions(updated, actor)), nil
}

// ResumeWork 用 resolution/evidence 关闭 waiting_input；旧 Attempt 不会被复活。
func (s *Service) ResumeWork(
	ctx context.Context,
	actor ActorContext,
	input ResumeWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		false,
		false,
	)
	if err != nil {
		return MutationResult{}, err
	}
	if rejected != nil && rejected.ReasonCode != ErrorCodeStaleExecution {
		return *rejected, nil
	}
	if limitErr := newProjectionLimitError("resume_evidence", len(input.Evidence), ""); limitErr != nil {
		return RejectedResult(snapshot, limitErr, nil), nil
	}
	evidence := normalizeNonEmptyValues(input.Evidence)
	if strings.TrimSpace(input.CommandID) == "" ||
		strings.TrimSpace(input.Resolution) == "" ||
		len(evidence) == 0 {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id, resolution and at least one evidence item are required",
		), nil), nil
	}
	input.WorkItemID, input.LogicalKey, err = bindWorkReferenceFromTrustedResponsibility(
		actor,
		snapshot,
		input.WorkItemID,
		input.LogicalKey,
	)
	if err != nil {
		return RejectedResult(snapshot, err, nextActions(snapshot, actor)), nil
	}
	work, spec, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	state := findStateByWorkID(snapshot, work.ID)
	if state == nil || state.CurrentSpecID != spec.ID {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"current Work Item state/spec fence is missing",
		), nil), nil
	}
	assignment := latestAssignmentForCurrentSpec(snapshot, work.ID, spec.ID)
	isCoordinator := snapshot.Execution.CoordinatorAgentID == strings.TrimSpace(actor.AgentID)
	if !isCoordinator && (assignment == nil || assignment.OwnerAgentID != strings.TrimSpace(actor.AgentID)) {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeWrongOwner,
			"only the latest current-spec Assignment owner or coordinator may resume work",
			work.LogicalKey,
			"",
		), nil), nil
	}
	if state.Status == protocol.WorkItemStatusOpen {
		return NoOpResult(snapshot, "work is already open"), nil
	}
	if rejected != nil {
		return *rejected, nil
	}
	if state.Status != protocol.WorkItemStatusWaitingInput {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeCompletionBlocked,
			"only waiting_input work can be resumed",
			work.LogicalKey,
			string(state.Status),
		), nil), nil
	}
	resolution := strings.TrimSpace(input.Resolution)
	metadata := cloneMap(state.Metadata)
	if metadata == nil {
		metadata = make(map[string]any, 2)
	}
	metadata["last_resume_resolution"] = resolution
	metadata["last_resume_evidence"] = slices.Clone(evidence)
	updated, resumeErr := s.repository.Resume(ctx, orchestrationstore.ResumeCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedStateVersion:     state.Version,
		State: protocol.WorkItemState{
			WorkItemID:    work.ID,
			ExecutionID:   snapshot.Execution.ID,
			CurrentSpecID: spec.ID,
			Status:        protocol.WorkItemStatusOpen,
			Metadata:      metadata,
		},
		Resolution: resolution,
		Evidence:   evidence,
		Meta:       s.commandMeta(actor, input.CommandID, "resume"),
	})
	if resumeErr != nil {
		return s.storageMutationResult(snapshot, resumeErr, nextActions(snapshot, actor))
	}
	return AppliedResult(updated, []string{"work_item_state:" + work.ID}, nextActions(updated, actor)), nil
}

// CompleteIfReady 完成 Execution；任何 blocker 都返回结构化 completion_blocked。
func (s *Service) CompleteIfReady(
	ctx context.Context,
	actor ActorContext,
	input CompleteExecutionInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		true,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "command_id is required"), nil), nil
	}
	if snapshot.Execution.Status == protocol.ExecutionStatusCompleted {
		return NoOpResult(snapshot, "execution is already completed"), nil
	}
	if len(snapshot.CompletionBlockers) > 0 {
		return RejectedResult(snapshot, domainError(
			ErrorCodeCompletionBlocked,
			"execution still has completion blockers: "+strings.Join(snapshot.CompletionBlockers, ", "),
		), nextActions(snapshot, actor)), nil
	}
	updated, completeErr := s.repository.Complete(ctx, orchestrationstore.CompleteCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Meta:                     s.commandMeta(actor, input.CommandID, "complete"),
	})
	if completeErr != nil {
		return s.storageMutationResult(snapshot, completeErr, nextActions(snapshot, actor))
	}
	return AppliedResult(updated, []string{"execution:" + updated.Execution.ID}, nil), nil
}
