package room

import (
	"context"
	"sort"
	"strings"
	"sync"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type activeRoomSlot struct {
	RoomSessionID     string
	SDKSessionID      string
	AgentID           string
	AgentRoundID      string
	MsgID             string
	RuntimeSessionKey string
	WorkspacePath     string
	Client            runtimectx.Client
	Cancel            context.CancelFunc
	Status            string
	Index             int
	TimestampMS       int64
	Trigger           roomTrigger
	InterruptReason   string
	QueuedInputs      []roomQueuedInput
	GuidedInputs      []roomQueuedInput
	SuppressOutput    bool
	NoReplyCandidate  bool
	PendingStream     []protocol.EventMessage
	Done              chan struct{}
	inputMu           sync.Mutex
	doneOnce          sync.Once
}

type activeRoomRound struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	RoomType       string
	Context        *protocol.ConversationContextAggregate
	RoundID        string
	RootRoundID    string
	HopIndex       int
	OwnerUserID    string
	Cancel         context.CancelFunc
	Slots          map[string]*activeRoomSlot
	PublicMentions []publicMentionWake
	Done           chan struct{}
	doneOnce       sync.Once
}

type roomTrigger struct {
	TriggerType   string
	Content       string
	MessageID     string
	SourceAgentID string
	TargetAgentID string
	Metadata      map[string]any
}

type publicMentionWake struct {
	SourceAgentID string
	TargetAgentID string
	Content       string
	MessageID     string
}

type roomQueuedInput struct {
	RoundID string
	Content string
}

type roomRoundMapperAdapter struct {
	mapper *slotMessageMapper
}

func (a roomRoundMapperAdapter) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) (runtimectx.RoundMapResult, error) {
	events, messages, terminalStatus, err := a.mapper.Map(incoming, interruptReason...)
	if err != nil {
		return runtimectx.RoundMapResult{}, err
	}
	return runtimectx.RoundMapResult{
		Events:          events,
		DurableMessages: messages,
		TerminalStatus:  terminalStatus,
	}, nil
}

func (a roomRoundMapperAdapter) SessionID() string {
	return a.mapper.SessionID()
}

// ActiveRoundSnapshot 表示 Room 当前仍在执行的主轮次快照。
type ActiveRoundSnapshot struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	RoundID        string
	Pending        []map[string]any
}

// CountRunningTasks 返回指定 Agent 当前在 Room 中的活跃任务数。
func (s *RealtimeService) CountRunningTasks(agentID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, roundValue := range s.activeRounds {
		for _, slot := range roundValue.Slots {
			if slot.AgentID == agentID && slot.Status != "finished" && slot.Status != "error" && slot.Status != "cancelled" {
				count++
			}
		}
	}
	return count
}

// GetActiveRoundSnapshot 返回指定 conversation 的活跃 slot 快照。
func (s *RealtimeService) GetActiveRoundSnapshot(conversationID string) *ActiveRoundSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	pending := make([]map[string]any, 0)
	snapshot := &ActiveRoundSnapshot{}
	for _, roundValue := range s.activeRounds {
		if roundValue == nil || roundValue.ConversationID != conversationID {
			continue
		}
		if snapshot.SessionKey == "" {
			snapshot.SessionKey = roundValue.SessionKey
			snapshot.RoomID = roundValue.RoomID
			snapshot.ConversationID = roundValue.ConversationID
			snapshot.RoundID = roundValue.RoundID
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || slot.Status == "finished" || slot.Status == "error" || slot.Status == "cancelled" {
				continue
			}
			status := slot.Status
			if status == "running" {
				status = "streaming"
			}
			pending = append(pending, map[string]any{
				"agent_id":  slot.AgentID,
				"msg_id":    slot.MsgID,
				"round_id":  slot.AgentRoundID,
				"status":    status,
				"timestamp": slot.TimestampMS,
				"index":     slot.Index,
			})
		}
	}
	if len(pending) == 0 {
		return nil
	}
	sort.Slice(pending, func(i int, j int) bool {
		leftTime := normalizeInt64(pending[i]["timestamp"])
		rightTime := normalizeInt64(pending[j]["timestamp"])
		if leftTime != rightTime {
			return leftTime < rightTime
		}
		return intValue(pending[i]["index"]) < intValue(pending[j]["index"])
	})
	for _, item := range pending {
		delete(item, "index")
	}
	snapshot.Pending = pending
	return snapshot
}

