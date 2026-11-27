// chat_delivery_test.go 覆盖 Room 黑盒测试夹具、消息投递、运行时交互与定向消息测试。
package realtime_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

var NewServiceWithFactory = realtimesvc.NewServiceWithFactory

func TestMain(m *testing.M) {
	os.Exit(handlertest.RunWithMinimalAppRoot(m))
}

func registerRealtimeServiceCleanup(
	t *testing.T,
	service *realtimesvc.Service,
	runtimeManager *runtimectx.Manager,
	sessionKey string,
) {
	t.Helper()
	stopSchedulers, err := service.StartDelayedWakeScheduler(context.Background())
	if err != nil {
		t.Fatalf("启动 Room 测试调度器失败: %v", err)
	}
	t.Cleanup(func() {
		stopSchedulers()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := runtimeManager.CloseSession(cleanupCtx, sessionKey); err != nil &&
			!runtimectx.IsRuntimeTransportClosedError(err) {
			t.Errorf("清理 Room 测试 runtime 失败: %v", err)
		}
	})
}

func createSingleAgentGroupRoom(
	ctx context.Context,
	service *roomsvc.Service,
	agentID string,
) (*protocol.ConversationContextAggregate, error) {
	return service.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentID},
	})
}

type fakeRoomClient struct {
	mu             sync.Mutex
	sessionID      string
	connectErr     error
	messages       chan sdkprotocol.ReceivedMessage
	interruptCalls int
	disconnects    int
	stoppedTasks   []string
	taskMessages   []string
	queryPrompts   []string
	sentContents   []string
	onQuery        func(context.Context, string) error
	onInterrupt    func(context.Context)
}

func newFakeRoomClient() *fakeRoomClient {
	return &fakeRoomClient{
		sessionID: "room-sdk-session",
		messages:  make(chan sdkprotocol.ReceivedMessage, 32),
	}
}

func sendFakeRoomThinkingStream(client *fakeRoomClient, messageID string, thinking string) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: client.sessionID,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": messageID},
		}},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: client.sessionID,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type": "thinking", "thinking": thinking,
			},
		}},
	}
}

func (c *fakeRoomClient) Connect(context.Context) error { return c.connectErr }

func (c *fakeRoomClient) Query(ctx context.Context, prompt string) error {
	c.mu.Lock()
	c.queryPrompts = append(c.queryPrompts, prompt)
	c.mu.Unlock()
	if c.onQuery != nil {
		return c.onQuery(ctx, prompt)
	}
	return nil
}

func (c *fakeRoomClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeRoomClient) SendContent(_ context.Context, content any, _ *string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if text, ok := content.(string); ok {
		c.sentContents = append(c.sentContents, text)
	}
	return nil
}

func (c *fakeRoomClient) Interrupt(ctx context.Context) error {
	c.mu.Lock()
	c.interruptCalls++
	callback := c.onInterrupt
	c.mu.Unlock()
	if callback != nil {
		callback(ctx)
	}
	return nil
}

func (c *fakeRoomClient) StopTask(_ context.Context, taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stoppedTasks = append(c.stoppedTasks, taskID)
	return nil
}

func (c *fakeRoomClient) SendTaskMessage(_ context.Context, taskID string, _ string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskMessages = append(c.taskMessages, taskID)
	return nil
}

func (c *fakeRoomClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *fakeRoomClient) SetPermissionMode(context.Context, sdkpermission.Mode) error { return nil }

func (c *fakeRoomClient) Retire() {}

func (c *fakeRoomClient) Disconnect(context.Context) error {
	c.mu.Lock()
	c.disconnects++
	c.mu.Unlock()
	return nil
}

func (c *fakeRoomClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *fakeRoomClient) SessionID() string { return c.sessionID }

type fakeRoomFactory struct {
	mu      sync.Mutex
	clients []*fakeRoomClient
	index   int
	options []agentclient.Options
}

func (f *fakeRoomFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options = append(f.options, options)
	if f.index >= len(f.clients) {
		return newFakeRoomClient()
	}
	client := f.clients[f.index]
	f.index++
	return client
}

func (f *fakeRoomFactory) LastOptions() agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.options) == 0 {
		return agentclient.Options{}
	}
	return f.options[len(f.options)-1]
}

func (f *fakeRoomFactory) Options() []agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]agentclient.Options(nil), f.options...)
}

func sendFakeAssistantResult(client *fakeRoomClient, messageID string, text string) {
	sendFakeAssistantResultWithUsage(client, messageID, text, nil)
}

