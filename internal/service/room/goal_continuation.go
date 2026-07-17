// INPUT: Room Goal 状态/lead、成员目录、协作者 active slot、显式输入队列与上一轮执行结果。
// OUTPUT: 启动 slot 前对齐的有效 lead，以及所有同 Goal 工作收敛后经原子 claim 的隐藏 continuation。
// POS: Room 与 Goal 权限/状态机之间的续跑适配层。
package room

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ShouldDeferGoalContinuation 避免隐藏 Goal 续跑抢占显式输入，并按 Codex 语义跳过 Plan 模式续跑。
func (s *RealtimeService) ShouldDeferGoalContinuation(ctx context.Context, sessionKey string) bool {
	return s.shouldDeferGoalContinuation(ctx, sessionKey, true)
}

func (s *RealtimeService) shouldDeferGoalContinuation(ctx context.Context, sessionKey string, dispatchQueuedInput bool) bool {
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || sessionKey == "" {
		return false
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindRoom || strings.TrimSpace(parsed.ConversationID) == "" {
		return s.runtime != nil && len(s.runtime.GetRunningRoundIDs(sessionKey)) > 0
	}
	if s.rooms == nil {
		// Tests and reduced embeddings may not configure the Room repository. In
		// that case the shared runtime is the only safe source of busy state.
		return s.runtime != nil && len(s.runtime.GetRunningRoundIDs(sessionKey)) > 0
	}
	ctx, contextValue, err := s.internalConversationContext(ctx, parsed.ConversationID, true)
	if err != nil || contextValue == nil {
		if err != nil {
			s.loggerFor(ctx).Warn("解析 Room Goal 续跑待发送队列上下文失败", "session_key", sessionKey, "err", err)
		}
		return false
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room Goal 续跑待发送队列失败", "session_key", sessionKey, "err", err)
		return false
	}
	if len(entries) == 0 {
		return s.shouldDeferGoalContinuationForTargetState(ctx, sessionKey, contextValue)
	}
	entry, ok := s.findDispatchableInputQueueEntry(sessionKey, parsed.ConversationID, entries)
	if !ok {
		return true
	}
	if dispatchQueuedInput {
		s.dispatchNextInputQueueItem(
			contextWithQueueOwner(ctx, entry.Item.OwnerUserID),
			sessionKey,
			contextValue.Room.ID,
			contextValue.Conversation.ID,
		)
	}
	return true
}

func (s *RealtimeService) shouldDeferGoalContinuationForTargetState(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) bool {
	if s == nil || contextValue == nil {
		return false
	}
	s.publicMentionDispatchMu.Lock()
	activeBlocker := s.activeRoomGoalBlocker(sessionKey, contextValue.Conversation.ID, "", "")
	s.publicMentionDispatchMu.Unlock()
	if activeBlocker != "" {
		return true
	}
	if s.agents == nil {
		return false
	}
	agentNameByID, agentByID, err := s.buildAgentDirectory(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room Goal 续跑 Agent plan mode 状态失败", "conversation_id", contextValue.Conversation.ID, "err", err)
		return false
	}
	targetAgentID := goalContinuationTargetAgentID(contextValue, agentNameByID, s.currentRoomGoalForSession(ctx, sessionKey))
	if targetAgentID == "" {
		return true
	}
	if len(s.findActiveDeliverySlotsByAgent(
		sessionKey,
		contextValue.Conversation.ID,
		[]string{targetAgentID},
	)) > 0 {
		return true
	}
	agentValue := agentByID[targetAgentID]
	if agentValue == nil {
		return true
	}
	return goalsvc.ShouldIgnoreRuntimeForPermissionMode(agentValue.Options.PermissionMode)
}

// GoalContinuationTargetMissing 判断共享 Room Goal 的 conversation 是否已被删除。
func (s *RealtimeService) GoalContinuationTargetMissing(ctx context.Context, sessionKey string) (bool, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || sessionKey == "" {
		return false, nil
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return true, nil
	}
	parsed := protocol.ParseSessionKey(normalized)
	if parsed.Kind != protocol.SessionKeyKindRoom || strings.TrimSpace(parsed.ConversationID) == "" {
		return false, nil
	}
	return s.GoalContinuationConversationMissing(ctx, parsed.ConversationID)
}

// GoalContinuationConversationMissing 判断 Room conversation 是否已不存在。
func (s *RealtimeService) GoalContinuationConversationMissing(ctx context.Context, conversationID string) (bool, error) {
	conversationID = strings.TrimSpace(conversationID)
	if s == nil || s.rooms == nil || conversationID == "" {
		return false, nil
	}
	_, contextValue, err := s.internalConversationContext(ctx, conversationID, true)
	if errors.Is(err, ErrRoomNotFound) || errors.Is(err, ErrConversationNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return contextValue == nil, nil
}

func goalContinuationTargetAgentID(
	contextValue *protocol.ConversationContextAggregate,
	agentNameByID map[string]string,
	goal *protocol.Goal,
) string {
	if goal != nil {
		leadAgentID := goalsvc.RoomLeadAgentID(*goal)
		if leadAgentID != "" {
			if _, ok := agentNameByID[leadAgentID]; ok {
				return leadAgentID
			}
		}
	}
	if contextValue != nil {
		hostAgentID := strings.TrimSpace(contextValue.Room.HostAgentID)
		if hostAgentID != "" {
			if _, ok := agentNameByID[hostAgentID]; ok {
				return hostAgentID
			}
		}
	}
	if len(agentNameByID) == 1 {
		for agentID := range agentNameByID {
			return agentID
		}
	}
	return ""
}

type currentGoalProvider interface {
	CurrentOptional(context.Context, string) (*protocol.Goal, error)
}

type roomGoalLeadSetter interface {
	SetRoomGoalLead(context.Context, string, string, string) (*protocol.Goal, error)
}

func (s *RealtimeService) reconcileRoomGoalLead(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	agentNameByID map[string]string,
) error {
	provider, hasProvider := s.goals.(currentGoalProvider)
	setter, hasSetter := s.goals.(roomGoalLeadSetter)
	if !hasProvider || !hasSetter || contextValue == nil {
		return nil
	}
	goal, err := provider.CurrentOptional(ctx, sessionKey)
	if err != nil {
		return err
	}
	if goal == nil {
		return nil
	}
	leadAgentID := goalContinuationTargetAgentID(contextValue, agentNameByID, goal)
	if leadAgentID == "" {
		return fmt.Errorf("Room Goal %s has no valid lead; assign a Room host or Goal lead before continuing", goal.ID)
	}
	leadName := strings.TrimSpace(agentNameByID[leadAgentID])
	if goalsvc.RoomLeadAgentID(*goal) == leadAgentID && goalsvc.RoomLeadAgentName(*goal) == leadName {
		return nil
	}
	_, err = setter.SetRoomGoalLead(ctx, goal.ID, leadAgentID, leadName)
	return err
}

func (s *RealtimeService) currentRoomGoalForSession(ctx context.Context, sessionKey string) *protocol.Goal {
	provider, ok := s.goals.(currentGoalProvider)
	if !ok {
		return nil
	}
	goal, err := provider.CurrentOptional(ctx, sessionKey)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room Goal 负责人失败", "session_key", sessionKey, "err", err)
		return nil
	}
	if goal == nil || protocol.NormalizeGoalStatus(goal.Status) != protocol.GoalStatusActive {
		return nil
	}
	return goal
}

func (s *RealtimeService) dispatchPostRoundWork(ctx context.Context, roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	if roundValue.RunningSubagents.Load() {
		return
	}
	if s.ShouldDeferGoalContinuation(ctx, roundValue.SessionKey) {
		return
	}
	s.dispatchGoalContinuation(ctx, roundValue)
}

func (s *RealtimeService) dispatchGoalContinuation(ctx context.Context, roundValue *activeRoomRound) {
	if s == nil || roundValue == nil || s.goals == nil {
		return
	}
	planner, ok := s.goals.(goalContinuationProvider)
	if !ok {
		return
	}
	plan, err := goalsvc.PrepareContinuationForDispatch(
		ctx,
		planner,
		roundValue.SessionKey,
		roundValue.RoundID,
		func(plan protocol.GoalContinuation) bool {
			return s.ShouldDeferGoalContinuation(ctx, plan.Goal.SessionKey)
		},
	)
	if err != nil {
		if goalsvc.IsExpectedMutationError(err) {
			return
		}
		s.loggerFor(ctx).Warn("准备 Room Goal 自动续跑失败",
			"session_key", roundValue.SessionKey,
			"round_id", roundValue.RoundID,
			"err", err,
		)
		return
	}
	if plan == nil {
		return
	}
	if err := s.DispatchGoalContinuation(ctx, *plan); err != nil {
		if goalsvc.IsExpectedMutationError(err) {
			return
		}
		s.recordGoalContinuationDispatchFailure(ctx, *plan, err)
		s.loggerFor(ctx).Warn("启动 Room Goal 自动续跑失败",
			"session_key", roundValue.SessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
	}
}

func (s *RealtimeService) recordGoalContinuationDispatchFailure(ctx context.Context, plan protocol.GoalContinuation, dispatchErr error) {
	if s == nil || s.goals == nil || dispatchErr == nil {
		return
	}
	reason := strings.TrimSpace(dispatchErr.Error())
	if reason == "" {
		reason = "Goal continuation dispatch failed before runtime start"
	}
	if _, err := s.goals.RecordContinuationFailure(ctx, plan.Goal.ID, plan.RoundID, reason, plan.Goal.ObjectiveRevision()); err != nil &&
		!goalsvc.IsExpectedMutationError(err) {
		s.loggerFor(ctx).Warn("记录 Room Goal 续跑投递失败原因失败",
			"session_key", plan.Goal.SessionKey,
			"goal_id", plan.Goal.ID,
			"round_id", plan.RoundID,
			"err", err,
		)
	}
}

// DispatchGoalContinuation 把共享 Room Goal 的隐藏续跑交给 Room 运行链路。
func (s *RealtimeService) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	if s == nil {
		return errors.New("room goal continuation dispatcher is not configured")
	}
	planner, ok := s.goals.(goalContinuationProvider)
	if !ok {
		return errors.New("room goal continuation provider is not configured")
	}
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()

	validated, err := goalsvc.ValidateContinuationForDispatch(
		ctx,
		planner,
		plan,
		func(candidate protocol.GoalContinuation) bool {
			return s.shouldDeferGoalContinuation(ctx, candidate.Goal.SessionKey, false)
		},
	)
	if err != nil || validated == nil {
		return err
	}
	if _, err = planner.ClaimContinuationPlan(ctx, *validated); err != nil {
		return err
	}
	if err := s.dispatchPreparedGoalContinuation(ctx, *validated); err != nil {
		return err
	}
	return nil
}

func (s *RealtimeService) dispatchPreparedGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindRoom || strings.TrimSpace(parsed.ConversationID) == "" {
		return errors.New("room goal continuation requires a room session key")
	}
	targetAgentIDs, collaborationContext := s.goalContinuationDispatchTarget(ctx, parsed.ConversationID, plan.Goal)
	goalContext := appendPromptSection(plan.Prompt, collaborationContext)
	if collaborationContext != "" {
		s.recordRoomGoalCollaborationRequired(ctx, plan.Goal.ID, plan.RoundID)
	}
	return s.handleChat(ctx, ChatRequest{
		SessionKey:            sessionKey,
		ConversationID:        parsed.ConversationID,
		GoalContext:           goalContext,
		GoalID:                plan.Goal.ID,
		GoalObjectiveRevision: plan.Goal.ObjectiveRevision(),
		TargetAgentIDs:        targetAgentIDs,
		RoundID:               plan.RoundID,
		DeliveryPolicy:        protocol.ChatDeliveryPolicyQueue,
		Internal:              true,
		InputOptions:          goalContinuationInputOptions(plan),
	})
}

