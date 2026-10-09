// INPUT: 已启动的 runtime client、本轮输入、消息映射器与宿主的落盘/广播/身份同步回调。
// OUTPUT: DM 与 Room 共用的单 Agent round 执行骨架。
// POS: 宿主决定输入内容、消息落点与输出可见性；这里只负责执行编排、子智能体准入、观察与 fork 收口。
package runtimehost

import (
	"context"
	"errors"
	"log/slog"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/exec"
	"github.com/nexus-research-lab/nexus/internal/runtime/trace"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	orchestrationruntimehook "github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
)

// AgentRound 描述一次单 Agent round；回调由宿主实现，字段为空的回调不参与执行。
type AgentRound struct {
	// Actor 返回当前执行身份；round 内 Goal/Execution 绑定变化后应返回最新值。
	Actor             func() orchestrationsvc.ActorContext
	RuntimeSessionKey string
	// HookRoundID 是子智能体准入回调的登记键；AgentRoundID 标识本 Agent 的执行轮次。
	HookRoundID   string
	AgentRoundID  string
	RoomSessionID string
	Client        runtimectx.Client
	Mapper        RoundMapper
	Content       any
	AtomicInput   bool
	// ContextualInputs 跟在执行上下文块之后注入。
	ContextualInputs []runtimectx.ContextualInputBlock
	InputOptions     sdkprotocol.OutboundMessageOptions
	InterruptReason  func() string
	AfterQuery       func() error
	// ObserveMessage 在共用观察之后接收每条 SDK 消息。
	ObserveMessage       func(sdkprotocol.ReceivedMessage)
	SyncSessionID        func(string) error
	HandleDurableMessage func(protocol.Message) error
	EmitEvent            func(protocol.EventMessage) error
	// ForkPending 报告 fork 后的独立 SDK session 是否仍未提交。
	ForkPending func() bool
	Logger      *slog.Logger
	// StreamLogger 记录 SDK 消息 debug 日志；为 nil 时不记录。
	StreamLogger *slog.Logger
}

// ExecuteAgentRound 执行一轮：注入执行上下文、登记子智能体准入、驱动 runtime 并在结束时
// 收口 fork 与执行观察。fork 未提交独立 session 时本轮失败并回收该 runtime。
func (h *Host) ExecuteAgentRound(ctx context.Context, round AgentRound) (exec.RoundExecutionResult, error) {
	actor := round.Actor()
	executionInputs, err := h.ExecutionContextualInputs(ctx, actor)
	if err != nil {
		return exec.RoundExecutionResult{}, err
	}
	if h.SubagentAdmission != nil {
		h.Runtime.SetSubagentHookCallbacks(
			round.RuntimeSessionKey,
			round.HookRoundID,
			orchestrationruntimehook.Callbacks(h.SubagentAdmission, orchestrationruntimehook.Context{
				Actor:             actor,
				ActorProvider:     round.Actor,
				RuntimeSessionKey: round.RuntimeSessionKey,
				RoomSessionID:     round.RoomSessionID,
				Logger:            h.LoggerFor(ctx),
			}),
		)
		defer h.Runtime.ClearSubagentHookCallbacks(round.RuntimeSessionKey, round.HookRoundID)
	}
	observer := h.ExecutionObserver()
	observer.Begin(actor)
	result, executeErr := exec.ExecuteRound(ctx, exec.RoundExecutionRequest{
		Content:          round.Content,
		AtomicInput:      round.AtomicInput,
		ContextualInputs: append(executionInputs, round.ContextualInputs...),
		InputOptions:     round.InputOptions,
		Client:           round.Client,
		Mapper:           round.Mapper,
		IdleTimeout:      h.Config.RuntimeRoundIdleTimeout(),
		IdlePauseState: func() (bool, <-chan struct{}) {
			return h.Permission.PendingRequestState(round.RuntimeSessionKey)
		},
		InterruptReason: round.InterruptReason,
		AfterQuery:      round.AfterQuery,
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			current := round.Actor()
			observer.ObserveMessage(current, incoming)
			observer.ObserveCompactBoundary(current, round.RuntimeSessionKey, round.AgentRoundID, incoming)
			if round.ObserveMessage != nil {
				round.ObserveMessage(incoming)
			}
			h.logSDKMessage(ctx, round.StreamLogger, incoming)
		},
		SyncSessionID:        round.SyncSessionID,
		HandleDurableMessage: round.HandleDurableMessage,
		EmitEvent:            round.EmitEvent,
	})
	if executeErr == nil && round.ForkPending() {
		executeErr = errors.New("runtime fork 未提交可恢复的独立 SDK session")
	}
	if executeErr != nil && round.ForkPending() {
		h.CloseUncommittedForkRuntime(round.RuntimeSessionKey, round.Client, round.Logger, executeErr)
	}
	failureReason := ""
	if executeErr != nil {
		failureReason = executeErr.Error()
	}
	observer.Finish(round.Actor(), result.TerminalStatus, failureReason)
	return result, executeErr
}

func (h *Host) logSDKMessage(ctx context.Context, logger *slog.Logger, incoming sdkprotocol.ReceivedMessage) {
	if logger == nil || !logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	if incoming.Type == sdkprotocol.MessageTypeStreamEvent && !h.Config.MessageDebugStreamEvent {
		return
	}
	fields := trace.BuildSDKMessageLogFieldsWithOptions(incoming, trace.SDKMessageLogOptions{
		IncludeStreamEvent:  h.Config.MessageDebugStreamEvent,
		IncludeSnapshotData: true,
	})
	if len(fields) > 0 {
		logger.Debug("Agent 收到 SDK 消息", fields...)
	}
}

// ConfirmsGuidance 判断该持久消息是否证明 runtime 已消费本轮引导输入：assistant 消息或成功的 result。
func ConfirmsGuidance(message protocol.Message) bool {
	switch protocol.MessageRole(message) {
	case "assistant":
		return true
	case "result":
		subtype := textutil.AnyString(message["subtype"])
		return message["is_error"] != true && (subtype == "" || subtype == "success")
	}
	return false
}

// IsGuidanceHookEvent 判断 hook 事件是否可以注入引导输入；只有 PostToolUse（或未标注事件）可以。
func IsGuidanceHookEvent(input sdkhook.Input) bool {
	return input.EventName == "" || input.EventName == sdkhook.EventPostToolUse
}

// GuidanceHookOutput 把引导输入作为 PostToolUse 附加上下文返回；runtime 确认已应用后调用 onApplied。
func GuidanceHookOutput(inputs []runtimectx.GuidedInput, onApplied func()) sdkhook.Output {
	return sdkhook.Output{
		SpecificOutput: &sdkhook.SpecificOutput{
			HookEventName:     sdkhook.EventPostToolUse,
			AdditionalContext: runtimectx.FormatGuidanceAdditionalContext(inputs),
		},
		OnApplied: func(sdkhook.AppliedAck) { onApplied() },
	}
}
