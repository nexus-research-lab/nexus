// INPUT: 已完成 Agent 输出中的显式公区 @ 与目标 Agent 当前执行态。
// OUTPUT: 任意非 self 成员间幂等 handoff、同 Agent 串行 guide/queue/新轮唤醒，以及保留 root usage scope 的 pending wake 到 active slot 原子交接。
// POS: Room Agent 间公开协作的显式路由与纯资源护栏入口；不解释或限制业务协作拓扑。
package realtime

import (
	"cmp"
	"context"
	"errors"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	// 多阶段 Room 会自然跨越数十次唤醒；这些上限只拦截失控链路。
	roomMaxWakeHops      = 128
	roomMaxHandoffFanout = 8
	roomMaxRootHandoffs  = 256
)

type pendingPublicMentionSlot struct {
	wake          publicMentionWake
	targetAgentID string
	sessionRecord protocol.SessionRecord
	agentValue    *protocol.Agent
}

func (s *Service) collectPublicMentionWakes(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	if roundValue == nil || roundValue.Context == nil || slot == nil {
		return nil
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return err
	}
	if !roomdomain.IsFinalPublicAssistantMessage(message) {
		return nil
	}
	if slot.getStatus() != "finished" {
		// 只有 source slot 成功收尾才允许创建 target handoff。
		return nil
	}
	content := strings.TrimSpace(roomdomain.ExtractAssistantResultText(message))
	if content == "" {
		return nil
	}
	if err := s.annotatePublicAssistantMessage(roundValue, slot, message); err != nil {
		return err
	}
	// result 投影可能晚于首个 assistant 快照到达；此时实时事件已经拿到标注，
	// 但首条 transcript 引用仍是旧快照。追加同 message_id 的引用作为可压缩更新，
	// 让历史回放与实时渲染保持同一份 agent_mentions。
	if len(protocolAgentMentions(message["agent_mentions"])) > 0 {
		if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
			return err
		}
		if err := s.persistSharedDurableMessage(
			roundValue.OwnerUserID,
			roundValue.ConversationID,
			slot,
			message,
		); err != nil {
			return err
		}
	}
	// 标注阶段会剥离 fanout 控制标记并重写 span；必须用清理后的正文
	// 生成 queue trigger，避免隐藏标记进入目标 Agent 上下文。
	content = strings.TrimSpace(roomdomain.ExtractAssistantResultText(message))
	if content == "" {
		return nil
	}
	wakes := publicMentionWakesFromMessage(roundValue, slot, message, content)
	if len(wakes) == 0 {
		return nil
	}
	for _, wake := range wakes {
		if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
			return err
		}
		if s.publicHandoffs != nil {
			if err := s.publicHandoffs.MarkSourceFinished(
				roundValue.OwnerUserID,
				roundValue.ConversationID,
				wake.HandoffID,
			); err != nil {
				return err
			}
		}
		s.enqueuePublicMentionWake(roundValue, wake)
	}
	// source slot 完成即触发，不等待同一 root 的其他 slot。
	s.startQueuedPublicMentionWakes(ctx, roundValue)
	return nil
}

func publicMentionWakesFromMessage(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
	content string,
) []publicMentionWake {
	messageID := strings.TrimSpace(anyString(message["message_id"]))
	if messageID == "" || roundValue == nil || slot == nil {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]publicMentionWake, 0)
	for _, mention := range protocolAgentMentions(message["agent_mentions"]) {
		targetAgentID := strings.TrimSpace(mention.AgentID)
		if targetAgentID == "" || targetAgentID == strings.TrimSpace(slot.AgentID) ||
			!roomdomain.IsMemberAgent(roundValue.Context.Members, targetAgentID) {
			continue
		}
		handoffID := strings.TrimSpace(mention.HandoffID)
		if handoffID == "" {
			// 用户消息、旧历史或畸形 annotation 可能没有 handoff_id；
			// 它们只作展示，不能绕过当前服务端派生并写入 ledger 的交接。
			continue
		}
		if _, exists := seen[targetAgentID]; exists {
			continue
		}
		seen[targetAgentID] = struct{}{}
		result = append(result, publicMentionWake{
			HandoffID:     handoffID,
			TriggerType:   "public_mention",
			QueueSource:   protocol.InputQueueSourceAgentPublicMention,
			SourceAgentID: strings.TrimSpace(slot.AgentID),
			TargetAgentID: targetAgentID,
			Content:       content,
			MessageID:     messageID,
		})
	}
	return result
}

func protocolAgentMentions(value any) []protocol.AgentMention {
	result := make([]protocol.AgentMention, 0)
	switch typed := value.(type) {
	case []protocol.AgentMention:
		return append(result, typed...)
	case []map[string]any:
		for _, payload := range typed {
			result = append(result, protocol.AgentMention{
				AgentID:   strings.TrimSpace(anyString(payload["agent_id"])),
				Label:     strings.TrimSpace(anyString(payload["label"])),
				HandoffID: strings.TrimSpace(anyString(payload["handoff_id"])),
			})
		}
	case []any:
		for _, item := range typed {
			payload, ok := item.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, protocol.AgentMention{
				AgentID:   strings.TrimSpace(anyString(payload["agent_id"])),
				Label:     strings.TrimSpace(anyString(payload["label"])),
				HandoffID: strings.TrimSpace(anyString(payload["handoff_id"])),
			})
		}
	}
	return result
}

