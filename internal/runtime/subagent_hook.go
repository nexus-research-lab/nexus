// INPUT: 当前 runtime session 的权威 subagent hook callbacks 与 bridge hook 事件。
// OUTPUT: Agent PreToolUse 强准入、SubagentStart/Stop/failure 转发与 parent-exit durable grace schedule。
// POS: warm SDK session 与每轮 actor-specific Execution policy 之间的动态 hook 路由。
package runtime

import (
	"context"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	subagentHookTimeout            = 5 * time.Second
	subagentHookUnavailableCode    = "subagent_admission_unavailable"
	subagentHookUnavailableMessage = "authoritative Execution admission context is unavailable"
	subagentHookCorrelationCode    = "subagent_lifecycle_correlation_required"
	subagentHookAmbiguousCode      = "subagent_lifecycle_ambiguous"
)

// SubagentHookCallbacks 由当前 DM/Room round 注入；warm client 的 hook 只做动态路由。
type SubagentHookCallbacks struct {
	PreToolUse         sdkhook.Callback
	PostToolUseFailure sdkhook.Callback
	SubagentStart      sdkhook.Callback
	SubagentStop       sdkhook.Callback
	ParentRoundExit    SubagentRoundExitCallback
}

// SubagentRoundExitInput 是 parent physical round 退出时持久化 grace deadline
// 所需的 immutable child correlation。
type SubagentRoundExitInput struct {
	ToolUseID           string
	SDKSessionID        string
	SDKAgentID          string
	SDKTaskID           string
	ParentRoundExitedAt time.Time
	ReconcileAfter      time.Time
}

// SubagentRoundExitCallback 把 runtime-local round exit 转成 durable recovery intent。
type SubagentRoundExitCallback func(context.Context, SubagentRoundExitInput) error

type subagentHookBinding struct {
	Sequence     uint64
	RoundID      string
	ToolUseID    string
	SDKSessionID string
	SDKAgentID   string
	SDKTaskID    string
	Started      bool
	Terminal     bool
	Callbacks    SubagentHookCallbacks
}

// SetSubagentHookCallbacks 注册一个物理 round 的 actor-specific callbacks。
// 多个 round 同时注册时，缺少 round/tool correlation 的 Agent launch 会 fail closed。
func (m *Manager) SetSubagentHookCallbacks(
	sessionKey string,
	roundID string,
	callbacks SubagentHookCallbacks,
) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	state := m.ensureStateLocked(sessionKey)
	state.SubagentHooks[roundID] = callbacks
	m.touchStateLocked(state)
	m.mu.Unlock()
}

// ClearSubagentHookCallbacks 移除已经退出的 parent round launch capability。
// PreToolUse 已创建的 immutable lifecycle binding 会继续保留，供迟到 Stop 收口；
// bridge 在 grace period 内始终不返回终态时，冻结的 callback 会把 child Attempt
// 作为 interrupted 收束，避免永久占住 Assignment 的 subagent admission。
func (m *Manager) ClearSubagentHookCallbacks(sessionKey string, roundID string) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	state := m.sessions[sessionKey]
	pending := make([]subagentHookBinding, 0, 1)
	if state != nil {
		delete(state.SubagentHooks, roundID)
		for _, binding := range state.SubagentHookBindings {
			if binding.RoundID == roundID && !binding.Terminal {
				pending = append(pending, binding)
			}
		}
		m.touchStateLocked(state)
	}
	m.mu.Unlock()
	parentRoundExitedAt := time.Now().UTC()
	reconcileAfter := parentRoundExitedAt.Add(protocol.SubagentReconciliationGrace)
	for _, binding := range pending {
		binding := binding
		durableScheduled := false
		if callback := binding.Callbacks.ParentRoundExit; callback != nil {
			ctx, cancel := context.WithTimeout(context.Background(), subagentHookTimeout)
			err := callback(ctx, SubagentRoundExitInput{
				ToolUseID:           binding.ToolUseID,
				SDKSessionID:        binding.SDKSessionID,
				SDKAgentID:          binding.SDKAgentID,
				SDKTaskID:           binding.SDKTaskID,
				ParentRoundExitedAt: parentRoundExitedAt,
				ReconcileAfter:      reconcileAfter,
			})
			cancel()
			durableScheduled = err == nil
		}
		if !durableScheduled {
			m.expireSubagentHookBinding(sessionKey, binding, 1)
			continue
		}
		delay := time.Until(reconcileAfter)
		if delay < 0 {
			delay = 0
		}
		time.AfterFunc(delay, func() {
			m.expireSubagentHookBinding(sessionKey, binding, 1)
		})
	}
}

