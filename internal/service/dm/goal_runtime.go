// INPUT: DM round 的模型结果、assistant/child 快照、usage 与 objective revision。
// OUTPUT: revision 安全的 Goal usage、round-start child 回补、进展和终态回调。
// POS: DM runtime 结果到 Goal 状态机的结算投影层。
package dm

import (
	"context"
	"errors"
	"strings"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

const (
	goalUsagePersistAttempts       = 5
	subagentUsageRetryInitialDelay = 320 * time.Millisecond
	subagentUsageRetryMaxDelay     = 5 * time.Second
)

func (r *roundRunner) recordGoalUsage(ctx context.Context, result exec.RoundExecutionResult, finalAssistant protocol.Message) {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return
	}
	snapshot, ok := r.finalGoalUsageSnapshot(result, finalAssistant)
	if !ok {
		return
	}
	r.recordGoalUsageSnapshot(ctx, snapshot)
}

func (r *roundRunner) finalizeGoalUsage(ctx context.Context, result exec.RoundExecutionResult, finalAssistant protocol.Message) {
	snapshot, _ := r.finalGoalUsageSnapshot(result, finalAssistant)
	version := r.rememberTerminalGoalUsageSnapshot(snapshot)
	if !r.ensureSubagentGoalUsageRoundClaimed(ctx) {
		r.service.loggerFor(ctx).Warn(
			"DM terminal Goal usage 等待 round-start child 回补",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
		)
		return
	}
	settled := r.settleTerminalGoalUsageSnapshotWithRetry(ctx, snapshot)
	if !settled {
		r.service.loggerFor(ctx).Warn(
			"DM terminal Goal usage 未能持久化",
			"session_key", r.sessionKey,
			"goal_id", r.goalIDForUsage,
			"round_id", r.roundID,
		)
		return
	}
	if r.clearTerminalGoalUsageSnapshot(version) {
		r.closeGoalUsageIfNoTerminalSnapshotPending()
	}
}

func (r *roundRunner) settleTerminalGoalUsageSnapshotWithRetry(
	ctx context.Context,
	snapshot goalsvc.RuntimeUsageSnapshot,
) bool {
	for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
		if attempt > 0 && !r.waitGoalUsagePersistRetry(ctx, attempt) {
			return false
		}
		if r.settleTerminalGoalUsageSnapshot(ctx, snapshot) {
			return true
		}
	}
	return false
}

func (r *roundRunner) waitGoalUsagePersistRetry(ctx context.Context, attempt int) bool {
	baseDelay := 20 * time.Millisecond
	if r != nil && r.goalUsageRetryBaseDelay > 0 {
		baseDelay = r.goalUsageRetryBaseDelay
	}
	delay := baseDelay * time.Duration(1<<min(attempt-1, 4))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r *roundRunner) recordGoalUsageLimit(result exec.RoundExecutionResult) {
	if r.service.goals == nil || r.ignoreGoalRuntime() || !result.UsageLimitReached {
		return
	}
	r.goalUsageMu.Lock()
	goalID := strings.TrimSpace(r.goalIDForUsage)
	r.goalUsageMu.Unlock()
	var err error
	if goalID != "" {
		if provider, ok := r.service.goals.(interface {
			UsageLimitForGoal(context.Context, string, string, string) (*protocol.Goal, error)
		}); ok {
			_, err = provider.UsageLimitForGoal(context.Background(), goalID, r.roundID, result.UsageLimitReason)
		} else {
			_, err = r.service.goals.UsageLimitForSession(context.Background(), r.sessionKey, r.roundID, result.UsageLimitReason)
		}
	} else {
		_, err = r.service.goals.UsageLimitForSession(context.Background(), r.sessionKey, r.roundID, result.UsageLimitReason)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) {
		r.service.loggerFor(context.Background()).Warn("标记 Goal usage limit 失败",
			"session_key", r.sessionKey,
			"goal_id", goalID,
			"round_id", r.roundID,
			"err", err,
		)
	}
}

func (r *roundRunner) flushGoalUsage(ctx context.Context) error {
	snapshot, ok := r.finalGoalUsageSnapshot(exec.RoundExecutionResult{}, r.lastGoalAssistantMessage())
	settlementBoundary := goalsvc.RuntimeUsageSettlementBoundary(ctx)
	if !ok && !settlementBoundary {
		return nil
	}
	// 外部 Goal mutation 的 flush 发生在 round 中途，不是 provider terminal。
	// estimated actual 必须继续等待真正终态，否则 exact total 无法向下校准。
	snapshot.Terminal = false
	snapshot.SettlementBoundary = settlementBoundary
	if r.tryRecordGoalUsageSnapshot(ctx, snapshot) {
		return nil
	}
	return errors.New("persist DM Goal usage checkpoint")
}

func (r *roundRunner) clearGoalUsage() {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if r.goalUsage != nil {
		r.goalUsage.Close()
	}
	r.goalIDForUsage = ""
	r.childGoalIDForUsage = ""
	r.subagentUsageClaimPending = false
	r.goalTokenUsageObserved = false
}