func (s *Service) enqueuePublicMentionWake(roundValue *activeRoomRound, wake publicMentionWake) {
	if roundValue == nil || strings.TrimSpace(wake.TargetAgentID) == "" {
		return
	}
	s.rounds.enqueuePublicMention(roundValue, wake)
}

func (s *Service) takePublicMentionWakes(roundValue *activeRoomRound) []publicMentionWake {
	if roundValue == nil {
		return nil
	}
	return s.rounds.takePublicMentions(roundValue)
}

func (s *Service) startQueuedPublicMentionWakes(ctx context.Context, roundValue *activeRoomRound) bool {
	if roundValue == nil {
		return false
	}
	lease := s.lockRoomDispatch(roundValue.SessionKey, roundValue.ConversationID)
	defer lease.Unlock()
	return s.startQueuedPublicMentionWakesLocked(ctx, roundValue)
}

// startQueuedPublicMentionWakesLocked 在 conversation 派发闸门内启动待处理唤醒。
func (s *Service) startQueuedPublicMentionWakesLocked(ctx context.Context, roundValue *activeRoomRound) bool {
	wakes := s.takePublicMentionWakes(roundValue)
	if len(wakes) == 0 {
		return false
	}
	if err := s.startPublicMentionRoundLocked(ctx, roundValue, wakes, true); err != nil {
		s.loggerFor(ctx).Error("启动 Room 公区 @ 唤醒失败",
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
			"root", roomRootRoundID(roundValue),
			"err", err,
		)
		return false
	}
	return true
}

func (s *Service) startPublicMentionRound(
	ctx context.Context,
	parentRound *activeRoomRound,
	wakes []publicMentionWake,
) error {
	if parentRound == nil {
		return nil
	}
	lease := s.lockRoomDispatch(parentRound.SessionKey, parentRound.ConversationID)
	defer lease.Unlock()
	return s.startPublicMentionRoundLocked(ctx, parentRound, wakes, true)
}

