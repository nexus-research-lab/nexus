package realtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type authorityFenceRoomStore struct {
	mu                  sync.RWMutex
	contextValue        *protocol.ConversationContextAggregate
	respectCancellation bool
}

func (s *authorityFenceRoomStore) GetConversationContext(
	ctx context.Context,
	_ string,
) (*protocol.ConversationContextAggregate, error) {
	if s.respectCancellation {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return s.snapshot(), nil
}

func (s *authorityFenceRoomStore) GetConversationContextForSystem(
	ctx context.Context,
	_ string,
) (*protocol.ConversationContextAggregate, error) {
	if s.respectCancellation {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return s.snapshot(), nil
}

func (*authorityFenceRoomStore) UpdateSessionRuntimeIdentity(context.Context, string, string, string) error {
	return nil
}

func (*authorityFenceRoomStore) TouchConversationActivity(context.Context, string, time.Time) error {
	return nil
}

func (*authorityFenceRoomStore) MarkConversationStarted(context.Context, string, time.Time) error {
	return nil
}

func (*authorityFenceRoomStore) BuildRoomSkillPrompt(context.Context, []string) (string, error) {
	return "", nil
}

func (s *authorityFenceRoomStore) snapshot() *protocol.ConversationContextAggregate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAuthorityFenceContext(s.contextValue)
}

func (s *authorityFenceRoomStore) update(
	update func(*protocol.ConversationContextAggregate),
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update(s.contextValue)
}

func cloneAuthorityFenceContext(
	contextValue *protocol.ConversationContextAggregate,
) *protocol.ConversationContextAggregate {
	if contextValue == nil {
		return nil
	}
	cloned := *contextValue
	cloned.Room.SkillNames = append([]string(nil), contextValue.Room.SkillNames...)
	cloned.Members = append([]protocol.MemberRecord(nil), contextValue.Members...)
	cloned.MemberAgents = append([]protocol.Agent(nil), contextValue.MemberAgents...)
	cloned.Sessions = append([]protocol.SessionRecord(nil), contextValue.Sessions...)
	return &cloned
}

func newAuthorityFenceContext() *protocol.ConversationContextAggregate {
	return &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID:                     "room-a",
			RoomType:               protocol.RoomTypeGroup,
			PrivateMessagesEnabled: true,
			AuthorityEpoch:         1,
		},
		Conversation: protocol.ConversationRecord{ID: "conversation-a", RoomID: "room-a"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-a"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-b"},
		},
	}
}

func newAuthorityFenceRound(
	contextValue *protocol.ConversationContextAggregate,
) *activeRoomRound {
	return &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey("conversation-a"),
		RoomID:         "room-a",
		ConversationID: "conversation-a",
		Context:        cloneAuthorityFenceContext(contextValue),
		AuthorityEpoch: contextValue.Room.AuthorityEpoch,
		RootRoundID:    "round-a",
		RoundID:        "round-a",
	}
}

type authorityFenceBroadcaster struct {
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (b *authorityFenceBroadcaster) Broadcast(
	context.Context,
	string,
	protocol.EventMessage,
) []error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, protocol.EventMessage{})
	return nil
}

func (b *authorityFenceBroadcaster) eventCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

func TestRoomSlotOutputFenceRechecksMembership(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	service := &Service{rooms: store}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{AgentID: "agent-a"}

	if err := service.ensureSlotOutputAuthorized(t.Context(), roundValue, slot); err != nil {
		t.Fatalf("current member output was rejected: %v", err)
	}
	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Members = contextValue.Members[1:]
		contextValue.Room.AuthorityEpoch++
	})
	if err := service.ensureSlotOutputAuthorized(t.Context(), roundValue, slot); !errors.Is(err, errRoomSlotAuthorityRevoked) {
		t.Fatalf("removed member output was not fenced: %v", err)
	}
}

func TestRoomSlotOutputFenceRejectsPausedMember(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	service := &Service{rooms: store}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{AgentID: "agent-a"}

	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Members[0].ParticipationPaused = true
	})
	if err := service.ensureSlotOutputAuthorized(t.Context(), roundValue, slot); !errors.Is(err, errRoomSlotAuthorityRevoked) {
		t.Fatalf("paused member output was not fenced: %v", err)
	}
}

func TestRoomSlotOutputFenceRechecksWithCancelledRuntimeContext(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{
		contextValue:        initialContext,
		respectCancellation: true,
	}
	service := &Service{rooms: store}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{AgentID: "agent-a"}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := service.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		t.Fatalf("cancelled runtime context prevented authoritative recheck: %v", err)
	}
}

