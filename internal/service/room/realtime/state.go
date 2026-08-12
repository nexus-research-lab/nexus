// INPUT: Room round/slot 生命周期、structured WorkBinding/ReviewBinding、运行时消息与并发状态变更。
// OUTPUT: 固定权限世代、trusted dispatch identity、稳定 owner/root usage scope、Work/Goal 绑定、结算屏障、游标与最终回复快照。
// POS: Room 实时执行过程的内存状态与权限能力模型。
package realtime

import (
	"context"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// roomSlotRuntimeState 只负责 runtime 生命周期，不持有 Goal 或 delivery 数据。
type roomSlotRuntimeState struct {
	mu               sync.RWMutex
	sdkSessionID     string
	runtimeKind      string
	contextWindow    int
	contextColdStart bool
	client           runtimectx.Client
	cancel           context.CancelFunc
	status           string
	interruptReason  string
	errorMessage     string
	done             chan struct{}
	doneOnce         sync.Once
}

type roomGoalAuthoritySource string

const (
	roomGoalAuthorityExplicitRound      roomGoalAuthoritySource = "explicit_round"
	roomGoalAuthorityExecutionBinding   roomGoalAuthoritySource = "execution_binding"
	roomGoalAuthorityModelCreate        roomGoalAuthoritySource = "model_create"
	roomGoalAuthorityExternalActivation roomGoalAuthoritySource = "external_activation"
)

// roomGoalMutationAuthority is a fixed per-round capability. Goal steering may
// update what the runtime can read, but it must never retarget a predecessor
// round's semantic writes to a later objective revision.
type roomGoalMutationAuthority struct {
	SessionKey        string
	GoalID            string
	ObjectiveRevision int64
	ExecutionID       string
	RootRoundID       string
	Source            roomGoalAuthoritySource
}

func (a roomGoalMutationAuthority) valid() bool {
	return strings.TrimSpace(a.SessionKey) != "" &&
		strings.TrimSpace(a.GoalID) != "" &&
		a.ObjectiveRevision > 0 &&
		a.Source != ""
}

// roomSlotGoalState 负责 Goal accounting、固定 revision mutation capability 与协作进度。
type roomSlotGoalState struct {
	mu                   sync.RWMutex
	sessionKey           string
	context              string
	idForUsage           string
	childIDForUsage      string
	mutationAuthority    roomGoalMutationAuthority
	authorityOnce        sync.Once
	authorityState       *runtimectx.GoalAuthorityState
	objectiveRevision    atomic.Int64
	runtimeIgnored       bool
	usage                *goalsvc.RuntimeUsageAccumulator
	usageStartedAt       time.Time
	lastAssistant        protocol.Message
	completionCandidateID   string
	completionAssistant     protocol.Message
	completionReceipt       protocol.GoalCompletionReceipt
	completionReceiptStored bool
	toolProgress         bool
	subagentTasks        map[string]struct{}
	subagentUsagePending map[string]roomSubagentUsageObservation
	usageRetrying        bool
	subagentHistory      bool
	usageClaimPending    bool
	usageScopeConsumed   bool
	terminalSettled      bool
	resultUsageWritten   bool
}

// roomSubagentUsageObservation 是尚未确认持久化的 child checkpoint + lifecycle
// evidence。三个字段都单调推进，避免旧请求成功返回后误清除更新的 terminal 状态。
type roomSubagentUsageObservation struct {
	cumulativeTotal            int64
	terminal                   bool
	terminalTokenUsageObserved bool
	observedAt                 time.Time
}

// roomSlotCursorState 负责 public/private context 的消费边界。
type roomSlotCursorState struct {
	mu               sync.RWMutex
	publicID         string
	publicTimestamp  int64
	messageID        string
	messageTimestamp int64
}

// roomSlotDeliveryState 负责输入队列、回复路由和输出投影。
type roomSlotDeliveryState struct {
	mu                     sync.Mutex
	replyRoute             protocol.RoomReplyRoute
	replySourceMessage     string
	handoffID              string
	queuedInputs           []roomQueuedInput
	suppressOutput         bool
	publicMessagePublished bool
	noReplyCandidate       bool
	pendingStream          []protocol.EventMessage
}

// roomSlotConversationState 只保存 slot 与 conversation shard 的关联，避免
// guidance 回调在 round 收尾期间读取到半更新的 registry 指针。
type roomSlotConversationState struct {
	mu    sync.RWMutex
	id    string
	state *roomConversationState
}

// roomSlotMutableState 只组合彼此独立同步的状态域，不提供跨域总锁。
// activeRoomSlot 因而只表达稳定身份与一个明确的 mutable state 边界。
type roomSlotMutableState struct {
	runtime      roomSlotRuntimeState
	goal         roomSlotGoalState
	cursor       roomSlotCursorState
	delivery     roomSlotDeliveryState
	conversation roomSlotConversationState
}

type activeRoomSlot struct {
	// 以下字段是 slot 创建后不再改变的稳定身份。
	RoomSessionID         string
	OwnerUserID           string
	AgentID               string
	AgentRoundID          string
	GoalUsageScopeRoundID string
	MsgID                 string
	RuntimeSessionKey     string
	WorkspacePath         string
	Index                 int
	TimestampMS           int64
	Trigger               roomTrigger
	TriggerAttachments    []protocol.ChatAttachment
	WorkBinding           *protocol.ExecutionWorkBinding
	ReviewBinding         *protocol.ExecutionReviewBinding
	mutable               roomSlotMutableState
}

func (s *activeRoomSlot) ensureGoalObjectiveRevision(initial int64) *atomic.Int64 {
	if s == nil {
		return nil
	}
	state := &s.mutable.goal.objectiveRevision
	for initial > 0 {
		current := state.Load()
		if initial <= current || state.CompareAndSwap(current, initial) {
			break
		}
	}
	return state
}

func (s *activeRoomSlot) ensureGoalAuthorityState() *runtimectx.GoalAuthorityState {
	if s == nil {
		return nil
	}
	goalState := &s.mutable.goal
	goalState.authorityOnce.Do(func() {
		goalState.authorityState = runtimectx.NewGoalAuthorityStateWithRevision(
			"",
			"",
			&goalState.objectiveRevision,
		)
	})
	return goalState.authorityState
}

func (s *activeRoomSlot) currentGoalObjectiveRevision() int64 {
	if s == nil {
		return 0
	}
	return s.mutable.goal.objectiveRevision.Load()
}

func (s *activeRoomSlot) adoptGoalObjectiveRevision(revision int64) {
	if revision <= 0 {
		return
	}
	state := &s.mutable.goal.objectiveRevision
	for {
		current := state.Load()
		if revision <= current || state.CompareAndSwap(current, revision) {
			return
		}
	}
}

type activeRoomRound struct {
	SessionKey                  string
	RoomID                      string
	ConversationID              string
	CoordinatorAgentID          string
	RoomType                    string
	Context                     *protocol.ConversationContextAggregate
	RoundID                     string
	RootRoundID                 string
	registrationSequence        uint64
	HopIndex                    int
	OwnerUserID                 string
	Internal                    bool
	AuthorityEpoch              int64
	TrustedConfigurationContext bool
	ExecutionOrigin             string
	// trustedQueuedConfigurationContext marks only the runtime created from a
	// successfully claimed direct-user queue admission.
	trustedQueuedConfigurationContext bool
	// pendingTrustedQueueDispatch is a one-hop scaffold used while turning a
	// queued public user trigger into its runtime round. It is never inherited
	// by later Agent-to-Agent public handoffs.
	pendingTrustedQueueDispatch bool
	InputOptions                sdkprotocol.OutboundMessageOptions
	Cancel                      context.CancelFunc
	PermissionMode              sdkpermission.Mode
	PermissionHandler           sdkpermission.Handler
	EventObserver               RoomEventObserver
	GoalContext                 string
	GoalID                      string
	GoalObjectiveRevision       int64
	ExecutionID                 string
	Slots                       map[string]*activeRoomSlot
	RunningSubagents            atomic.Bool
	postRoundDispatched         atomic.Bool
	Done                        chan struct{}
	doneOnce                    sync.Once
}

type roomTrigger = roomdomain.Trigger

type publicMentionWake struct {
	HandoffID     string
	TriggerType   string
	QueueSource   protocol.InputQueueSource
	SourceAgentID string
	TargetAgentID string
	Content       string
	MessageID     string
	ReplyRoute    protocol.RoomReplyRoute
	WorkBinding   *protocol.ExecutionWorkBinding
	ReviewBinding *protocol.ExecutionReviewBinding
}

type roomQueuedInput struct {
	RoundID string
	Content string
}

// INPUT: Room Agent slot 的 runtime、Goal、cursor 与 delivery 子状态。
// OUTPUT: 各领域独立同步的 slot 状态快照、普通输入 drain 与客户端绑定。
// POS: 单个 Room Agent 执行槽的状态所有者。
func (slot *activeRoomSlot) bindConversationState(conversationID string, state *roomConversationState) {
	if slot == nil {
		return
	}
	slot.mutable.conversation.mu.Lock()
	if slot.mutable.conversation.state == nil || slot.mutable.conversation.state == state {
		// slot 的 conversation 归属只在首次注册时建立；后续 ACK/cleanup
		// 必须继续使用同一 shard，不能被迟到的 location 覆盖。
		slot.mutable.conversation.id = strings.TrimSpace(conversationID)
		slot.mutable.conversation.state = state
	}
	slot.mutable.conversation.mu.Unlock()
}

func (slot *activeRoomSlot) clearConversationState(expected *roomConversationState) {
	if slot == nil {
		return
	}
	slot.mutable.conversation.mu.Lock()
	if expected == nil || slot.mutable.conversation.state == expected {
		slot.mutable.conversation.id = ""
		slot.mutable.conversation.state = nil
	}
	slot.mutable.conversation.mu.Unlock()
}

func (slot *activeRoomSlot) conversationBinding() (string, *roomConversationState) {
	if slot == nil {
		return "", nil
	}
	slot.mutable.conversation.mu.RLock()
	defer slot.mutable.conversation.mu.RUnlock()
	return slot.mutable.conversation.id, slot.mutable.conversation.state
}

func (s *Service) finishSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	s.forgetRoomSlotGuidance(slot)
	slot.closeDone()
}