func (s *Service) startPublicMentionRoundLocked(
	ctx context.Context,
	parentRound *activeRoomRound,
	wakes []publicMentionWake,
	refreshContextOptions ...bool,
) error {
	if parentRound == nil || parentRound.Context == nil || len(wakes) == 0 {
		return nil
	}
	refreshContext := true
	if len(refreshContextOptions) > 0 {
		refreshContext = refreshContextOptions[0]
	}
	var err error
	if refreshContext && s.rooms != nil {
		currentContextValue, lookupErr := s.rooms.GetConversationContextForSystem(
			ctx,
			parentRound.ConversationID,
		)
		if lookupErr != nil {
			return lookupErr
		}
		if currentContextValue != nil {
			if err = requireGroupRoomContext(currentContextValue); err != nil {
				return err
			}
			parentRound.Context = currentContextValue
			parentRound.RoomID = currentContextValue.Room.ID
			parentRound.OwnerUserID = currentContextValue.Room.OwnerUserID
		}
	}
	ctx = contextWithExactQueueOwner(ctx, parentRound.OwnerUserID)
	admittedWakes := make([]publicMentionWake, 0, len(wakes))
	for _, wake := range wakes {
		if wake.ReviewBinding != nil {
			if err := s.authorizeManagedExecutionReviewTarget(
				ctx,
				parentRound,
				wake.TargetAgentID,
				wake.ReviewBinding,
			); err != nil {
				if !isPermanentExecutionAdmissionError(err) {
					return err
				}
				s.terminalizeRejectedExecutionWake(parentRound, wake)
				s.loggerFor(ctx).Info(
					"拒绝迟到的 Execution review return",
					"review_dispatch_id",
					wake.ReviewBinding.ReviewDispatchID,
					"target_agent_id",
					wake.TargetAgentID,
					"err",
					err,
				)
				continue
			}
			admittedWakes = append(admittedWakes, wake)
			continue
		}
		if err := s.authorizeManagedExecutionTarget(
			ctx,
			parentRound,
			wake.TargetAgentID,
			wake.WorkBinding,
		); err != nil {
			if wake.WorkBinding == nil || !isPermanentExecutionAdmissionError(err) {
				return err
			}
			s.terminalizeRejectedExecutionWake(parentRound, wake)
			s.loggerFor(ctx).Info(
				"拒绝迟到的 Execution work dispatch",
				"dispatch_id",
				wake.WorkBinding.DispatchID,
				"target_agent_id",
				wake.TargetAgentID,
				"err",
				err,
			)
			continue
		}
		admittedWakes = append(admittedWakes, wake)
	}
	wakes = admittedWakes
	if len(wakes) == 0 {
		return nil
	}
	wakes, err = s.admitPublicMentionWakes(ctx, parentRound, wakes)
	if err != nil {
		return err
	}
	if len(wakes) == 0 {
		return nil
	}
	// root admission 已经先处理幂等、self、fanout 与总量；hop 只作为
	// 跨重启或异常数据的最后兜底，不能承担业务拓扑判断。
	if parentRound.HopIndex >= roomMaxWakeHops {
		s.terminalizePublicMentionWakes(
			ctx,
			parentRound.OwnerUserID,
			parentRound.ConversationID,
			wakes,
			"error",
		)
		s.loggerFor(ctx).Warn("Room 唤醒达到跳数上限",
			"r", parentRound.RoomID,
			"c", parentRound.ConversationID,
			"root", roomRootRoundID(parentRound),
		)
		return nil
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(parentRound.ConversationID)
	contextValue := parentRound.Context
	wakes, err = s.queueBusyPublicMentionWakes(ctx, parentRound, sessionKey, wakes)
	if err != nil {
		return err
	}
	if len(wakes) == 0 {
		s.logQueuedPublicMentionWakes(ctx, parentRound, sessionKey)
		return nil
	}
	claimedWakes := make([]publicMentionWake, 0, len(wakes))
	for _, wake := range wakes {
		if s.publicHandoffs == nil || strings.TrimSpace(wake.HandoffID) == "" {
			claimedWakes = append(claimedWakes, wake)
			continue
		}
		_, claimed, claimErr := s.publicHandoffs.Claim(
			parentRound.OwnerUserID,
			parentRound.ConversationID,
			wake.HandoffID,
		)
		if claimErr != nil {
			return claimErr
		}
		if claimed {
			claimedWakes = append(claimedWakes, wake)
		}
	}
	wakes = claimedWakes
	if len(wakes) == 0 {
		return nil
	}
	agentNameByID, agentByID, err := s.buildAgentDirectory(ctx, contextValue)
	if err != nil {
		return err
	}
	publicHistory, err := s.roomHistory.ReadMessages(
		contextValue.Room.OwnerUserID,
		contextValue.Conversation.ID,
		nil,
	)
	if err != nil {
		return err
	}
	pendingSlots := buildPendingPublicMentionSlots(contextValue, wakes, agentByID)
	availableTargets := make(map[string]struct{}, len(pendingSlots))
	for _, pendingSlot := range pendingSlots {
		availableTargets[pendingSlot.targetAgentID] = struct{}{}
	}
	for _, wake := range wakes {
		if _, ok := availableTargets[strings.TrimSpace(wake.TargetAgentID)]; ok ||
			s.publicHandoffs == nil || strings.TrimSpace(wake.HandoffID) == "" {
			continue
		}
		if err := s.publicHandoffs.MarkTerminal(
			parentRound.OwnerUserID,
			parentRound.ConversationID,
			wake.HandoffID,
			"error",
		); err != nil {
			s.loggerFor(ctx).Warn("目标 Agent 不可用，收口 Room handoff 失败", "handoff_id", wake.HandoffID, "err", err)
		}
	}
	if len(pendingSlots) == 0 {
		s.logMissingPublicMentionSlots(ctx, sessionKey, contextValue, len(wakes))
		return nil
	}
	roundID := roomWakeRoundID(wakes)
	activeRound := newPublicMentionRound(parentRound, sessionKey, roundID)
	if strings.TrimSpace(activeRound.OwnerUserID) == "" {
		activeRound.OwnerUserID = authctx.OwnerUserID(ctx)
	}
	targetAgentIDs, pending := addPublicMentionSlots(activeRound, contextValue, pendingSlots)
	if !s.launchPublicMentionRound(
		ctx,
		activeRound,
		wakes,
		pendingSlots,
		targetAgentIDs,
		pending,
		publicHistory,
		agentNameByID,
		agentByID,
	) {
		return runtimectx.ErrRuntimeSessionClosing
	}
	if s.publicHandoffs != nil {
		for _, wake := range wakes {
			if strings.TrimSpace(wake.HandoffID) == "" {
				continue
			}
			if err := s.publicHandoffs.MarkStarted(
				activeRound.OwnerUserID,
				activeRound.ConversationID,
				wake.HandoffID,
				roundID,
			); err != nil {
				s.loggerFor(ctx).Warn("记录 Room handoff 启动状态失败", "handoff_id", wake.HandoffID, "err", err)
			}
		}
	}
	return nil
}

func isPermanentExecutionAdmissionError(err error) bool {
	if errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
		return true
	}
	var domainErr *orchestrationsvc.DomainError
	return errors.As(err, &domainErr)
}

func (s *Service) terminalizeRejectedExecutionWake(
	parentRound *activeRoomRound,
	wake publicMentionWake,
) {
	if s == nil || s.publicHandoffs == nil || parentRound == nil ||
		strings.TrimSpace(wake.HandoffID) == "" {
		return
	}
	_ = s.publicHandoffs.MarkTerminal(
		parentRound.OwnerUserID,
		parentRound.ConversationID,
		wake.HandoffID,
		"interrupted",
	)
}

