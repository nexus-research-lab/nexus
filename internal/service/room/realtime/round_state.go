// INPUT: Room round/slot 生命周期、运行时消息与并发状态变更。
// OUTPUT: 可并发读取的执行状态、Goal objective revision、游标、用量与最终回复快照。
// POS: Room 实时执行过程的内存状态模型。
package realtime

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
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

// roomSlotGoalState 负责 Goal accounting、objective fencing 与协作进度。
type roomSlotGoalState struct {
	mu                 sync.RWMutex
	sessionKey         string
	context            string
	idForUsage         string
	objectiveRevision  atomic.Int64
	runtimeIgnored     bool
	usage              *goalsvc.RuntimeUsageAccumulator
	usageStartedAt     time.Time
	lastAssistant      protocol.Message
	toolProgress       bool
	subagentTasks      map[string]struct{}
	subagentHistory    bool
	resultUsageWritten bool
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
	RoomSessionID      string
	AgentID            string
	AgentRoundID       string
	MsgID              string
	RuntimeSessionKey  string
	WorkspacePath      string
	Index              int
	TimestampMS        int64
	Trigger            roomTrigger
	TriggerAttachments []protocol.ChatAttachment
	mutable            roomSlotMutableState
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
	SessionKey            string
	RoomID                string
	ConversationID        string
	RoomType              string
	Context               *protocol.ConversationContextAggregate
	RoundID               string
	RootRoundID           string
	registrationSequence  uint64
	HopIndex              int
	OwnerUserID           string
	Internal              bool
	InputOptions          sdkprotocol.OutboundMessageOptions
	Cancel                context.CancelFunc
	PermissionMode        sdkpermission.Mode
	PermissionHandler     sdkpermission.Handler
	EventObserver         RoomEventObserver
	GoalContext           string
	GoalID                string
	GoalObjectiveRevision int64
	Slots                 map[string]*activeRoomSlot
	RunningSubagents      atomic.Bool
	Done                  chan struct{}
	doneOnce              sync.Once
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
}

type roomQueuedInput struct {
	RoundID string
	Content string
}
