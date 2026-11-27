// chat_test.go 覆盖 DM 聊天、验收、分支、改写、配额、IM 回复与基础测试夹具。
package dm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	os.Exit(handlertest.RunWithMinimalAppRoot(m))
}

// accelerateDMGoalUsageRetry 缩短白盒测试中的退避时钟，不改变生产默认值。
func accelerateDMGoalUsageRetry(runner *roundRunner) {
	if runner != nil {
		runner.goalUsageRetryBaseDelay = time.Millisecond
	}
}

// accelerateDMExternalTyping 缩短白盒测试中的 typing 启动等待，不改变生产默认值。
func accelerateDMExternalTyping(runner *roundRunner) {
	if runner != nil {
		runner.externalTypingDelay = 5 * time.Millisecond
	}
}

// dmTestDBs 让同一测试库只有一个连接池：与生产一致（SQLite 单写者、MaxOpenConns=1），
// 多个池在回滚日志模式下互相抢锁会绕过 busy_timeout 直接 SQLITE_BUSY。
var dmTestDBs sync.Map

func dmTestDB(t *testing.T, cfg config.Config) *sql.DB {
	t.Helper()
	if db, ok := dmTestDBs.Load(cfg.DatabaseURL); ok {
		return db.(*sql.DB)
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	dmTestDBs.Store(cfg.DatabaseURL, db)
	t.Cleanup(func() {
		dmTestDBs.Delete(cfg.DatabaseURL)
		_ = db.Close()
	})
	return db
}

func newDMAgentService(t *testing.T, cfg config.Config) *agentsvc.Service {
	t.Helper()
	return agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", dmTestDB(t, cfg)))
}

// newPlainDMAgent 创建普通 Agent；主智能体的 DM 是主窗口，输入走 inbox，测旧待发送队列要用普通 Agent。
func newPlainDMAgent(t *testing.T, agentService *agentsvc.Service) *protocol.Agent {
	t.Helper()
	created, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{Name: "Queue Agent"})
	if err != nil {
		t.Fatalf("创建普通测试 Agent 失败: %v", err)
	}
	return created
}

// newDMInboxStore 打开与测试 Agent 相同的数据库；主智能体 DM 的输入都经过 inbox。
func newDMInboxStore(t *testing.T, cfg config.Config) *inboxstore.Repository {
	t.Helper()
	return inboxstore.NewRepository(cfg, dmTestDB(t, cfg))
}

func newDMProviderService(t *testing.T, cfg config.Config) *providercfg.Service {
	t.Helper()
	return providercfg.NewServiceWithDB(cfg, dmTestDB(t, cfg))
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

func newDMTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	cfg := config.Config{
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
	// 每个测试都拿到已迁移的独立 SQLite（迁移结果按模板复制，代价很小）。
	handlertest.MigrateSQLiteFromDir(t, cfg.DatabaseURL, dmMigrationDir(t))
	return cfg
}

func mustFindDMSession(
	t *testing.T,
	service *Service,
	cfg config.Config,
	sessionKey string,
) (protocol.Session, string) {
	t.Helper()
	workspacePath := dmMainWorkspacePath(cfg)
	if agentID := protocol.ParseSessionKey(sessionKey).AgentID; agentID != cfg.DefaultAgentID {
		agentValue, err := service.Agents.GetAgent(context.Background(), agentID)
		if err != nil {
			t.Fatalf("读取 session 所属 Agent 失败: %v", err)
		}
		workspacePath = agentValue.WorkspacePath
	}
	item, workspacePath, err := service.Files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取 session 元数据失败: %v", err)
	}
	if item == nil {
		t.Fatalf("session 元数据不存在: %s", sessionKey)
	}
	return *item, workspacePath
}

func dmMainWorkspacePath(cfg config.Config) string {
	return agentsvc.ResolveWorkspacePath(cfg, authctx.SystemUserID, cfg.DefaultAgentID)
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
	writeTranscriptFixtureAt(
		t,
		workspacestore.TranscriptProjectsDirForWorkspace(workspacePath),
		workspacePath,
		sessionID,
		rows,
	)
}