// WithSubagentAdmissionHooks 给 warm runtime client 注册稳定 hook 路由。
func (m *Manager) WithSubagentAdmissionHooks(
	options agentclient.Options,
	sessionKey string,
) agentclient.Options {
	if strings.TrimSpace(sessionKey) == "" {
		return options
	}
	sessionKey = strings.TrimSpace(sessionKey)
	hooks := cloneSDKHooks(options.Hooks.Matchers)
	hooks[sdkhook.EventPreToolUse] = append(
		hooks[sdkhook.EventPreToolUse],
		sdkhook.Matcher{
			Matcher: "Agent",
			Hooks: []sdkhook.Callback{
				m.dynamicSubagentHook(sessionKey, sdkhook.EventPreToolUse),
			},
			Timeout: subagentHookTimeout,
		},
	)
	hooks[sdkhook.EventPostToolUseFailure] = append(
		hooks[sdkhook.EventPostToolUseFailure],
		sdkhook.Matcher{
			Matcher: "Agent",
			Hooks: []sdkhook.Callback{
				m.dynamicSubagentHook(sessionKey, sdkhook.EventPostToolUseFailure),
			},
			Timeout: subagentHookTimeout,
		},
	)
	hooks[sdkhook.EventSubagentStart] = append(
		hooks[sdkhook.EventSubagentStart],
		sdkhook.Matcher{
			Hooks: []sdkhook.Callback{
				m.dynamicSubagentHook(sessionKey, sdkhook.EventSubagentStart),
			},
			Timeout: subagentHookTimeout,
		},
	)
	hooks[sdkhook.EventSubagentStop] = append(
		hooks[sdkhook.EventSubagentStop],
		sdkhook.Matcher{
			Hooks: []sdkhook.Callback{
				m.dynamicSubagentHook(sessionKey, sdkhook.EventSubagentStop),
			},
			Timeout: subagentHookTimeout,
		},
	)
	options.Hooks.Matchers = hooks
	return options
}

func (m *Manager) dynamicSubagentHook(
	sessionKey string,
	event sdkhook.Event,
) sdkhook.Callback {
	return func(ctx context.Context, input sdkhook.Input, toolUseID string) (sdkhook.Output, error) {
		if event == sdkhook.EventPreToolUse || event == sdkhook.EventPostToolUseFailure {
			if !strings.EqualFold(strings.TrimSpace(input.ToolName), "Agent") {
				return sdkhook.Output{}, nil
			}
		}
		switch event {
		case sdkhook.EventPreToolUse:
			return m.routeSubagentPreToolUse(ctx, sessionKey, input, toolUseID)
		case sdkhook.EventPostToolUseFailure:
			return m.routeSubagentLifecycle(ctx, sessionKey, event, input, firstSubagentHookValue(toolUseID, input.ToolUseID))
		case sdkhook.EventSubagentStart, sdkhook.EventSubagentStop:
			// Some bridge events expose an unrelated input.ToolUseID. Only the
			// callback correlation ID, then Task/SDK Agent identity, is trusted.
			return m.routeSubagentLifecycle(ctx, sessionKey, event, input, strings.TrimSpace(toolUseID))
		default:
			return sdkhook.Output{}, nil
		}
	}
}