// admitPublicMentionWakes 在真正 claim 前做幂等、self 与 root 级资源护栏。
// 只有显式 handoff 的公区 @ 进入这里；directed message 仍由自己的路由规则管理。
func (s *Service) admitPublicMentionWakes(
	ctx context.Context,
	parentRound *activeRoomRound,
	wakes []publicMentionWake,
) ([]publicMentionWake, error) {
	if parentRound == nil || len(wakes) == 0 {
		return wakes, nil
	}
	guardedCount := 0
	for _, wake := range wakes {
		if normalizeWakeQueueSource(wake) == protocol.InputQueueSourceAgentPublicMention &&
			strings.TrimSpace(wake.HandoffID) != "" {
			guardedCount++
		}
	}
	if guardedCount == 0 {
		return wakes, nil
	}
	rootRoundID := roomRootRoundID(parentRound)
	edges := make([]workspacestore.RoomPublicHandoff, 0)
	if s.publicHandoffs != nil {
		var err error
		edges, err = s.publicHandoffs.ListRoot(
			parentRound.OwnerUserID,
			parentRound.ConversationID,
			rootRoundID,
		)
		if err != nil {
			return nil, err
		}
	}
	existingByID := make(map[string]workspacestore.RoomPublicHandoff, len(edges))
	historicalRootHandoffs := 0
	for _, edge := range edges {
		if handoffID := strings.TrimSpace(edge.HandoffID); handoffID != "" {
			existingByID[handoffID] = edge
		}
	}
	currentWakeIDs := make(map[string]struct{}, len(wakes))
	for _, wake := range wakes {
		if handoffID := strings.TrimSpace(wake.HandoffID); handoffID != "" {
			currentWakeIDs[handoffID] = struct{}{}
		}
	}
	for _, edge := range edges {
		if _, current := currentWakeIDs[strings.TrimSpace(edge.HandoffID)]; current {
			continue
		}
		historicalRootHandoffs++
	}
	accepted := make([]publicMentionWake, 0, len(wakes))
	acceptedGuarded := 0
	acceptedIDs := make(map[string]struct{}, len(wakes))
	for _, wake := range wakes {
		if normalizeWakeQueueSource(wake) != protocol.InputQueueSourceAgentPublicMention ||
			strings.TrimSpace(wake.HandoffID) == "" {
			accepted = append(accepted, wake)
			continue
		}
		handoffID := strings.TrimSpace(wake.HandoffID)
		if _, duplicate := acceptedIDs[handoffID]; duplicate {
			// 同一批次的重复 span 只保留一份，Claim 仍负责跨进程去重。
			continue
		}
		acceptedIDs[handoffID] = struct{}{}
		sourceAgentID := strings.TrimSpace(wake.SourceAgentID)
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		existing, hasExisting := existingByID[handoffID]
		if hasExisting {
			if strings.TrimSpace(existing.RootRoundID) != rootRoundID ||
				strings.TrimSpace(existing.SourceAgentID) != sourceAgentID ||
				strings.TrimSpace(existing.TargetAgentID) != targetAgentID {
				s.terminalizePublicMentionWakes(
					ctx,
					parentRound.OwnerUserID,
					parentRound.ConversationID,
					[]publicMentionWake{wake},
					"error",
				)
				continue
			}
			if roomPublicHandoffIsTerminal(existing.Status) {
				// 已完成或已拒绝的重复 wake 不应重新打开 ledger。
				continue
			}
			if roomPublicHandoffIsInFlight(existing.Status) {
				// claimed/started 可能正由另一条恢复路径消费；保持幂等，
				// 不再重新 claim 或回写一个正在执行的边。
				accepted = append(accepted, wake)
				acceptedGuarded++
				continue
			}
		}
		if acceptedGuarded >= roomMaxHandoffFanout {
			s.terminalizePublicMentionWakes(
				ctx,
				parentRound.OwnerUserID,
				parentRound.ConversationID,
				[]publicMentionWake{wake},
				"error",
			)
			continue
		}
		projectedRootHandoffs := historicalRootHandoffs + acceptedGuarded + 1
		if projectedRootHandoffs > roomMaxRootHandoffs {
			s.terminalizePublicMentionWakes(
				ctx,
				parentRound.OwnerUserID,
				parentRound.ConversationID,
				[]publicMentionWake{wake},
				"error",
			)
			continue
		}
		if hasExisting {
			// annotation 在 start 前已经 Detect；恢复路径也会再次看到同一
			// 条边。ledger 身份必须一致，资源上限也不能因为恢复而绕过。
			if sourceAgentID == "" || targetAgentID == "" || sourceAgentID == targetAgentID {
				s.terminalizePublicMentionWakes(
					ctx,
					parentRound.OwnerUserID,
					parentRound.ConversationID,
					[]publicMentionWake{wake},
					"error",
				)
				continue
			}
			accepted = append(accepted, wake)
			acceptedGuarded++
			continue
		}
		if sourceAgentID == "" || targetAgentID == "" || sourceAgentID == targetAgentID {
			s.terminalizePublicMentionWakes(
				ctx,
				parentRound.OwnerUserID,
				parentRound.ConversationID,
				[]publicMentionWake{wake},
				"error",
			)
			continue
		}
		accepted = append(accepted, wake)
		acceptedGuarded++
	}
	return accepted, nil
}

