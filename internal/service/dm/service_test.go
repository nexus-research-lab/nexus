package dm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type fakeDMClient struct {
	mu              sync.Mutex
	sessionID       string
	messages        chan sdkprotocol.ReceivedMessage
	interruptCalls  int
	connectCalls    int
	disconnectCalls int
	interruptErrors []error
	disconnectErrs  []error
	connectErrors   []error
	queryErrors     []error
	queryPrompts    []string
	sentContents    []string
	queryOptions    []sdkprotocol.OutboundMessageOptions
	reconfigureOps  []agentclient.Options
	onQuery         func(context.Context, string)
	onInterrupt     func(context.Context)
}

type fakeTokenUsageRecorder struct {
	inputs []usagesvc.RecordInput
}

func (r *fakeTokenUsageRecorder) RecordMessageUsage(_ context.Context, input usagesvc.RecordInput) error {
	r.inputs = append(r.inputs, input)
	return nil
}

type externalReplyCall struct {
	agentID string
	text    string
	target  ExternalReplyTarget
}

type fakeExternalReplyDispatcher struct {
	mu          sync.Mutex
	calls       []externalReplyCall
	typingCalls []externalTypingCall
}

type externalTypingCall struct {
	agentID string
	target  ExternalReplyTarget
	active  bool
}

func (d *fakeExternalReplyDispatcher) DeliverExternalReply(
	_ context.Context,
	agentID string,
	text string,
	target ExternalReplyTarget,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, externalReplyCall{
		agentID: agentID,
		text:    text,
		target:  target,
	})
	return nil
}

func (d *fakeExternalReplyDispatcher) callsSnapshot() []externalReplyCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]externalReplyCall(nil), d.calls...)
}

func (d *fakeExternalReplyDispatcher) SetExternalTyping(
	_ context.Context,
	agentID string,
	target ExternalReplyTarget,
	active bool,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.typingCalls = append(d.typingCalls, externalTypingCall{
		agentID: agentID,
		target:  target,
		active:  active,
	})
	return nil
}

func (d *fakeExternalReplyDispatcher) typingCallsSnapshot() []externalTypingCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]externalTypingCall(nil), d.typingCalls...)
}

func TestRoundRunnerDeliversExternalAssistantReply(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{
		service:    &Service{replies: dispatcher},
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: "agent:agent-1:weixin-personal:dm:user-1",
		roundID:    "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:     "explicit",
			Channel:  "weixin-personal",
			To:       "user-1",
			ThreadID: "context-token-1",
		},
	}

	runner.deliverExternalAssistantReply(context.Background(), protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "你好，我是五子棋。"},
		},
	})

	calls := dispatcher.callsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("期望外部回复投递 1 次，实际 %d", len(calls))
	}
	if calls[0].agentID != "agent-1" || calls[0].text != "你好，我是五子棋。" {
		t.Fatalf("外部回复内容不正确: %+v", calls[0])
	}
	if calls[0].target.Channel != "weixin-personal" ||
		calls[0].target.To != "user-1" ||
		calls[0].target.ThreadID != "context-token-1" {
		t.Fatalf("外部回复目标不正确: %+v", calls[0].target)
	}
}

func TestRoundRunnerSkipsExternalReplyForWebSocketSession(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{
		service:    &Service{replies: dispatcher},
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: "agent:agent-1:ws:dm:user-1",
		roundID:    "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:    "explicit",
			Channel: "weixin-personal",
			To:      "user-1",
		},
	}

	runner.deliverExternalAssistantReply(context.Background(), protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "普通网页会话不应发外部通道。"},
		},
	})

	if calls := dispatcher.callsSnapshot(); len(calls) != 0 {
		t.Fatalf("WebSocket 会话不应触发外部回复: %+v", calls)
	}
}

func TestRoundRunnerMaintainsExternalTypingState(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{
		service:    &Service{replies: dispatcher},
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: "agent:agent-1:weixin-personal:dm:user-1",
		roundID:    "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:     "explicit",
			Channel:  "weixin-personal",
			To:       "user-1",
			ThreadID: "context-token-1",
		},
	}

	stop := runner.startExternalReplyTyping(context.Background())
	deadline := time.After(2 * time.Second)
	for {
		if calls := dispatcher.typingCallsSnapshot(); len(calls) > 0 && calls[0].active {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("未发送 typing start: %+v", dispatcher.typingCallsSnapshot())
		case <-time.After(10 * time.Millisecond):
		}
	}
	stop()

	calls := dispatcher.typingCallsSnapshot()
	if len(calls) < 2 {
		t.Fatalf("typing start/stop 调用不足: %+v", calls)
	}
	if !calls[0].active || calls[len(calls)-1].active {
		t.Fatalf("typing 状态顺序不正确: %+v", calls)
	}
	if calls[0].target.ThreadID != "context-token-1" || calls[len(calls)-1].target.To != "user-1" {
		t.Fatalf("typing 目标不正确: %+v", calls)
	}
}

func TestRoundRunnerSkipsExternalTypingForQuickReply(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{
		service:    &Service{replies: dispatcher},
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: "agent:agent-1:weixin-personal:dm:user-1",
		roundID:    "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:     "explicit",
			Channel:  "weixin-personal",
			To:       "user-1",
			ThreadID: "context-token-1",
		},
	}

	stop := runner.startExternalReplyTyping(context.Background())
	stop()
	time.Sleep(externalTypingStartDelay + 50*time.Millisecond)

	if calls := dispatcher.typingCallsSnapshot(); len(calls) != 0 {
		t.Fatalf("快速结束不应发送 typing 状态: %+v", calls)
	}
}

func TestScheduleTitleGenerationSkipsRoomConversationForExternalDMSession(t *testing.T) {
	t.Parallel()

	titleScheduler := &fakeDMTitleScheduler{}
	service := &Service{titles: titleScheduler}
	sessionKey := "agent:agent-1:weixin-personal:dm:wx-user-1"
	conversationID := "wx-user-1"
	service.scheduleTitleGeneration(
		context.Background(),
		protocol.ParseSessionKey(sessionKey),
		protocol.Session{
			SessionKey:     sessionKey,
			AgentID:        "agent-1",
			ChannelType:    "weixin-personal",
			ChatType:       "dm",
			Title:          "New Chat",
			MessageCount:   1,
			ConversationID: &conversationID,
		},
		"你好",
		1,
		"kimi-code",
		"kimi-for-coding",
	)

	request := titleScheduler.LastRequest()
	if request.SessionKey != sessionKey {
		t.Fatalf("标题请求 session_key 不正确: %+v", request)
	}
	if request.ConversationID != "" || request.ConversationRoomID != "" || request.ConversationMessageCount != -1 {
		t.Fatalf("外部 DM 不应作为 room conversation 调度标题: %+v", request)
	}
}

func TestRoundRunnerUsagePrefersResultAggregateOverTerminalAssistant(t *testing.T) {
	t.Parallel()

	recorder := &fakeTokenUsageRecorder{}
	runner := &roundRunner{
		service:     &Service{usage: recorder},
		ownerUserID: "user-1",
		sessionKey:  "agent:demo:dm:session",
		roundID:     "round-1",
	}
	result := protocol.Message{
		"role":        "result",
		"message_id":  "result-1",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage": map[string]any{
			"input_tokens": 10,
		},
	}
	assistant := protocol.Message{
		"role":        "assistant",
		"message_id":  "assistant-1",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage": map[string]any{
			"input_tokens": 3,
		},
	}

	runner.recordUsage(result)
	runner.recordTerminalAssistantUsage(assistant)

	if len(recorder.inputs) != 1 {
		t.Fatalf("usage 记录数量 = %d，期望只记录 result 聚合 usage", len(recorder.inputs))
	}
	if recorder.inputs[0].MessageID != "result-1" {
		t.Fatalf("应记录 result usage，实际=%+v", recorder.inputs[0])
	}
}

func TestRoundRunnerUsageFallsBackToTerminalAssistantWhenResultUsageEmpty(t *testing.T) {
	t.Parallel()

	recorder := &fakeTokenUsageRecorder{}
	runner := &roundRunner{
		service:     &Service{usage: recorder},
		ownerUserID: "user-1",
		sessionKey:  "agent:demo:dm:session",
		roundID:     "round-1",
	}

	runner.recordUsage(protocol.Message{
		"role":        "result",
		"message_id":  "result-empty",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage":       map[string]any{},
	})
	runner.recordTerminalAssistantUsage(protocol.Message{
		"role":        "assistant",
		"message_id":  "assistant-1",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage": map[string]any{
			"input_tokens": 3,
		},
	})

	if len(recorder.inputs) != 1 {
		t.Fatalf("usage 记录数量 = %d，期望 fallback 记录 assistant usage", len(recorder.inputs))
	}
	if recorder.inputs[0].MessageID != "assistant-1" {
		t.Fatalf("应 fallback 记录 assistant usage，实际=%+v", recorder.inputs[0])
	}
}

func newFakeDMClient() *fakeDMClient {
	return &fakeDMClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 16),
	}
}

func (c *fakeDMClient) Connect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectCalls++
	if len(c.connectErrors) == 0 {
		return nil
	}
	err := c.connectErrors[0]
	c.connectErrors = c.connectErrors[1:]
	return err
}

func (c *fakeDMClient) Query(ctx context.Context, prompt string) error {
	return c.QueryWithOptions(ctx, prompt, sdkprotocol.OutboundMessageOptions{})
}

func (c *fakeDMClient) QueryWithOptions(ctx context.Context, prompt string, options sdkprotocol.OutboundMessageOptions) error {
	if c.onQuery != nil {
		c.onQuery(ctx, prompt)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queryPrompts = append(c.queryPrompts, prompt)
	c.queryOptions = append(c.queryOptions, options)
	if len(c.queryErrors) > 0 {
		err := c.queryErrors[0]
		c.queryErrors = c.queryErrors[1:]
		return err
	}
	return ctx.Err()
}

func (c *fakeDMClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeDMClient) SendContent(_ context.Context, content any, _ *string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sentContents = append(c.sentContents, normalizeTestString(content))
	return nil
}

func (c *fakeDMClient) Interrupt(ctx context.Context) error {
	c.mu.Lock()
	c.interruptCalls++
	if len(c.interruptErrors) > 0 {
		err := c.interruptErrors[0]
		c.interruptErrors = c.interruptErrors[1:]
		c.mu.Unlock()
		return err
	}
	callback := c.onInterrupt
	c.mu.Unlock()
	if callback != nil {
		callback(ctx)
	}
	return nil
}

func (c *fakeDMClient) Disconnect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnectCalls++
	if len(c.disconnectErrs) > 0 {
		err := c.disconnectErrs[0]
		c.disconnectErrs = c.disconnectErrs[1:]
		return err
	}
	return nil
}