func (slot *activeRoomSlot) getStatus() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return slot.mutable.runtime.status
}

func (slot *activeRoomSlot) setStatus(status string) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.status = status
	slot.mutable.runtime.mu.Unlock()
}

// setErrorMessage 保存 slot 的终态原因，供 root round 收口时重放给前端。
func (slot *activeRoomSlot) setErrorMessage(message string) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.errorMessage = strings.TrimSpace(message)
	slot.mutable.runtime.mu.Unlock()
}

// getErrorMessage 读取 slot 的终态原因。
func (slot *activeRoomSlot) getErrorMessage() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.runtime.errorMessage)
}

func (slot *activeRoomSlot) isTerminal() bool {
	switch slot.getStatus() {
	case "finished", "error", "cancelled":
		return true
	default:
		return false
	}
}

func (slot *activeRoomSlot) setSDKSessionID(sessionID string) bool {
	if slot == nil {
		return false
	}
	sessionID = strings.TrimSpace(sessionID)
	slot.mutable.runtime.mu.Lock()
	defer slot.mutable.runtime.mu.Unlock()
	if sessionID == "" || sessionID == strings.TrimSpace(slot.mutable.runtime.sdkSessionID) {
		return false
	}
	slot.mutable.runtime.sdkSessionID = sessionID
	return true
}