func TestRoomSlotOutputFenceRejectsAuthorityEpochChange(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	service := &Service{rooms: store}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{AgentID: "agent-a"}

	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		// 模拟 host transfer：成员仍在，但旧 round 的授权世代已经失效。
		contextValue.Room.AuthorityEpoch++
	})
	if err := service.ensureSlotOutputAuthorized(t.Context(), roundValue, slot); !errors.Is(err, errRoomSlotAuthorityRevoked) {
		t.Fatalf("stale authority epoch was not fenced: %v", err)
	}
}

func TestRoomSlotOutputFenceRejectsPrivateReplyAfterDisable(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	service := &Service{rooms: store}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{AgentID: "agent-a"}
	slot.setDeliveryMetadata(protocol.RoomReplyRoute{
		Mode:       protocol.RoomReplyRoutePrivate,
		Recipients: []string{"agent-b"},
	}, "directed-source", "")

	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Room.PrivateMessagesEnabled = false
	})
	if err := service.ensureSlotOutputAuthorized(t.Context(), roundValue, slot); !errors.Is(err, errRoomSlotAuthorityRevoked) {
		t.Fatalf("disabled private reply was not fenced: %v", err)
	}
}

func TestRoomDirectedReplyDropsAfterAuthorityRevocation(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	root := t.TempDir()
	broadcaster := &authorityFenceBroadcaster{}
	service := &Service{
		rooms:            store,
		directedMessages: workspacestore.NewRoomDirectedMessageStore(root),
		broadcaster:      broadcaster,
	}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{AgentID: "agent-a"}
	slot.setDeliveryMetadata(protocol.RoomReplyRoute{
		Mode:       protocol.RoomReplyRoutePrivate,
		Recipients: []string{"agent-b"},
	}, "directed-source", "")
	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Members = contextValue.Members[1:]
		contextValue.Room.AuthorityEpoch++
	})

	err := service.recordRoomDirectedMessageReply(t.Context(), roundValue, slot, protocol.Message{
		"message_id":  "assistant-private",
		"role":        "assistant",
		"content":     "revoked private reply",
		"is_complete": true,
	})
	if !errors.Is(err, errRoomSlotAuthorityRevoked) {
		t.Fatalf("revoked directed reply returned err=%v", err)
	}
	messages, readErr := service.directedMessages.ReadContextMessages(
		authctx.SystemUserID,
		"conversation-a",
		"agent-b",
	)
	if readErr != nil {
		t.Fatalf("read directed messages: %v", readErr)
	}
	if len(messages) != 0 {
		t.Fatalf("revoked directed reply was persisted: %+v", messages)
	}
	if broadcaster.eventCount() != 0 {
		t.Fatalf("revoked directed reply emitted %d events", broadcaster.eventCount())
	}
}

func TestRoomCompletionAdmissionPrecedesGoalMutation(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{
		rooms:       store,
		goals:       goalProvider,
		broadcaster: &authorityFenceBroadcaster{},
	}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{AgentID: "agent-a", AgentRoundID: "agent-round-a"}
	slot.setStatus("running")
	slot.setGoalBinding(roundValue.SessionKey, "goal-a")
	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Room.AuthorityEpoch++
	})
	execution := &slotExecution{
		service: service,
		ctx:     t.Context(),
		round:   roundValue,
		slot:    slot,
		// mapper 故意为空：撤权闸门必须在读取 runtime 最终快照前返回。
	}

	err := execution.complete(exec.RoundExecutionResult{TerminalStatus: "finished"})
	if !errors.Is(err, errRoomSlotAuthorityRevoked) {
		t.Fatalf("revoked completion returned err=%v", err)
	}
	assertAuthorityFenceGoalProviderUntouched(t, goalProvider)
}

func TestRoomFailureAfterRevocationIsSilentlyRetired(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	goalProvider := &fakeRoomGoalContextProvider{}
	root := t.TempDir()
	broadcaster := &authorityFenceBroadcaster{}
	service := &Service{
		rooms:       store,
		goals:       goalProvider,
		history:     workspacestore.NewAgentHistoryStore(root),
		roomHistory: workspacestore.NewRoomHistoryStore(root),
		broadcaster: broadcaster,
	}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{
		AgentID:           "agent-a",
		AgentRoundID:      "agent-round-a",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey("conversation-a", "agent-a", protocol.RoomTypeGroup),
		WorkspacePath:     root + "/agent-a",
	}
	slot.setStatus("running")
	slot.setGoalBinding(roundValue.SessionKey, "goal-a")
	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Members = contextValue.Members[1:]
		contextValue.Room.AuthorityEpoch++
	})

	service.handleSlotFailure(
		t.Context(),
		roundValue,
		slot,
		nil,
		exec.RoundExecutionResult{},
		errors.New("runtime failed after removal"),
	)

	if slot.getStatus() != "cancelled" || slot.getErrorMessage() != "" || !slot.shouldSuppressOutput() {
		t.Fatalf(
			"revoked slot was not silently retired: status=%q error=%q suppressed=%v",
			slot.getStatus(),
			slot.getErrorMessage(),
			slot.shouldSuppressOutput(),
		)
	}
	assertAuthorityFenceGoalProviderUntouched(t, goalProvider)
	assertAuthorityFenceHistoriesEmpty(t, service, roundValue, slot)
	if broadcaster.eventCount() != 0 {
		t.Fatalf("revoked failure emitted %d events", broadcaster.eventCount())
	}
}

