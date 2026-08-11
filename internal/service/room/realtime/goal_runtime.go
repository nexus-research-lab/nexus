// INPUT: Room slot 的 Goal 上下文、objective revision、parent/child usage 与运行结果。
// OUTPUT: slot 级 Goal accounting、跨 runtime 的 root-scope child 回补、共享 finalization barrier、协作证据和 objective steering。
// POS: Room runtime 与共享 Goal 领域之间的唯一投影入口。
package realtime

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
)

const (
	goalContextualInputName    = "goal"
	goalUsagePersistAttempts   = 5
	roomGoalUsageRetryMaxDelay = 5 * time.Second
)

type roomSubagentUsageSettlement struct {
	taskID          string
	cumulativeTotal int64
	observation     roomSubagentUsageObservation
}

type roomGoalUsageSourceRecorder interface {
	RecordUsageSourceSnapshot(context.Context, protocol.GoalUsageSourceSnapshot) (protocol.GoalUsageSourceResult, error)
}

type roomGoalUsageParentRecorder interface {
	RecordUsageParentSnapshot(context.Context, protocol.GoalUsageParentSnapshot) (protocol.GoalUsageParentResult, error)
}

type roomGoalUsageScopeBinder interface {
	BindUsageScopeFromNow(context.Context, protocol.GoalUsageScopeBinding) (protocol.GoalUsageScopeBindResult, error)
}

// QueueRoomContextualGuidanceInput 把共享 Goal steering 分发到每个活跃 slot，并排除产生 retarget 的 caller。
func (s *Service) QueueRoomContextualGuidanceInput(
	ctx context.Context,
	sessionKey string,
	roundID string,
	contextName string,
	content string,
	excludedAgentID string,
	objectiveRevision int64,
) ([]string, error) {
	if s == nil || s.runtime == nil {
		return nil, runtimectx.ErrNoRunningRound
	}
	sessionKey = strings.TrimSpace(sessionKey)
	excludedAgentID = strings.TrimSpace(excludedAgentID)
	targets := map[string]*activeRoomSlot{}
	for _, roundValue := range s.rounds.snapshot() {
		if roundValue == nil || strings.TrimSpace(roundValue.SessionKey) != sessionKey {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || (excludedAgentID != "" && strings.TrimSpace(slot.AgentID) == excludedAgentID) {
				continue
			}
			// Session affinity is not Goal authority. Only a round that already
			// carries an immutable Goal mutation capability may receive retarget
			// steering; ordinary chat/raw @ slots remain untouched.
			if !slot.goalMutationAuthority().valid() {
				continue
			}
			if runtimeSessionKey := strings.TrimSpace(slot.RuntimeSessionKey); runtimeSessionKey != "" {
				targets[runtimeSessionKey] = slot
			}
		}
	}

	roundIDs := map[string]struct{}{}
	var queueErrors []error
	for _, runtimeSessionKey := range slices.Sorted(maps.Keys(targets)) {
		slot := targets[runtimeSessionKey]
		var onConsumed func()
		if objectiveRevision > 0 {
			onConsumed = func() {
				slot.adoptGoalObjectiveRevision(objectiveRevision)
			}
		}
		queued, err := s.runtime.QueueContextualGuidanceInputOnConsumed(ctx, runtimeSessionKey, roundID, contextName, content, onConsumed)
		if err != nil {
			if errors.Is(err, runtimectx.ErrNoRunningRound) {
				continue
			}
			queueErrors = append(queueErrors, fmt.Errorf("queue Room Goal guidance for %s: %w", runtimeSessionKey, err))
			continue
		}
		for _, queuedRoundID := range queued {
			roundIDs[queuedRoundID] = struct{}{}
		}
	}
	if len(roundIDs) == 0 {
		if err := errors.Join(queueErrors...); err != nil {
			return nil, err
		}
		return nil, runtimectx.ErrNoRunningRound
	}
	return slices.Sorted(maps.Keys(roundIDs)), errors.Join(queueErrors...)
}

// GoalObjectiveRevisionState 返回指定 Room slot 与 MCP server 共用的 objective revision 状态。
func (s *Service) GoalObjectiveRevisionState(
	sessionKey string,
	roundID string,
	agentID string,
	initial int64,
) *atomic.Int64 {
	if s == nil {
		return nil
	}
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	agentID = strings.TrimSpace(agentID)
	var target *activeRoomSlot
	for _, roundValue := range s.rounds.snapshot() {
		if roundValue == nil || strings.TrimSpace(roundValue.SessionKey) != sessionKey {
			continue
		}
		if roundID != "" && roomRootRoundID(roundValue) != roundID && strings.TrimSpace(roundValue.RoundID) != roundID {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot != nil && strings.TrimSpace(slot.AgentID) == agentID {
				target = slot
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		return nil
	}
	return target.ensureGoalObjectiveRevision(initial)
}

func goalContextualInputs(contextText string, goalID string, sessionKey string) []runtimectx.ContextualInputBlock {
	contextText = strings.TrimSpace(contextText)
	if contextText == "" {
		return nil
	}
	metadata := map[string]string{}
	if goalID = strings.TrimSpace(goalID); goalID != "" {
		metadata["goal_id"] = goalID
	}
	if sessionKey = strings.TrimSpace(sessionKey); sessionKey != "" {
		metadata["session_key"] = sessionKey
	}
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock(goalContextualInputName, contextText, 0, metadata),
	}
}

func (s *Service) resolveGoalRuntimeContextForSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	appendSystemPrompt string,
) (string, string, string, string, int64) {
	defaultGoalSessionKey := ""
	if roundValue != nil {
		defaultGoalSessionKey = strings.TrimSpace(roundValue.SessionKey)
	}
	for _, sessionKey := range goalSessionCandidates(roundValue, slot) {
		goalContext, goalID, objectiveRevision, ok := s.goalRuntimeContext(ctx, sessionKey)
		if !ok {
			continue
		}
		if slot != nil {
			slot.ensureGoalObjectiveRevision(objectiveRevision)
		}
		return appendSystemPrompt, goalContext, goalID, sessionKey, objectiveRevision
	}
	return appendSystemPrompt, "", "", defaultGoalSessionKey, 0
}

func goalSessionCandidates(roundValue *activeRoomRound, slot *activeRoomSlot) []string {
	candidates := []string{}
	if roundValue != nil {
		roundSessionKey := strings.TrimSpace(roundValue.SessionKey)
		if protocol.IsRoomSharedSessionKey(roundSessionKey) {
			return []string{roundSessionKey}
		}
		candidates = append(candidates, roundSessionKey)
	}
	if slot != nil {
		candidates = append(candidates, slot.RuntimeSessionKey)
	}
	result := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		sessionKey := strings.TrimSpace(candidate)
		if sessionKey == "" {
			continue
		}
		if _, exists := seen[sessionKey]; exists {
			continue
		}
		seen[sessionKey] = struct{}{}
		result = append(result, sessionKey)
	}
	return result
}

func (s *Service) goalRuntimeContext(ctx context.Context, sessionKey string) (string, string, int64, bool) {
	goalContext, goal, ok := s.goalRuntimeSnapshot(ctx, sessionKey)
	if !ok {
		return "", "", 0, false
	}
	goalID := ""
	objectiveRevision := int64(0)
	if goal != nil {
		goalID = strings.TrimSpace(goal.ID)
		objectiveRevision = goal.ObjectiveRevision()
	}
	return goalContext, goalID, objectiveRevision, true
}

func (s *Service) goalRuntimeSnapshot(
	ctx context.Context,
	sessionKey string,
) (string, *protocol.Goal, bool) {
	if s.goals == nil {
		return "", nil, false
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) {
			return "", nil, false
		}
		s.loggerFor(ctx).Warn("读取 Room Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return "", nil, false
	}
	if strings.TrimSpace(goalContext) == "" {
		return "", goal, true
	}
	return strings.TrimSpace(goalContext), goal, true
}

func (s *Service) recordGoalContinuationProgressForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	roundValue *activeRoomRound,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	authority := slot.goalMutationAuthority()
	if !authority.valid() {
		return
	}
	goalID := authority.GoalID
	s.recordRoomGoalCollaborationEvidenceForSlot(ctx, slot, finalAssistant)
	purpose := ""
	if roundValue != nil {
		purpose = strings.TrimSpace(roundValue.InputOptions.Purpose)
	}
	if purpose == "goal_continuation" && result.TerminalStatus == "error" {
		reason := cmp.Or(
			strings.TrimSpace(result.ErrorMessage),
			messageutil.ExtractAssistantDisplayText(finalAssistant),
			"Goal continuation runtime failed",
		)
		s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 续跑失败原因失败", func() error {
			_, err := s.goals.RecordContinuationFailure(ctx, goalID, slot.AgentRoundID, reason, authority.ObjectiveRevision)
			return err
		})
		return
	}
	if purpose != "goal_continuation" {
		s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 显式活动失败", func() error {
			_, err := s.goals.RecordGoalActivity(ctx, goalID, slot.AgentRoundID, authority.ObjectiveRevision)
			return err
		})
		return
	}
	if messageutil.AssistantMissedGoalCompletionTool(finalAssistant) {
		reason := "assistant claimed goal completion but did not call mcp__nexus_goal__update_goal"
		s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 完成工具漏调用失败", func() error {
			_, err := s.goals.RecordCompletionToolMiss(ctx, goalID, slot.AgentRoundID, reason, authority.ObjectiveRevision)
			return err
		})
		return
	}
	hasProgress := slotHasGoalToolProgress(slot)
	if !hasProgress && slot.hasRunningSubagentTask() {
		return
	}
	s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 续跑进展失败", func() error {
		_, err := s.goals.RecordContinuationProgress(ctx, goalID, slot.AgentRoundID, hasProgress, authority.ObjectiveRevision)
		return err
	}, "progressed", hasProgress)
}

func (s *Service) recordSlotGoalMutation(
	ctx context.Context,
	slot *activeRoomSlot,
	logMessage string,
	mutation func() error,
	fields ...any,
) {
	err := mutation()
	if err == nil || goalsvc.IsExpectedMutationError(err) {
		return
	}
	baseFields := []any{
		"session_key", goalSessionKeyForSlot(slot),
		"goal_id", slot.goalIDForUsage(),
		"round_id", slot.AgentRoundID,
	}
	baseFields = append(baseFields, fields...)
	baseFields = append(baseFields, "err", err)
	s.loggerFor(ctx).Warn(logMessage, baseFields...)
}

func (s *Service) recordRoomGoalCollaborationEvidenceForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	finalAssistant protocol.Message,
) {
	if s == nil || s.goals == nil || slot == nil || !protocol.IsRoomSharedSessionKey(goalSessionKeyForSlot(slot)) {
		return
	}
	authority := slot.goalMutationAuthority()
	if !authority.valid() {
		return
	}
	if roomdomain.IsNoReplyAssistantMessage(finalAssistant) {
		return
	}
	if strings.TrimSpace(messageutil.ExtractAssistantDisplayText(finalAssistant)) == "" {
		return
	}
	s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 协作证据失败", func() error {
		_, err := s.goals.RecordRoomGoalCollaborationEvidence(ctx, authority.GoalID, slot.AgentRoundID, slot.AgentID, authority.ObjectiveRevision)
		return err
	}, "agent_id", slot.AgentID)
}

func rememberGoalToolProgressForSlot(slot *activeRoomSlot, progressed bool) {
	if slot == nil || !progressed {
		return
	}
	slot.markGoalToolProgress()
}

func slotHasGoalToolProgress(slot *activeRoomSlot) bool {
	if slot == nil {
		return false
	}
	return slot.hasGoalToolProgress()
}

func goalSessionKeyForSlot(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	if sessionKey := strings.TrimSpace(slot.goalSessionKey()); sessionKey != "" {
		return sessionKey
	}
	return strings.TrimSpace(slot.RuntimeSessionKey)
}

func goalUsageOwnerUserIDForRoomSlot(ctx context.Context, slot *activeRoomSlot) string {
	if slot != nil {
		if ownerUserID := strings.TrimSpace(slot.OwnerUserID); ownerUserID != "" {
			return ownerUserID
		}
	}
	return authctx.OwnerUserID(ctx)
}

func goalUsageScopeRoundIDForRoomSlot(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	if scopeRoundID := strings.TrimSpace(slot.GoalUsageScopeRoundID); scopeRoundID != "" {
		return scopeRoundID
	}
	return strings.TrimSpace(slot.AgentRoundID)
}

func goalUsageSessionKeyForRoomSlot(slot *activeRoomSlot, candidate string) string {
	// 与 Goal MCP session 解析保持同一边界：group Room 聚合到共享流，
	// private/DM Room 继续使用各 Agent 自己的 Goal session。
	normalize := func(raw string) string {
		sessionKey := strings.TrimSpace(raw)
		if sessionKey == "" {
			return ""
		}
		parsed := protocol.ParseSessionKey(sessionKey)
		if parsed.Kind == protocol.SessionKeyKindAgent &&
			parsed.ChatType == "group" &&
			strings.TrimSpace(parsed.Ref) != "" {
			return protocol.BuildRoomSharedSessionKey(parsed.Ref)
		}
		return sessionKey
	}
	if sessionKey := normalize(candidate); sessionKey != "" {
		return sessionKey
	}
	if slot == nil {
		return ""
	}
	if sessionKey := normalize(slot.goalSessionKey()); sessionKey != "" {
		return sessionKey
	}
	return normalize(slot.RuntimeSessionKey)
}

func beginGoalUsageForSlot(slot *activeRoomSlot) {
	if slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	slot.beginGoalUsage()
}