func (slot *activeRoomSlot) clearSDKSessionID() bool {
	if slot == nil {
		return false
	}
	slot.mutable.runtime.mu.Lock()
	defer slot.mutable.runtime.mu.Unlock()
	if strings.TrimSpace(slot.mutable.runtime.sdkSessionID) == "" {
		return false
	}
	slot.mutable.runtime.sdkSessionID = ""
	return true
}

func (slot *activeRoomSlot) getSDKSessionID() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.runtime.sdkSessionID)
}

func (slot *activeRoomSlot) drainQueuedInputs() []roomQueuedInput {
	if slot == nil {
		return nil
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	if len(slot.mutable.delivery.queuedInputs) == 0 {
		return nil
	}
	inputs := slices.Clone(slot.mutable.delivery.queuedInputs)
	slot.mutable.delivery.queuedInputs = nil
	return inputs
}

func (slot *activeRoomSlot) setDeliveryMetadata(
	replyRoute protocol.RoomReplyRoute,
	replySourceMessage string,
	handoffID string,
) {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.replyRoute = replyRoute
	slot.mutable.delivery.replySourceMessage = strings.TrimSpace(replySourceMessage)
	slot.mutable.delivery.handoffID = strings.TrimSpace(handoffID)
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) replyRoute() protocol.RoomReplyRoute {
	if slot == nil {
		return protocol.RoomReplyRoute{}
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.replyRoute
}

func (slot *activeRoomSlot) replySourceMessage() string {
	if slot == nil {
		return ""
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.replySourceMessage
}

func (slot *activeRoomSlot) handoffID() string {
	if slot == nil {
		return ""
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.handoffID
}

func (slot *activeRoomSlot) setCursors(publicID string, publicTimestamp int64, messageID string, messageTimestamp int64) {
	if slot == nil {
		return
	}
	slot.mutable.cursor.mu.Lock()
	slot.mutable.cursor.publicID = strings.TrimSpace(publicID)
	slot.mutable.cursor.publicTimestamp = publicTimestamp
	slot.mutable.cursor.messageID = strings.TrimSpace(messageID)
	slot.mutable.cursor.messageTimestamp = messageTimestamp
	slot.mutable.cursor.mu.Unlock()
}

func (slot *activeRoomSlot) publicCursor() (string, int64) {
	if slot == nil {
		return "", 0
	}
	slot.mutable.cursor.mu.RLock()
	defer slot.mutable.cursor.mu.RUnlock()
	return slot.mutable.cursor.publicID, slot.mutable.cursor.publicTimestamp
}

func (slot *activeRoomSlot) messageCursor() (string, int64) {
	if slot == nil {
		return "", 0
	}
	slot.mutable.cursor.mu.RLock()
	defer slot.mutable.cursor.mu.RUnlock()
	return slot.mutable.cursor.messageID, slot.mutable.cursor.messageTimestamp
}

func (slot *activeRoomSlot) cursorSnapshot() (string, int64, string, int64) {
	if slot == nil {
		return "", 0, "", 0
	}
	slot.mutable.cursor.mu.RLock()
	defer slot.mutable.cursor.mu.RUnlock()
	return slot.mutable.cursor.publicID, slot.mutable.cursor.publicTimestamp, slot.mutable.cursor.messageID, slot.mutable.cursor.messageTimestamp
}

func (slot *activeRoomSlot) setClient(client runtimectx.Client) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.client = client
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) getClient() runtimectx.Client {
	if slot == nil {
		return nil
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return slot.mutable.runtime.client
}

func (slot *activeRoomSlot) setResultUsageWritten() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.resultUsageWritten = true
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) resultUsageWasWritten() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.resultUsageWritten
}

func (slot *activeRoomSlot) setCancel(cancel context.CancelFunc) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.cancel = cancel
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) cancelRuntime() {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.RLock()
	cancel := slot.mutable.runtime.cancel
	slot.mutable.runtime.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (slot *activeRoomSlot) setRuntimeKind(runtimeKind string) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.runtimeKind = strings.TrimSpace(runtimeKind)
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) runtimeKind() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.runtime.runtimeKind)
}