func writeTranscriptFixtureAt(
	t *testing.T,
	projectsRoot string,
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
		projectsRoot,
		workspacestore.TranscriptProjectDirectoryName(workspacePath),
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

func stringPointer(t *testing.T, value *string) string {
	t.Helper()
	if value == nil || strings.TrimSpace(*value) == "" {
		t.Fatal("session_id 未持久化")
	}
	return strings.TrimSpace(*value)
}

func dmMigrationDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
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

func (s *dmTestSender) Key() string { return s.key }

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

// A terminal event precedes the round's final metadata writes. Join session
// teardown before TempDir cleanup instead of treating that event as worker exit.
func cleanupDMRuntimeSession(t *testing.T, runtimeManager *runtimectx.Manager, sessionKey string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := runtimeManager.CloseSession(ctx, sessionKey); err != nil {
			t.Errorf("关闭 DM 测试 session 失败: %v", err)
		}
	})
}

func waitForDMRuntimeIdle(t *testing.T, runtimeManager *runtimectx.Manager, sessionKey string) {
	t.Helper()
	waitContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	deadline := time.After(3 * time.Second)
	for {
		if len(runtimeManager.GetRunningRoundIDs(sessionKey)) == 0 {
			if err := runtimeManager.WaitBackgroundTasks(waitContext, sessionKey); err != nil {
				t.Fatalf("等待 DM 后台任务结束失败: %v", err)
			}
			// round 进入终态后仍会登记队列/Goal 后台任务；再做一次
			// 稳定性检查，避免 TempDir 清理与异步 workspace 写入竞态。
			select {
			case <-time.After(10 * time.Millisecond):
			case <-deadline:
				t.Fatalf("等待 DM runtime 空闲超时")
			}
			if len(runtimeManager.GetRunningRoundIDs(sessionKey)) == 0 {
				if err := runtimeManager.WaitBackgroundTasks(waitContext, sessionKey); err != nil {
					t.Fatalf("等待 DM 后台任务稳定结束失败: %v", err)
				}
				return
			}
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

func assertNotContainsErrorEventForMessage(t *testing.T, events []protocol.EventMessage, messageID string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeError && event.MessageID == messageID {
			t.Fatalf("durable 错误结果不应重复广播绑定消息 %s 的 error 事件: %+v", messageID, events)
		}
	}
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

func anyToString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func TestServiceHandleChatPersistsMessages(t *testing.T) {
	cfg := newDMTestConfig(t)

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
	service.SetInboxStore(newDMInboxStore(t, cfg))
	prefs := preferencessvc.DefaultPreferences()
	prefs.EmotionEnabled = true
	service.SetPreferences(fakeDMPreferencesService{prefs: prefs})
	service.SetUsageRecorder(&fakeTokenUsageRecorder{})
	sender := newDMTestSender("sender-1")
	sessionKey := "agent:nexus:ws:dm:test-chat"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "你好",
		RoundID:    "round-1",
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
		"## Emotion State",
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

func TestServiceHandleChatFailsRoundWhenStreamEndsWithoutTerminalResult(t *testing.T) {
	cfg := newDMTestConfig(t)

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
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-premature")
	sessionKey := "agent:nexus:ws:dm:premature-close"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试提前结束",
		RoundID:    "round-premature",
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
	assertNotContainsErrorEventForMessage(t, events, "assistant-premature")
}

func TestDetachedAcceptanceProjectsRuntimeStartupFailure(t *testing.T) {
	cfg := newDMTestConfig(t)

	startupErr := errors.New("runtime startup failed after acceptance")
	client := newFakeDMClient()
	client.connectErrors = []error{startupErr}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	permission := permissionctx.NewContext()
	agentService := newDMAgentService(t, cfg)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:" + newPlainDMAgent(t, agentService).AgentID + ":ws:dm:detached-startup-failure"
	sender := newDMTestSender("sender-detached-startup-failure")
	permission.BindSession(sessionKey, sender)

	if err := service.HandleRealtimeChat(context.Background(), Request{
		SessionKey:      sessionKey,
		Content:         "先受理再启动",
		ClientRequestID: "request-startup-failure",
		ClientMessageID: "message-startup-failure",
		RoundID:         "round-startup-failure",
	}); err != nil {
		t.Fatalf("已持久受理的请求不应同步返回 runtime 错误: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == protocol.RoundStatusError
	})
	if len(events) < 2 || events[0].EventType != protocol.EventTypeChatAck {
		t.Fatalf("ACK 必须先于 runtime 失败投影: %+v", events)
	}
	assertContainsResultSubtype(t, events, "error")
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)

	history := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(history) < 2 || history[0]["client_message_id"] != "message-startup-failure" {
		t.Fatalf("启动失败后 durable 历史不完整: %+v", history)
	}
}

func TestDetachedAcceptanceRejectsOversizedClientMessageID(t *testing.T) {
	cfg := newDMTestConfig(t)
	service := NewService(
		cfg,
		newDMAgentService(t, cfg),
		runtimectx.NewManagerWithFactory(&fakeDMFactory{client: newFakeDMClient()}),
		permissionctx.NewContext(),
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	err := service.HandleRealtimeChat(context.Background(), Request{
		SessionKey:      "agent:nexus:ws:dm:oversized-client-message-id",
		Content:         "限制客户端消息 ID 长度",
		ClientMessageID: strings.Repeat("x", protocol.MaxClientMessageIDBytes+1),
	})
	if err == nil || !strings.Contains(err.Error(), "client_message_id 过长") {
		t.Fatalf("超长 client_message_id 应在持久化前拒绝: %v", err)
	}
}

type forkRoomSessionStore struct {
	sessions  map[string]protocol.Session
	updateErr error
}

func (s forkRoomSessionStore) GetRoomSessionByKey(
	_ context.Context,
	_ string,
	key protocol.SessionKey,
) (*protocol.Session, error) {
	item, ok := s.sessions[key.Raw]
	if !ok {
		return nil, nil
	}
	copy := item
	return &copy, nil
}

func (s forkRoomSessionStore) UpdateRoomSessionRuntimeIdentity(context.Context, string, string, string) error {
	return s.updateErr
}

func TestConversationForkPreparesAndMaterializesIndependentHistory(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	if err := agentService.EnsureReady(context.Background()); err != nil {
		t.Fatalf("初始化 Agent 失败: %v", err)
	}
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	workspacePath := dmMainWorkspacePath(cfg)
	sourceSessionID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	sourceSessionKey := "agent:nexus:ws:dm:fork-source"
	targetSessionKey := "agent:nexus:ws:dm:fork-target"
	racingTargetSessionKey := "agent:nexus:ws:dm:fork-target-racing-delete"
	now := time.Now().UTC()
	service.SetRoomSessionStore(forkRoomSessionStore{sessions: map[string]protocol.Session{
		sourceSessionKey: {
			SessionKey:   sourceSessionKey,
			AgentID:      cfg.DefaultAgentID,
			SessionID:    &sourceSessionID,
			ChannelType:  "websocket",
			ChatType:     "dm",
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Options: map[string]any{
				protocol.OptionRuntimeKind:     "claude",
				protocol.OptionRuntimeProvider: "glm",
				protocol.OptionRuntimeModel:    "glm-5.1",
			},
			IsActive: true,
		},
		targetSessionKey: {
			SessionKey:   targetSessionKey,
			AgentID:      cfg.DefaultAgentID,
			ChannelType:  "websocket",
			ChatType:     "dm",
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Options:      map[string]any{},
			IsActive:     true,
		},
		racingTargetSessionKey: {
			SessionKey:   racingTargetSessionKey,
			AgentID:      cfg.DefaultAgentID,
			ChannelType:  "websocket",
			ChatType:     "dm",
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Options:      map[string]any{},
			IsActive:     true,
		},
	}})

	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	transcriptRows := make([]map[string]any, 0, 6)
	for index := 1; index <= 3; index++ {
		roundID := "round-" + strconv.Itoa(index)
		timestamp := int64(index * 1000)
		if err := history.AppendRoundMarker(
			workspacePath,
			sourceSessionKey,
			roundID,
			"问题 "+roundID,
			timestamp,
		); err != nil {
			t.Fatalf("写入 %s marker 失败: %v", roundID, err)
		}
		userID := "user-" + roundID
		assistantID := "assistant-" + roundID
		parentID := ""
		if index > 1 {
			parentID = "assistant-round-" + strconv.Itoa(index-1)
		}
		transcriptRows = append(transcriptRows,
			map[string]any{
				"type": "user", "uuid": userID, "parentUuid": parentID,
				"sessionId": sourceSessionID, "timestamp": timestamp,
				"message": map[string]any{"role": "user", "content": "问题 " + roundID},
			},
			map[string]any{
				"type": "assistant", "uuid": assistantID, "parentUuid": userID,
				"sessionId": sourceSessionID, "timestamp": timestamp + 100,
				"message": map[string]any{
					"role":    "assistant",
					"content": []map[string]any{{"type": "text", "text": "回答 " + roundID}},
				},
			},
		)
		if err := history.AppendOverlayMessage(workspacePath, sourceSessionKey, protocol.Message{
			"message_id": "result-" + roundID,
			"round_id":   roundID,
			"role":       "result",
			"subtype":    "success",
			"timestamp":  timestamp + 200,
		}); err != nil {
			t.Fatalf("写入 %s result 失败: %v", roundID, err)
		}
	}
	writeTranscriptFixture(t, workspacePath, sourceSessionID, transcriptRows)

	preparedSessionID, preparedMessageID, err := service.PrepareConversationFork(
		context.Background(),
		sourceSessionKey,
		"round-2",
	)
	if err != nil {
		t.Fatalf("预检 fork 失败: %v", err)
	}
	if preparedSessionID != sourceSessionID || preparedMessageID != "assistant-round-2" {
		t.Fatalf("fork 边界 = (%q, %q)", preparedSessionID, preparedMessageID)
	}
	if err = service.ForkConversationSession(
		context.Background(),
		sourceSessionKey,
		targetSessionKey,
		"round-2",
		preparedSessionID,
		preparedMessageID,
	); err != nil {
		t.Fatalf("物化 fork 失败: %v", err)
	}

	target, _ := mustFindDMSession(t, service, cfg, targetSessionKey)
	if sourceID, messageID := pendingConversationFork(target.Options); sourceID != sourceSessionID || messageID != preparedMessageID {
		t.Fatalf("pending fork 依赖不正确: source=%q message=%q", sourceID, messageID)
	}
	targetRows := readDMSessionHistory(t, cfg, service, targetSessionKey)
	if forkHistoryHasRound(targetRows, "round-3") ||
		!forkHistoryHasRound(targetRows, "round-1") ||
		!forkHistoryHasRound(targetRows, "round-2") {
		t.Fatalf("target fork 历史边界不正确: %+v", targetRows)
	}
	sourceRows := readDMSessionHistory(t, cfg, service, sourceSessionKey)
	if !forkHistoryHasRound(sourceRows, "round-3") {
		t.Fatal("fork 不应改写 source 历史")
	}

	preparedSessionID, preparedMessageID, err = service.PrepareConversationFork(
		context.Background(),
		sourceSessionKey,
		"round-2",
	)
	if err != nil {
		t.Fatalf("并发删除前预检 fork 失败: %v", err)
	}
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取 fork Agent 失败: %v", err)
	}
	if _, err = service.Files.ForOwner(agentValue.OwnerUserID).DeleteSession(
		workspacePath,
		sourceSessionKey,
	); err != nil {
		t.Fatalf("模拟删除 source workspace Session 失败: %v", err)
	}
	targetRows = readDMSessionHistory(t, cfg, service, targetSessionKey)
	if !forkHistoryHasRound(targetRows, "round-1") || !forkHistoryHasRound(targetRows, "round-2") {
		t.Fatalf("删除 source workspace Session 后 pending fork 应保留完整轮次: %+v", targetRows)
	}
	if err = service.ForkConversationSession(
		context.Background(),
		sourceSessionKey,
		racingTargetSessionKey,
		"round-2",
		preparedSessionID,
		preparedMessageID,
	); err == nil {
		t.Fatal("source marker 并发消失时不应生成轮次身份不完整的 fork")
	}
}

func forkHistoryHasRound(rows []protocol.Message, roundID string) bool {
	for _, row := range rows {
		if strings.TrimSpace(protocol.MessageRoundID(row)) == roundID {
			return true
		}
	}
	return false
}

func TestLatestForkableAssistantRoundPreservesBoundaryFailure(t *testing.T) {
	boundaryErr := errors.New("transcript is unreadable")
	_, err := latestForkableAssistantRound([]protocol.Message{{
		"round_id": "round-target",
		"role":     "assistant",
		"result_summary": map[string]any{
			"subtype": "success",
		},
	}}, nil, func(string) error {
		return boundaryErr
	})
	if !errors.Is(err, boundaryErr) {
		t.Fatalf("boundary error = %v, want %v", err, boundaryErr)
	}
}

func TestTransientForkAtTranscriptTailOmitsProviderSpecificMessageBoundary(t *testing.T) {
	legacyMessageID := "msg_20260821090320f5fbc914cb404581-2"
	if got := conversationForkResumeAt(map[string]any{
		protocol.OptionRuntimeForkAtTranscriptTail: true,
	}, legacyMessageID); got != "" {
		t.Fatalf("conversationForkResumeAt() = %q, want full source transcript", got)
	}
	if got := conversationForkResumeAt(nil, legacyMessageID); got != legacyMessageID {
		t.Fatalf("bounded conversationForkResumeAt() = %q, want %q", got, legacyMessageID)
	}
	if !transcriptRoundIsSessionTail(workspacestore.TranscriptRoundTail{
		RoundIDs: []string{"round-target"},
	}, "round-target") {
		t.Fatal("single terminal round was not recognized as the transcript tail")
	}
	if transcriptRoundIsSessionTail(workspacestore.TranscriptRoundTail{
		RoundIDs: []string{"round-target", "round-later"},
	}, "round-target") {
		t.Fatal("historical round was incorrectly recognized as the transcript tail")
	}
}

func TestLastVisibleUserMessageSkipsHiddenControlRows(t *testing.T) {
	visible := protocol.Message{
		"content":  "读取文件",
		"role":     "user",
		"round_id": "round-visible",
	}
	rows := []protocol.Message{
		visible,
		{
			"content":          "内部续跑",
			"hidden_from_user": true,
			"is_synthetic":     true,
			"role":             "user",
			"round_id":         "round-internal",
		},
	}

	got, ok := lastVisibleUserMessage(rows)
	if !ok {
		t.Fatal("lastVisibleUserMessage() returned no visible message")
	}
	if got["round_id"] != "round-visible" {
		t.Fatalf("lastVisibleUserMessage() round_id = %#v, want round-visible", got["round_id"])
	}
}

type fakeDMQuotaChecker struct {
	err error
}

func TestServiceHandleChatMarksInternalGoalUsageLimitedWhenQuotaExceeded(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	factory := &fakeDMFactory{client: client}
	service := NewService(cfg, agentService, runtimectx.NewManagerWithFactory(factory), permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	goalProvider := &fakeGoalContextProvider{}
	service.SetGoalContextProvider(goalProvider)
	quotaErr := subscriptionsvc.QuotaExceededError{UsedTokens: 10, LimitTokens: 10}
	service.SetQuotaChecker(fakeDMQuotaChecker{err: quotaErr})

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID: "owner-internal-goal-quota",
		Role:   authsvc.RoleOwner,
	})
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Internal Goal Quota Agent"})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentSessionKey(agentValue.AgentID, protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "internal-goal-quota", "")
	err = service.HandleChat(ctx, Request{
		SessionKey: sessionKey,
		Content:    "internal goal continuation",
		GoalID:     "goal-internal-quota",
		RoundID:    "round-internal-goal-quota",
		Internal:   true,
	})
	if !errors.Is(err, subscriptionsvc.ErrQuotaExceeded) {
		t.Fatalf("账号额度耗尽应阻止内部 Goal runtime，实际: %v", err)
	}
	if got := goalProvider.recordedUsageLimitReasons(); len(got) != 1 || got[0] != quotaErr.ClientMessage() {
		t.Fatalf("Goal usage limit reasons = %#v, want actionable subscription quota message", got)
	}
	if got := factory.LastOptions(); got.Model != "" || client.connectCalls != 0 {
		t.Fatalf("账号额度耗尽时不应创建或连接内部 Goal runtime: options=%+v connect_calls=%d", got, client.connectCalls)
	}
}

func (f fakeDMQuotaChecker) EnsureQuotaAvailable(context.Context, string) error {
	return f.err
}

func TestIMFeedbackQueuesAtOriginalSessionAndDispatchesOnlyOnce(t *testing.T) {
	cfg := newDMTestConfig(t)
	agents := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	prompts := make(chan string, 8)
	client.onQuery = func(_ context.Context, p string) { prompts <- p }
	manager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	permission := permissionctx.NewContext()
	service := NewService(cfg, agents, manager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	agentID := newPlainDMAgent(t, agents).AgentID
	session := "agent:" + agentID + ":ws:dm:im-feedback-origin"
	sender := newDMTestSender("im-feedback")
	permission.BindSession(session, sender)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: authctx.SystemUserID, Role: authctx.RoleOwner})
	if err := service.HandleChat(ctx, Request{SessionKey: session, Content: "正在处理的原任务", RoundID: "original-round", BroadcastUserMessage: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-prompts:
	case <-time.After(2 * time.Second):
		t.Fatal("original task did not start")
	}
	db := dmTestDB(t, cfg)
	var err error
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = manager.CloseSession(closeCtx, session)
	})
	store := imdelivery.NewRepository(cfg, db)
	service.SetIMDeliveryStore(store, func(context.Context, string, string) error { return nil })
	d := imdelivery.Delivery{ID: "delivery", OwnerUserID: authctx.SystemUserID, Source: imdelivery.Source{Kind: "tool", AgentID: agentID, SessionKey: session}, TargetSessionKey: "im", Content: "待确认草案"}
	if _, err = store.SaveDelivery(ctx, d); err != nil {
		t.Fatal(err)
	}
	reply := imdelivery.Reply{ID: "feedback", DeliveryID: d.ID, OwnerUserID: d.OwnerUserID, Input: imdelivery.Input{ID: "human"}, Content: "确认收到", ForwardedContent: "来自微信联系人的反馈：确认收到"}
	reply, err = store.SaveReply(ctx, reply)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.AcceptIMDeliveryReply(ctx, d, reply); err != nil {
		t.Fatal(err)
	}
	if err = service.AcceptIMDeliveryReply(ctx, d, reply); err != nil {
		t.Fatal(err)
	}
	_, location, err := service.resolveInputQueueLocation(ctx, session, agentID)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := service.InputQueue.Snapshot(location)
	if err != nil || len(queued) != 1 || queued[0].Source != protocol.InputQueueSourceIMDeliveryReply {
		t.Fatalf("feedback not durably queued: %+v %v", queued, err)
	}
	altered := queued[0]
	altered.Content = "批准执行"
	tampered, err := imReplyDispatchFromQueueItem(session, altered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.dispatchIMDeliveryReply(ctx, tampered); err == nil {
		t.Fatal("tampered feedback accepted")
	}
	if err = service.guideInputQueueItem(ctx, session, location, reply.ID); err == nil {
		t.Fatal("feedback injected into busy turn")
	}
	select {
	case p := <-prompts:
		t.Fatalf("busy task was interrupted: %s", p)
	default:
	}
	finishDMQueueRound(client, "original-finished")
	select {
	case p := <-prompts:
		if !strings.Contains(p, "确认收到") {
			t.Fatal(p)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("original session did not continue")
	}
	saved, err := store.GetReply(ctx, reply.OwnerUserID, reply.ID)
	if err != nil || saved.State != "started" {
		t.Fatalf("missing dispatch claim %+v %v", saved, err)
	}
	if err = service.AcceptIMDeliveryReply(ctx, d, reply); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-prompts:
		t.Fatalf("duplicate feedback executed: %s", p)
	default:
	}
	finishDMQueueRound(client, "feedback-finished")
	collectEventsUntil(t, sender.events, func(e protocol.EventMessage) bool {
		return e.EventType == protocol.EventTypeRoundStatus && e.Data["round_id"] == reply.ID && e.Data["status"] == "finished"
	})
}

func TestMainWindowIMFeedbackRunsFromInboxOnlyOnce(t *testing.T) {
	f := newMainInboxFixture(t, "im-feedback-main")
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: authctx.SystemUserID, Role: authctx.RoleOwner})
	cfg := f.service.Config
	store := imdelivery.NewRepository(cfg, dmTestDB(t, cfg))
	f.service.SetIMDeliveryStore(store, func(context.Context, string, string) error { return nil })
	d := imdelivery.Delivery{ID: "delivery", OwnerUserID: authctx.SystemUserID, Source: imdelivery.Source{Kind: "tool", AgentID: "nexus", SessionKey: f.sessionKey}, TargetSessionKey: "im", Content: "待确认草案"}
	if _, err := store.SaveDelivery(ctx, d); err != nil {
		t.Fatal(err)
	}
	reply, err := store.SaveReply(ctx, imdelivery.Reply{ID: "feedback-main", DeliveryID: d.ID, OwnerUserID: d.OwnerUserID, Input: imdelivery.Input{ID: "human"}, Content: "确认收到", ForwardedContent: "来自微信联系人的反馈：确认收到"})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.service.AcceptIMDeliveryReply(ctx, d, reply); err != nil {
		t.Fatal(err)
	}
	if prompts := f.waitPrompts(t, 1); !strings.Contains(prompts[0], "确认收到") {
		t.Fatalf("IM 回执轮缺少反馈内容: %q", prompts[0])
	}
	if _, location, locErr := f.service.resolveInputQueueLocation(ctx, f.sessionKey, ""); locErr == nil {
		if queued, _ := f.service.InputQueue.Snapshot(location); len(queued) != 0 {
			t.Fatalf("主窗口回执不应写入 JSONL 队列: %+v", queued)
		}
	}
	if err = f.service.AcceptIMDeliveryReply(ctx, d, reply); err != nil {
		t.Fatal(err)
	}
	f.finishRound("feedback-main")
	collectEventsUntil(t, f.sender.events, func(e protocol.EventMessage) bool {
		return e.EventType == protocol.EventTypeRoundStatus && e.Data["round_id"] == reply.ID && e.Data["status"] == "finished"
	})
	if prompts := f.prompts(); len(prompts) != 1 {
		t.Fatalf("重复受理的回执不应再执行: %v", prompts)
	}
	saved, err := store.GetReply(ctx, reply.OwnerUserID, reply.ID)
	if err != nil || saved.State != "started" {
		t.Fatalf("回执状态 = %+v %v", saved, err)
	}
}
