// INPUT: 已提交的 Execution/Runtime Graph mutation 与 owner/session identity。
// OUTPUT: 不携带业务写权限的 WorkGraph 失效通知，供 transport 重新拉取只读投影。
// POS: orchestration 到实时传输层的窄事件 port；服务不依赖 handler 或 WebSocket。
package orchestration

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) invalidateMutationResult(
	ctx context.Context,
	result MutationResult,
	err error,
) {
	// Idempotent mutation replay is also a valid invalidation source: the
	// original event may have been lost while the durable state already exists.
	// The transport/frontend intentionally coalesce duplicate notifications.
	if (result.Outcome == MutationApplied || result.Outcome == MutationNoOp) &&
		result.Snapshot != nil {
		// The exact durable rows decide whether dispatch or saga work exists.
		// Waking both coordinators here also repairs a notification lost by an
		// earlier idempotent command attempt without coupling command handlers to
		// individual outbox implementations.
		s.WakeExecutionDispatch()
		s.WakeOrchestrationRecovery()
		s.invalidateSnapshot(ctx, result.Snapshot)
		return
	}
	var pending *GoalBindingConfirmationPendingError
	if errors.As(err, &pending) && pending.DurableMutation && pending.Snapshot != nil {
		s.WakeOrchestrationRecovery()
		s.invalidateSnapshot(ctx, pending.Snapshot)
	}
}

// ExecutionInvalidation 是一次 durable WorkGraph 读取失效事实。
// ExecutionID/Version 可在 planless、撤销或 identity fence 场景为空/0；
// OwnerUserID + SessionKey 始终决定唯一广播范围。
type ExecutionInvalidation struct {
	OwnerUserID string
	SessionKey  string
	ExecutionID string
	Version     int64
}

// ExecutionInvalidationSink 由传输装配层实现，禁止反向提供 mutation 能力。
type ExecutionInvalidationSink interface {
	InvalidateExecution(context.Context, ExecutionInvalidation)
}

// SetExecutionInvalidationSink 注入成功 mutation 后的实时失效出口。
func (s *Service) SetExecutionInvalidationSink(sink ExecutionInvalidationSink) {
	if s == nil {
		return
	}
	s.invalidationMu.Lock()
	s.invalidationSink = sink
	s.invalidationMu.Unlock()
}

func (s *Service) invalidateSnapshot(
	ctx context.Context,
	snapshot *protocol.ExecutionSnapshot,
) {
	if snapshot == nil {
		return
	}
	s.invalidateExecution(ctx, ExecutionInvalidation{
		OwnerUserID: strings.TrimSpace(snapshot.Execution.OwnerUserID),
		SessionKey:  strings.TrimSpace(snapshot.Execution.SessionKey),
		ExecutionID: strings.TrimSpace(snapshot.Execution.ID),
		Version:     snapshot.Execution.Version,
	})
}

func (s *Service) invalidateActor(ctx context.Context, actor ActorContext) {
	s.invalidateExecution(ctx, ExecutionInvalidation{
		OwnerUserID: strings.TrimSpace(actor.OwnerUserID),
		SessionKey:  strings.TrimSpace(actor.SessionKey),
		ExecutionID: strings.TrimSpace(actor.ExecutionID),
	})
}

func (s *Service) invalidateExecutionID(ctx context.Context, executionID string) {
	if s == nil || s.repository == nil || strings.TrimSpace(executionID) == "" {
		return
	}
	snapshot, err := s.repository.GetSnapshot(ctx, strings.TrimSpace(executionID))
	if err == nil {
		s.invalidateSnapshot(ctx, snapshot)
	}
}

func (s *Service) invalidateExecution(
	ctx context.Context,
	invalidation ExecutionInvalidation,
) {
	if s == nil || strings.TrimSpace(invalidation.OwnerUserID) == "" ||
		strings.TrimSpace(invalidation.SessionKey) == "" {
		return
	}
	invalidation.OwnerUserID = strings.TrimSpace(invalidation.OwnerUserID)
	invalidation.SessionKey = strings.TrimSpace(invalidation.SessionKey)
	invalidation.ExecutionID = strings.TrimSpace(invalidation.ExecutionID)
	s.invalidationMu.RLock()
	sink := s.invalidationSink
	s.invalidationMu.RUnlock()
	if sink != nil {
		sink.InvalidateExecution(ctx, invalidation)
	}
}
