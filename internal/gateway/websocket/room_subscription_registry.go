package websocket

import (
	"context"
	"sort"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type roomEventSender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

type roomSubscription struct {
	Sender         roomEventSender
	ConversationID string
}

// roomSubscriptionRegistry 负责 Room 广播订阅、durable 回放与 resync。
type roomSubscriptionRegistry struct {
	mu              sync.Mutex
	roomSubs        map[string]map[string]roomSubscription
	senderRooms     map[string]map[string]struct{}
	roomSequences   map[string]int64
	roomReplay      map[string][]protocol.EventMessage
	replayBufferCap int
}

func newRoomSubscriptionRegistry(bufferCap int) *roomSubscriptionRegistry {
	if bufferCap <= 0 {
		bufferCap = 128
	}
	return &roomSubscriptionRegistry{
		roomSubs:        make(map[string]map[string]roomSubscription),
		senderRooms:     make(map[string]map[string]struct{}),
		roomSequences:   make(map[string]int64),
		roomReplay:      make(map[string][]protocol.EventMessage),
		replayBufferCap: bufferCap,
	}
}

func (r *roomSubscriptionRegistry) SubscribeRoom(
	ctx context.Context,
	sender roomEventSender,
	roomID string,
	conversationID string,
	lastSeenRoomSeq *int64,
) error {
	if sender == nil || sender.IsClosed() || roomID == "" {
		return nil
	}

	r.mu.Lock()
	roomSubscribers := r.roomSubs[roomID]
	if roomSubscribers == nil {
		roomSubscribers = make(map[string]roomSubscription)
		r.roomSubs[roomID] = roomSubscribers
	}
	roomSubscribers[sender.Key()] = roomSubscription{
		Sender:         sender,
		ConversationID: conversationID,
	}
	senderRooms := r.senderRooms[sender.Key()]
	if senderRooms == nil {
		senderRooms = make(map[string]struct{})
		r.senderRooms[sender.Key()] = senderRooms
	}
	senderRooms[roomID] = struct{}{}

	if lastSeenRoomSeq == nil || *lastSeenRoomSeq <= 0 {
		r.mu.Unlock()
		return nil
	}

	latestRoomSeq := r.roomSequences[roomID]
	buffer := append([]protocol.EventMessage(nil), r.roomReplay[roomID]...)
	r.mu.Unlock()

	return r.replayRoomEvents(ctx, sender, roomID, conversationID, *lastSeenRoomSeq, latestRoomSeq, buffer)
}

func (r *roomSubscriptionRegistry) UnsubscribeRoom(sender roomEventSender, roomID string) {
	if sender == nil || roomID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	roomSubscribers := r.roomSubs[roomID]
	if len(roomSubscribers) == 0 {
		return
	}
	delete(roomSubscribers, sender.Key())
	if len(roomSubscribers) == 0 {
		delete(r.roomSubs, roomID)
	}

	senderRooms := r.senderRooms[sender.Key()]
	if len(senderRooms) == 0 {
		return
	}
	delete(senderRooms, roomID)
	if len(senderRooms) == 0 {
		delete(r.senderRooms, sender.Key())
	}
}

func (r *roomSubscriptionRegistry) UnregisterSender(sender roomEventSender) {
	if sender == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	senderRooms := r.senderRooms[sender.Key()]
	for roomID := range senderRooms {
		roomSubscribers := r.roomSubs[roomID]
		delete(roomSubscribers, sender.Key())
		if len(roomSubscribers) == 0 {
			delete(r.roomSubs, roomID)
		}
	}
	delete(r.senderRooms, sender.Key())
}

func (r *roomSubscriptionRegistry) RemoveRoom(roomID string) {
	if roomID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	roomSubscribers := r.roomSubs[roomID]
	for senderKey := range roomSubscribers {
		senderRooms := r.senderRooms[senderKey]
		delete(senderRooms, roomID)
		if len(senderRooms) == 0 {
			delete(r.senderRooms, senderKey)
		}
	}
	delete(r.roomSubs, roomID)
	delete(r.roomSequences, roomID)
	delete(r.roomReplay, roomID)
}

func (r *roomSubscriptionRegistry) CurrentRoomSeq(roomID string) int64 {
	if roomID == "" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.roomSequences[roomID]
}

func (r *roomSubscriptionRegistry) Broadcast(ctx context.Context, roomID string, event protocol.EventMessage) []error {
	if roomID == "" {
		return nil
	}

	r.mu.Lock()
	prepared := r.prepareRoomEventLocked(roomID, event)
	roomSubscribers := append([]roomSubscription(nil), r.matchingSubscribersLocked(roomID, prepared.ConversationID)...)
	r.mu.Unlock()

	if len(roomSubscribers) == 0 {
		return nil
	}

	errs := make([]error, 0)
	for _, subscription := range roomSubscribers {
		if subscription.Sender == nil || subscription.Sender.IsClosed() {
			continue
		}
		if err := subscription.Sender.SendEvent(ctx, prepared); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (r *roomSubscriptionRegistry) replayRoomEvents(
	ctx context.Context,
	sender roomEventSender,
	roomID string,
	conversationID string,
	lastSeenRoomSeq int64,
	latestRoomSeq int64,
	buffer []protocol.EventMessage,
) error {
	if latestRoomSeq <= lastSeenRoomSeq {
		return nil
	}
	if len(buffer) == 0 {
		return sender.SendEvent(ctx, r.newRoomResyncRequiredEvent(roomID, conversationID, lastSeenRoomSeq, latestRoomSeq, nil))
	}

	earliestRoomSeq := int64(0)
	if buffer[0].RoomSeq != nil {
		earliestRoomSeq = *buffer[0].RoomSeq
	}
	if earliestRoomSeq == 0 {
		return nil
	}
	if lastSeenRoomSeq < earliestRoomSeq-1 {
		return sender.SendEvent(ctx, r.newRoomResyncRequiredEvent(roomID, conversationID, lastSeenRoomSeq, latestRoomSeq, &earliestRoomSeq))
	}

	for _, event := range buffer {
		if event.RoomSeq == nil || *event.RoomSeq <= lastSeenRoomSeq {
			continue
		}
		if !conversationMatches(conversationID, event.ConversationID) {
			continue
		}
		if err := sender.SendEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (r *roomSubscriptionRegistry) prepareRoomEventLocked(roomID string, event protocol.EventMessage) protocol.EventMessage {
	prepared := event
	if prepared.RoomID == "" {
		prepared.RoomID = roomID
	}
	if prepared.DeliveryMode != "durable" || prepared.RoomSeq != nil {
		return prepared
	}

	nextRoomSeq := r.roomSequences[roomID] + 1
	r.roomSequences[roomID] = nextRoomSeq
	prepared.RoomSeq = &nextRoomSeq

	buffer := append(r.roomReplay[roomID], prepared)
	if len(buffer) > r.replayBufferCap {
		buffer = append([]protocol.EventMessage(nil), buffer[len(buffer)-r.replayBufferCap:]...)
	}
	r.roomReplay[roomID] = buffer
	return prepared
}

func (r *roomSubscriptionRegistry) matchingSubscribersLocked(roomID string, conversationID string) []roomSubscription {
	roomSubscribers := r.roomSubs[roomID]
	if len(roomSubscribers) == 0 {
		return nil
	}

	keys := make([]string, 0, len(roomSubscribers))
	for senderKey := range roomSubscribers {
		keys = append(keys, senderKey)
	}
	sort.Strings(keys)

	result := make([]roomSubscription, 0, len(keys))
	for _, senderKey := range keys {
		subscription := roomSubscribers[senderKey]
		if subscription.Sender == nil || subscription.Sender.IsClosed() {
			delete(roomSubscribers, senderKey)
			continue
		}
		if !conversationMatches(subscription.ConversationID, conversationID) {
			continue
		}
		result = append(result, subscription)
	}
	if len(roomSubscribers) == 0 {
		delete(r.roomSubs, roomID)
	}
	return result
}

func (r *roomSubscriptionRegistry) newRoomResyncRequiredEvent(
	roomID string,
	conversationID string,
	lastSeenRoomSeq int64,
	latestRoomSeq int64,
	bufferStartRoomSeq *int64,
) protocol.EventMessage {
	data := map[string]any{
		"room_id":            roomID,
		"conversation_id":    conversationID,
		"last_seen_room_seq": lastSeenRoomSeq,
		"latest_room_seq":    latestRoomSeq,
	}
	if bufferStartRoomSeq != nil {
		data["buffer_start_room_seq"] = *bufferStartRoomSeq
	} else {
		data["buffer_start_room_seq"] = nil
	}
	event := protocol.NewEvent(protocol.EventTypeRoomResyncRequired, data)
	event.RoomID = roomID
	event.ConversationID = conversationID
	return event
}

func conversationMatches(subscribedConversationID string, eventConversationID string) bool {
	if subscribedConversationID == "" || eventConversationID == "" {
		return true
	}
	return subscribedConversationID == eventConversationID
}