func (s *Service) registerSlotGoalRuntime(slot *activeRoomSlot) func() {
	if s.runtime == nil || slot == nil || slot.goalRuntimeIgnored() {
		return func() {}
	}
	sessionKey := goalSessionKeyForSlot(slot)
	roundID := strings.TrimSpace(slot.AgentRoundID)
	if sessionKey == "" || roundID == "" {
		return func() {}
	}
	s.runtime.RegisterGoalAccountingFlush(sessionKey, roundID, func(ctx context.Context) error {
		return s.flushGoalUsageForSlot(ctx, slot)
	})
	s.runtime.RegisterGoalAccountingClear(sessionKey, roundID, func() {
		clearGoalUsageForSlot(slot)
	})
	s.runtime.RegisterGoalAccountingFinalize(sessionKey, roundID, func() bool {
		if _, ok := s.goals.(roomGoalUsageFinalizationProvider); !ok {
			return false
		}
		return slot.beginGoalUsageFinalizing()
	})
	s.runtime.RegisterGoalAccountingActivate(sessionKey, roundID, func(ctx context.Context, goalID string) error {
		return s.activateGoalUsageForSlot(ctx, slot, goalID)
	})
	s.runtime.RegisterGoalAccountingCreateGuard(
		sessionKey,
		roundID,
		goalUsageScopeRoundIDForRoomSlot(slot),
		slot.goalUsageScopeConsumed,
	)
	return func() {
		s.runtime.RegisterGoalAccountingFlush(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingClear(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingFinalize(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingActivate(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingCreateGuard(sessionKey, roundID, "", nil)
	}
}

func (s *Service) recordGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	snapshot, ok := slotFinalGoalUsageSnapshot(slot, result, finalAssistant)
	if !ok {
		return
	}
	_ = s.settleTerminalGoalUsageSnapshotForSlotWithRetry(ctx, slot, snapshot)
}

func (s *Service) finalizeGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	snapshot, _ := slotFinalGoalUsageSnapshot(slot, result, finalAssistant)
	settled := s.settleTerminalGoalUsageSnapshotForSlotWithRetry(ctx, slot, snapshot)
	slot.setGoalUsageTerminalSettled(settled)
	if !settled {
		s.loggerFor(ctx).Warn(
			"Room terminal Goal usage 未能持久化",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", slot.goalIDForUsage(),
			"round_id", slot.AgentRoundID,
		)
		return
	}
	closeGoalUsageForSlot(slot)
}

func (s *Service) settleTerminalGoalUsageSnapshotForSlotWithRetry(
	ctx context.Context,
	slot *activeRoomSlot,
	snapshot goalsvc.RuntimeUsageSnapshot,
) bool {
	for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
		if attempt > 0 && !s.waitRoomGoalUsagePersistRetry(ctx, attempt) {
			return false
		}
		if s.settleTerminalGoalUsageSnapshotForSlot(ctx, slot, snapshot) {
			return true
		}
	}
	return false
}

func (s *Service) waitRoomGoalUsagePersistRetry(ctx context.Context, attempt int) bool {
	baseDelay := 20 * time.Millisecond
	if s != nil && s.goalUsageRetryBaseDelay > 0 {
		baseDelay = s.goalUsageRetryBaseDelay
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

func (s *Service) recordGoalUsageLimitForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() || !result.UsageLimitReached {
		return
	}
	goalID := strings.TrimSpace(slot.goalIDForUsage())
	var err error
	if goalID != "" {
		if provider, ok := s.goals.(interface {
			UsageLimitForGoal(context.Context, string, string, string) (*protocol.Goal, error)
		}); ok {
			_, err = provider.UsageLimitForGoal(ctx, goalID, slot.AgentRoundID, result.UsageLimitReason)
		} else {
			_, err = s.goals.UsageLimitForSession(ctx, goalSessionKeyForSlot(slot), slot.AgentRoundID, result.UsageLimitReason)
		}
	} else {
		_, err = s.goals.UsageLimitForSession(ctx, goalSessionKeyForSlot(slot), slot.AgentRoundID, result.UsageLimitReason)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) {
		s.loggerFor(ctx).Warn("标记 Room Goal usage limit 失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", goalID,
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
}

func (s *Service) flushGoalUsageForSlot(ctx context.Context, slot *activeRoomSlot) error {
	snapshot, ok := slotFinalGoalUsageSnapshot(slot, exec.RoundExecutionResult{}, slot.lastGoalAssistantMessage())
	if !ok {
		return nil
	}
	// Goal mutation flush 是 mid-round checkpoint，不得提前把 estimated actual
	// 当成 terminal 真值，否则后续 provider exact total 无法向下校准。
	snapshot.Terminal = false
	snapshot.SettlementBoundary = goalsvc.RuntimeUsageSettlementBoundary(ctx)
	if s.tryRecordGoalUsageSnapshotForSlot(ctx, slot, snapshot) {
		return nil
	}
	return errors.New("persist Room Goal usage checkpoint")
}

func (s *Service) recordGoalUsageFromSlotAssistantMessage(
	ctx context.Context,
	slot *activeRoomSlot,
	message protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	if protocol.MessageRole(message) != "assistant" {
		return
	}
	observations := messageutil.AssistantToolResults(message)
	if len(observations) > 0 {
		rememberGoalToolProgressForSlot(slot, messageutil.AssistantHasCountedToolProgress(message))
	}
	snapshot := slotAssistantGoalUsageSnapshot(slot, message)
	hasSuccessfulCreate := false
	hasSuccessfulUpdate := false
	for _, observation := range observations {
		if observation.IsError ||
			observation.MutationOutcome == protocol.MutationResultRejected {
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
		s.startRoomGoalUsageFromRoundStartForScope(ctx, slot, goalSessionKeyForSlot(slot))
		goalID, goalSessionKey := s.ensureModelCreatedRoomGoalBinding(ctx, slot)
		s.claimSubagentGoalUsageForRoomScope(ctx, slot, goalID, goalSessionKey)
		_ = s.tryRecordGoalUsageSnapshotForSlotInScope(ctx, slot, snapshot, slot)
	} else {
		s.recordGoalUsageSnapshotForSlot(ctx, slot, snapshot)
	}
	if hasSuccessfulUpdate {
		// update_goal 返回时各 slot 的 terminal usage 尚未齐全；只冻结 Goal
		// 绑定，等每个 slot 的 round terminal 完成对账后再分别关闭。
		s.beginRoomGoalUsageFinalizing(goalSessionKeyForSlot(slot))
	}
}

func slotFinalGoalUsageSnapshot(
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) (goalsvc.RuntimeUsageSnapshot, bool) {
	usage, resultUsagePresent := runtimectx.GoalUsageFromTokenUsageWithPresence(result.Usage)
	cumulative := resultUsagePresent
	usageOK := resultUsagePresent
	if !resultUsagePresent && protocol.MessageRole(finalAssistant) == "assistant" {
		usage, usageOK = runtimectx.GoalUsageFromRaw(finalAssistant["usage"])
	}
	elapsedSeconds := result.ElapsedTimeSeconds
	if elapsedSeconds <= 0 {
		elapsedSeconds = slotGoalUsageElapsedSeconds(slot)
	}
	return goalsvc.RuntimeUsageSnapshot{
		Usage:              usage,
		ElapsedSeconds:     elapsedSeconds,
		TokenUsageObserved: usageOK,
		TurnID:             strings.TrimSpace(anyString(finalAssistant["message_id"])),
		Cumulative:         cumulative,
		Terminal:           true,
	}, usageOK || elapsedSeconds > 0
}

func slotAssistantGoalUsageSnapshot(slot *activeRoomSlot, message protocol.Message) goalsvc.RuntimeUsageSnapshot {
	usage, usageObserved := runtimectx.GoalUsageFromRaw(message["usage"])
	return goalsvc.RuntimeUsageSnapshot{
		Usage:              usage,
		ElapsedSeconds:     slotGoalUsageElapsedSeconds(slot),
		TokenUsageObserved: usageObserved,
		TurnID:             strings.TrimSpace(anyString(message["message_id"])),
	}
}

func (s *Service) recordGoalUsageSnapshotForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	snapshot goalsvc.RuntimeUsageSnapshot,
) {
	_ = s.tryRecordGoalUsageSnapshotForSlot(ctx, slot, snapshot)
}

func (s *Service) tryRecordGoalUsageSnapshotForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	snapshot goalsvc.RuntimeUsageSnapshot,
) bool {
	return s.tryRecordGoalUsageSnapshotForSlotInScope(ctx, slot, snapshot, nil)
}

func (s *Service) tryRecordGoalUsageSnapshotForSlotInScope(
	ctx context.Context,
	slot *activeRoomSlot,
	snapshot goalsvc.RuntimeUsageSnapshot,
	scopeOrigin *activeRoomSlot,
) bool {
	if s.goals == nil || slot == nil {
		return true
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.runtimeIgnored {
		slot.mutable.goal.mu.Unlock()
		return true
	}
	goalID := strings.TrimSpace(slot.mutable.goal.idForUsage)
	goalSessionKey := strings.TrimSpace(slot.mutable.goal.sessionKey)
	if goalSessionKey == "" {
		goalSessionKey = strings.TrimSpace(slot.RuntimeSessionKey)
	}
	if slot.mutable.goal.usage != nil {
		usage, ok := slot.mutable.goal.usage.PrepareDelta(snapshot)
		if ok {
			updated, persisted := s.persistGoalUsageDeltaForSlotTarget(ctx, slot, usage, goalID, goalSessionKey)
			if persisted {
				slot.mutable.goal.usage.CommitDelta(usage)
			}
			slot.mutable.goal.mu.Unlock()
			if updated != nil && goalID == "" {
				s.bindRecordedRoomGoalUsage(scopeOrigin, updated)
			}
			return persisted
		}
		slot.mutable.goal.mu.Unlock()
		return true
	}
	usage := snapshot.Usage
	usage.RuntimeSeconds = snapshot.ElapsedSeconds
	if isZeroRoomGoalUsage(usage) {
		slot.mutable.goal.mu.Unlock()
		return true
	}
	updated, persisted := s.persistGoalUsageDeltaForSlotTarget(ctx, slot, usage, goalID, goalSessionKey)
	slot.mutable.goal.mu.Unlock()
	if updated != nil && goalID == "" {
		s.bindRecordedRoomGoalUsage(scopeOrigin, updated)
	}
	return persisted
}

func (s *Service) bindRecordedRoomGoalUsage(
	scopeOrigin *activeRoomSlot,
	updated *protocol.Goal,
) {
	if updated == nil {
		return
	}
	if scopeOrigin != nil {
		s.bindRoomGoalUsageForScope(scopeOrigin, updated.SessionKey, updated.ID)
		return
	}
	s.bindRoomGoalUsage(updated.SessionKey, updated.ID)
}

func (s *Service) settleTerminalGoalUsageSnapshotForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	snapshot goalsvc.RuntimeUsageSnapshot,
) bool {
	if s.goals == nil || slot == nil {
		return true
	}
	snapshot.Terminal = true
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.runtimeIgnored {
		slot.mutable.goal.mu.Unlock()
		return true
	}

	var (
		usage                   protocol.GoalUsage
		hasDelta                bool
		unboundTerminalEligible bool
		tokenUsageObserved      = snapshot.TokenUsageObserved
	)
	if slot.mutable.goal.usage != nil {
		unboundTerminalEligible = slot.mutable.goal.usage.EligibleForUnboundTerminal()
		usage, hasDelta = slot.mutable.goal.usage.PrepareDelta(snapshot)
		tokenUsageObserved = slot.mutable.goal.usage.TokenUsageObserved()
	} else {
		usage = snapshot.Usage
		usage.RuntimeSeconds = snapshot.ElapsedSeconds
		hasDelta = !isZeroRoomGoalUsage(usage)
	}
	goalID := strings.TrimSpace(slot.mutable.goal.idForUsage)
	goalSessionKey := strings.TrimSpace(slot.mutable.goal.sessionKey)
	if goalSessionKey == "" {
		goalSessionKey = strings.TrimSpace(slot.RuntimeSessionKey)
	}
	if goalID == "" && unboundTerminalEligible {
		// 从未绑定过 Goal 的 Room slot 要把完整 terminal 证据暂存在 root
		// scope；后继 handoff slot 中的 model Create 会原子认领。已经
		// Reset/Close 的 accumulator 不具备该资格，避免 settlement 后整轮重算。
		usage = snapshot.Usage
		usage.RuntimeSeconds = snapshot.ElapsedSeconds
		hasDelta = !isZeroRoomGoalUsage(usage)
	}

	recorder, durable := s.goals.(roomGoalUsageParentRecorder)
	if durable && (goalID != "" || unboundTerminalEligible) {
		result, err := recorder.RecordUsageParentSnapshot(ctx, protocol.GoalUsageParentSnapshot{
			OwnerUserID:        goalUsageOwnerUserIDForRoomSlot(ctx, slot),
			GoalSessionKey:     goalUsageSessionKeyForRoomSlot(slot, goalSessionKey),
			ScopeRoundID:       goalUsageScopeRoundIDForRoomSlot(slot),
			SourceRoundID:      strings.TrimSpace(slot.AgentRoundID),
			GoalID:             goalID,
			Usage:              usage,
			TokenUsageObserved: tokenUsageObserved,
		})
		if err != nil && !errors.Is(err, goalsvc.ErrGoalInvalidState) {
			slot.mutable.goal.mu.Unlock()
			return false
		}
		if err == nil {
			if slot.mutable.goal.usage != nil && hasDelta {
				slot.mutable.goal.usage.CommitDelta(usage)
			}
			slot.mutable.goal.mu.Unlock()
			if result.Goal != nil {
				s.bindRoomGoalUsageForScope(slot, result.Goal.SessionKey, result.Goal.ID)
			}
			return true
		}
		// 兼容非 SQL 测试/嵌入 provider：Goal service 暴露方法，但底层
		// repository 没有 durable ledger capability 时继续走 legacy delta。
	}
	if goalID == "" && !unboundTerminalEligible {
		slot.mutable.goal.mu.Unlock()
		return true
	}
	if !hasDelta {
		slot.mutable.goal.mu.Unlock()
		return true
	}
	if _, persisted := s.persistGoalUsageDeltaForSlotTarget(
		ctx,
		slot,
		usage,
		goalID,
		goalSessionKey,
	); !persisted {
		slot.mutable.goal.mu.Unlock()
		return false
	}
	if slot.mutable.goal.usage != nil {
		slot.mutable.goal.usage.CommitDelta(usage)
	}
	slot.mutable.goal.mu.Unlock()
	return true
}

type roomGoalUsageFinalizationProvider interface {
	UsageByGoalID(context.Context, string) (*protocol.GoalUsageReport, error)
	FinalizeUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
}

// finalizeCompletedRoomGoalUsage 只在同一共享 session 的全部 parent slot
// terminal usage 已落库且 child source 已 drain 后建立一次最终 fence。
func (s *Service) finalizeCompletedRoomGoalUsage(
	ctx context.Context,
	anchor *activeRoomRound,
) bool {
	if s == nil || s.goals == nil || anchor == nil {
		return true
	}
	sessionKey := strings.TrimSpace(anchor.SessionKey)
	goalRounds := make(map[string]string)
	for _, slot := range anchor.Slots {
		if slot == nil {
			continue
		}
		goalID := strings.TrimSpace(slot.childGoalIDForUsage())
		if goalID == "" {
			continue
		}
		roundID := strings.TrimSpace(slot.AgentRoundID)
		if roundID == "" {
			roundID = strings.TrimSpace(anchor.RoundID)
		}
		goalRounds[goalID] = roundID
	}
	if len(goalRounds) == 0 {
		return true
	}
	rounds := []*activeRoomRound{anchor}
	seenRounds := map[*activeRoomRound]struct{}{anchor: {}}
	for _, candidate := range s.rounds.snapshot() {
		if candidate == nil || strings.TrimSpace(candidate.SessionKey) != sessionKey {
			continue
		}
		if _, exists := seenRounds[candidate]; exists {
			continue
		}
		seenRounds[candidate] = struct{}{}
		rounds = append(rounds, candidate)
	}

	for _, roundValue := range rounds {
		if roundValue == nil {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil {
				continue
			}
			goalID := strings.TrimSpace(slot.childGoalIDForUsage())
			if _, belongsToAnchorGoal := goalRounds[goalID]; !belongsToAnchorGoal {
				continue
			}
			if slot.goalUsageClaimPending() {
				if !s.claimSubagentGoalUsageForRoomSlot(
					ctx,
					slot,
					goalID,
					goalSessionKeyForSlot(slot),
				) {
					return false
				}
			}
			if !slot.isTerminal() || slot.hasRunningSubagentTask() {
				return false
			}
			if !slot.goalUsageTerminalSettled() {
				if !s.settleTerminalGoalUsageSnapshotForSlotWithRetry(
					ctx,
					slot,
					goalsvc.RuntimeUsageSnapshot{Terminal: true},
				) {
					return false
				}
				slot.setGoalUsageTerminalSettled(true)
				closeGoalUsageForSlot(slot)
			}
		}
	}
	finalizer, ok := s.goals.(roomGoalUsageFinalizationProvider)
	if !ok {
		return true
	}
	for goalID, roundID := range goalRounds {
		if !s.finalizeCompletedRoomGoalWithRetry(ctx, finalizer, goalID, roundID) {
			return false
		}
	}
	return true
}

func (s *Service) finalizeCompletedRoomGoalWithRetry(
	ctx context.Context,
	finalizer roomGoalUsageFinalizationProvider,
	goalID string,
	roundID string,
) bool {
	for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
		if attempt > 0 && !s.waitRoomGoalUsagePersistRetry(ctx, attempt) {
			return false
		}
		report, err := finalizer.UsageByGoalID(ctx, goalID)
		if err != nil {
			continue
		}
		if report == nil || protocol.NormalizeGoalStatus(report.Status) != protocol.GoalStatusComplete {
			return true
		}
		if report.UsageFinalized {
			return true
		}
		if _, err = finalizer.FinalizeUsageForGoal(
			ctx,
			goalID,
			protocol.GoalUsage{},
			roundID,
		); err == nil {
			return true
		} else if errors.Is(err, goalsvc.ErrGoalUsageUnavailable) {
			// Durable parent ledger 已证明 provider usage 缺失。这是终态真相，
			// 不是可恢复写失败：保留 usage_finalized=false 并释放业务收尾。
			return true
		}
	}
	return false
}

func (s *Service) recordGoalUsageDeltaForSlot(ctx context.Context, slot *activeRoomSlot, usage protocol.GoalUsage) *protocol.Goal {
	if s.goals == nil || slot == nil || isZeroRoomGoalUsage(usage) {
		return nil
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.runtimeIgnored {
		slot.mutable.goal.mu.Unlock()
		return nil
	}
	goalID := strings.TrimSpace(slot.mutable.goal.idForUsage)
	goalSessionKey := strings.TrimSpace(slot.mutable.goal.sessionKey)
	if goalSessionKey == "" {
		goalSessionKey = strings.TrimSpace(slot.RuntimeSessionKey)
	}
	updated := s.recordGoalUsageDeltaForSlotTarget(ctx, slot, usage, goalID, goalSessionKey)
	slot.mutable.goal.mu.Unlock()
	if updated != nil && goalID == "" {
		s.bindRoomGoalUsageForScope(slot, updated.SessionKey, updated.ID)
	}
	return updated
}

// recordGoalUsageDeltaForSlotTarget 在 slot Goal 锁内使用产生 delta 时的固定绑定。
// 它不回读 slot Goal 状态，也不执行跨 slot 绑定，避免在临界区内递归加锁。
func (s *Service) recordGoalUsageDeltaForSlotTarget(
	ctx context.Context,
	slot *activeRoomSlot,
	usage protocol.GoalUsage,
	goalID string,
	goalSessionKey string,
) *protocol.Goal {
	updated, _ := s.persistGoalUsageDeltaForSlotTarget(ctx, slot, usage, goalID, goalSessionKey)
	return updated
}

func (s *Service) persistGoalUsageDeltaForSlotTarget(
	ctx context.Context,
	slot *activeRoomSlot,
	usage protocol.GoalUsage,
	goalID string,
	goalSessionKey string,
) (*protocol.Goal, bool) {
	if s.goals == nil || slot == nil || isZeroRoomGoalUsage(usage) {
		return nil, false
	}
	var err error
	var updated *protocol.Goal
	if goalID != "" {
		updated, err = s.goals.RecordUsageForGoal(ctx, goalID, usage, slot.AgentRoundID)
	} else {
		updated, err = s.goals.RecordUsageForSession(ctx, goalSessionKey, usage, slot.AgentRoundID)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
		s.loggerFor(ctx).Warn("记录 Room Goal usage 失败",
			"session_key", goalSessionKey,
			"goal_id", goalID,
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
	if err != nil || updated == nil {
		return nil, err == nil
	}
	return updated, true
}

func (s *Service) bindRoomGoalUsage(sessionKey string, goalID string) {
	if s == nil {
		return
	}
	sessionKey = strings.TrimSpace(sessionKey)
	goalID = strings.TrimSpace(goalID)
	if sessionKey == "" || goalID == "" {
		return
	}
	for _, roundValue := range s.rounds.snapshot() {
		if roundValue == nil || strings.TrimSpace(roundValue.SessionKey) != sessionKey {
			continue
		}
		for _, candidate := range roundValue.Slots {
			if candidate != nil {
				candidate.setGoalBinding(sessionKey, goalID)
			}
		}
	}
}

func (s *Service) roomGoalUsageSlotsForScope(
	origin *activeRoomSlot,
	goalSessionKey string,
) []*activeRoomSlot {
	if s == nil || origin == nil {
		return nil
	}
	scopeRoundID := goalUsageScopeRoundIDForRoomSlot(origin)
	goalSessionKey = goalUsageSessionKeyForRoomSlot(origin, goalSessionKey)
	if scopeRoundID == "" || goalSessionKey == "" {
		return []*activeRoomSlot{origin}
	}
	slots := make([]*activeRoomSlot, 0)
	seen := make(map[*activeRoomSlot]struct{})
	appendSlot := func(candidate *activeRoomSlot) {
		if candidate == nil {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		if goalUsageScopeRoundIDForRoomSlot(candidate) != scopeRoundID ||
			goalUsageSessionKeyForRoomSlot(candidate, goalSessionKeyForSlot(candidate)) != goalSessionKey {
			return
		}
		seen[candidate] = struct{}{}
		slots = append(slots, candidate)
	}
	appendSlot(origin)
	for _, roundValue := range s.rounds.snapshot() {
		if roundValue == nil {
			continue
		}
		for _, candidate := range roundValue.Slots {
			appendSlot(candidate)
		}
	}
	return slots
}

func (s *Service) bindRoomGoalUsageForScope(
	origin *activeRoomSlot,
	sessionKey string,
	goalID string,
) {
	sessionKey = strings.TrimSpace(sessionKey)
	goalID = strings.TrimSpace(goalID)
	if sessionKey == "" || goalID == "" {
		return
	}
	for _, slot := range s.roomGoalUsageSlotsForScope(origin, sessionKey) {
		slot.setGoalBinding(sessionKey, goalID)
	}
}

func (s *Service) startRoomGoalUsageFromRoundStartForScope(
	ctx context.Context,
	origin *activeRoomSlot,
	sessionKey string,
) {
	var updated *protocol.Goal
	for _, candidate := range s.roomGoalUsageSlotsForScope(origin, sessionKey) {
		if candidate.goalRuntimeIgnored() {
			continue
		}
		if result := s.startGoalUsageFromRoundStartForSlot(ctx, candidate); result != nil {
			updated = result
		}
	}
	if updated != nil {
		s.bindRoomGoalUsageForScope(origin, updated.SessionKey, updated.ID)
	}
}

func (s *Service) startGoalUsageFromRoundStartForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
) *protocol.Goal {
	if s == nil || s.goals == nil || slot == nil {
		return nil
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.runtimeIgnored {
		slot.mutable.goal.mu.Unlock()
		return nil
	}
	if slot.mutable.goal.usage != nil && slot.mutable.goal.usage.Active() {
		slot.mutable.goal.mu.Unlock()
		return nil
	}
	if slot.mutable.goal.usage == nil {
		slot.mutable.goal.usage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	backlog, ok := slot.mutable.goal.usage.PrepareActivationFromRoundStart()
	if !ok {
		slot.mutable.goal.mu.Unlock()
		return nil
	}
	goalID := strings.TrimSpace(slot.mutable.goal.idForUsage)
	goalSessionKey := strings.TrimSpace(slot.mutable.goal.sessionKey)
	if goalSessionKey == "" {
		goalSessionKey = strings.TrimSpace(slot.RuntimeSessionKey)
	}
	updated, persisted := s.persistGoalUsageDeltaForSlotTarget(ctx, slot, backlog, goalID, goalSessionKey)
	if persisted {
		slot.mutable.goal.usage.CommitDelta(backlog)
	}
	slot.mutable.goal.mu.Unlock()
	return updated
}

func (s *Service) ensureModelCreatedRoomGoalBinding(
	ctx context.Context,
	slot *activeRoomSlot,
) (string, string) {
	if s == nil || s.goals == nil || slot == nil {
		return "", ""
	}
	goalID := strings.TrimSpace(slot.goalIDForUsage())
	goalSessionKey := goalSessionKeyForSlot(slot)
	if goalID != "" {
		return goalID, goalSessionKey
	}
	_, goal, err := s.goals.RuntimeContext(ctx, goalSessionKey)
	if err != nil {
		if !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
			s.loggerFor(ctx).Warn(
				"读取 Room model 创建后的 Goal 绑定失败",
				"session_key", goalSessionKey,
				"round_id", slot.AgentRoundID,
				"err", err,
			)
		}
		return "", goalSessionKey
	}
	if goal == nil || strings.TrimSpace(goal.ID) == "" {
		return "", goalSessionKey
	}
	goalID = strings.TrimSpace(goal.ID)
	goalSessionKey = strings.TrimSpace(goal.SessionKey)
	s.bindRoomGoalUsageForScope(slot, goalSessionKey, goalID)
	executionID := ""
	switch protocol.GoalExecutionBindingStateFromGoal(*goal) {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
	case protocol.GoalExecutionBindingStateConfirmed:
		executionID = protocol.GoalReservedExecutionID(*goal)
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		return goalID, goalSessionKey
	}
	s.grantRoomGoalMutationAuthorityForScope(
		slot,
		goalSessionKey,
		goalID,
		goal.ObjectiveRevision(),
		executionID,
		roomGoalAuthorityModelCreate,
	)
	return goalID, goalSessionKey
}

func (s *Service) grantRoomGoalMutationAuthorityForScope(
	origin *activeRoomSlot,
	goalSessionKey string,
	goalID string,
	objectiveRevision int64,
	executionID string,
	source roomGoalAuthoritySource,
) {
	if s == nil || origin == nil {
		return
	}
	goalSessionKey = goalUsageSessionKeyForRoomSlot(origin, goalSessionKey)
	goalID = strings.TrimSpace(goalID)
	if goalSessionKey == "" || goalID == "" || objectiveRevision <= 0 {
		return
	}
	for _, candidate := range s.roomGoalUsageSlotsForScope(origin, goalSessionKey) {
		candidate.grantGoalMutationAuthority(roomGoalMutationAuthority{
			SessionKey:        goalSessionKey,
			GoalID:            goalID,
			ObjectiveRevision: objectiveRevision,
			ExecutionID: firstNonEmptyString(
				executionIDFromRoomBindings(
					candidate.WorkBinding,
					candidate.ReviewBinding,
				),
				executionID,
			),
			RootRoundID: goalUsageScopeRoundIDForRoomSlot(candidate),
			Source:      source,
		})
	}
}

func (s *Service) claimSubagentGoalUsageForRoomScope(
	ctx context.Context,
	origin *activeRoomSlot,
	goalID string,
	goalSessionKey string,
) {
	if s == nil || s.goals == nil || origin == nil || origin.goalRuntimeIgnored() {
		return
	}
	goalID = strings.TrimSpace(goalID)
	goalSessionKey = strings.TrimSpace(goalSessionKey)
	if goalID == "" || goalSessionKey == "" {
		return
	}
	for _, candidate := range s.roomGoalUsageSlotsForScope(origin, goalSessionKey) {
		if candidate.goalRuntimeIgnored() ||
			!strings.EqualFold(strings.TrimSpace(candidate.runtimeKind()), "nxs") {
			continue
		}
		if !s.claimSubagentGoalUsageForRoomSlot(
			ctx,
			candidate,
			goalID,
			goalSessionKey,
		) {
			s.loggerFor(ctx).Warn(
				"回补 Room model 创建前的 nxs 子任务 Goal usage 失败",
				"session_key", goalSessionKey,
				"runtime_session_key", candidate.RuntimeSessionKey,
				"goal_id", goalID,
				"scope_round_id", goalUsageScopeRoundIDForRoomSlot(candidate),
				"source_round_id", candidate.AgentRoundID,
			)
		}
	}
}

func (s *Service) claimSubagentGoalUsageForRoomSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	goalID string,
	goalSessionKey string,
) bool {
	if s == nil || s.goals == nil || slot == nil {
		return true
	}
	claimer, ok := s.goals.(interface {
		ClaimUsageSourceRound(context.Context, protocol.GoalUsageSourceRoundClaim) (protocol.GoalUsageSourceResult, error)
	})
	if !ok {
		slot.setGoalUsageClaimPending(false)
		return true
	}
	slot.setGoalUsageClaimPending(true)
	claim := protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:       goalUsageOwnerUserIDForRoomSlot(ctx, slot),
		RuntimeSessionKey: slot.RuntimeSessionKey,
		SourceKind:        protocol.GoalUsageSourceKindNXSTask,
		RoundID:           slot.AgentRoundID,
		ScopeRoundID:      goalUsageScopeRoundIDForRoomSlot(slot),
		GoalID:            strings.TrimSpace(goalID),
		GoalSessionKey:    goalUsageSessionKeyForRoomSlot(slot, goalSessionKey),
	}
	for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
		if attempt > 0 && !s.waitRoomGoalUsagePersistRetry(ctx, attempt) {
			return false
		}
		if _, err := claimer.ClaimUsageSourceRound(ctx, claim); err != nil {
			continue
		}
		slot.setGoalUsageClaimPending(false)
		return true
	}
	return false
}

func (s *Service) recordSubagentGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	message protocol.Message,
) []roomSubagentUsageSettlement {
	if s == nil || slot == nil ||
		!strings.EqualFold(strings.TrimSpace(slot.runtimeKind()), "nxs") {
		return nil
	}
	observations := roomSubagentUsageObservations(slot, message)
	if len(observations) == 0 {
		return nil
	}
	settled := make([]roomSubagentUsageSettlement, 0, len(observations))
	_, persistent := s.goals.(roomGoalUsageSourceRecorder)
	if persistent {
		unlockScope := s.lockRoomGoalUsageScope(ctx, slot)
		defer unlockScope()
		// pending 的建立本身也属于 scope 临界区。若 activation 已先拿到锁，
		// 此消息线性化在 bind 之后；若此处先拿到锁，它一定会在 bind 前落库。
		for _, child := range observations {
			slot.markSubagentUsageObservationPending(child.observation, child.taskID)
		}
		for _, child := range observations {
			pending := slot.subagentUsageObservationPendingSnapshot()
			observation, exists := pending[child.taskID]
			if !exists {
				// external activation 已在同一 scope 临界区内完成了 pre-bind
				// flush；调用方的 conditional clear 仍可安全 no-op。
				settled = append(settled, child)
				continue
			}
			goalID := strings.TrimSpace(slot.childGoalIDForUsage())
			goalSessionKey := goalUsageSessionKeyForRoomSlot(slot, goalSessionKeyForSlot(slot))
			var (
				result protocol.GoalUsageSourceResult
				err    error
			)
			for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
				if attempt > 0 && !s.waitRoomGoalUsagePersistRetry(ctx, attempt) {
					break
				}
				result, err = s.persistSubagentGoalUsageObservationForSlot(
					ctx,
					slot,
					child.taskID,
					observation,
					goalID,
					goalSessionKey,
				)
				if err == nil {
					break
				}
			}
			if err != nil {
				s.loggerFor(ctx).Warn("记录 Room nxs 子任务 Goal usage 失败",
					"session_key", goalSessionKey,
					"goal_id", goalID,
					"scope_round_id", goalUsageScopeRoundIDForRoomSlot(slot),
					"source_round_id", slot.AgentRoundID,
					"task_id", child.taskID,
					"err", err,
				)
				continue
			}
			settled = append(settled, roomSubagentUsageSettlement{
				taskID:          child.taskID,
				cumulativeTotal: observation.cumulativeTotal,
				observation:     observation,
			})
			slot.clearSubagentUsageObservationPending(child.taskID, observation)
			if result.Goal != nil {
				s.bindRoomGoalUsageForScope(slot, result.Goal.SessionKey, result.Goal.ID)
			}
		}
		return settled
	}

	for _, child := range observations {
		slot.markSubagentUsageObservationPending(child.observation, child.taskID)
	}
	goalID := strings.TrimSpace(slot.childGoalIDForUsage())
	attributed := goalID != "" && !slot.goalRuntimeIgnored()
	for _, child := range observations {
		if s.runtime == nil {
			settled = append(settled, roomSubagentUsageSettlement{
				taskID:          child.taskID,
				cumulativeTotal: child.observation.cumulativeTotal,
				observation:     child.observation,
			})
			continue
		}
		delta := s.runtime.ObserveSubagentUsage(
			slot.RuntimeSessionKey,
			child.taskID,
			child.observation.cumulativeTotal,
		)
		if delta > 0 && attributed && s.goals != nil {
			// 兼容测试/非 SQL provider：每个 slot 按 runtime session 去重，
			// 再把 child provider actual 汇总到共享 Goal。
			s.recordGoalUsageDeltaForSlot(ctx, slot, protocol.GoalUsage{ActualTotalTokens: delta})
		}
		settled = append(settled, roomSubagentUsageSettlement{
			taskID:          child.taskID,
			cumulativeTotal: child.observation.cumulativeTotal,
			observation:     child.observation,
		})
	}
	return settled
}

func roomSubagentUsageObservations(
	slot *activeRoomSlot,
	message protocol.Message,
) []roomSubagentUsageSettlement {
	usage := messageutil.SubagentTaskUsageSnapshots(message)
	observedAt := time.Now().UTC()
	observations := make([]roomSubagentUsageSettlement, 0, len(usage)+1)
	indexByTask := make(map[string]int, len(usage)+1)
	for _, child := range usage {
		taskID := strings.TrimSpace(child.TaskID)
		if taskID == "" || child.TotalTokens <= 0 {
			continue
		}
		indexByTask[taskID] = len(observations)
		observations = append(observations, roomSubagentUsageSettlement{
			taskID:          taskID,
			cumulativeTotal: child.TotalTokens,
			observation: roomSubagentUsageObservation{
				cumulativeTotal: child.TotalTokens,
				observedAt:      observedAt,
			},
		})
	}

	metadata, _ := message["metadata"].(map[string]any)
	taskID := strings.TrimSpace(anyString(metadata["task_id"]))
	if taskID == "" ||
		(!metadataLooksLikeSubagentTask(metadata) && (slot == nil || !slot.knowsSubagentTask(taskID))) {
		return observations
	}
	terminal := isTerminalSubagentTaskStatus(anyString(metadata["status"]))
	if index, exists := indexByTask[taskID]; exists {
		observations[index].observation.terminal =
			observations[index].observation.terminal || terminal
		observations[index].observation.terminalTokenUsageObserved =
			observations[index].observation.terminalTokenUsageObserved ||
				(terminal && observations[index].observation.cumulativeTotal > 0)
		return observations
	}
	observations = append(observations, roomSubagentUsageSettlement{
		taskID: taskID,
		observation: roomSubagentUsageObservation{
			terminal:   terminal,
			observedAt: observedAt,
		},
	})
	return observations
}

func (s *Service) persistSubagentGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	taskID string,
	cumulativeTotal int64,
	goalID string,
	goalSessionKey string,
) (protocol.GoalUsageSourceResult, error) {
	return s.persistSubagentGoalUsageObservationForSlot(
		ctx,
		slot,
		taskID,
		roomSubagentUsageObservation{
			cumulativeTotal: cumulativeTotal,
		},
		goalID,
		goalSessionKey,
	)
}

