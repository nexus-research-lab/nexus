package room

import (
	"context"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *RealtimeService) roomSlotGuidanceHook(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	location workspacestore.InputQueueLocation,
) sdkprotocol.HookCallback {
	return func(ctx context.Context, input sdkprotocol.HookInput, _ string) (sdkprotocol.HookOutput, error) {
		if input.EventName != "" && input.EventName != sdkprotocol.HookEventPostToolUse {
			return sdkprotocol.HookOutput{}, nil
		}
		queuedInputs := slot.drainGuidedInputs()
		queueItems, _, err := s.inputQueue.DispatchGuidance(location, slot.AgentRoundID)
		if err != nil {
			return sdkprotocol.HookOutput{}, err
		}
		if len(queuedInputs) == 0 && len(queueItems) == 0 {
			return sdkprotocol.HookOutput{}, nil
		}
		if len(queueItems) > 0 && roundValue != nil && roundValue.Context != nil {
			if err := s.broadcastRoomInputQueueSnapshot(ctx, roundValue.SessionKey, roundValue.Context); err != nil {
				s.loggerFor(ctx).Warn("广播 Room 引导队列消费快照失败",
					"session_key", roundValue.SessionKey,
					"room_id", roundValue.RoomID,
					"conversation_id", roundValue.ConversationID,
					"agent_id", slot.AgentID,
					"err", err,
				)
			}
		}

		sourceRoundID, triggerContent := latestGuidanceTrigger(queuedInputs, queueItems)
		inputs := make([]runtimectx.GuidedInput, 0, 1+len(queuedInputs)+len(queueItems))
		if roundValue != nil && roundValue.Context != nil {
			agentNameByID, _, directoryErr := s.buildAgentDirectory(ctx, roundValue.Context)
			if directoryErr != nil {
				return sdkprotocol.HookOutput{}, directoryErr
			}
			publicHistory, historyErr := s.roomHistory.ReadMessages(roundValue.ConversationID, nil)
			if historyErr != nil {
				return sdkprotocol.HookOutput{}, historyErr
			}
			publicContext, contextErr := s.buildSlotGuidedPublicContext(ctx, roundValue, slot, publicHistory, agentNameByID, roomTrigger{
				TriggerType:   "public_chat",
				Content:       triggerContent,
				MessageID:     strings.TrimSpace(sourceRoundID),
				TargetAgentID: slot.AgentID,
			})
			if contextErr != nil {
				return sdkprotocol.HookOutput{}, contextErr
			}
			if strings.TrimSpace(publicContext) != "" {
				inputs = append(inputs, runtimectx.GuidedInput{
					RoundID: sourceRoundID,
					Content: publicContext,
				})
			}
		}
		if len(inputs) == 0 {
			for _, item := range queuedInputs {
				inputs = append(inputs, runtimectx.GuidedInput{
					RoundID: item.RoundID,
					Content: item.Content,
				})
			}
			for _, item := range queueItems {
				sourceRoundID := "queue_" + strings.TrimSpace(item.ID)
				inputs = append(inputs, runtimectx.GuidedInput{
					RoundID: sourceRoundID,
					Content: item.Content,
				})
			}
		}
		for _, item := range queueItems {
			sourceRoundID := "queue_" + strings.TrimSpace(item.ID)
			if roundValue != nil {
				guidanceMessage := buildRoomGuidanceMessage(
					roundValue.SessionKey,
					roundValue.RoomID,
					roundValue.ConversationID,
					slot,
					sourceRoundID,
					item.Content,
				)
				s.broadcastSlotGuidanceMessage(ctx, roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID, sourceRoundID, guidanceMessage)
			}
		}
		return sdkprotocol.HookOutput{
			HookSpecificOutput: map[string]any{
				"hookEventName":     string(sdkprotocol.HookEventPostToolUse),
				"additionalContext": runtimectx.FormatGuidanceAdditionalContext(inputs),
			},
		}, nil
	}
}

func latestGuidanceTrigger(queuedInputs []roomQueuedInput, queueItems []protocol.InputQueueItem) (string, string) {
	roundID := ""
	content := ""
	for _, item := range queuedInputs {
		if strings.TrimSpace(item.RoundID) != "" {
			roundID = strings.TrimSpace(item.RoundID)
		}
		if strings.TrimSpace(item.Content) != "" {
			content = strings.TrimSpace(item.Content)
		}
	}
	for _, item := range queueItems {
		if strings.TrimSpace(item.ID) != "" {
			roundID = "queue_" + strings.TrimSpace(item.ID)
		}
		if strings.TrimSpace(item.Content) != "" {
			content = strings.TrimSpace(item.Content)
		}
	}
	return roundID, content
}