type fakeRoomTitleScheduler struct {
	mu       sync.Mutex
	requests []titlegen.Request
}

func (s *fakeRoomTitleScheduler) Schedule(_ context.Context, request titlegen.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, request)
}

func (s *fakeRoomTitleScheduler) LastRequest() titlegen.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return titlegen.Request{}
	}
	return s.requests[len(s.requests)-1]
}

func sendFakeTerminalAssistantAndClose(client *fakeRoomClient, messageID string, text string, usage map[string]any) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: client.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         messageID,
				Model:      "sonnet",
				StopReason: "end_turn",
				Usage:      usage,
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: text},
				},
			},
		},
	}
	close(client.messages)
}

func sendFakeAssistantResultWithUsage(client *fakeRoomClient, messageID string, text string, usage map[string]any) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: client.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    messageID,
				Model: "sonnet",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: text},
				},
			},
		},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      messageID + "-result",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
			Result:     "done",
			Usage:      usage,
		},
	}
}

type realtimeTestSender struct {
	key    string
	events chan protocol.EventMessage
}

func newRealtimeTestSender(key string) *realtimeTestSender {
	return &realtimeTestSender{
		key:    key,
		events: make(chan protocol.EventMessage, 64),
	}
}

func (s *realtimeTestSender) Key() string { return s.key }

func (s *realtimeTestSender) IsClosed() bool { return false }

func (s *realtimeTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func collectRoomEventsUntil(
	t *testing.T,
	events <-chan protocol.EventMessage,
	stop func([]protocol.EventMessage, protocol.EventMessage) bool,
) []protocol.EventMessage {
	t.Helper()
	result := make([]protocol.EventMessage, 0, 16)
	timeout := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			result = append(result, event)
			if stop(result, event) {
				return result
			}
		case <-timeout:
			t.Fatalf("等待 Room 事件超时，当前事件: %+v", result)
		}
	}
}

func writeRoomTranscriptFixture(
	t *testing.T,
	ownerUserID string,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	if strings.TrimSpace(sessionID) == "" {
		t.Fatal("session_id 为空，无法写入 room transcript fixture")
	}
	projectDir := filepath.Join(
		appfs.UserRuntimeRoot(ownerUserID),
		"projects",
		workspacestore.TranscriptProjectDirectoryName(workspacePath),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 room transcript 目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("创建 room transcript fixture 失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 room transcript fixture 失败: %v", err)
		}
	}
}

func anyToString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func countRoomResultSubtype(events []protocol.EventMessage, subtype string) int {
	count := 0
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["role"] == "result" && event.Data["subtype"] == subtype {
			count++
			continue
		}
		if event.Data["role"] == "assistant" {
			summary, ok := event.Data["result_summary"].(map[string]any)
			if ok && summary["subtype"] == subtype {
				count++
			}
		}
	}
	return count
}

func findRoomAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
	t.Helper()
	// 同一消息可能被终态投影更新，校验最后到达的快照。
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		if event.EventType != protocol.EventTypeMessage || event.MessageID != messageID {
			continue
		}
		if event.Data["role"] != "assistant" {
			continue
		}
		return protocol.Message(event.Data)
	}
	t.Fatalf("未找到 Room assistant message_id=%s 的 durable 消息: %+v", messageID, events)
	return nil
}

func roomContentBlocksFromPayload(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rawBlocks, ok := payload["content"]
	if !ok {
		t.Fatalf("Room 消息缺少 content: %+v", payload)
	}
	switch typed := rawBlocks.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("Room content block 类型不正确: %+v", payload)
			}
			result = append(result, block)
		}
		return result
	default:
		t.Fatalf("Room content 类型不正确: %+v", payload)
		return nil
	}
}

// 共享 Room 测试夹具。

func createTestAgent(
	t *testing.T,
	service *agentsvc.Service,
	ctx context.Context,
	name string,
) *protocol.Agent {
	t.Helper()

	item, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: name})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	return item
}

func newRoomTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, ".nexus"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18011,
		ProjectName:    "nexus-room-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateRoomSQLite(t *testing.T, databaseURL string) {
	t.Helper()
	handlertest.MigrateSQLiteFromDir(t, databaseURL, roomMigrationDir(t))
}

func newRoomTestAgentService(
	t *testing.T,
	cfg config.Config,
) (*agentsvc.Service, *sql.DB, error) {
	t.Helper()
	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		return nil, nil, err
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭 Room 测试数据库失败: %v", err)
		}
	})
	return agentService, db, nil
}

func roomMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "db", "migrations", "sqlite")
}

// 活跃目标投递测试。

type fakeRoomQuotaChecker struct {
	err error
}

type testRoomGoalSessionOwnershipVerifier struct {
	agentNames map[string]string
}

func (v testRoomGoalSessionOwnershipVerifier) VerifyGoalSessionOwnership(
	_ context.Context,
	request goalsvc.GoalSessionOwnershipRequest,
) (goalsvc.GoalSessionOwnershipProof, error) {
	agentID := request.TrustedAgentID
	return goalsvc.GoalSessionOwnershipProof{
		TrustedAgentID:   agentID,
		TrustedAgentName: v.agentNames[agentID],
	}, nil
}

func TestRealtimeServiceHandleChatMarksInternalGoalUsageLimitedWhenQuotaExceeded(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.GoalEnabled = true
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-room-internal-goal-quota",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "内部 Goal 额度助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	factory := &fakeRoomFactory{clients: []*fakeRoomClient{newFakeRoomClient()}}
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		factory,
	)
	goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	goalService.SetSessionOwnershipVerifier(testRoomGoalSessionOwnershipVerifier{
		agentNames: map[string]string{memberAgent.AgentID: memberAgent.Name},
	})
	service.SetGoalContextProvider(goalService)
	quotaErr := subscriptionsvc.QuotaExceededError{UsedTokens: 10, LimitTokens: 10}
	service.SetQuotaChecker(fakeRoomQuotaChecker{err: quotaErr})

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	goal, err := goalService.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  sharedSessionKey,
		Objective:   "保持账号额度边界",
		OwnerUserID: "owner-room-internal-goal-quota",
	})
	if err != nil {
		t.Fatalf("创建 Room Goal 失败: %v", err)
	}
	err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "internal goal continuation",
		GoalID:         goal.ID,
		RoundID:        "room-round-internal-goal-quota",
		Internal:       true,
	})
	if !errors.Is(err, subscriptionsvc.ErrQuotaExceeded) {
		t.Fatalf("账号额度耗尽应阻止内部 Room Goal runtime，实际: %v", err)
	}
	limited, err := goalService.Current(ctx, sharedSessionKey)
	if err != nil {
		t.Fatalf("读取受限 Room Goal 失败: %v", err)
	}
	if limited.Status != protocol.GoalStatusUsageLimited || limited.LastError != quotaErr.ClientMessage() {
		t.Fatalf("受限 Room Goal = %#v, want usage_limited with actionable subscription quota message", limited)
	}
	if got := factory.LastOptions(); got.Model != "" {
		t.Fatalf("账号额度耗尽时不应创建内部 Room Goal runtime: %+v", got)
	}
}

func (f fakeRoomQuotaChecker) EnsureQuotaAvailable(context.Context, string) error {
	return f.err
}

func TestRealtimeServiceFailsRoomRoundWithoutResult(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-assistant-terminal",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "终态助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{memberAgent.AgentID},
		Name:     "assistant 终态房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go sendFakeTerminalAssistantAndClose(client, "assistant-terminal-no-result", "这是完整回复。", map[string]any{
			"input_tokens":  7,
			"output_tokens": 11,
		})
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-assistant-terminal")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@终态助手 写一句话",
		RoundID:        "room-round-assistant-terminal",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "error"
	})
	for _, event := range events {
		if event.Data["status"] == "finished" {
			t.Fatalf("缺少 result 不能宣告 Room round 成功: %+v", event)
		}
	}
	assistantPayload := findRoomAssistantMessagePayload(t, events, "assistant-terminal-no-result")
	if assistantPayload["is_complete"] != true || assistantPayload["stop_reason"] != "error" {
		t.Fatalf("已有回复应投影为失败终态: %+v", assistantPayload)
	}
	blocks := roomContentBlocksFromPayload(t, assistantPayload)
	if len(blocks) != 1 || blocks[0]["text"] != "这是完整回复。" {
		t.Fatalf("失败收口不能丢失已生成的正文: %+v", assistantPayload)
	}
}