func (s *Service) persistSubagentGoalUsageObservationForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	taskID string,
	observation roomSubagentUsageObservation,
	goalID string,
	goalSessionKey string,
) (protocol.GoalUsageSourceResult, error) {
	recorder, ok := s.goals.(roomGoalUsageSourceRecorder)
	if !ok || slot == nil {
		return protocol.GoalUsageSourceResult{}, nil
	}
	result, err := recorder.RecordUsageSourceSnapshot(ctx, protocol.GoalUsageSourceSnapshot{
		OwnerUserID:            goalUsageOwnerUserIDForRoomSlot(ctx, slot),
		RuntimeSessionKey:      slot.RuntimeSessionKey,
		SourceKind:             protocol.GoalUsageSourceKindNXSTask,
		SourceID:               strings.TrimSpace(taskID),
		CumulativeActualTokens: observation.cumulativeTotal,
		EvidenceRequired:       true,
		Terminal:               observation.terminal,
		TokenUsageObserved:     observation.terminalTokenUsageObserved,
		GoalID:                 strings.TrimSpace(goalID),
		GoalSessionKey:         goalUsageSessionKeyForRoomSlot(slot, goalSessionKey),
		RoundID:                slot.AgentRoundID,
		ScopeRoundID:           goalUsageScopeRoundIDForRoomSlot(slot),
		ObservedAt:             observation.observedAt,
	})
	if err == nil && result.Goal != nil {
		s.bindRoomGoalUsageForScope(slot, result.Goal.SessionKey, result.Goal.ID)
	}
	return result, err
}

func closeGoalUsageForSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.closeGoalUsage()
}

func clearGoalUsageForSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.clearGoalUsage()
}

func (s *Service) clearRoomGoalUsage(sessionKey string) {
	if s == nil {
		return
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return
	}
	for _, roundValue := range s.rounds.snapshot() {
		if roundValue == nil || strings.TrimSpace(roundValue.SessionKey) != sessionKey {
			continue
		}
		for _, slot := range roundValue.Slots {
			clearGoalUsageForSlot(slot)
		}
	}
}

func (s *Service) beginRoomGoalUsageFinalizing(sessionKey string) {
	if s == nil {
		return
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return
	}
	for _, roundValue := range s.rounds.snapshot() {
		if roundValue == nil || strings.TrimSpace(roundValue.SessionKey) != sessionKey {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot != nil {
				slot.beginGoalUsageFinalizing()
			}
		}
	}
}

func (s *Service) activateGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	goalID string,
) error {
	if slot == nil || slot.goalRuntimeIgnored() {
		return nil
	}
	goalID = strings.TrimSpace(goalID)
	unlockScope := s.lockRoomGoalUsageScope(ctx, slot)
	defer unlockScope()
	if goalID != "" {
		if err := s.flushRoomSubagentUsageBeforeExternalBind(ctx, slot); err != nil {
			// 只要还有当前进程已知 checkpoint 未能落库，就不能建立 from-now
			// 边界；否则 retry 会在 bind 后把旧累计误归新 Goal。
			return err
		}
		if binder, ok := s.goals.(roomGoalUsageScopeBinder); ok {
			binding := protocol.GoalUsageScopeBinding{
				OwnerUserID:    goalUsageOwnerUserIDForRoomSlot(ctx, slot),
				GoalSessionKey: goalUsageSessionKeyForRoomSlot(slot, goalSessionKeyForSlot(slot)),
				SourceKind:     protocol.GoalUsageSourceKindNXSTask,
				ScopeRoundID:   goalUsageScopeRoundIDForRoomSlot(slot),
				GoalID:         goalID,
				BoundAt:        time.Now().UTC(),
			}
			var err error
			for attempt := 0; attempt < goalUsagePersistAttempts; attempt++ {
				if attempt > 0 && !s.waitRoomGoalUsagePersistRetry(ctx, attempt) {
					return ctx.Err()
				}
				if _, err = binder.BindUsageScopeFromNow(ctx, binding); err == nil {
					break
				}
				if errors.Is(err, goalsvc.ErrGoalInvalidState) {
					// 非 SQL provider 没有 durable scope capability；保留原有
					// in-memory accounting 路径，不把能力缺失当成瞬时写失败。
					err = nil
					break
				}
			}
			if err != nil {
				// durable bind 是 Reset 的前置条件。失败时保持旧 Goal/baseline，
				// 由 Goal service 把错误表面化并回滚新建 Goal。
				return err
			}
		}
	}
	activateGoalUsageForSlot(ctx, slot, goalID)
	return nil
}

