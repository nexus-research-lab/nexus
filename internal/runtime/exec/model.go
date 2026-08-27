package exec

import (
	"errors"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// exec 复用 runtime 的最小 SDK 能力与隐藏上下文，避免双向依赖。
type (
	Client               = runtimectx.Client
	ContextualInputBlock = runtimectx.ContextualInputBlock
)

var (
	// ErrRoundInterrupted 表示 round 在收到终态前被中断。
	ErrRoundInterrupted = errors.New("round interrupted")
	// ErrRoundStreamClosedBeforeTerminal 表示 SDK 在产出终态前提前结束消息流。
	ErrRoundStreamClosedBeforeTerminal = errors.New("round stream closed before terminal")
	// ErrRoundStreamIdleTimeout 表示 SDK 消息流长时间无新事件且未结束。
	ErrRoundStreamIdleTimeout = errors.New("round stream idle timeout")
)

// RoundMapResult 表示单条 SDK 消息映射后的统一结果。
type RoundMapResult struct {
	Events          []protocol.EventMessage
	DurableMessages []protocol.Message
	TerminalStatus  string
	ResultSubtype   string
}

// RoundMapper 负责把 SDK 消息映射成统一事件与 durable 消息。
type RoundMapper interface {
	Map(sdkprotocol.ReceivedMessage, ...string) (RoundMapResult, error)
	SessionID() string
}

// RoundExecutionRequest 表示执行单轮查询所需的回调与依赖。
type RoundExecutionRequest struct {
	Query            string
	Content          any
	AtomicInput      bool
	ContextualInputs []ContextualInputBlock
	InputOptions     sdkprotocol.OutboundMessageOptions
	Client           Client
	Mapper           RoundMapper
	IdleTimeout      time.Duration

	// IdlePauseState 把人工交互等已知阻塞阶段从消息流空闲检测中剔除。
	// changed 关闭后调用方会重新读取状态，状态恢复时重新获得完整 IdleTimeout。
	IdlePauseState             func() (paused bool, changed <-chan struct{})
	InterruptReason            func() string
	InterruptedTerminalTimeout time.Duration
	AssistantTerminalGrace     time.Duration
	SyncSessionID              func(string) error
	AfterQuery                 func() error
	HandleDurableMessage       func(protocol.Message) error
	EmitEvent                  func(protocol.EventMessage) error
	ObserveIncomingMessage     func(sdkprotocol.ReceivedMessage)
}

// RoundExecutionResult 表示 round 执行的终态结果。
type RoundExecutionResult struct {
	TerminalStatus       string
	ResultSubtype        string
	ErrorMessage         string
	TerminalCategory     sdkprotocol.TerminalCategory
	Usage                sdkprotocol.TokenUsage
	ElapsedTimeSeconds   int64
	CompletedByAssistant bool
	UsageLimitReached    bool
	UsageLimitReason     string
}
