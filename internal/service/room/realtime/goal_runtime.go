// INPUT: Room slot 的 Goal 上下文、objective revision 与运行结果。
// OUTPUT: slot 级 Goal accounting、协作证据和消费后生效的逐 slot objective steering。
// POS: Room runtime 与共享 Goal 领域之间的唯一投影入口。
package realtime

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
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

const goalContextualInputName = "goal"

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
	if s.goals == nil {
		return "", "", 0, false
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) {
			return "", "", 0, false
		}
		s.loggerFor(ctx).Warn("读取 Room Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return "", "", 0, false
	}
	goalID := ""
	objectiveRevision := int64(0)
	if goal != nil {
		goalID = strings.TrimSpace(goal.ID)
		objectiveRevision = goal.ObjectiveRevision()
	}
	if strings.TrimSpace(goalContext) == "" {
		return "", goalID, objectiveRevision, true
	}
	return strings.TrimSpace(goalContext), goalID, objectiveRevision, true
}

func (s *Service) recordGoalContinuationProgressForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	roundValue *activeRoomRound,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() || strings.TrimSpace(slot.goalIDForUsage()) == "" {
		return
	}
	goalID := slot.goalIDForUsage()
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
			_, err := s.goals.RecordContinuationFailure(ctx, goalID, slot.AgentRoundID, reason, slot.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	if purpose != "goal_continuation" {
		s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 显式活动失败", func() error {
			_, err := s.goals.RecordGoalActivity(ctx, goalID, slot.AgentRoundID, slot.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	if messageutil.AssistantMissedGoalCompletionTool(finalAssistant) {
		reason := "assistant claimed goal completion but did not call mcp__nexus_goal__update_goal"
		s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 完成工具漏调用失败", func() error {
			_, err := s.goals.RecordCompletionToolMiss(ctx, goalID, slot.AgentRoundID, reason, slot.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	hasProgress := slotHasGoalToolProgress(slot)
	if !hasProgress && slot.hasRunningSubagentTask() {
		return
	}
	s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 续跑进展失败", func() error {
		_, err := s.goals.RecordContinuationProgress(ctx, goalID, slot.AgentRoundID, hasProgress, slot.currentGoalObjectiveRevision())
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
	if roomdomain.IsNoReplyAssistantMessage(finalAssistant) {
		return
	}
	if strings.TrimSpace(messageutil.ExtractAssistantDisplayText(finalAssistant)) == "" {
		return
	}
	s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 协作证据失败", func() error {
		_, err := s.goals.RecordRoomGoalCollaborationEvidence(ctx, slot.goalIDForUsage(), slot.AgentRoundID, slot.AgentID, slot.currentGoalObjectiveRevision())
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
	s.runtime.RegisterGoalAccountingActivate(sessionKey, roundID, func(ctx context.Context) error {
		activateGoalUsageForSlot(ctx, slot)
		return nil
	})
	return func() {
		s.runtime.RegisterGoalAccountingFlush(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingClear(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingActivate(sessionKey, roundID, nil)
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
	s.recordGoalUsageSnapshotForSlot(ctx, slot, snapshot)
}

func (s *Service) recordGoalUsageLimitForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() || !result.UsageLimitReached {
		return
	}
	_, err := s.goals.UsageLimitForSession(ctx, goalSessionKeyForSlot(slot), slot.AgentRoundID, result.UsageLimitReason)
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) {
		s.loggerFor(ctx).Warn("标记 Room Goal usage limit 失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", slot.goalIDForUsage(),
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
}

func (s *Service) flushGoalUsageForSlot(ctx context.Context, slot *activeRoomSlot) error {
	s.recordGoalUsageForSlot(ctx, slot, exec.RoundExecutionResult{}, slot.lastGoalAssistantMessage())
	return nil
}

func (s *Service) recordGoalUsageFromSlotAssistantMessage(
	ctx context.Context,
	slot *activeRoomSlot,
	message protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	observations := messageutil.AssistantToolResults(message)
	if len(observations) == 0 {
		return
	}
	rememberGoalToolProgressForSlot(slot, messageutil.AssistantHasCountedToolProgress(message))
	snapshot := slotAssistantGoalUsageSnapshot(slot, message)
	hasSuccessfulCreate := false
	hasSuccessfulUpdate := false
	for _, observation := range observations {
		if observation.IsError {
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
		if slot.resetGoalUsageIfInactive(snapshot) {
			return
		}
	}
	s.recordGoalUsageSnapshotForSlot(ctx, slot, snapshot)
	if hasSuccessfulUpdate {
		clearGoalUsageForSlot(slot)
	}
}

func slotFinalGoalUsageSnapshot(
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) (goalsvc.RuntimeUsageSnapshot, bool) {
	usage := runtimectx.GoalUsageFromTokenUsage(result.Usage)
	usageOK := !result.Usage.IsZero()
	if !usageOK && protocol.MessageRole(finalAssistant) == "assistant" {
		usage, usageOK = runtimectx.GoalUsageFromRaw(finalAssistant["usage"])
	}
	elapsedSeconds := result.ElapsedTimeSeconds
	if elapsedSeconds <= 0 {
		elapsedSeconds = slotGoalUsageElapsedSeconds(slot)
	}
	return goalsvc.RuntimeUsageSnapshot{
		Usage:          usage,
		ElapsedSeconds: elapsedSeconds,
	}, usageOK || elapsedSeconds > 0
}

func slotAssistantGoalUsageSnapshot(slot *activeRoomSlot, message protocol.Message) goalsvc.RuntimeUsageSnapshot {
	usage, _ := runtimectx.GoalUsageFromRaw(message["usage"])
	return goalsvc.RuntimeUsageSnapshot{
		Usage:          usage,
		ElapsedSeconds: slotGoalUsageElapsedSeconds(slot),
	}
}

func (s *Service) recordGoalUsageSnapshotForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	snapshot goalsvc.RuntimeUsageSnapshot,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	if usage, ok, tracked := slot.goalUsageDelta(snapshot); tracked {
		if ok {
			s.recordGoalUsageDeltaForSlot(ctx, slot, usage)
		}
		return
	}
	usage := snapshot.Usage
	usage.RuntimeSeconds = snapshot.ElapsedSeconds
	if isZeroRoomGoalUsage(usage) {
		return
	}
	s.recordGoalUsageDeltaForSlot(ctx, slot, usage)
}

func (s *Service) recordGoalUsageDeltaForSlot(ctx context.Context, slot *activeRoomSlot, usage protocol.GoalUsage) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() || isZeroRoomGoalUsage(usage) {
		return
	}
	var err error
	goalID := slot.goalIDForUsage()
	if strings.TrimSpace(goalID) != "" {
		_, err = s.goals.RecordUsageForGoal(ctx, goalID, usage, slot.AgentRoundID)
	} else {
		_, err = s.goals.RecordUsageForSession(ctx, goalSessionKeyForSlot(slot), usage, slot.AgentRoundID)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
		s.loggerFor(ctx).Warn("记录 Room Goal usage 失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", goalID,
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
}

func clearGoalUsageForSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.closeGoalUsage()
}

func activateGoalUsageForSlot(_ context.Context, slot *activeRoomSlot) {
	if slot == nil || slot.goalRuntimeIgnored() {
		return
	}
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