func (slot *activeRoomSlot) setContextWindow(window int) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.contextWindow = window
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) contextWindow() int {
	if slot == nil {
		return 0
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return slot.mutable.runtime.contextWindow
}

func (slot *activeRoomSlot) setContextColdStart(coldStart bool) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.contextColdStart = coldStart
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) contextColdStart() bool {
	if slot == nil {
		return false
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return slot.mutable.runtime.contextColdStart
}

func (slot *activeRoomSlot) beginGoalUsage() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.usage = goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(slot.mutable.goal.idForUsage) != "")
	slot.mutable.goal.usageStartedAt = time.Now()
	slot.mutable.goal.terminalSettled = false
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalUsageAccumulator(usage *goalsvc.RuntimeUsageAccumulator) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.usage = usage
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) startGoalUsageFromRoundStartIfInactive() (protocol.GoalUsage, bool) {
	if slot == nil {
		return protocol.GoalUsage{}, false
	}
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if slot.mutable.goal.usage != nil && slot.mutable.goal.usage.Active() {
		return protocol.GoalUsage{}, false
	}
	if slot.mutable.goal.usage == nil {
		slot.mutable.goal.usage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	// 模型在本轮创建 Goal 时，当前 slot 的整轮工作都属于这个 Goal。
	return slot.mutable.goal.usage.ActivateFromRoundStart()
}

func (slot *activeRoomSlot) resetGoalUsage(snapshot goalsvc.RuntimeUsageSnapshot) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.usage == nil {
		slot.mutable.goal.usage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	slot.mutable.goal.usage.Reset(snapshot)
	slot.mutable.goal.terminalSettled = false
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageDelta(snapshot goalsvc.RuntimeUsageSnapshot) (protocol.GoalUsage, bool, bool) {
	if slot == nil {
		return protocol.GoalUsage{}, false, false
	}
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if slot.mutable.goal.usage == nil {
		return protocol.GoalUsage{}, false, false
	}
	usage, ok := slot.mutable.goal.usage.Delta(snapshot)
	return usage, ok, true
}

func (slot *activeRoomSlot) goalUsageActive() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	return slot.mutable.goal.usage != nil && slot.mutable.goal.usage.Active()
}

func (slot *activeRoomSlot) goalUsageActiveForGoal(goalID string) bool {
	if slot == nil || strings.TrimSpace(goalID) == "" {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.goal.idForUsage) == strings.TrimSpace(goalID) &&
		slot.mutable.goal.usage != nil &&
		slot.mutable.goal.usage.Active()
}

func (slot *activeRoomSlot) closeGoalUsage() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.usage != nil {
		slot.mutable.goal.usage.Close()
	}
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) clearGoalUsage() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.usage != nil {
		slot.mutable.goal.usage.Close()
	}
	slot.mutable.goal.idForUsage = ""
	slot.mutable.goal.childIDForUsage = ""
	slot.mutable.goal.mutationAuthority = roomGoalMutationAuthority{}
	slot.mutable.goal.usageClaimPending = false
	slot.mutable.goal.terminalSettled = false
	if authority := slot.ensureGoalAuthorityState(); authority != nil {
		authority.Clear()
	}
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalUsageTerminalSettled(settled bool) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.terminalSettled = settled
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageTerminalSettled() bool {
	if slot == nil {
		return true
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.terminalSettled
}

func (slot *activeRoomSlot) goalUsageSettlementRequired() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.usage != nil ||
		strings.TrimSpace(slot.mutable.goal.idForUsage) != "" ||
		strings.TrimSpace(slot.mutable.goal.childIDForUsage) != ""
}