func (c *fakeDMClient) Reconfigure(_ context.Context, options agentclient.Options) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconfigureOps = append(c.reconfigureOps, options)
	return nil
}

func (c *fakeDMClient) SessionID() string { return c.sessionID }

func normalizeTestString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

type fakeDMFactory struct {
	mu      sync.Mutex
	client  *fakeDMClient
	clients []*fakeDMClient
	options []agentclient.Options
}

func (f *fakeDMFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options = append(f.options, options)
	if len(f.clients) > 0 {
		client := f.clients[0]
		f.clients = f.clients[1:]
		return client
	}
	return f.client
}

func (f *fakeDMFactory) LastOptions() agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.options) == 0 {
		return agentclient.Options{}
	}
	return f.options[len(f.options)-1]
}

func (f *fakeDMFactory) OptionAt(index int) agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.options) {
		return agentclient.Options{}
	}
	return f.options[index]
}

type fakeDMRoomSessionStore struct {
	mu      sync.Mutex
	updates []fakeDMRoomSessionUpdate
}

type fakeDMRoomSessionUpdate struct {
	roomSessionID string
	sdkSessionID  string
}

func (s *fakeDMRoomSessionStore) GetRoomSessionByKey(
	context.Context,
	string,
	protocol.SessionKey,
) (*protocol.Session, error) {
	return nil, nil
}

func (s *fakeDMRoomSessionStore) UpdateRoomSessionSDKSessionID(
	_ context.Context,
	roomSessionID string,
	sdkSessionID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, fakeDMRoomSessionUpdate{
		roomSessionID: strings.TrimSpace(roomSessionID),
		sdkSessionID:  strings.TrimSpace(sdkSessionID),
	})
	return nil
}

func (s *fakeDMRoomSessionStore) Updates() []fakeDMRoomSessionUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]fakeDMRoomSessionUpdate(nil), s.updates...)
}

type fakeGoalContextProvider struct {
	mu               sync.Mutex
	plan             *protocol.GoalContinuation
	planCalls        int
	runtimeContext   string
	runtimeGoal      *protocol.Goal
	runtimeCalls     int
	usage            []protocol.GoalUsage
	usageLimitReason []string
	progress         []bool
	failures         []string
	completionMisses []string
	activities       []string
	current          *bool
}

func (p *fakeGoalContextProvider) RuntimeContext(context.Context, string) (string, *protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runtimeCalls++
	if p.runtimeGoal == nil {
		return p.runtimeContext, nil, nil
	}
	goal := *p.runtimeGoal
	return p.runtimeContext, &goal, nil
}

func (p *fakeGoalContextProvider) RecordUsageForSession(_ context.Context, _ string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordUsageForGoal(_ context.Context, _ string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	return nil, nil
}

func (p *fakeGoalContextProvider) UsageLimitForSession(_ context.Context, _ string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usageLimitReason = append(p.usageLimitReason, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordContinuationProgress(_ context.Context, _ string, _ string, progressed bool) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress = append(p.progress, progressed)
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordContinuationFailure(_ context.Context, _ string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = append(p.failures, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordCompletionToolMiss(_ context.Context, _ string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completionMisses = append(p.completionMisses, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeGoalContextProvider) RecordGoalActivity(_ context.Context, _ string, roundID string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activities = append(p.activities, strings.TrimSpace(roundID))
	return nil, nil
}

func (p *fakeGoalContextProvider) PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.planCalls++
	if p.planCalls > 1 || p.plan == nil {
		return nil, nil
	}
	plan := *p.plan
	return &plan, nil
}

func (p *fakeGoalContextProvider) GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return true, nil
	}
	return *p.current, nil
}

func TestRoundRunnerRecordsGoalUsageAtToolCompletion(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 3))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[0].InputTokens != 4 || usages[0].OutputTokens != 3 || usages[0].Total() != 7 {
		t.Fatalf("first usage = %#v, want 4/3", usages[0])
	}
	if usages[1].InputTokens != 6 || usages[1].OutputTokens != 2 || usages[1].Total() != 8 {
		t.Fatalf("second usage = %#v, want remaining 6/2", usages[1])
	}
}

func TestRoundRunnerRecordsAbortGoalUsageFromAssistantSnapshot(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{}, goalAssistantUsageMessage(7, 3))

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[1].InputTokens != 3 || usages[1].OutputTokens != 2 || usages[1].Total() != 5 {
		t.Fatalf("abort usage = %#v, want remaining 3/2", usages[1])
	}
}

func TestRoundRunnerMarksUsageLimitAfterAccounting(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  3,
			OutputTokens: 2,
			TotalTokens:  5,
		},
		UsageLimitReached: true,
		UsageLimitReason:  "You've hit your usage limit.",
	}, nil)
	runner.recordGoalUsageLimit(runtimectx.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "You've hit your usage limit.",
	})

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 || usages[0].Total() != 5 {
		t.Fatalf("usages = %#v, want usage recorded before limit", usages)
	}
	reasons := goalProvider.recordedUsageLimitReasons()
	if len(reasons) != 1 || reasons[0] != "You've hit your usage limit." {
		t.Fatalf("usage limit reasons = %#v, want runtime reason", reasons)
	}
}

func TestRoundRunnerRecordsEmptyGoalContinuationProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || progress[0] {
		t.Fatalf("progress = %#v, want one false continuation progress", progress)
	}
}

func TestRoundRunnerRecordsGoalContinuationFailure(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{
		TerminalStatus: "error",
		ResultSubtype:  "error",
		ErrorMessage:   "Failed to authenticate. API Error: 401",
	})

	failures := goalProvider.recordedFailures()
	if len(failures) != 1 || failures[0] != "Failed to authenticate. API Error: 401" {
		t.Fatalf("failures = %#v, want provider error", failures)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want failure path instead of empty progress", progress)
	}
}

func TestRoundRunnerRecordsGoalContinuationToolProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || !progress[0] {
		t.Fatalf("progress = %#v, want one true continuation progress", progress)
	}
}

func TestRoundRunnerRecordsGoalCompletionToolMiss(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}
	runner.rememberGoalAssistantMessage(goalCompletionToolMissAssistantMessage())

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	misses := goalProvider.recordedCompletionMisses()
	if len(misses) != 1 || !strings.Contains(misses[0], "mcp__nexus_goal__update_goal") {
		t.Fatalf("completion misses = %#v, want one missing update_goal record", misses)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want completion miss path instead of empty progress", progress)
	}
}

func TestRoundRunnerRecordsUserGoalActivityInsteadOfContinuationProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-user",
		goalIDForUsage: "goal-1",
	}

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.activities) != 1 || goalProvider.activities[0] != "round-user" {
		t.Fatalf("activities = %#v, want explicit goal activity", goalProvider.activities)
	}
	if len(goalProvider.progress) != 0 {
		t.Fatalf("progress = %#v, want no continuation progress for user round", goalProvider.progress)
	}
}

func TestRoundRunnerClosesGoalUsageAfterUpdateGoal(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "update_goal", false, 10, 2))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  20,
			OutputTokens: 5,
			TotalTokens:  25,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 {
		t.Fatalf("len(usages) = %d, want 1", len(usages))
	}
	if usages[0].InputTokens != 10 || usages[0].OutputTokens != 2 || usages[0].Total() != 12 {
		t.Fatalf("usage = %#v, want update_goal usage only", usages[0])
	}
}

func TestRoundRunnerClosesGoalUsageAfterMCPUpdateGoal(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "mcp__nexus_goal__update_goal", false, 10, 2))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  20,
			OutputTokens: 5,
			TotalTokens:  25,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 {
		t.Fatalf("len(usages) = %d, want 1", len(usages))
	}
	if usages[0].InputTokens != 10 || usages[0].OutputTokens != 2 || usages[0].Total() != 12 {
		t.Fatalf("usage = %#v, want MCP update_goal usage only", usages[0])
	}
}

func TestRoundRunnerClearGoalUsageStopsLaterAccounting(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.clearGoalUsage()
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  20,
			OutputTokens: 5,
			TotalTokens:  25,
		},
	}, nil)

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("usages = %#v, want none after clear", usages)
	}
}

func TestRoundRunnerActivateGoalUsageRestartsFromCurrentSnapshot(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.clearGoalUsage()
	runner.rememberGoalAssistantMessage(goalToolResultAssistantMessage("tool-2", "read_file", false, 7, 3))
	if err := runner.activateGoalUsage(context.Background()); err != nil {
		t.Fatal(err)
	}
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want initial usage and post-activate delta", len(usages))
	}
	if usages[1].InputTokens != 3 || usages[1].OutputTokens != 2 || usages[1].Total() != 5 {
		t.Fatalf("post-activate usage = %#v, want 3/2", usages[1])
	}
}

func TestRoundRunnerResetsGoalUsageAfterCreateGoal(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:    &Service{goals: goalProvider},
		sessionKey: "agent:nexus:ws:dm:test",
		roundID:    "round-1",
		goalUsage:  goalsvc.NewRuntimeUsageAccumulator(false),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "create_goal", false, 5, 1))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  8,
			OutputTokens: 3,
			TotalTokens:  11,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 {
		t.Fatalf("len(usages) = %d, want 1", len(usages))
	}
	if usages[0].InputTokens != 3 || usages[0].OutputTokens != 2 || usages[0].Total() != 5 {
		t.Fatalf("usage = %#v, want post-create delta 3/2", usages[0])
	}
}

func TestRoundRunnerResetsGoalUsageAfterMCPCreateGoal(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:    &Service{goals: goalProvider},
		sessionKey: "agent:nexus:ws:dm:test",
		roundID:    "round-1",
		goalUsage:  goalsvc.NewRuntimeUsageAccumulator(false),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "mcp__nexus_goal__create_goal", false, 5, 1))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  8,
			OutputTokens: 3,
			TotalTokens:  11,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 {
		t.Fatalf("len(usages) = %d, want 1", len(usages))
	}
	if usages[0].InputTokens != 3 || usages[0].OutputTokens != 2 || usages[0].Total() != 5 {
		t.Fatalf("usage = %#v, want post-MCP-create delta 3/2", usages[0])
	}
}

