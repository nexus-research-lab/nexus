// INPUT: Room round/slot 生命周期、原子 Slash 输入、structured WorkBinding/ReviewBinding、运行时消息与并发状态变更。
// OUTPUT: 固定权限世代、trusted dispatch identity、稳定 owner/root usage scope、原子 runtime 输入、Work/Goal 绑定、结算屏障、游标、工具回执与最终回复快照。
// POS: Room 实时执行过程的内存状态与权限能力模型。
package realtime

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

// roomSlotRuntimeState 只负责 runtime 生命周期，不持有 Goal 或 delivery 数据。
type roomSlotRuntimeState struct {
	mu                  sync.RWMutex
	sdkSessionID        string
	runtimeKind         string
	contextWindow       int
	contextColdStart    bool
	client              runtimectx.Client
	cancel              context.CancelFunc
	status              string
	interruptReason     string
	errorMessage        string
	done                chan struct{}
	doneOnce            sync.Once
	responsibilityOnce  sync.Once
	responsibilityState *runtimectx.ResponsibilityAuthorityState
	sdkIdentityOnce     sync.Once
	sdkSessionIdentity  *runtimectx.SDKSessionIdentityState
	commandReceiptOnce  sync.Once
	commandReceipts     *nexusmcp.CommandReceiptState
	workBindingOnce     sync.Once
	workBindingState    *runtimectx.WorkBindingState
}

func (s *activeRoomSlot) ensureCommandReceiptState() *nexusmcp.CommandReceiptState {
	if s == nil {
		return nil
	}
	runtimeState := &s.mutable.runtime
	runtimeState.commandReceiptOnce.Do(func() {
		runtimeState.commandReceipts = nexusmcp.NewCommandReceiptState()
	})
	return runtimeState.commandReceipts
}