func (m *Manager) routeSubagentPreToolUse(
	ctx context.Context,
	sessionKey string,
	input sdkhook.Input,
	callbackToolUseID string,
) (sdkhook.Output, error) {
	toolUseID := firstSubagentHookValue(callbackToolUseID, input.ToolUseID)
	if toolUseID == "" {
		return denySubagentHookOutput(
			sdkhook.EventPreToolUse,
			subagentHookCorrelationCode,
			"Agent launch requires a stable tool_use_id",
		), nil
	}
	m.mu.Lock()
	state := m.sessions[sessionKey]
	if state == nil || len(state.SubagentHooks) == 0 {
		m.mu.Unlock()
		return denySubagentHookOutput(sdkhook.EventPreToolUse, subagentHookUnavailableCode, subagentHookUnavailableMessage), nil
	}
	if len(state.SubagentHooks) != 1 {
		m.mu.Unlock()
		return denySubagentHookOutput(
			sdkhook.EventPreToolUse,
			subagentHookAmbiguousCode,
			"multiple runtime rounds can launch an Agent; exact parent correlation is unavailable",
		), nil
	}
	if _, exists := state.SubagentHookBindings[toolUseID]; exists {
		m.mu.Unlock()
		return denySubagentHookOutput(
			sdkhook.EventPreToolUse,
			subagentHookAmbiguousCode,
			"tool_use_id is already bound to another subagent lifecycle",
		), nil
	}
	var roundID string
	var callbacks SubagentHookCallbacks
	for candidateRoundID, candidate := range state.SubagentHooks {
		roundID = candidateRoundID
		callbacks = candidate
	}
	if callbacks.PreToolUse == nil {
		m.mu.Unlock()
		return denySubagentHookOutput(sdkhook.EventPreToolUse, subagentHookUnavailableCode, subagentHookUnavailableMessage), nil
	}
	state.NextSubagentBindingSeq++
	binding := subagentHookBinding{
		Sequence:     state.NextSubagentBindingSeq,
		RoundID:      roundID,
		ToolUseID:    toolUseID,
		SDKSessionID: strings.TrimSpace(input.SessionID),
		SDKTaskID:    strings.TrimSpace(input.TaskID),
		Callbacks:    callbacks,
	}
	state.SubagentHookBindings[toolUseID] = binding
	m.touchStateLocked(state)
	m.mu.Unlock()

	output, err := callbacks.PreToolUse(ctx, input, toolUseID)
	if err != nil || !subagentHookOutputAccepted(sdkhook.EventPreToolUse, output) {
		m.removeSubagentHookBinding(sessionKey, toolUseID, binding.Sequence)
	}
	return output, err
}

func (m *Manager) routeSubagentLifecycle(
	ctx context.Context,
	sessionKey string,
	event sdkhook.Event,
	input sdkhook.Input,
	toolUseID string,
) (sdkhook.Output, error) {
	binding, found, ambiguous := m.resolveSubagentHookBinding(
		sessionKey,
		event,
		input,
		toolUseID,
	)
	if ambiguous {
		return denySubagentHookOutput(
			event,
			subagentHookAmbiguousCode,
			"subagent lifecycle matches multiple immutable parent bindings",
		), nil
	}
	if !found {
		return denySubagentHookOutput(
			event,
			subagentHookCorrelationCode,
			"subagent lifecycle has no immutable parent binding",
		), nil
	}
	callback := subagentLifecycleCallback(binding.Callbacks, event)
	if callback == nil {
		return denySubagentHookOutput(event, subagentHookUnavailableCode, subagentHookUnavailableMessage), nil
	}
	output, err := callback(ctx, input, binding.ToolUseID)
	if err == nil && subagentHookOutputAccepted(event, output) {
		m.markSubagentHookLifecycle(sessionKey, binding, event, input)
	}
	return output, err
}

