package host

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestScheduledTaskCLIManagementCommands(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	agentID := createCLITestAgent(t, cfg)
	deliverySessionKey := protocol.BuildAgentSessionKey(
		agentID,
		protocol.SessionChannelInternalSegment,
		"dm",
		"cli-delivery",
		"",
	)
	prepareCLIDeliverySession(t, cfg, agentID, deliverySessionKey)
	createPayload := runCLICommand(
		t,
		cfg,
		"automation",
		"task",
		"create",
		"--name",
		"每日新闻",
		"--agent-id",
		agentID,
		"--instruction",
		"每天搜索新闻并汇总",
		"--interval-seconds",
		"3600",
		"--target-kind",
		automationdomain.SessionTargetNamed,
		"--named-session-key",
		"news",
		"--delivery-mode",
		automationdomain.DeliveryModeExplicit,
		"--delivery-channel",
		protocol.SessionChannelInternalSegment,
		"--delivery-session-key",
		deliverySessionKey,
	)
	task := asMap(t, createPayload["item"])
	jobID := asString(t, task["job_id"])
	assertNestedString(t, task, "source", "kind", automationdomain.SourceKindCLI)
	assertNestedString(t, task, "delivery", "mode", automationdomain.DeliveryModeExplicit)
	assertNestedString(t, task, "delivery", "channel", protocol.SessionChannelInternalSegment)
	assertNestedString(t, task, "delivery", "session_key", deliverySessionKey)

	nextSessionKey := protocol.BuildAgentSessionKey(
		agentID,
		protocol.SessionChannelInternalSegment,
		"dm",
		"automation-secondary",
		"",
	)
	prepareCLIDeliverySession(t, cfg, agentID, nextSessionKey)
	updatePayload := runCLICommand(t, cfg, "automation", "task", "update", jobID, "--delivery-session-key", nextSessionKey)
	updated := asMap(t, updatePayload["item"])
	assertNestedString(t, updated, "delivery", "mode", automationdomain.DeliveryModeExplicit)
	assertNestedString(t, updated, "delivery", "channel", protocol.SessionChannelInternalSegment)
	assertNestedString(t, updated, "delivery", "session_key", nextSessionKey)

	disablePayload := runCLICommand(t, cfg, "automation", "task", "disable", jobID)
	if asMap(t, disablePayload["item"])["enabled"] != false {
		t.Fatalf("disable 应停用任务: %+v", disablePayload)
	}
	enablePayload := runCLICommand(t, cfg, "automation", "task", "enable", jobID)
	if asMap(t, enablePayload["item"])["enabled"] != true {
		t.Fatalf("enable 应启用任务: %+v", enablePayload)
	}

	inspectPayload := runCLICommand(t, cfg, "automation", "task", "inspect", jobID, "--run-limit", "5", "--event-limit", "5")
	inspect := asMap(t, inspectPayload["item"])
	health := asMap(t, inspect["health"])
	if state := asString(t, health["state"]); state != "scheduled" {
		t.Fatalf("inspect health state = %s, 期望 scheduled: %+v", state, inspect)
	}

	eventsPayload := runCLICommand(t, cfg, "automation", "task", "events", jobID, "--limit", "10")
	assertEventActions(t, eventsPayload["items"], "create", "update", "disable", "enable")

	reportPayload := runCLICommand(t, cfg, "automation", "task", "report", "--agent-id", agentID)
	report := asMap(t, reportPayload["item"])
	totals := asMap(t, report["totals"])
	if asInt(t, totals["task_count"]) != 1 || asInt(t, totals["enabled_task_count"]) != 1 {
		t.Fatalf("report totals 不正确: %+v", totals)
	}
}

func TestScheduledTaskCLIRetryDeliveryCommand(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	agentID := createCLITestAgent(t, cfg)
	createPayload := runCLICommand(
		t,
		cfg,
		"automation",
		"task",
		"create",
		"--name",
		"飞书日报",
		"--agent-id",
		agentID,
		"--instruction",
		"每天汇总新闻",
		"--interval-seconds",
		"3600",
		"--target-kind",
		automationdomain.SessionTargetNamed,
		"--named-session-key",
		"news",
		"--delivery-mode",
		automationdomain.DeliveryModeNone,
	)
	jobID := asString(t, asMap(t, createPayload["item"])["job_id"])
	prepareCLILegacyBareDeliveryTask(t, cfg.DatabaseURL, jobID)
	insertCLIFailedDeliveryRun(t, cfg.DatabaseURL, jobID)

	inspectPayload := runCLICommand(t, cfg, "automation", "task", "inspect", jobID)
	health := asMap(t, asMap(t, inspectPayload["item"])["health"])
	if !asBool(t, health["manual_redelivery_available"]) {
		t.Fatalf("inspect 应暴露可手动补投递: %+v", health)
	}

	reportPayload := runCLICommand(t, cfg, "automation", "task", "report", "--date", "2026-05-22", "--job-id", jobID)
	totals := asMap(t, asMap(t, reportPayload["item"])["totals"])
	if asInt(t, totals["delivery_failed_run_count"]) != 1 {
		t.Fatalf("report 应统计投递失败: %+v", totals)
	}

	deliverySessionKey := protocol.BuildAgentSessionKey(
		agentID,
		protocol.SessionChannelInternalSegment,
		"dm",
		"delivery-recovery",
		"",
	)
	prepareCLIDeliverySession(t, cfg, agentID, deliverySessionKey)
	runCLICommand(
		t,
		cfg,
		"automation",
		"task",
		"update",
		jobID,
		"--delivery-channel",
		protocol.SessionChannelInternalSegment,
		"--delivery-session-key",
		deliverySessionKey,
	)
	retryPayload := runCLICommand(t, cfg, "automation", "task", "retry-delivery", jobID, "run-delivery-failed")
	retried := asMap(t, retryPayload["item"])
	if status := asString(t, retried["delivery_status"]); status != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("retry-delivery 应投递成功，实际 %s: %+v", status, retried)
	}
	if to := asString(t, retried["delivery_to"]); !strings.Contains(to, "explicit:internal:") {
		t.Fatalf("retry-delivery 应记录内部投递目标，实际 %q: %+v", to, retried)
	}
}

