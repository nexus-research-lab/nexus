package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	automationmcpcontract "github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type automationIngressFixture struct {
	t              *testing.T
	cfg            config.Config
	db             *sql.DB
	runtimeManager *runtimectx.Manager
	runtimeFactory *automationIngressRuntimeFactory
	channelRouter  *channels.Router
	channelControl *channels.ControlService
	automation     *automationsvc.Service
	ingress        *channels.IngressService
	workspacePath  string
}

func newAutomationIngressFixture(t *testing.T) *automationIngressFixture {
	t.Helper()
	cfg := newAutomationIngressTestConfig(t)
	db := migrateAutomationIngressSQLite(t, cfg)
	t.Cleanup(func() { _ = db.Close() })

	core := NewCoreServicesWithDB(cfg, db)
	permission := permissionctx.NewContext()
	runtimeFactory := newAutomationIngressRuntimeFactory(t)
	runtimeManager := runtimectx.NewManagerWithFactory(runtimeFactory)
	dmService := dmsvc.NewService(cfg, core.Agent, runtimeManager, permission)
	channelRouter := channels.NewRouter(cfg, db, core.Agent, permission)
	channelControl := channels.NewControlService(cfg, db, core.Agent, channelRouter)
	workspaceService := workspacepkg.NewService(cfg, core.Agent)
	automationService := automationsvc.NewService(
		cfg,
		db,
		core.Agent,
		dmService,
		nil,
		permission,
		workspaceService,
		channelRouter,
	)
	automationService.SetRuntimeSessionCloser(runtimeManager)
	dmService.SetMCPServerBuilder(newAutomationMCPBuilder(automationService, core.Agent, cfg.DefaultTimezone))
	ingressService := channels.NewIngressService(cfg, core.Agent, dmService, channelRouter)
	agentValue, err := core.Agent.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取默认智能体失败: %v", err)
	}

	return &automationIngressFixture{
		t:              t,
		cfg:            cfg,
		db:             db,
		runtimeManager: runtimeManager,
		runtimeFactory: runtimeFactory,
		channelRouter:  channelRouter,
		channelControl: channelControl,
		automation:     automationService,
		ingress:        ingressService,
		workspacePath:  strings.TrimSpace(agentValue.WorkspacePath),
	}
}

func (f *automationIngressFixture) acceptFeishuMessage(content string, roundID string, reqID string) {
	f.t.Helper()
	if _, err := f.ingress.Accept(context.Background(), channels.IngressRequest{
		Channel:  channels.ChannelTypeFeishu,
		ChatType: "group",
		Ref:      "oc_group_123",
		Content:  content,
		RoundID:  roundID,
		ReqID:    reqID,
	}); err != nil {
		f.t.Fatalf("飞书 ingress 应进入 DM runtime: %v", err)
	}
	f.waitForFeishuSessionIdle()
}

func (f *automationIngressFixture) waitForFeishuSessionIdle() {
	f.t.Helper()
	sessionKey := protocol.BuildAgentSessionKey(
		f.cfg.DefaultAgentID,
		protocol.SessionChannelFeishuSegment,
		"group",
		"oc_group_123",
		"",
	)
	waitForAutomationIngress(f.t, 3*time.Second, func() bool {
		return len(f.runtimeManager.GetRunningRoundIDs(sessionKey)) == 0
	})
}

func (f *automationIngressFixture) seedDailyFeishuTask(name string, cronExpression string) protocol.CronJob {
	f.t.Helper()
	return f.seedFeishuTask(name, "每天搜索重要新闻并发到这个飞书群", cronExpression)
}

func (f *automationIngressFixture) seedLongRunningFeishuTask(name string, cronExpression string) protocol.CronJob {
	f.t.Helper()
	return f.seedFeishuTask(name, "模拟长时间运行的新闻任务", cronExpression)
}