func (m *Manager) resolveSubagentHookBinding(
	sessionKey string,
	event sdkhook.Event,
	input sdkhook.Input,
	toolUseID string,
) (subagentHookBinding, bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := m.sessions[sessionKey]
	if state == nil {
		return subagentHookBinding{}, false, false
	}
	if toolUseID = strings.TrimSpace(toolUseID); toolUseID != "" {
		binding, ok := state.SubagentHookBindings[toolUseID]
		return binding, ok, false
	}
	candidates := make([]subagentHookBinding, 0, 1)
	for _, binding := range state.SubagentHookBindings {
		if !subagentHookBindingMatchesLifecycle(binding, event, input) {
			continue
		}
		candidates = append(candidates, binding)
		if len(candidates) > 1 {
			return subagentHookBinding{}, false, true
		}
	}
	if len(candidates) != 1 {
		return subagentHookBinding{}, false, false
	}
	return candidates[0], true, false
}

func subagentHookBindingMatchesLifecycle(
	binding subagentHookBinding,
	event sdkhook.Event,
	input sdkhook.Input,
) bool {
	agentID := strings.TrimSpace(input.AgentID)
	taskID := strings.TrimSpace(input.TaskID)
	if event == sdkhook.EventSubagentStart && (binding.Started || binding.Terminal) {
		// A duplicate/late Start must keep matching its frozen child identity.
		// Excluding it would let a newer unstarted binding absorb the old event.
		if (agentID == "" || binding.SDKAgentID == "" || binding.SDKAgentID != agentID) &&
			(taskID == "" || binding.SDKTaskID == "" || binding.SDKTaskID != taskID) {
			return false
		}
	}
	if event != sdkhook.EventSubagentStart && !binding.Started && event != sdkhook.EventPostToolUseFailure {
		return false
	}
	if sessionID := strings.TrimSpace(input.SessionID); sessionID != "" &&
		binding.SDKSessionID != "" && binding.SDKSessionID != sessionID {
		return false
	}
	if taskID != "" &&
		binding.SDKTaskID != "" && binding.SDKTaskID != taskID {
		return false
	}
	if agentID != "" &&
		binding.SDKAgentID != "" && binding.SDKAgentID != agentID {
		return false
	}
	if event == sdkhook.EventSubagentStop {
		if taskID != "" {
			return binding.SDKTaskID == "" || binding.SDKTaskID == taskID
		}
		if agentID != "" {
			return binding.SDKAgentID == agentID
		}
	}
	return true
}

func subagentLifecycleCallback(
	callbacks SubagentHookCallbacks,
	event sdkhook.Event,
) sdkhook.Callback {
	switch event {
	case sdkhook.EventPostToolUseFailure:
		return callbacks.PostToolUseFailure
	case sdkhook.EventSubagentStart:
		return callbacks.SubagentStart
	case sdkhook.EventSubagentStop:
		return callbacks.SubagentStop
	default:
		return nil
	}
}

func (m *Manager) markSubagentHookLifecycle(
	sessionKey string,
	binding subagentHookBinding,
	event sdkhook.Event,
	input sdkhook.Input,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state == nil {
		return
	}
	current, ok := state.SubagentHookBindings[binding.ToolUseID]
	if !ok || current.Sequence != binding.Sequence {
		return
	}
	if sessionID := strings.TrimSpace(input.SessionID); sessionID != "" {
		current.SDKSessionID = sessionID
	}
	if agentID := strings.TrimSpace(input.AgentID); agentID != "" {
		current.SDKAgentID = agentID
	}
	if taskID := strings.TrimSpace(input.TaskID); taskID != "" {
		current.SDKTaskID = taskID
	}
	if event == sdkhook.EventSubagentStart {
		current.Started = true
	}
	if event == sdkhook.EventSubagentStop || event == sdkhook.EventPostToolUseFailure {
		current.Terminal = true
	}
	state.SubagentHookBindings[binding.ToolUseID] = current
	m.touchStateLocked(state)
}

