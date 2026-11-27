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
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
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

func writeOwnerTranscriptFixture(
	t *testing.T,
	ownerUserID string,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	writeTranscriptFixtureAt(
		t,
		filepath.Join(appfs.UserRuntimeRoot(ownerUserID), "projects"),
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

type blockingDMTestSender struct {
	key  string
	done chan struct{}
	once sync.Once
}

func (s *blockingDMTestSender) Key() string { return s.key }

func (s *blockingDMTestSender) IsClosed() bool { return false }

func (s *blockingDMTestSender) SendEvent(ctx context.Context, _ protocol.EventMessage) error {
	<-ctx.Done()
	s.once.Do(func() {
		close(s.done)
	})
	return ctx.Err()
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
	service := &Service{Host: runtimehost.Host{Permission: permission}}

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

func TestDMPrepareRuntimeHandlesSlashCommandPayloads(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:nexus:ws:dm:slash-command"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})

	execution, err := service.prepareChatExecution(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "/model",
	})
	if err != nil {
		t.Fatalf("prepareChatExecution() error = %v", err)
	}
	preparation, err := execution.prepareRuntime()
	if err != nil {
		t.Fatalf("prepareRuntime() error = %v", err)
	}
	if got := preparation.content.PlainText(); got != "/model" {
		t.Fatalf("Slash runtime payload = %q, want clean command", got)
	}
	if !preparation.atomicInput {
		t.Fatal("Slash runtime payload must be marked atomic")
	}

	const rawVisualize = "/visualize quarterly revenue"
	visualizeExecution, err := service.prepareChatExecution(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    rawVisualize,
	})
	if err != nil {
		t.Fatalf("prepareChatExecution(/visualize) error = %v", err)
	}
	visualizePreparation, err := visualizeExecution.prepareRuntime()
	if err != nil {
		t.Fatalf("prepareRuntime(/visualize) error = %v", err)
	}
	wantVisualize := "Use Generative UI to create an interactive visual for the following request:\n\nquarterly revenue"
	if got := visualizePreparation.content.PlainText(); got != wantVisualize {
		t.Fatalf("/visualize runtime payload = %q, want %q", got, wantVisualize)
	}
	if visualizeExecution.request.Content != rawVisualize {
		t.Fatalf("display payload = %q, want raw Slash input", visualizeExecution.request.Content)
	}
	if !visualizePreparation.atomicInput {
		t.Fatal("/visualize runtime payload must be marked atomic")
	}
	planExecution, err := service.prepareChatExecution(context.Background(), Request{SessionKey: sessionKey, Content: "/plan improve search"})
	if err != nil {
		t.Fatal(err)
	}
	planPreparation, err := planExecution.prepareRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if planPreparation.permissionMode != "plan" || !planPreparation.atomicInput || !strings.Contains(planPreparation.content.PlainText(), "ExitPlanMode") {
		t.Fatalf("plan request did not enter guarded planning: %#v", planPreparation)
	}
	if planExecution.request.Content != "/plan improve search" || protocol.SessionRuntimeSettingsFromOptions(planPreparation.session.Options).PermissionMode == "plan" {
		t.Fatal("plan command changed the original message or persisted a permission setting")
	}
	next, err := service.prepareChatExecution(context.Background(), Request{SessionKey: sessionKey, Content: "next task"})
	if err != nil {
		t.Fatal(err)
	}
	nextPreparation, err := next.prepareRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if nextPreparation.permissionMode == "plan" {
		t.Fatal("plan permission leaked into the next request")
	}

}

func TestDMPrepareRuntimeRejectsSlashCommandAttachments(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{
		client: newFakeDMClient(),
	})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	execution, err := service.prepareChatExecution(context.Background(), Request{
		SessionKey: "agent:nexus:ws:dm:slash-attachment",
		Content:    "/model sonnet",
		Attachments: []protocol.ChatAttachment{{
			FileName:      "image.png",
			WorkspacePath: "tmp/attachments/image.png",
			Kind:          protocol.ChatAttachmentKindImage,
		}},
	})
	if err != nil {
		t.Fatalf("prepareChatExecution() error = %v", err)
	}
	_, err = execution.prepareRuntime()
	var target slashCommandAttachmentError
	if !errors.As(err, &target) {
		t.Fatalf("prepareRuntime() error = %v, want slash attachment error", err)
	}
}