func (s *activeRoomSlot) ensureSDKSessionIdentityState() *runtimectx.SDKSessionIdentityState {
	if s == nil {
		return nil
	}
	runtimeState := &s.mutable.runtime
	runtimeState.sdkIdentityOnce.Do(func() {
		runtimeState.sdkSessionIdentity = runtimectx.NewSDKSessionIdentityState(
			s.getSDKSessionID(),
		)
	})
	return runtimeState.sdkSessionIdentity
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

func goalCollaborationBindingForSlot(
	_ *activeRoomRound,
	slot *activeRoomSlot,
) *protocol.GoalCollaborationBinding {
	if slot == nil {
		return nil
	}
	fixed := slot.goalMutationAuthority()
	if fixed.valid() {
		// Goal mutation authority fixes the physical round to one Goal identity,
		// while its shared state advances only after a trusted retarget or
		// consumed host steering. Collaboration attribution is not mutation
		// authority, so snapshot the current exact revision without rewriting
		// the immutable round capability.
		shared, ok := slot.boundGoalAuthority()
		if !ok || shared.GoalID != fixed.GoalID ||
			shared.ObjectiveRevision < fixed.ObjectiveRevision {
			return nil
		}
		return &protocol.GoalCollaborationBinding{
			GoalID:            fixed.GoalID,
			ObjectiveRevision: shared.ObjectiveRevision,
		}
	}
	return slot.goalCollaborationBinding()
}

func cloneGoalCollaborationBinding(
	binding *protocol.GoalCollaborationBinding,
) *protocol.GoalCollaborationBinding {
	return protocol.NormalizeGoalCollaborationBinding(binding)
}

// roomSlotGoalState 负责 Goal accounting、固定起始 capability、服务端确认 revision 与协作进度。
type roomSlotGoalState struct {
	// GoalRoundState 是与 DM/Room 共用的每轮 Goal 状态；其 Mu 保护全部 Goal 字段。
	runtimehost.GoalRoundState
	sessionKey           string
	collaborationBinding *protocol.GoalCollaborationBinding
	mutationAuthority    roomGoalMutationAuthority
	authorityOnce        sync.Once
	authorityState       *runtimectx.GoalAuthorityState
	objectiveRevision    atomic.Int64
	runtimeIgnored       bool
	pendingCollaboration bool
	subagentHistory      bool
	terminalSettled      bool
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
	mu                          sync.Mutex
	replyRoute                  protocol.RoomReplyRoute
	replySourceMessage          string
	handoffID                   string
	queuedInputs                []roomQueuedInput
	suppressOutput              bool
	publicMessagePublished      bool
	publicMessageReceiptPending bool
	noReplyCandidate            bool
	pendingStream               []protocol.EventMessage
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
	HiddenFromUser        bool
	QueueSource           protocol.InputQueueSource
	Trigger               roomTrigger
	TriggerAttachments    []protocol.ChatAttachment
	AtomicRuntimeInput    string
	WorkBinding           *protocol.ExecutionWorkBinding
	ReviewBinding         *protocol.ExecutionReviewBinding
	mutable               roomSlotMutableState
}

func (s *activeRoomSlot) ensureWorkBindingState() *runtimectx.WorkBindingState {
	if s == nil {
		return nil
	}
	runtimeState := &s.mutable.runtime
	runtimeState.workBindingOnce.Do(func() {
		runtimeState.workBindingState = runtimectx.NewWorkBindingStateFromResponsibility(
			s.ensureResponsibilityAuthorityState(),
		)
	})
	return runtimeState.workBindingState
}

func (s *activeRoomSlot) ensureResponsibilityAuthorityState() *runtimectx.ResponsibilityAuthorityState {
	if s == nil {
		return nil
	}
	runtimeState := &s.mutable.runtime
	runtimeState.responsibilityOnce.Do(func() {
		runtimeState.responsibilityState = runtimectx.NewResponsibilityAuthorityState(
			s.ensureGoalAuthorityState(),
			executionIDFromRoomBindings(s.WorkBinding, s.ReviewBinding),
			s.WorkBinding,
			s.ReviewBinding,
		)
	})
	return runtimeState.responsibilityState
}

func (s *activeRoomSlot) currentWorkBinding() *protocol.ExecutionWorkBinding {
	state := s.ensureWorkBindingState()
	if state == nil {
		return nil
	}
	binding, _ := state.Load()
	return binding
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

func (s *activeRoomSlot) boundGoalAuthority() (runtimectx.GoalAuthority, bool) {
	if s == nil {
		return runtimectx.GoalAuthority{}, false
	}
	state := s.ensureResponsibilityAuthorityState()
	if state == nil {
		return runtimectx.GoalAuthority{}, false
	}
	return state.LoadGoalAuthority()
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
	PublicContext               []protocol.Message
	PublicAgentDirectory        map[string]string
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
	RuntimeToolPolicy           *protocol.RuntimeToolPolicy
	AutomationRun               *protocol.AutomationRunContext
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
	HandoffID                string
	TriggerType              string
	QueueSource              protocol.InputQueueSource
	SourceAgentID            string
	TargetAgentID            string
	Content                  string
	MessageID                string
	ReplyRoute               protocol.RoomReplyRoute
	GoalCollaborationBinding *protocol.GoalCollaborationBinding
	WorkBinding              *protocol.ExecutionWorkBinding
	ReviewBinding            *protocol.ExecutionReviewBinding
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
	return slot.mutable.runtime.errorMessage
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
	if sessionID == "" || sessionID == strings.TrimSpace(slot.mutable.runtime.sdkSessionID) {
		slot.mutable.runtime.mu.Unlock()
		return false
	}
	slot.mutable.runtime.sdkSessionID = sessionID
	slot.mutable.runtime.mu.Unlock()
	slot.ensureSDKSessionIdentityState().Set(sessionID)
	return true
}

func (slot *activeRoomSlot) clearSDKSessionID() bool {
	if slot == nil {
		return false
	}
	slot.mutable.runtime.mu.Lock()
	if strings.TrimSpace(slot.mutable.runtime.sdkSessionID) == "" {
		slot.mutable.runtime.mu.Unlock()
		return false
	}
	slot.mutable.runtime.sdkSessionID = ""
	slot.mutable.runtime.mu.Unlock()
	slot.ensureSDKSessionIdentityState().Set("")
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
	return slot.mutable.runtime.runtimeKind
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
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.Usage = goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(slot.mutable.goal.IDForUsage) != "")
	slot.mutable.goal.UsageStartedAt = time.Now()
	slot.mutable.goal.terminalSettled = false
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) resetGoalUsage(snapshot goalsvc.RuntimeUsageSnapshot) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	if slot.mutable.goal.Usage == nil {
		slot.mutable.goal.Usage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	slot.mutable.goal.Usage.Reset(snapshot)
	slot.mutable.goal.terminalSettled = false
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageActiveForGoal(goalID string) bool {
	if slot == nil || strings.TrimSpace(goalID) == "" {
		return false
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return strings.TrimSpace(slot.mutable.goal.IDForUsage) == strings.TrimSpace(goalID) &&
		slot.mutable.goal.Usage != nil &&
		slot.mutable.goal.Usage.Active()
}

func (slot *activeRoomSlot) closeGoalUsage() {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	if slot.mutable.goal.Usage != nil {
		slot.mutable.goal.Usage.Close()
	}
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) clearGoalUsage() {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	if slot.mutable.goal.Usage != nil {
		slot.mutable.goal.Usage.Close()
	}
	slot.mutable.goal.IDForUsage = ""
	slot.mutable.goal.ChildIDForUsage = ""
	slot.mutable.goal.mutationAuthority = roomGoalMutationAuthority{}
	slot.mutable.goal.UsageClaimPending = false
	slot.mutable.goal.terminalSettled = false
	if authority := slot.ensureResponsibilityAuthorityState(); authority != nil {
		authority.ClearGoalAuthority()
	}
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) setGoalUsageTerminalSettled(settled bool) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.terminalSettled = settled
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageTerminalSettled() bool {
	if slot == nil {
		return true
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.terminalSettled
}

func (slot *activeRoomSlot) goalUsageSettlementRequired() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.Usage != nil ||
		strings.TrimSpace(slot.mutable.goal.IDForUsage) != "" ||
		strings.TrimSpace(slot.mutable.goal.ChildIDForUsage) != ""
}

func (slot *activeRoomSlot) setGoalUsageClaimPending(pending bool) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.UsageClaimPending = pending
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) goalUsageClaimPending() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.UsageClaimPending
}

func (slot *activeRoomSlot) beginGoalUsageFinalizing() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.Lock()
	defer slot.mutable.goal.Mu.Unlock()
	if slot.mutable.goal.Usage == nil ||
		!slot.mutable.goal.Usage.Active() ||
		strings.TrimSpace(slot.mutable.goal.IDForUsage) == "" {
		return false
	}
	slot.mutable.goal.Usage.BeginFinalizing()
	return true
}