func TestRoundRunnerIgnoresGoalRuntimeInPlanMode(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:          &Service{goals: goalProvider},
		sessionKey:       "agent:nexus:ws:dm:test-goal-plan-runtime",
		roundID:          "round-plan",
		goalIDForUsage:   "goal-plan",
		goalUsage:        goalsvc.NewRuntimeUsageAccumulator(true),
		goalUsageStarted: time.Now(),
		permissionMode:   sdkpermission.ModePlan,
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 2,
		},
		ElapsedTimeSeconds: 3,
	}, protocol.Message{})
	runner.recordGoalUsageLimit(runtimectx.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "usage limit",
	})
	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("plan mode recorded goal usage: %#v", usages)
	}
	if reasons := goalProvider.recordedUsageLimitReasons(); len(reasons) != 0 {
		t.Fatalf("plan mode recorded usage limit: %#v", reasons)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("plan mode recorded continuation progress: %#v", progress)
	}
}

func (p *fakeGoalContextProvider) recordedUsage() []protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsage(nil), p.usage...)
}

func (p *fakeGoalContextProvider) recordedUsageLimitReasons() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.usageLimitReason...)
}

func (p *fakeGoalContextProvider) recordedProgress() []bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]bool(nil), p.progress...)
}

func (p *fakeGoalContextProvider) recordedFailures() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.failures...)
}

func (p *fakeGoalContextProvider) recordedCompletionMisses() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.completionMisses...)
}

func (p *fakeGoalContextProvider) runtimeContextCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.runtimeCalls
}

func goalToolResultAssistantMessage(
	toolUseID string,
	toolName string,
	isError bool,
	inputTokens int64,
	outputTokens int64,
) protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
		"content": []map[string]any{
			{"type": "tool_use", "id": toolUseID, "name": toolName},
			{"type": "tool_result", "tool_use_id": toolUseID, "is_error": isError},
		},
	}
}

func goalAssistantUsageMessage(inputTokens int64, outputTokens int64) protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}
}

func goalCompletionToolMissAssistantMessage() protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "任务已经完成，但我没有看到 mcp__nexus_goal__update_goal 工具，无法调用它来标记完成。"},
		},
	}
}

type dmTestSender struct {
	key    string
	events chan protocol.EventMessage
}

func newDMTestSender(key string) *dmTestSender {
	return &dmTestSender{
		key:    key,
		events: make(chan protocol.EventMessage, 32),
	}
}

func (s *dmTestSender) Key() string    { return s.key }
func (s *dmTestSender) IsClosed() bool { return false }
func (s *dmTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

type fakeDMTitleScheduler struct {
	mu       sync.Mutex
	requests []titlegen.Request
}

func (s *fakeDMTitleScheduler) Schedule(_ context.Context, request titlegen.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, request)
}

func (s *fakeDMTitleScheduler) LastRequest() titlegen.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return titlegen.Request{}
	}
	return s.requests[len(s.requests)-1]
}

type fakeDMPreferencesService struct {
	prefs preferencessvc.Preferences
}

func (s fakeDMPreferencesService) Get(_ context.Context, _ string) (preferencessvc.Preferences, error) {
	return s.prefs, nil
}

type blockingDMTestSender struct {
	key  string
	done chan struct{}
	once sync.Once
}

func (s *blockingDMTestSender) Key() string    { return s.key }
func (s *blockingDMTestSender) IsClosed() bool { return false }
func (s *blockingDMTestSender) SendEvent(ctx context.Context, _ protocol.EventMessage) error {
	<-ctx.Done()
	s.once.Do(func() {
		close(s.done)
	})
	return ctx.Err()
}

func TestDMBroadcastEventHasTotalTimeout(t *testing.T) {
	previousTimeout := dmBroadcastTimeout
	dmBroadcastTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		dmBroadcastTimeout = previousTimeout
	})

	permission := permissionctx.NewContext()
	sender := &blockingDMTestSender{
		key:  "slow-sender",
		done: make(chan struct{}),
	}
	permission.BindSession("session-1", sender)
	service := &Service{permission: permission}

	startedAt := time.Now()
	service.broadcastEventWithTimeout(context.Background(), "session-1", protocol.NewEvent(protocol.EventTypeMessage, map[string]any{}))
	if elapsed := time.Since(startedAt); elapsed > 200*time.Millisecond {
		t.Fatalf("DM 广播未按总超时返回: elapsed=%s", elapsed)
	}
	select {
	case <-sender.done:
	default:
		t.Fatal("慢 sender 没有收到取消信号")
	}
}

func newDMAgentService(t *testing.T, cfg config.Config) *agentsvc.Service {
	t.Helper()
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
}

func newDMProviderService(t *testing.T, cfg config.Config) *providercfg.Service {
	t.Helper()
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开 provider 测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return providercfg.NewServiceWithDB(cfg, db)
}

func createDMProviderWithModel(
	t *testing.T,
	service *providercfg.Service,
	input providercfg.CreateInput,
	model string,
	isDefault bool,
) *providercfg.Record {
	t.Helper()
	record, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(context.Background(), record.Provider, model, providercfg.UpdateModelInput{
		Enabled:   true,
		IsDefault: isDefault,
	}); err != nil {
		t.Fatalf("设置 provider 模型失败: %v", err)
	}
	return record
}

func TestServiceHandleChatPersistsMessages(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "你好，世界"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    12,
					DurationAPIMS: 10,
					NumTurns:      1,
					Result:        "done",
					Usage: map[string]any{
						"input_tokens":  3,
						"output_tokens": 5,
					},
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-1")
	sessionKey := "agent:nexus:ws:dm:test-chat"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "你好",
		RoundID:    "round-1",
		ReqID:      "round-1",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	assertEventTypes(t, events, []protocol.EventType{
		protocol.EventTypeChatAck,
		protocol.EventTypeRoundStatus,
		protocol.EventTypeSessionStatus,
		protocol.EventTypeMessage,
		protocol.EventTypeMessage,
		protocol.EventTypeRoundStatus,
	})
	client.mu.Lock()
	queryPrompts := append([]string(nil), client.queryPrompts...)
	client.mu.Unlock()
	if len(queryPrompts) != 1 {
		t.Fatalf("期望发送 1 条 runtime query，实际 %d", len(queryPrompts))
	}
	for _, expected := range []string{
		"你好",
		"<nexus_runtime_context>",
		"## Date Awareness",
		"## Emotion State",
		"Context ID: dm:" + sessionKey,
		"Base: focused",
	} {
		if !strings.Contains(queryPrompts[0], expected) {
			t.Fatalf("runtime query 缺少动态上下文 %q:\n%s", expected, queryPrompts[0])
		}
	}

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	transcriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeTranscriptFixture(t, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": transcriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-1",
			"sessionId":  stringPointer(t, sessionValue.SessionID),
			"parentUuid": "transcript-user-1",
			"timestamp":  transcriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "你好，世界"},
				},
			},
		},
	})
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("期望 2 条消息，实际 %d", len(messages))
	}
	if messages[0]["role"] != "user" || messages[1]["role"] != "assistant" {
		t.Fatalf("消息角色顺序不正确: %+v", messages)
	}
	summary, ok := messages[1]["result_summary"].(map[string]any)
	if !ok || anyToString(summary["result"]) != "done" || anyToInt(summary["duration_ms"]) != 12 {
		t.Fatalf("result 摘要应挂在 assistant 上: %+v", messages[1])
	}
	usage, _ := summary["usage"].(map[string]any)
	outputTokens := anyToInt(usage["output_tokens"])
	if outputTokens != 5 {
		t.Fatalf("result usage 应保留: %+v", messages[1])
	}
}

func TestServiceHandleChatSchedulesHiddenGoalContinuation(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, prompt string) {
		go func() {
			resultID := "result-first"
			if strings.Contains(prompt, "hidden continuation prompt") {
				resultID = "result-goal-continuation"
				client.messages <- sdkprotocol.ReceivedMessage{
					Type:      sdkprotocol.MessageTypeAssistant,
					SessionID: client.sessionID,
					Assistant: &sdkprotocol.AssistantMessage{
						Message: sdkprotocol.ConversationEnvelope{
							ID:    "assistant-goal-continuation",
							Model: "sonnet",
							Content: []sdkprotocol.ContentBlock{
								sdkprotocol.TextBlock{Text: "继续推进 Goal"},
							},
						},
					},
				}
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      resultID,
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    1,
					DurationAPIMS: 1,
					NumTurns:      1,
					Result:        "done",
					Usage: map[string]any{
						"input_tokens":  int64(2),
						"output_tokens": int64(3),
					},
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetGoalContextProvider(&fakeGoalContextProvider{plan: &protocol.GoalContinuation{
		Goal: protocol.Goal{
			ID:         "goal-1",
			SessionKey: "agent:nexus:ws:dm:test-goal-continuation",
			Objective:  "finish work",
			Status:     protocol.GoalStatusActive,
		},
		RoundID:        "goal_continuation_1",
		Prompt:         "hidden continuation prompt",
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
		Metadata:       map[string]string{"goal_id": "goal-1"},
	}})
	sender := newDMTestSender("sender-goal-continuation")
	sessionKey := "agent:nexus:ws:dm:test-goal-continuation"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:           sessionKey,
		Content:              "开始",
		RoundID:              "round-1",
		ReqID:                "round-1",
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "goal_continuation_1" &&
			event.Data["status"] == "finished"
	})
	continuationRunning := false
	continuationAssistantVisible := false
	for _, event := range events {
		if event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "goal_continuation_1" {
			t.Fatalf("隐藏 Goal continuation 不应广播 chat ack: %+v", events)
		}
		if event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "goal_continuation_1" &&
			event.Data["status"] == "running" {
			continuationRunning = true
		}
		if event.EventType == protocol.EventTypeMessage &&
			event.Data["round_id"] == "goal_continuation_1" &&
			event.Data["role"] == "assistant" {
			for _, block := range contentBlocksFromPayload(t, event.Data) {
				if block["type"] == "text" && block["text"] == "继续推进 Goal" {
					continuationAssistantVisible = true
				}
			}
		}
	}
	if !continuationRunning {
		t.Fatalf("隐藏 Goal continuation 应广播 running round，避免前端空白运行态: %+v", events)
	}
	if !continuationAssistantVisible {
		t.Fatalf("隐藏 Goal continuation 的 assistant 输出应通过消息事件进入当前会话: %+v", events)
	}

	client.mu.Lock()
	queryOptions := append([]sdkprotocol.OutboundMessageOptions(nil), client.queryOptions...)
	queryPrompts := append([]string(nil), client.queryPrompts...)
	client.mu.Unlock()
	if len(queryOptions) < 2 {
		t.Fatalf("Goal continuation 应发送到 runtime: %+v", queryOptions)
	}
	runtimeOptions := queryOptions[1]
	if runtimeOptions.Meta ||
		runtimeOptions.HiddenFromUser ||
		runtimeOptions.Synthetic ||
		runtimeOptions.Purpose != "" ||
		runtimeOptions.Priority != "" ||
		runtimeOptions.Metadata != nil {
		t.Fatalf("Goal continuation 发给 runtime 时应作为普通可执行输入: %+v", queryOptions)
	}

	rows := readDMSessionHistory(t, cfg, service, sessionKey)
	assistantVisible := false
	for _, row := range rows {
		if row["role"] == "user" && row["round_id"] == "goal_continuation_1" {
			t.Fatalf("隐藏 Goal continuation 不应成为可见用户历史: %+v", rows)
		}
		if row["role"] == "assistant" && row["round_id"] == "goal_continuation_1" {
			assistantVisible = true
		}
	}
	if !assistantVisible {
		t.Fatalf("Goal continuation 的 assistant 输出应进入可见历史: %+v", rows)
	}
	if len(queryPrompts) < 2 ||
		!strings.Contains(queryPrompts[1], "<internal_context source=\"goal\">\nhidden continuation prompt\n</internal_context>") {
		t.Fatalf("Goal continuation 应作为 internal goal context 注入 runtime: %+v", queryPrompts)
	}
}

