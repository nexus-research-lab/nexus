// INPUT: Room conversation 的活跃 round、slot 与跨 root 生命周期更新。
// OUTPUT: 按 conversation 隔离的注册表、派发锁和携带 public handoff 关联的权威 pending snapshot。
// POS: Room realtime 短生命周期状态与订阅恢复投影的单一真相源。
package realtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
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
	// dispatchRefs 由 roomRoundRegistry.mu 保护；dispatchMu 只保护派发临界区。
	dispatchMu   sync.Mutex
	dispatchRefs int
}

// roomRoundRegistry 只保护 conversation state 索引和 dispatch 引用；
// 具体 round 数据与派发临界区均由 conversation state 自己保护。
type roomRoundRegistry struct {
	mu            *sync.RWMutex
	conversations map[string]*roomConversationState
}

// 零值注册表只在局部测试构造中出现；共享兜底锁保证懒初始化仍可并发安全。
var zeroRoomRoundRegistryMu sync.RWMutex

func newRoomRoundRegistry() roomRoundRegistry {
	return roomRoundRegistry{
		mu:            &sync.RWMutex{},
		conversations: make(map[string]*roomConversationState),
	}
}

func newRoomRoundRegistryFromRounds(rounds map[string]*activeRoomRound) roomRoundRegistry {
	registry := newRoomRoundRegistry()
	for _, roundValue := range rounds {
		registry.register(roundValue)
	}
	return registry
}

func (r *roomRoundRegistry) mutex() *sync.RWMutex {
	if r == nil || r.mu == nil {
		return &zeroRoomRoundRegistryMu
	}
	return r.mu
}

func newRoomConversationState() *roomConversationState {
	return &roomConversationState{
		rounds:         make(map[string]*activeRoomRound),
		roundKeys:      make(map[*activeRoomRound]string),
		guidance:       make(map[*activeRoomSlot]pendingRoomGuidance),
		publicMentions: make(map[*activeRoomRound][]publicMentionWake),
	}
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
	mu := r.mutex()
	mu.RLock()
	state := r.conversations[conversationID]
	mu.RUnlock()
	if state != nil || !create {
		return state
	}

	mu.Lock()
	defer mu.Unlock()
	if r.conversations == nil {
		r.conversations = make(map[string]*roomConversationState)
	}
	if state = r.conversations[conversationID]; state == nil {
		state = newRoomConversationState()
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
	mu := r.mutex()
	mu.Lock()
	if r.conversations[conversationID] != expected || expected.dispatchRefs != 0 {
		mu.Unlock()
		return
	}
	expected.mu.RLock()
	empty := len(expected.rounds) == 0 && len(expected.guidance) == 0 && len(expected.publicMentions) == 0
	expected.mu.RUnlock()
	if empty {
		delete(r.conversations, conversationID)
	}
	mu.Unlock()
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
	mu := r.mutex()
	mu.RLock()
	defer mu.RUnlock()
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

// ActiveRoundSnapshot 表示 Room 当前仍在执行的 slot 快照。
// Pending 各自携带 root；RoundID 只保留给单 root 旧客户端作 fallback。
type ActiveRoundSnapshot struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	RoundID        string
	Pending        []protocol.ChatAckPendingSlot
}

// CountRunningTasks 返回指定 Agent 当前在 Room 中的活跃任务数。
func (s *Service) CountRunningTasks(agentID string) int {
	count := 0
	for _, roundValue := range s.rounds.snapshot() {
		for _, slot := range roundValue.Slots {
			if slot != nil && slot.AgentID == agentID && !slot.isTerminal() {
				count++
			}
		}
	}
	return count
}

// SetPermissionModeForAgent 将权限模式热同步到指定 agent 已存在的 Room runtime。
func (s *Service) SetPermissionModeForAgent(ctx context.Context, agentID string, mode sdkpermission.Mode) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil
	}
	type permissionModeTarget struct {
		round  *activeRoomRound
		slot   *activeRoomSlot
		client runtimectx.Client
	}
	targets := make([]permissionModeTarget, 0)
	for _, roundValue := range s.rounds.snapshot() {
		if roundValue == nil {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || slot.AgentID != agentID || slot.isTerminal() {
				continue
			}
			client := slot.getClient()
			if client == nil {
				continue
			}
			targets = append(targets, permissionModeTarget{
				round: roundValue, slot: slot, client: client,
			})
		}
	}
	errs := make([]error, 0)
	for _, target := range targets {
		if err := target.client.SetPermissionMode(ctx, mode); err != nil {
			interruptErr := s.failClosedPermissionReload(context.WithoutCancel(ctx), target.slot)
			errs = append(errs, fmt.Errorf(
				"Room session %s Agent %s 权限热同步失败，已中断旧 slot: %w",
				target.round.SessionKey,
				target.slot.AgentID,
				errors.Join(err, interruptErr),
			))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) failClosedPermissionReload(ctx context.Context, slot *activeRoomSlot) error {
	if slot == nil {
		return nil
	}
	const reason = "Agent 权限模式已更新，旧 Room runtime 无法安全热同步"
	slot.suppressOutput()
	slot.setInterruptReason(reason)
	slot.setStatus("cancelled")
	var interruptErr error
	if client := slot.getClient(); client != nil {
		interruptErr = client.Interrupt(ctx)
	}
	slot.cancelRuntime()
	if s.permission != nil {
		s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, reason)
	}
	return interruptErr
}