func roomPublicHandoffIsTerminal(status string) bool {
	switch strings.TrimSpace(status) {
	case "finished", "error", "interrupted":
		return true
	default:
		return false
	}
}

func roomPublicHandoffIsInFlight(status string) bool {
	switch strings.TrimSpace(status) {
	case "claimed", "started":
		return true
	default:
		return false
	}
}

// terminalizePublicMentionWakes 收口因平台护栏被拒绝的 handoff，避免
// 已经从内存 pending 列表取出的边在 ledger 中永久停留为 source_finished。
func (s *Service) terminalizePublicMentionWakes(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	wakes []publicMentionWake,
	status string,
) {
	if s == nil || s.publicHandoffs == nil {
		return
	}
	for _, wake := range wakes {
		handoffID := strings.TrimSpace(wake.HandoffID)
		if handoffID == "" {
			continue
		}
		if err := s.publicHandoffs.MarkTerminal(ownerUserID, conversationID, handoffID, status); err != nil {
			s.loggerFor(ctx).Warn("收口受护栏拒绝的 Room handoff 失败",
				"conversation_id", conversationID,
				"handoff_id", handoffID,
				"status", status,
				"err", err,
			)
		}
	}
}

func (s *Service) logQueuedPublicMentionWakes(
	ctx context.Context,
	parentRound *activeRoomRound,
	sessionKey string,
) {
	s.loggerFor(ctx).Info("Room 公区 @ 目标均已进入队列",
		"s", sessionKey,
		"r", parentRound.Context.Room.ID,
		"c", parentRound.Context.Conversation.ID,
		"parent", parentRound.RoundID,
		"root", roomRootRoundID(parentRound),
	)
}

func buildPendingPublicMentionSlots(
	contextValue *protocol.ConversationContextAggregate,
	wakes []publicMentionWake,
	agentByID map[string]*protocol.Agent,
) []pendingPublicMentionSlot {
	pendingSlots := make([]pendingPublicMentionSlot, 0, len(wakes))
	targetSeen := make(map[string]struct{}, len(wakes))
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID == "" {
			continue
		}
		if _, exists := targetSeen[targetAgentID]; exists {
			continue
		}
		targetSeen[targetAgentID] = struct{}{}
		sessionRecord, ok := findRoomSessionForAgent(contextValue.Sessions, targetAgentID)
		if !ok || agentByID[targetAgentID] == nil {
			continue
		}
		pendingSlots = append(pendingSlots, pendingPublicMentionSlot{
			wake:          wake,
			targetAgentID: targetAgentID,
			sessionRecord: sessionRecord,
			agentValue:    agentByID[targetAgentID],
		})
	}
	return pendingSlots
}

func (s *Service) logMissingPublicMentionSlots(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	wakeCount int,
) {
	s.loggerFor(ctx).Warn("Room 公区 @ 没有可启动的目标 slot",
		"s", sessionKey,
		"r", contextValue.Room.ID,
		"c", contextValue.Conversation.ID,
		"wakes", wakeCount,
	)
}

func newPublicMentionRound(parentRound *activeRoomRound, sessionKey string, roundID string) *activeRoomRound {
	contextValue := parentRound.Context
	return &activeRoomRound{
		SessionKey:                        sessionKey,
		RoomID:                            contextValue.Room.ID,
		ConversationID:                    contextValue.Conversation.ID,
		CoordinatorAgentID:                parentRound.CoordinatorAgentID,
		RoomType:                          contextValue.Room.RoomType,
		Context:                           contextValue,
		RoundID:                           roundID,
		RootRoundID:                       cmp.Or(roomRootRoundID(parentRound), roundID),
		HopIndex:                          parentRound.HopIndex + 1,
		OwnerUserID:                       parentRound.OwnerUserID,
		AuthorityEpoch:                    contextValue.Room.AuthorityEpoch,
		TrustedConfigurationContext:       parentRound.pendingTrustedQueueDispatch,
		ExecutionOrigin:                   queueExecutionOrigin(parentRound.pendingTrustedQueueDispatch),
		trustedQueuedConfigurationContext: parentRound.pendingTrustedQueueDispatch,
		Slots:                             make(map[string]*activeRoomSlot),
		Done:                              make(chan struct{}),
	}
}

func queueExecutionOrigin(trusted bool) string {
	if trusted {
		return "queue"
	}
	return ""
}

