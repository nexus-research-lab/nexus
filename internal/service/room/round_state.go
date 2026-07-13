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
	RoomSessionID      string
	SDKSessionID       string
	AgentID            string
	AgentRoundID       string
	MsgID              string
	RuntimeSessionKey  string
	GoalSessionKey     string
	GoalContext        string
	GoalIDForUsage     string
	GoalRuntimeIgnored bool
	GoalUsage          *goalsvc.RuntimeUsageAccumulator
	GoalUsageStartedAt time.Time
	GoalLastAssistant  protocol.Message
	GoalToolProgress   bool
	SubagentTasks      map[string]struct{}
	SubagentHistory    bool
	resultUsageWritten bool
	WorkspacePath      string
	RuntimeKind        string
	Client             runtimectx.Client
	Cancel             context.CancelFunc
	Status             string
	Index              int
	TimestampMS        int64
	Trigger            roomTrigger
	TriggerAttachments []protocol.ChatAttachment
	PublicCursorID     string
	PublicCursorTS     int64
	MessageCursorID    string
	MessageCursorTS    int64
	ReplyRoute         protocol.RoomReplyRoute
	ReplySourceMessage string
	ReplySourceAgent   string
	InterruptReason    string
	QueuedInputs       []roomQueuedInput
	GuidedInputs       []roomQueuedInput
	SuppressOutput     bool
	NoReplyCandidate   bool
	PendingStream      []protocol.EventMessage
	Done               chan struct{}
	stateMu            sync.RWMutex
	inputMu            sync.Mutex
	doneOnce           sync.Once
}

type activeRoomRound struct {
	SessionKey        string
	RoomID            string
	ConversationID    string
	RoomType          string
	Context           *protocol.ConversationContextAggregate
	RoundID           string
	RootRoundID       string
	HopIndex          int
	OwnerUserID       string
	Internal          bool
	InputOptions      sdkprotocol.OutboundMessageOptions
	Cancel            context.CancelFunc
	PermissionMode    sdkpermission.Mode
	PermissionHandler sdkpermission.Handler
	EventObserver     RoomEventObserver
	GoalContext       string
	Slots             map[string]*activeRoomSlot
	PublicMentions    []publicMentionWake
	RunningSubagents  atomic.Bool
	Done              chan struct{}
	doneOnce          sync.Once
}

type roomTrigger = roomdomain.Trigger

type publicMentionWake struct {
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