// GetActiveRoundSnapshot 返回指定 conversation 的活跃 slot 快照。
func (s *Service) GetActiveRoundSnapshot(conversationID string) *ActiveRoundSnapshot {
	pending := make([]protocol.ChatAckPendingSlot, 0)
	snapshot := &ActiveRoundSnapshot{}
	rootRoundIDs := make(map[string]struct{})
	for _, roundValue := range s.rounds.snapshotConversation(conversationID) {
		if roundValue == nil || roundValue.ConversationID != conversationID {
			continue
		}
		if snapshot.SessionKey == "" {
			snapshot.SessionKey = roundValue.SessionKey
			snapshot.RoomID = roundValue.RoomID
			snapshot.ConversationID = roundValue.ConversationID
		}
		rootRoundID := roomRootRoundID(roundValue)
		if rootRoundID != "" {
			rootRoundIDs[rootRoundID] = struct{}{}
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || slot.isTerminal() {
				continue
			}
			status := slot.getStatus()
			if status == "running" {
				status = "streaming"
			}
			pending = append(pending, protocol.ChatAckPendingSlot{
				AgentID:        slot.AgentID,
				AgentRoundID:   slot.AgentRoundID,
				MsgID:          slot.MsgID,
				RoundID:        rootRoundID,
				HandoffID:      slot.handoffID(),
				HiddenFromUser: roomSlotHiddenFromUser(slot),
				Status:         status,
				Timestamp:      slot.TimestampMS,
				Index:          slot.Index,
			})
		}
	}
	if len(pending) == 0 {
		return nil
	}
	sort.Slice(pending, func(i int, j int) bool {
		if pending[i].Timestamp != pending[j].Timestamp {
			return pending[i].Timestamp < pending[j].Timestamp
		}
		return pending[i].Index < pending[j].Index
	})
	snapshot.Pending = pending
	if len(rootRoundIDs) == 1 {
		for rootRoundID := range rootRoundIDs {
			snapshot.RoundID = rootRoundID
		}
	}
	return snapshot
}

func (s *Service) registerRound(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	s.rounds.register(roundValue)
}

func (s *Service) finishRound(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	s.runtime.MarkRoundTerminal(roundValue.SessionKey, roundValue.RoundID)
	s.rounds.unregister(roundValue)
	roundValue.doneOnce.Do(func() {
		close(roundValue.Done)
	})
}

func roomRootRoundID(roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	if rootRoundID := strings.TrimSpace(roundValue.RootRoundID); rootRoundID != "" {
		return rootRoundID
	}
	return strings.TrimSpace(roundValue.RoundID)
}