func (slot *activeRoomSlot) setGoalUsageClaimPending(pending bool) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.usageClaimPending = pending
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageClaimPending() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.usageClaimPending
}

func (slot *activeRoomSlot) beginGoalUsageFinalizing() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if slot.mutable.goal.usage == nil ||
		!slot.mutable.goal.usage.Active() ||
		strings.TrimSpace(slot.mutable.goal.idForUsage) == "" {
		return false
	}
	slot.mutable.goal.usage.BeginFinalizing()
	return true
}

func (slot *activeRoomSlot) goalUsageStartedAt() time.Time {
	if slot == nil {
		return time.Time{}
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.usageStartedAt
}

func (slot *activeRoomSlot) setInterruptReason(reason string) {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	slot.mutable.runtime.interruptReason = reason
	slot.mutable.runtime.mu.Unlock()
}

func (slot *activeRoomSlot) getInterruptReason() string {
	if slot == nil {
		return ""
	}
	slot.mutable.runtime.mu.RLock()
	defer slot.mutable.runtime.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.runtime.interruptReason)
}

func (slot *activeRoomSlot) doneChannel() <-chan struct{} {
	if slot == nil {
		return nil
	}
	slot.mutable.runtime.mu.Lock()
	defer slot.mutable.runtime.mu.Unlock()
	if slot.mutable.runtime.done == nil {
		slot.mutable.runtime.done = make(chan struct{})
	}
	return slot.mutable.runtime.done
}

func (slot *activeRoomSlot) closeDone() {
	if slot == nil {
		return
	}
	slot.mutable.runtime.mu.Lock()
	if slot.mutable.runtime.done == nil {
		slot.mutable.runtime.done = make(chan struct{})
	}
	slot.mutable.runtime.doneOnce.Do(func() { close(slot.mutable.runtime.done) })
	slot.mutable.runtime.mu.Unlock()
}

func normalizeRoomInterruptReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return reason
	}
	// Room 的停止是槽位状态，不应把默认英文文案写进公开结果正文。
	return messagepkg.InterruptWithoutMessage
}

func markRoomSlotInterrupted(slot *activeRoomSlot, reason string) {
	if slot == nil {
		return
	}
	slot.setInterruptReason(normalizeRoomInterruptReason(reason))
}

func roomSlotInterruptReason(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	return slot.getInterruptReason()
}

func roomInterruptDisplayReason(reason string) string {
	return messagepkg.NormalizeInterruptDisplayText(reason)
}

func roomSlotInterruptDisplayReason(slot *activeRoomSlot) string {
	return roomInterruptDisplayReason(roomSlotInterruptReason(slot))
}

