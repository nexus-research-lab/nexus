package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// GuidedInput 是等待注入当前 round 的用户引导。
type GuidedInput struct {
	RoundID string
	Content string
}

// QueueGuidanceInput 把用户引导暂存到运行中 session，等待 PostToolUse hook 消费。
func (m *Manager) QueueGuidanceInput(_ context.Context, sessionKey string, roundID string, content string) ([]string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.RunningRounds) == 0 {
		return nil, ErrNoRunningRound
	}

	roundIDs := make([]string, 0, len(state.RunningRounds))
	for runningRoundID := range state.RunningRounds {
		roundIDs = append(roundIDs, runningRoundID)
	}
	sort.Strings(roundIDs)
	state.GuidedInputs = append(state.GuidedInputs, GuidedInput{
		RoundID: strings.TrimSpace(roundID),
		Content: content,
	})
	return roundIDs, nil
}

// PendingGuidanceCount 返回当前 session 中尚未被 hook 注入的引导数量。
func (m *Manager) PendingGuidanceCount(sessionKey string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil {
		return 0
	}
	return len(state.GuidedInputs)
}

// WithGuidanceHook 给 SDK options 挂载 Nexus 运行时引导 hook。
func (m *Manager) WithGuidanceHook(options agentclient.Options, sessionKey string) agentclient.Options {
	if strings.TrimSpace(sessionKey) == "" {
		return options
	}
	return WithPostToolUseGuidanceHook(options, m.postToolUseGuidanceHook(sessionKey))
}

// WithPostToolUseGuidanceHook 给 SDK options 追加一个 PostToolUse 引导 hook。
func WithPostToolUseGuidanceHook(options agentclient.Options, callback sdkprotocol.HookCallback) agentclient.Options {
	if callback == nil {
		return options
	}
	hooks := cloneSDKHooks(options.Hooks)
	hooks[sdkprotocol.HookEventPostToolUse] = append(
		hooks[sdkprotocol.HookEventPostToolUse],
		sdkprotocol.HookMatcher{
			Hooks:   []sdkprotocol.HookCallback{callback},
			Timeout: 2 * time.Second,
		},
	)
	options.Hooks = hooks
	return options
}

func (m *Manager) postToolUseGuidanceHook(sessionKey string) sdkprotocol.HookCallback {
	return func(_ context.Context, input sdkprotocol.HookInput, _ string) (sdkprotocol.HookOutput, error) {
		if input.EventName != "" && input.EventName != sdkprotocol.HookEventPostToolUse {
			return sdkprotocol.HookOutput{}, nil
		}
		inputs := m.drainGuidanceInputs(sessionKey)
		if len(inputs) == 0 {
			return sdkprotocol.HookOutput{}, nil
		}
		return sdkprotocol.HookOutput{
			HookSpecificOutput: map[string]any{
				"hookEventName":     string(sdkprotocol.HookEventPostToolUse),
				"additionalContext": FormatGuidanceAdditionalContext(inputs),
			},
		}, nil
	}
}

func (m *Manager) drainGuidanceInputs(sessionKey string) []GuidedInput {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GuidedInputs) == 0 {
		return nil
	}
	inputs := append([]GuidedInput(nil), state.GuidedInputs...)
	state.GuidedInputs = nil
	return inputs
}

// FormatGuidanceAdditionalContext 把待注入引导渲染成 hook additionalContext。
func FormatGuidanceAdditionalContext(inputs []GuidedInput) string {
	lines := []string{
		"<nexus_guidance>",
		"用户在你执行当前 round 时补充了以下引导。请在继续下一步前结合这些要求；如果与原任务冲突，以最新引导为准。",
	}
	for index, input := range inputs {
		content := strings.TrimSpace(input.Content)
		if content == "" {
			continue
		}
		if strings.TrimSpace(input.RoundID) != "" {
			lines = append(lines, fmt.Sprintf("%d. round_id=%s: %s", index+1, strings.TrimSpace(input.RoundID), content))
		} else {
			lines = append(lines, fmt.Sprintf("%d. %s", index+1, content))
		}
	}
	lines = append(lines, "</nexus_guidance>")
	return strings.Join(lines, "\n")
}

func cloneSDKHooks(input map[sdkprotocol.HookEvent][]sdkprotocol.HookMatcher) map[sdkprotocol.HookEvent][]sdkprotocol.HookMatcher {
	output := make(map[sdkprotocol.HookEvent][]sdkprotocol.HookMatcher, len(input)+1)
	for event, matchers := range input {
		copied := make([]sdkprotocol.HookMatcher, 0, len(matchers))
		for _, matcher := range matchers {
			next := matcher
			next.Hooks = append([]sdkprotocol.HookCallback(nil), matcher.Hooks...)
			copied = append(copied, next)
		}
		output[event] = copied
	}
	return output
}
