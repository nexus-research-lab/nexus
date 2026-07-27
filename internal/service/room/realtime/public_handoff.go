// INPUT: Room 最终 assistant 公区消息、成员目录与 source slot 身份。
// OUTPUT: 带 agent_mentions 标注的消息，以及保留 owner/root scope、可幂等恢复的 handoff ledger 记录。
// POS: @ 解析、正文 span 与 handoff identity 的单一收口。
package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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

// buildRoomMentionAnnotations 统一 Room 公区 mention 的成员过滤和 handoff 选择。
// 普通 assistant 输出与主动发布消息必须共享这条规则，避免两条链路产生不同的协作语义。
func buildRoomMentionAnnotations(
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	messageID string,
	blocks []roomMentionTextBlock,
	fanout bool,
) []protocol.AgentMention {
	resolved := resolveRoomMentionMatches(contextValue, sourceAgentID, blocks)
	if len(resolved) == 0 {
		return nil
	}

	selectedTargets := make(map[string]struct{}, len(resolved))
	for _, item := range resolved {
		targetAgentID := strings.TrimSpace(item.match.AgentID)
		if fanout || len(selectedTargets) == 0 {
			selectedTargets[targetAgentID] = struct{}{}
		}
	}

	messageID = strings.TrimSpace(messageID)
	result := make([]protocol.AgentMention, 0, len(resolved))
	for _, item := range resolved {
		targetAgentID := strings.TrimSpace(item.match.AgentID)
		handoffID := ""
		if _, ok := selectedTargets[targetAgentID]; ok {
			handoffID = roomPublicHandoffID(
				contextValue.Conversation.ID,
				messageID,
				targetAgentID,
			)
		}
		result = append(result, protocol.AgentMention{
			AgentID:           targetAgentID,
			Label:             strings.TrimSpace(item.match.Label),
			ContentBlockIndex: item.block.index,
			StartRune:         item.match.StartRune,
			EndRune:           item.match.EndRune,
			HandoffID:         handoffID,
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

func (s *Service) transformRoomDurableMessage(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) protocol.Message {
	setRoomDisplayOrder(slot, message)
	if !roomShouldAnnotatePublicMessage(roundValue, slot, message) {
		return message
	}
	if err := s.annotatePublicAssistantMessage(roundValue, slot, message); err != nil {
		s.loggerFor(context.Background()).Warn("Room 公区 @ 标注写入 handoff ledger 失败",
			"conversation_id", roundValue.ConversationID,
			"message_id", strings.TrimSpace(anyString(message["message_id"])),
			"err", err,
		)
	}
	return message
}

// setRoomDisplayOrder 为同一 root round 的 Agent 回复提供跨重启稳定的并列顺序。
// 时间戳负责事实排序，slot index 只处理同一毫秒内的并发 tie-break。
func setRoomDisplayOrder(slot *activeRoomSlot, message protocol.Message) {
	if slot == nil || message == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	if protocol.Int64FromAny(message["display_order"]) > 0 {
		return
	}
	timestamp := protocol.Int64FromAny(message["timestamp"])
	if timestamp <= 0 {
		timestamp = slot.TimestampMS
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
	// 控制标记只参与路由决策，不进入持久化正文；清理后重新计算 span，
	// 确保前端偏移始终对应用户实际看到的文本。
	fanout := roomdomain.HasFanoutMarker(message)
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
		fanout,
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
		_, _, err := s.publicHandoffs.Detect(workspacestore.RoomPublicHandoff{
			HandoffID:          handoffID,
			ConversationID:     roundValue.ConversationID,
			RoomID:             roundValue.RoomID,
			RootRoundID:        roomRootRoundID(roundValue),
			SourceAgentRoundID: strings.TrimSpace(slot.AgentRoundID),
			SourceMessageID:    messageID,
			SourceAgentID:      strings.TrimSpace(slot.AgentID),
			TargetAgentID:      strings.TrimSpace(mention.AgentID),
			Content:            content,
			HopIndex:           roundValue.HopIndex,
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
	if err := s.publicHandoffs.MarkTerminal(roundValue.ConversationID, handoffID, status); err != nil {
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
	if err := s.publicHandoffs.CancelForSource(roundValue.ConversationID, slot.AgentRoundID, status); err != nil {
		s.loggerFor(ctx).Warn("取消 Room source handoff 失败", "agent_round_id", slot.AgentRoundID, "err", err)
	}
}

func (s *Service) markRoomQueueHandoffTerminal(
	conversationID string,
	item protocol.InputQueueItem,
) error {
	if s.publicHandoffs == nil || strings.TrimSpace(item.HandoffID) == "" {
		return nil
	}
	return s.publicHandoffs.MarkTerminal(conversationID, item.HandoffID, "finished")
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
	edges, err := s.publicHandoffs.ListRoot(roundValue.ConversationID, rootRoundID)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 Room root handoff 失败", "root", rootRoundID, "err", err)
		return
	}
	if err = s.publicHandoffs.CancelForRoot(roundValue.ConversationID, rootRoundID, status); err != nil {
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
		if entry.Item.Source != protocol.InputQueueSourceAgentPublicMention {
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

// INPUT: 进程启动时 handoff ledger 中尚未完成的 source_finished 记录。
// OUTPUT: 重新进入统一 busy/idle 派发路径的 target wake。
// POS: Room 公区协作的 durable recovery 边界。
// StartPublicHandoffReconciler 恢复进程退出前已确认 source 成功但尚未启动的 handoff。
func (s *Service) StartPublicHandoffReconciler(ctx context.Context) (func(), error) {
	if s == nil || s.publicHandoffs == nil || s.rooms == nil {
		return nil, nil
	}
	pending, err := s.publicHandoffs.PendingAll()
	if err != nil {
		return nil, err
	}
	for _, handoff := range pending {
		if err := s.reconcilePublicHandoff(ctx, handoff); err != nil {
			s.loggerFor(ctx).Warn("恢复 Room 公区 handoff 失败",
				"conversation_id", handoff.ConversationID,
				"handoff_id", handoff.HandoffID,
				"err", err,
			)
		}
	}
	return nil, nil
}

func (s *Service) reconcilePublicHandoff(ctx context.Context, handoff workspacestore.RoomPublicHandoff) error {
	conversationID := strings.TrimSpace(handoff.ConversationID)
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	if contextValue == nil {
		return nil
	}
	ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
	if !roomdomain.IsMemberAgent(contextValue.Members, handoff.TargetAgentID) {
		return s.publicHandoffs.MarkTerminal(conversationID, handoff.HandoffID, "error")
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
				go s.dispatchNextInputQueueItem(
					contextWithQueueOwner(context.Background(), ""),
					protocol.BuildRoomSharedSessionKey(conversationID),
					contextValue.Room.ID,
					conversationID,
				)
			}
			return nil
		}
		// 出队与 target 启动之间崩溃时，队列项已经不存在；
		// 将 handoff 重新暴露为可 claim 的 source_finished。
		if err := s.publicHandoffs.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
			return err
		}
		handoff.Status = "source_finished"
	}
	if handoff.Status == "detected" {
		if s.roomHistory == nil {
			return nil
		}
		messages, readErr := s.roomHistory.ReadMessages(conversationID, nil)
		if readErr != nil {
			return readErr
		}
		if !roomHistoryContainsMessage(messages, handoff.SourceMessageID) {
			// detected 可能落在 source transcript 写入前的崩溃窗口；
			// 保留 ledger，下一次启动或 source 收尾路径继续处理。
			return nil
		}
		if err := s.publicHandoffs.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
			return err
		}
		handoff.Status = "source_finished"
	}
	sourceRoundID := strings.TrimSpace(handoff.SourceMessageID)
	rootRoundID := strings.TrimSpace(handoff.RootRoundID)
	if rootRoundID == "" {
		rootRoundID = sourceRoundID
	}
	parentRound := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
		RoomID:         contextValue.Room.ID,
		ConversationID: conversationID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        sourceRoundID,
		RootRoundID:    rootRoundID,
		HopIndex:       handoff.HopIndex,
		OwnerUserID:    ownerUserID,
		Slots:          make(map[string]*activeRoomSlot),
	}
	wake := publicMentionWake{
		HandoffID:     handoff.HandoffID,
		TriggerType:   "public_mention",
		QueueSource:   protocol.InputQueueSourceAgentPublicMention,
		SourceAgentID: handoff.SourceAgentID,
		TargetAgentID: handoff.TargetAgentID,
		Content:       handoff.Content,
		MessageID:     handoff.SourceMessageID,
		ReplyRoute:    handoff.ReplyRoute,
	}
	return s.startPublicMentionRound(ctx, parentRound, []publicMentionWake{wake})
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
	for _, item := range items {
		if strings.TrimSpace(item.HandoffID) == strings.TrimSpace(handoff.HandoffID) ||
			(strings.TrimSpace(handoff.QueueItemID) != "" && item.ID == handoff.QueueItemID) {
			return true, nil
		}
	}
	return false, nil
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