func (f *automationIngressFixture) seedFeishuTask(name string, instruction string, cronExpression string) protocol.CronJob {
	f.t.Helper()
	sessionKey := protocol.BuildAgentSessionKey(
		f.cfg.DefaultAgentID,
		protocol.SessionChannelFeishuSegment,
		"group",
		"oc_group_123",
		"",
	)
	created, err := f.automation.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        name,
		AgentID:     f.cfg.DefaultAgentID,
		Instruction: instruction,
		Schedule: protocol.Schedule{
			Kind:           protocol.ScheduleKindCron,
			CronExpression: &cronExpression,
			Timezone:       f.cfg.DefaultTimezone,
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetNamed,
			NamedSessionKey: "feishu-group-news",
		},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: protocol.SessionChannelFeishu,
			To:      "oc_group_123",
		},
		Source: protocol.Source{
			Kind:           protocol.SourceKindAgent,
			CreatorAgentID: f.cfg.DefaultAgentID,
			ContextType:    "agent",
			ContextID:      f.cfg.DefaultAgentID,
			SessionKey:     sessionKey,
		},
		Enabled: true,
	})
	if err != nil {
		f.t.Fatalf("创建测试定时任务失败: %v", err)
	}
	return *created
}

func (f *automationIngressFixture) runTaskUntilDeliveryFailed(jobID string) string {
	f.t.Helper()
	runResult, err := f.automation.RunTaskNow(context.Background(), jobID)
	if err != nil {
		f.t.Fatalf("立即运行测试任务失败: %v", err)
	}
	if runResult.RunID == nil || strings.TrimSpace(*runResult.RunID) == "" {
		f.t.Fatalf("立即运行应返回 run_id: %+v", runResult)
	}
	runID := strings.TrimSpace(*runResult.RunID)
	waitForAutomationIngress(f.t, 3*time.Second, func() bool {
		runs, listErr := f.automation.ListTaskRuns(context.Background(), jobID)
		return listErr == nil &&
			len(runs) == 1 &&
			runs[0].RunID == runID &&
			runs[0].DeliveryStatus == protocol.DeliveryStatusFailed
	})
	return runID
}

func (f *automationIngressFixture) assertAgentInboxReceived(sessionKey string, expectedText string) {
	f.t.Helper()
	store := workspacestore.NewSessionFileStore(f.cfg.WorkspacePath)
	sessionValue, _, err := store.FindSession([]string{f.workspacePath}, sessionKey)
	if err != nil {
		f.t.Fatalf("读取智能体收件箱 session 失败: %v", err)
	}
	if sessionValue == nil {
		f.fatalfInbox("智能体收件箱 session 未创建", sessionKey, nil)
	}
	history := workspacestore.NewAgentHistoryStore(f.cfg.WorkspacePath)
	messages, err := history.ReadMessages(f.workspacePath, *sessionValue, nil)
	if err != nil {
		f.t.Fatalf("读取智能体收件箱消息失败: %v", err)
	}
	if len(messages) != 1 || automationIngressMessageText(messages[0]) != expectedText {
		f.fatalfInbox("智能体收件箱消息不正确", sessionKey, messages)
	}
	summary, ok := messages[0]["result_summary"].(map[string]any)
	if !ok || automationIngressString(summary, "subtype") != "success" {
		f.fatalfInbox("智能体收件箱应挂载成功 result_summary", sessionKey, messages)
	}
}

func (f *automationIngressFixture) fatalfInbox(message string, sessionKey string, payload any) {
	f.t.Helper()
	f.t.Fatalf("%s: session_key=%s payload=%+v", message, sessionKey, payload)
}

type automationIngressRuntimeFactory struct {
	t            *testing.T
	mu           sync.Mutex
	clients      []*automationIngressRuntimeClient
	allowedTools map[string]bool
}

func newAutomationIngressRuntimeFactory(t *testing.T) *automationIngressRuntimeFactory {
	t.Helper()
	return &automationIngressRuntimeFactory{t: t, allowedTools: map[string]bool{}}
}

func (f *automationIngressRuntimeFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	client := &automationIngressRuntimeClient{
		t:         f.t,
		factory:   f,
		options:   options,
		sessionID: fmt.Sprintf("automation-ingress-sdk-session-%d", len(f.clients)+1),
		messages:  make(chan sdkprotocol.ReceivedMessage, 8),
	}
	f.clients = append(f.clients, client)
	return client
}

func (f *automationIngressRuntimeFactory) markAllowedTool(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowedTools[strings.TrimSpace(name)] = true
}

func (f *automationIngressRuntimeFactory) AllowedTool(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.allowedTools[strings.TrimSpace(name)]
}

