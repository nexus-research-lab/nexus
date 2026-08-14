// INPUT: Room Goal 状态/lead、成员目录、协作者 active slot、显式输入队列与上一轮执行结果。
// OUTPUT: 启动 slot 前对齐的有效 lead，以及使用稳定或旧记录恢复的 Execution reservation、按复杂度和成员适配分工、在同 Goal 工作收敛后原子 claim 的隐藏 continuation。
// POS: Room 与 Goal 权限/状态机之间的续跑适配层。
package realtime

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ShouldDeferGoalContinuation 避免隐藏 Goal 续跑抢占显式输入，并按 Codex 语义跳过 Plan 模式续跑。
func (s *Service) ShouldDeferGoalContinuation(ctx context.Context, sessionKey string) bool {
	return s.shouldDeferGoalContinuation(ctx, sessionKey, true)
}

func (s *Service) shouldDeferGoalContinuation(ctx context.Context, sessionKey string, dispatchQueuedInput bool) bool {
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
	lease := s.lockRoomDispatch(sessionKey, parsed.ConversationID)
	defer lease.Unlock()
	return s.shouldDeferGoalContinuationLocked(ctx, sessionKey, parsed.ConversationID, dispatchQueuedInput)
}

// shouldDeferGoalContinuationLocked 在 conversation 派发闸门内判断续跑是否应等待。
func (s *Service) shouldDeferGoalContinuationLocked(
	ctx context.Context,
	sessionKey string,
	conversationID string,
	dispatchQueuedInput bool,
) bool {
	ctx, contextValue, err := s.internalConversationContext(ctx, conversationID, true)
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
	if provider, ok := s.goals.(currentGoalProvider); ok {
		currentGoal, goalErr := provider.CurrentOptional(ctx, sessionKey)
		if goalErr != nil {
			s.loggerFor(ctx).Warn("读取 Room Goal queue revision 失败", "session_key", sessionKey, "err", goalErr)
			return true
		}
		entries, err = s.pruneStaleGoalCollaborationQueueEntries(
			ctx,
			sessionKey,
			contextValue,
			entries,
			currentGoal,
		)
		if err != nil {
			s.loggerFor(ctx).Warn("清理过期 Room Goal queue 失败", "session_key", sessionKey, "err", err)
			return true
		}
	}
	if len(entries) == 0 {
		return s.shouldDeferGoalContinuationForTargetStateLocked(ctx, sessionKey, contextValue)
	}
	entry, ok := s.findDispatchableInputQueueEntry(
		sessionKey,
		conversationID,
		contextValue.Members,
		entries,
	)
	if !ok {
		return true
	}
	if dispatchQueuedInput {
		s.dispatchNextInputQueueItemLocked(
			contextWithExactQueueOwner(ctx, entry.Item.OwnerUserID),
			sessionKey,
			contextValue.Room.ID,
			contextValue.Conversation.ID,
		)
	}
	return true
}

func (s *Service) shouldDeferGoalContinuationForTargetState(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) bool {
	if contextValue == nil {
		return false
	}
	lease := s.lockRoomDispatch(sessionKey, contextValue.Conversation.ID)
	defer lease.Unlock()
	return s.shouldDeferGoalContinuationForTargetStateLocked(ctx, sessionKey, contextValue)
}

