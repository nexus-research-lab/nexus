// INPUT: 运行中 Room slot 与持久化 guide 队列。
// OUTPUT: PostToolUse additionalContext，并把已消费引导归入实际回复 round。
// POS: Room 轮内插话的唯一消费入口。
package room

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *RealtimeService) roomSlotGuidanceHook(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	location workspacestore.InputQueueLocation,
) sdkhook.Callback {
	var hookMu sync.Mutex
	return func(ctx context.Context, input sdkhook.Input, _ string) (sdkhook.Output, error) {
		hookMu.Lock()
		defer hookMu.Unlock()
		if input.EventName != "" && input.EventName != sdkhook.EventPostToolUse {
			return sdkhook.Output{}, nil
		}
		if err := s.acknowledgeRoomSlotGuidance(ctx, roundValue, slot); err != nil {
			return sdkhook.Output{}, err
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
	queueItems        []protocol.InputQueueItem
	runtimeQueueItems []protocol.InputQueueItem
	sourceRoundID     string
	triggerContent    string
	inputs            []runtimectx.GuidedInput
}

// ponytail: bridge 没有 control_response post-send ack；下一次 hook/result 才确认，进程崩溃窗口允许安全重投。
type pendingRoomGuidance struct {
	location workspacestore.InputQueueLocation
	items    []protocol.InputQueueItem
}

func (e *roomGuidanceExecution) run() (sdkhook.Output, error) {
	hasInput, err := e.loadInputs()
	if err != nil || !hasInput {
		return sdkhook.Output{}, err
	}
	if err = e.renderQueueItems(); err != nil {
		return sdkhook.Output{}, err
	}
	e.sourceRoundID, e.triggerContent = latestGuidanceTrigger(e.runtimeQueueItems)
	if err = e.buildInputs(); err != nil {
		return sdkhook.Output{}, err
	}
	e.service.rememberRoomSlotGuidance(e.slot, e.location, e.queueItems)
	return sdkhook.Output{
		SpecificOutput: &sdkhook.SpecificOutput{
			HookEventName:     sdkhook.EventPostToolUse,
			AdditionalContext: runtimectx.FormatGuidanceAdditionalContext(e.inputs),
		},
	}, nil
}

func (s *RealtimeService) rememberRoomSlotGuidance(
	slot *activeRoomSlot,
	location workspacestore.InputQueueLocation,
	items []protocol.InputQueueItem,
) {
	if slot == nil || len(items) == 0 {
		return
	}
	s.guidanceMu.Lock()
	defer s.guidanceMu.Unlock()
	if s.guidance == nil {
		s.guidance = make(map[*activeRoomSlot]pendingRoomGuidance)
	}
	s.guidance[slot] = pendingRoomGuidance{location: location, items: slices.Clone(items)}
}

func (e *roomGuidanceExecution) loadInputs() (bool, error) {
	queueItems, err := e.service.inputQueue.SnapshotGuidance(e.location, e.slot.AgentRoundID)
	if err != nil {
		return false, err
	}
	e.queueItems = queueItems
	return len(e.queueItems) > 0, nil
}

func (s *RealtimeService) acknowledgeRoomSlotGuidance(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) error {
	if slot == nil {
		return nil
	}
	s.guidanceMu.Lock()
	pending, ok := s.guidance[slot]
	if !ok {
		s.guidanceMu.Unlock()
		return nil
	}
	claimed, _, err := s.inputQueue.DispatchPreparedGuidance(pending.location, pending.items, slot.AgentRoundID)
	if err != nil {
		s.guidanceMu.Unlock()
		return err
	}
	if len(claimed) != len(pending.items) {
		delete(s.guidance, slot)
		s.guidanceMu.Unlock()
		return nil
	}
	if roundValue != nil && roundValue.Context != nil {
		rootRoundID := roomRootRoundID(roundValue)
		for _, item := range claimed {
			if err = s.syncQueuedPublicMessageDeliveryPolicy(ctx, roundValue.SessionKey, roundValue.Context, item, rootRoundID); err != nil {
				restored, restoreErr := s.restoreRoomSlotGuidance(pending.location, claimed)
				if restoreErr == nil {
					pending.items = restored
					s.guidance[slot] = pending
				}
				s.guidanceMu.Unlock()
				return errors.Join(err, restoreErr)
			}
		}
	}
	delete(s.guidance, slot)
	s.guidanceMu.Unlock()
	if roundValue != nil && roundValue.Context != nil {
		if err = s.broadcastRoomInputQueueSnapshot(ctx, roundValue.SessionKey, roundValue.Context); err != nil {
			s.loggerFor(ctx).Warn("广播 Room 引导队列消费快照失败",
				"session_key", roundValue.SessionKey,
				"room_id", roundValue.RoomID,
				"conversation_id", roundValue.ConversationID,
				"agent_id", slot.AgentID,
				"err", err,
			)
		}
	}
	return nil
}

func (s *RealtimeService) restoreRoomSlotGuidance(
	location workspacestore.InputQueueLocation,
	items []protocol.InputQueueItem,
) ([]protocol.InputQueueItem, error) {
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(items))
	for _, item := range items {
		entries = append(entries, workspacestore.InputQueueEnqueue{Location: location, Item: item})
	}
	return s.inputQueue.EnqueueBatchWithItems(entries)
}

func (s *RealtimeService) forgetRoomSlotGuidance(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	s.guidanceMu.Lock()
	delete(s.guidance, slot)
	s.guidanceMu.Unlock()
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
	e.inputs = make([]runtimectx.GuidedInput, 0, 1+len(e.queueItems))
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
	for _, item := range e.runtimeQueueItems {
		e.inputs = append(e.inputs, runtimectx.GuidedInput{
			RoundID: "queue_" + strings.TrimSpace(item.ID),
			Content: item.Content,
		})
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

func latestGuidanceTrigger(queueItems []protocol.InputQueueItem) (string, string) {
	roundID := ""
	content := ""
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
