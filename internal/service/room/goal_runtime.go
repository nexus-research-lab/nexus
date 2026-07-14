// INPUT: Room slot 的 Goal 上下文、objective revision 与运行结果。
// OUTPUT: slot 级 Goal accounting、协作证据和消费后生效的逐 slot objective steering。
// POS: Room runtime 与共享 Goal 领域之间的唯一投影入口。
package room

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

const goalContextualInputName = "goal"

// QueueRoomContextualGuidanceInput 把共享 Goal steering 分发到每个活跃 slot，并排除产生 retarget 的 caller。
func (s *RealtimeService) QueueRoomContextualGuidanceInput(
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
	s.mu.Lock()
	for _, roundValue := range s.activeRounds {
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
	s.mu.Unlock()

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
func (s *RealtimeService) GoalObjectiveRevisionState(
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
	s.mu.Lock()
	var target *activeRoomSlot
	for _, roundValue := range s.activeRounds {
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
	s.mu.Unlock()
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

func (s *RealtimeService) resolveGoalRuntimeContextForSlot(
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

func (s *RealtimeService) goalRuntimeContext(ctx context.Context, sessionKey string) (string, string, int64, bool) {
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

func (s *RealtimeService) recordGoalContinuationProgressForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	roundValue *activeRoomRound,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.GoalRuntimeIgnored || strings.TrimSpace(slot.GoalIDForUsage) == "" {
		return
	}
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
			_, err := s.goals.RecordContinuationFailure(ctx, slot.GoalIDForUsage, slot.AgentRoundID, reason, slot.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	if purpose != "goal_continuation" {
		s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 显式活动失败", func() error {
			_, err := s.goals.RecordGoalActivity(ctx, slot.GoalIDForUsage, slot.AgentRoundID, slot.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	if messageutil.AssistantMissedGoalCompletionTool(finalAssistant) {
		reason := "assistant claimed goal completion but did not call mcp__nexus_goal__update_goal"
		s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 完成工具漏调用失败", func() error {
			_, err := s.goals.RecordCompletionToolMiss(ctx, slot.GoalIDForUsage, slot.AgentRoundID, reason, slot.currentGoalObjectiveRevision())
			return err
		})
		return
	}
	hasProgress := slotHasGoalToolProgress(slot)
	if !hasProgress && slot.hasRunningSubagentTask() {
		return
	}
	s.recordSlotGoalMutation(ctx, slot, "记录 Room Goal 续跑进展失败", func() error {
		_, err := s.goals.RecordContinuationProgress(ctx, slot.GoalIDForUsage, slot.AgentRoundID, hasProgress, slot.currentGoalObjectiveRevision())
		return err
	}, "progressed", hasProgress)
}

func (s *RealtimeService) recordSlotGoalMutation(
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
		"goal_id", slot.GoalIDForUsage,
		"round_id", slot.AgentRoundID,
	}
	baseFields = append(baseFields, fields...)
	baseFields = append(baseFields, "err", err)
	s.loggerFor(ctx).Warn(logMessage, baseFields...)
}

func (s *RealtimeService) recordRoomGoalCollaborationEvidenceForSlot(
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
		_, err := s.goals.RecordRoomGoalCollaborationEvidence(ctx, slot.GoalIDForUsage, slot.AgentRoundID, slot.AgentID, slot.currentGoalObjectiveRevision())
		return err
	}, "agent_id", slot.AgentID)
}

func rememberGoalToolProgressForSlot(slot *activeRoomSlot, progressed bool) {
	if slot == nil || !progressed {
		return
	}
	slot.stateMu.Lock()
	slot.GoalToolProgress = true
	slot.stateMu.Unlock()
}

func slotHasGoalToolProgress(slot *activeRoomSlot) bool {
	if slot == nil {
		return false
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return slot.GoalToolProgress
}

func goalSessionKeyForSlot(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	if sessionKey := strings.TrimSpace(slot.GoalSessionKey); sessionKey != "" {
		return sessionKey
	}
	return strings.TrimSpace(slot.RuntimeSessionKey)
}