func (f *automationIngressRuntimeFactory) AllowedTools() map[string]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[string]bool, len(f.allowedTools))
	for key, value := range f.allowedTools {
		result[key] = value
	}
	return result
}

type automationIngressRuntimeClient struct {
	t         *testing.T
	factory   *automationIngressRuntimeFactory
	options   agentclient.Options
	sessionID string
	messages  chan sdkprotocol.ReceivedMessage
}

func (c *automationIngressRuntimeClient) Connect(context.Context) error {
	return nil
}

func (c *automationIngressRuntimeClient) Query(ctx context.Context, prompt string) error {
	switch {
	case strings.Contains(prompt, "每天 9 点搜索重要新闻"):
		c.emitToolOutcome(c.createScheduledTaskThroughMCP(ctx), "已创建飞书群每日新闻定时任务。")
	case strings.Contains(prompt, "改成每天 10 点"):
		c.emitToolOutcome(c.callScheduledTaskTool(ctx, "update_scheduled_task", map[string]any{
			"query": "飞书群每日新闻",
			"schedule": map[string]any{
				"kind":       "daily",
				"daily_time": "10:00",
				"timezone":   "Asia/Shanghai",
			},
		}), "已把飞书群每日新闻改成每天 10 点。")
	case strings.Contains(prompt, "暂停飞书群每日新闻"):
		c.emitToolOutcome(c.callScheduledTaskTool(ctx, "disable_scheduled_task", map[string]any{
			"query": "飞书群每日新闻",
		}), "已暂停飞书群每日新闻。")
	case strings.Contains(prompt, "恢复飞书群每日新闻"):
		c.emitToolOutcome(c.callScheduledTaskTool(ctx, "enable_scheduled_task", map[string]any{
			"query": "飞书群每日新闻",
		}), "已恢复飞书群每日新闻。")
	case strings.Contains(prompt, "删除飞书群每日新闻"):
		c.emitToolOutcome(c.callScheduledTaskTool(ctx, "delete_scheduled_task", map[string]any{
			"query": "飞书群每日新闻",
		}), "已删除飞书群每日新闻。")
	case strings.Contains(prompt, "停止正在运行的飞书群每日新闻"):
		c.emitToolOutcome(c.callScheduledTaskTool(ctx, "disable_scheduled_task", map[string]any{
			"query":             "飞书群每日新闻",
			"cancel_active_run": true,
		}), "已停止正在运行的飞书群每日新闻。")
	case strings.Contains(prompt, "检查今天飞书群每日新闻发送情况"):
		c.emitToolOutcome(c.inspectFailedDeliveryThroughMCP(ctx), "飞书群每日新闻今天发送失败，可补发。")
	case strings.Contains(prompt, "失败发送改到智能体收件箱并补发"):
		c.emitToolOutcome(c.retryFailedDeliveryToAgentInboxThroughMCP(ctx), "已把失败发送补发到智能体收件箱。")
	case strings.Contains(prompt, "模拟长时间运行的新闻任务"):
		c.emitLongRunningTask(ctx)
	default:
		c.emitAssistantAndResult("今日新闻摘要", "success")
	}
	return nil
}

func (c *automationIngressRuntimeClient) createScheduledTaskThroughMCP(ctx context.Context) error {
	return c.callScheduledTaskTool(ctx, "create_scheduled_task", map[string]any{
		"name":              "飞书群每日新闻",
		"instruction":       "每天搜索重要新闻并发到这个飞书群",
		"execution_mode":    "dedicated",
		"named_session_key": "feishu-group-news",
		"reply_mode":        "channel",
		"schedule": map[string]any{
			"kind":       "daily",
			"daily_time": "09:00",
			"timezone":   "Asia/Shanghai",
		},
	})
}

