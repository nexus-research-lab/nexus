// INPUT: runtime source 的累计 actual token 快照、durable scope 暂存与固定 Goal 绑定。
// OUTPUT: checkpoint 去重、scope 回补、Goal usage 与审计事件的原子持久化结果。
// POS: nxs child usage 进入 Goal accounting 的唯一服务入口。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type usageSourceRepository interface {
	ApplyUsageSourceSnapshot(context.Context, protocol.GoalUsageSourceSnapshot) (protocol.GoalUsageSourceResult, error)
}

type usageSourceRoundClaimRepository interface {
	ClaimUsageSourceRound(context.Context, protocol.GoalUsageSourceRoundClaim) (protocol.GoalUsageSourceResult, error)
}

type usageScopeCreateRepository interface {
	CreateGoalWithUsageScope(
		context.Context,
		protocol.Goal,
		protocol.GoalEvent,
		protocol.GoalUsageScopeBinding,
	) (protocol.GoalUsageScopeCreateResult, error)
}

type usageParentRepository interface {
	RecordUsageParentSnapshot(
		context.Context,
		protocol.GoalUsageParentSnapshot,
	) (protocol.GoalUsageParentResult, error)
}

type usageScopeBindRepository interface {
	BindUsageScopeFromNow(
		context.Context,
		protocol.GoalUsageScopeBinding,
	) (protocol.GoalUsageScopeBindResult, error)
}

// RecordUsageSourceSnapshot 原子推进全局 source checkpoint、scope/source-round
// child lifecycle evidence，并在 scope 已绑定时归属新增 actual。
func (s *Service) RecordUsageSourceSnapshot(
	ctx context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	snapshot.OwnerUserID = strings.TrimSpace(snapshot.OwnerUserID)
	snapshot.RuntimeSessionKey = strings.TrimSpace(snapshot.RuntimeSessionKey)
	snapshot.SourceKind = strings.TrimSpace(snapshot.SourceKind)
	snapshot.SourceID = strings.TrimSpace(snapshot.SourceID)
	snapshot.GoalID = strings.TrimSpace(snapshot.GoalID)
	snapshot.GoalSessionKey = strings.TrimSpace(snapshot.GoalSessionKey)
	snapshot.RoundID = strings.TrimSpace(snapshot.RoundID)
	snapshot.ScopeRoundID = strings.TrimSpace(snapshot.ScopeRoundID)
	if snapshot.ScopeRoundID == "" {
		snapshot.ScopeRoundID = snapshot.RoundID
	}
	if snapshot.OwnerUserID == "" ||
		snapshot.RuntimeSessionKey == "" ||
		snapshot.SourceKind != protocol.GoalUsageSourceKindNXSTask ||
		snapshot.SourceID == "" ||
		snapshot.GoalSessionKey == "" ||
		snapshot.ScopeRoundID == "" ||
		snapshot.CumulativeActualTokens < 0 {
		return protocol.GoalUsageSourceResult{}, ErrGoalInvalidInput
	}
	repository, ok := s.repo.(usageSourceRepository)
	if !ok {
		return protocol.GoalUsageSourceResult{}, fmt.Errorf("%w: usage source checkpoints unavailable", ErrGoalInvalidState)
	}
	if snapshot.EventID == "" {
		snapshot.EventID = s.idFactory("goal_event")
	}
	if snapshot.ObservedAt.IsZero() {
		snapshot.ObservedAt = s.nowFn()
	}
	result, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	if result.Goal != nil && result.Event != nil {
		s.broadcastGoalEvent(ctx, *result.Goal, *result.Event)
		s.queueGoalSteering(ctx, *result.Goal, *result.Event)
	}
	return result, nil
}