func addPublicMentionSlots(
	activeRound *activeRoomRound,
	contextValue *protocol.ConversationContextAggregate,
	pendingSlots []pendingPublicMentionSlot,
) ([]string, []protocol.ChatAckPendingSlot) {
	targetAgentIDs := make([]string, 0, len(pendingSlots))
	pending := make([]protocol.ChatAckPendingSlot, 0, len(pendingSlots))
	for index, pendingSlot := range pendingSlots {
		targetAgentIDs = append(targetAgentIDs, pendingSlot.targetAgentID)
		msgID := newRealtimeID()
		agentRoundID := protocol.NewAgentRoundID()
		slotIndex := index
		slot := buildPublicMentionSlot(
			activeRound,
			contextValue,
			pendingSlot.sessionRecord,
			pendingSlot.agentValue,
			pendingSlot.wake,
			agentRoundID,
			msgID,
			slotIndex,
		)
		activeRound.Slots[msgID] = slot
		pending = append(pending, protocol.ChatAckPendingSlot{
			AgentID:      pendingSlot.targetAgentID,
			AgentRoundID: agentRoundID,
			MsgID:        msgID,
			RoundID:      roomRootRoundID(activeRound),
			HandoffID:    strings.TrimSpace(pendingSlot.wake.HandoffID),
			Status:       "pending",
			Timestamp:    slot.TimestampMS,
			Index:        slotIndex,
		})
	}
	return targetAgentIDs, pending
}

func (s *Service) launchPublicMentionRound(
	ctx context.Context,
	activeRound *activeRoomRound,
	wakes []publicMentionWake,
	pendingSlots []pendingPublicMentionSlot,
	targetAgentIDs []string,
	pending []protocol.ChatAckPendingSlot,
	publicHistory []protocol.Message,
	agentNameByID map[string]string,
	agentByID map[string]*protocol.Agent,
) bool {
	sessionKey := activeRound.SessionKey
	contextValue := activeRound.Context
	roundID := activeRound.RoundID
	roundCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	activeRound.Cancel = cancel
	s.registerRound(activeRound)
	if err := s.runtime.StartRound(roundCtx, sessionKey, roundID, cancel); err != nil {
		s.finishRound(activeRound)
		return false
	}
	s.loggerFor(ctx).Info(roomWakeStartLogMessage(wakes),
		"s", sessionKey,
		"r", contextValue.Room.ID,
		"c", contextValue.Conversation.ID,
		"hop", activeRound.HopIndex,
		"targets", targetAgentIDs,
		"pending", len(pending),
	)
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapRoundStatusEvent(sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, roundID, "running", ""))
	// 公区 @ 唤醒由后端发起，没有前端请求，client 关联字段留空。
	// pending slot 从 ACK 到 stream/result 都属于历史 root；内部 wake round
	// 只负责执行生命周期，不能先把卡片挂到尾部再搬回旧 root。
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapServerPendingSlotsEvent(
		sessionKey,
		contextValue.Room.ID,
		contextValue.Conversation.ID,
		roomRootRoundID(activeRound),
		pending,
	))
	for _, pendingSlot := range pendingSlots {
		if normalizeWakeQueueSource(pendingSlot.wake) != protocol.InputQueueSourceAgentRoomMessage {
			continue
		}
		s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, newRoomDirectedMessageWakeEvent(activeRound, pendingSlot.wake, "wake_started", map[string]any{
			"round_id": roundID,
		}))
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	go s.runRound(roundCtx, activeRound, publicHistory, agentNameByID, agentByID)
	return true
}

func buildPublicMentionSlot(
	roundValue *activeRoomRound,
	contextValue *protocol.ConversationContextAggregate,
	sessionRecord protocol.SessionRecord,
	agentValue *protocol.Agent,
	wake publicMentionWake,
	agentRoundID string,
	msgID string,
	index int,
) *activeRoomSlot {
	triggerType := strings.TrimSpace(wake.TriggerType)
	if triggerType == "" {
		triggerType = "public_mention"
	}
	trigger := roomTrigger{
		TriggerType:   triggerType,
		Content:       strings.TrimSpace(wake.Content),
		MessageID:     strings.TrimSpace(wake.MessageID),
		SourceAgentID: strings.TrimSpace(wake.SourceAgentID),
		TargetAgentID: strings.TrimSpace(wake.TargetAgentID),
		ReplyRoute:    wake.ReplyRoute,
	}
	slot := &activeRoomSlot{
		RoomSessionID:         sessionRecord.ID,
		OwnerUserID:           strings.TrimSpace(roundValue.OwnerUserID),
		AgentID:               strings.TrimSpace(wake.TargetAgentID),
		AgentRoundID:          agentRoundID,
		GoalUsageScopeRoundID: roomRootRoundID(roundValue),
		MsgID:                 msgID,
		RuntimeSessionKey:     protocol.BuildRoomAgentSessionKey(contextValue.Conversation.ID, wake.TargetAgentID, contextValue.Room.RoomType),
		WorkspacePath:         agentValue.WorkspacePath,
		Index:                 index,
		TimestampMS:           time.Now().UnixMilli(),
		Trigger:               trigger,
		WorkBinding:           cloneExecutionWorkBinding(wake.WorkBinding),
		ReviewBinding:         cloneExecutionReviewBinding(wake.ReviewBinding),
	}
	slot.setSDKSessionID(strings.TrimSpace(sessionRecord.SDKSessionID))
	slot.setStatus("pending")
	slot.setDeliveryMetadata(wake.ReplyRoute, wake.MessageID, wake.HandoffID)
	slot.doneChannel()
	return slot
}