func (c *automationIngressRuntimeClient) inspectFailedDeliveryThroughMCP(ctx context.Context) error {
	var report protocol.CronDailyReport
	if err := c.callScheduledTaskToolPayload(ctx, "get_scheduled_task_daily_report", map[string]any{
		"query":    "飞书群每日新闻",
		"date":     automationIngressToday(),
		"timezone": "Asia/Shanghai",
	}, &report); err != nil {
		return err
	}
	runID := automationIngressFailedDeliveryRunID(report)
	if runID == "" {
		return fmt.Errorf("daily report missing failed delivery run: %+v", report.Tasks)
	}

	var status protocol.CronTaskStatus
	if err := c.callScheduledTaskToolPayload(ctx, "get_scheduled_task_status", map[string]any{
		"query":     "飞书群每日新闻",
		"run_limit": 3,
	}, &status); err != nil {
		return err
	}
	if !status.Health.ManualRedeliveryAvailable || status.Health.DeliveryFailedRunCount == 0 {
		return fmt.Errorf("task status should expose manual redelivery: %+v", status.Health)
	}
	return nil
}

func (c *automationIngressRuntimeClient) retryFailedDeliveryToAgentInboxThroughMCP(ctx context.Context) error {
	var report protocol.CronDailyReport
	if err := c.callScheduledTaskToolPayload(ctx, "get_scheduled_task_daily_report", map[string]any{
		"query":    "飞书群每日新闻",
		"date":     automationIngressToday(),
		"timezone": "Asia/Shanghai",
	}, &report); err != nil {
		return err
	}
	runID := automationIngressFailedDeliveryRunID(report)
	if runID == "" {
		return fmt.Errorf("daily report missing failed delivery run before retry: %+v", report.Tasks)
	}
	if err := c.callScheduledTaskTool(ctx, "update_scheduled_task", map[string]any{
		"query":      "飞书群每日新闻",
		"reply_mode": "agent",
	}); err != nil {
		return err
	}
	var retried protocol.CronRun
	if err := c.callScheduledTaskToolPayload(ctx, "retry_scheduled_task_delivery", map[string]any{
		"query":  "飞书群每日新闻",
		"run_id": runID,
	}, &retried); err != nil {
		return err
	}
	if retried.RunID != runID || retried.DeliveryStatus != protocol.DeliveryStatusSucceeded {
		return fmt.Errorf("retry result should be succeeded for run %s: %+v", runID, retried)
	}
	return nil
}

func (c *automationIngressRuntimeClient) callScheduledTaskTool(ctx context.Context, name string, arguments map[string]any) error {
	_, err := c.callScheduledTaskToolText(ctx, name, arguments)
	return err
}

func (c *automationIngressRuntimeClient) callScheduledTaskToolPayload(ctx context.Context, name string, arguments map[string]any, out any) error {
	text, err := c.callScheduledTaskToolText(ctx, name, arguments)
	if err != nil {
		return err
	}
	if err = json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("decode %s response: %w", name, err)
	}
	return nil
}

func (c *automationIngressRuntimeClient) callScheduledTaskToolText(ctx context.Context, name string, arguments map[string]any) (string, error) {
	handler := c.options.Callbacks.PermissionHandler
	if handler == nil {
		return "", fmt.Errorf("permission handler missing")
	}
	decision, err := handler(ctx, sdkpermission.Request{
		ToolName: "mcp__nexus_automation__" + name,
		Input:    arguments,
	})
	if err != nil {
		return "", err
	}
	if decision.Behavior != sdkpermission.BehaviorAllow {
		return "", fmt.Errorf("%s permission denied: %+v", name, decision)
	}
	c.factory.markAllowedTool(name)

	server := c.options.MCP.SDKServers[automationmcpcontract.ServerName]
	if server == nil {
		return "", fmt.Errorf("nexus_automation MCP server missing")
	}
	response, err := server.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	})
	if err != nil {
		return "", err
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("MCP response missing result: %+v", response)
	}
	if isError, _ := result["isError"].(bool); isError {
		payload, _ := json.Marshal(result)
		return "", fmt.Errorf("MCP tool returned error: %s", payload)
	}
	return automationIngressToolText(result)
}

func (c *automationIngressRuntimeClient) emitToolOutcome(err error, success string) {
	if err != nil {
		c.t.Errorf("通过 runtime MCP 管理定时任务失败: %v", err)
		c.emitAssistantAndResult("定时任务操作失败: "+err.Error(), "failed")
		return
	}
	c.emitAssistantAndResult(success, "success")
}

func (c *automationIngressRuntimeClient) emitLongRunningTask(ctx context.Context) {
	select {
	case <-time.After(300 * time.Millisecond):
		c.emitAssistantAndResult("长时间运行任务完成", "success")
	case <-ctx.Done():
		return
	}
}

