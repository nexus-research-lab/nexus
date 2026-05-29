package room

import (
	"context"
	"fmt"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"
)

func (s *RealtimeService) memoryOptions() memorysvc.MemoryOptions {
	return memorysvc.MemoryOptions{
		Enabled:        s.config.MemoryEnabled,
		AutoRecall:     s.config.MemoryAutoRecall,
		AutoExtract:    s.config.MemoryAutoExtract,
		MaxResults:     s.config.MemoryMaxResults,
		ScoreThreshold: s.config.MemoryScoreThreshold,
	}.Normalize()
}

func (s *RealtimeService) roomSharedMemoryRoot(conversationID string) string {
	return workspacestore.New(s.config.WorkspacePath).RoomConversationDir(conversationID)
}

func (s *RealtimeService) buildRoomMemorySystemPrompt(ctx context.Context, roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	engine := memorysvc.NewEngine(s.roomSharedMemoryRoot(roundValue.ConversationID), s.memoryOptions())
	stable, err := engine.StableContext(ctx, 2400)
	if err != nil {
		s.loggerFor(ctx).Warn("Room 热记忆读取失败",
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"err", err,
		)
		return ""
	}
	if strings.TrimSpace(stable) == "" {
		return ""
	}
	return "# Room Shared Hot Memory\n\n" + stable
}

func (s *RealtimeService) prependRoomMemoryContext(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	agentValue *protocol.Agent,
	base string,
) string {
	if roundValue == nil || slot == nil || strings.TrimSpace(base) == "" {
		return base
	}
	query := strings.TrimSpace(slot.Trigger.Content)
	if query == "" {
		query = base
	}
	contexts := make([]string, 0, 2)
	shared := s.recallRoomMemory(ctx, s.roomSharedMemoryRoot(roundValue.ConversationID), memorysvc.MemoryScope{
		Kind:           memorysvc.ScopeKindRoomShared,
		UserID:         roundValue.OwnerUserID,
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
	}, query)
	if shared != "" {
		contexts = append(contexts, shared)
	}
	if agentValue != nil {
		agentSpecific := s.recallRoomMemory(ctx, agentValue.WorkspacePath, memorysvc.MemoryScope{
			Kind:           memorysvc.ScopeKindRoomAgentSession,
			UserID:         roundValue.OwnerUserID,
			AgentID:        slot.AgentID,
			SessionKey:     slot.RuntimeSessionKey,
			SessionID:      slot.getSDKSessionID(),
			RoomID:         roundValue.RoomID,
			ConversationID: roundValue.ConversationID,
		}, query)
		if agentSpecific != "" {
			contexts = append(contexts, agentSpecific)
		}
	}
	if len(contexts) == 0 {
		return base
	}
	return strings.Join(contexts, "\n\n") + "\n\n" + base
}

func (s *RealtimeService) recallRoomMemory(ctx context.Context, workspacePath string, scope memorysvc.MemoryScope, query string) string {
	engine := memorysvc.NewEngine(workspacePath, s.memoryOptions())
	injection, err := engine.BeforeRecall(ctx, scope, memorysvc.RecallRequest{
		Query:      query,
		MaxResults: s.config.MemoryMaxResults,
	})
	if err != nil {
		s.loggerFor(ctx).Warn("Room 动态记忆召回失败",
			"scope", scope.Key(),
			"err", err,
		)
		return ""
	}
	return strings.TrimSpace(injection.DynamicUserContext)
}

func (s *RealtimeService) commitRoomMemoryTurn(roundValue *activeRoomRound, slot *activeRoomSlot, assistant protocol.Message) {
	if roundValue == nil || slot == nil {
		return
	}
	assistantText := roomdomain.ExtractAssistantResultText(assistant)
	if strings.TrimSpace(assistantText) == "" || strings.TrimSpace(roomSlotInterruptReason(slot)) != "" {
		return
	}
	if strings.Contains(assistantText, roomdomain.NoReplyMarker) {
		return
	}
	userText := strings.TrimSpace(slot.Trigger.Content)
	if userText == "" {
		userText = fmt.Sprintf("Room round %s public reply", roundValue.RoundID)
	}
	sharedEngine := memorysvc.NewEngine(s.roomSharedMemoryRoot(roundValue.ConversationID), s.memoryOptions())
	_, sharedErr := sharedEngine.CommitTurn(context.Background(), memorysvc.MemoryScope{
		Kind:           memorysvc.ScopeKindRoomShared,
		UserID:         roundValue.OwnerUserID,
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
	}, memorysvc.CommittedTurn{
		UserText:       userText,
		AssistantText:  assistantText,
		SessionKey:     roundValue.SessionKey,
		RoundID:        slot.AgentRoundID,
		AgentID:        slot.AgentID,
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
	})
	if sharedErr != nil {
		s.loggerFor(context.Background()).Warn("Room 共享记忆提交失败",
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"agent_id", slot.AgentID,
			"round_id", slot.AgentRoundID,
			"err", sharedErr,
		)
	}
	if strings.TrimSpace(slot.WorkspacePath) == "" {
		return
	}
	agentEngine := memorysvc.NewEngine(slot.WorkspacePath, s.memoryOptions())
	_, agentErr := agentEngine.CommitTurn(context.Background(), memorysvc.MemoryScope{
		Kind:           memorysvc.ScopeKindRoomAgentSession,
		UserID:         roundValue.OwnerUserID,
		AgentID:        slot.AgentID,
		SessionKey:     slot.RuntimeSessionKey,
		SessionID:      slot.getSDKSessionID(),
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
	}, memorysvc.CommittedTurn{
		UserText:       userText,
		AssistantText:  assistantText,
		SessionKey:     slot.RuntimeSessionKey,
		SessionID:      slot.getSDKSessionID(),
		RoundID:        slot.AgentRoundID,
		AgentID:        slot.AgentID,
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
	})
	if agentErr != nil {
		s.loggerFor(context.Background()).Warn("Room 成员记忆提交失败",
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"agent_id", slot.AgentID,
			"round_id", slot.AgentRoundID,
			"err", agentErr,
		)
	}
}

func roomSlotCanCommitMemory(slot *activeRoomSlot) bool {
	return slot != nil &&
		slot.getStatus() == "finished" &&
		!slot.shouldSuppressOutput() &&
		roomSlotPublishesPublicOutput(slot)
}