func (slot *activeRoomSlot) goalUsageStartedAt() time.Time {
	if slot == nil {
		return time.Time{}
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.UsageStartedAt
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

func (slot *activeRoomSlot) markPublicMessagePublished() {
	if slot == nil {
		return
	}
	slot.mutable.delivery.mu.Lock()
	slot.mutable.delivery.publicMessagePublished = true
	slot.mutable.delivery.publicMessageReceiptPending = true
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
		if slot.mutable.delivery.publicMessageReceiptPending &&
			event.EventType == protocol.EventTypeMessage &&
			len(messagepkg.AssistantToolResults(protocol.Message(event.Data))) > 0 {
			slot.mutable.delivery.publicMessageReceiptPending = false
			event.DeliveryMode = protocol.DeliveryModeEphemeral
			return []protocol.EventMessage{event}
		}
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

func (slot *activeRoomSlot) consumeRuntimeCommandReceipts() []nexusmcp.CommandReceipt {
	return slot.mutable.goal.ConsumeCommandReceipts(slot.ensureCommandReceiptState())
}

func (slot *activeRoomSlot) markGoalCompletionCandidate(goalID string) {
	if slot == nil || strings.TrimSpace(goalID) == "" {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.CompletionCandidateID = strings.TrimSpace(goalID)
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) rememberSubagentTaskMessage(message protocol.Message) {
	if slot == nil || !slot.mutable.goal.RememberSubagentTaskMessage(message) {
		return
	}
	runtimeKind := slot.runtimeKind()
	slot.mutable.goal.Mu.Lock()
	defer slot.mutable.goal.Mu.Unlock()
	if metadata, _ := message["metadata"].(map[string]any); metadata != nil && runtimeKind != "" {
		metadata["runtime_kind"] = runtimeKind
	}
	slot.mutable.goal.subagentHistory = true
}

func (slot *activeRoomSlot) hasSubagentHistory() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.subagentHistory
}

func (slot *activeRoomSlot) subagentUsagePendingSnapshot() map[string]int64 {
	if slot == nil {
		return nil
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	if len(slot.mutable.goal.SubagentUsagePending) == 0 {
		return nil
	}
	pending := make(map[string]int64, len(slot.mutable.goal.SubagentUsagePending))
	for taskID, observation := range slot.mutable.goal.SubagentUsagePending {
		pending[taskID] = observation.CumulativeTotal
	}
	return pending
}

func (slot *activeRoomSlot) subagentUsageObservationPendingSnapshot() map[string]goalsvc.SubagentUsageObservation {
	if slot == nil {
		return nil
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	if len(slot.mutable.goal.SubagentUsagePending) == 0 {
		return nil
	}
	pending := make(map[string]goalsvc.SubagentUsageObservation, len(slot.mutable.goal.SubagentUsagePending))
	for taskID, observation := range slot.mutable.goal.SubagentUsagePending {
		pending[taskID] = observation
	}
	return pending
}

func (slot *activeRoomSlot) tryStartSubagentUsageRetry() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.Lock()
	defer slot.mutable.goal.Mu.Unlock()
	if slot.mutable.goal.UsageRetrying ||
		len(slot.mutable.goal.SubagentUsagePending) == 0 {
		return false
	}
	slot.mutable.goal.UsageRetrying = true
	return true
}

// tryStartGoalUsageRetry 复用 slot 的唯一 usage worker。parent terminal
// settlement 或 shared finalization 失败时，即使没有 child pending 也必须启动。
func (slot *activeRoomSlot) tryStartGoalUsageRetry() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.Lock()
	defer slot.mutable.goal.Mu.Unlock()
	if slot.mutable.goal.UsageRetrying {
		return false
	}
	slot.mutable.goal.UsageRetrying = true
	return true
}

func (slot *activeRoomSlot) finishSubagentUsageRetry() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.UsageRetrying = false
	needsRestart := len(slot.mutable.goal.SubagentUsagePending) > 0
	slot.mutable.goal.Mu.Unlock()
	return needsRestart
}

func (slot *activeRoomSlot) markPendingGoalCollaboration() {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.pendingCollaboration = true
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) hasPendingGoalCollaboration() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.pendingCollaboration
}

func (slot *activeRoomSlot) clearPendingGoalCollaboration() {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.pendingCollaboration = false
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) hasGoalToolProgress() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.ToolProgress
}