func TestServiceHandleChatBroadcastsMergedParallelToolResults(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-parallel-tools",
						Model: "glm-5.1",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.ToolUseBlock{
								ID:    "tool-connectors",
								Name:  "mcp__nexus_connectors__connector_list",
								Input: json.RawMessage(`{}`),
							},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-parallel-tools",
						Model: "glm-5.1",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.ToolUseBlock{
								ID:    "tool-automation",
								Name:  "mcp__nexus_automation__list_scheduled_tasks",
								Input: json.RawMessage(`{}`),
							},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeUser,
				SessionID: client.sessionID,
				User: &sdkprotocol.UserMessage{
					Message: sdkprotocol.ConversationEnvelope{
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.ToolResultBlock{
								ToolUseID: "tool-connectors",
								Content:   json.RawMessage(`[{"type":"text","text":"[]"}]`),
							},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeUser,
				SessionID: client.sessionID,
				User: &sdkprotocol.UserMessage{
					Message: sdkprotocol.ConversationEnvelope{
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.ToolResultBlock{
								ToolUseID: "tool-automation",
								Content:   json.RawMessage(`[{"type":"text","text":"[]"}]`),
							},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-parallel-final",
						Model: "glm-5.1",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "两个工具都正常调用。"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-parallel-tools",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1200,
					NumTurns:   1,
					Result:     "两个工具都正常调用。",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-parallel-tools")
	sessionKey := "agent:nexus:ws:dm:parallel-tools"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "再试一下两个工具",
		RoundID:    "round-parallel-tools",
		ReqID:      "round-parallel-tools",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	assistantPayload := findLatestAssistantMessagePayload(t, events, "assistant-parallel-tools")
	blocks := contentBlocksFromPayload(t, assistantPayload)
	assertContentBlockTypes(t, blocks, []string{"tool_use", "tool_use", "tool_result", "tool_result", "text"})
	assertToolResultIDs(t, blocks, []string{"tool-connectors", "tool-automation"})
	if assistantPayload["is_complete"] != true || assistantPayload["stop_reason"] != "end_turn" {
		t.Fatalf("最终实时 assistant 应标记完成: %+v", assistantPayload)
	}
	if _, exists := assistantPayload["stream_status"]; exists {
		t.Fatalf("durable assistant 不应补写 stream_status: %+v", assistantPayload)
	}
}

func TestServiceEnsureClientInjectsRuntimePrompt(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	created, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{
		Name:        "提示词助手",
		Description: "负责执行工作区规则",
		VibeTags:    []string{"规则优先", "稳健"},
	})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	if err = os.WriteFile(
		filepath.Join(created.WorkspacePath, "AGENTS.md"),
		[]byte("# AGENTS.md\n\n执行规则：必须先加载工作区规则。\n"),
		0o644,
	); err != nil {
		t.Fatalf("写入 AGENTS.md 失败: %v", err)
	}

	agentValue, err := agentService.GetAgent(context.Background(), created.AgentID)
	if err != nil {
		t.Fatalf("读取测试 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)

	sessionKey := protocol.BuildAgentSessionKey(created.AgentID, protocol.SessionChannelWebSocketSegment, "dm", "prompt-ref", "")
	parsed := protocol.ParseSessionKey(sessionKey)
	sessionItem, err := service.ensureSession(context.Background(), agentValue, parsed, sessionKey)
	if err != nil {
		t.Fatalf("初始化 session 失败: %v", err)
	}
	if _, _, _, _, _, _, _, err = service.ensureClient(context.Background(), sessionKey, agentValue, sessionItem, Request{
		SessionKey:     sessionKey,
		PermissionMode: sdkpermission.ModeDefault,
	}); err != nil {
		t.Fatalf("构建 runtime client 失败: %v", err)
	}

	appendSystemPrompt := factory.LastOptions().System.Append
	if !strings.Contains(appendSystemPrompt, "执行规则：必须先加载工作区规则") {
		t.Fatalf("runtime prompt 未注入 AGENTS.md 内容: %s", appendSystemPrompt)
	}
	if !strings.Contains(appendSystemPrompt, "Description: 负责执行工作区规则") {
		t.Fatalf("runtime prompt 未注入 Agent description: %s", appendSystemPrompt)
	}
	if !strings.Contains(appendSystemPrompt, "Vibe Tags: 规则优先, 稳健") {
		t.Fatalf("runtime prompt 未注入 Agent vibe_tags: %s", appendSystemPrompt)
	}
}

func TestServiceEnsureClientSkipsGoalRuntimeContextInPlanMode(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取默认 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	factory := &fakeDMFactory{client: newFakeDMClient()}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	goalProvider := &fakeGoalContextProvider{
		runtimeContext: "should not enter plan mode",
		runtimeGoal: &protocol.Goal{
			ID:         "goal-plan-context",
			SessionKey: "agent:nexus:ws:dm:test-plan-context",
			Status:     protocol.GoalStatusActive,
		},
	}
	service.SetGoalContextProvider(goalProvider)

	sessionKey := protocol.BuildAgentSessionKey(cfg.DefaultAgentID, protocol.SessionChannelWebSocketSegment, "dm", "plan-context", "")
	parsed := protocol.ParseSessionKey(sessionKey)
	sessionItem, err := service.ensureSession(context.Background(), agentValue, parsed, sessionKey)
	if err != nil {
		t.Fatalf("初始化 session 失败: %v", err)
	}
	_, _, _, _, goalID, goalContext, permissionMode, err := service.ensureClient(context.Background(), sessionKey, agentValue, sessionItem, Request{
		SessionKey:     sessionKey,
		PermissionMode: sdkpermission.ModePlan,
	})
	if err != nil {
		t.Fatalf("构建 plan mode runtime client 失败: %v", err)
	}
	if permissionMode != sdkpermission.ModePlan {
		t.Fatalf("permissionMode = %q, want plan", permissionMode)
	}
	if goalID != "" || goalContext != "" {
		t.Fatalf("plan mode goal runtime context = (%q, %q), want empty", goalID, goalContext)
	}
	if calls := goalProvider.runtimeContextCallCount(); calls != 0 {
		t.Fatalf("plan mode should not read Goal runtime context, calls = %d", calls)
	}
}

func TestServiceEnsureClientKeepsBudgetLimitedGoalUsageTarget(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取默认 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	factory := &fakeDMFactory{client: newFakeDMClient()}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	goalProvider := &fakeGoalContextProvider{
		runtimeGoal: &protocol.Goal{
			ID:         "goal-budget-limited",
			SessionKey: "agent:nexus:ws:dm:test-budget-limited",
			Status:     protocol.GoalStatusBudgetLimited,
		},
	}
	service.SetGoalContextProvider(goalProvider)

	sessionKey := protocol.BuildAgentSessionKey(cfg.DefaultAgentID, protocol.SessionChannelWebSocketSegment, "dm", "budget-limited", "")
	parsed := protocol.ParseSessionKey(sessionKey)
	sessionItem, err := service.ensureSession(context.Background(), agentValue, parsed, sessionKey)
	if err != nil {
		t.Fatalf("初始化 session 失败: %v", err)
	}
	_, _, _, _, goalID, goalContext, _, err := service.ensureClient(context.Background(), sessionKey, agentValue, sessionItem, Request{
		SessionKey:     sessionKey,
		PermissionMode: sdkpermission.ModeDefault,
	})
	if err != nil {
		t.Fatalf("构建 runtime client 失败: %v", err)
	}
	if goalID != "goal-budget-limited" || goalContext != "" {
		t.Fatalf("budget_limited goal runtime = (%q, %q), want usage target without context", goalID, goalContext)
	}
}

func TestServiceHandleChatKeepsThinkingDuringStreamingAndHistoryReplay(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"id":    "assistant-think-1",
							"model": "sonnet",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type":     "thinking",
							"thinking": "先分析",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{
							"type":     "thinking_delta",
							"thinking": " 再收口",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type": "text",
							"text": "今天天气",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{
							"type": "text_delta",
							"text": " 很不错",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-think-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "今天天气 很不错"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-think-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    12,
					DurationAPIMS: 10,
					NumTurns:      1,
					Result:        "done",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-think-stream")
	sessionKey := "agent:nexus:ws:dm:think-stream"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "今天天气怎么样呀",
		RoundID:    "round-think-stream",
		ReqID:      "round-think-stream",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	assertStreamBlockIndex(t, events, "thinking", 0)
	assertStreamBlockIndex(t, events, "text", 1)

	assistantPayload := findAssistantMessagePayload(t, events, "assistant-think-1")
	assistantBlocks := contentBlocksFromPayload(t, assistantPayload)
	if len(assistantBlocks) != 2 {
		t.Fatalf("durable assistant 内容块数量不正确: %+v", assistantPayload)
	}
	if assistantBlocks[0]["type"] != "thinking" || assistantBlocks[0]["thinking"] != "先分析 再收口" {
		t.Fatalf("durable assistant 未保留完整 thinking: %+v", assistantBlocks)
	}
	if assistantBlocks[1]["type"] != "text" || assistantBlocks[1]["text"] != "今天天气 很不错" {
		t.Fatalf("durable assistant 未保留 text: %+v", assistantBlocks)
	}

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	thinkingTranscriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeTranscriptFixture(t, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-think-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": thinkingTranscriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "今天天气怎么样呀",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-think-1",
			"sessionId":  stringPointer(t, sessionValue.SessionID),
			"parentUuid": "transcript-think-user-1",
			"timestamp":  thinkingTranscriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "thinking", "thinking": "先分析 再收口"},
					{"type": "text", "text": "今天天气 很不错"},
				},
			},
		},
	})
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("期望 2 条消息，实际 %d", len(messages))
	}
	historyBlocks := contentBlocksFromPayload(t, messages[1])
	if len(historyBlocks) != 2 || historyBlocks[0]["type"] != "thinking" || historyBlocks[1]["type"] != "text" {
		t.Fatalf("历史 assistant 内容块不正确: %+v", messages[1])
	}
	if _, exists := messages[1]["stream_status"]; exists {
		t.Fatalf("历史 assistant 不应携带 stream_status: %+v", messages[1])
	}
	if _, ok := messages[1]["result_summary"].(map[string]any); !ok {
		t.Fatalf("历史 assistant 应挂载 result 摘要: %+v", messages[1])
	}
}

func TestServiceHandleChatForwardsRuntimeOptions(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	maxThinkingTokens := 2048
	maxTurns := 6
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)
	updatedAgent, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			MaxThinkingTokens: &maxThinkingTokens,
			MaxTurns:          &maxTurns,
			SettingSources:    []string{"user"},
		},
	})
	if err != nil {
		t.Fatalf("更新 agent 配置失败: %v", err)
	}
	if updatedAgent == nil {
		t.Fatal("更新 agent 后返回为空")
	}
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-no-model",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	titleScheduler := &fakeDMTitleScheduler{}
	service.SetTitleGenerator(titleScheduler)
	sender := newDMTestSender("sender-no-model")
	sessionKey := "agent:nexus:ws:dm:no-model"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 model 透传",
		RoundID:    "round-no-model",
		ReqID:      "round-no-model",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 未向 SDK options 透传 provider model: %+v", options)
	}
	if options.Env["ANTHROPIC_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入 provider model: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_AUTH_TOKEN"] != "glm-token" {
		t.Fatalf("runtime 未注入 provider bearer token: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_API_KEY"] != "" {
		t.Fatalf("非官方 Anthropic-compatible runtime 应清空 provider API key，避免继承脏 key: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入默认 sonnet model: %+v", options.Env)
	}
	if options.Env["CLAUDE_CODE_SUBAGENT_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入 subagent model: %+v", options.Env)
	}
	if options.Runtime.MaxThinkingTokens != maxThinkingTokens {
		t.Fatalf("runtime 未向 SDK 透传 max thinking tokens: %+v", options)
	}
	if options.Runtime.MaxTurns != maxTurns {
		t.Fatalf("runtime 未向 SDK 透传 max turns: %+v", options)
	}
	if len(options.SettingSources) != 1 || options.SettingSources[0] != "user" {
		t.Fatalf("runtime 未向 SDK 透传 setting_sources: %+v", options)
	}
	if !options.IncludePartialMessages {
		t.Fatalf("runtime 未开启 partial messages: %+v", options)
	}
	if len(options.Tools.Allow) != 0 {
		t.Fatalf("runtime 不应在无显式白名单时为了 Goal 收窄 allowed tools: %+v", options.Tools.Allow)
	}
	if options.Callbacks.PermissionHandler == nil {
		t.Fatal("runtime 权限处理器为空")
	}
	goalSkillDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Skill",
		Input:    map[string]any{"name": "goal-manager"},
	})
	if err != nil {
		t.Fatalf("Goal Skill 权限处理失败: %v", err)
	}
	if goalSkillDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("Goal Skill 应自动放行: %+v", goalSkillDecision)
	}
	goalToolDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_goal__update_goal",
		Input:    map[string]any{"status": "complete"},
	})
	if err != nil {
		t.Fatalf("Goal 工具权限处理失败: %v", err)
	}
	if goalToolDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("Goal 工具应自动放行: %+v", goalToolDecision)
	}
	titleRequest := titleScheduler.LastRequest()
	if titleRequest.Provider != "glm" || titleRequest.Model != "glm-5.1" {
		t.Fatalf("标题生成未复用本轮 runtime provider/model: %+v", titleRequest)
	}
}