func (s *RealtimeService) goalContinuationDispatchTarget(
	ctx context.Context,
	conversationID string,
	goal protocol.Goal,
) ([]string, string) {
	if s == nil || s.rooms == nil {
		return nil, ""
	}
	ctx, contextValue, err := s.internalConversationContext(ctx, conversationID, true)
	if err != nil || contextValue == nil {
		if err != nil {
			s.loggerFor(ctx).Warn("读取 Room Goal 续跑目标失败", "conversation_id", conversationID, "err", err)
		}
		return nil, ""
	}
	agentNameByID, _, err := s.buildAgentDirectory(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room Goal 续跑目标 Agent 失败", "conversation_id", conversationID, "err", err)
		return nil, ""
	}
	targetAgentID := goalContinuationTargetAgentID(contextValue, agentNameByID, &goal)
	if targetAgentID == "" {
		return nil, ""
	}
	return []string{targetAgentID}, buildRoomGoalCollaborationContext(agentNameByID, targetAgentID)
}

func (s *RealtimeService) recordRoomGoalCollaborationRequired(ctx context.Context, goalID string, roundID string) {
	if s == nil || s.goals == nil || strings.TrimSpace(goalID) == "" {
		return
	}
	if _, err := s.goals.RecordRoomGoalCollaborationRequired(ctx, goalID, roundID); err != nil &&
		!errors.Is(err, goalsvc.ErrGoalDisabled) &&
		!errors.Is(err, goalsvc.ErrGoalNotFound) &&
		!errors.Is(err, goalsvc.ErrGoalInvalidState) &&
		!errors.Is(err, goalsvc.ErrGoalVersionStale) {
		s.loggerFor(ctx).Warn("标记 Room Goal 协作要求失败",
			"goal_id", goalID,
			"round_id", roundID,
			"err", err,
		)
	}
}