func (slot *activeRoomSlot) beginNoReplyCandidate() {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.noReplyCandidate = true
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) suppressOutput() {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.suppressOutput = true
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) publicMessageWasPublished() bool {
	if slot == nil {
		return false
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.publicMessagePublished
}

func (slot *activeRoomSlot) setPendingStream(events []protocol.EventMessage) {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.pendingStream = slices.Clone(events)
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) markPublicMessagePublished() {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.publicMessagePublished = true
	slot.mutable.delivery.suppressOutput = true
	slot.mutable.delivery.pendingStream = nil
	slot.mutable.delivery.noReplyCandidate = false
	slot.mutable.delivery.mu.Unlock()
}

func (slot *activeRoomSlot) shouldSuppressOutput() bool {
	if slot == nil {
		return false
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	return slot.mutable.delivery.suppressOutput
}

func (slot *activeRoomSlot) eventsReadyForEmission(event protocol.EventMessage) []protocol.EventMessage {
	if slot == nil {
		return []protocol.EventMessage{event}
	}
	slot.mutable.delivery.mu.Lock()
	defer slot.mutable.delivery.mu.Unlock()
	if slot.mutable.delivery.suppressOutput {
		slot.mutable.delivery.pendingStream = nil
		return nil
	}
	if slot.mutable.delivery.noReplyCandidate {
		if event.EventType != protocol.EventTypeStream {
			slot.mutable.delivery.noReplyCandidate = false
		} else if roomdomain.IsNoReplyCandidateStreamEvent(event) {
			slot.mutable.delivery.pendingStream = append(slot.mutable.delivery.pendingStream, event)
			return nil
		} else {
			slot.mutable.delivery.noReplyCandidate = false
		}
	}
	if len(slot.mutable.delivery.pendingStream) == 0 {
		return []protocol.EventMessage{event}
	}
	events := slices.Clone(slot.mutable.delivery.pendingStream)
	slot.mutable.delivery.pendingStream = nil
	events = append(events, event)
	return events
}

func (slot *activeRoomSlot) markCancelled() bool {
	if slot == nil {
		return false
	}
	slot.mutable.runtime.mu.Lock()
	defer slot.mutable.runtime.mu.Unlock()
	if slot.mutable.runtime.status == "cancelled" {
		return false
	}
	slot.mutable.runtime.status = "cancelled"
	return true
}

func (slot *activeRoomSlot) rememberGoalAssistantMessage(message protocol.Message) {
	if slot == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.lastAssistant = protocol.Clone(message)
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) lastGoalAssistantMessage() protocol.Message {
	if slot == nil {
		return nil
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return protocol.Clone(slot.mutable.goal.lastAssistant)
}

func (slot *activeRoomSlot) markGoalCompletionCandidate(goalID string) {
	if slot == nil || strings.TrimSpace(goalID) == "" {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.completionCandidateID = strings.TrimSpace(goalID)
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) rememberGoalCompletionAssistant(message protocol.Message) {
	if slot == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.completionCandidateID != "" {
		slot.mutable.goal.completionAssistant = protocol.Clone(message)
	}
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalCompletionReceiptSnapshot() (
	string,
	protocol.Message,
	protocol.GoalCompletionReceipt,
	bool,
) {
	if slot == nil {
		return "", nil, protocol.GoalCompletionReceipt{}, false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return strings.TrimSpace(slot.mutable.goal.completionCandidateID),
		protocol.Clone(slot.mutable.goal.completionAssistant),
		slot.mutable.goal.completionReceipt,
		slot.mutable.goal.completionReceiptStored
}

func (slot *activeRoomSlot) markGoalCompletionReceiptStored(
	goalID string,
	receipt protocol.GoalCompletionReceipt,
) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	if strings.TrimSpace(slot.mutable.goal.completionCandidateID) == strings.TrimSpace(goalID) {
		slot.mutable.goal.completionReceipt = receipt
		slot.mutable.goal.completionReceiptStored = true
	}
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) rememberSubagentTaskMessage(message protocol.Message) {
	if slot == nil {
		return
	}
	metadata, _ := message["metadata"].(map[string]any)
	taskID := strings.TrimSpace(anyString(metadata["task_id"]))
	if taskID == "" {
		return
	}
	subtype := strings.TrimSpace(anyString(metadata["subtype"]))
	status := strings.TrimSpace(anyString(metadata["status"]))
	if !metadataLooksLikeSubagentTask(metadata) && !slot.knowsSubagentTask(taskID) {
		return
	}
	runtimeKind := slot.runtimeKind()
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if runtimeKind != "" {
		metadata["runtime_kind"] = runtimeKind
	}
	slot.mutable.goal.subagentHistory = true
	if slot.mutable.goal.subagentTasks == nil {
		slot.mutable.goal.subagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_progress", "task_updated":
		if isTerminalSubagentTaskStatus(status) {
			delete(slot.mutable.goal.subagentTasks, taskID)
			return
		}
		slot.mutable.goal.subagentTasks[taskID] = struct{}{}
	case "task_notification":
		if isTerminalSubagentTaskStatus(status) {
			delete(slot.mutable.goal.subagentTasks, taskID)
		}
	}
}

func (slot *activeRoomSlot) knowsSubagentTask(taskID string) bool {
	if slot == nil || strings.TrimSpace(taskID) == "" {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	taskID = strings.TrimSpace(taskID)
	if _, ok := slot.mutable.goal.subagentTasks[taskID]; ok {
		return true
	}
	_, ok := slot.mutable.goal.subagentUsagePending[taskID]
	return ok
}

func (slot *activeRoomSlot) hasSubagentHistory() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.subagentHistory
}

func (slot *activeRoomSlot) hasRunningSubagentTask() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return len(slot.mutable.goal.subagentTasks) > 0 ||
		len(slot.mutable.goal.subagentUsagePending) > 0
}

// markSubagentUsagePending 建立独立的 source 持久化 join barrier，并保留每个 task
// 最大的累计值（首次显式 0 也会保留）。它与 runtime task 生命周期分开，防止终态消息先移除
// task、后写 checkpoint 时被并发 finalization 穿透。
func (slot *activeRoomSlot) markSubagentUsagePending(taskID string, cumulativeTotal int64) {
	slot.markSubagentUsageObservationPending(roomSubagentUsageObservation{
		cumulativeTotal: cumulativeTotal,
	}, taskID)
}

func (slot *activeRoomSlot) markSubagentUsageObservationPending(
	observation roomSubagentUsageObservation,
	taskID string,
) {
	if slot == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	slot.mutable.goal.mu.Lock()
	if slot.mutable.goal.subagentUsagePending == nil {
		slot.mutable.goal.subagentUsagePending = make(map[string]roomSubagentUsageObservation)
	}
	taskID = strings.TrimSpace(taskID)
	current := slot.mutable.goal.subagentUsagePending[taskID]
	if observation.cumulativeTotal > current.cumulativeTotal {
		current.cumulativeTotal = observation.cumulativeTotal
		current.observedAt = observation.observedAt
	}
	if observation.terminal && !current.terminal {
		current.observedAt = observation.observedAt
	}
	if current.observedAt.IsZero() {
		current.observedAt = observation.observedAt
	}
	current.terminal = current.terminal || observation.terminal
	current.terminalTokenUsageObserved =
		current.terminalTokenUsageObserved || observation.terminalTokenUsageObserved
	slot.mutable.goal.subagentUsagePending[taskID] = current
	slot.mutable.goal.mu.Unlock()
}

// clearSubagentUsagePending 只确认不晚于 settledTotal 的 pending。旧请求成功返回时，
// 若同 task 已到达更大的累计值，则必须保留新值给 retry worker 重放。
func (slot *activeRoomSlot) clearSubagentUsagePending(taskID string, settledTotal int64) {
	slot.clearSubagentUsageObservationPending(taskID, roomSubagentUsageObservation{
		cumulativeTotal:            settledTotal,
		terminal:                   true,
		terminalTokenUsageObserved: true,
	})
}

func (slot *activeRoomSlot) clearSubagentUsageObservationPending(
	taskID string,
	settled roomSubagentUsageObservation,
) {
	if slot == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	slot.mutable.goal.mu.Lock()
	taskID = strings.TrimSpace(taskID)
	if pending, ok := slot.mutable.goal.subagentUsagePending[taskID]; ok &&
		pending.cumulativeTotal <= settled.cumulativeTotal &&
		(!pending.terminal || settled.terminal) &&
		(!pending.terminalTokenUsageObserved || settled.terminalTokenUsageObserved) {
		delete(slot.mutable.goal.subagentUsagePending, taskID)
	}
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) subagentUsagePendingSnapshot() map[string]int64 {
	if slot == nil {
		return nil
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	if len(slot.mutable.goal.subagentUsagePending) == 0 {
		return nil
	}
	pending := make(map[string]int64, len(slot.mutable.goal.subagentUsagePending))
	for taskID, observation := range slot.mutable.goal.subagentUsagePending {
		pending[taskID] = observation.cumulativeTotal
	}
	return pending
}

func (slot *activeRoomSlot) subagentUsageObservationPendingSnapshot() map[string]roomSubagentUsageObservation {
	if slot == nil {
		return nil
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	if len(slot.mutable.goal.subagentUsagePending) == 0 {
		return nil
	}
	pending := make(map[string]roomSubagentUsageObservation, len(slot.mutable.goal.subagentUsagePending))
	for taskID, observation := range slot.mutable.goal.subagentUsagePending {
		pending[taskID] = observation
	}
	return pending
}

func (slot *activeRoomSlot) tryStartSubagentUsageRetry() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if slot.mutable.goal.usageRetrying ||
		len(slot.mutable.goal.subagentUsagePending) == 0 {
		return false
	}
	slot.mutable.goal.usageRetrying = true
	return true
}

// tryStartGoalUsageRetry 复用 slot 的唯一 usage worker。parent terminal
// settlement 或 shared finalization 失败时，即使没有 child pending 也必须启动。
func (slot *activeRoomSlot) tryStartGoalUsageRetry() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.Lock()
	defer slot.mutable.goal.mu.Unlock()
	if slot.mutable.goal.usageRetrying {
		return false
	}
	slot.mutable.goal.usageRetrying = true
	return true
}

func (slot *activeRoomSlot) finishSubagentUsageRetry() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.usageRetrying = false
	needsRestart := len(slot.mutable.goal.subagentUsagePending) > 0
	slot.mutable.goal.mu.Unlock()
	return needsRestart
}

func (slot *activeRoomSlot) setSubagentTasks(tasks map[string]struct{}) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.subagentTasks = tasks
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) markGoalToolProgress() {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.toolProgress = true
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) hasGoalToolProgress() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.toolProgress
}

func (slot *activeRoomSlot) goalContext() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.context
}