func TestServiceHandleChatUsesPreferenceDefaultModelForIncompleteAgentSelection(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "deepseek",
		DisplayName: "DeepSeek",
		AuthToken:   "deepseek-token",
		BaseURL:     "https://api.deepseek.com/anthropic",
		Enabled:     true,
	}, "deepseek-v4-flash", true)
	updatedAgent, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			Provider: "kimi-code",
		},
	})
	if err != nil {
		t.Fatalf("写入历史 provider-only agent 配置失败: %v", err)
	}
	if updatedAgent == nil {
		t.Fatal("更新 agent 后返回为空")
	}
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-preference-default",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "nxs",
		DefaultAgentOptions: protocol.Options{
			Provider: "deepseek",
			Model:    "deepseek-v4-flash",
		},
	}})
	sender := newDMTestSender("sender-preference-default")
	sessionKey := "agent:nexus:ws:dm:preference-default"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试常规默认模型",
		RoundID:    "round-preference-default",
		ReqID:      "round-preference-default",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Model != "deepseek-v4-flash" {
		t.Fatalf("runtime 未使用常规设置默认模型: %+v", options)
	}
	if options.Env["NEXUS_RUNTIME_PROVIDER"] != "deepseek" {
		t.Fatalf("runtime 未使用常规设置默认 Provider: %+v", options.Env)
	}
	if options.Runtime.Kind != agentclient.RuntimeNXS {
		t.Fatalf("runtime 未使用常规设置中的 nxs: %+v", options)
	}
}

func TestServiceHandleChatBypassPermissionsKeepsQuestionChannel(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	maxTurns := 4
	agentValue, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			PermissionMode: "bypassPermissions",
			MaxTurns:       &maxTurns,
			SettingSources: []string{"project"},
		},
	})
	if err != nil || agentValue == nil {
		t.Fatalf("更新 agent 失败: value=%+v err=%v", agentValue, err)
	}

	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-bypass",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-bypass")
	sessionKey := "agent:nexus:ws:dm:bypass"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 bypass 权限处理器",
		RoundID:    "round-bypass",
		ReqID:      "round-bypass",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Runtime.PermissionMode != sdkpermission.ModeBypassPermissions {
		t.Fatalf("bypass 权限模式未透传: %+v", options)
	}
	if options.Callbacks.PermissionHandler == nil {
		t.Fatalf("bypass 权限模式应保留 AskUserQuestion 交互通道: %+v", options)
	}
}

func TestServiceHandleChatUsesExplicitProvider(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "kimi",
		DisplayName: "Kimi",
		AuthToken:   "kimi-token",
		BaseURL:     "https://api.moonshot.cn/anthropic",
		Enabled:     true,
	}, "kimi-k2.5", false)

	created, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{
		Name: "显式 Provider 助手",
		Options: &protocol.Options{
			Provider: "kimi",
			Model:    "kimi-k2.5",
		},
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-explicit-provider",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	sessionKey := "agent:" + created.AgentID + ":ws:dm:explicit-provider"
	sender := newDMTestSender("sender-explicit-provider")
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		AgentID:    created.AgentID,
		Content:    "测试显式 provider",
		RoundID:    "round-explicit-provider",
		ReqID:      "round-explicit-provider",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Env["ANTHROPIC_MODEL"] != "kimi-k2.5" {
		t.Fatalf("显式 provider 未命中新 provider model: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_BASE_URL"] != "https://api.moonshot.cn/anthropic" {
		t.Fatalf("显式 provider 未命中新 provider base_url: %+v", options.Env)
	}
	if !options.IncludePartialMessages {
		t.Fatalf("显式 provider runtime 未开启 partial messages: %+v", options)
	}
}

func TestServiceHandleChatUsesPersistedSessionIDAsResume(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-resume",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-resume")
	sessionKey := "agent:nexus:ws:dm:resume-chat"
	permission.BindSession(sessionKey, sender)

	resumeID := "11111111-1111-4111-8111-111111111111"
	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	writeTranscriptFixture(t, workspacePath, resumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "11000000-0000-4000-8000-000000000001",
			"sessionId": resumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "之前的消息",
			},
		},
	})
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &resumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Resume Chat",
		MessageCount: 0,
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("预写入会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 resume",
		RoundID:    "round-resume",
		ReqID:      "round-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != resumeID {
		t.Fatalf("runtime 未将持久化 session_id 作为 resume 透传: %+v", options)
	}
}

func TestServiceHandleChatDoesNotPersistSDKSessionIDWithoutTranscript(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = "11111111-2222-4222-8222-111111111111"
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-without-transcript",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-without-transcript")
	sessionKey := "agent:nexus:ws:dm:without-transcript"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 transcript 未落盘不写 resume",
		RoundID:    "round-without-transcript",
		ReqID:      "round-without-transcript",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if strings.TrimSpace(options.Session.ResumeID) != "" {
		t.Fatalf("新 DM 会话不应携带 resume: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if sessionValue.SessionID != nil && strings.TrimSpace(*sessionValue.SessionID) != "" {
		t.Fatalf("transcript 未落盘时不应写入 sdk session_id: %+v", sessionValue)
	}
}

