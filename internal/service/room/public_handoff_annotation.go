// INPUT: Room 最终 assistant 公区消息、成员目录与 source slot 身份。
// OUTPUT: 带 agent_mentions 标注的消息，以及可幂等恢复的 handoff ledger 记录。
// POS: @ 解析、正文 span 与 handoff identity 的单一收口。
package room

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type roomMentionTextBlock struct {
	index int
	text  string
}

func (s *RealtimeService) transformRoomDurableMessage(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) protocol.Message {
	setRoomDisplayOrder(slot, message)
	if !roomShouldAnnotatePublicMessage(roundValue, slot, message) {
		return message
	}
	if _, exists := message["agent_mentions"]; exists {
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

func (s *RealtimeService) annotatePublicAssistantMessage(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	blocks := roomMentionTextBlocks(message["content"])
	if len(blocks) == 0 {
		if content := roomdomain.ExtractAssistantResultText(message); content != "" {
			blocks = []roomMentionTextBlock{{index: 0, text: content}}
		}
	}
	if len(blocks) == 0 {
		return nil
	}
	aliases := roomdomain.BuildMentionAliases(roundValue.Context)
	if len(aliases) == 0 {
		return nil
	}
	messageID := strings.TrimSpace(anyString(message["message_id"]))
	if messageID == "" {
		return nil
	}
	mentionValues := make([]protocol.AgentMention, 0)
	handoffByAgent := make(map[string]string)
	for _, block := range blocks {
		for _, match := range roomdomain.ResolveMentionMatches(block.text, aliases) {
			targetAgentID := strings.TrimSpace(match.AgentID)
			if targetAgentID == "" || targetAgentID == strings.TrimSpace(slot.AgentID) ||
				!roomdomain.IsMemberAgent(roundValue.Context.Members, targetAgentID) {
				continue
			}
			handoffID := handoffByAgent[targetAgentID]
			if handoffID == "" {
				handoffID = roomPublicHandoffID(roundValue.ConversationID, messageID, targetAgentID)
				handoffByAgent[targetAgentID] = handoffID
				if s.publicHandoffs != nil {
					_, _, err := s.publicHandoffs.Detect(workspacestore.RoomPublicHandoff{
						HandoffID:          handoffID,
						ConversationID:     roundValue.ConversationID,
						RoomID:             roundValue.RoomID,
						RootRoundID:        roomRootRoundID(roundValue),
						SourceAgentRoundID: strings.TrimSpace(slot.AgentRoundID),
						SourceMessageID:    messageID,
						SourceAgentID:      strings.TrimSpace(slot.AgentID),
						TargetAgentID:      targetAgentID,
						Content:            strings.TrimSpace(roomdomain.ExtractAssistantResultText(message)),
						HopIndex:           roundValue.HopIndex,
					})
					if err != nil {
						return err
					}
				}
			}
			mentionValues = append(mentionValues, protocol.AgentMention{
				AgentID:           targetAgentID,
				Label:             strings.TrimSpace(match.Label),
				ContentBlockIndex: block.index,
				StartRune:         match.StartRune,
				EndRune:           match.EndRune,
				HandoffID:         handoffID,
			})
		}
	}
	if len(mentionValues) > 0 {
		message["agent_mentions"] = mentionValues
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

func (s *RealtimeService) markPublicHandoffTerminal(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	status string,
) {
	if s.publicHandoffs == nil || roundValue == nil || slot == nil {
		return
	}
	handoffID := strings.TrimSpace(slot.HandoffID)
	if handoffID == "" {
		return
	}
	if err := s.publicHandoffs.MarkTerminal(roundValue.ConversationID, handoffID, status); err != nil {
		s.loggerFor(ctx).Warn("记录 Room handoff 终态失败", "handoff_id", handoffID, "status", status, "err", err)
	}
}

func (s *RealtimeService) cancelSourcePublicHandoffs(
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

func (s *RealtimeService) markRoomQueueHandoffTerminal(
	conversationID string,
	item protocol.InputQueueItem,
) error {
	if s.publicHandoffs == nil || strings.TrimSpace(item.HandoffID) == "" {
		return nil
	}
	return s.publicHandoffs.MarkTerminal(conversationID, item.HandoffID, "finished")
}
