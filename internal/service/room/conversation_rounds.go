package room

import (
	"strconv"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// roomConversationState 持有一个 conversation 的全部短生命周期编排状态。
// 不同 conversation 之间不共享这把锁，避免一个 Room 的扫描阻塞另一个 Room。
type roomConversationState struct {
	mu                   sync.RWMutex
	registrationSequence uint64
	rounds               map[string]*activeRoomRound
	roundKeys            map[*activeRoomRound]string
	guidance             map[*activeRoomSlot]pendingRoomGuidance
	publicMentions       map[*activeRoomRound][]publicMentionWake
}

// roomRoundRegistry 只保护 conversation state 索引；具体 round 数据由 shard 自己保护。
type roomRoundRegistry struct {
	mu            sync.RWMutex
	conversations map[string]*roomConversationState
}

func newRoomRoundRegistry() roomRoundRegistry {
	return roomRoundRegistry{conversations: make(map[string]*roomConversationState)}
}

func newRoomRoundRegistryFromRounds(rounds map[string]*activeRoomRound) roomRoundRegistry {
	registry := newRoomRoundRegistry()
	for _, roundValue := range rounds {
		registry.register(roundValue)
	}
	return registry
}

func roomConversationKey(conversationID string, sessionKey string) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID != "" {
		return conversationID
	}
	if conversationID = roomConversationIDFromSessionKey(sessionKey); conversationID != "" {
		return conversationID
	}
	return "__room_unknown_conversation__"
}

func roomConversationIDFromSessionKey(sessionKey string) string {
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.ConversationID != "" {
		return strings.TrimSpace(parsed.ConversationID)
	}
	// Room Agent runtime 使用 agent:<id>:...:<conversation_id>；解析器把
	// 末段放在 Ref 中，不能只依赖 shared room key 的 ConversationID 字段。
	if parsed.Kind == protocol.SessionKeyKindAgent && strings.EqualFold(parsed.ChatType, "group") {
		return strings.TrimSpace(parsed.Ref)
	}
	return ""
}

func roomRegistryRoundKey(roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	roundID := strings.TrimSpace(roundValue.RoundID)
	if roundID == "" {
		roundID = roomRootRoundID(roundValue)
	}
	if roundID == "" {
		return ""
	}
	return roomActiveRoundKey(roundValue.SessionKey, roundID)
}

func roomRoundIdentity(roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	if rootRoundID := roomRootRoundID(roundValue); rootRoundID != "" {
		return rootRoundID
	}
	if roundValue.registrationSequence > 0 {
		return "__registration:" + strconv.FormatUint(roundValue.registrationSequence, 10)
	}
	return roomActiveRoundKey(roundValue.SessionKey, roundValue.RoundID)
}