func TestServiceHandleChatRetriesWithoutStaleSDKSessionWhenResumeConnectFails(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = "sdk-fresh-after-stale"
	client.connectErrors = []error{agentclient.ErrNotConnected}
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-stale-resume-retry",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	roomStore := &fakeDMRoomSessionStore{}
	service.SetRoomSessionStore(roomStore)
	sender := newDMTestSender("sender-stale-resume-retry")
	sessionKey := "agent:nexus:ws:dm:stale-resume-retry"
	permission.BindSession(sessionKey, sender)

	staleResumeID := "22222222-2222-4222-8222-222222222222"
	roomSessionID := "room-session-stale-resume-1"
	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	writeTranscriptFixture(t, workspacePath, staleResumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "22000000-0000-4000-8000-000000000001",
			"sessionId": staleResumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧会话消息",
			},
		},
	})
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:    sessionKey,
		AgentID:       cfg.DefaultAgentID,
		SessionID:     &staleResumeID,
		RoomSessionID: &roomSessionID,
		ChannelType:   "websocket",
		ChatType:      "dm",
		Status:        "active",
		CreatedAt:     now,
		LastActivity:  now,
		Title:         "Stale Resume Retry",
		Options: map[string]any{
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "glm-5.1",
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入 stale resume 会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 stale resume 自动恢复",
		RoundID:    "round-stale-resume-retry",
		ReqID:      "round-stale-resume-retry",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	firstOptions := factory.OptionAt(0)
	if firstOptions.Session.ResumeID != staleResumeID {
		t.Fatalf("首次 runtime connect 应携带持久化 resume: %+v", firstOptions)
	}
	retryOptions := factory.OptionAt(1)
	if retryOptions.Session.ResumeID != "" {
		t.Fatalf("stale resume 连接失败后重试不应继续携带 resume: %+v", retryOptions)
	}

	client.mu.Lock()
	connectCalls := client.connectCalls
	disconnectCalls := client.disconnectCalls
	client.mu.Unlock()
	if connectCalls != 2 {
		t.Fatalf("stale resume 应触发一次无 resume 重试，connectCalls=%d", connectCalls)
	}
	if disconnectCalls == 0 {
		t.Fatal("重试前应清理 runtime manager 中的旧 client")
	}

	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != client.sessionID {
		t.Fatalf("新 sdk session_id 未回写: %+v", sessionValue)
	}
	updates := roomStore.Updates()
	if len(updates) != 2 {
		t.Fatalf("room sdk_session_id 应先清空再回写新值: %+v", updates)
	}
	if updates[0].roomSessionID != roomSessionID || updates[0].sdkSessionID != "" {
		t.Fatalf("首次 room sdk_session_id 更新应清空 stale 值: %+v", updates)
	}
	if updates[1].roomSessionID != roomSessionID || updates[1].sdkSessionID != client.sessionID {
		t.Fatalf("第二次 room sdk_session_id 更新应写入新值: %+v", updates)
	}
}

func TestServiceHandleChatKeepsSDKSessionResumeWhenRuntimeFingerprintMissingAndTranscriptExists(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	resumeID := "33333333-3333-4333-8333-333333333333"
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = resumeID
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-legacy-resume",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	sender := newDMTestSender("sender-legacy-resume")
	sessionKey := "agent:nexus:ws:dm:legacy-resume-chat"
	permission.BindSession(sessionKey, sender)

	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	writeTranscriptFixture(t, workspacePath, resumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "33000000-0000-4000-8000-000000000001",
			"sessionId": resumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧版无指纹会话",
			},
		},
	})
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &resumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Legacy Resume Chat",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("预写入 legacy 会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 legacy resume",
		RoundID:    "round-legacy-resume",
		ReqID:      "round-legacy-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != resumeID {
		t.Fatalf("legacy session 缺少 runtime 指纹时仍应 resume: %+v", options)
	}
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 应使用当前 provider model: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != resumeID {
		t.Fatalf("legacy resume 不应被清空或替换: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeProvider] != "glm" {
		t.Fatalf("legacy resume 后应补写 runtime provider 指纹: %+v", sessionValue.Options)
	}
	if sessionValue.Options[protocol.OptionRuntimeModel] != "glm-5.1" {
		t.Fatalf("legacy resume 后应补写 runtime model 指纹: %+v", sessionValue.Options)
	}
}

func TestServiceHandleChatReusesSDKSessionWhenRuntimeModelFingerprintDiffersWithTranscript(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	permission := permissionctx.NewContext()
	resumeID := "77777777-7777-4777-8777-777777777777"
	client := newFakeDMClient()
	client.sessionID = resumeID
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-resume-model-change",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	sender := newDMTestSender("sender-stale-model")
	sessionKey := "agent:nexus:ws:dm:stale-model"
	permission.BindSession(sessionKey, sender)

	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	writeTranscriptFixture(t, workspacePath, resumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "77000000-0000-4000-8000-000000000001",
			"sessionId": resumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧模型会话消息",
			},
		},
	})
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &resumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Stale Model",
		Options: map[string]any{
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "old-model",
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试模型变更仍 resume",
		RoundID:    "round-stale-model",
		ReqID:      "round-stale-model",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != resumeID {
		t.Fatalf("runtime 模型变更不应阻止 transcript resume: %+v", options)
	}
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 应使用当前 provider model: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != resumeID {
		t.Fatalf("模型变更 resume 不应替换 sdk session_id: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeModel] != "glm-5.1" {
		t.Fatalf("runtime model 指纹未回写: %+v", sessionValue.Options)
	}
}

func TestServiceHandleChatSkipsStaleSDKSessionWhenRuntimeKindFingerprintDiffersWithoutTranscript(t *testing.T) {
	isolateDMRuntimeKindEnv(t)

	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = "sdk-new-claude-kind"
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-new-claude-kind",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "claude",
	}})
	sender := newDMTestSender("sender-stale-runtime-kind")
	sessionKey := "agent:nexus:ws:dm:stale-runtime-kind"
	permission.BindSession(sessionKey, sender)

	staleResumeID := "44444444-4444-4444-8444-444444444444"
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID), protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &staleResumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Stale Runtime Kind",
		Options: map[string]any{
			protocol.OptionRuntimeKind:     "nxs",
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "glm-5.1",
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入 runtime kind stale 会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 runtime kind 变更不 resume",
		RoundID:    "round-stale-runtime-kind",
		ReqID:      "round-stale-runtime-kind",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != "" {
		t.Fatalf("runtime kind 变更后不应 resume 过期 sdk session: %+v", options)
	}
	if options.Runtime.Kind != agentclient.RuntimeClaude {
		t.Fatalf("runtime 应切到 Claude: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != "sdk-new-claude-kind" {
		t.Fatalf("新 sdk session_id 未回写: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeKind] != "claude" {
		t.Fatalf("runtime kind 指纹未回写: %+v", sessionValue.Options)
	}
}

func TestServiceHandleChatReusesSDKSessionWhenRuntimeKindSwitchHasSharedTranscript(t *testing.T) {
	isolateDMRuntimeKindEnv(t)

	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	permission := permissionctx.NewContext()
	sharedResumeID := "55555555-5555-4555-8555-555555555555"
	client := newFakeDMClient()
	client.sessionID = sharedResumeID
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: sharedResumeID,
				UUID:      "result-shared-runtime-resume",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "claude",
	}})
	sender := newDMTestSender("sender-shared-runtime-resume")
	sessionKey := "agent:nexus:ws:dm:shared-runtime-resume"
	permission.BindSession(sessionKey, sender)

	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	writeTranscriptFixture(t, workspacePath, sharedResumeID, []map[string]any{
		{
			"type":       "user",
			"uuid":       "55000000-0000-4000-8000-000000000001",
			"sessionId":  sharedResumeID,
			"parentUuid": nil,
			"timestamp":  "2026-06-09T00:00:00Z",
			"cwd":        workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "继续之前的任务",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "55000000-0000-4000-8000-000000000002",
			"sessionId":  sharedResumeID,
			"parentUuid": "55000000-0000-4000-8000-000000000001",
			"timestamp":  "2026-06-09T00:00:01Z",
			"message": map[string]any{
				"role": "assistant",
				"content": []any{map[string]any{
					"type": "text",
					"text": "旧 runtime 已经写入 transcript",
				}},
			},
		},
	})
	if exists, err := service.history.TranscriptSessionExists(workspacePath, sharedResumeID); err != nil || !exists {
		t.Fatalf("shared transcript fixture 未被 history store 识别: exists=%v err=%v", exists, err)
	}

	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &sharedResumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Shared Runtime Resume",
		Options: map[string]any{
			protocol.OptionRuntimeKind:     "nxs",
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "glm-5.1",
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入 shared runtime resume 会话 meta 失败: %v", err)
	}
	if sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey); stringPointer(t, sessionValue.SessionID) != sharedResumeID {
		t.Fatalf("预写入 session meta 未被 DM service 识别: %+v", sessionValue)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 runtime 切换共享 resume",
		RoundID:    "round-shared-runtime-resume",
		ReqID:      "round-shared-runtime-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != sharedResumeID {
		t.Fatalf("runtime kind 切换时应复用共享 transcript resume: %+v", options)
	}
	if options.Runtime.Kind != agentclient.RuntimeClaude {
		t.Fatalf("runtime 应切到 Claude: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != sharedResumeID {
		t.Fatalf("共享 resume 不应替换 sdk session_id: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeKind] != "claude" {
		t.Fatalf("runtime kind 指纹未更新为新 runtime: %+v", sessionValue.Options)
	}
}

func TestServiceResolveReusableSDKSessionIDReusesSharedTranscriptRuntimeSwitch(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	service := NewService(cfg, agentService, runtimectx.NewManagerWithFactory(&fakeDMFactory{}), permissionctx.NewContext())

	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	resumeID := "66666666-6666-4666-8666-666666666666"
	writeTranscriptFixture(t, workspacePath, resumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "66000000-0000-4000-8000-000000000001",
			"sessionId": resumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "继续",
			},
		},
	})

	got := service.resolveReusableSDKSessionID(context.Background(), workspacePath, protocol.Session{
		SessionKey: "agent:nexus:ws:dm:resolve-shared-runtime-resume",
		AgentID:    cfg.DefaultAgentID,
		SessionID:  &resumeID,
		Options: map[string]any{
			protocol.OptionRuntimeKind:     "nxs",
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "glm-5.1",
		},
	}, "glm", agentclient.Options{
		Model: "glm-5.1",
		Session: agentclient.SessionOptions{
			ResumeID: resumeID,
		},
		Runtime: agentclient.RuntimeOptions{
			Kind: agentclient.RuntimeClaude,
		},
	})
	if got != resumeID {
		t.Fatalf("runtime 切换存在共享 transcript 时应复用 resume: got=%q want=%q", got, resumeID)
	}
}

func TestServiceHandleInterruptEmitsInterruptedRound(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-interrupted",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-1")
	sessionKey := "agent:nexus:ws:dm:test-interrupt"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "你好",
		RoundID:    "round-2",
		ReqID:      "round-2",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	assertContainsRoundStatus(t, events, "interrupted")
	assertContainsResultSubtype(t, events, "interrupted")

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	client.mu.Unlock()
	if interruptCalls == 0 {
		t.Fatal("期望 fake client 收到 interrupt")
	}

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	writeTranscriptFixture(t, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "interrupt-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
	})
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("中断后消息数量不正确: got=%d want=2 messages=%+v", len(messages), messages)
	}
	if messages[1]["role"] != "assistant" {
		t.Fatalf("中断后应返回合成 assistant: %+v", messages)
	}
	summary, ok := messages[1]["result_summary"].(map[string]any)
	if !ok || summary["subtype"] != "interrupted" {
		t.Fatalf("中断后未挂载 interrupted result_summary: %+v", messages)
	}
}