func normalizeWakeQueueSource(wake publicMentionWake) protocol.InputQueueSource {
	if wake.QueueSource == protocol.InputQueueSourceAgentRoomMessage {
		return protocol.InputQueueSourceAgentRoomMessage
	}
	return protocol.InputQueueSourceAgentPublicMention
}

func roomWakeRoundID(wakes []publicMentionWake) string {
	prefix := "room_mention_"
	if len(wakes) > 0 && normalizeWakeQueueSource(wakes[0]) == protocol.InputQueueSourceAgentRoomMessage {
		prefix = "room_directed_message_"
	}
	return prefix + newRealtimeID()
}

func roomWakeStartLogMessage(wakes []publicMentionWake) string {
	if len(wakes) > 0 && normalizeWakeQueueSource(wakes[0]) == protocol.InputQueueSourceAgentRoomMessage {
		return "启动 Room directed message 唤醒 round"
	}
	return "启动 Room 公区 @ 唤醒 round"
}

func roomWakeQueuedLogMessage(wake publicMentionWake, participationPaused bool) string {
	if participationPaused {
		return "Room 成员暂停参与，Agent 唤醒保留在后端待发送队列"
	}
	if normalizeWakeQueueSource(wake) == protocol.InputQueueSourceAgentRoomMessage {
		return "Room directed message 目标正忙，写入后端待发送队列"
	}
	return "Room 公区 @ 目标正忙，写入后端待发送队列"
}

func findRoomSessionForAgent(sessions []protocol.SessionRecord, agentID string) (protocol.SessionRecord, bool) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return protocol.SessionRecord{}, false
	}
	for _, sessionRecord := range sessions {
		if strings.TrimSpace(sessionRecord.AgentID) == agentID {
			return sessionRecord, true
		}
	}
	return protocol.SessionRecord{}, false
}