func TestRealtimeServiceKeepsSubagentRoomSlotInRuntimeManager(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "子任务助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{memberAgent.AgentID},
		Name:     "subagent runtime 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeTaskStarted,
				SessionID: client.sessionID,
				TaskStarted: &sdkprotocol.TaskStartedMessage{
					TaskID:      "task-room-1",
					AgentID:     "sdk-subagent-1",
					AgentType:   "worker",
					Description: "检查 Room runtime",
					TaskType:    "local_agent",
					Additional: map[string]any{
						"child_session_id": "child-room-1",
						"name":             "Room runtime audit",
					},
				},
			}
			sendFakeAssistantResult(client, "assistant-room-subagent", "子任务已在后台执行。")
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-subagent-runtime")
	permission.BindSession(sharedSessionKey, sender)
	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@子任务助手 启动后台检查",
		RoundID:        "room-round-subagent-runtime",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		memberAgent.AgentID,
		roomContext.Room.RoomType,
	)
	if !runtimeManager.HasSubagentHistory(runtimeSessionKey) {
		t.Fatal("Room slot 未在 runtime manager 保留 subagent history")
	}
	if rounds := runtimeManager.GetRunningRoundIDs(runtimeSessionKey); len(rounds) != 0 {
		t.Fatalf("父 round 结束后仍残留 running round: %+v", rounds)
	}
	if err = runtimeManager.StopTask(ctx, runtimeSessionKey, "task-room-1"); err != nil {
		t.Fatalf("Room task stop 未路由到 slot client: %v", err)
	}
	if err = runtimeManager.SendTaskMessage(ctx, runtimeSessionKey, "task-room-1", "继续检查", "继续检查"); err != nil {
		t.Fatalf("Room task follow-up 未路由到 slot client: %v", err)
	}
	client.mu.Lock()
	if client.disconnects != 0 || len(client.stoppedTasks) != 1 || len(client.taskMessages) != 1 {
		t.Fatalf("Room slot client 生命周期/控制不正确: disconnects=%d stopped=%+v messages=%+v", client.disconnects, client.stoppedTasks, client.taskMessages)
	}
	client.mu.Unlock()

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room history 失败: %v", err)
	}
	var taskMetadata map[string]any
	for _, message := range messages {
		metadata, _ := message["metadata"].(map[string]any)
		if metadata["task_id"] == "task-room-1" {
			taskMetadata = metadata
			break
		}
	}
	if taskMetadata["runtime_kind"] != "nxs" || taskMetadata["child_session_id"] != "child-room-1" {
		t.Fatalf("Room task metadata 未保留 runtime/thread 身份: %+v", taskMetadata)
	}
	if err = runtimeManager.CloseSession(ctx, runtimeSessionKey); err != nil {
		t.Fatalf("清理 Room runtime 失败: %v", err)
	}
}

type roomDirectedMessageBroadcaster struct {
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (b *roomDirectedMessageBroadcaster) Broadcast(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	return nil
}

func (b *roomDirectedMessageBroadcaster) Events() []protocol.EventMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	events := make([]protocol.EventMessage, len(b.events))
	copy(events, b.events)
	return events
}

func TestRealtimeServiceRechecksPrivateMessagingAtCallTime(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID: "user-room-private-revocation", Username: "room-owner", Role: authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "撤权测试 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	disabled := false
	if _, err = roomService.UpdateRoom(ctx, roomContext.Room.ID, protocol.UpdateRoomRequest{
		PrivateMessagesEnabled: &disabled,
	}); err != nil {
		t.Fatalf("关闭私域消息失败: %v", err)
	}

	service := realtimesvc.NewService(
		cfg, roomService, agentService, runtimectx.NewManager(), permissionctx.NewContext(),
	)
	request := protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID, Recipients: []string{devin.AgentID},
		Content: "旧 runtime 不应继续发送", ReplyRoute: protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone},
	}
	if _, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, request); err == nil ||
		!strings.Contains(err.Error(), "disabled") {
		t.Fatalf("关闭后旧 directed message 调用应被 service 拒绝，实际 err=%v", err)
	}
	if _, err = service.HandlePublicMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomPublicMessageRequest{
		SourceAgentID: amy.AgentID, Content: "旧 runtime 不应继续主动广播",
	}); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("关闭后旧 public message 调用应被 service 拒绝，实际 err=%v", err)
	}
	if _, err = service.HandlePlatformPublicMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomPublicMessageRequest{
		SourceAgentID: amy.AgentID, Content: "平台通讯群消息不依赖私域开关",
	}); err != nil {
		t.Fatalf("平台通讯群消息不应依赖私域开关: %v", err)
	}
}