func (s *RealtimeService) registerRound(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	s.mu.Lock()
	s.activeRounds[roomActiveRoundKey(roundValue.SessionKey, roundValue.RoundID)] = roundValue
	s.mu.Unlock()
}

func (s *RealtimeService) finishRound(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	s.runtime.MarkRoundFinished(roundValue.SessionKey, roundValue.RoundID)
	s.mu.Lock()
	delete(s.activeRounds, roomActiveRoundKey(roundValue.SessionKey, roundValue.RoundID))
	s.mu.Unlock()
	roundValue.doneOnce.Do(func() {
		close(roundValue.Done)
	})
}

func roomRootRoundID(roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	if strings.TrimSpace(roundValue.RootRoundID) != "" {
		return strings.TrimSpace(roundValue.RootRoundID)
	}
	return strings.TrimSpace(roundValue.RoundID)
}

func roomActiveRoundKey(sessionKey string, roundID string) string {
	return strings.TrimSpace(sessionKey) + "::" + strings.TrimSpace(roundID)
}

func (s *RealtimeService) finishSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.doneOnce.Do(func() {
		close(slot.Done)
	})
}

func (slot *activeRoomSlot) enqueueQueuedInput(roundID string, content string) {
	if slot == nil || strings.TrimSpace(content) == "" {
		return
	}
	slot.inputMu.Lock()
	defer slot.inputMu.Unlock()
	slot.QueuedInputs = append(slot.QueuedInputs, roomQueuedInput{
		RoundID: strings.TrimSpace(roundID),
		Content: strings.TrimSpace(content),
	})
}

func (slot *activeRoomSlot) drainQueuedInputs() []roomQueuedInput {
	if slot == nil {
		return nil
	}
	slot.inputMu.Lock()
	defer slot.inputMu.Unlock()
	if len(slot.QueuedInputs) == 0 {
		return nil
	}
	inputs := append([]roomQueuedInput(nil), slot.QueuedInputs...)
	slot.QueuedInputs = nil
	return inputs
}

func (slot *activeRoomSlot) enqueueGuidedInput(roundID string, content string) {
	if slot == nil || strings.TrimSpace(content) == "" {
		return
	}
	slot.inputMu.Lock()
	defer slot.inputMu.Unlock()
	slot.GuidedInputs = append(slot.GuidedInputs, roomQueuedInput{
		RoundID: strings.TrimSpace(roundID),
		Content: strings.TrimSpace(content),
	})
}

func (slot *activeRoomSlot) drainGuidedInputs() []roomQueuedInput {
	if slot == nil {
		return nil
	}
	slot.inputMu.Lock()
	defer slot.inputMu.Unlock()
	if len(slot.GuidedInputs) == 0 {
		return nil
	}
	inputs := append([]roomQueuedInput(nil), slot.GuidedInputs...)
	slot.GuidedInputs = nil
	return inputs
}

func normalizeRoomInterruptReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return reason
	}
	return "请求已停止"
}

func markRoomSlotInterrupted(slot *activeRoomSlot, reason string) {
	if slot == nil {
		return
	}
	slot.InterruptReason = normalizeRoomInterruptReason(reason)
}

func roomSlotInterruptReason(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	return strings.TrimSpace(slot.InterruptReason)
}

func (r *activeRoomRound) allSlotsCancelled() bool {
	if len(r.Slots) == 0 {
		return false
	}
	for _, slot := range r.Slots {
		if slot.Status != "cancelled" {
			return false
		}
	}
	return true
}

func (r *activeRoomRound) hasSlotError() bool {
	if r == nil {
		return false
	}
	for _, slot := range r.Slots {
		if slot != nil && slot.Status == "error" {
			return true
		}
	}
	return false
}