// INPUT: Agent 公区 @ / Room 定向消息唤醒与目标 Agent 当前活跃 slot。
// OUTPUT: 公区 @ 对 busy 目标优先绑定当前轮 guide，其他唤醒排队；idle 目标继续立即开新轮。
// POS: Agent 唤醒进入 Room runtime 前的 busy/idle 分流与 durable queue 登记点。
func (s *Service) queueBusyPublicMentionWakes(
	ctx context.Context,
	parentRound *activeRoomRound,
	sessionKey string,
	wakes []publicMentionWake,
) ([]publicMentionWake, error) {
	if parentRound == nil || len(wakes) == 0 {
		return wakes, nil
	}
	targetAgentIDs := make([]string, 0, len(wakes))
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID != "" {
			targetAgentIDs = append(targetAgentIDs, targetAgentID)
		}
	}
	busySlots := s.findActiveDeliverySlotsByAgent(sessionKey, parentRound.ConversationID, targetAgentIDs)
	_, pausedAgentIDs := partitionRoomParticipationTargets(
		parentRound.Context.Members,
		targetAgentIDs,
	)
	pausedTargets := make(map[string]struct{}, len(pausedAgentIDs))
	for _, agentID := range pausedAgentIDs {
		pausedTargets[agentID] = struct{}{}
	}
	if len(busySlots) == 0 && len(pausedTargets) == 0 {
		return wakes, nil
	}

	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, parentRound.Context)
	if err != nil {
		return nil, err
	}
	ready := make([]publicMentionWake, 0, len(wakes))
	queued := false
	dispatchQueued := false
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID == "" {
			continue
		}
		busySlot := busySlots[targetAgentID]
		_, participationPaused := pausedTargets[targetAgentID]
		if busySlot == nil && !participationPaused {
			ready = append(ready, wake)
			continue
		}
		location, ok := locationsByAgentID[targetAgentID]
		if !ok {
			s.loggerFor(ctx).Warn("Room 公区 @ 目标正忙但缺少队列位置",
				"s", sessionKey,
				"r", parentRound.RoomID,
				"c", parentRound.ConversationID,
				"t", targetAgentID,
			)
			continue
		}
		queueSource := normalizeWakeQueueSource(wake)
		deliveryPolicy := protocol.ChatDeliveryPolicyQueue
		rootRoundID := roomRootRoundID(parentRound)
		if !participationPaused &&
			queueSource == protocol.InputQueueSourceAgentPublicMention &&
			s.supportsRoomGuidanceAck(busySlot) {
			// 公区 @ 已经是目标 Agent 可见的新上下文。目标忙碌时先绑定它
			// 当前 slot 的 PostToolUse hook；只有 hook 没有消费，slot 收尾才会
			// 把它降级为普通 queue 并续开下一轮，避免同 Agent 并发第二个 slot。
			deliveryPolicy = protocol.ChatDeliveryPolicyGuide
			rootRoundID = strings.TrimSpace(busySlot.AgentRoundID)
		} else if queueSource == protocol.InputQueueSourceAgentPublicMention {
			// 没有 applied ACK 时，不能把 queue item 从 durable 真相源中
			// 提前移走；直接排队，等当前 slot 终态后再启动下一轮。
			rootRoundID = roomRootRoundID(parentRound)
		}
		queuedItemID := workspacestore.NewInputQueueID()
		if wake.WorkBinding != nil && strings.TrimSpace(wake.WorkBinding.DispatchID) != "" {
			queuedItemID = "execution_dispatch_" + strings.TrimSpace(wake.WorkBinding.DispatchID)
		} else if wake.ReviewBinding != nil &&
			strings.TrimSpace(wake.ReviewBinding.ReviewDispatchID) != "" {
			queuedItemID = "execution_review_dispatch_" +
				strings.TrimSpace(wake.ReviewBinding.ReviewDispatchID)
		}
		queuedItem := protocol.InputQueueItem{
			ID:              queuedItemID,
			Scope:           protocol.InputQueueScopeRoom,
			SessionKey:      location.Location.SessionKey,
			RoomID:          parentRound.RoomID,
			ConversationID:  parentRound.ConversationID,
			AgentID:         targetAgentID,
			SourceAgentID:   strings.TrimSpace(wake.SourceAgentID),
			SourceMessageID: strings.TrimSpace(wake.MessageID),
			HandoffID:       strings.TrimSpace(wake.HandoffID),
			TargetAgentIDs:  []string{targetAgentID},
			Source:          queueSource,
			Content:         strings.TrimSpace(wake.Content),
			DeliveryPolicy:  deliveryPolicy,
			ReplyRoute:      wake.ReplyRoute,
			OwnerUserID:     parentRound.OwnerUserID,
			RootRoundID:     rootRoundID,
			HopIndex:        parentRound.HopIndex,
			WorkBinding:     cloneExecutionWorkBinding(wake.WorkBinding),
			ReviewBinding:   cloneExecutionReviewBinding(wake.ReviewBinding),
		}
		queueItems, inserted, err := s.inputQueue.EnqueueBounded(location.Location, queuedItem, 0)
		if err != nil {
			return nil, err
		}
		if !inserted {
			for _, existing := range queueItems {
				if strings.TrimSpace(existing.HandoffID) == strings.TrimSpace(wake.HandoffID) ||
					(existing.Source == queuedItem.Source &&
						strings.TrimSpace(existing.SourceMessageID) == strings.TrimSpace(queuedItem.SourceMessageID) &&
						strings.TrimSpace(existing.AgentID) == strings.TrimSpace(queuedItem.AgentID)) {
					queuedItem = existing
					queuedItemID = existing.ID
					break
				}
			}
		}
		if s.publicHandoffs != nil && strings.TrimSpace(wake.HandoffID) != "" {
			if err := s.publicHandoffs.MarkQueued(
				parentRound.OwnerUserID,
				parentRound.ConversationID,
				wake.HandoffID,
				queuedItemID,
			); err != nil {
				return nil, err
			}
		}
		if deliveryPolicy == protocol.ChatDeliveryPolicyGuide && !isActiveDeliverySlot(busySlot) {
			if _, err := s.inputQueue.UpdateDeliveryPolicy(
				location.Location,
				queuedItemID,
				protocol.ChatDeliveryPolicyQueue,
			); err != nil {
				return nil, err
			}
			deliveryPolicy = protocol.ChatDeliveryPolicyQueue
			dispatchQueued = true
		}
		queued = true
		activeAgentRoundID := ""
		if busySlot != nil {
			activeAgentRoundID = busySlot.AgentRoundID
		}
		s.loggerFor(ctx).Info(roomWakeQueuedLogMessage(wake, participationPaused),
			"s", sessionKey,
			"qs", location.Location.SessionKey,
			"r", parentRound.RoomID,
			"c", parentRound.ConversationID,
			"src", wake.SourceAgentID,
			"t", targetAgentID,
			"active_round_id", activeAgentRoundID,
			"participation_paused", participationPaused,
			"delivery_policy", deliveryPolicy,
		)
		if queueSource == protocol.InputQueueSourceAgentRoomMessage {
			s.broadcastSharedEventWithTimeout(ctx, sessionKey, parentRound.RoomID, newRoomDirectedMessageWakeEvent(parentRound, wake, "wake_queued", map[string]any{
				"queue_session_key": location.Location.SessionKey,
			}))
		}
	}
	if queued {
		if err := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, parentRound.Context); err != nil {
			return nil, err
		}
	}
	if dispatchQueued {
		s.startSessionBackgroundTask(
			sessionKey,
			parentRound.OwnerUserID,
			func(taskCtx context.Context) {
				s.dispatchNextInputQueueItem(
					taskCtx,
					sessionKey,
					parentRound.RoomID,
					parentRound.ConversationID,
				)
			},
		)
	}
	return ready, nil
}

// supportsRoomGuidanceAck 只允许已经协商 applied ACK 的 runtime 使用 guide。
// nil runtime、旧 runtime 和未知能力都必须走持久化 queue，避免“已返回 hook
// 但实际没有应用”的崩溃窗口造成消息丢失。
func (s *Service) supportsRoomGuidanceAck(slot *activeRoomSlot) bool {
	return s != nil && s.runtime != nil && slot != nil &&
		s.runtime.SupportsHookResponseAck(slot.RuntimeSessionKey)
}
