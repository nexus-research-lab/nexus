// INPUT: 运行中 Room slot、内存引导和持久化 guide 队列。
// OUTPUT: PostToolUse additionalContext，并把已消费引导归入实际回复 round。
// POS: Room 轮内插话的唯一消费入口。
package room

import (
	"context"
	"strings"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func buildRoomGuidanceMessage(
	sessionKey string,
	roomID string,
	conversationID string,
	slot *activeRoomSlot,
	sourceRoundID string,
	content string,
) protocol.Message {
	if slot == nil {
		return protocol.Message{}
	}
	return roomdomain.BuildGuidanceMessage(roomdomain.GuidanceMessageInput{
		SessionKey:     sessionKey,
		RoomID:         roomID,
		ConversationID: conversationID,
		AgentID:        slot.AgentID,
		AgentRoundID:   slot.AgentRoundID,
		SourceRoundID:  sourceRoundID,
		Content:        content,
		SDKSessionID:   slot.getSDKSessionID(),
	})
}

func (s *RealtimeService) broadcastSlotGuidanceMessage(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ string,
	_ protocol.Message,
) {
	// 引导消息只进入运行中 slot 的执行链路，不能作为公区输出事件展示。
}

func (s *RealtimeService) roomSlotGuidanceHook(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	location workspacestore.InputQueueLocation,
) sdkhook.Callback {
	return func(ctx context.Context, input sdkhook.Input, _ string) (sdkhook.Output, error) {
		if input.EventName != "" && input.EventName != sdkhook.EventPostToolUse {
			return sdkhook.Output{}, nil
		}
		execution := roomGuidanceExecution{
			service:  s,
			ctx:      ctx,
			round:    roundValue,
			slot:     slot,
			location: location,
		}
		return execution.run()
	}
}

type roomGuidanceExecution struct {
	service           *RealtimeService
	ctx               context.Context
	round             *activeRoomRound
	slot              *activeRoomSlot
	location          workspacestore.InputQueueLocation
	queuedInputs      []roomQueuedInput
	queueItems        []protocol.InputQueueItem
	runtimeQueueItems []protocol.InputQueueItem
	sourceRoundID     string
	triggerContent    string
	inputs            []runtimectx.GuidedInput
}

func (e *roomGuidanceExecution) run() (sdkhook.Output, error) {
	hasInput, err := e.loadInputs()
	if err != nil || !hasInput {
		return sdkhook.Output{}, err
	}
	e.broadcastQueueSnapshot()
	if err = e.renderQueueItems(); err != nil {
		return sdkhook.Output{}, err
	}
	e.sourceRoundID, e.triggerContent = latestGuidanceTrigger(e.queuedInputs, e.runtimeQueueItems)
	if err = e.buildInputs(); err != nil {
		return sdkhook.Output{}, err
	}
	e.syncConsumedPublicMessages()
	e.broadcastGuidanceMessages()
	return sdkhook.Output{
		SpecificOutput: &sdkhook.SpecificOutput{
			HookEventName:     sdkhook.EventPostToolUse,
			AdditionalContext: runtimectx.FormatGuidanceAdditionalContext(e.inputs),
		},
	}, nil
}

func (e *roomGuidanceExecution) syncConsumedPublicMessages() {
	if e.round == nil || e.round.Context == nil {
		return
	}
	rootRoundID := roomRootRoundID(e.round)
	for _, input := range e.queuedInputs {
		e.service.syncQueuedPublicMessageDeliveryPolicy(e.ctx, e.round.SessionKey, e.round.Context, protocol.InputQueueItem{
			SourceMessageID: input.RoundID,
			DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
		}, rootRoundID)
	}
	for _, item := range e.queueItems {
		e.service.syncQueuedPublicMessageDeliveryPolicy(e.ctx, e.round.SessionKey, e.round.Context, item, rootRoundID)
	}
}

func (e *roomGuidanceExecution) loadInputs() (bool, error) {
	e.queuedInputs = e.slot.drainGuidedInputs()
	queueItems, _, err := e.service.inputQueue.DispatchGuidance(e.location, e.slot.AgentRoundID)
	if err != nil {
		return false, err
	}
	e.queueItems = queueItems
	return len(e.queuedInputs) > 0 || len(e.queueItems) > 0, nil
}

func (e *roomGuidanceExecution) broadcastQueueSnapshot() {
	if len(e.queueItems) == 0 || e.round == nil || e.round.Context == nil {
		return
	}
	if err := e.service.broadcastRoomInputQueueSnapshot(e.ctx, e.round.SessionKey, e.round.Context); err != nil {
		e.service.loggerFor(e.ctx).Warn("广播 Room 引导队列消费快照失败",
			"session_key", e.round.SessionKey,
			"room_id", e.round.RoomID,
			"conversation_id", e.round.ConversationID,
			"agent_id", e.slot.AgentID,
			"err", err,
		)
	}
}

func (e *roomGuidanceExecution) renderQueueItems() error {
	e.runtimeQueueItems = make([]protocol.InputQueueItem, 0, len(e.queueItems))
	for _, item := range e.queueItems {
		runtimeContent, err := e.service.renderRuntimeContentWithAttachments(e.ctx, item.Content, item.Attachments)
		if err != nil {
			return err
		}
		// 轮内只注入排队内容；情绪等动态上下文留在 round trigger，避免破坏 prompt 前缀缓存。
		item.Content = runtimeContent.PlainText()
		e.runtimeQueueItems = append(e.runtimeQueueItems, item)
	}
	return nil
}

func (e *roomGuidanceExecution) buildInputs() error {
	e.inputs = make([]runtimectx.GuidedInput, 0, 1+len(e.queuedInputs)+len(e.queueItems))
	if err := e.appendPublicContext(); err != nil {
		return err
	}
	e.inputs = appendUnanchoredGuidanceQueueItems(e.inputs, e.runtimeQueueItems)
	if len(e.inputs) == 0 {
		e.appendFallbackInputs()
	}
	return nil
}

func (e *roomGuidanceExecution) appendPublicContext() error {
	if e.round == nil || e.round.Context == nil {
		return nil
	}
	agentNameByID, _, err := e.service.buildAgentDirectory(e.ctx, e.round.Context)
	if err != nil {
		return err
	}
	publicHistory, err := e.service.roomHistory.ReadMessages(e.round.ConversationID, nil)
	if err != nil {
		return err
	}
	publicContext, err := e.service.buildSlotGuidedPublicContext(e.ctx, e.round, e.slot, publicHistory, agentNameByID, roomTrigger{
		TriggerType:   "public_chat",
		Content:       e.triggerContent,
		MessageID:     strings.TrimSpace(e.sourceRoundID),
		TargetAgentID: e.slot.AgentID,
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(publicContext) != "" {
		e.inputs = append(e.inputs, runtimectx.GuidedInput{RoundID: e.sourceRoundID, Content: publicContext})
	}
	return nil
}

func (e *roomGuidanceExecution) appendFallbackInputs() {
	for _, item := range e.queuedInputs {
		e.inputs = append(e.inputs, runtimectx.GuidedInput{RoundID: item.RoundID, Content: item.Content})
	}
	for _, item := range e.runtimeQueueItems {
		e.inputs = append(e.inputs, runtimectx.GuidedInput{
			RoundID: "queue_" + strings.TrimSpace(item.ID),
			Content: item.Content,
		})
	}
}

func (e *roomGuidanceExecution) broadcastGuidanceMessages() {
	if e.round == nil {
		return
	}
	for _, item := range e.queueItems {
		sourceRoundID := "queue_" + strings.TrimSpace(item.ID)
		guidanceMessage := buildRoomGuidanceMessage(
			e.round.SessionKey,
			e.round.RoomID,
			e.round.ConversationID,
			e.slot,
			sourceRoundID,
			item.Content,
		)
		e.service.broadcastSlotGuidanceMessage(
			e.ctx,
			e.round.SessionKey,
			e.round.RoomID,
			e.round.ConversationID,
			sourceRoundID,
			guidanceMessage,
		)
	}
}

func appendUnanchoredGuidanceQueueItems(
	inputs []runtimectx.GuidedInput,
	queueItems []protocol.InputQueueItem,
) []runtimectx.GuidedInput {
	for _, item := range queueItems {
		if strings.TrimSpace(item.SourceMessageID) != "" {
			continue
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		inputs = append(inputs, runtimectx.GuidedInput{
			RoundID: "queue_" + strings.TrimSpace(item.ID),
			Content: content,
		})
	}
	return inputs
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