func prepareCLIDeliverySession(t *testing.T, cfg config.Config, agentID string, sessionKey string) {
	t.Helper()
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开 CLI 测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	var workspacePath string
	if err = db.QueryRow(`SELECT workspace_path FROM agents WHERE id = ?`, agentID).Scan(&workspacePath); err != nil {
		t.Fatalf("读取 CLI Agent workspace 失败: %v", err)
	}
	now := time.Now().UTC()
	if _, err = workspacestore.NewSessionFileStore(cfg.WorkspacePath).
		ForOwner(authctx.SystemUserID).
		UpsertSession(workspacePath, protocol.Session{
			SessionKey: sessionKey, AgentID: agentID,
			ChannelType: protocol.SessionChannelInternalSegment, ChatType: protocol.RoomTypeDM,
			Status: "active", CreatedAt: now, LastActivity: now,
			Title: "CLI 真实接收会话", Options: map[string]any{}, IsActive: true,
		}); err != nil {
		t.Fatalf("准备 CLI 真实接收会话失败: %v", err)
	}
}

func prepareCLILegacyBareDeliveryTask(t *testing.T, databaseURL string, jobID string) {
	t.Helper()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开 CLI 测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err = db.Exec(`
UPDATE automation_scheduled_tasks
SET delivery_mode = 'explicit', delivery_channel = 'feishu',
    delivery_to = 'bad-chat-id', delivery_session_key = NULL
WHERE job_id = ?`, jobID); err != nil {
		t.Fatalf("模拟 CLI 历史裸投递任务失败: %v", err)
	}
}

func createCLITestAgent(t *testing.T, cfg config.Config) string {
	t.Helper()

	payload := runCLICommand(t, cfg, "agent", "create", "--name", "cli-reporter")
	return asString(t, asMap(t, payload["item"])["agent_id"])
}

func insertCLIFailedDeliveryRun(t *testing.T, databaseURL string, jobID string) {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开 CLI 测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	scheduledFor := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	_, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    session_key, message_count, delivery_mode, delivery_to, delivery_status,
    delivery_error, delivery_attempts, scheduled_for, started_at, finished_at,
    attempts, result_text
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-delivery-failed",
		jobID,
		authctx.SystemUserID,
		automationdomain.RunStatusSucceeded,
		"schedule",
		"agent:cli-reporter:automation:dm:cron:run-delivery-failed",
		1,
		automationdomain.DeliveryModeExplicit,
		"explicit:feishu:bad-chat-id",
		automationdomain.DeliveryStatusFailed,
		"bad chat_id",
		1,
		scheduledFor,
		scheduledFor,
		scheduledFor.Add(time.Minute),
		1,
		"今日新闻摘要",
	)
	if err != nil {
		t.Fatalf("写入失败投递 run 失败: %v", err)
	}
}

func assertNestedString(t *testing.T, item map[string]any, objectKey string, field string, expected string) {
	t.Helper()

	nested := asMap(t, item[objectKey])
	if actual := asString(t, nested[field]); actual != expected {
		t.Fatalf("%s.%s = %q, 期望 %q: %+v", objectKey, field, actual, expected, item)
	}
}

func assertEventActions(t *testing.T, value any, expected ...string) {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("events 输出结构不正确: %#v", value)
	}
	actions := make(map[string]struct{}, len(items))
	for _, raw := range items {
		event := asMap(t, raw)
		actions[asString(t, event["action"])] = struct{}{}
	}
	for _, action := range expected {
		if _, ok := actions[action]; !ok {
			t.Fatalf("events 缺少 action=%s: %+v", action, actions)
		}
	}
}

func asInt(t *testing.T, value any) int {
	t.Helper()

	number, ok := value.(float64)
	if !ok {
		t.Fatalf("输出结构不是数字: %#v", value)
	}
	return int(number)
}