func TestRealtimeServiceQueuesDirectedMessageWhenTargetRunning(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message-queue",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "私信排队 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	devinCurrentClient := newFakeRoomClient()
	devinInitialPrompt := make(chan string, 1)
	queuedPrompt := make(chan string, 1)
	var devinQueryCount atomic.Int32
	devinCurrentClient.onQuery = func(_ context.Context, prompt string) error {
		if devinQueryCount.Add(1) == 1 {
			devinInitialPrompt <- prompt
			return nil
		}
		queuedPrompt <- prompt
		go sendFakeAssistantResult(devinCurrentClient, "devin-directed-message-after-busy", "这是给 Amy 的排队私下回复")
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{devinCurrentClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	registerRealtimeServiceCleanup(t, service, runtimeManager, sharedSessionKey)
	sender := newRealtimeTestSender("room-sender-directed-message-queue")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 先处理一个长任务",
		RoundID:        "room-round-devin-directed-busy",
	}); err != nil {
		t.Fatalf("启动 Devin 长任务失败: %v", err)
	}
	select {
	case prompt := <-devinInitialPrompt:
		if !strings.Contains(prompt, "@Devin 先处理一个长任务") {
			t.Fatalf("Devin 首轮 prompt 缺少长任务: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("Devin 长任务未进入 runtime Query")
	}

	message, err := service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "只给 Devin 的排队私信",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
		},
	})
	if err != nil {
		t.Fatalf("创建 directed message 失败: %v", err)
	}

	targetQueueLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  devin.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, devin.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	targetQueueItems, err := workspacestore.NewInputQueueStore(cfg.WorkspacePath).Snapshot(targetQueueLocation)
	if err != nil {
		t.Fatalf("读取目标 agent session 队列失败: %v", err)
	}
	if len(targetQueueItems) != 1 {
		t.Fatalf("directed message 应排队到目标 agent session: %+v", targetQueueItems)
	}
	queuedItem := targetQueueItems[0]
	if queuedItem.Source != protocol.InputQueueSourceAgentRoomMessage ||
		queuedItem.SourceAgentID != amy.AgentID ||
		queuedItem.SourceMessageID != message.MessageID ||
		len(queuedItem.TargetAgentIDs) != 1 ||
		queuedItem.TargetAgentIDs[0] != devin.AgentID ||
		queuedItem.ReplyRoute.Mode != protocol.RoomReplyRoutePrivate ||
		len(queuedItem.ReplyRoute.Recipients) != 1 ||
		queuedItem.ReplyRoute.Recipients[0] != amy.AgentID {
		t.Fatalf("directed message 队列项缺少来源、目标或 reply_route: %+v", queuedItem)
	}
	if strings.Contains(queuedItem.Content, "只给 Devin 的排队私信") {
		t.Fatalf("directed message 队列项不应泄漏私信正文: %+v", queuedItem)
	}
	select {
	case prompt := <-queuedPrompt:
		t.Fatalf("目标 agent 尚未空闲前不应启动 queued directed message: %s", prompt)
	default:
	}

	go sendFakeAssistantResult(devinCurrentClient, "devin-current-directed-task-done", "当前长任务完成。")
	select {
	case prompt := <-queuedPrompt:
		for _, expected := range []string{
			"<room_directed_messages>",
			"只给 Devin 的排队私信",
			"reply_route=private recipients=Amy",
		} {
			if !strings.Contains(prompt, expected) {
				t.Fatalf("queued directed message prompt 缺少片段 %q:\n%s", expected, prompt)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("目标 agent 空闲后未派发 queued directed message")
	}

	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	waitForRoomDirectedMessageContent(t, messageStore, roomContext.Room.OwnerUserID, roomContext.Conversation.ID, amy.AgentID, "这是给 Amy 的排队私下回复")
}

func TestRealtimeServiceCarriesPublicRouteFromPrivateHandback(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-public-message-handback",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "狼人杀测试 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	witchClient := newFakeRoomClient()
	var witchSawPrompt atomic.Bool
	witchClient.onQuery = func(_ context.Context, prompt string) error {
		witchSawPrompt.Store(strings.Contains(prompt, "今晚被刀的是 Lucy") &&
			strings.Contains(prompt, "reply_route=private recipients=Amy"))
		sendFakeAssistantResult(witchClient, "devin-witch-reply", "救:Lucy；不毒")
		return nil
	}

	var service *realtimesvc.Service
	hostClient := newFakeRoomClient()
	var hostSawHandback atomic.Bool
	hostClient.onQuery = func(_ context.Context, prompt string) error {
		hostSawHandback.Store(strings.Contains(prompt, "救:Lucy；不毒") &&
			strings.Contains(prompt, "reply_route=public"))
		sendFakeAssistantResult(hostClient, "amy-after-private-handback", "<nexus_room_no_reply/>")
		return nil
	}

	service = realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{witchClient, hostClient}},
	)
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "今晚被刀的是 Lucy。是否用解药救？是否用毒药毒谁？格式：救:<名字>|不救；毒:<名字>|不毒。",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyImmediate,
			NextReplyRoute: &protocol.RoomReplyRoute{
				Mode: protocol.RoomReplyRoutePublic,
			},
		},
	})
	if err != nil {
		t.Fatalf("创建女巫 directed message 失败: %v", err)
	}

	waitForAtomicBool(t, &witchSawPrompt, "女巫未看到私信问题")
	waitForAtomicBool(t, &hostSawHandback, "主持人未收到私域回交")
	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	amyMessages := waitForRoomDirectedMessageContent(t, messageStore, roomContext.Room.OwnerUserID, roomContext.Conversation.ID, amy.AgentID, "救:Lucy；不毒")
	if !roomDirectedMessageContentHasReplyRoute(amyMessages, "救:Lucy；不毒", protocol.RoomReplyRoutePublic) {
		t.Fatalf("私域回交应携带主持人下一跳公区路线: %+v", amyMessages)
	}
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	publicMessages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	publicText := fmt.Sprint(publicMessages)
	if strings.Contains(publicText, "救:Lucy") {
		t.Fatalf("女巫私下回复不应泄漏到公区 feed: %+v", publicMessages)
	}
}

