// INPUT: 当前 Execution 的权威 objective/criteria、Agent 的结构化审计报告与 runtime identity。
// OUTPUT: 经共享契约校验的可选 Gate NodeRun、未对齐时返回 Agent 的观测回边，以及 durable 写后的 session 失效事实。
// POS: Agent 自主执行策略与 Execution Runtime Graph 之间的语义观测适配；不完成、不重跑、不路由。
package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/objectivealignment"
)

// AuditExecutionAlignmentInput 只携带 Agent 的判定；目标边界由当前
// Execution snapshot 提供，runtime 与 command identity 由宿主注入。
type AuditExecutionAlignmentInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	Report           protocol.ObjectiveAlignmentReport
}

// AuditExecutionAlignment 把一次目标对齐检查记录为可见 Gate。它不改变
// Execution/Plan/Work Item 生命周期，也不根据 decision 替 Agent 选择下一步。
func (s *Service) AuditExecutionAlignment(
	ctx context.Context,
	actor ActorContext,
	input AuditExecutionAlignmentInput,
) (returned MutationResult, returnedErr error) {
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
	commandID := strings.TrimSpace(input.CommandID)
	if commandID == "" {
		return RejectedResult(
			snapshot,
			domainError(ErrorCodeInvalidInput, "command_id is required"),
			nextActions(snapshot, actor),
		), nil
	}
	report, auditErr := objectivealignment.Audit(objectivealignment.Target{
		Objective: snapshot.Execution.Objective,
		Criteria:  snapshot.Execution.CompletionCriteria,
	}, input.Report)
	if auditErr != nil {
		return RejectedResult(snapshot, auditErr, nextActions(snapshot, actor)), nil
	}
	repository, ok := s.repository.(runtimeGraphRepository)
	if !ok || repository == nil {
		return MutationResult{}, fmt.Errorf("runtime graph repository is unavailable")
	}

	actor.ExecutionID = snapshot.Execution.ID
	if actor.RootRoundID == "" {
		actor.RootRoundID = snapshot.Execution.RootRoundID
	}
	identity, err := runtimeGraphIdentityFromActor(actor)
	if err != nil {
		return MutationResult{}, err
	}
	// Gate node and edges are separate durable writes; expose a committed prefix
	// even if a later edge fails.
	defer s.invalidateActor(ctx, actor)
	if err = s.beginRuntimeRound(ctx, actor, false); err != nil {
		return MutationResult{}, err
	}
	now := s.now().UTC()
	rootNodeID := runtimeGraphNodeID(
		identity,
		protocol.ExecutionRuntimeNodeAgent,
		identity.AgentRoundID,
	)
	gateNodeID := runtimeGraphNodeID(
		identity,
		protocol.ExecutionRuntimeNodeGate,
		commandID,
	)
	if err = repository.UpsertRuntimeGraphNode(ctx, protocol.ExecutionRuntimeNodeRun{
		ID:             gateNodeID,
		GraphID:        identity.GraphID,
		OwnerUserID:    identity.OwnerUserID,
		SessionKey:     identity.SessionKey,
		ExecutionID:    identity.ExecutionID,
		Kind:           protocol.ExecutionRuntimeNodeGate,
		SubjectID:      commandID,
		RootRoundID:    identity.RootRoundID,
		RuntimeRoundID: identity.RuntimeRoundID,
		AgentRoundID:   identity.AgentRoundID,
		AgentID:        identity.AgentID,
		Name:           "objective_alignment",
		Description:    report.Summary,
		Status:         protocol.ExecutionRuntimeNodeSucceeded,
		StartedAt:      now,
		UpdatedAt:      now,
		FinishedAt:     &now,
		Metadata: map[string]any{
			"decision":         string(report.Decision),
			"summary":          report.Summary,
			"criteria_results": report.CriteriaResults,
		},
	}); err != nil {
		return MutationResult{}, err
	}
	if err = upsertRuntimeGraphEdge(
		ctx,
		repository,
		identity,
		rootNodeID,
		gateNodeID,
		protocol.ExecutionRuntimeEdgeGuard,
		now,
	); err != nil {
		return MutationResult{}, err
	}
	if report.Decision != protocol.ObjectiveAlignmentAligned {
		if err = upsertRuntimeGraphEdge(
			ctx,
			repository,
			identity,
			gateNodeID,
			rootNodeID,
			protocol.ExecutionRuntimeEdgeLoopBack,
			now,
		); err != nil {
			return MutationResult{}, err
		}
	}

	result := AppliedResult(
		snapshot,
		[]string{"runtime_gate:" + gateNodeID},
		nextActions(snapshot, actor),
	)
	result.Message = fmt.Sprintf(
		"objective alignment recorded as %s; the Agent retains control of the next step",
		report.Decision,
	)
	return result, nil
}

func upsertRuntimeGraphEdge(
	ctx context.Context,
	repository runtimeGraphRepository,
	identity runtimeGraphIdentity,
	sourceNodeID string,
	targetNodeID string,
	kind protocol.ExecutionRuntimeEdgeKind,
	createdAt time.Time,
) error {
	edge := protocol.ExecutionRuntimeEdgeRun{
		GraphID:      identity.GraphID,
		OwnerUserID:  identity.OwnerUserID,
		SessionKey:   identity.SessionKey,
		SourceNodeID: strings.TrimSpace(sourceNodeID),
		TargetNodeID: strings.TrimSpace(targetNodeID),
		Kind:         kind,
		CreatedAt:    createdAt.UTC(),
	}
	edge.ID = runtimeGraphEdgeID(edge)
	return repository.UpsertRuntimeGraphEdge(ctx, edge)
}