func roomActiveRoundKey(sessionKey string, roundID string) string {
	return strings.TrimSpace(sessionKey) + "::" + strings.TrimSpace(roundID)
}

func (r *activeRoomRound) allSlotsCancelled() bool {
	if len(r.Slots) == 0 {
		return false
	}
	for _, slot := range r.Slots {
		if slot == nil || slot.getStatus() != "cancelled" {
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
		if slot != nil && slot.getStatus() == "error" {
			return true
		}
	}
	return false
}

// firstSlotErrorMessage 返回按展示顺序最早出现的 slot 错误。
func (r *activeRoomRound) firstSlotErrorMessage() string {
	if r == nil {
		return ""
	}
	var firstMessage string
	firstIndex := 0
	found := false
	for _, slot := range r.Slots {
		if slot == nil {
			continue
		}
		message := slot.getErrorMessage()
		if message == "" || (found && slot.Index >= firstIndex) {
			continue
		}
		firstIndex = slot.Index
		firstMessage = message
		found = true
	}
	return firstMessage
}

func (r *activeRoomRound) hasRunningSubagentTasks() bool {
	if r == nil {
		return false
	}
	for _, slot := range r.Slots {
		if slot != nil && slot.hasRunningSubagentTask() {
			return true
		}
	}
	return false
}

// roomDispatchLease 把 queue、wake、continuation 和 round finish 的顺序
// 绑定到 conversation state；runtime 执行本身不依赖这把锁。
type roomDispatchLease struct {
	registry *roomRoundRegistry
	key      string
	state    *roomConversationState
	once     sync.Once
}

func (r *roomRoundRegistry) acquireDispatch(key string) *roomDispatchLease {
	if r == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "__room_unknown_conversation__"
	}

	mu := r.mutex()
	mu.Lock()
	if r.conversations == nil {
		r.conversations = make(map[string]*roomConversationState)
	}
	state := r.conversations[key]
	if state == nil {
		state = newRoomConversationState()
		r.conversations[key] = state
	}
	state.dispatchRefs++
	mu.Unlock()

	state.dispatchMu.Lock()
	return &roomDispatchLease{
		registry: r,
		key:      key,
		state:    state,
	}
}

func (l *roomDispatchLease) Unlock() {
	if l == nil || l.registry == nil || l.state == nil {
		return
	}
	l.once.Do(func() {
		l.state.dispatchMu.Unlock()
		l.registry.releaseDispatch(l.key, l.state)
	})
}

func (r *roomRoundRegistry) releaseDispatch(key string, state *roomConversationState) {
	if r == nil || state == nil {
		return
	}
	mu := r.mutex()
	mu.Lock()
	current := r.conversations[key]
	if current != state {
		mu.Unlock()
		return
	}
	if state.dispatchRefs > 0 {
		state.dispatchRefs--
	}
	last := state.dispatchRefs == 0
	mu.Unlock()
	if last {
		r.prune(key, state)
	}
}

func roomDispatchStateKey(sessionKey string, conversationID string) string {
	sessionKey = strings.TrimSpace(sessionKey)
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversationID = roomConversationIDFromSessionKey(sessionKey)
	}
	if conversationID != "" {
		// conversation ID 是 Room 的并发边界；shared session 与 agent session
		// 可能不同，但它们仍必须落在同一份 conversation state 下。
		return conversationID
	}
	if sessionKey != "" {
		// 没有可解析 conversation 的 session 仍需保持彼此隔离，不能
		// 把所有未知会话合并到同一把锁。
		return "__room_dispatch_session:" + sessionKey
	}
	return "__room_unknown_conversation__"
}

func (s *Service) lockRoomDispatch(sessionKey string, conversationID string) *roomDispatchLease {
	if s == nil {
		return nil
	}
	return s.rounds.acquireDispatch(roomDispatchStateKey(sessionKey, conversationID))
}