func (m *Manager) expireSubagentHookBinding(
	sessionKey string,
	binding subagentHookBinding,
	reconcileAttempt int,
) {
	m.mu.RLock()
	state := m.sessions[sessionKey]
	current, ok := subagentHookBinding{}, false
	if state != nil {
		current, ok = state.SubagentHookBindings[binding.ToolUseID]
	}
	m.mu.RUnlock()
	if !ok ||
		current.Sequence != binding.Sequence ||
		current.RoundID != binding.RoundID ||
		current.Terminal {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), subagentHookTimeout)
	output, err := m.routeSubagentLifecycle(
		ctx,
		sessionKey,
		sdkhook.EventPostToolUseFailure,
		sdkhook.Input{
			EventName:   sdkhook.EventPostToolUseFailure,
			ToolName:    "Agent",
			ToolUseID:   current.ToolUseID,
			SessionID:   current.SDKSessionID,
			AgentID:     current.SDKAgentID,
			TaskID:      current.SDKTaskID,
			IsInterrupt: true,
			Error:       "parent runtime round ended before subagent lifecycle completed",
		},
		current.ToolUseID,
	)
	cancel()
	if err == nil && subagentHookOutputAccepted(sdkhook.EventPostToolUseFailure, output) {
		return
	}
	// Keep retrying while the immutable binding remains live. A fixed retry
	// count turned a short database outage into a permanent running child in a
	// long-lived server. The delay is capped below, and a successful/late
	// terminal hook removes the binding so the next callback becomes a no-op.
	delay := protocol.SubagentReconciliationGrace << min(reconcileAttempt, 2)
	time.AfterFunc(delay, func() {
		m.expireSubagentHookBinding(sessionKey, binding, min(reconcileAttempt+1, 3))
	})
}

func (m *Manager) removeSubagentHookBinding(
	sessionKey string,
	toolUseID string,
	sequence uint64,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state == nil {
		return
	}
	current, ok := state.SubagentHookBindings[toolUseID]
	if ok && current.Sequence == sequence {
		delete(state.SubagentHookBindings, toolUseID)
	}
}

func subagentHookOutputAccepted(event sdkhook.Event, output sdkhook.Output) bool {
	if output.Continue != nil && !*output.Continue {
		return false
	}
	if strings.TrimSpace(output.StopReason) != "" {
		return false
	}
	if event == sdkhook.EventPreToolUse &&
		output.SpecificOutput != nil &&
		output.SpecificOutput.PermissionDecision == sdkpermission.BehaviorDeny {
		return false
	}
	return true
}

func firstSubagentHookValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// DenySubagentHookOutput 把结构化 reason code 投影成 bridge 可执行的拒绝。
func DenySubagentHookOutput(
	event sdkhook.Event,
	reasonCode string,
	message string,
) sdkhook.Output {
	return denySubagentHookOutput(event, reasonCode, message)
}

func denySubagentHookOutput(
	event sdkhook.Event,
	reasonCode string,
	message string,
) sdkhook.Output {
	reason := formatSubagentHookReason(reasonCode, message)
	if event == sdkhook.EventPreToolUse {
		return sdkhook.Output{
			SpecificOutput: &sdkhook.SpecificOutput{
				HookEventName:            sdkhook.EventPreToolUse,
				PermissionDecision:       sdkpermission.BehaviorDeny,
				PermissionDecisionReason: reason,
			},
		}
	}
	shouldContinue := false
	return sdkhook.Output{
		Continue:   &shouldContinue,
		StopReason: reason,
	}
}

func formatSubagentHookReason(reasonCode string, message string) string {
	reasonCode = strings.TrimSpace(reasonCode)
	message = strings.TrimSpace(message)
	switch {
	case reasonCode == "":
		return message
	case message == "":
		return "[" + reasonCode + "]"
	default:
		return "[" + reasonCode + "] " + message
	}
}