func TestAuthorizeDMSessionAccessSeparatesExternalViewingFromWebHostCommands(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{
		client: newFakeDMClient(),
	})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:nexus:ws:dm:host-command"
	externalSessionKey := protocol.BuildAgentAccountSessionKey(
		"nexus",
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"account-a",
		"user-a",
		"",
	)

	if err := service.AuthorizeDMSessionAccess(context.Background(), sessionKey, "nexus"); err != nil {
		t.Fatalf("AuthorizeDMSessionAccess(web) error = %v", err)
	}
	if err := service.AuthorizeDMSessionAccess(context.Background(), externalSessionKey, "nexus"); err != nil {
		t.Fatalf("AuthorizeDMSessionAccess(IM) error = %v", err)
	}
	if err := service.AuthorizeHostCommand(context.Background(), sessionKey, "nexus"); err != nil {
		t.Fatalf("AuthorizeHostCommand() error = %v", err)
	}
	if runtimeManager.HasSession(sessionKey) || runtimeManager.HasSession(externalSessionKey) {
		t.Fatal("session access authorization must not start a runtime")
	}
	if err := service.AuthorizeHostCommand(
		context.Background(),
		externalSessionKey,
		"nexus",
	); err == nil {
		t.Fatal("external channel host command authorization must fail")
	}
	if err := service.AuthorizeHostCommand(
		context.Background(),
		sessionKey,
		"another-agent",
	); err == nil {
		t.Fatal("mismatched agent_id host command authorization must fail")
	}
	if err := service.AuthorizeDMSessionAccess(
		context.Background(),
		externalSessionKey,
		"another-agent",
	); err == nil {
		t.Fatal("mismatched agent_id session access authorization must fail")
	}
}

