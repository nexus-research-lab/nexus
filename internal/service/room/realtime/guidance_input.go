// INPUT: 运行中 Room slot 与保留来源语义的持久化 guide 队列。
// OUTPUT: 原子预留的 PostToolUse additionalContext，并在 applied ACK 后归入实际回复 round。
// POS: Room 轮内插话的唯一消费入口；预留与队列编辑共用派发锁。
package realtime

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) roomSlotGuidanceHook(
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
		if s.shouldConfirmRoomGuidanceByFallback(slot) {
			if err := s.acknowledgeRoomSlotGuidance(ctx, roundValue, slot, nil); err != nil {
				return sdkhook.Output{}, err
			}
		}
		if roundValue != nil {
			s.rounds.bindSlot(slot, roundValue)
		} else {
			s.rounds.bindSlotToConversation(slot, location.ConversationID)
		}
		if s.hasPendingRoomSlotGuidance(slot) {
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

func (s *Service) shouldConfirmRoomGuidanceByFallback(slot *activeRoomSlot) bool {
	return s == nil || s.runtime == nil || slot == nil ||
		!s.runtime.SupportsHookResponseAck(slot.RuntimeSessionKey)
}

type roomGuidanceExecution struct {
	service           *Service
	ctx               context.Context
	round             *activeRoomRound
	slot              *activeRoomSlot
	location          workspacestore.InputQueueLocation
	queueItems        []protocol.InputQueueItem
	runtimeQueueItems []protocol.InputQueueItem
	sourceRoundID     string
	trigger           roomTrigger
	inputs            []runtimectx.GuidedInput
}

// ponytail: 旧 runtime 没有 applied ACK；下一次 hook/result 仍作为兼容确认，进程崩溃窗口允许安全重投。
type pendingRoomGuidance struct {
	location workspacestore.InputQueueLocation
	items    []protocol.InputQueueItem
}

func (e *roomGuidanceExecution) run() (sdkhook.Output, error) {
	if e.service == nil {
		return sdkhook.Output{}, nil
	}
	if err := e.bindOwnerContext(); err != nil {
		return sdkhook.Output{}, err
	}
	sessionKey := e.location.SessionKey
	conversationID := e.location.ConversationID
	if e.round != nil {
		sessionKey = e.round.SessionKey
		conversationID = e.round.ConversationID
	}
	lease := e.service.lockRoomDispatch(sessionKey, conversationID)
	var output sdkhook.Output
	var runErr error
	func() {
		defer lease.Unlock()
		hasInput, err := e.loadInputs()
		if err != nil || !hasInput {
			runErr = err
			return
		}
		if err = e.renderQueueItems(); err != nil {
			runErr = err
			return
		}
		e.sourceRoundID, e.trigger = latestGuidanceTrigger(e.runtimeQueueItems)
		if err = e.buildInputs(); err != nil {
			runErr = err
			return
		}
		pending := e.service.rememberRoomSlotGuidance(e.slot, e.location, e.queueItems)
		output = sdkhook.Output{
			SpecificOutput: &sdkhook.SpecificOutput{
				HookEventName:     sdkhook.EventPostToolUse,
				AdditionalContext: runtimectx.FormatGuidanceAdditionalContext(e.inputs),
			},
			OnApplied: func(sdkhook.AppliedAck) {
				ownerUserID := ""
				if e.round != nil {
					ownerUserID = e.round.OwnerUserID
				}
				ctx := contextWithExactQueueOwner(context.Background(), ownerUserID)
				if ackErr := e.service.acknowledgeRoomSlotGuidance(ctx, e.round, e.slot, &pending); ackErr != nil {
					e.service.loggerFor(ctx).Warn("确认 Room 引导 applied ACK 失败，保留为后续队列输入", "err", ackErr)
				}
			},
		}
	}()
	return output, runErr
}

func (e *roomGuidanceExecution) bindOwnerContext() error {
	ownerUserID := strings.TrimSpace(e.location.OwnerUserID)
	for _, candidate := range []string{
		roomRoundOwnerUserID(e.round),
		roomSlotOwnerUserID(e.slot),
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if ownerUserID != "" && ownerUserID != candidate {
			return errors.New("Room guidance owner does not match queue location")
		}
		ownerUserID = candidate
	}
	e.ctx = contextWithExactQueueOwner(e.ctx, ownerUserID)
	return nil
}

func roomRoundOwnerUserID(roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	return roundValue.OwnerUserID
}

func roomSlotOwnerUserID(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	return slot.OwnerUserID
}

func (s *Service) rememberRoomSlotGuidance(
	slot *activeRoomSlot,
	location workspacestore.InputQueueLocation,
	items []protocol.InputQueueItem,
) pendingRoomGuidance {
	if slot == nil || len(items) == 0 {
		return pendingRoomGuidance{}
	}
	s.rounds.bindSlotToConversation(slot, location.ConversationID)
	pending := pendingRoomGuidance{location: location, items: slices.Clone(items)}
	s.rounds.putGuidance(slot, pending)
	return pending
}

func (s *Service) hasPendingRoomSlotGuidance(slot *activeRoomSlot) bool {
	return s.rounds.hasGuidance(slot)
}

func (s *Service) hasInFlightRoomGuidance(itemID string) bool {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return false
	}
	for _, pending := range s.rounds.guidanceSnapshot() {
		if slices.ContainsFunc(pending.items, func(item protocol.InputQueueItem) bool { return item.ID == itemID }) {
			return true
		}
	}
	return false
}

func (e *roomGuidanceExecution) loadInputs() (bool, error) {
	queueItems, err := e.service.inputQueue.SnapshotGuidance(e.location, e.slot.AgentRoundID)
	if err != nil {
		return false, err
	}
	e.queueItems = queueItems
	return len(e.queueItems) > 0, nil
}

func (s *Service) acknowledgeRoomSlotGuidance(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	expected *pendingRoomGuidance,
) error {
	if slot == nil {
		return nil
	}
	sessionKey, conversationID := roomSlotDispatchContext(roundValue, slot)
	lease := s.lockRoomDispatch(sessionKey, conversationID)
	defer lease.Unlock()
	return s.acknowledgeRoomSlotGuidanceLocked(ctx, roundValue, slot, expected)
}

// acknowledgeRoomSlotGuidanceLocked 在 conversation 派发闸门内完成 durable ACK。
func (s *Service) acknowledgeRoomSlotGuidanceLocked(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	expected *pendingRoomGuidance,
) error {
	if slot == nil {
		return nil
	}
	pending, ok := s.rounds.loadGuidance(slot)
	if !ok {
		return nil
	}
	if expected != nil && !reflect.DeepEqual(pending, *expected) {
		return nil
	}
	claimed, _, err := s.inputQueue.DispatchPreparedGuidance(pending.location, pending.items, slot.AgentRoundID)
	if err != nil {
		return err
	}
	if len(claimed) != len(pending.items) {
		s.rounds.deleteGuidance(slot)
		return nil
	}
	if roundValue != nil && roundValue.Context != nil {
		rootRoundID := roomRootRoundID(roundValue)
		for _, item := range claimed {
			logicalRootRoundID := s.logicalPublicHandoffRootRoundID(
				roundValue.ConversationID,
				item,
				rootRoundID,
			)
			if err = s.syncQueuedPublicUserMessage(ctx, roundValue.SessionKey, roundValue.Context, item, logicalRootRoundID, true); err != nil {
				restored, restoreErr := s.restoreRoomSlotGuidance(pending.location, claimed)
				if restoreErr == nil {
					pending.items = restored
					s.rounds.putGuidance(slot, pending)
				}
				return errors.Join(err, restoreErr)
			}
		}
		for _, item := range claimed {
			if protocol.NormalizeGoalCollaborationBinding(item.GoalCollaborationBinding) != nil {
				restored, restoreErr := s.restoreRoomSlotGuidance(pending.location, claimed)
				if restoreErr == nil {
					pending.items = restored
					s.rounds.putGuidance(slot, pending)
				}
				return errors.Join(
					errors.New("Goal collaboration handoff cannot be acknowledged as ordinary Room guidance"),
					restoreErr,
				)
			}
			if err = s.markRoomQueueHandoffTerminal(roundValue.ConversationID, item); err != nil {
				restored, restoreErr := s.restoreRoomSlotGuidance(pending.location, claimed)
				if restoreErr == nil {
					pending.items = restored
					s.rounds.putGuidance(slot, pending)
				}
				return errors.Join(err, restoreErr)
			}
		}
	}
	s.rounds.deleteGuidance(slot)
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

func (s *Service) restoreRoomSlotGuidance(
	location workspacestore.InputQueueLocation,
	items []protocol.InputQueueItem,
) ([]protocol.InputQueueItem, error) {
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(items))
	for _, item := range items {
		entries = append(entries, workspacestore.InputQueueEnqueue{Location: location, Item: item})
	}
	return s.inputQueue.EnqueueBatchWithItems(entries)
}

func (s *Service) forgetRoomSlotGuidance(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	sessionKey, conversationID := roomSlotDispatchContext(nil, slot)
	lease := s.lockRoomDispatch(sessionKey, conversationID)
	defer lease.Unlock()
	s.rounds.deleteGuidance(slot)
}

func roomSlotDispatchContext(roundValue *activeRoomRound, slot *activeRoomSlot) (string, string) {
	if roundValue != nil {
		return roundValue.SessionKey, roundValue.ConversationID
	}
	conversationID, _ := slot.conversationBinding()
	if conversationID == "" {
		conversationID = roomConversationIDFromSessionKey(slot.RuntimeSessionKey)
	}
	return slot.RuntimeSessionKey, conversationID
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
	publicHistory, err := e.service.roomHistory.ReadMessages(
		e.round.OwnerUserID,
		e.round.ConversationID,
		nil,
	)
	if err != nil {
		return err
	}
	trigger := e.trigger
	if strings.TrimSpace(trigger.TriggerType) == "" {
		trigger.TriggerType = "public_chat"
	}
	if strings.TrimSpace(trigger.MessageID) == "" {
		trigger.MessageID = strings.TrimSpace(e.sourceRoundID)
	}
	trigger.TargetAgentID = e.slot.AgentID
	publicContext, err := e.service.buildSlotGuidedPublicContext(
		e.ctx,
		e.round,
		e.slot,
		publicHistory,
		agentNameByID,
		trigger,
	)
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

func latestGuidanceTrigger(queueItems []protocol.InputQueueItem) (string, roomTrigger) {
	roundID := ""
	trigger := roomTrigger{}
	for _, item := range queueItems {
		if itemID := strings.TrimSpace(item.ID); itemID != "" {
			roundID = "queue_" + itemID
		}
		if content := strings.TrimSpace(item.Content); content != "" {
			trigger = roomTrigger{
				TriggerType:   guidanceTriggerType(item.Source),
				Content:       content,
				MessageID:     firstNonEmptyString(item.SourceMessageID, roundID),
				SourceAgentID: strings.TrimSpace(item.SourceAgentID),
				TargetAgentID: strings.TrimSpace(item.AgentID),
				ReplyRoute:    item.ReplyRoute,
			}
		}
	}
	return roundID, trigger
}

func guidanceTriggerType(source protocol.InputQueueSource) string {
	switch source {
	case protocol.InputQueueSourceAgentPublicMention:
		return "public_mention"
	case protocol.InputQueueSourceAgentRoomMessage:
		return "room_directed_message"
	default:
		return "public_chat"
	}
}