func (s *Service) shouldDeferGoalContinuationForTargetStateLocked(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) bool {
	if s == nil || contextValue == nil {
		return false
	}
	activeBlocker := s.activeRoomGoalBlocker(sessionKey, contextValue.Conversation.ID, "", "")
	if activeBlocker != "" {
		return true
	}
	currentGoal := s.currentRoomGoalForSession(ctx, sessionKey)
	if currentGoal != nil && s.roomGoalCollaborationInFlight(
		ctx,
		contextValue,
		*currentGoal,
	) {
		return true
	}
	if targetAgentID := goalContinuationMemberTargetAgentID(
		contextValue,
		currentGoal,
	); targetAgentID != "" && roomdomain.IsMemberParticipationPaused(
		contextValue.Members,
		targetAgentID,
	) {
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
	targetAgentID := goalContinuationTargetAgentID(contextValue, agentNameByID, currentGoal)
	if targetAgentID == "" {
		return true
	}
	if roomdomain.IsMemberParticipationPaused(
		contextValue.Members,
		targetAgentID,
	) {
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
	permissionMode := agentValue.Options.PermissionMode
	if override := protocol.SessionRuntimeSettingsFromOptions(
		roomSessionOptionsFromContext(contextValue, targetAgentID),
	).PermissionMode; override != "" {
		permissionMode = override
	}
	return goalsvc.ShouldIgnoreRuntimeForPermissionMode(permissionMode)
}

func (s *Service) roomGoalCollaborationInFlight(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	goal protocol.Goal,
) bool {
	if s == nil || s.publicHandoffs == nil || contextValue == nil {
		return false
	}
	inFlight, err := s.publicHandoffs.GoalCollaborationInFlightAll(
		contextValue.Room.OwnerUserID,
		protocol.GoalCollaborationBinding{
			GoalID:            goal.ID,
			ObjectiveRevision: goal.ObjectiveRevision(),
		},
	)
	if err != nil {
		s.loggerFor(ctx).Warn(
			"读取 Room Goal collaboration fence 失败，延后自动续跑",
			"conversation_id", contextValue.Conversation.ID,
			"goal_id", goal.ID,
			"err", err,
		)
		return true
	}
	return inFlight
}

// GoalContinuationTargetMissing 判断共享 Room Goal 的 conversation 是否已被删除。
func (s *Service) GoalContinuationTargetMissing(ctx context.Context, sessionKey string) (bool, error) {
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
func (s *Service) GoalContinuationConversationMissing(ctx context.Context, conversationID string) (bool, error) {
	conversationID = strings.TrimSpace(conversationID)
	if s == nil || s.rooms == nil || conversationID == "" {
		return false, nil
	}
	_, contextValue, err := s.internalConversationContext(ctx, conversationID, true)
	if errors.Is(err, roomsvc.ErrRoomNotFound) || errors.Is(err, roomsvc.ErrConversationNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return contextValue == nil, nil
}

// goalContinuationMemberTargetAgentID 只用 Room 持久成员身份定位续跑责任人，
// 让 participation gate 不依赖易失的 Agent 目录加载结果。
func goalContinuationMemberTargetAgentID(
	contextValue *protocol.ConversationContextAggregate,
	goal *protocol.Goal,
) string {
	if contextValue == nil {
		return ""
	}
	memberAgentIDs := make(map[string]struct{}, len(contextValue.Members))
	for _, member := range contextValue.Members {
		if member.MemberType != protocol.MemberTypeAgent {
			continue
		}
		agentID := strings.TrimSpace(member.MemberAgentID)
		if agentID != "" {
			memberAgentIDs[agentID] = struct{}{}
		}
	}
	if goal != nil {
		leadAgentID := goalsvc.RoomLeadAgentID(*goal)
		if _, ok := memberAgentIDs[leadAgentID]; ok {
			return leadAgentID
		}
	}
	hostAgentID := strings.TrimSpace(contextValue.Room.HostAgentID)
	if _, ok := memberAgentIDs[hostAgentID]; ok {
		return hostAgentID
	}
	if len(memberAgentIDs) == 1 {
		for agentID := range memberAgentIDs {
			return agentID
		}
	}
	return ""
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

type goalByIDProvider interface {
	GoalByIDForOwner(context.Context, string, string) (*protocol.Goal, error)
}

type roomGoalLeadSetter interface {
	SetRoomGoalLead(context.Context, string, string) (*protocol.Goal, error)
}

func (s *Service) reconcileRoomGoalLead(
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
	_, err = setter.SetRoomGoalLead(ctx, goal.ID, leadAgentID)
	return err
}

func (s *Service) currentRoomGoalForSession(ctx context.Context, sessionKey string) *protocol.Goal {
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

// roomGoalCollaborationBindingIsCurrent is the fail-closed admission check for
// durable Room collaboration work. Attribution survives longer than one
// process and can therefore outlive a pause, terminal transition, or objective
// retarget. Every wake/recovery boundary must re-read the canonical Goal before
// turning that old fact into a new Agent round.
func (s *Service) roomGoalCollaborationBindingIsCurrent(
	ctx context.Context,
	conversationID string,
	binding *protocol.GoalCollaborationBinding,
) (bool, error) {
	binding = protocol.NormalizeGoalCollaborationBinding(binding)
	if binding == nil {
		return true, nil
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return false, errors.New("conversation_id is required for Goal collaboration admission")
	}
	goal, err := s.goalForCollaborationBinding(ctx, conversationID, binding)
	if err != nil {
		return false, fmt.Errorf("read current Room Goal for collaboration admission: %w", err)
	}
	return roomGoalCollaborationBindingMatchesGoal(goal, binding), nil
}

func (s *Service) goalForCollaborationBinding(
	ctx context.Context,
	conversationID string,
	binding *protocol.GoalCollaborationBinding,
) (*protocol.Goal, error) {
	if provider, ok := s.goals.(goalByIDProvider); ok {
		ownerUserID, err := s.roomCollaborationOwnerUserID(ctx, conversationID)
		if err != nil {
			return nil, err
		}
		return provider.GoalByIDForOwner(ctx, binding.GoalID, ownerUserID)
	}
	provider, ok := s.goals.(currentGoalProvider)
	if !ok {
		return nil, errors.New("current Goal provider is required for Goal collaboration admission")
	}
	// Compatibility for lightweight providers and old tests. Production uses
	// GoalByID so a trusted Goal handoff may cross conversation boundaries.
	return provider.CurrentOptional(
		ctx,
		protocol.BuildRoomSharedSessionKey(conversationID),
	)
}

func (s *Service) roomCollaborationOwnerUserID(
	ctx context.Context,
	conversationID string,
) (string, error) {
	if ownerUserID := strings.TrimSpace(authctx.OwnerUserID(ctx)); ownerUserID != "" {
		return ownerUserID, nil
	}
	_, contextValue, err := s.internalConversationContext(ctx, conversationID, true)
	if err != nil {
		return "", err
	}
	if contextValue == nil || strings.TrimSpace(contextValue.Room.OwnerUserID) == "" {
		return "", errors.New("Room owner is required for Goal collaboration admission")
	}
	return strings.TrimSpace(contextValue.Room.OwnerUserID), nil
}

func roomGoalCollaborationBindingMatchesGoal(
	goal *protocol.Goal,
	binding *protocol.GoalCollaborationBinding,
) bool {
	binding = protocol.NormalizeGoalCollaborationBinding(binding)
	return goal != nil && binding != nil &&
		protocol.NormalizeGoalStatus(goal.Status) == protocol.GoalStatusActive &&
		!goalsvc.GoalObjectiveTransitionPending(*goal) &&
		strings.TrimSpace(goal.ID) == binding.GoalID &&
		goal.ObjectiveRevision() == binding.ObjectiveRevision
}

func (s *Service) dispatchPostRoundWork(ctx context.Context, roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	if roundValue.RunningSubagents.Load() {
		return
	}
	continuationSessionKey := roundValue.SessionKey
	if !roomRoundHasGoalAuthority(roundValue) {
		goal, reconciled := s.reconcileRoomGoalCollaborationRound(ctx, roundValue)
		if !reconciled {
			return
		}
		continuationSessionKey = goal.SessionKey
	}
	if roomRoundHasPendingGoalCollaboration(roundValue) {
		return
	}
	if s.ShouldDeferGoalContinuation(ctx, continuationSessionKey) {
		return
	}
	s.dispatchGoalContinuationForSession(
		ctx,
		continuationSessionKey,
		roundValue.RoundID,
	)
}

func roomRoundHasPendingGoalCollaboration(roundValue *activeRoomRound) bool {
	if roundValue == nil {
		return false
	}
	for _, slot := range roundValue.Slots {
		if slot != nil && slot.hasPendingGoalCollaboration() {
			return true
		}
	}
	return false
}

// reconcileRoomGoalCollaborationRound reconnects one completed raw handoff to
// its exact active Goal. The target round stays unbound: this path never grants
// MCP Goal mutation authority and only a later continuation can mutate the
// Goal. Error/no-reply terminal rounds still return control to that
// continuation; handback only clears stale empty-progress suppression and
// preserves continuation limits. Only public substantive output satisfies
// Room collaboration evidence.
func (s *Service) reconcileRoomGoalCollaborationRound(
	ctx context.Context,
	roundValue *activeRoomRound,
) (*protocol.Goal, bool) {
	if s == nil || s.goals == nil || roundValue == nil ||
		!protocol.IsRoomSharedSessionKey(roundValue.SessionKey) {
		return nil, false
	}
	var binding *protocol.GoalCollaborationBinding
	for _, slot := range roundValue.Slots {
		candidate := slot.goalCollaborationBinding()
		if candidate == nil {
			continue
		}
		if binding != nil && *binding != *candidate {
			return nil, false
		}
		binding = candidate
	}
	if binding == nil {
		return nil, false
	}
	goal, goalErr := s.goalForCollaborationBinding(
		ctx,
		roundValue.ConversationID,
		binding,
	)
	if goalErr != nil || !roomGoalCollaborationBindingMatchesGoal(goal, binding) {
		s.markRoomGoalCollaborationRoundHandbackSettled(roundValue, binding)
		return nil, false
	}
	s.releaseActiveGoalCollaborationSources(roundValue, binding)
	for _, slot := range roundValue.Slots {
		if slot == nil || !slot.isTerminal() {
			continue
		}
		candidate := slot.goalCollaborationBinding()
		if candidate == nil || *candidate != *binding {
			continue
		}
		lastAssistant := slot.lastGoalAssistantMessage()
		if roomdomain.IsNoReplyAssistantMessage(lastAssistant) ||
			strings.TrimSpace(messageutil.ExtractAssistantDisplayText(lastAssistant)) == "" {
			continue
		}
		if slot.getStatus() == "finished" && roomSlotPublishesPublicOutput(slot) {
			if _, err := s.goals.RecordRoomGoalCollaborationEvidence(
				ctx,
				binding.GoalID,
				slot.AgentRoundID,
				slot.AgentID,
				binding.ObjectiveRevision,
			); err != nil && !goalsvc.IsExpectedMutationError(err) {
				s.loggerFor(ctx).Warn(
					"记录 Room Goal handoff 协作证据失败",
					"session_key", roundValue.SessionKey,
					"goal_id", binding.GoalID,
					"round_id", slot.AgentRoundID,
					"agent_id", slot.AgentID,
					"err", err,
				)
			}
		}
	}
	if _, err := s.goals.RecordRoomGoalCollaborationHandback(
		ctx,
		binding.GoalID,
		roundValue.RoundID,
		binding.ObjectiveRevision,
	); err != nil {
		if !goalsvc.IsExpectedMutationError(err) {
			s.loggerFor(ctx).Warn(
				"恢复 Room Goal handoff 后续跑失败",
				"session_key", roundValue.SessionKey,
				"goal_id", binding.GoalID,
				"round_id", roundValue.RoundID,
				"err", err,
			)
		}
		return nil, false
	}
	s.markRoomGoalCollaborationRoundHandbackSettled(roundValue, binding)
	return goal, true
}

func (s *Service) markRoomGoalCollaborationRoundHandbackSettled(
	roundValue *activeRoomRound,
	binding *protocol.GoalCollaborationBinding,
) {
	if s == nil || s.publicHandoffs == nil || roundValue == nil ||
		protocol.NormalizeGoalCollaborationBinding(binding) == nil {
		return
	}
	for _, slot := range roundValue.Slots {
		if slot == nil {
			continue
		}
		candidate := slot.goalCollaborationBinding()
		if candidate == nil || *candidate != *binding {
			continue
		}
		handoffID := strings.TrimSpace(slot.handoffID())
		if handoffID == "" {
			continue
		}
		if err := s.publicHandoffs.MarkGoalHandbackSettled(
			roundValue.OwnerUserID,
			roundValue.ConversationID,
			handoffID,
		); err != nil {
			s.loggerFor(context.Background()).Warn(
				"记录 Room Goal handback 收口失败",
				"conversation_id", roundValue.ConversationID,
				"handoff_id", handoffID,
				"err", err,
			)
		}
	}
}

// releaseActiveGoalCollaborationSources closes the in-memory source barrier
// for this exact root/revision. If the source is still running, its own
// post-round path will continue later; if it already ended, this target round
// becomes the continuation trigger.
func (s *Service) releaseActiveGoalCollaborationSources(
	roundValue *activeRoomRound,
	binding *protocol.GoalCollaborationBinding,
) {
	if s == nil || roundValue == nil || binding == nil {
		return
	}
	for _, candidateRound := range s.rounds.snapshot() {
		if candidateRound == nil || candidateRound == roundValue {
			continue
		}
		if roomRootRoundID(candidateRound) != roomRootRoundID(roundValue) {
			continue
		}
		if ownerUserID := strings.TrimSpace(roundValue.OwnerUserID); ownerUserID != "" &&
			strings.TrimSpace(candidateRound.OwnerUserID) != ownerUserID {
			continue
		}
		for _, slot := range candidateRound.Slots {
			if slot == nil || !slot.hasPendingGoalCollaboration() {
				continue
			}
			candidate := goalCollaborationBindingForSlot(candidateRound, slot)
			if candidate != nil && *candidate == *binding {
				slot.clearPendingGoalCollaboration()
			}
		}
	}
}

func roomRoundHasGoalAuthority(roundValue *activeRoomRound) bool {
	if roundValue == nil {
		return false
	}
	for _, slot := range roundValue.Slots {
		if slot != nil && slot.goalMutationAuthority().valid() {
			return true
		}
	}
	return false
}

func (s *Service) dispatchGoalContinuationForSession(
	ctx context.Context,
	sessionKey string,
	causedByRoundID string,
) {
	if s == nil || strings.TrimSpace(sessionKey) == "" || s.goals == nil {
		return
	}
	planner, ok := s.goals.(goalContinuationProvider)
	if !ok {
		return
	}
	plan, err := goalsvc.PrepareContinuationForDispatch(
		ctx,
		planner,
		sessionKey,
		causedByRoundID,
		func(plan protocol.GoalContinuation) bool {
			return s.ShouldDeferGoalContinuation(ctx, plan.Goal.SessionKey)
		},
	)
	if err != nil {
		if goalsvc.IsExpectedMutationError(err) {
			return
		}
		s.loggerFor(ctx).Warn("准备 Room Goal 自动续跑失败",
			"session_key", sessionKey,
			"round_id", causedByRoundID,
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
			"session_key", sessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
	}
}

func (s *Service) recordGoalContinuationDispatchFailure(ctx context.Context, plan protocol.GoalContinuation, dispatchErr error) {
	if s == nil || s.goals == nil || dispatchErr == nil {
		return
	}
	reason := strings.TrimSpace(dispatchErr.Error())
	if reason == "" {
		reason = "Goal continuation dispatch failed before runtime start"
	}
	if err := retryRoomGoalContinuationPlan(ctx, s.goals, plan, reason); err != nil &&
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
func (s *Service) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	if s == nil {
		return errors.New("room goal continuation dispatcher is not configured")
	}
	planner, ok := s.goals.(goalContinuationProvider)
	if !ok {
		return errors.New("room goal continuation provider is not configured")
	}
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	lease := s.lockRoomDispatch(sessionKey, parsed.ConversationID)
	defer lease.Unlock()

	validated, err := goalsvc.ValidateContinuationForDispatch(
		ctx,
		planner,
		plan,
		func(_ protocol.GoalContinuation) bool {
			// ValidateContinuationForDispatch 只回调当前候选；沿用外层
			// conversation 闸门，避免同一把非可重入锁被再次获取。
			return s.shouldDeferGoalContinuationLocked(ctx, sessionKey, parsed.ConversationID, false)
		},
	)
	if err != nil || validated == nil {
		return err
	}
	if _, err = planner.ClaimContinuationPlan(ctx, *validated); err != nil {
		return err
	}
	if err := s.dispatchPreparedGoalContinuationLocked(ctx, planner, *validated); err != nil {
		return err
	}
	return nil
}

type durableRoomGoalContinuationLauncher interface {
	MarkContinuationPlanStarted(context.Context, protocol.GoalContinuation) error
	RetryContinuationPlan(context.Context, protocol.GoalContinuation, string) error
}

type durableRoomGoalContinuationSettler interface {
	SettleContinuationPlan(context.Context, string, string, int64) error
}

func settleRoomGoalContinuationAfterRuntime(ctx context.Context, provider goalContextProvider, goalID, roundID string, objectiveRevision int64) error {
	if durable, ok := provider.(durableRoomGoalContinuationSettler); ok {
		return durable.SettleContinuationPlan(ctx, goalID, roundID, objectiveRevision)
	}
	return nil
}

func markRoomGoalContinuationStarted(ctx context.Context, provider goalContinuationProvider, plan protocol.GoalContinuation) error {
	if durable, ok := provider.(durableRoomGoalContinuationLauncher); ok {
		return durable.MarkContinuationPlanStarted(ctx, plan)
	}
	return nil
}

func retryRoomGoalContinuationPlan(ctx context.Context, provider goalContextProvider, plan protocol.GoalContinuation, reason string) error {
	if durable, ok := provider.(durableRoomGoalContinuationLauncher); ok {
		return durable.RetryContinuationPlan(ctx, plan, reason)
	}
	_, err := provider.RecordContinuationRuntimeFailure(
		ctx,
		plan.Goal.ID,
		goalsvc.ContinuationRuntimeIdentity{
			ReceiptRoundID: plan.RoundID,
			AuditRoundID:   plan.RoundID,
		},
		reason,
		plan.Goal.ObjectiveRevision(),
	)
	return err
}

// dispatchPreparedGoalContinuationLocked 在 conversation 派发闸门内启动续跑。
func (s *Service) dispatchPreparedGoalContinuationLocked(
	ctx context.Context,
	planner goalContinuationProvider,
	plan protocol.GoalContinuation,
) error {
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindRoom || strings.TrimSpace(parsed.ConversationID) == "" {
		return errors.New("room goal continuation requires a room session key")
	}
	targetAgentIDs, collaborationContext := s.goalContinuationDispatchTarget(ctx, parsed.ConversationID, plan.Goal)
	goalContext := appendPromptSection(plan.Prompt, collaborationContext)
	return s.handleChatLocked(ctx, ChatRequest{
		SessionKey:            sessionKey,
		ConversationID:        parsed.ConversationID,
		GoalContext:           goalContext,
		GoalID:                plan.Goal.ID,
		GoalObjectiveRevision: plan.Goal.ObjectiveRevision(),
		ExecutionID:           strings.TrimSpace(plan.ExecutionID),
		TargetAgentIDs:        targetAgentIDs,
		CoordinatorAgentID:    firstRoomTargetAgentID(targetAgentIDs),
		RoundID:               plan.RoundID,
		DeliveryPolicy:        protocol.ChatDeliveryPolicyQueue,
		Internal:              true,
		InputOptions:          goalContinuationInputOptions(plan),
		continuationStartAdmission: func(admissionCtx context.Context) error {
			return markRoomGoalContinuationStarted(admissionCtx, planner, plan)
		},
	})
}

func (s *Service) goalContinuationDispatchTarget(
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
Room Goal collaboration options:
- Lead agent for this continuation: %s (agent_id=%s).
- Available conversation targets:
%s
- Collaboration is optional. The current lead may complete the Goal when the objective is satisfied; non-lead evidence is audit context, not a completion gate.
- Before choosing a route, assess task complexity, separable work, member fit, and whether responsibility must persist. An @mention is conversation-only and never creates an Assignment, but a substantive public reply to this Goal-attributed handoff is recorded as collaboration evidence.
- Use @ for a genuinely untracked contribution, or create/continue a managed WorkGraph and use assign_work for one distinct Ready Work Item when the member must own an accountable deliverable.
- Once accountable work is assigned, do not duplicate that deliverable. Use lead time for coordination, unblocking, integration, and verification; take over only through the managed control path when necessary.
- Do not call the Goal update tool while an @ handoff, Assignment, queue item, or wake for this Goal is still running. Finish or explicitly cancel that work first.
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