func TestRoomIdleSubagentDropsDurableAndEventsAfterRevocation(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	root := t.TempDir()
	broadcaster := &authorityFenceBroadcaster{}
	service := &Service{
		rooms:       store,
		history:     workspacestore.NewAgentHistoryStore(root),
		roomHistory: workspacestore.NewRoomHistoryStore(root),
		broadcaster: broadcaster,
	}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{
		AgentID:           "agent-a",
		AgentRoundID:      "agent-round-a",
		MsgID:             "slot-message-a",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey("conversation-a", "agent-a", protocol.RoomTypeGroup),
		WorkspacePath:     root + "/agent-a",
	}
	incoming := authorityFenceAssistantMessage()
	probeMapper := newAuthorityFenceMapper(roundValue, slot)
	events, durableMessages, _, err := probeMapper.Map(incoming)
	if err != nil || len(events) == 0 || len(durableMessages) == 0 {
		t.Fatalf("idle fixture must contain event and durable output: events=%d durable=%d err=%v", len(events), len(durableMessages), err)
	}
	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Room.AuthorityEpoch++
	})

	if keepDraining := service.handleIdleSubagentMessage(
		t.Context(),
		roundValue,
		slot,
		newAuthorityFenceMapper(roundValue, slot),
		authorityFenceAssistantMessage(),
	); keepDraining {
		t.Fatal("revoked idle drain should stop")
	}
	assertAuthorityFenceHistoriesEmpty(t, service, roundValue, slot)
	if broadcaster.eventCount() != 0 {
		t.Fatalf("revoked idle output emitted %d events", broadcaster.eventCount())
	}
}

func TestRoomSlotOutputFenceConcurrentAuthorityEpochChange(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	service := &Service{rooms: store}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{AgentID: "agent-a"}

	start := make(chan struct{})
	errs := make(chan error, 8)
	var readers sync.WaitGroup
	for reader := 0; reader < 8; reader++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			<-start
			for attempt := 0; attempt < 500; attempt++ {
				err := service.ensureSlotOutputAuthorized(t.Context(), roundValue, slot)
				if err != nil && !errors.Is(err, errRoomSlotAuthorityRevoked) {
					errs <- fmt.Errorf("unexpected admission error: %w", err)
					return
				}
			}
		}()
	}
	close(start)
	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Room.AuthorityEpoch++
	})
	readers.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func newAuthorityFenceMapper(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) *roomdomain.SlotMessageMapper {
	return roomdomain.NewSlotMessageMapper(
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		roundValue.RootRoundID,
		slot.AgentRoundID,
		slot.WorkspacePath,
	)
}

func authorityFenceAssistantMessage() sdkprotocol.ReceivedMessage {
	return sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "idle-assistant-a",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "revoked idle output"},
				},
			},
		},
	}
}

func assertAuthorityFenceGoalProviderUntouched(
	t *testing.T,
	provider *fakeRoomGoalContextProvider,
) {
	t.Helper()
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.usage) != 0 ||
		len(provider.usageLimitReason) != 0 ||
		len(provider.progress) != 0 ||
		len(provider.failures) != 0 ||
		len(provider.completionMisses) != 0 ||
		len(provider.activities) != 0 ||
		len(provider.collabEvidence) != 0 {
		t.Fatalf("revoked output mutated Goal state: %+v", provider)
	}
}

func assertAuthorityFenceHistoriesEmpty(
	t *testing.T,
	service *Service,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) {
	t.Helper()
	sharedMessages, err := service.roomHistory.ReadMessages(
		authctx.SystemUserID,
		roundValue.ConversationID,
		nil,
	)
	if err != nil {
		t.Fatalf("read shared history: %v", err)
	}
	if len(sharedMessages) != 0 {
		t.Fatalf("revoked output reached shared history: %+v", sharedMessages)
	}
	privateMessages, err := service.history.ReadMessages(slot.WorkspacePath, protocol.Session{
		SessionKey: slot.RuntimeSessionKey,
		AgentID:    slot.AgentID,
	}, nil)
	if err != nil {
		t.Fatalf("read private history: %v", err)
	}
	if len(privateMessages) != 0 {
		t.Fatalf("revoked output reached private history: %+v", privateMessages)
	}
}