func (slot *activeRoomSlot) goalIDForUsage() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.idForUsage
}

func (slot *activeRoomSlot) childGoalIDForUsage() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	if goalID := strings.TrimSpace(slot.mutable.goal.childIDForUsage); goalID != "" {
		return goalID
	}
	return strings.TrimSpace(slot.mutable.goal.idForUsage)
}

func (slot *activeRoomSlot) goalSessionKey() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.sessionKey
}

func (slot *activeRoomSlot) setGoalContext(contextText string) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.context = contextText
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) setGoalBinding(sessionKey string, goalID string) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.sessionKey = strings.TrimSpace(sessionKey)
	slot.mutable.goal.idForUsage = strings.TrimSpace(goalID)
	slot.mutable.goal.childIDForUsage = strings.TrimSpace(goalID)
	if strings.TrimSpace(goalID) != "" {
		slot.mutable.goal.usageScopeConsumed = true
	}
	slot.mutable.goal.mu.Unlock()
}

// grantGoalMutationAuthority binds one exact Goal objective revision to this
// round. The binding is monotonic: a late retarget/guidance callback cannot
// upgrade an old round to the successor revision.
func (slot *activeRoomSlot) grantGoalMutationAuthority(
	authority roomGoalMutationAuthority,
) bool {
	if slot == nil {
		return false
	}
	authority.SessionKey = strings.TrimSpace(authority.SessionKey)
	authority.GoalID = strings.TrimSpace(authority.GoalID)
	authority.ExecutionID = strings.TrimSpace(authority.ExecutionID)
	authority.RootRoundID = strings.TrimSpace(authority.RootRoundID)
	if !authority.valid() {
		return false
	}
	slot.mutable.goal.mu.Lock()
	current := slot.mutable.goal.mutationAuthority
	if current.valid() && current != authority {
		slot.mutable.goal.mu.Unlock()
		return false
	}
	shared := slot.ensureGoalAuthorityState()
	if shared == nil || !shared.Bind(
		authority.GoalID,
		authority.ObjectiveRevision,
		authority.ExecutionID,
	) {
		slot.mutable.goal.mu.Unlock()
		return false
	}
	slot.mutable.goal.mutationAuthority = authority
	slot.mutable.goal.sessionKey = authority.SessionKey
	slot.mutable.goal.idForUsage = authority.GoalID
	slot.mutable.goal.childIDForUsage = authority.GoalID
	slot.mutable.goal.usageScopeConsumed = true
	slot.mutable.goal.mu.Unlock()
	slot.ensureGoalObjectiveRevision(authority.ObjectiveRevision)
	return true
}