func (r *roomRoundRegistry) state(conversationID string, create bool) *roomConversationState {
	if r == nil {
		return nil
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}
	r.mu.RLock()
	state := r.conversations[conversationID]
	r.mu.RUnlock()
	if state != nil || !create {
		return state
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conversations == nil {
		r.conversations = make(map[string]*roomConversationState)
	}
	if state = r.conversations[conversationID]; state == nil {
		state = &roomConversationState{
			rounds:         make(map[string]*activeRoomRound),
			roundKeys:      make(map[*activeRoomRound]string),
			guidance:       make(map[*activeRoomSlot]pendingRoomGuidance),
			publicMentions: make(map[*activeRoomRound][]publicMentionWake),
		}
		r.conversations[conversationID] = state
	}
	return state
}

func (r *roomRoundRegistry) register(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	conversationID := roomConversationKey(roundValue.ConversationID, roundValue.SessionKey)
	state := r.state(conversationID, true)
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.registrationSequence++
	roundValue.registrationSequence = state.registrationSequence
	if state.rounds == nil {
		state.rounds = make(map[string]*activeRoomRound)
	}
	if state.roundKeys == nil {
		state.roundKeys = make(map[*activeRoomRound]string)
	}
	key := roomRegistryRoundKey(roundValue)
	if key == "" {
		// 构造态 round 可能还没有业务 ID；注册序号仍能保证同一 shard 内不覆盖。
		key = roomActiveRoundKey(roundValue.SessionKey, "__registration:"+strconv.FormatUint(state.registrationSequence, 10))
	}
	if existing := state.rounds[key]; existing != nil && existing != roundValue {
		// 同一业务 ID 的构造态 round 不能互相覆盖；查找仍可通过 round ID
		// 扫描命中，注册表键改用本 shard 内唯一序号。
		key = roomActiveRoundKey(roundValue.SessionKey, "__registration:"+strconv.FormatUint(state.registrationSequence, 10))
	}
	if previousKey := state.roundKeys[roundValue]; previousKey != "" {
		delete(state.rounds, previousKey)
	}
	state.roundKeys[roundValue] = key
	state.rounds[key] = roundValue
	for _, slot := range roundValue.Slots {
		if slot == nil {
			continue
		}
		slot.bindConversationState(conversationID, state)
	}
}

func (r *roomRoundRegistry) unregister(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	conversationID := roomConversationKey(roundValue.ConversationID, roundValue.SessionKey)
	state := r.state(conversationID, false)
	if state == nil {
		return
	}
	state.mu.Lock()
	key := state.roundKeys[roundValue]
	if key == "" {
		candidate := roomRegistryRoundKey(roundValue)
		if candidate != "" && state.rounds[candidate] == roundValue {
			key = candidate
		}
	}
	if key == "" {
		state.mu.Unlock()
		return
	}
	delete(state.rounds, key)
	delete(state.roundKeys, roundValue)
	for _, slot := range roundValue.Slots {
		if _, pending := state.guidance[slot]; pending {
			// ACK 可能在 round 注销后才到达；保留 shard 关联，避免构造态
			// session key 无法再次定位这条 durable guidance。
			continue
		}
		slot.clearConversationState(state)
	}
	shouldPrune := len(state.rounds) == 0 && len(state.guidance) == 0 && len(state.publicMentions) == 0
	state.mu.Unlock()
	if shouldPrune {
		r.prune(conversationID, state)
	}
}

func (r *roomRoundRegistry) bindSlot(slot *activeRoomSlot, roundValue *activeRoomRound) {
	if slot == nil || roundValue == nil {
		return
	}
	conversationID := roomConversationKey(roundValue.ConversationID, roundValue.SessionKey)
	state := r.state(conversationID, true)
	if state == nil {
		return
	}
	state.mu.Lock()
	slot.bindConversationState(conversationID, state)
	state.mu.Unlock()
}

func (r *roomRoundRegistry) bindSlotToConversation(slot *activeRoomSlot, conversationID string) {
	if slot == nil {
		return
	}
	key := roomConversationKey(conversationID, slot.RuntimeSessionKey)
	state := r.state(key, true)
	if state == nil {
		return
	}
	state.mu.Lock()
	slot.bindConversationState(key, state)
	state.mu.Unlock()
}

func (r *roomRoundRegistry) prune(conversationID string, expected *roomConversationState) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || expected == nil {
		return
	}
	r.mu.Lock()
	if r.conversations[conversationID] == expected {
		delete(r.conversations, conversationID)
	}
	r.mu.Unlock()
}

func (r *roomRoundRegistry) snapshot() []*activeRoomRound {
	states := r.states()
	result := make([]*activeRoomRound, 0)
	for _, state := range states {
		state.mu.RLock()
		for _, roundValue := range state.rounds {
			if roundValue != nil {
				result = append(result, roundValue)
			}
		}
		state.mu.RUnlock()
	}
	return result
}

func (r *roomRoundRegistry) snapshotConversation(conversationID string) []*activeRoomRound {
	state := r.state(conversationID, false)
	if state == nil {
		return nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]*activeRoomRound, 0, len(state.rounds))
	for _, roundValue := range state.rounds {
		if roundValue != nil {
			result = append(result, roundValue)
		}
	}
	return result
}

func (r *roomRoundRegistry) states() []*roomConversationState {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*roomConversationState, 0, len(r.conversations))
	for _, state := range r.conversations {
		if state != nil {
			result = append(result, state)
		}
	}
	return result
}

func (r *roomRoundRegistry) findByRoundID(sessionKey string, roundID string) *activeRoomRound {
	for _, roundValue := range r.roundsForSession(sessionKey) {
		if roundValue == nil || roundValue.SessionKey != sessionKey {
			continue
		}
		if strings.TrimSpace(roundValue.RootRoundID) == roundID || strings.TrimSpace(roundValue.RoundID) == roundID {
			return roundValue
		}
	}
	return nil
}

func (r *roomRoundRegistry) roundsForSession(sessionKey string) []*activeRoomRound {
	if conversationID := roomConversationIDFromSessionKey(sessionKey); conversationID != "" {
		return r.snapshotConversation(conversationID)
	}
	return r.snapshot()
}

func (r *roomRoundRegistry) findSlot(sessionKey string, msgID string) (*activeRoomRound, *activeRoomSlot) {
	for _, roundValue := range r.roundsForSession(sessionKey) {
		if roundValue.SessionKey != sessionKey {
			continue
		}
		if slot := roundValue.Slots[msgID]; slot != nil {
			return roundValue, slot
		}
	}
	return nil, nil
}

func (r *roomRoundRegistry) findSlotByAgentRound(sessionKey string, agentRoundID string) (*activeRoomRound, *activeRoomSlot) {
	for _, roundValue := range r.roundsForSession(sessionKey) {
		if roundValue.SessionKey != sessionKey {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot != nil && strings.TrimSpace(slot.AgentRoundID) == agentRoundID {
				return roundValue, slot
			}
		}
	}
	return nil, nil
}