func buildRoomGoalCollaborationContext(agentNameByID map[string]string, leadAgentID string) string {
	leadAgentID = strings.TrimSpace(leadAgentID)
	if leadAgentID == "" || len(agentNameByID) <= 1 {
		return ""
	}
	type memberLine struct {
		agentID string
		name    string
	}
	members := make([]memberLine, 0, len(agentNameByID)-1)
	for agentID, name := range agentNameByID {
		normalizedAgentID := strings.TrimSpace(agentID)
		if normalizedAgentID == "" || normalizedAgentID == leadAgentID {
			continue
		}
		members = append(members, memberLine{
			agentID: normalizedAgentID,
			name:    cmp.Or(strings.TrimSpace(name), normalizedAgentID),
		})
	}
	if len(members) == 0 {
		return ""
	}
	sort.Slice(members, func(i int, j int) bool {
		if members[i].name != members[j].name {
			return members[i].name < members[j].name
		}
		return members[i].agentID < members[j].agentID
	})
	lines := make([]string, 0, len(members))
	for _, member := range members {
		lines = append(lines, fmt.Sprintf("- @%s (agent_id=%s)", member.name, member.agentID))
	}
	leadName := cmp.Or(strings.TrimSpace(agentNameByID[leadAgentID]), leadAgentID)
	return strings.TrimSpace(fmt.Sprintf(`
Room Goal collaboration requirement:
- This Room Goal has multiple members. Visible collaboration is a required part of completing the Goal, not optional polish.
- Lead agent for this continuation: %s (agent_id=%s).
- Available public delegation targets:
%s
- If the room-visible history does not already contain a substantive reply from at least one non-lead member for this Goal, your public reply for this turn must @ exactly one target above and assign a concrete deliverable.
- Do not call the Goal update tool in the same turn as the first public delegation.
- Do not mark the Room Goal complete using only your own private work. Completion requires room-visible collaborator evidence plus your final audit.
`, leadName, leadAgentID, strings.Join(lines, "\n")))
}

func goalContinuationInputOptions(plan protocol.GoalContinuation) sdkprotocol.OutboundMessageOptions {
	return sdkprotocol.OutboundMessageOptions{
		Meta:           true,
		Synthetic:      plan.Synthetic,
		HiddenFromUser: plan.HiddenFromUser,
		Purpose:        plan.Purpose,
		Priority:       "internal",
		Metadata:       plan.Metadata,
	}
}