func TestServiceHandleInterruptCleansStaleRuntimeWhenClientInterruptFails(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.interruptErrors = []error{errors.New("os: process already finished")}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-interrupt-stale")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-stale"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "停止一个已经退出的进程",
		RoundID:    "round-interrupt-stale",
		ReqID:      "round-interrupt-stale",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("失效进程中断应被业务层清理而不是返回错误: %v", err)
	}
	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeSessionStatus && event.Data["is_generating"] == false
	})
	if len(runtimeManager.GetRunningRoundIDs(sessionKey)) != 0 {
		t.Fatal("失效进程清理后不应残留 running round")
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if sessionValue.Status != "closed" || sessionValue.IsActive {
		t.Fatalf("失效进程清理后 session meta 应关闭: %+v events=%+v", sessionValue, events)
	}
}

func TestServiceHandleChatQueuesRunningRoundByDefault(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-queue-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-queue")
	sessionKey := "agent:nexus:ws:dm:test-queue"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先做一个长任务",
		RoundID:    "round-queue-1",
		ReqID:      "round-queue-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "这是补充要求",
		RoundID:    "round-queue-2",
		ReqID:      "round-queue-2",
	}); err != nil {
		t.Fatalf("第二条排队消息 HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "round-queue-2"
	})

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("默认排队不应中断运行中 DM round: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 1 ||
		!strings.Contains(sentContents[0], "这是补充要求") ||
		!strings.Contains(sentContents[0], "<nexus_runtime_context>") {
		t.Fatalf("运行中 DM round 未收到排队输入: %+v", sentContents)
	}
	if len(factory.options) != 1 {
		t.Fatalf("排队输入不应创建新 runtime client: got=%d want=1", len(factory.options))
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

func TestServiceHandleChatGuidePolicyQueuesHookGuidance(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-guide-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-guide")
	sessionKey := "agent:nexus:ws:dm:test-guide"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查一下项目结构",
		RoundID:    "round-guide-1",
		ReqID:      "round-guide-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:           sessionKey,
		Content:              "等工具结果回来后优先看错误日志",
		RoundID:              "round-guide-2",
		ReqID:                "round-guide-2",
		DeliveryPolicy:       protocol.ChatDeliveryPolicyGuide,
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("引导消息 HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "round-guide-2"
	})
	guidanceEvents := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["role"] == "system"
	})
	guidanceEvent := guidanceEvents[len(guidanceEvents)-1]
	if guidanceEvent.Data["round_id"] != "round-guide-1" || guidanceEvent.Data["message_id"] != "round-guide-2" {
		t.Fatalf("引导消息应归入运行中的 round: %+v", guidanceEvent.Data)
	}
	if guidanceEvent.DeliveryMode != "ephemeral" {
		t.Fatalf("引导消息只应作为实时展示事件广播: %+v", guidanceEvent)
	}
	metadata, _ := guidanceEvent.Data["metadata"].(map[string]any)
	if metadata["subtype"] != "guided_input" {
		t.Fatalf("引导消息缺少 typed metadata: %+v", guidanceEvent.Data)
	}

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("引导不应中断运行中 DM round: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 0 {
		t.Fatalf("引导不应走普通 streaming input: %+v", sentContents)
	}
	if count := runtimeManager.PendingGuidanceCount(sessionKey); count != 1 {
		t.Fatalf("引导输入未登记到运行时队列: count=%d", count)
	}
	if len(factory.options) != 1 {
		t.Fatalf("引导不应创建新 runtime client: got=%d want=1", len(factory.options))
	}
	if matchers := factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse]; len(matchers) == 0 {
		t.Fatalf("runtime options 未挂载 PostToolUse 引导 hook: %+v", factory.options[0].Hooks)
	}
	rows := readDMSessionHistory(t, cfg, service, sessionKey)
	for _, row := range rows {
		if row["message_id"] == "round-guide-2" {
			t.Fatalf("引导消息不应直接写入 overlay 历史，历史回放应来自 runtime transcript: %+v", rows)
		}
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

func TestServiceInputQueueGuideWaitsForPostToolUse(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-guide-input-queue")
	sessionKey := "agent:nexus:ws:dm:test-guide-input-queue"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查项目",
		RoundID:    "round-guide-input-queue-1",
		ReqID:      "round-guide-input-queue-1",
	}); err != nil {
		t.Fatalf("启动运行中 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey:     sessionKey,
		Action:         "enqueue",
		Content:        "路径发给我吧",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("写入 DM 待发送队列失败: %v", err)
	}
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("待发送队列应保留一条消息: %+v", items)
	}
	itemID := items[0].ID

	if err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey: sessionKey,
		Action:     "guide",
		ItemID:     itemID,
	}); err != nil {
		t.Fatalf("标记 DM 引导队列失败: %v", err)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取标记后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].ID != itemID ||
		items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		items[0].RootRoundID != "" {
		t.Fatalf("点击引导后应保留为可跨 round 注入的队列项: %+v", items)
	}

	var additionalContext string
	for _, matcher := range factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(context.Background(), sdkhook.Input{
				EventName: sdkhook.EventPostToolUse,
			}, "tool-1")
			if hookErr != nil {
				t.Fatalf("执行 PostToolUse hook 失败: %v", hookErr)
			}
			if output.SpecificOutput != nil && output.SpecificOutput.AdditionalContext != "" {
				text := output.SpecificOutput.AdditionalContext
				additionalContext = text
			}
		}
	}
	if !strings.Contains(additionalContext, "路径发给我吧") ||
		!strings.Contains(additionalContext, "queue_"+itemID) {
		t.Fatalf("PostToolUse hook 未注入队列引导: %q", additionalContext)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取消费后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("PostToolUse 真正注入后才应消费队列项: %+v", items)
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

func TestServiceGoalContinuationDefersToQueuedUserInput(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	sentPrompt := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) {
		sentPrompt <- prompt
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-goal-defer-queue",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "queued done",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sessionKey := "agent:nexus:ws:dm:test-goal-defer-queue"
	normalizedSessionKey, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, cfg.DefaultAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if normalizedSessionKey != sessionKey {
		t.Fatalf("normalized session key = %q, want %q", normalizedSessionKey, sessionKey)
	}
	if _, err = service.inputQueue.Enqueue(location, protocol.InputQueueItem{
		Scope:          protocol.InputQueueScopeDM,
		SessionKey:     sessionKey,
		AgentID:        cfg.DefaultAgentID,
		Source:         protocol.InputQueueSourceUser,
		Content:        "用户排队输入应先执行",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatal(err)
	}

	if !service.ShouldDeferGoalContinuation(context.Background(), sessionKey, cfg.DefaultAgentID) {
		t.Fatal("Goal continuation should defer while queued user input exists")
	}
	select {
	case prompt := <-sentPrompt:
		if !strings.Contains(prompt, "用户排队输入应先执行") {
			t.Fatalf("prompt = %q, want queued user input", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queued user input was not dispatched before Goal continuation")
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %#v, want queued input dispatched", items)
	}
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
}

func TestServiceGoalContinuationDefersInPlanMode(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	if _, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{PermissionMode: string(sdkpermission.ModePlan)},
	}); err != nil {
		t.Fatalf("更新 agent plan mode 失败: %v", err)
	}
	service := NewService(cfg, agentService, runtimectx.NewManager(), permissionctx.NewContext())
	sessionKey := "agent:nexus:ws:dm:test-goal-defer-plan"

	if !service.ShouldDeferGoalContinuation(context.Background(), sessionKey, cfg.DefaultAgentID) {
		t.Fatal("Goal continuation should defer while the target agent is in plan mode")
	}
}

func TestServiceHandleChatInterruptPolicyStopsRunningRound(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, prompt string) {
		if strings.Contains(prompt, "第二轮") {
			go func() {
				client.messages <- sdkprotocol.ReceivedMessage{
					Type:      sdkprotocol.MessageTypeResult,
					SessionID: client.sessionID,
					UUID:      "result-interrupt-policy",
					Result: &sdkprotocol.ResultMessage{
						Subtype:    "success",
						DurationMS: 1,
						NumTurns:   1,
						Result:     "ok",
					},
				}
			}()
		}
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-interrupt-policy-old",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-interrupt-policy")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-policy"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第一轮",
		RoundID:    "round-interrupt-policy-1",
		ReqID:      "round-interrupt-policy-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:     sessionKey,
		Content:        "第二轮",
		RoundID:        "round-interrupt-policy-2",
		ReqID:          "round-interrupt-policy-2",
		DeliveryPolicy: protocol.ChatDeliveryPolicyInterrupt,
	}); err != nil {
		t.Fatalf("打断策略 HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "round-interrupt-policy-2" &&
			event.Data["status"] == "finished"
	})
	assertContainsRoundStatus(t, events, "finished")

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls == 0 {
		t.Fatal("打断策略应中断运行中 DM round")
	}
	if len(sentContents) != 0 {
		t.Fatalf("打断策略不应走 streaming input: %+v", sentContents)
	}
}

func TestServiceHandleInterruptCoercesTerminalErrorIntoInterrupted(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-error-after-interrupt",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "error",
					DurationMS:    8,
					DurationAPIMS: 123,
					NumTurns:      2,
					Result:        "",
					IsError:       true,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-interrupt-error")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-error"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "停止测试",
		RoundID:    "round-interrupt-error",
		ReqID:      "round-interrupt-error",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	assertContainsRoundStatus(t, events, "interrupted")
	assertContainsResultSubtype(t, events, "interrupted")

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	writeTranscriptFixture(t, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "interrupt-error-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "停止测试",
			},
		},
	})
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("中断错误收口后消息数量不正确: got=%d want=2 messages=%+v", len(messages), messages)
	}
	summary, ok := messages[1]["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("中断错误未挂载 result_summary: %+v", messages)
	}
	if summary["subtype"] != "interrupted" {
		t.Fatalf("中断错误应收口为 interrupted: %+v", summary)
	}
	if _, exists := summary["result"]; exists {
		t.Fatalf("中断错误不应再补默认文案: %+v", summary)
	}
}