func (r *roomRoundRegistry) guidanceStateForSlot(slot *activeRoomSlot) *roomConversationState {
	if slot == nil {
		return nil
	}
	conversationID, state := slot.conversationBinding()
	if state != nil {
		return state
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversationID = roomConversationIDFromSessionKey(slot.RuntimeSessionKey)
	}
	if conversationID == "" {
		return nil
	}
	return r.state(conversationID, true)
}

func (r *roomRoundRegistry) putGuidance(slot *activeRoomSlot, pending pendingRoomGuidance) {
	state := r.guidanceStateForSlot(slot)
	if state == nil {
		return
	}
	state.mu.Lock()
	if state.guidance == nil {
		state.guidance = make(map[*activeRoomSlot]pendingRoomGuidance)
	}
	state.guidance[slot] = pending
	state.mu.Unlock()
}

func (r *roomRoundRegistry) hasGuidance(slot *activeRoomSlot) bool {
	state := r.guidanceStateForSlot(slot)
	if state == nil {
		return false
	}
	state.mu.RLock()
	_, ok := state.guidance[slot]
	state.mu.RUnlock()
	return ok
}

func (r *roomRoundRegistry) guidanceSnapshot() []pendingRoomGuidance {
	result := make([]pendingRoomGuidance, 0)
	for _, state := range r.states() {
		state.mu.RLock()
		for _, pending := range state.guidance {
			result = append(result, pending)
		}
		state.mu.RUnlock()
	}
	return result
}

func (r *roomRoundRegistry) loadGuidance(slot *activeRoomSlot) (pendingRoomGuidance, bool) {
	state := r.guidanceStateForSlot(slot)
	if state == nil {
		return pendingRoomGuidance{}, false
	}
	state.mu.RLock()
	pending, ok := state.guidance[slot]
	state.mu.RUnlock()
	return pending, ok
}

func (r *roomRoundRegistry) deleteGuidance(slot *activeRoomSlot) {
	state := r.guidanceStateForSlot(slot)
	if state == nil {
		return
	}
	conversationID, _ := slot.conversationBinding()
	state.mu.Lock()
	delete(state.guidance, slot)
	shouldPrune := len(state.guidance) == 0 && len(state.rounds) == 0 && len(state.publicMentions) == 0
	state.mu.Unlock()
	if shouldPrune {
		slot.clearConversationState(state)
		r.prune(conversationID, state)
	}
}

func (r *roomRoundRegistry) updateGuidance(slot *activeRoomSlot, update func(*pendingRoomGuidance) bool) bool {
	state := r.guidanceStateForSlot(slot)
	if state == nil {
		return false
	}
	state.mu.Lock()
	pending, ok := state.guidance[slot]
	if ok && update(&pending) {
		state.guidance[slot] = pending
	}
	state.mu.Unlock()
	return ok
}

func (r *roomRoundRegistry) enqueuePublicMention(roundValue *activeRoomRound, wake publicMentionWake) bool {
	if roundValue == nil {
		return false
	}
	state := r.state(roomConversationKey(roundValue.ConversationID, roundValue.SessionKey), false)
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.publicMentions == nil {
		state.publicMentions = make(map[*activeRoomRound][]publicMentionWake)
	}
	pending := state.publicMentions[roundValue]
	for _, existing := range pending {
		if existing.TargetAgentID == wake.TargetAgentID &&
			strings.TrimSpace(existing.MessageID) == strings.TrimSpace(wake.MessageID) &&
			strings.TrimSpace(existing.Content) == strings.TrimSpace(wake.Content) {
			return false
		}
	}
	state.publicMentions[roundValue] = append(pending, wake)
	return true
}

func (r *roomRoundRegistry) takePublicMentions(roundValue *activeRoomRound) []publicMentionWake {
	if roundValue == nil {
		return nil
	}
	conversationID := roomConversationKey(roundValue.ConversationID, roundValue.SessionKey)
	state := r.state(conversationID, false)
	if state == nil {
		return nil
	}
	state.mu.Lock()
	wakes := append([]publicMentionWake(nil), state.publicMentions[roundValue]...)
	delete(state.publicMentions, roundValue)
	shouldPrune := len(wakes) > 0 && len(state.rounds) == 0 && len(state.guidance) == 0 && len(state.publicMentions) == 0
	state.mu.Unlock()
	if shouldPrune {
		r.prune(conversationID, state)
	}
	return wakes
}

func (r *roomRoundRegistry) hasPublicMentions(roundValue *activeRoomRound) bool {
	if roundValue == nil {
		return false
	}
	state := r.state(roomConversationKey(roundValue.ConversationID, roundValue.SessionKey), false)
	if state == nil {
		return false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return len(state.publicMentions[roundValue]) > 0
}

func (r *roomRoundRegistry) hasPublicMentionsForConversation(conversationID string) bool {
	state := r.state(strings.TrimSpace(conversationID), false)
	if state == nil {
		return false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return len(state.publicMentions) > 0
}
