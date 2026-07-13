package dm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	_ "modernc.org/sqlite"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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