// beginGoalUsageFinalizing 用于外部 complete：保留当前 Goal 固定绑定，
// 让 provider terminal 与迟到 child usage 完成最终对账后再关闭。
func (r *roundRunner) beginGoalUsageFinalizing() bool {
	if r == nil || r.service == nil || r.service.goals == nil || r.ignoreGoalRuntime() {
		return false
	}
	if _, ok := r.service.goals.(dmGoalUsageFinalizationProvider); !ok {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if strings.TrimSpace(r.goalIDForUsage) == "" ||
		r.goalUsage == nil ||
		!r.goalUsage.Active() {
		return false
	}
	r.goalUsage.BeginFinalizing()
	return r.goalUsage.Active()
}

func (r *roundRunner) initializeGoalUsageCreateGuard() {
	if r == nil {
		return
	}
	r.goalUsageMu.Lock()
	if strings.TrimSpace(r.goalIDForUsage) != "" {
		r.goalUsageScopeConsumed = true
	}
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) goalUsageScopeWasConsumed() bool {
	if r == nil {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return r.goalUsageScopeConsumed
}

type dmGoalUsageScopeBinder interface {
	BindUsageScopeFromNow(
		context.Context,
		protocol.GoalUsageScopeBinding,
	) (protocol.GoalUsageScopeBindResult, error)
}

func (r *roundRunner) activateGoalUsage(ctx context.Context, goalID string) error {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return nil
	}
	goalID = strings.TrimSpace(goalID)
	// Source persistence and durable binding are ordered independently from
	// in-memory lifecycle state. Child terminal handling must remain live while
	// a checkpoint provider call is blocked.
	r.goalUsageBindingMu.Lock()
	defer r.goalUsageBindingMu.Unlock()
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if goalID != "" &&
		strings.TrimSpace(r.goalIDForUsage) == goalID &&
		r.goalUsage != nil &&
		r.goalUsage.Active() {
		r.goalUsageScopeConsumed = true
		if strings.TrimSpace(r.childGoalIDForUsage) == "" {
			r.childGoalIDForUsage = goalID
		}
		return nil
	}
	if goalID != "" {
		if binder, ok := r.service.goals.(dmGoalUsageScopeBinder); ok {
			// Durable source checkpoint 与 from-now bind 必须共享同一临界区。
			// 已经进入 pending 的 child observation 先按旧/空 Goal 语义落库，
			// 再由 repository 在绑定事务内将绑定前 backlog 排除，并把仍在运行
			// 的 child 标记为不可建立精确 baseline。
			if recorder, recordsSources := r.service.goals.(dmGoalUsageSourceRecorder); recordsSources {
				if err := r.flushPendingSubagentUsageBeforeBindLocked(ctx, recorder); err != nil {
					return err
				}
			}
			binding := protocol.GoalUsageScopeBinding{
				OwnerUserID:    r.ownerUserID,
				GoalSessionKey: r.sessionKey,
				SourceKind:     protocol.GoalUsageSourceKindNXSTask,
				ScopeRoundID:   r.roundID,
				GoalID:         goalID,
				BoundAt:        time.Now().UTC(),
			}
			var err error
			for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
				if attempt > 0 && !r.waitGoalUsagePersistRetry(ctx, attempt) {
					return ctx.Err()
				}
				if _, err = binder.BindUsageScopeFromNow(ctx, binding); err == nil {
					break
				}
				if errors.Is(err, goalsvc.ErrGoalInvalidState) {
					// 非 SQL provider 没有 durable scope capability；保留兼容的
					// in-memory Reset，不把能力缺失当成瞬时写失败。
					err = nil
					break
				}
			}
			if err != nil {
				// durable bind 是 Reset/consumed 的前置条件。失败时必须保持
				// 原 Goal binding 与 accumulator baseline。
				return err
			}
		}
	}
	usage, _ := runtimectx.GoalUsageFromRaw(r.goalLastAssistant["usage"])
	snapshot := goalsvc.RuntimeUsageSnapshot{
		Usage:          usage,
		ElapsedSeconds: r.elapsedGoalUsageSeconds(),
		TurnID:         strings.TrimSpace(dmdomain.NormalizeString(r.goalLastAssistant["message_id"])),
	}
	r.goalIDForUsage = goalID
	r.childGoalIDForUsage = goalID
	if goalID != "" {
		r.goalUsageScopeConsumed = true
	}
	r.subagentUsageClaimPending = false
	if r.goalUsage == nil {
		r.goalUsage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	r.goalUsage.Reset(snapshot)
	r.goalTokenUsageObserved = false
	return nil
}

func (r *roundRunner) rememberGoalAssistantMessage(message protocol.Message) {
	if protocol.MessageRole(message) != "assistant" {
		return
	}
	r.goalUsageMu.Lock()
	r.goalLastAssistant = protocol.Clone(message)
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) lastGoalAssistantMessage() protocol.Message {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return protocol.Clone(r.goalLastAssistant)
}

func (r *roundRunner) recordGoalUsageFromAssistantMessage(message protocol.Message) {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return
	}
	if protocol.MessageRole(message) != "assistant" {
		return
	}
	observations := messageutil.AssistantToolResults(message)
	if len(observations) > 0 {
		r.rememberGoalToolProgress(messageutil.AssistantHasCountedToolProgress(message))
	}
	snapshot := r.assistantGoalUsageSnapshot(message)
	hasSuccessfulCreate := false
	hasSuccessfulUpdate := false
	for _, observation := range observations {
		if observation.IsError ||
			observation.MutationOutcome == protocol.MutationResultRejected ||
			observation.MutationOutcome == protocol.MutationResultSuperseded {
			continue
		}
		switch messageutil.CanonicalToolName(observation.ToolName) {
		case "create_goal":
			hasSuccessfulCreate = true
		case "update_goal":
			hasSuccessfulUpdate = true
		}
	}
	if hasSuccessfulCreate {
		r.goalUsageMu.Lock()
		r.goalUsageScopeConsumed = true
		if r.goalUsage == nil || !r.goalUsage.Active() {
			if r.goalUsage == nil {
				r.goalUsage = goalsvc.NewRuntimeUsageAccumulator(false)
			}
			// 模型在本轮中创建 Goal 时，本轮就是该 Goal 的第一段工作，
			// 因此从 round 起点结算，而不是把 create_goal 时的累计量当作基线丢弃。
			if backlog, ok := r.goalUsage.PrepareActivationFromRoundStart(); ok {
				// 激活边界、delta 与 Goal 绑定必须在同一临界区内落库，
				// 避免外部 Goal 切换把旧 baseline 的 backlog 写到新 Goal。
				if _, persisted := r.persistGoalUsageDeltaLocked(context.Background(), backlog); persisted {
					r.goalUsage.CommitDelta(backlog)
				}
			}
			r.goalTokenUsageObserved = r.goalUsage.TokenUsageObserved()
		}
		r.goalUsageMu.Unlock()
		goalID := r.ensureModelCreatedGoalBinding(context.Background())
		r.claimSubagentGoalUsageRound(context.Background(), goalID)
	}
	r.recordGoalUsageSnapshot(context.Background(), snapshot)
	if hasSuccessfulUpdate {
		// update_goal 的 tool result 到达时，当前 provider turn 尚未生成最终回复。
		// 优先使用结果返回的 exact Goal ID；旧 provider 才回退到本 round 固定
		// binding。保持该绑定直到 terminal usage 完成最终对账后再关闭。
		r.goalUsageMu.Lock()
		if goalID := messageutil.SuccessfulGoalCompletionID(
			observations,
			r.goalIDForUsage,
		); goalID != "" {
			r.goalCompletionCandidateID = goalID
		}
		if r.goalUsage != nil {
			r.goalUsage.BeginFinalizing()
		}
		r.goalUsageMu.Unlock()
	}
}

