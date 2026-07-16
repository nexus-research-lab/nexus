// INPUT: 已完成 Agent 输出中的公区 @ 与目标 Agent 当前执行态。
// OUTPUT: 同 Agent 串行的 public mention guide/新轮唤醒。
// POS: Room Agent 间公开协作的唤醒编排入口。
package room

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	// hop 是最后一道保险；更早的 root admission 会先拦截环和异常 fanout。
	roomMaxWakeHops      = 16
	roomMaxHandoffFanout = 8
	roomMaxRootHandoffs  = 32
)

type pendingPublicMentionSlot struct {
	wake          publicMentionWake
	targetAgentID string
	sessionRecord protocol.SessionRecord
	agentValue    *protocol.Agent
}

func (s *RealtimeService) collectPublicMentionWakes(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	if roundValue == nil || roundValue.Context == nil || slot == nil {
		return nil
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
		if s.publicHandoffs != nil {
			if err := s.publicHandoffs.MarkSourceFinished(roundValue.ConversationID, wake.HandoffID); err != nil {
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
			// 没有 handoff_id 的 mention 只是展示 span；真实交接必须
			// 由服务端显式选中并写入 ledger，不能因解析到 @ 就再次唤醒。
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

func (s *RealtimeService) enqueuePublicMentionWake(roundValue *activeRoomRound, wake publicMentionWake) {
	if roundValue == nil || strings.TrimSpace(wake.TargetAgentID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range roundValue.PublicMentions {
		if existing.TargetAgentID == wake.TargetAgentID &&
			strings.TrimSpace(existing.MessageID) == strings.TrimSpace(wake.MessageID) &&
			strings.TrimSpace(existing.Content) == strings.TrimSpace(wake.Content) {
			return
		}
	}
	roundValue.PublicMentions = append(roundValue.PublicMentions, wake)
}

func (s *RealtimeService) takePublicMentionWakes(roundValue *activeRoomRound) []publicMentionWake {
	if roundValue == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	wakes := slices.Clone(roundValue.PublicMentions)
	roundValue.PublicMentions = nil
	return wakes
}

func (s *RealtimeService) startQueuedPublicMentionWakes(ctx context.Context, roundValue *activeRoomRound) bool {
	wakes := s.takePublicMentionWakes(roundValue)
	if len(wakes) == 0 {
		return false
	}
	if err := s.startPublicMentionRound(ctx, roundValue, wakes); err != nil {
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

func (s *RealtimeService) startPublicMentionRound(
	ctx context.Context,
	parentRound *activeRoomRound,
	wakes []publicMentionWake,
) error {
	s.publicMentionDispatchMu.Lock()
	defer s.publicMentionDispatchMu.Unlock()

	if parentRound == nil || parentRound.Context == nil || len(wakes) == 0 {
		return nil
	}
	wakes, err := s.admitPublicMentionWakes(ctx, parentRound, wakes)
	if err != nil {
		return err
	}
	if len(wakes) == 0 {
		return nil
	}
	// root admission 已经先处理 visited/cycle/fanout；hop 只作为最后一道
	// 跨重启或异常数据兜底，避免正常链路被单一计数提前截断。
	if parentRound.HopIndex >= roomMaxWakeHops {
		s.terminalizePublicMentionWakes(ctx, parentRound.ConversationID, wakes, "error")
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
		_, claimed, claimErr := s.publicHandoffs.Claim(parentRound.ConversationID, wake.HandoffID)
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
	publicHistory, err := s.roomHistory.ReadMessages(contextValue.Conversation.ID, nil)
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
		if err := s.publicHandoffs.MarkTerminal(parentRound.ConversationID, wake.HandoffID, "error"); err != nil {
			s.loggerFor(ctx).Warn("目标 Agent 不可用，收口 Room handoff 失败", "handoff_id", wake.HandoffID, "err", err)
		}
	}
	if len(pendingSlots) == 0 {
		s.logMissingPublicMentionSlots(ctx, sessionKey, contextValue, len(wakes))
		return nil
	}
	roundID := roomWakeRoundID(wakes)
	activeRound := newPublicMentionRound(parentRound, sessionKey, roundID)
	targetAgentIDs, pending := addPublicMentionSlots(activeRound, contextValue, pendingSlots)
	s.launchPublicMentionRound(
		ctx,
		activeRound,
		wakes,
		pendingSlots,
		targetAgentIDs,
		pending,
		publicHistory,
		agentNameByID,
		agentByID,
	)
	if s.publicHandoffs != nil {
		for _, wake := range wakes {
			if strings.TrimSpace(wake.HandoffID) == "" {
				continue
			}
			if err := s.publicHandoffs.MarkStarted(activeRound.ConversationID, wake.HandoffID, roundID); err != nil {
				s.loggerFor(ctx).Warn("记录 Room handoff 启动状态失败", "handoff_id", wake.HandoffID, "err", err)
			}
		}
	}
	return nil
}

// admitPublicMentionWakes 在真正 claim 前做 root 级资源与拓扑护栏。
// 只有显式 handoff 的公区 @ 进入这里；directed message 仍由自己的路由规则管理。
func (s *RealtimeService) admitPublicMentionWakes(
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
		edges, err = s.publicHandoffs.ListRoot(parentRound.ConversationID, rootRoundID)
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
	workingEdges := make([]workspacestore.RoomPublicHandoff, 0, len(edges)+len(wakes))
	for _, edge := range edges {
		// 同一批次的 sibling 边不能互相制造环；只把历史 root 边
		// 带入拓扑判断，再按当前接受顺序逐条加入。
		if _, current := currentWakeIDs[strings.TrimSpace(edge.HandoffID)]; current {
			continue
		}
		historicalRootHandoffs++
		workingEdges = append(workingEdges, edge)
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
				s.terminalizePublicMentionWakes(ctx, parentRound.ConversationID, []publicMentionWake{wake}, "error")
				continue
			}
			if roomPublicHandoffIsTerminal(existing.Status) {
				// 已完成或已拒绝的重复 wake 不应重新打开 ledger。
				continue
			}
			if roomPublicHandoffIsInFlight(existing.Status) {
				// claimed/started 可能正由另一条恢复路径消费；保持幂等，
				// 不再用当前拓扑快照回写一个正在执行的边。
				accepted = append(accepted, wake)
				workingEdges = append(workingEdges, existing)
				acceptedGuarded++
				continue
			}
		}
		if acceptedGuarded >= roomMaxHandoffFanout {
			s.terminalizePublicMentionWakes(ctx, parentRound.ConversationID, []publicMentionWake{wake}, "error")
			continue
		}
		projectedRootHandoffs := historicalRootHandoffs + acceptedGuarded + 1
		if projectedRootHandoffs > roomMaxRootHandoffs {
			s.terminalizePublicMentionWakes(ctx, parentRound.ConversationID, []publicMentionWake{wake}, "error")
			continue
		}
		if hasExisting {
			// annotation 在 start 前已经 Detect；恢复路径也会再次看到同一
			// 条边。它们仍需经过同一拓扑校验，不能因为已写 ledger 就绕过
			// cycle/fanout 护栏。
			if roomPublicHandoffCreatesCycle(workingEdges, sourceAgentID, targetAgentID) {
				s.terminalizePublicMentionWakes(ctx, parentRound.ConversationID, []publicMentionWake{wake}, "error")
				continue
			}
			accepted = append(accepted, wake)
			workingEdges = append(workingEdges, existing)
			acceptedGuarded++
			continue
		}
		if sourceAgentID == "" || targetAgentID == "" || sourceAgentID == targetAgentID ||
			roomPublicHandoffCreatesCycle(workingEdges, sourceAgentID, targetAgentID) {
			s.terminalizePublicMentionWakes(ctx, parentRound.ConversationID, []publicMentionWake{wake}, "error")
			continue
		}
		workingEdges = append(workingEdges, workspacestore.RoomPublicHandoff{
			HandoffID:      handoffID,
			SourceAgentID:  sourceAgentID,
			TargetAgentID:  targetAgentID,
			RootRoundID:    rootRoundID,
			ConversationID: parentRound.ConversationID,
			Status:         "source_finished",
		})
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

func roomPublicHandoffCreatesCycle(
	edges []workspacestore.RoomPublicHandoff,
	sourceAgentID string,
	targetAgentID string,
) bool {
	sourceAgentID = strings.TrimSpace(sourceAgentID)
	targetAgentID = strings.TrimSpace(targetAgentID)
	if sourceAgentID == "" || targetAgentID == "" || sourceAgentID == targetAgentID {
		return true
	}
	graph := make(map[string][]string)
	for _, edge := range edges {
		source := strings.TrimSpace(edge.SourceAgentID)
		target := strings.TrimSpace(edge.TargetAgentID)
		if source == "" || target == "" {
			continue
		}
		graph[source] = append(graph[source], target)
	}
	stack := []string{targetAgentID}
	visited := make(map[string]struct{})
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current == sourceAgentID {
			return true
		}
		if _, ok := visited[current]; ok {
			continue
		}
		visited[current] = struct{}{}
		stack = append(stack, graph[current]...)
	}
	return false
}

// terminalizePublicMentionWakes 收口因平台护栏被拒绝的 handoff，避免
// 已经从内存 pending 列表取出的边在 ledger 中永久停留为 source_finished。
func (s *RealtimeService) terminalizePublicMentionWakes(
	ctx context.Context,
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
		if err := s.publicHandoffs.MarkTerminal(conversationID, handoffID, status); err != nil {
			s.loggerFor(ctx).Warn("收口受护栏拒绝的 Room handoff 失败",
				"conversation_id", conversationID,
				"handoff_id", handoffID,
				"status", status,
				"err", err,
			)
		}
	}
}

func (s *RealtimeService) logQueuedPublicMentionWakes(
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

func (s *RealtimeService) logMissingPublicMentionSlots(
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
		SessionKey:     sessionKey,
		RoomID:         contextValue.Room.ID,
		ConversationID: contextValue.Conversation.ID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        roundID,
		RootRoundID:    cmp.Or(roomRootRoundID(parentRound), roundID),
		HopIndex:       parentRound.HopIndex + 1,
		OwnerUserID:    parentRound.OwnerUserID,
		Slots:          make(map[string]*activeRoomSlot),
		Done:           make(chan struct{}),
	}
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
		activeRound.Slots[msgID] = buildPublicMentionSlot(
			contextValue,
			pendingSlot.sessionRecord,
			pendingSlot.agentValue,
			pendingSlot.wake,
			agentRoundID,
			msgID,
			slotIndex,
		)
		pending = append(pending, protocol.ChatAckPendingSlot{
			AgentID:      pendingSlot.targetAgentID,
			AgentRoundID: agentRoundID,
			MsgID:        msgID,
			Status:       "pending",
			Timestamp:    time.Now().UnixMilli(),
			Index:        slotIndex,
		})
	}
	return targetAgentIDs, pending
}

func (s *RealtimeService) launchPublicMentionRound(
	ctx context.Context,
	activeRound *activeRoomRound,
	wakes []publicMentionWake,
	pendingSlots []pendingPublicMentionSlot,
	targetAgentIDs []string,
	pending []protocol.ChatAckPendingSlot,
	publicHistory []protocol.Message,
	agentNameByID map[string]string,
	agentByID map[string]*protocol.Agent,
) {
	sessionKey := activeRound.SessionKey
	contextValue := activeRound.Context
	roundID := activeRound.RoundID
	roundCtx, cancel := context.WithCancel(context.Background())
	activeRound.Cancel = cancel
	s.registerRound(activeRound)
	s.runtime.StartRound(sessionKey, roundID, cancel)
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
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapChatAckEvent(sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, "", "", roundID, "", false, pending))
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
}

func buildPublicMentionSlot(
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
	return &activeRoomSlot{
		RoomSessionID:      sessionRecord.ID,
		SDKSessionID:       strings.TrimSpace(sessionRecord.SDKSessionID),
		AgentID:            strings.TrimSpace(wake.TargetAgentID),
		AgentRoundID:       agentRoundID,
		MsgID:              msgID,
		RuntimeSessionKey:  protocol.BuildRoomAgentSessionKey(contextValue.Conversation.ID, wake.TargetAgentID, contextValue.Room.RoomType),
		WorkspacePath:      agentValue.WorkspacePath,
		Status:             "pending",
		Index:              index,
		TimestampMS:        time.Now().UnixMilli(),
		Trigger:            trigger,
		ReplyRoute:         wake.ReplyRoute,
		ReplySourceMessage: strings.TrimSpace(wake.MessageID),
		ReplySourceAgent:   strings.TrimSpace(wake.SourceAgentID),
		HandoffID:          strings.TrimSpace(wake.HandoffID),
		Done:               make(chan struct{}),
	}
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

func roomWakeQueuedLogMessage(wake publicMentionWake) string {
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