func (slot *activeRoomSlot) goalMutationAuthority() roomGoalMutationAuthority {
	if slot == nil {
		return roomGoalMutationAuthority{}
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.mutationAuthority
}

// goalUsageScopeConsumed 是 slot/root scope 生命周期内的单调事实。清理或
// finalization 会关闭当前 accumulator，但不会允许同一 live scope 再消费新 Goal。
func (slot *activeRoomSlot) goalUsageScopeConsumed() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.usageScopeConsumed
}

func (slot *activeRoomSlot) setGoalRuntimeIgnored(ignored bool) {
	if slot == nil {
		return
	}
	slot.mutable.goal.mu.Lock()
	slot.mutable.goal.runtimeIgnored = ignored
	slot.mutable.goal.mu.Unlock()
}

func (slot *activeRoomSlot) goalRuntimeIgnored() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.mu.RLock()
	defer slot.mutable.goal.mu.RUnlock()
	return slot.mutable.goal.runtimeIgnored
}

func metadataLooksLikeSubagentTask(metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	taskType := strings.ToLower(strings.TrimSpace(anyString(metadata["task_type"])))
	if taskType == "local_shell" {
		return false
	}
	if taskType != "" {
		return taskType == "local_agent"
	}
	return strings.TrimSpace(anyString(metadata["agent_id"])) != "" ||
		strings.TrimSpace(anyString(metadata["agent_type"])) != ""
}

func isTerminalSubagentTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "error", "stopped", "killed", "cancelled":
		return true
	default:
		return false
	}
}