// ClaimUsageSourceRound 把同一 runtime round 暂存的 source token 原子归入
// model 在该 round 中创建的 Goal；重复调用只会得到空结果。
func (s *Service) ClaimUsageSourceRound(
	ctx context.Context,
	claim protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	claim.OwnerUserID = strings.TrimSpace(claim.OwnerUserID)
	claim.RuntimeSessionKey = strings.TrimSpace(claim.RuntimeSessionKey)
	claim.SourceKind = strings.TrimSpace(claim.SourceKind)
	claim.RoundID = strings.TrimSpace(claim.RoundID)
	claim.ScopeRoundID = strings.TrimSpace(claim.ScopeRoundID)
	if claim.ScopeRoundID == "" {
		claim.ScopeRoundID = claim.RoundID
	}
	claim.GoalID = strings.TrimSpace(claim.GoalID)
	claim.GoalSessionKey = strings.TrimSpace(claim.GoalSessionKey)
	if claim.OwnerUserID == "" ||
		claim.SourceKind != protocol.GoalUsageSourceKindNXSTask ||
		claim.ScopeRoundID == "" ||
		claim.GoalID == "" ||
		claim.GoalSessionKey == "" {
		return protocol.GoalUsageSourceResult{}, ErrGoalInvalidInput
	}
	repository, ok := s.repo.(usageSourceRoundClaimRepository)
	if !ok {
		return protocol.GoalUsageSourceResult{}, fmt.Errorf(
			"%w: usage source round claims unavailable",
			ErrGoalInvalidState,
		)
	}
	if claim.EventID == "" {
		claim.EventID = s.idFactory("goal_event")
	}
	if claim.ClaimedAt.IsZero() {
		claim.ClaimedAt = s.nowFn()
	}
	result, err := repository.ClaimUsageSourceRound(ctx, claim)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	if result.Goal != nil && result.Event != nil {
		s.broadcastGoalEvent(ctx, *result.Goal, *result.Event)
		s.queueGoalSteering(ctx, *result.Goal, *result.Event)
	}
	return result, nil
}

// RecordUsageParentSnapshot 持久化 Room parent slot 的终态 usage 证据，并在
// root scope 已绑定时 exactly-once 归入 Goal。
func (s *Service) RecordUsageParentSnapshot(
	ctx context.Context,
	snapshot protocol.GoalUsageParentSnapshot,
) (protocol.GoalUsageParentResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	snapshot.OwnerUserID = strings.TrimSpace(snapshot.OwnerUserID)
	snapshot.GoalSessionKey = strings.TrimSpace(snapshot.GoalSessionKey)
	snapshot.ScopeRoundID = strings.TrimSpace(snapshot.ScopeRoundID)
	snapshot.SourceRoundID = strings.TrimSpace(snapshot.SourceRoundID)
	snapshot.GoalID = strings.TrimSpace(snapshot.GoalID)
	if snapshot.OwnerUserID == "" ||
		snapshot.GoalSessionKey == "" ||
		snapshot.ScopeRoundID == "" ||
		snapshot.SourceRoundID == "" {
		return protocol.GoalUsageParentResult{}, ErrGoalInvalidInput
	}
	repository, ok := s.repo.(usageParentRepository)
	if !ok {
		return protocol.GoalUsageParentResult{}, fmt.Errorf(
			"%w: parent usage ledger unavailable",
			ErrGoalInvalidState,
		)
	}
	if snapshot.EventID == "" {
		snapshot.EventID = s.idFactory("goal_event")
	}
	if snapshot.ObservedAt.IsZero() {
		snapshot.ObservedAt = s.nowFn()
	}
	result, err := repository.RecordUsageParentSnapshot(ctx, snapshot)
	if err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	if result.Goal != nil && result.Event != nil {
		s.broadcastGoalEvent(ctx, *result.Goal, *result.Event)
		s.queueGoalSteering(ctx, *result.Goal, *result.Event)
	}
	return result, nil
}

// BindUsageScopeFromNow 按 external Reset 语义建立 durable scope 绑定；
// 绑定前 child/parent pending 与已终止 child evidence 被原子排除，仍在运行的
// child evidence 保留为新 Goal 的 required barrier。
func (s *Service) BindUsageScopeFromNow(
	ctx context.Context,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return protocol.GoalUsageScopeBindResult{}, err
	}
	binding.OwnerUserID = strings.TrimSpace(binding.OwnerUserID)
	binding.GoalSessionKey = strings.TrimSpace(binding.GoalSessionKey)
	binding.SourceKind = strings.TrimSpace(binding.SourceKind)
	if binding.SourceKind == "" {
		binding.SourceKind = protocol.GoalUsageSourceKindNXSTask
	}
	binding.ScopeRoundID = strings.TrimSpace(binding.ScopeRoundID)
	binding.GoalID = strings.TrimSpace(binding.GoalID)
	if binding.OwnerUserID == "" ||
		binding.GoalSessionKey == "" ||
		binding.SourceKind != protocol.GoalUsageSourceKindNXSTask ||
		binding.ScopeRoundID == "" ||
		binding.GoalID == "" {
		return protocol.GoalUsageScopeBindResult{}, ErrGoalInvalidInput
	}
	repository, ok := s.repo.(usageScopeBindRepository)
	if !ok {
		return protocol.GoalUsageScopeBindResult{}, fmt.Errorf(
			"%w: usage scope binding unavailable",
			ErrGoalInvalidState,
		)
	}
	if binding.BoundAt.IsZero() {
		binding.BoundAt = s.nowFn()
	}
	return repository.BindUsageScopeFromNow(ctx, binding)
}