func (r *roundRunner) recordGoalContinuationProgress(result exec.RoundExecutionResult) {
	if r.service.goals == nil || r.ignoreGoalRuntime() || strings.TrimSpace(r.goalIDForUsage) == "" {
		return
	}
	if strings.TrimSpace(r.inputOptions.Purpose) == "goal_continuation" && result.TerminalStatus == "error" {
		assistantText := ""
		if r.mapper != nil {
			assistantText = messageutil.ExtractAssistantDisplayText(r.mapper.LastAssistantMessage())
		}
		reason := dmdomain.FirstNonEmpty(
			strings.TrimSpace(result.ErrorMessage),
			assistantText,
			"Goal continuation runtime failed",
		)
		r.recordGoalMutation("记录 Goal 续跑失败原因失败", func() error {
			_, err := r.service.goals.RecordContinuationFailure(context.Background(), r.goalIDForUsage, r.roundID, reason, r.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	if strings.TrimSpace(r.inputOptions.Purpose) != "goal_continuation" {
		r.recordGoalMutation("记录 Goal 显式活动失败", func() error {
			_, err := r.service.goals.RecordGoalActivity(context.Background(), r.goalIDForUsage, r.roundID, r.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	if messageutil.AssistantMissedGoalCompletionTool(r.lastGoalAssistantMessage()) {
		reason := "assistant claimed goal completion but did not call mcp__nexus_goal__update_goal"
		r.recordGoalMutation("记录 Goal 完成工具漏调用失败", func() error {
			_, err := r.service.goals.RecordCompletionToolMiss(context.Background(), r.goalIDForUsage, r.roundID, reason, r.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	progressed := r.hasGoalToolProgress()
	if !progressed && r.hasRunningSubagentTask() {
		return
	}
	r.recordGoalMutation("记录 Goal 续跑进展失败", func() error {
		_, err := r.service.goals.RecordContinuationProgress(context.Background(), r.goalIDForUsage, r.roundID, progressed, r.currentGoalObjectiveRevision())
		return err
	}, "progressed", progressed)
}

func (r *roundRunner) currentGoalObjectiveRevision() int64 {
	if r == nil || r.goalObjectiveRevision == nil {
		return 0
	}
	return r.goalObjectiveRevision.Load()
}

func (r *roundRunner) hasGoalRoundBinding() bool {
	if r == nil || r.ignoreGoalRuntime() || r.currentGoalObjectiveRevision() <= 0 {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return strings.TrimSpace(r.goalIDForUsage) != ""
}

func (r *roundRunner) recordGoalMutation(logMessage string, mutation func() error, fields ...any) {
	err := mutation()
	if err == nil || goalsvc.IsExpectedMutationError(err) {
		return
	}
	baseFields := []any{
		"session_key", r.sessionKey,
		"goal_id", r.goalIDForUsage,
		"round_id", r.roundID,
	}
	baseFields = append(baseFields, fields...)
	baseFields = append(baseFields, "err", err)
	r.service.loggerFor(context.Background()).Warn(logMessage, baseFields...)
}

func (r *roundRunner) rememberGoalToolProgress(progressed bool) {
	if !progressed {
		return
	}
	r.goalUsageMu.Lock()
	r.goalToolProgress = true
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) hasGoalToolProgress() bool {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return r.goalToolProgress
}

func (r *roundRunner) finalGoalUsageSnapshot(
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) (goalsvc.RuntimeUsageSnapshot, bool) {
	usage, usageOK := runtimectx.GoalUsageFromTokenUsageWithPresence(result.Usage)
	cumulative := usageOK
	if !cumulative && protocol.MessageRole(finalAssistant) == "assistant" {
		usage, usageOK = runtimectx.GoalUsageFromRaw(finalAssistant["usage"])
	}
	elapsedSeconds := result.ElapsedTimeSeconds
	if elapsedSeconds <= 0 {
		elapsedSeconds = r.elapsedGoalUsageSeconds()
	}
	return goalsvc.RuntimeUsageSnapshot{
		Usage:              usage,
		ElapsedSeconds:     elapsedSeconds,
		TokenUsageObserved: usageOK,
		TurnID:             strings.TrimSpace(dmdomain.NormalizeString(finalAssistant["message_id"])),
		Cumulative:         cumulative,
		Terminal:           true,
	}, usageOK || elapsedSeconds > 0
}

// rememberTerminalGoalUsageSnapshot 保留 provider terminal 快照，直到 claim、
// parent delta 与 completion fence 全部可持久化。同步重试耗尽后，后台 worker
// 仍能使用原始累计真值继续结算，而不是退回空快照。
func (r *roundRunner) rememberTerminalGoalUsageSnapshot(
	snapshot goalsvc.RuntimeUsageSnapshot,
) uint64 {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	r.goalTerminalUsageVersion++
	r.goalTerminalUsageSnapshot = snapshot
	r.goalTerminalUsagePending = true
	if snapshot.TokenUsageObserved {
		r.goalTokenUsageObserved = true
	}
	return r.goalTerminalUsageVersion
}

func (r *roundRunner) pendingTerminalGoalUsageSnapshot() (
	goalsvc.RuntimeUsageSnapshot,
	uint64,
	bool,
) {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return r.goalTerminalUsageSnapshot, r.goalTerminalUsageVersion, r.goalTerminalUsagePending
}

// clearTerminalGoalUsageSnapshot 采用版本条件清除，防止较旧的并发重试成功
// 抹掉随后到达、仍未持久化的新 terminal 累计快照。
func (r *roundRunner) clearTerminalGoalUsageSnapshot(version uint64) bool {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if r.goalTerminalUsagePending && r.goalTerminalUsageVersion == version {
		r.goalTerminalUsagePending = false
		return true
	}
	return !r.goalTerminalUsagePending
}

// closeGoalUsageIfNoTerminalSnapshotPending 把版本确认与 Close 放在同一临界区。
// 若旧 settlement 完成后已有更新 terminal 快照接力，必须保持 accumulator
// 活跃，直到新版本也完成持久化。
func (r *roundRunner) closeGoalUsageIfNoTerminalSnapshotPending() bool {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if r.goalTerminalUsagePending {
		return false
	}
	if r.goalUsage != nil {
		r.goalUsage.Close()
	}
	return true
}

func (r *roundRunner) assistantGoalUsageSnapshot(message protocol.Message) goalsvc.RuntimeUsageSnapshot {
	usage, usageObserved := runtimectx.GoalUsageFromRaw(message["usage"])
	return goalsvc.RuntimeUsageSnapshot{
		Usage:              usage,
		ElapsedSeconds:     r.elapsedGoalUsageSeconds(),
		TokenUsageObserved: usageObserved,
		TurnID:             strings.TrimSpace(dmdomain.NormalizeString(message["message_id"])),
	}
}

func (r *roundRunner) recordGoalUsageSnapshot(ctx context.Context, snapshot goalsvc.RuntimeUsageSnapshot) {
	_ = r.tryRecordGoalUsageSnapshot(ctx, snapshot)
}

func (r *roundRunner) tryRecordGoalUsageSnapshot(
	ctx context.Context,
	snapshot goalsvc.RuntimeUsageSnapshot,
) bool {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return true
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if r.goalUsage != nil {
		usage, ok := r.goalUsage.PrepareDelta(snapshot)
		if r.goalUsage.TokenUsageObserved() && r.goalUsage.Active() {
			r.goalTokenUsageObserved = true
		}
		if !ok {
			return true
		}
		if _, persisted := r.persistGoalUsageDeltaLocked(ctx, usage); persisted {
			r.goalUsage.CommitDelta(usage)
			return true
		}
		return false
	}
	usage := snapshot.Usage
	usage.RuntimeSeconds = snapshot.ElapsedSeconds
	if isZeroGoalUsage(usage) {
		return true
	}
	_, persisted := r.persistGoalUsageDeltaLocked(ctx, usage)
	return persisted
}

type dmGoalUsageFinalizationProvider interface {
	UsageByGoalID(context.Context, string) (*protocol.GoalUsageReport, error)
	FinalizeUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
}

type dmGoalUsageParentRecorder interface {
	RecordUsageParentSnapshot(
		context.Context,
		protocol.GoalUsageParentSnapshot,
	) (protocol.GoalUsageParentResult, error)
}

func (r *roundRunner) settleTerminalGoalUsageSnapshot(
	ctx context.Context,
	snapshot goalsvc.RuntimeUsageSnapshot,
) bool {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return true
	}
	snapshot.Terminal = true
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if snapshot.TokenUsageObserved {
		r.goalTokenUsageObserved = true
	}

	var (
		usage    protocol.GoalUsage
		hasDelta bool
	)
	if r.goalUsage != nil {
		usage, hasDelta = r.goalUsage.PrepareDelta(snapshot)
		if r.goalUsage.TokenUsageObserved() {
			r.goalTokenUsageObserved = true
		}
	} else {
		usage = snapshot.Usage
		usage.RuntimeSeconds = snapshot.ElapsedSeconds
		hasDelta = !isZeroGoalUsage(usage)
	}
	goalID := strings.TrimSpace(r.goalIDForUsage)
	finalizer, canFinalize := r.service.goals.(dmGoalUsageFinalizationProvider)
	var report *protocol.GoalUsageReport
	if canFinalize && goalID != "" {
		var err error
		report, err = finalizer.UsageByGoalID(ctx, goalID)
		if err != nil {
			return false
		}
		if report != nil && report.UsageFinalized {
			// FinalizeUsageForGoal 可能已在存储中提交，只是调用方收到错误。
			// 此时本地 accumulator 仍会重放 delta；以 durable fence 为准直接收口，
			// 避免把已结算 token 永久重试。
			return true
		}
	}

	parentRecorded := false
	if recorder, ok := r.service.goals.(dmGoalUsageParentRecorder); ok && goalID != "" {
		if _, err := recorder.RecordUsageParentSnapshot(ctx, protocol.GoalUsageParentSnapshot{
			OwnerUserID:        r.ownerUserID,
			GoalSessionKey:     r.sessionKey,
			ScopeRoundID:       r.roundID,
			SourceRoundID:      r.roundID,
			GoalID:             goalID,
			Usage:              usage,
			TokenUsageObserved: r.goalTokenUsageObserved,
		}); err != nil {
			return false
		}
		if hasDelta && r.goalUsage != nil {
			r.goalUsage.CommitDelta(usage)
		}
		hasDelta = false
		parentRecorded = true
	}

	if report != nil &&
		protocol.NormalizeGoalStatus(report.Status) == protocol.GoalStatusComplete &&
		len(r.subagentTasks) == 0 &&
		len(r.subagentUsagePending) == 0 {
		if !r.goalTokenUsageObserved {
			// 缺失 parent provider usage 只能落 durable unavailable evidence，
			// 不能用零值建立 authoritative finalization fence。
			if parentRecorded || !hasDelta {
				return true
			}
			if _, persisted := r.persistGoalUsageDeltaLocked(ctx, usage); !persisted {
				return false
			}
			if r.goalUsage != nil {
				r.goalUsage.CommitDelta(usage)
			}
			return true
		}
		finalDelta := usage
		if parentRecorded {
			// durable parent ledger 已 exactly-once 归属 terminal usage；
			// finalization 只负责冻结，不能再次叠加 token。
			finalDelta = protocol.GoalUsage{}
		}
		if _, err := finalizer.FinalizeUsageForGoal(ctx, goalID, finalDelta, r.roundID); err != nil {
			if errors.Is(err, goalsvc.ErrGoalUsageUnavailable) {
				// Durable child terminal evidence 已明确证明 usage 不可得。
				// 这是不可重试的精度结论：保留 usage_finalized=false，但允许
				// parent/child join 与 post-round 工作正常收口。
				return true
			}
			return false
		}
		if hasDelta && r.goalUsage != nil {
			r.goalUsage.CommitDelta(usage)
		}
		return true
	}
	if parentRecorded {
		return true
	}
	if !hasDelta {
		return true
	}
	if _, persisted := r.persistGoalUsageDeltaLocked(ctx, usage); !persisted {
		return false
	}
	if r.goalUsage != nil {
		r.goalUsage.CommitDelta(usage)
	}
	return true
}

// finalizeCompletedGoalUsageAfterSubagents 在 parent terminal 已结算且最后一个
// child source checkpoint 已提交后，重试未完成的结算并建立最终 fence。
func (r *roundRunner) finalizeCompletedGoalUsageAfterSubagents(ctx context.Context) bool {
	if r == nil || r.hasRunningSubagentTask() {
		return false
	}
	if !r.ensureSubagentGoalUsageRoundClaimed(ctx) {
		return false
	}
	snapshot, version, pending := r.pendingTerminalGoalUsageSnapshot()
	if !pending {
		snapshot = goalsvc.RuntimeUsageSnapshot{Terminal: true}
	}
	settled := r.settleTerminalGoalUsageSnapshotWithRetry(ctx, snapshot)
	if settled {
		canClose := true
		if pending {
			canClose = r.clearTerminalGoalUsageSnapshot(version)
		} else {
			_, _, newerPending := r.pendingTerminalGoalUsageSnapshot()
			canClose = !newerPending
		}
		if canClose {
			r.closeGoalUsageIfNoTerminalSnapshotPending()
		}
		r.persistGoalCompletionReceipt(ctx, true)
	}
	return settled
}

func (r *roundRunner) recordGoalUsageDelta(ctx context.Context, usage protocol.GoalUsage) *protocol.Goal {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return r.recordGoalUsageDeltaLocked(ctx, usage)
}

// recordGoalUsageDeltaLocked 把 delta 与产生它时的 Goal 绑定原子结算。
// 调用方必须持有 goalUsageMu，防止 clear/activate 在 baseline 与持久化之间换绑。
func (r *roundRunner) recordGoalUsageDeltaLocked(ctx context.Context, usage protocol.GoalUsage) *protocol.Goal {
	updated, _ := r.persistGoalUsageDeltaLocked(ctx, usage)
	return updated
}

func (r *roundRunner) persistGoalUsageDeltaLocked(
	ctx context.Context,
	usage protocol.GoalUsage,
) (*protocol.Goal, bool) {
	if r.service.goals == nil || r.ignoreGoalRuntime() || isZeroGoalUsage(usage) {
		return nil, false
	}
	goalID := strings.TrimSpace(r.goalIDForUsage)
	var updated *protocol.Goal
	var err error
	if goalID != "" {
		updated, err = r.service.goals.RecordUsageForGoal(ctx, goalID, usage, r.roundID)
	} else {
		updated, err = r.service.goals.RecordUsageForSession(ctx, r.sessionKey, usage, r.roundID)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
		r.service.loggerFor(context.Background()).Warn("记录 Goal usage 失败",
			"session_key", r.sessionKey,
			"goal_id", goalID,
			"round_id", r.roundID,
			"err", err,
		)
	}
	if err != nil || updated == nil {
		return nil, err == nil
	}
	if goalID == "" && strings.TrimSpace(r.goalIDForUsage) == "" {
		r.goalIDForUsage = strings.TrimSpace(updated.ID)
		r.childGoalIDForUsage = strings.TrimSpace(updated.ID)
	}
	return updated, true
}

func (r *roundRunner) ensureModelCreatedGoalBinding(ctx context.Context) string {
	if r == nil || r.service == nil || r.service.goals == nil {
		return ""
	}
	r.goalUsageMu.Lock()
	goalID := strings.TrimSpace(r.goalIDForUsage)
	if goalID != "" {
		if strings.TrimSpace(r.childGoalIDForUsage) == "" {
			r.childGoalIDForUsage = goalID
		}
		r.goalUsageMu.Unlock()
		return goalID
	}
	r.goalUsageMu.Unlock()

	_, goal, err := r.service.goals.RuntimeContext(ctx, r.sessionKey)
	if err != nil {
		if !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
			r.service.loggerFor(ctx).Warn(
				"读取 model 创建后的 Goal 绑定失败",
				"session_key", r.sessionKey,
				"round_id", r.roundID,
				"err", err,
			)
		}
		return ""
	}
	if goal == nil || strings.TrimSpace(goal.ID) == "" {
		return ""
	}
	r.goalUsageMu.Lock()
	if strings.TrimSpace(r.goalIDForUsage) == "" {
		r.goalIDForUsage = strings.TrimSpace(goal.ID)
	}
	if strings.TrimSpace(r.childGoalIDForUsage) == "" {
		r.childGoalIDForUsage = strings.TrimSpace(goal.ID)
	}
	goalID = strings.TrimSpace(r.childGoalIDForUsage)
	r.goalUsageMu.Unlock()
	return goalID
}

func (r *roundRunner) claimSubagentGoalUsageRound(ctx context.Context, goalID string) {
	if r == nil || r.service == nil || r.service.goals == nil ||
		r.ignoreGoalRuntime() ||
		!strings.EqualFold(strings.TrimSpace(r.runtimeKind), "nxs") {
		return
	}
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		return
	}
	r.goalUsageMu.Lock()
	r.childGoalIDForUsage = goalID
	r.subagentUsageClaimPending = true
	r.goalUsageMu.Unlock()
	if r.ensureSubagentGoalUsageRoundClaimed(ctx) {
		return
	}
	r.service.loggerFor(ctx).Warn(
		"回补 model 创建前的 nxs 子任务 Goal usage 失败",
		"session_key", r.sessionKey,
		"goal_id", goalID,
		"round_id", r.roundID,
	)
}

func (r *roundRunner) ensureSubagentGoalUsageRoundClaimed(ctx context.Context) bool {
	if r == nil || r.service == nil || r.service.goals == nil {
		return true
	}
	r.goalUsageMu.Lock()
	pending := r.subagentUsageClaimPending
	goalID := strings.TrimSpace(r.childGoalIDForUsage)
	r.goalUsageMu.Unlock()
	if !pending {
		return true
	}
	claimer, ok := r.service.goals.(interface {
		ClaimUsageSourceRound(context.Context, protocol.GoalUsageSourceRoundClaim) (protocol.GoalUsageSourceResult, error)
	})
	if !ok {
		r.goalUsageMu.Lock()
		r.subagentUsageClaimPending = false
		r.goalUsageMu.Unlock()
		return true
	}
	claim := protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:       r.ownerUserID,
		RuntimeSessionKey: r.sessionKey,
		SourceKind:        protocol.GoalUsageSourceKindNXSTask,
		RoundID:           r.roundID,
		ScopeRoundID:      r.roundID,
		GoalID:            goalID,
		GoalSessionKey:    r.sessionKey,
	}
	for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
		if attempt > 0 && !r.waitGoalUsagePersistRetry(ctx, attempt) {
			return false
		}
		if _, err := claimer.ClaimUsageSourceRound(ctx, claim); err != nil {
			continue
		}
		r.goalUsageMu.Lock()
		r.subagentUsageClaimPending = false
		r.goalUsageMu.Unlock()
		return true
	}
	return false
}

type dmGoalUsageSourceRecorder interface {
	RecordUsageSourceSnapshot(context.Context, protocol.GoalUsageSourceSnapshot) (protocol.GoalUsageSourceResult, error)
}

type dmSubagentUsageSettlement struct {
	taskID      string
	observation dmSubagentUsageObservation
}

func (r *roundRunner) recordSubagentGoalUsage(
	ctx context.Context,
	message protocol.Message,
) []dmSubagentUsageSettlement {
	if r == nil || r.service == nil ||
		!strings.EqualFold(strings.TrimSpace(r.runtimeKind), "nxs") {
		return nil
	}
	observations := dmSubagentUsageObservations(r, message)
	if len(observations) == 0 {
		return nil
	}
	settledSnapshots := make([]dmSubagentUsageSettlement, 0, len(observations))
	recorder, persistent := r.service.goals.(dmGoalUsageSourceRecorder)
	if persistent {
		hadFailure := false
		// Child checkpoint/evidence 与 external from-now bind 共用专用顺序锁。
		// 整条 durable message 在一个边界内归属，避免 bind 插入多个 child
		// observation 之间，或旧请求在 bind 后才按新 Goal 重放；普通 lifecycle
		// state 不被慢 provider 调用阻塞。
		r.goalUsageBindingMu.Lock()
		for _, child := range observations {
			r.goalUsageMu.Lock()
			r.markSubagentUsageObservationPendingLocked(child.taskID, child.observation)
			observation := r.subagentUsagePending[child.taskID]
			currentGoalID := strings.TrimSpace(r.childGoalIDForUsage)
			if currentGoalID == "" {
				currentGoalID = strings.TrimSpace(r.goalIDForUsage)
			}
			snapshot := r.subagentUsageSourceSnapshotLocked(child.taskID, observation)
			r.goalUsageMu.Unlock()
			var (
				result protocol.GoalUsageSourceResult
				err    error
			)
			for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
				if attempt > 0 && !r.waitGoalUsagePersistRetry(ctx, attempt) {
					break
				}
				result, err = recorder.RecordUsageSourceSnapshot(ctx, snapshot)
				if err == nil {
					break
				}
			}
			if err != nil {
				hadFailure = true
				r.service.loggerFor(ctx).Warn("记录 nxs 子任务 Goal usage 失败",
					"session_key", r.sessionKey,
					"goal_id", currentGoalID,
					"round_id", r.roundID,
					"task_id", child.taskID,
					"err", err,
				)
				continue
			}
			r.goalUsageMu.Lock()
			r.clearSubagentUsageObservationPendingLocked(child.taskID, observation)
			settledSnapshots = append(settledSnapshots, dmSubagentUsageSettlement{
				taskID:      child.taskID,
				observation: observation,
			})
			r.rememberSubagentUsageResultBindingLocked(result)
			r.goalUsageMu.Unlock()
		}
		r.goalUsageBindingMu.Unlock()
		if hadFailure {
			r.startGoalUsageRetryWorker()
		}
		return settledSnapshots
	}

	for _, child := range observations {
		r.markSubagentUsageObservationPending(child.taskID, child.observation)
	}
	r.goalUsageMu.Lock()
	goalID := strings.TrimSpace(r.childGoalIDForUsage)
	if goalID == "" {
		goalID = strings.TrimSpace(r.goalIDForUsage)
	}
	attributed := goalID != "" && !r.ignoreGoalRuntime()
	r.goalUsageMu.Unlock()
	for _, child := range observations {
		if r.service.runtime == nil {
			settledSnapshots = append(settledSnapshots, dmSubagentUsageSettlement{
				taskID:      child.taskID,
				observation: child.observation,
			})
			continue
		}
		delta := r.service.runtime.ObserveSubagentUsage(
			r.sessionKey,
			child.taskID,
			child.observation.cumulativeTotal,
		)
		if delta > 0 && attributed && r.service.goals != nil && !r.ignoreGoalRuntime() {
			// 兼容测试/非 SQL provider：TaskUsage 只有 provider actual total，
			// 没有 breakdown 时不得冒充预算 token。
			r.recordGoalUsageDelta(ctx, protocol.GoalUsage{ActualTotalTokens: delta})
		}
		settledSnapshots = append(settledSnapshots, dmSubagentUsageSettlement{
			taskID:      child.taskID,
			observation: child.observation,
		})
	}
	return settledSnapshots
}

func dmSubagentUsageObservations(
	runner *roundRunner,
	message protocol.Message,
) []dmSubagentUsageSettlement {
	usage := messageutil.SubagentTaskUsageSnapshots(message)
	observedAt := time.Now().UTC()
	observations := make([]dmSubagentUsageSettlement, 0, len(usage)+1)
	indexByTask := make(map[string]int, len(usage)+1)
	for _, child := range usage {
		taskID := strings.TrimSpace(child.TaskID)
		if taskID == "" || child.TotalTokens <= 0 {
			continue
		}
		indexByTask[taskID] = len(observations)
		observations = append(observations, dmSubagentUsageSettlement{
			taskID: taskID,
			observation: dmSubagentUsageObservation{
				cumulativeTotal: child.TotalTokens,
				observedAt:      observedAt,
			},
		})
	}

	metadata, _ := message["metadata"].(map[string]any)
	taskID := strings.TrimSpace(dmAnyString(metadata["task_id"]))
	if taskID == "" ||
		(!dmMetadataLooksLikeSubagentTask(metadata) &&
			(runner == nil || !runner.knowsSubagentTask(taskID))) {
		return observations
	}
	terminal := dmIsTerminalSubagentTaskStatus(dmAnyString(metadata["status"]))
	if index, exists := indexByTask[taskID]; exists {
		observations[index].observation.terminal =
			observations[index].observation.terminal || terminal
		observations[index].observation.terminalTokenUsageObserved =
			observations[index].observation.terminalTokenUsageObserved ||
				(terminal && observations[index].observation.cumulativeTotal > 0)
		return observations
	}
	observations = append(observations, dmSubagentUsageSettlement{
		taskID: taskID,
		observation: dmSubagentUsageObservation{
			terminal:   terminal,
			observedAt: observedAt,
		},
	})
	return observations
}

func (r *roundRunner) persistSubagentUsageObservation(
	ctx context.Context,
	recorder dmGoalUsageSourceRecorder,
	taskID string,
	observation dmSubagentUsageObservation,
) (protocol.GoalUsageSourceResult, error) {
	r.goalUsageBindingMu.Lock()
	defer r.goalUsageBindingMu.Unlock()
	r.goalUsageMu.Lock()
	snapshot := r.subagentUsageSourceSnapshotLocked(taskID, observation)
	r.goalUsageMu.Unlock()
	return recorder.RecordUsageSourceSnapshot(ctx, snapshot)
}

// persistSubagentUsageObservationLocked resolves the child Goal binding and
// persists the source snapshot under one lock. Callers must hold goalUsageMu so
// an external from-now bind cannot move the source between resolution and commit.
func (r *roundRunner) persistSubagentUsageObservationLocked(
	ctx context.Context,
	recorder dmGoalUsageSourceRecorder,
	taskID string,
	observation dmSubagentUsageObservation,
) (protocol.GoalUsageSourceResult, error) {
	return recorder.RecordUsageSourceSnapshot(
		ctx,
		r.subagentUsageSourceSnapshotLocked(taskID, observation),
	)
}

func (r *roundRunner) subagentUsageSourceSnapshotLocked(
	taskID string,
	observation dmSubagentUsageObservation,
) protocol.GoalUsageSourceSnapshot {
	goalID := strings.TrimSpace(r.childGoalIDForUsage)
	if goalID == "" {
		goalID = strings.TrimSpace(r.goalIDForUsage)
	}
	return protocol.GoalUsageSourceSnapshot{
		OwnerUserID:            r.ownerUserID,
		RuntimeSessionKey:      r.sessionKey,
		SourceKind:             protocol.GoalUsageSourceKindNXSTask,
		SourceID:               strings.TrimSpace(taskID),
		CumulativeActualTokens: observation.cumulativeTotal,
		EvidenceRequired:       true,
		Terminal:               observation.terminal,
		TokenUsageObserved:     observation.terminalTokenUsageObserved,
		GoalID:                 goalID,
		GoalSessionKey:         r.sessionKey,
		RoundID:                r.roundID,
		ScopeRoundID:           r.roundID,
		ObservedAt:             observation.observedAt,
	}
}

func (r *roundRunner) rememberSubagentUsageResultBindingLocked(
	result protocol.GoalUsageSourceResult,
) {
	if result.Goal == nil || strings.TrimSpace(result.Goal.ID) == "" {
		return
	}
	goalID := strings.TrimSpace(result.Goal.ID)
	if strings.TrimSpace(r.goalIDForUsage) == "" {
		r.goalIDForUsage = goalID
	}
	if strings.TrimSpace(r.childGoalIDForUsage) == "" {
		r.childGoalIDForUsage = goalID
	}
	r.goalUsageScopeConsumed = true
}

// flushPendingSubagentUsageBeforeBindLocked drains every observation that was
// already visible to this runner before external activation. Running children
// intentionally remain non-terminal evidence: BindUsageScopeFromNow owns the
// durable "baseline unavailable" decision and must see that evidence first.
func (r *roundRunner) flushPendingSubagentUsageBeforeBindLocked(
	ctx context.Context,
	recorder dmGoalUsageSourceRecorder,
) error {
	for taskID, observation := range r.subagentUsagePending {
		var (
			result protocol.GoalUsageSourceResult
			err    error
		)
		for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
			if attempt > 0 && !r.waitGoalUsagePersistRetry(ctx, attempt) {
				return ctx.Err()
			}
			result, err = r.persistSubagentUsageObservationLocked(
				ctx,
				recorder,
				taskID,
				observation,
			)
			if err == nil {
				break
			}
		}
		if err != nil {
			return err
		}
		r.clearSubagentUsageObservationPendingLocked(taskID, observation)
		r.rememberSubagentUsageResultBindingLocked(result)
	}
	return nil
}

// startGoalUsageRetryWorker 统一承载 child source checkpoint、round claim、
// parent terminal delta 与 completion fence 的后台收敛。每个 runner 同时最多一个。
func (r *roundRunner) startGoalUsageRetryWorker() {
	if r == nil {
		return
	}
	var recorder dmGoalUsageSourceRecorder
	if r.service != nil {
		recorder, _ = r.service.goals.(dmGoalUsageSourceRecorder)
	}
	r.goalUsageMu.Lock()
	if r.goalUsageRetryRunning {
		r.goalUsageMu.Unlock()
		return
	}
	r.goalUsageRetryRunning = true
	r.goalUsageMu.Unlock()
	go r.retryPendingGoalUsage(recorder)
}

func (r *roundRunner) retryPendingGoalUsage(recorder dmGoalUsageSourceRecorder) {
	delay := time.Duration(0)
	for {
		if delay > 0 {
			timer := time.NewTimer(delay)
			<-timer.C
		}
		pending, done, parentTerminal := r.pendingSubagentUsageForRetry()
		if done {
			if parentTerminal == "" {
				if r.stopGoalUsageRetryWhileParentActive() {
					return
				}
				delay = 0
				continue
			}
			if r.completeSubagentJoinAfterParentTerminal() {
				if r.stopGoalUsageRetryAfterSettlement() {
					return
				}
				delay = 0
				continue
			}
			delay = r.nextSubagentUsageRetryDelay(delay)
			continue
		}
		hadFailure := false
		for _, taskID := range pending {
			if recorder == nil {
				hadFailure = true
				continue
			}
			if err := r.retryPendingSubagentUsageObservation(
				context.Background(),
				recorder,
				taskID,
			); err != nil {
				hadFailure = true
			}
		}
		if hadFailure {
			delay = r.nextSubagentUsageRetryDelay(delay)
		} else {
			delay = 0
		}
	}
}

func (r *roundRunner) nextSubagentUsageRetryDelay(delay time.Duration) time.Duration {
	initialDelay := subagentUsageRetryInitialDelay
	if r != nil && r.goalUsageRetryBaseDelay > 0 {
		initialDelay = r.goalUsageRetryBaseDelay
	}
	if delay == 0 {
		return initialDelay
	}
	return min(delay*2, subagentUsageRetryMaxDelay)
}

// retryPendingSubagentUsageObservation re-reads the current pending value only
// after acquiring goalUsageMu. A stale retry copy must never survive a completed
// external bind and then persist against the new Goal.
func (r *roundRunner) retryPendingSubagentUsageObservation(
	ctx context.Context,
	recorder dmGoalUsageSourceRecorder,
	taskID string,
) error {
	r.goalUsageBindingMu.Lock()
	defer r.goalUsageBindingMu.Unlock()
	r.goalUsageMu.Lock()
	observation, ok := r.subagentUsagePending[strings.TrimSpace(taskID)]
	if !ok {
		r.goalUsageMu.Unlock()
		return nil
	}
	snapshot := r.subagentUsageSourceSnapshotLocked(taskID, observation)
	r.goalUsageMu.Unlock()
	result, err := recorder.RecordUsageSourceSnapshot(ctx, snapshot)
	if err != nil {
		return err
	}
	r.goalUsageMu.Lock()
	r.clearSubagentUsageObservationPendingLocked(taskID, observation)
	r.rememberSubagentUsageResultBindingLocked(result)
	r.goalUsageMu.Unlock()
	return nil
}

func (r *roundRunner) pendingSubagentUsageForRetry() (
	[]string,
	bool,
	string,
) {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if len(r.subagentUsagePending) == 0 {
		return nil, true, r.subagentParentTerminal
	}
	pending := make([]string, 0, len(r.subagentUsagePending))
	for taskID := range r.subagentUsagePending {
		pending = append(pending, taskID)
	}
	return pending, false, ""
}

// stopGoalUsageRetryWhileParentActive 只在 parent 仍未进入终态且没有 pending
// source 时退出。parent terminal 与 worker 退出共用同一把锁，避免终态刚标记、
// 旧 worker 却清掉 running 标记后无人接力。
func (r *roundRunner) stopGoalUsageRetryWhileParentActive() bool {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if len(r.subagentUsagePending) > 0 || r.subagentParentTerminal != "" {
		return false
	}
	r.goalUsageRetryRunning = false
	return true
}

func (r *roundRunner) stopGoalUsageRetryAfterSettlement() bool {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if len(r.subagentTasks) > 0 || len(r.subagentUsagePending) > 0 {
		return false
	}
	r.goalUsageRetryRunning = false
	return true
}

func (r *roundRunner) elapsedGoalUsageSeconds() int64 {
	if r.goalUsageStarted.IsZero() {
		return 0
	}
	elapsed := int64(time.Since(r.goalUsageStarted).Seconds())
	return max(elapsed, 0)
}

func (r *roundRunner) ignoreGoalRuntime() bool {
	if r == nil {
		return false
	}
	return goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(r.permissionMode))
}

func isZeroGoalUsage(usage protocol.GoalUsage) bool {
	return usage.InputTokens == 0 &&
		usage.OutputTokens == 0 &&
		usage.CacheCreationInputTokens == 0 &&
		usage.CacheReadInputTokens == 0 &&
		usage.ReasoningTokens == 0 &&
		usage.TotalTokens == 0 &&
		usage.BudgetTotalTokens == 0 &&
		usage.ActualTotalTokens == 0 &&
		usage.RuntimeSeconds == 0
}