func (c *automationIngressRuntimeClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *automationIngressRuntimeClient) SendContent(context.Context, any, *string, string) error {
	return nil
}

func (c *automationIngressRuntimeClient) Interrupt(context.Context) error {
	return nil
}

func (c *automationIngressRuntimeClient) Disconnect(context.Context) error {
	return nil
}

func (c *automationIngressRuntimeClient) Reconfigure(_ context.Context, options agentclient.Options) error {
	c.options = options
	return nil
}

func (c *automationIngressRuntimeClient) SessionID() string {
	return c.sessionID
}

func (c *automationIngressRuntimeClient) emitAssistantAndResult(text string, subtype string) {
	c.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: c.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-" + c.sessionID,
				Model: "test",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: text},
				},
			},
		},
	}
	c.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: c.sessionID,
		UUID:      "result-" + c.sessionID,
		Result: &sdkprotocol.ResultMessage{
			Subtype: subtype,
			Result:  text,
		},
	}
}

func newAutomationIngressTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:                    "127.0.0.1",
		Port:                    18042,
		ProjectName:             "nexus-automation-ingress-test",
		APIPrefix:               "/nexus/v1",
		WebSocketPath:           "/nexus/v1/chat/ws",
		DefaultAgentID:          "nexus",
		DefaultTimezone:         "Asia/Shanghai",
		WorkspacePath:           filepath.Join(root, "workspace"),
		DatabaseDriver:          "sqlite",
		DatabaseURL:             filepath.Join(root, "nexus.db"),
		ConnectorCredentialsKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
	}
}

func migrateAutomationIngressSQLite(t *testing.T, cfg config.Config) *sql.DB {
	t.Helper()
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, automationIngressMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
	return db
}

func automationIngressMigrationDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位当前测试文件")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "db", "migrations", "sqlite")
}

func waitForAutomationIngress(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !predicate() {
		t.Fatalf("等待条件超时: %s", timeout)
	}
}

func automationIngressToolText(result map[string]any) (string, error) {
	switch content := result["content"].(type) {
	case []map[string]any:
		if len(content) == 0 {
			return "", fmt.Errorf("MCP tool response content is empty: %+v", result)
		}
		text := automationIngressString(content[0], "text")
		if text == "" {
			return "", fmt.Errorf("MCP tool response text is empty: %+v", content[0])
		}
		return text, nil
	case []any:
		if len(content) == 0 {
			return "", fmt.Errorf("MCP tool response content is empty: %+v", result)
		}
		item, ok := content[0].(map[string]any)
		if !ok {
			return "", fmt.Errorf("MCP tool response content item is invalid: %+v", content[0])
		}
		text := automationIngressString(item, "text")
		if text == "" {
			return "", fmt.Errorf("MCP tool response text is empty: %+v", item)
		}
		return text, nil
	default:
		return "", fmt.Errorf("MCP tool response content is invalid: %+v", result)
	}
}

func automationIngressToday() string {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Now().Format("2006-01-02")
	}
	return time.Now().In(location).Format("2006-01-02")
}

func automationIngressFailedDeliveryRunID(report protocol.CronDailyReport) string {
	for _, task := range report.Tasks {
		for _, runID := range task.ManualRedeliveryRunIDs {
			if trimmed := strings.TrimSpace(runID); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func automationIngressMessageText(message protocol.Message) string {
	if text := automationIngressString(map[string]any(message), "content"); text != "" {
		return text
	}
	items, ok := message["content"].([]map[string]any)
	if ok {
		return joinAutomationIngressTextBlocks(items)
	}
	rawItems, ok := message["content"].([]any)
	if !ok {
		return ""
	}
	normalized := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if ok {
			normalized = append(normalized, item)
		}
	}
	return joinAutomationIngressTextBlocks(normalized)
}

func joinAutomationIngressTextBlocks(items []map[string]any) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if automationIngressString(item, "type") != "text" {
			continue
		}
		if text := automationIngressString(item, "text"); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func automationIngressString(message map[string]any, key string) string {
	value, _ := message[key].(string)
	return strings.TrimSpace(value)
}