func TestServiceHandleChatBroadcastsMergedParallelToolResults(t *testing.T) {
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
						ID:    "assistant-parallel-tools",
						Model: "glm-5.1",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.ToolUseBlock{
								ID:    "tool-connectors",
								Name:  "mcp__nexus_feishu_docx__search",
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
								Name:  "mcp__nexus_automation__automation_query",
								Input: json.RawMessage(`{"operation":"list"}`),
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
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-parallel-tools")
	sessionKey := "agent:nexus:ws:dm:parallel-tools"
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "再试一下两个工具",
		RoundID:    "round-parallel-tools",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	// 工具段与最终文本段是两条独立 assistant 消息（按快照 id 分段），
	// 与直播 stream 的 message_start 分段语义一致。
	toolsPayload := findLatestAssistantMessagePayload(t, events, "assistant-parallel-tools")
	toolBlocks := contentBlocksFromPayload(t, toolsPayload)
	assertContentBlockTypes(t, toolBlocks, []string{"tool_use", "tool_use", "tool_result", "tool_result"})
	assertToolResultIDs(t, toolBlocks, []string{"tool-connectors", "tool-automation"})

	finalPayload := findLatestAssistantMessagePayload(t, events, "assistant-parallel-final")
	finalBlocks := contentBlocksFromPayload(t, finalPayload)
	assertContentBlockTypes(t, finalBlocks, []string{"text"})
	if finalPayload["is_complete"] != true || finalPayload["stop_reason"] != "end_turn" {
		t.Fatalf("最终实时 assistant 应标记完成: %+v", finalPayload)
	}
	if _, exists := finalPayload["stream_status"]; exists {
		t.Fatalf("durable assistant 不应补写 stream_status: %+v", finalPayload)
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

type blockingDMAcceptanceActivity struct {
	started chan context.Context
	release chan struct{}
}

func (a *blockingDMAcceptanceActivity) MarkConversationStarted(
	ctx context.Context,
	_ string,
	_ time.Time,
) error {
	a.started <- ctx
	<-a.release
	return ctx.Err()
}

func TestDetachedAcceptanceUsesRoundContextAfterDurableMarker(t *testing.T) {
	cfg := newDMTestConfig(t)

	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-detached-post-marker",
				Result:    &sdkprotocol.ResultMessage{Subtype: "success"},
			}
		}()
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	permission := permissionctx.NewContext()
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	activity := &blockingDMAcceptanceActivity{
		started: make(chan context.Context, 1),
		release: make(chan struct{}),
	}
	service.SetRoomConversationActivityStore(activity)
	sessionKey := "agent:nexus:ws:dm:detached-post-marker"
	sender := newDMTestSender("sender-detached-post-marker")
	permission.BindSession(sessionKey, sender)

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	handleDone := make(chan error, 1)
	go func() {
		handleDone <- service.HandleRealtimeChat(requestCtx, Request{
			SessionKey:      sessionKey,
			Content:         "持久化后连接断开仍应继续",
			ClientRequestID: "request-detached-post-marker",
			ClientMessageID: "message-detached-post-marker",
		})
	}()
	var activityCtx context.Context
	select {
	case activityCtx = <-activity.started:
	case <-time.After(3 * time.Second):
		t.Fatal("durable marker 后未进入派生状态写入")
	}
	cancelRequest()
	select {
	case <-activityCtx.Done():
		t.Fatalf("durable marker 后的派生写入不应继承 WebSocket 取消: %v", activityCtx.Err())
	case <-time.After(50 * time.Millisecond):
	}
	close(activity.release)
	select {
	case err := <-handleDone:
		if err != nil {
			t.Fatalf("durable marker 后连接取消不应让受理失败: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("durable marker 后 HandleRealtimeChat 未返回")
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == protocol.RoundStatusFinished
	})
	waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
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

func TestEnsureClientForConversationForkUsesSourceBoundary(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	client := newFakeDMClient()
	client.sessionID = "fork-session-new"
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	service.SetProviderResolver(providerService)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "nxs",
	}})

	ctx := context.Background()
	agentValue, err := agentService.GetAgent(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取默认 Agent 失败: %v", err)
	}
	workspacePath := dmMainWorkspacePath(cfg)
	sourceSessionID := "11111111-1111-4111-8111-111111111111"
	boundaryUUID := "11000000-0000-4000-8000-000000000001"
	writeTranscriptFixture(t, workspacePath, sourceSessionID, []map[string]any{{
		"type":      "assistant",
		"uuid":      boundaryUUID,
		"sessionId": sourceSessionID,
		"timestamp": "2026-08-17T00:00:00Z",
		"cwd":       workspacePath,
		"message": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"type": "text", "text": "完成"}},
		},
	}})
	targetSessionKey := "agent:nexus:ws:dm:fork-target-options"
	now := time.Now().UTC()
	targetSession := protocol.Session{
		SessionKey:   targetSessionKey,
		AgentID:      cfg.DefaultAgentID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Fork Target",
		Options: protocol.WithSessionRuntimeSettings(map[string]any{
			protocol.OptionRuntimeKind:                "claude",
			protocol.OptionRuntimeProvider:            "glm",
			protocol.OptionRuntimeModel:               "glm-5.1",
			protocol.OptionRuntimeForkSourceSessionID: sourceSessionID,
			protocol.OptionRuntimeForkMessageID:       boundaryUUID,
		}, protocol.SessionRuntimeSettings{
			Provider: "glm",
			Model:    "glm-5.1",
		}),
		IsActive: true,
	}
	if _, err = service.Files.UpsertSession(workspacePath, targetSession); err != nil {
		t.Fatalf("写入 target session 失败: %v", err)
	}

	if _, err = service.ensureClient(ctx, targetSessionKey, agentValue, targetSession, Request{
		RoundID:      "round-fork-setup",
		AgentRoundID: "agent-round-fork-setup",
	}); err != nil {
		t.Fatalf("准备 fork runtime 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), targetSessionKey)
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != sourceSessionID ||
		options.Session.ResumeAt != boundaryUUID ||
		!options.Session.Fork {
		t.Fatalf("fork runtime options 不正确: %+v", options.Session)
	}
	if options.Runtime.Kind != agentclient.RuntimeClaude {
		t.Fatalf("fork 必须沿用 source runtime，got=%q", options.Runtime.Kind)
	}
	if options.Session.ID != "" {
		t.Fatalf("fork target session id 应由 runtime 分配: %+v", options.Session)
	}
}

func TestResolveConversationForkBoundaryFindsHistoricalTranscriptSegment(t *testing.T) {
	cfg := newDMTestConfig(t)
	workspacePath := dmMainWorkspacePath(cfg)
	sessionKey := "agent:nexus:ws:dm:segmented-fork-source"
	oldSessionID := "22222222-2222-4222-8222-222222222222"
	currentSessionID := "33333333-3333-4333-8333-333333333333"
	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-old", "旧问题", 1000); err != nil {
		t.Fatalf("写入旧 round marker 失败: %v", err)
	}
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-new", "新问题", 2000); err != nil {
		t.Fatalf("写入新 round marker 失败: %v", err)
	}
	writeTranscriptFixture(t, workspacePath, oldSessionID, []map[string]any{
		{
			"type": "user", "uuid": "old-user", "sessionId": oldSessionID,
			"timestamp": 1000,
			"message":   map[string]any{"role": "user", "content": "旧问题"},
		},
		{
			"type": "assistant", "uuid": "old-assistant", "parentUuid": "old-user",
			"sessionId": oldSessionID, "timestamp": 1100,
			"message": map[string]any{"role": "assistant", "content": "旧回答"},
		},
	})
	writeTranscriptFixture(t, workspacePath, currentSessionID, []map[string]any{
		{
			"type": "user", "uuid": "new-user", "sessionId": currentSessionID,
			"timestamp": 2000,
			"message":   map[string]any{"role": "user", "content": "新问题"},
		},
		{
			"type": "assistant", "uuid": "new-assistant", "parentUuid": "new-user",
			"sessionId": currentSessionID, "timestamp": 2100,
			"message": map[string]any{"role": "assistant", "content": "新回答"},
		},
	})

	sessionID, messageID, err := resolveConversationForkBoundary(
		history,
		workspacePath,
		sessionKey,
		protocol.Session{
			SessionID:            &currentSessionID,
			TranscriptSessionIDs: []string{oldSessionID, currentSessionID},
			Options: map[string]any{
				protocol.OptionRuntimeSegmentedTranscript: true,
			},
		},
		"round-old",
	)
	if err != nil {
		t.Fatalf("解析历史 transcript segment 失败: %v", err)
	}
	if sessionID != oldSessionID || messageID != "old-assistant" {
		t.Fatalf("fork boundary = (%q, %q), want (%q, old-assistant)", sessionID, messageID, oldSessionID)
	}
}

