// INPUT: Room round/slot 生命周期、运行时消息与并发状态变更。
// OUTPUT: 可并发读取的执行状态、Goal objective revision、游标、用量与最终回复快照。
// POS: Room 实时执行过程的内存状态模型。
package room

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

type activeRoomSlot struct {
	RoomSessionID          string
	SDKSessionID           string
	AgentID                string
	AgentRoundID           string
	MsgID                  string
	RuntimeSessionKey      string
	GoalSessionKey         string
	GoalContext            string
	GoalIDForUsage         string
	goalObjectiveRevision  *atomic.Int64
	GoalRuntimeIgnored     bool
	GoalUsage              *goalsvc.RuntimeUsageAccumulator
	GoalUsageStartedAt     time.Time
	GoalLastAssistant      protocol.Message
	GoalToolProgress       bool
	SubagentTasks          map[string]struct{}
	SubagentHistory        bool
	resultUsageWritten     bool
	WorkspacePath          string
	RuntimeKind            string
	ContextWindow          int
	ContextColdStart       bool
	Client                 runtimectx.Client
	Cancel                 context.CancelFunc
	Status                 string
	Index                  int
	TimestampMS            int64
	Trigger                roomTrigger
	TriggerAttachments     []protocol.ChatAttachment
	PublicCursorID         string
	PublicCursorTS         int64
	MessageCursorID        string
	MessageCursorTS        int64
	ReplyRoute             protocol.RoomReplyRoute
	ReplySourceMessage     string
	ReplySourceAgent       string
	HandoffID              string
	InterruptReason        string
	QueuedInputs           []roomQueuedInput
	GuidedInputs           []roomQueuedInput
	SuppressOutput         bool
	PublicMessagePublished bool
	NoReplyCandidate       bool
	PendingStream          []protocol.EventMessage
	Done                   chan struct{}
	stateMu                sync.RWMutex
	inputMu                sync.Mutex
	doneOnce               sync.Once
}

func (s *activeRoomSlot) ensureGoalObjectiveRevision(initial int64) *atomic.Int64 {
	if s == nil {
		return nil
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.goalObjectiveRevision == nil {
		s.goalObjectiveRevision = &atomic.Int64{}
		s.goalObjectiveRevision.Store(initial)
	}
	return s.goalObjectiveRevision
}

func (s *activeRoomSlot) currentGoalObjectiveRevision() int64 {
	if s == nil {
		return 0
	}
	s.stateMu.RLock()
	state := s.goalObjectiveRevision
	s.stateMu.RUnlock()
	if state == nil {
		return 0
	}
	return state.Load()
}

func (s *activeRoomSlot) adoptGoalObjectiveRevision(revision int64) {
	if revision <= 0 {
		return
	}
	state := s.ensureGoalObjectiveRevision(revision)
	for state != nil {
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
	PublicMentions        []publicMentionWake
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
