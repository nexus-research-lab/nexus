// INPUT: 运行中 round 的用户引导与内部上下文。
// OUTPUT: PostToolUse 可消费的 SDK additionalContext，以及消费后回调。
// POS: runtime 层统一的轮内引导队列与格式化入口。
package runtime

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

// GuidedInput 是等待注入当前 round 的用户引导或运行时上下文。
type GuidedInput struct {
	RoundID     string
	Content     string
	ContextName string
	onConsumed  func()
}

// QueueGuidanceInput 把用户引导暂存到运行中 session，等待 PostToolUse hook 消费。
func (m *Manager) QueueGuidanceInput(_ context.Context, sessionKey string, roundID string, content string) ([]string, error) {
	return m.queueGuidanceInput(sessionKey, roundID, content, "", nil)
}

// QueueContextualGuidanceInput 把运行时拥有的上下文暂存到运行中 session。
func (m *Manager) QueueContextualGuidanceInput(_ context.Context, sessionKey string, roundID string, contextName string, content string) ([]string, error) {
	return m.queueGuidanceInput(sessionKey, roundID, content, contextName, nil)
}

// QueueContextualGuidanceInputOnConsumed 暂存内部上下文，并在 PostToolUse 真正取走该上下文后执行回调。
func (m *Manager) QueueContextualGuidanceInputOnConsumed(
	_ context.Context,
	sessionKey string,
	roundID string,
	contextName string,
	content string,
	onConsumed func(),
) ([]string, error) {
	return m.queueGuidanceInput(sessionKey, roundID, content, contextName, onConsumed)
}

func (m *Manager) queueGuidanceInput(sessionKey string, roundID string, content string, contextName string, onConsumed func()) ([]string, error) {
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

	roundIDs := slices.Sorted(maps.Keys(state.RunningRounds))
	state.GuidedInputs = append(state.GuidedInputs, GuidedInput{
		RoundID:     strings.TrimSpace(roundID),
		Content:     content,
		ContextName: normalizeGuidanceContextName(contextName),
		onConsumed:  onConsumed,
	})
	m.touchStateLocked(state)
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
func WithPostToolUseGuidanceHook(options agentclient.Options, callback sdkhook.Callback) agentclient.Options {
	if callback == nil {
		return options
	}
	wrappedCallback := func(ctx context.Context, input sdkhook.Input, toolUseID string) (sdkhook.Output, error) {
		output, err := callback(ctx, input, toolUseID)
		if err != nil {
			return output, err
		}
		if hookOutputIsEmpty(output) {
			return noopHookOutput(), nil
		}
		return output, nil
	}
	hooks := cloneSDKHooks(options.Hooks.Matchers)
	hooks[sdkhook.EventPostToolUse] = append(
		hooks[sdkhook.EventPostToolUse],
		sdkhook.Matcher{
			Hooks:   []sdkhook.Callback{wrappedCallback},
			Timeout: 2 * time.Second,
		},
	)
	options.Hooks.Matchers = hooks
	return options
}

func noopHookOutput() sdkhook.Output {
	continueValue := true
	return sdkhook.Output{Continue: &continueValue}
}

func hookOutputIsEmpty(output sdkhook.Output) bool {
	return output.Async == nil &&
		output.Continue == nil &&
		output.SuppressOutput == nil &&
		output.StopReason == "" &&
		output.Decision == "" &&
		output.SystemMessage == "" &&
		output.Reason == "" &&
		output.SpecificOutput == nil &&
		len(output.RawSpecificOutput) == 0
}

func (m *Manager) postToolUseGuidanceHook(sessionKey string) sdkhook.Callback {
	return func(_ context.Context, input sdkhook.Input, _ string) (sdkhook.Output, error) {
		if input.EventName != "" && input.EventName != sdkhook.EventPostToolUse {
			return sdkhook.Output{}, nil
		}
		inputs := m.drainGuidanceInputs(sessionKey)
		if len(inputs) == 0 {
			return sdkhook.Output{}, nil
		}
		additionalContext := FormatGuidanceAdditionalContext(inputs)
		for _, item := range inputs {
			if item.onConsumed != nil {
				item.onConsumed()
			}
		}
		return sdkhook.Output{
			SpecificOutput: &sdkhook.SpecificOutput{
				HookEventName:     sdkhook.EventPostToolUse,
				AdditionalContext: additionalContext,
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
	inputs := slices.Clone(state.GuidedInputs)
	state.GuidedInputs = nil
	m.touchStateLocked(state)
	return inputs
}

// FormatGuidanceAdditionalContext 把待注入引导渲染成 hook additionalContext。
func FormatGuidanceAdditionalContext(inputs []GuidedInput) string {
	parts := make([]string, 0, 2)
	contextBlocks := renderGuidanceContextBlocks(inputs)
	if contextBlocks != "" {
		parts = append(parts, contextBlocks)
	}
	guidanceBlock := renderUserGuidanceBlock(inputs)
	if guidanceBlock != "" {
		parts = append(parts, guidanceBlock)
	}
	return strings.Join(parts, "\n\n")
}

func renderUserGuidanceBlock(inputs []GuidedInput) string {
	lines := []string{
		"<nexus_guidance>",
		"用户在你执行当前 round 时补充了以下引导。请在继续下一步前结合这些要求；如果与原任务冲突，以最新引导为准；最终可见回复必须明确回应最新用户引导。",
	}
	count := 0
	for _, input := range inputs {
		if input.ContextName != "" {
			continue
		}
		content := strings.TrimSpace(input.Content)
		if content == "" {
			continue
		}
		count++
		if strings.TrimSpace(input.RoundID) != "" {
			lines = append(lines, fmt.Sprintf("%d. round_id=%s: %s", count, strings.TrimSpace(input.RoundID), content))
		} else {
			lines = append(lines, fmt.Sprintf("%d. %s", count, content))
		}
	}
	if count == 0 {
		return ""
	}
	lines = append(lines, "</nexus_guidance>")
	return strings.Join(lines, "\n")
}

func renderGuidanceContextBlocks(inputs []GuidedInput) string {
	blocks := make([]string, 0, len(inputs))
	for _, input := range inputs {
		name := normalizeGuidanceContextName(input.ContextName)
		content := strings.TrimSpace(input.Content)
		if name == "" || content == "" {
			continue
		}
		if source := internalContextSourceName(name); source != "" {
			blocks = append(blocks, renderInternalContext(source, content))
			continue
		}
		blocks = append(blocks, fmt.Sprintf("<%s>\n%s\n</%s>", name, content, name))
	}
	return strings.Join(blocks, "\n\n")
}

func normalizeGuidanceContextName(name string) string {
	name = strings.TrimSpace(name)
	for _, r := range name {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' ||
			r == '-' {
			continue
		}
		return ""
	}
	return name
}

func cloneSDKHooks(input map[sdkhook.Event][]sdkhook.Matcher) map[sdkhook.Event][]sdkhook.Matcher {
	output := make(map[sdkhook.Event][]sdkhook.Matcher, len(input)+1)
	for event, matchers := range input {
		copied := make([]sdkhook.Matcher, 0, len(matchers))
		for _, matcher := range matchers {
			next := matcher
			next.Hooks = slices.Clone(matcher.Hooks)
			copied = append(copied, next)
		}
		output[event] = copied
	}
	return output
}