func TestServiceHandleChatAfterInterruptKeepsSameClientAndConsumesExplicitStop(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()

	client := newFakeDMClient()
	client.sessionID = "sdk-interrupt-old"
	queryCount := 0
	client.onQuery = func(_ context.Context, _ string) {
		queryCount++
		if queryCount != 2 {
			return
		}
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-after-resume",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-after-interrupt",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-reconnect")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-reconnect"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第一轮",
		RoundID:    "round-interrupt-1",
		ReqID:      "round-interrupt-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第二轮",
		RoundID:    "round-interrupt-2",
		ReqID:      "round-interrupt-2",
	}); err != nil {
		t.Fatalf("第二轮 HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == "finished" &&
			event.Data["round_id"] == "round-interrupt-2"
	})

	if len(factory.options) != 1 {
		t.Fatalf("只应创建一次 runtime client，第二轮应复用现有 client: got=%d want=1", len(factory.options))
	}
	if len(client.reconfigureOps) == 0 {
		t.Fatalf("第二轮应复用 client 并执行 reconfigure")
	}
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["round_id"] != "round-interrupt-2" {
			continue
		}
		summary, ok := event.Data["result_summary"].(map[string]any)
		if !ok {
			continue
		}
		if summary["subtype"] == "interrupted" {
			t.Fatalf("第二轮不应消费上一轮残留结果: %+v", events)
		}
	}
}

func TestServiceHandleChatPersistsStructuredChannelMetadata(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-structured",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-structured")
	sessionKey := "agent:nexus:tg:group:-100123456:topic:12"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "结构化入口",
		RoundID:    "round-structured",
		ReqID:      "round-structured",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	item, _, err := service.files.FindSession([]string{filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)}, sessionKey)
	if err != nil {
		t.Fatalf("读取 session 元数据失败: %v", err)
	}
	if item == nil {
		t.Fatal("session 元数据不存在")
	}
	if item.ChannelType != "telegram" || item.ChatType != "group" {
		t.Fatalf("session 元数据不正确: %+v", *item)
	}
}

func TestServiceHandleChatFailsRoundWhenStreamEndsWithoutTerminalResult(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"id":    "assistant-premature",
							"model": "sonnet",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type":     "thinking",
							"thinking": "先分析",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{
							"type":     "thinking_delta",
							"thinking": " 再收口",
						},
					},
				},
			}
			close(client.messages)
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-premature")
	sessionKey := "agent:nexus:ws:dm:premature-close"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试提前结束",
		RoundID:    "round-premature",
		ReqID:      "round-premature",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "error"
	})

	assertContainsRoundStatus(t, events, "error")
	assertContainsStreamEventType(t, events, "message_start")
	assertContainsStreamEventType(t, events, "content_block_delta")
	assertContainsResultSubtype(t, events, "error")
	assertContainsErrorEventForMessage(t, events, "assistant-premature")
}

func newDMTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18032,
		ProjectName:    "nexus-dm-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func isolateDMRuntimeKindEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NEXUS_AGENT_RUNTIME_KIND", "")
	t.Setenv("NEXUS_AGENT_RUNTIME", "")
}

var dmTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func mustFindDMSession(
	t *testing.T,
	service *Service,
	cfg config.Config,
	sessionKey string,
) (protocol.Session, string) {
	t.Helper()
	item, workspacePath, err := service.files.FindSession([]string{filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)}, sessionKey)
	if err != nil {
		t.Fatalf("读取 session 元数据失败: %v", err)
	}
	if item == nil {
		t.Fatalf("session 元数据不存在: %s", sessionKey)
	}
	return *item, workspacePath
}

func readDMSessionHistory(
	t *testing.T,
	cfg config.Config,
	service *Service,
	sessionKey string,
) []protocol.Message {
	t.Helper()
	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	historyStore := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	rows, err := historyStore.ReadMessages(workspacePath, sessionValue, nil)
	if err != nil {
		t.Fatalf("读取 transcript 历史失败: %v", err)
	}
	return rows
}

func writeTranscriptFixture(
	t *testing.T,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	if strings.TrimSpace(sessionID) == "" {
		t.Fatal("session_id 为空，无法写入 transcript fixture")
	}
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 目录失败: %v", err)
	}
	projectDir := filepath.Join(
		os.Getenv("NEXUS_CONFIG_DIR"),
		"projects",
		sanitizeDMTranscriptPath(canonicalizeDMTranscriptPath(workspacePath)),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("创建 transcript fixture 失败: %v", err)
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 transcript fixture 失败: %v", err)
		}
	}
}

func canonicalizeDMTranscriptPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	absolutePath, err := filepath.Abs(path)
	if err == nil {
		path = absolutePath
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func sanitizeDMTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := dmTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + dmTranscriptHash(path)
}

func dmTranscriptHash(value string) string {
	var hash int32
	for _, character := range value {
		hash = hash*31 + int32(character)
	}

	number := int64(hash)
	if number < 0 {
		number = -number
	}
	if number == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, 8)
	for number > 0 {
		result = append(result, digits[number%36])
		number /= 36
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
}

func anyToInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return 0
	}
}

func stringPointer(t *testing.T, value *string) string {
	t.Helper()
	if value == nil || strings.TrimSpace(*value) == "" {
		t.Fatal("session_id 未持久化")
	}
	return strings.TrimSpace(*value)
}

func migrateDMSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, dmMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func dmMigrationDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}

func waitForEvent(t *testing.T, events <-chan protocol.EventMessage, eventType protocol.EventType, status string) {
	t.Helper()
	_ = collectEventsUntil(t, events, func(event protocol.EventMessage) bool {
		if event.EventType != eventType {
			return false
		}
		if status == "" {
			return true
		}
		return event.Data["status"] == status
	})
}

func collectEventsUntil(
	t *testing.T,
	events <-chan protocol.EventMessage,
	stop func(protocol.EventMessage) bool,
) []protocol.EventMessage {
	t.Helper()
	result := make([]protocol.EventMessage, 0, 8)
	timeout := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			result = append(result, event)
			if stop(event) {
				return result
			}
		case <-timeout:
			t.Fatalf("等待事件超时，当前事件: %+v", result)
		}
	}
}

func waitForDMRuntimeIdle(t *testing.T, runtimeManager *runtimectx.Manager, sessionKey string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		if len(runtimeManager.GetRunningRoundIDs(sessionKey)) == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("等待 DM round 结束超时，running_rounds=%+v", runtimeManager.GetRunningRoundIDs(sessionKey))
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func assertEventTypes(t *testing.T, events []protocol.EventMessage, expected []protocol.EventType) {
	t.Helper()
	if len(events) < len(expected) {
		t.Fatalf("事件数量不足: got=%d want>=%d", len(events), len(expected))
	}
	for index, eventType := range expected {
		if events[index].EventType != eventType {
			t.Fatalf("第 %d 个事件类型不正确: got=%s want=%s all=%+v", index, events[index].EventType, eventType, events)
		}
	}
}

func assertContainsRoundStatus(t *testing.T, events []protocol.EventMessage, status string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == status {
			return
		}
	}
	t.Fatalf("未找到 round_status=%s: %+v", status, events)
}

func assertContainsStreamEventType(t *testing.T, events []protocol.EventMessage, streamType string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeStream && event.Data["type"] == streamType {
			return
		}
	}
	t.Fatalf("未找到 stream.type=%s: %+v", streamType, events)
}

func assertContainsResultSubtype(t *testing.T, events []protocol.EventMessage, subtype string) {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["role"] == "result" && event.Data["subtype"] == subtype {
			return
		}
		if event.Data["role"] == "assistant" {
			summary, ok := event.Data["result_summary"].(map[string]any)
			if ok && summary["subtype"] == subtype {
				return
			}
		}
	}
	t.Fatalf("未找到 result.subtype=%s: %+v", subtype, events)
}

func assertContainsErrorEventForMessage(t *testing.T, events []protocol.EventMessage, messageID string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeError && event.MessageID == messageID {
			return
		}
	}
	t.Fatalf("未找到绑定消息 %s 的 error 事件: %+v", messageID, events)
}

func assertStreamBlockIndex(t *testing.T, events []protocol.EventMessage, blockType string, expectedIndex int) {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeStream {
			continue
		}
		contentBlock, ok := event.Data["content_block"].(map[string]any)
		if !ok || contentBlock["type"] != blockType {
			continue
		}
		if event.Data["index"] != expectedIndex {
			t.Fatalf("%s stream index 不正确: got=%v want=%d event=%+v", blockType, event.Data["index"], expectedIndex, event)
		}
		return
	}
	t.Fatalf("未找到 block_type=%s 的 stream 事件: %+v", blockType, events)
}

func findAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage || event.MessageID != messageID {
			continue
		}
		if event.Data["role"] != "assistant" {
			continue
		}
		return protocol.Message(event.Data)
	}
	t.Fatalf("未找到 assistant message_id=%s 的 durable 消息: %+v", messageID, events)
	return nil
}

func findLatestAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
	t.Helper()
	var latest protocol.Message
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage || event.MessageID != messageID {
			continue
		}
		if event.Data["role"] != "assistant" {
			continue
		}
		latest = protocol.Message(event.Data)
	}
	if latest != nil {
		return latest
	}
	t.Fatalf("未找到 assistant message_id=%s 的最后 durable 消息: %+v", messageID, events)
	return nil
}

func contentBlocksFromPayload(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rawBlocks, ok := payload["content"]
	if !ok {
		t.Fatalf("消息缺少 content: %+v", payload)
	}
	switch typed := rawBlocks.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("content block 类型不正确: %+v", payload)
			}
			result = append(result, block)
		}
		return result
	default:
		t.Fatalf("content 类型不正确: %+v", payload)
		return nil
	}
}

func assertContentBlockTypes(t *testing.T, blocks []map[string]any, expected []string) {
	t.Helper()
	if len(blocks) != len(expected) {
		t.Fatalf("content block 数量不正确: got=%d want=%d blocks=%+v", len(blocks), len(expected), blocks)
	}
	for index, expectedType := range expected {
		if blocks[index]["type"] != expectedType {
			t.Fatalf("第 %d 个 content block 类型不正确: got=%v want=%s blocks=%+v", index, blocks[index]["type"], expectedType, blocks)
		}
	}
}

func assertToolResultIDs(t *testing.T, blocks []map[string]any, expected []string) {
	t.Helper()
	resultIDs := make([]string, 0, len(expected))
	for _, block := range blocks {
		if block["type"] != "tool_result" {
			continue
		}
		resultIDs = append(resultIDs, anyToString(block["tool_use_id"]))
	}
	if len(resultIDs) != len(expected) {
		t.Fatalf("tool_result 数量不正确: got=%+v want=%+v blocks=%+v", resultIDs, expected, blocks)
	}
	for index, expectedID := range expected {
		if resultIDs[index] != expectedID {
			t.Fatalf("tool_result 顺序不正确: got=%+v want=%+v blocks=%+v", resultIDs, expected, blocks)
		}
	}
}

func anyToString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
