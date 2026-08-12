// INPUT: Room 最终 assistant 公区消息、Goal-attributed directed wake、成员目录与 source slot 身份。
// OUTPUT: 带 agent_mentions 标注的消息，以及保留 owner/root scope、路由来源且可幂等恢复的 handoff ledger 记录。
// POS: @ 解析、directed wake 与 handoff identity 的单一持久恢复边界。
package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"strings"
)

type roomMentionTextBlock struct {
	index int
	text  string
}

type roomResolvedMention struct {
	block roomMentionTextBlock
	match roomdomain.MentionMatch
}

// buildRoomMentionAnnotations 统一 Room 公区 mention 的成员过滤和 handoff 标注。
// 普通 assistant 输出与主动发布消息必须共享这条规则，避免两条链路产生不同的协作语义。
func buildRoomMentionAnnotations(
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	messageID string,
	blocks []roomMentionTextBlock,
) []protocol.AgentMention {
	resolved := resolveRoomMentionMatches(contextValue, sourceAgentID, blocks)
	if len(resolved) == 0 {
		return nil
	}

	messageID = strings.TrimSpace(messageID)
	result := make([]protocol.AgentMention, 0, len(resolved))
	for _, item := range resolved {
		targetAgentID := strings.TrimSpace(item.match.AgentID)
		result = append(result, protocol.AgentMention{
			AgentID:           targetAgentID,
			Label:             strings.TrimSpace(item.match.Label),
			ContentBlockIndex: item.block.index,
			StartRune:         item.match.StartRune,
			EndRune:           item.match.EndRune,
			HandoffID: roomPublicHandoffID(
				contextValue.Conversation.ID,
				messageID,
				targetAgentID,
			),
		})
	}
	return result
}

func resolveRoomMentionMatches(
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	blocks []roomMentionTextBlock,
) []roomResolvedMention {
	if contextValue == nil || len(blocks) == 0 {
		return nil
	}
	aliases := roomdomain.BuildMentionAliases(contextValue)
	if len(aliases) == 0 {
		return nil
	}
	sourceAgentID = strings.TrimSpace(sourceAgentID)
	result := make([]roomResolvedMention, 0)
	for _, block := range blocks {
		for _, match := range roomdomain.ResolveMentionMatches(block.text, aliases) {
			targetAgentID := strings.TrimSpace(match.AgentID)
			if targetAgentID == "" || targetAgentID == sourceAgentID ||
				!roomdomain.IsMemberAgent(contextValue.Members, targetAgentID) {
				continue
			}
			result = append(result, roomResolvedMention{block: block, match: match})
		}
	}
	return result
}

func (s *Service) decorateRoomMessage(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) {
	setRoomDisplayOrder(slot, message)
	if !roomShouldAnnotatePublicMessage(roundValue, slot, message) {
		return
	}
	if err := s.annotatePublicAssistantMessage(roundValue, slot, message); err != nil {
		s.loggerFor(context.Background()).Warn("Room 公区 @ 标注写入 handoff ledger 失败",
			"conversation_id", roundValue.ConversationID,
			"message_id", strings.TrimSpace(anyString(message["message_id"])),
			"err", err,
		)
	}
}

// setRoomDisplayOrder 为同一 root round 的 Agent 回复提供跨重启稳定的启动顺序。
// slot 创建时间负责跨 wake 追加，index 只处理同一批并发 Agent 的 tie-break；
// completion message 自己的时间戳只用于展示，不能在终态时重排卡片。
func setRoomDisplayOrder(slot *activeRoomSlot, message protocol.Message) {
	if slot == nil || message == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	if protocol.Int64FromAny(message["display_order"]) > 0 {
		return
	}
	timestamp := slot.TimestampMS
	if timestamp <= 0 {
		timestamp = protocol.Int64FromAny(message["timestamp"])
	}
	if timestamp <= 0 {
		return
	}
	message["display_order"] = timestamp*1000 + int64(max(slot.Index, 0))
}

func roomShouldAnnotatePublicMessage(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) bool {
	return roundValue != nil && roundValue.Context != nil && slot != nil &&
		roomSlotPublishesPublicOutput(slot) && roomdomain.IsFinalPublicAssistantMessage(message)
}

func (s *Service) annotatePublicAssistantMessage(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	// 旧版 fanout 控制标记只做输入兼容，不再决定路由；每个有效 @ 都是
	// 真实 handoff。清理后重新计算 span，确保偏移对应用户实际看到的文本。
	cleaned := roomdomain.StripFanoutMarker(message)
	// agent_mentions 是服务端派生字段，不能信任 runtime 传入的旧 handoff_id。
	delete(cleaned, "agent_mentions")
	for key := range message {
		delete(message, key)
	}
	for key, value := range cleaned {
		message[key] = value
	}
	blocks := roomMentionTextBlocks(message["content"])
	if len(blocks) == 0 {
		if content := roomdomain.ExtractAssistantResultText(message); content != "" {
			blocks = []roomMentionTextBlock{{index: 0, text: content}}
		}
	}
	if len(blocks) == 0 {
		return nil
	}
	messageID := strings.TrimSpace(anyString(message["message_id"]))
	if messageID == "" {
		return nil
	}
	mentions := buildRoomMentionAnnotations(
		roundValue.Context,
		slot.AgentID,
		messageID,
		blocks,
	)
	if len(mentions) == 0 {
		return nil
	}
	if err := s.detectRoomMentionHandoffs(roundValue, slot, message, mentions); err != nil {
		return err
	}
	message["agent_mentions"] = mentions
	return nil
}