func activateGoalUsageForSlot(_ context.Context, slot *activeRoomSlot, goalID string) {
	if slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	goalID = strings.TrimSpace(goalID)
	if goalID != "" && slot.goalUsageActiveForGoal(goalID) {
		return
	}
	slot.setGoalBinding(goalSessionKeyForSlot(slot), goalID)
	slot.setGoalUsageClaimPending(false)
	snapshot := slotAssistantGoalUsageSnapshot(slot, slot.lastGoalAssistantMessage())
	slot.resetGoalUsage(snapshot)
}

func slotGoalUsageElapsedSeconds(slot *activeRoomSlot) int64 {
	startedAt := slot.goalUsageStartedAt()
	if startedAt.IsZero() {
		return 0
	}
	elapsed := int64(time.Since(startedAt).Seconds())
	return max(elapsed, 0)
}

func isZeroRoomGoalUsage(usage protocol.GoalUsage) bool {
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

// goalCancellationProvider 是用户取消当前 Room Goal 所需的最小 Goal 能力。
type goalCancellationProvider interface {
	CurrentOptional(context.Context, string) (*protocol.Goal, error)
	Clear(context.Context, string) (bool, error)
}

// cancelActiveRoomGoalForUser 定义用户取消的边界：只清除 active Goal，
// 不把暂停或已完成的历史 Goal 重新解释为取消，也不触发新的续跑。
func (s *Service) cancelActiveRoomGoalForUser(
	ctx context.Context,
	sessionKey string,
	content string,
) error {
	if s == nil || !isGoalCancellationRequest(content) {
		return nil
	}
	provider, ok := s.goals.(goalCancellationProvider)
	if !ok {
		return nil
	}
	goal, err := provider.CurrentOptional(ctx, strings.TrimSpace(sessionKey))
	if errors.Is(err, goalsvc.ErrGoalNotFound) || goal == nil {
		return nil
	}
	if err != nil {
		return err
	}
	if protocol.NormalizeGoalStatus(goal.Status) != protocol.GoalStatusActive {
		return nil
	}
	_, err = provider.Clear(ctx, goal.ID)
	if errors.Is(err, goalsvc.ErrGoalNotFound) {
		return nil
	}
	if err == nil {
		s.loggerFor(ctx).Info("用户取消 Room active Goal",
			"session_key", strings.TrimSpace(sessionKey),
			"goal_id", strings.TrimSpace(goal.ID),
			"content", strings.TrimSpace(content),
		)
	}
	return err
}

// isGoalCancellationRequest 只识别短、明确的停止意图，避免把普通讨论中的“停止”误判为取消。
func isGoalCancellationRequest(content string) bool {
	content = normalizeGoalCancellationText(content)
	if content == "" {
		return false
	}
	if content == "算了" || content == "不用了" || content == "取消" || content == "停止" || content == "停下" {
		return true
	}
	for _, phrase := range []string{
		"算了不用了",
		"不用继续",
		"取消这个任务",
		"取消任务",
		"停止这个任务",
		"停止任务",
	} {
		if strings.Contains(content, phrase) {
			return true
		}
	}
	return false
}

func normalizeGoalCancellationText(content string) string {
	content = strings.TrimSpace(strings.ToLower(content))
	var builder strings.Builder
	for _, runeValue := range content {
		if unicode.IsSpace(runeValue) || unicode.IsPunct(runeValue) {
			continue
		}
		builder.WriteRune(runeValue)
	}
	return builder.String()
}

// INPUT: 当前 Room Goal、调用方 Agent/root round、active slots 与 durable Room work。
// OUTPUT: complete 前第一个 outstanding-work blocker；调用方主 slot 不阻塞自身。
// POS: Room 实时/持久化工作到 Goal 终态 gate 的唯一投影入口。
// RoomGoalCompletionBlocker 返回阻止共享 Goal complete 的 Room 工作；空字符串表示已收敛。
func (s *Service) RoomGoalCompletionBlocker(
	ctx context.Context,
	goal protocol.Goal,
	callerAgentID string,
	callerRoundID string,
) (string, error) {
	if s == nil || !protocol.IsRoomSharedSessionKey(goal.SessionKey) {
		return "", nil
	}
	parsed := protocol.ParseSessionKey(goal.SessionKey)
	conversationID := strings.TrimSpace(parsed.ConversationID)
	if conversationID == "" {
		return "", nil
	}

	// 同一 conversation 的 queue、wake 和 active slot 必须在同一个派发闸门内观察，
	// 避免 wake 交接窗口被误判为 idle。
	lease := s.lockRoomDispatch(goal.SessionKey, conversationID)
	defer lease.Unlock()

	ctx, contextValue, err := s.internalConversationContext(ctx, conversationID, true)
	if err != nil {
		return "", err
	}
	if blocker := s.activeRoomGoalBlocker(goal.SessionKey, conversationID, callerAgentID, callerRoundID); blocker != "" {
		return blocker, nil
	}
	if blocker, err := s.roomGoalInputQueueBlocker(ctx, contextValue); err != nil || blocker != "" {
		return blocker, err
	}
	return s.roomGoalDelayedWakeBlocker(contextValue.Room.OwnerUserID, conversationID)
}

func (s *Service) activeRoomGoalBlocker(
	sessionKey string,
	conversationID string,
	callerAgentID string,
	callerRoundID string,
) string {
	sessionKey = strings.TrimSpace(sessionKey)
	conversationID = strings.TrimSpace(conversationID)
	callerAgentID = strings.TrimSpace(callerAgentID)
	callerRoundID = strings.TrimSpace(callerRoundID)

	for _, roundValue := range s.rounds.snapshotConversation(conversationID) {
		if roundValue == nil ||
			strings.TrimSpace(roundValue.SessionKey) != sessionKey ||
			strings.TrimSpace(roundValue.ConversationID) != conversationID {
			continue
		}
		// public @ 已从模型输出解析，但尚未交接成目标 slot。
		// 它挂在当前 shared Goal 的 Room round 上，清空或注册 slot 后自动解锁。
		if s.rounds.hasPublicMentions(roundValue) {
			return "a Room public-mention wake has not started"
		}
		for _, slot := range roundValue.Slots {
			if slot == nil {
				continue
			}
			isCallerSlot := callerAgentID != "" && callerRoundID != "" &&
				strings.TrimSpace(slot.AgentID) == callerAgentID &&
				(roomRootRoundID(roundValue) == callerRoundID ||
					strings.TrimSpace(roundValue.RoundID) == callerRoundID ||
					strings.TrimSpace(slot.AgentRoundID) == callerRoundID)
			if slot.hasRunningSubagentTask() {
				if isCallerSlot {
					return fmt.Sprintf("caller agent %s still has running subagent work", callerAgentID)
				}
				return fmt.Sprintf("agent %s still has running subagent work", strings.TrimSpace(slot.AgentID))
			}
			if slot.isTerminal() {
				if slot.goalUsageSettlementRequired() && !slot.goalUsageTerminalSettled() {
					return fmt.Sprintf("agent %s terminal Goal usage is not settled", strings.TrimSpace(slot.AgentID))
				}
				continue
			}
			if isCallerSlot {
				continue
			}
			return fmt.Sprintf("agent %s still has an active Room slot", strings.TrimSpace(slot.AgentID))
		}
	}
	if s.rounds.hasPublicMentionsForConversation(conversationID) {
		return "a Room public-mention wake has not started"
	}
	return ""
}

func (s *Service) roomGoalInputQueueBlocker(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) (string, error) {
	if s.inputQueue == nil || contextValue == nil {
		return "", nil
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	// InputQueue replay 已排除 expired/deleted/dispatched 项。队列尚无 goal_id，
	// 所以对同 conversation 的 active shared Goal 保守阻止，不会被日志历史永久卡住。
	return fmt.Sprintf("Room input queue item %s has not been consumed", strings.TrimSpace(entries[0].Item.ID)), nil
}

func (s *Service) roomGoalDelayedWakeBlocker(ownerUserID string, conversationID string) (string, error) {
	if s.directedWakes == nil {
		return "", nil
	}
	pending, err := s.directedWakes.Pending(ownerUserID)
	if err != nil {
		return "", err
	}
	for _, wake := range pending {
		if strings.TrimSpace(wake.Message.ConversationID) != strings.TrimSpace(conversationID) {
			continue
		}
		return fmt.Sprintf("Room directed wake %s has not started", strings.TrimSpace(wake.WakeID)), nil
	}
	return "", nil
}