func (slot *activeRoomSlot) goalContext() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.Context
}

func (slot *activeRoomSlot) childGoalIDForUsage() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	if goalID := strings.TrimSpace(slot.mutable.goal.ChildIDForUsage); goalID != "" {
		return goalID
	}
	return strings.TrimSpace(slot.mutable.goal.IDForUsage)
}

func (slot *activeRoomSlot) goalSessionKey() string {
	if slot == nil {
		return ""
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.sessionKey
}

func (slot *activeRoomSlot) setGoalContext(contextText string) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.Context = contextText
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) setGoalBinding(sessionKey string, goalID string) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.sessionKey = strings.TrimSpace(sessionKey)
	slot.mutable.goal.IDForUsage = strings.TrimSpace(goalID)
	slot.mutable.goal.ChildIDForUsage = strings.TrimSpace(goalID)
	if strings.TrimSpace(goalID) != "" {
		slot.mutable.goal.UsageScopeConsumed = true
	}
	slot.mutable.goal.Mu.Unlock()
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
	if !authority.valid() {
		return false
	}
	slot.mutable.goal.Mu.Lock()
	current := slot.mutable.goal.mutationAuthority
	if current.valid() && current != authority {
		slot.mutable.goal.Mu.Unlock()
		return false
	}
	shared := slot.ensureResponsibilityAuthorityState()
	if shared == nil || !shared.GrantGoalAuthority(
		authority.GoalID,
		authority.ObjectiveRevision,
		authority.ExecutionID,
	) {
		slot.mutable.goal.Mu.Unlock()
		return false
	}
	slot.mutable.goal.mutationAuthority = authority
	slot.mutable.goal.sessionKey = authority.SessionKey
	slot.mutable.goal.IDForUsage = authority.GoalID
	slot.mutable.goal.ChildIDForUsage = authority.GoalID
	slot.mutable.goal.UsageScopeConsumed = true
	slot.mutable.goal.Mu.Unlock()
	slot.ensureGoalObjectiveRevision(authority.ObjectiveRevision)
	return true
}

func (slot *activeRoomSlot) goalMutationAuthority() roomGoalMutationAuthority {
	if slot == nil {
		return roomGoalMutationAuthority{}
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.mutationAuthority
}

func (slot *activeRoomSlot) setGoalCollaborationBinding(
	binding *protocol.GoalCollaborationBinding,
) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.collaborationBinding = cloneGoalCollaborationBinding(binding)
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) goalCollaborationBinding() *protocol.GoalCollaborationBinding {
	if slot == nil {
		return nil
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return cloneGoalCollaborationBinding(slot.mutable.goal.collaborationBinding)
}

func (slot *activeRoomSlot) setGoalRuntimeIgnored(ignored bool) {
	if slot == nil {
		return
	}
	slot.mutable.goal.Mu.Lock()
	slot.mutable.goal.runtimeIgnored = ignored
	slot.mutable.goal.Mu.Unlock()
}

func (slot *activeRoomSlot) goalRuntimeIgnored() bool {
	if slot == nil {
		return false
	}
	slot.mutable.goal.Mu.RLock()
	defer slot.mutable.goal.Mu.RUnlock()
	return slot.mutable.goal.runtimeIgnored
}