func (s *Service) detectRoomMentionHandoffs(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
	mentions []protocol.AgentMention,
) error {
	if s.publicHandoffs == nil {
		return nil
	}
	messageID := strings.TrimSpace(anyString(message["message_id"]))
	content := strings.TrimSpace(roomdomain.ExtractAssistantResultText(message))
	detected := make(map[string]struct{}, len(mentions))
	for _, mention := range mentions {
		handoffID := strings.TrimSpace(mention.HandoffID)
		if handoffID == "" {
			continue
		}
		if _, ok := detected[handoffID]; ok {
			continue
		}
		detected[handoffID] = struct{}{}
		_, _, err := s.publicHandoffs.Detect(roundValue.OwnerUserID, workspacestore.RoomPublicHandoff{
			HandoffID:          handoffID,
			ConversationID:     roundValue.ConversationID,
			RoomID:             roundValue.RoomID,
			RootRoundID:        roomRootRoundID(roundValue),
			SourceAgentRoundID: strings.TrimSpace(slot.AgentRoundID),
			SourceMessageID:    messageID,
			SourceAgentID:      strings.TrimSpace(slot.AgentID),
			TargetAgentID:      strings.TrimSpace(mention.AgentID),
			Content:            content,
			QueueSource:        protocol.InputQueueSourceAgentPublicMention,
			GoalCollaborationBinding: goalCollaborationBindingForSlot(
				roundValue,
				slot,
			),
			HopIndex: roundValue.HopIndex,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func roomMentionTextBlocks(content any) []roomMentionTextBlock {
	switch typed := content.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []roomMentionTextBlock{{index: 0, text: typed}}
	case []map[string]any:
		result := make([]roomMentionTextBlock, 0, len(typed))
		for index, block := range typed {
			if strings.TrimSpace(anyString(block["type"])) != "text" {
				continue
			}
			if text := anyString(block["text"]); strings.TrimSpace(text) != "" {
				result = append(result, roomMentionTextBlock{index: index, text: text})
			}
		}
		return result
	case []any:
		result := make([]roomMentionTextBlock, 0, len(typed))
		for index, value := range typed {
			block, ok := value.(map[string]any)
			if !ok || strings.TrimSpace(anyString(block["type"])) != "text" {
				continue
			}
			if text := anyString(block["text"]); strings.TrimSpace(text) != "" {
				result = append(result, roomMentionTextBlock{index: index, text: text})
			}
		}
		return result
	default:
		return nil
	}
}

// annotateRoomUserMessage 写入用户消息中的 mention span；用户消息不创建 handoff，
// 它只把服务端已经解析出的目标身份传给共享渲染器。
func annotateRoomUserMessage(
	contextValue *protocol.ConversationContextAggregate,
	message protocol.Message,
) {
	if contextValue == nil || message == nil || protocol.MessageRole(message) != "user" {
		return
	}
	content, ok := message["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return
	}
	aliases := roomdomain.BuildMentionAliases(contextValue)
	if len(aliases) == 0 {
		return
	}
	mentions := make([]protocol.AgentMention, 0)
	for _, match := range roomdomain.ResolveMentionMatches(content, aliases) {
		targetAgentID := strings.TrimSpace(match.AgentID)
		if targetAgentID == "" || !roomdomain.IsMemberAgent(contextValue.Members, targetAgentID) {
			continue
		}
		mentions = append(mentions, protocol.AgentMention{
			AgentID:           targetAgentID,
			Label:             strings.TrimSpace(match.Label),
			ContentBlockIndex: 0,
			StartRune:         match.StartRune,
			EndRune:           match.EndRune,
		})
	}
	if len(mentions) > 0 {
		message["agent_mentions"] = mentions
	}
}

func roomPublicHandoffID(conversationID string, sourceMessageID string, targetAgentID string) string {
	seed := fmt.Sprintf("%s\x00%s\x00%s", strings.TrimSpace(conversationID), strings.TrimSpace(sourceMessageID), strings.TrimSpace(targetAgentID))
	digest := sha256.Sum256([]byte(seed))
	return "rh_" + hex.EncodeToString(digest[:12])
}

func roomDirectedGoalHandoffID(
	conversationID string,
	sourceMessageID string,
	targetAgentID string,
) string {
	seed := fmt.Sprintf(
		"directed_goal\x00%s\x00%s\x00%s",
		strings.TrimSpace(conversationID),
		strings.TrimSpace(sourceMessageID),
		strings.TrimSpace(targetAgentID),
	)
	digest := sha256.Sum256([]byte(seed))
	return "rh_" + hex.EncodeToString(digest[:12])
}

func (s *Service) markPublicHandoffTerminal(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	status string,
) {
	if s.publicHandoffs == nil || roundValue == nil || slot == nil {
		return
	}
	handoffID := strings.TrimSpace(slot.handoffID())
	if handoffID == "" {
		return
	}
	lastAssistant := slot.lastGoalAssistantMessage()
	hasSubstantiveOutput := !roomdomain.IsNoReplyAssistantMessage(lastAssistant) &&
		strings.TrimSpace(messageutil.ExtractAssistantDisplayText(lastAssistant)) != ""
	if err := s.publicHandoffs.MarkTerminalWithGoalOutcome(
		roundValue.OwnerUserID,
		roundValue.ConversationID,
		handoffID,
		status,
		slot.AgentRoundID,
		hasSubstantiveOutput,
		roomSlotPublishesPublicOutput(slot),
	); err != nil {
		s.loggerFor(ctx).Warn("记录 Room handoff 终态失败", "handoff_id", handoffID, "status", status, "err", err)
	}
}

func (s *Service) cancelSourcePublicHandoffs(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	status string,
) {
	if s.publicHandoffs == nil || roundValue == nil || slot == nil || strings.TrimSpace(slot.AgentRoundID) == "" {
		return
	}
	if err := s.publicHandoffs.CancelForSource(
		roundValue.OwnerUserID,
		roundValue.ConversationID,
		slot.AgentRoundID,
		status,
	); err != nil {
		s.loggerFor(ctx).Warn("取消 Room source handoff 失败", "agent_round_id", slot.AgentRoundID, "err", err)
	}
}

func (s *Service) markRoomQueueHandoffTerminal(
	conversationID string,
	item protocol.InputQueueItem,
) error {
	return s.markRoomQueueHandoffTerminalStatus(conversationID, item, "finished")
}

func (s *Service) markRoomQueueHandoffTerminalStatus(
	conversationID string,
	item protocol.InputQueueItem,
	status string,
) error {
	if s.publicHandoffs == nil || strings.TrimSpace(item.HandoffID) == "" {
		return nil
	}
	return s.publicHandoffs.MarkTerminal(item.OwnerUserID, conversationID, item.HandoffID, status)
}

// cancelRootPublicHandoffs 把 root 取消传播到 ledger 与尚未派发的 queue item。
// 已经进入 target runtime 的 slot 由 interruptActiveRound 继续负责中断。
func (s *Service) cancelRootPublicHandoffs(
	ctx context.Context,
	roundValue *activeRoomRound,
	status string,
) {
	if s == nil || s.publicHandoffs == nil || roundValue == nil {
		return
	}
	rootRoundID := roomRootRoundID(roundValue)
	edges, err := s.publicHandoffs.ListRoot(
		roundValue.OwnerUserID,
		roundValue.ConversationID,
		rootRoundID,
	)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room root handoff 失败", "root", rootRoundID, "err", err)
		return
	}
	if err = s.publicHandoffs.CancelForRoot(
		roundValue.OwnerUserID,
		roundValue.ConversationID,
		rootRoundID,
		status,
	); err != nil {
		s.loggerFor(ctx).Warn("取消 Room root handoff 失败", "root", rootRoundID, "err", err)
		return
	}
	if s.inputQueue == nil || roundValue.Context == nil || len(edges) == 0 {
		return
	}
	cancelledIDs := make(map[string]struct{}, len(edges))
	for _, edge := range edges {
		if handoffID := strings.TrimSpace(edge.HandoffID); handoffID != "" {
			cancelledIDs[handoffID] = struct{}{}
		}
	}
	entries, err := s.roomInputQueueEntries(ctx, roundValue.Context)
	if err != nil {
		s.loggerFor(ctx).Warn("读取待取消的 Room handoff queue 失败", "root", rootRoundID, "err", err)
		return
	}
	changed := false
	for _, entry := range entries {
		goalDirected := entry.Item.Source == protocol.InputQueueSourceAgentRoomMessage &&
			protocol.NormalizeGoalCollaborationBinding(entry.Item.GoalCollaborationBinding) != nil
		if entry.Item.Source != protocol.InputQueueSourceAgentPublicMention && !goalDirected {
			continue
		}
		if _, ok := cancelledIDs[strings.TrimSpace(entry.Item.HandoffID)]; !ok {
			continue
		}
		if _, err = s.inputQueue.Delete(entry.Location, entry.Item.ID); err != nil {
			s.loggerFor(ctx).Warn("删除已取消的 Room handoff queue 失败", "item_id", entry.Item.ID, "err", err)
			continue
		}
		changed = true
	}
	if changed {
		if err = s.broadcastRoomInputQueueSnapshot(ctx, roundValue.SessionKey, roundValue.Context); err != nil {
			s.loggerFor(ctx).Warn("广播取消后的 Room queue 快照失败", "root", rootRoundID, "err", err)
		}
	}
}

// INPUT: 进程启动时 directed-message 与 handoff ledger 中尚未完成的恢复事实。
// OUTPUT: 重新进入统一 busy/idle 派发路径的 target wake。
// POS: Room 公区、structured Execution 与 Goal-directed 协作的 durable recovery 边界。
// StartPublicHandoffReconciler 修补两阶段写入并恢复已确认 source 但尚未收口的 handoff。
func (s *Service) StartPublicHandoffReconciler(ctx context.Context) (func(), error) {
	if s == nil || s.publicHandoffs == nil || s.rooms == nil {
		return nil, nil
	}
	if err := s.repairGoalDirectedMessageHandoffs(ctx); err != nil {
		return nil, err
	}
	if err := s.repairLegacyRoomGoalHandoffAttribution(ctx); err != nil {
		return nil, err
	}
	pending, err := s.publicHandoffs.PendingAll()
	if err != nil {
		return nil, err
	}
	for _, handoff := range pending {
		if err := s.reconcilePublicHandoff(ctx, handoff); err != nil {
			s.loggerFor(ctx).Warn("恢复 Room handoff 失败",
				"conversation_id", handoff.ConversationID,
				"handoff_id", handoff.HandoffID,
				"err", err,
			)
		}
	}
	return nil, nil
}

// repairLegacyRoomGoalHandoffAttribution repairs the bounded upgrade gap from
// builds that persisted Room handoff roots before exact Goal attribution was
// added. It never inspects model prose. A root is eligible only when the
// current active Goal's latest non-usage audit event is the source Agent
// round's continuation_suppressed event, the root is fully terminal, and a
// non-lead terminal participant has a public substantive result in canonical
// Room history.
func (s *Service) repairLegacyRoomGoalHandoffAttribution(ctx context.Context) error {
	if s == nil || s.publicHandoffs == nil || s.roomHistory == nil || s.goals == nil {
		return nil
	}
	events, ok := s.goals.(goalEventProvider)
	if !ok {
		return nil
	}
	roots, err := s.publicHandoffs.LegacyUnattributedTerminalRootsAll()
	if err != nil {
		return err
	}
	for _, root := range roots {
		if len(root) == 0 {
			continue
		}
		head := root[0]
		sessionKey := protocol.BuildRoomSharedSessionKey(head.ConversationID)
		goal := s.currentRoomGoalForSession(ctx, sessionKey)
		if goal == nil || goal.EmptyProgressCount <= 0 || goalsvc.RoomCollaborationObserved(*goal) {
			continue
		}
		goalEvents, eventErr := events.Events(ctx, goal.ID, 200)
		if eventErr != nil {
			return eventErr
		}
		latest := latestRoomGoalStateEvent(goalEvents)
		if latest == nil || latest.EventType != "continuation_suppressed" ||
			!legacyHandoffRootMatchesSuppressedEvent(root, *latest) {
			continue
		}
		messages, historyErr := s.roomHistory.ReadMessages(
			head.OwnerUserID,
			head.ConversationID,
			nil,
		)
		if historyErr != nil {
			return historyErr
		}
		evidenceAgentID, evidenceAgentRoundID := legacyHandoffRootPublicEvidence(
			root,
			messages,
			goalsvc.RoomLeadAgentID(*goal),
		)
		if evidenceAgentID == "" || evidenceAgentRoundID == "" {
			continue
		}
		if err := s.publicHandoffs.BindLegacyTerminalRootToGoal(
			head.OwnerUserID,
			head.ConversationID,
			head.RootRoundID,
			protocol.GoalCollaborationBinding{
				GoalID:            goal.ID,
				ObjectiveRevision: goal.ObjectiveRevision(),
			},
			evidenceAgentID,
			evidenceAgentRoundID,
		); err != nil {
			return err
		}
	}
	return nil
}

func latestRoomGoalStateEvent(events []protocol.GoalEvent) *protocol.GoalEvent {
	var latest *protocol.GoalEvent
	for index := range events {
		candidate := &events[index]
		if candidate.EventType == "usage_recorded" {
			continue
		}
		if latest == nil || candidate.CreatedAt.After(latest.CreatedAt) ||
			(candidate.CreatedAt.Equal(latest.CreatedAt) && candidate.ID > latest.ID) {
			latest = candidate
		}
	}
	return latest
}

func legacyHandoffRootMatchesSuppressedEvent(
	root []workspacestore.RoomPublicHandoff,
	event protocol.GoalEvent,
) bool {
	if len(root) == 0 || event.EventType != "continuation_suppressed" ||
		strings.TrimSpace(event.RoundID) == "" {
		return false
	}
	for _, handoff := range root {
		if strings.TrimSpace(handoff.SourceAgentRoundID) == strings.TrimSpace(event.RoundID) {
			return true
		}
	}
	return false
}

func legacyHandoffRootPublicEvidence(
	root []workspacestore.RoomPublicHandoff,
	messages []protocol.Message,
	leadAgentID string,
) (string, string) {
	if len(root) == 0 {
		return "", ""
	}
	leadAgentID = strings.TrimSpace(leadAgentID)
	rootRoundID := strings.TrimSpace(root[0].RootRoundID)
	targets := make(map[string]struct{})
	for _, handoff := range root {
		targetID := strings.TrimSpace(handoff.TargetAgentID)
		if targetID != "" && targetID != leadAgentID && handoff.Status == "finished" {
			targets[targetID] = struct{}{}
		}
	}
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if strings.TrimSpace(anyString(message["round_id"])) != rootRoundID {
			continue
		}
		agentID := strings.TrimSpace(anyString(message["agent_id"]))
		if _, ok := targets[agentID]; !ok {
			continue
		}
		if !roomdomain.IsFinalPublicAssistantMessage(message) ||
			roomdomain.IsNoReplyAssistantMessage(message) ||
			strings.TrimSpace(roomdomain.ExtractAssistantResultText(message)) == "" {
			continue
		}
		agentRoundID := strings.TrimSpace(anyString(message["agent_round_id"]))
		if agentRoundID != "" {
			return agentID, agentRoundID
		}
	}
	return "", ""
}

// repairGoalDirectedMessageHandoffs closes the append-only two-store window in
// which the private message reached disk but the matching handoff Detect did
// not. Only the exact active Goal revision can be repaired; stale or terminal
// Goal facts stay inert and never recreate collaborator work.
func (s *Service) repairGoalDirectedMessageHandoffs(ctx context.Context) error {
	if s == nil || s.directedMessages == nil || s.publicHandoffs == nil || s.rooms == nil {
		return nil
	}
	records, err := s.directedMessages.GoalCollaborationMessagesAll()
	if err != nil {
		return err
	}
	for _, record := range records {
		message := record.Message
		binding := protocol.NormalizeGoalCollaborationBinding(
			message.GoalCollaborationBinding,
		)
		if binding == nil {
			continue
		}
		contextValue, contextErr := s.rooms.GetConversationContextForSystem(
			ctx,
			message.ConversationID,
		)
		if contextErr != nil {
			if errors.Is(contextErr, roomsvc.ErrRoomNotFound) ||
				errors.Is(contextErr, roomsvc.ErrConversationNotFound) {
				continue
			}
			return contextErr
		}
		if contextValue == nil {
			continue
		}
		ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
		if ownerUserID == "" ||
			(record.OwnerUserID != "" && ownerUserID != strings.TrimSpace(record.OwnerUserID)) ||
			appfs.UserPathSegment(ownerUserID) != strings.TrimSpace(record.OwnerPathSegment) ||
			strings.TrimSpace(contextValue.Room.ID) != strings.TrimSpace(message.RoomID) {
			continue
		}
		repairCtx := contextWithExactQueueOwner(ctx, ownerUserID)
		goal := s.currentRoomGoalForSession(
			repairCtx,
			protocol.BuildRoomSharedSessionKey(message.ConversationID),
		)
		if goal == nil || strings.TrimSpace(goal.ID) != binding.GoalID ||
			goal.ObjectiveRevision() != binding.ObjectiveRevision {
			continue
		}
		if err := s.ensureGoalDirectedMessageHandoffs(
			contextValue,
			message,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reconcilePublicHandoff(ctx context.Context, handoff workspacestore.RoomPublicHandoff) error {
	conversationID := strings.TrimSpace(handoff.ConversationID)
	ctx, contextValue, err := s.internalConversationContext(ctx, conversationID, true)
	if err != nil {
		if errors.Is(err, roomsvc.ErrRoomNotFound) ||
			errors.Is(err, roomsvc.ErrConversationNotFound) {
			if markErr := s.publicHandoffs.MarkTerminal(
				handoff.OwnerUserID,
				conversationID,
				handoff.HandoffID,
				"interrupted",
			); markErr != nil {
				return markErr
			}
			return s.settleDiscardedGoalHandoff(handoff.OwnerUserID, conversationID, handoff)
		}
		return err
	}
	if contextValue == nil {
		return nil
	}
	ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
	if !roomdomain.IsMemberAgent(contextValue.Members, handoff.TargetAgentID) {
		if err := s.publicHandoffs.MarkTerminal(
			contextValue.Room.OwnerUserID,
			conversationID,
			handoff.HandoffID,
			"error",
		); err != nil {
			return err
		}
		return s.settleDiscardedGoalHandoff(ownerUserID, conversationID, handoff)
	}
	if binding := protocol.NormalizeGoalCollaborationBinding(
		handoff.GoalCollaborationBinding,
	); binding != nil {
		current, currentErr := s.roomGoalCollaborationBindingIsCurrent(
			ctx,
			conversationID,
			binding,
		)
		if currentErr != nil {
			return currentErr
		}
		if !current {
			if err := s.deletePublicHandoffQueueItems(ctx, contextValue, handoff); err != nil {
				return err
			}
			if err := s.publicHandoffs.MarkTerminal(
				ownerUserID,
				conversationID,
				handoff.HandoffID,
				"interrupted",
			); err != nil {
				return err
			}
			return s.settleDiscardedGoalHandoff(ownerUserID, conversationID, handoff)
		}
		if roomPublicHandoffIsTerminal(handoff.Status) {
			return s.reconcileTerminalRoomGoalHandoff(
				ctx,
				ownerUserID,
				conversationID,
				handoff,
				binding,
			)
		}
	}
	if handoff.Status == "queued" {
		present, queueErr := s.publicHandoffQueueItemPresent(ctx, contextValue, handoff)
		if queueErr != nil {
			return queueErr
		}
		if present {
			// 队列项仍然是 durable 真相；让正常队列恢复负责出队，
			// 不在这里再创建一条 target round。
			if s.inputQueue != nil {
				sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
				s.startSessionBackgroundTask(
					sessionKey,
					contextValue.Room.OwnerUserID,
					func(taskCtx context.Context) {
						s.dispatchNextInputQueueItem(
							taskCtx,
							sessionKey,
							contextValue.Room.ID,
							conversationID,
						)
					},
				)
			}
			return nil
		}
		// 出队与 target 启动之间崩溃时，队列项已经不存在；
		// 将 handoff 重新暴露为可 claim 的 source_finished。
		if err := s.publicHandoffs.MarkSourceFinished(
			contextValue.Room.OwnerUserID,
			conversationID,
			handoff.HandoffID,
		); err != nil {
			return err
		}
		handoff.Status = "source_finished"
	}
	if handoff.Status == "claimed" {
		// PendingAll is a process-start scan: a claim from the prior process has
		// no live owner even when its wall-clock TTL has not elapsed.
		if err := s.publicHandoffs.ReleaseClaim(
			contextValue.Room.OwnerUserID,
			conversationID,
			handoff.HandoffID,
		); err != nil {
			return err
		}
		handoff.Status = "source_finished"
	}
	if handoff.Status == "started" &&
		(handoff.WorkBinding != nil || handoff.ReviewBinding != nil ||
			protocol.NormalizeGoalCollaborationBinding(handoff.GoalCollaborationBinding) != nil) {
		// A process crash can happen after the target round is registered but
		// before its runtime query reaches a terminal result. Re-open only
		// handoffs carrying a structured Execution binding or exact Goal
		// collaboration attribution; the durable identity keeps admission
		// idempotent and stale-safe.
		if err := s.publicHandoffs.ReopenStartedForRecovery(
			contextValue.Room.OwnerUserID,
			conversationID,
			handoff.HandoffID,
		); err != nil {
			return err
		}
		handoff.Status = "source_finished"
	}
	if handoff.Status == "detected" {
		if handoff.QueueSource == protocol.InputQueueSourceAgentRoomMessage &&
			protocol.NormalizeGoalCollaborationBinding(handoff.GoalCollaborationBinding) != nil {
			return s.recoverGoalDirectedMessageHandoff(
				ctx,
				contextValue,
				handoff,
			)
		}
		if s.roomHistory == nil {
			return nil
		}
		messages, readErr := s.roomHistory.ReadMessages(
			contextValue.Room.OwnerUserID,
			conversationID,
			nil,
		)
		if readErr != nil {
			return readErr
		}
		if !roomHistoryContainsMessage(messages, handoff.SourceMessageID) {
			// detected 可能落在 source transcript 写入前的崩溃窗口；
			// 保留 ledger，下一次启动或 source 收尾路径继续处理。
			return nil
		}
		if err := s.publicHandoffs.MarkSourceFinished(
			contextValue.Room.OwnerUserID,
			conversationID,
			handoff.HandoffID,
		); err != nil {
			return err
		}
		handoff.Status = "source_finished"
	}
	sourceRoundID := strings.TrimSpace(handoff.SourceMessageID)
	rootRoundID := strings.TrimSpace(handoff.RootRoundID)
	if rootRoundID == "" {
		rootRoundID = sourceRoundID
	}
	coordinatorAgentID := roomCoordinatorAgentID(
		handoff.SourceAgentID,
		contextValue,
	)
	if handoff.ReviewBinding != nil {
		coordinatorAgentID = strings.TrimSpace(
			handoff.ReviewBinding.TargetAgentID,
		)
	}
	parentRound := &activeRoomRound{
		SessionKey:         protocol.BuildRoomSharedSessionKey(conversationID),
		RoomID:             contextValue.Room.ID,
		ConversationID:     conversationID,
		CoordinatorAgentID: coordinatorAgentID,
		RoomType:           contextValue.Room.RoomType,
		Context:            contextValue,
		RoundID:            sourceRoundID,
		RootRoundID:        rootRoundID,
		HopIndex:           handoff.HopIndex,
		OwnerUserID:        ownerUserID,
		Slots:              make(map[string]*activeRoomSlot),
	}
	triggerType := "public_mention"
	queueSource := protocol.NormalizeInputQueueSource(string(handoff.QueueSource))
	if queueSource == protocol.InputQueueSourceAgentRoomMessage {
		triggerType = roomDirectedMessageTriggerType
	} else {
		queueSource = protocol.InputQueueSourceAgentPublicMention
	}
	if handoff.WorkBinding != nil {
		triggerType = "execution_dispatch"
		queueSource = protocol.InputQueueSourceAgentRoomMessage
	} else if handoff.ReviewBinding != nil {
		triggerType = "execution_review_return"
		queueSource = protocol.InputQueueSourceAgentRoomMessage
	}
	wake := publicMentionWake{
		HandoffID:     handoff.HandoffID,
		TriggerType:   triggerType,
		QueueSource:   queueSource,
		SourceAgentID: handoff.SourceAgentID,
		TargetAgentID: handoff.TargetAgentID,
		Content:       handoff.Content,
		MessageID:     handoff.SourceMessageID,
		ReplyRoute:    handoff.ReplyRoute,
		GoalCollaborationBinding: cloneGoalCollaborationBinding(
			handoff.GoalCollaborationBinding,
		),
		WorkBinding:   cloneExecutionWorkBinding(handoff.WorkBinding),
		ReviewBinding: cloneExecutionReviewBinding(handoff.ReviewBinding),
	}
	lease := s.lockRoomDispatch(parentRound.SessionKey, parentRound.ConversationID)
	defer lease.Unlock()
	// internalConversationContext above already loaded the persistent Room state
	// under the same reconciliation turn. Avoid a duplicate system lookup while
	// still entering the common participation-aware wake path.
	return s.startPublicMentionRoundLocked(
		ctx,
		parentRound,
		[]publicMentionWake{wake},
		false,
	)
}

func (s *Service) settleDiscardedGoalHandoff(
	ownerUserID string,
	conversationID string,
	handoff workspacestore.RoomPublicHandoff,
) error {
	if protocol.NormalizeGoalCollaborationBinding(handoff.GoalCollaborationBinding) == nil {
		return nil
	}
	return s.publicHandoffs.MarkGoalHandbackSettled(
		ownerUserID,
		conversationID,
		handoff.HandoffID,
	)
}

func (s *Service) reconcileTerminalRoomGoalHandoff(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	handoff workspacestore.RoomPublicHandoff,
	binding *protocol.GoalCollaborationBinding,
) error {
	if s == nil || s.goals == nil || binding == nil {
		return errors.New("Goal provider is required for Room collaboration handback recovery")
	}
	if handoff.GoalPublicEvidence {
		roundID := firstNonEmptyString(
			handoff.TargetAgentRoundID,
			handoff.TargetRoundID,
			handoff.HandoffID,
		)
		if _, err := s.goals.RecordRoomGoalCollaborationEvidence(
			ctx,
			binding.GoalID,
			roundID,
			handoff.TargetAgentID,
			binding.ObjectiveRevision,
		); err != nil && !goalsvc.IsExpectedMutationError(err) {
			return err
		}
	}
	handbackRoundID := firstNonEmptyString(
		handoff.TargetRoundID,
		handoff.TargetAgentRoundID,
		handoff.HandoffID,
	)
	if _, err := s.goals.RecordRoomGoalCollaborationHandback(
		ctx,
		binding.GoalID,
		handbackRoundID,
		binding.ObjectiveRevision,
	); err != nil {
		if goalsvc.IsExpectedMutationError(err) {
			return s.settleDiscardedGoalHandoff(ownerUserID, conversationID, handoff)
		}
		return err
	}
	return s.publicHandoffs.MarkGoalHandbackSettled(
		ownerUserID,
		conversationID,
		handoff.HandoffID,
	)
}

func (s *Service) recoverGoalDirectedMessageHandoff(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	handoff workspacestore.RoomPublicHandoff,
) error {
	if s == nil || contextValue == nil || s.directedMessages == nil {
		return nil
	}
	messages, err := s.directedMessages.ReadMessages(
		contextValue.Room.OwnerUserID,
		contextValue.Conversation.ID,
	)
	if err != nil {
		return err
	}
	var source *protocol.RoomDirectedMessageRecord
	for index := range messages {
		if strings.TrimSpace(messages[index].MessageID) ==
			strings.TrimSpace(handoff.SourceMessageID) {
			source = &messages[index]
			break
		}
	}
	if source == nil {
		// Detect follows the directed-message append. A missing source means the
		// ledger is incomplete or forged; keep the exact Goal fence closed.
		return nil
	}
	if source.WakePolicy == protocol.RoomWakePolicyDelayed {
		if s.directedWakes == nil {
			return nil
		}
		pending, pendingErr := s.directedWakes.Pending(
			contextValue.Room.OwnerUserID,
		)
		if pendingErr != nil {
			return pendingErr
		}
		for _, wake := range pending {
			if strings.TrimSpace(wake.WakeID) == strings.TrimSpace(source.MessageID) {
				return nil
			}
		}
		dueAt := source.Timestamp + int64(time.Duration(source.DelaySeconds)*time.Second/time.Millisecond)
		wake := workspacestore.RoomDirectedMessageWake{
			WakeID:      strings.TrimSpace(source.MessageID),
			OwnerUserID: contextValue.Room.OwnerUserID,
			Message:     *source,
			DueAt:       dueAt,
			CreatedAt:   source.Timestamp,
		}
		if err := s.directedWakes.Schedule(wake); err != nil {
			return err
		}
		delay := time.Until(time.UnixMilli(dueAt))
		if delay < 0 {
			delay = 0
		}
		s.schedulePersistedRoomDirectedWake(wake, delay)
		return nil
	}
	return s.runPersistedImmediateRoomDirectedMessageWake(
		contextWithExactQueueOwner(ctx, contextValue.Room.OwnerUserID),
		contextValue,
		*source,
	)
}

func (s *Service) publicHandoffQueueItemPresent(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	handoff workspacestore.RoomPublicHandoff,
) (bool, error) {
	if s.inputQueue == nil || contextValue == nil {
		return false, nil
	}
	locations, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return false, err
	}
	location, ok := locations[strings.TrimSpace(handoff.TargetAgentID)]
	if !ok {
		return false, nil
	}
	items, err := s.inputQueue.Snapshot(location.Location)
	if err != nil {
		return false, err
	}
	structured := handoff.WorkBinding != nil || handoff.ReviewBinding != nil
	goalDirected := handoff.QueueSource == protocol.InputQueueSourceAgentRoomMessage &&
		protocol.NormalizeGoalCollaborationBinding(handoff.GoalCollaborationBinding) != nil
	present := false
	for _, item := range items {
		if strings.TrimSpace(item.HandoffID) != strings.TrimSpace(handoff.HandoffID) &&
			(strings.TrimSpace(handoff.QueueItemID) == "" || item.ID != handoff.QueueItemID) {
			continue
		}
		if (!structured && !goalDirected) || inputQueueItemMatchesDurableHandoff(item, handoff) {
			present = true
			continue
		}
		// The durable structured handoff owns this reserved identity. A row
		// with the same ID but a different capability must not suppress
		// recovery or be delivered as ordinary conversation.
		if _, deleteErr := s.inputQueue.Delete(location.Location, item.ID); deleteErr != nil {
			return false, deleteErr
		}
	}
	return present, nil
}

// deletePublicHandoffQueueItems removes only rows owned by one durable
// handoff identity. It is used when a Goal revision fence rejects startup
// recovery, so stale collaboration cannot remain parked in a busy Agent's
// queue and later reappear after another restart.
func (s *Service) deletePublicHandoffQueueItems(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	handoff workspacestore.RoomPublicHandoff,
) error {
	if s.inputQueue == nil || contextValue == nil {
		return nil
	}
	locations, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return err
	}
	location, ok := locations[strings.TrimSpace(handoff.TargetAgentID)]
	if !ok {
		return nil
	}
	items, err := s.inputQueue.Snapshot(location.Location)
	if err != nil {
		return err
	}
	for _, item := range items {
		if strings.TrimSpace(item.HandoffID) != strings.TrimSpace(handoff.HandoffID) &&
			(strings.TrimSpace(handoff.QueueItemID) == "" || item.ID != handoff.QueueItemID) {
			continue
		}
		if _, err = s.inputQueue.Delete(location.Location, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func inputQueueItemMatchesDurableHandoff(
	item protocol.InputQueueItem,
	handoff workspacestore.RoomPublicHandoff,
) bool {
	targetAgentID := strings.TrimSpace(handoff.TargetAgentID)
	targets := inputQueueTargetAgentIDs(item)
	if protocol.NormalizeInputQueueScope(string(item.Scope)) != protocol.InputQueueScopeRoom ||
		protocol.NormalizeInputQueueSource(string(item.Source)) != protocol.InputQueueSourceAgentRoomMessage ||
		protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)) != protocol.ChatDeliveryPolicyQueue ||
		strings.TrimSpace(item.AgentID) != targetAgentID ||
		len(targets) != 1 ||
		strings.TrimSpace(targets[0]) != targetAgentID ||
		strings.TrimSpace(item.RoomID) != strings.TrimSpace(handoff.RoomID) ||
		strings.TrimSpace(item.ConversationID) != strings.TrimSpace(handoff.ConversationID) ||
		strings.TrimSpace(item.OwnerUserID) != strings.TrimSpace(handoff.OwnerUserID) ||
		strings.TrimSpace(item.SourceAgentID) != strings.TrimSpace(handoff.SourceAgentID) ||
		strings.TrimSpace(item.SourceMessageID) != strings.TrimSpace(handoff.SourceMessageID) ||
		strings.TrimSpace(item.HandoffID) != strings.TrimSpace(handoff.HandoffID) ||
		strings.TrimSpace(item.RootRoundID) != strings.TrimSpace(handoff.RootRoundID) ||
		strings.TrimSpace(item.Content) != strings.TrimSpace(handoff.Content) ||
		!reflect.DeepEqual(item.ReplyRoute, handoff.ReplyRoute) {
		return false
	}
	if candidate := protocol.NormalizeGoalCollaborationBinding(item.GoalCollaborationBinding); candidate == nil {
		if protocol.NormalizeGoalCollaborationBinding(handoff.GoalCollaborationBinding) != nil {
			return false
		}
	} else if expected := protocol.NormalizeGoalCollaborationBinding(handoff.GoalCollaborationBinding); expected == nil || *candidate != *expected {
		return false
	}
	if handoff.WorkBinding != nil {
		return item.ReviewBinding == nil &&
			executionWorkBindingEqual(item.WorkBinding, handoff.WorkBinding)
	}
	if handoff.ReviewBinding != nil {
		return item.WorkBinding == nil &&
			executionReviewBindingEqual(item.ReviewBinding, handoff.ReviewBinding)
	}
	return item.WorkBinding == nil && item.ReviewBinding == nil
}

func roomHistoryContainsMessage(messages []protocol.Message, messageID string) bool {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return false
	}
	for _, message := range messages {
		if strings.TrimSpace(anyString(message["message_id"])) == messageID {
			return true
		}
	}
	return false
}