func TestCompletedAssistantRoundUsesPagedMessageSemantics(t *testing.T) {
	tests := []struct {
		name   string
		rows   []protocol.Message
		active []string
		want   bool
	}{
		{
			name: "successful assistant",
			rows: []protocol.Message{{
				"round_id": "round-target",
				"role":     "assistant",
				"result_summary": map[string]any{
					"subtype": "success",
				},
			}},
			want: true,
		},
		{
			name: "active assistant",
			rows: []protocol.Message{{
				"round_id": "round-target",
				"role":     "assistant",
			}},
			active: []string{"round-target"},
		},
		{
			name: "interrupted assistant",
			rows: []protocol.Message{{
				"round_id":    "round-target",
				"role":        "assistant",
				"stop_reason": "cancelled",
			}},
		},
		{
			name: "result without assistant",
			rows: []protocol.Message{{
				"round_id": "round-target",
				"role":     "result",
				"subtype":  "success",
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := completedAssistantRound(test.rows, "round-target", test.active); got != test.want {
				t.Fatalf("completedAssistantRound() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLatestForkableAssistantRoundSkipsIncompleteTranscriptProjection(t *testing.T) {
	rows := []protocol.Message{
		{
			"round_id": "round-transcript",
			"role":     "assistant",
			"result_summary": map[string]any{
				"subtype": "success",
			},
		},
		{
			"round_id": "round-partial-page",
			"role":     "assistant",
		},
	}
	resolved := make([]string, 0, 2)
	roundID, err := latestForkableAssistantRound(rows, nil, func(candidate string) error {
		resolved = append(resolved, candidate)
		if candidate == "round-partial-page" {
			return errTransientForkRoundNotCompleted
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if roundID != "round-transcript" {
		t.Fatalf("latest forkable round = %q, want round-transcript", roundID)
	}
	if !slices.Equal(resolved, []string{"round-partial-page", "round-transcript"}) {
		t.Fatalf("resolved candidates = %v", resolved)
	}
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