func TestRealtimeServiceRejectsInvalidDirectedMessageRoute(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message-invalid",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "测试 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	service := realtimesvc.NewService(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
	)
	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{"agent-not-member"},
		Content:       "非法目标",
	})
	if err == nil {
		t.Fatal("非 Room 成员不应成为 directed message recipient")
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		WakeTargets:   []string{amy.AgentID},
		Content:       "唤醒目标越界",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
	})
	if err == nil || !strings.Contains(err.Error(), "subset of recipients") {
		t.Fatalf("wake_targets 必须是 recipients 子集: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		WakeTargets:   []string{devin.AgentID},
		Content:       "只记录却指定唤醒目标",
		WakePolicy:    protocol.RoomWakePolicyNone,
	})
	if err == nil || !strings.Contains(err.Error(), "wake_targets") {
		t.Fatalf("wake_policy=none 不应接受 wake_targets: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "非法 reply route",
		ReplyRoute: protocol.RoomReplyRoute{
			Mode: protocol.RoomReplyRoutePrivate,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "reply_route private requires recipients") {
		t.Fatalf("reply_route=private 缺少 recipients 错误不正确: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "非法 next reply route",
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
			NextReplyRoute: &protocol.RoomReplyRoute{
				Mode: protocol.RoomReplyRoutePublic,
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "next_reply_route requires reply_route private wake_policy=immediate") {
		t.Fatalf("reply_route=private wake=none 不应接受 next_reply_route: %v", err)
	}
}

func waitForAtomicBool(t *testing.T, value *atomic.Bool, message string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for !value.Load() {
		select {
		case <-deadline:
			t.Fatal(message)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func roomDirectedMessageContentsContain(messages []protocol.RoomDirectedMessageRecord, content string) bool {
	return slices.ContainsFunc(messages, func(message protocol.RoomDirectedMessageRecord) bool {
		return message.Content == content
	})
}

func roomDirectedMessageContentHasReplyRoute(
	messages []protocol.RoomDirectedMessageRecord,
	content string,
	mode protocol.RoomReplyRouteMode,
) bool {
	return slices.ContainsFunc(messages, func(message protocol.RoomDirectedMessageRecord) bool {
		return message.Content == content && message.ReplyRoute.Mode == mode
	})
}

func waitForRoomDirectedMessageContent(
	t *testing.T,
	store *workspacestore.RoomDirectedMessageStore,
	ownerUserID string,
	conversationID string,
	agentID string,
	content string,
) []protocol.RoomDirectedMessageRecord {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		messages, err := store.ReadContextMessages(ownerUserID, conversationID, agentID)
		if err != nil {
			t.Fatalf("读取 Room directed message 失败: %v", err)
		}
		if roomDirectedMessageContentsContain(messages, content) {
			return messages
		}
		select {
		case <-deadline:
			t.Fatalf("Room directed message 未投影给目标成员: agent=%s content=%q messages=%+v", agentID, content, messages)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
