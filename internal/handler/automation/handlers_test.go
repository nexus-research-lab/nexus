package automation_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestScheduledTaskObservabilityHTTP(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name": "新闻日报",
		"agent_id": "nexus",
		"schedule": {"kind": "every", "interval_seconds": 3600, "timezone": "UTC"},
		"session_target": {"kind": "isolated"},
		"delivery": {"mode": "none"},
		"instruction": "搜索今天的重要新闻",
		"enabled": true
	}`))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("创建任务状态码不正确: got=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Data struct {
			JobID string `json:"job_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	if created.Data.JobID == "" {
		t.Fatalf("创建响应缺少 job_id: %s", createRecorder.Body.String())
	}

	statusRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s/status?run_limit=2&event_limit=2", created.Data.JobID),
		nil,
	)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("查询状态状态码不正确: got=%d body=%s", statusRecorder.Code, statusRecorder.Body.String())
	}
	var status struct {
		Data struct {
			Job struct {
				JobID string `json:"job_id"`
			} `json:"job"`
			Health struct {
				State string `json:"state"`
			} `json:"health"`
			RecentEvents []struct {
				Action string `json:"action"`
			} `json:"recent_events"`
		} `json:"data"`
	}
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("解析状态响应失败: %v", err)
	}
	if status.Data.Job.JobID != created.Data.JobID || status.Data.Health.State == "" || len(status.Data.RecentEvents) == 0 {
		t.Fatalf("状态响应不完整: %+v", status.Data)
	}

	eventsRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s/events?limit=2", created.Data.JobID),
		nil,
	)
	if eventsRecorder.Code != http.StatusOK {
		t.Fatalf("查询事件状态码不正确: got=%d body=%s", eventsRecorder.Code, eventsRecorder.Body.String())
	}
	var events struct {
		Data []struct {
			Action string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal(eventsRecorder.Body.Bytes(), &events); err != nil {
		t.Fatalf("解析事件响应失败: %v", err)
	}
	if len(events.Data) == 0 || events.Data[0].Action == "" {
		t.Fatalf("事件响应不完整: %+v", events.Data)
	}

	reportRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/nexus/v1/capability/scheduled/reports/daily?date=2026-05-21&timezone=UTC&job_id=%s", created.Data.JobID),
		nil,
	)
	if reportRecorder.Code != http.StatusOK {
		t.Fatalf("查询日报状态码不正确: got=%d body=%s", reportRecorder.Code, reportRecorder.Body.String())
	}
	var report struct {
		Data struct {
			JobID  string `json:"job_id"`
			Totals struct {
				TaskCount int `json:"task_count"`
			} `json:"totals"`
			Tasks []struct {
				JobID string `json:"job_id"`
			} `json:"tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(reportRecorder.Body.Bytes(), &report); err != nil {
		t.Fatalf("解析日报响应失败: %v", err)
	}
	if report.Data.JobID != created.Data.JobID || report.Data.Totals.TaskCount != 1 || len(report.Data.Tasks) != 1 {
		t.Fatalf("日报响应不完整: %+v", report.Data)
	}
}

func TestScheduledTaskDeliveryRecoveryHTTP(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name": "飞书日报",
		"agent_id": "nexus",
		"schedule": {"kind": "every", "interval_seconds": 3600, "timezone": "UTC"},
		"session_target": {"kind": "isolated"},
		"delivery": {"mode": "explicit", "channel": "feishu", "to": "oc_missing_group"},
		"instruction": "搜索今天的重要新闻",
		"enabled": true
	}`))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("创建任务状态码不正确: got=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Data struct {
			JobID string `json:"job_id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	if created.Data.JobID == "" {
		t.Fatalf("创建响应缺少 job_id: %s", createRecorder.Body.String())
	}

	runID := "run-http-delivery-failed"
	insertHTTPFailedDeliveryRun(t, cfg.DatabaseURL, created.Data.JobID, runID)

	reportRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/nexus/v1/capability/scheduled/reports/daily?date=2026-05-22&timezone=UTC&job_id=%s", created.Data.JobID),
		nil,
	)
	if reportRecorder.Code != http.StatusOK {
		t.Fatalf("查询日报状态码不正确: got=%d body=%s", reportRecorder.Code, reportRecorder.Body.String())
	}
	var report struct {
		Data struct {
			Tasks []struct {
				Signals                []string `json:"signals"`
				SuggestedTools         []string `json:"suggested_tools"`
				ManualRedeliveryRunIDs []string `json:"manual_redelivery_run_ids"`
			} `json:"tasks"`
		} `json:"data"`
	}
	if err = json.Unmarshal(reportRecorder.Body.Bytes(), &report); err != nil {
		t.Fatalf("解析日报响应失败: %v", err)
	}
	if len(report.Data.Tasks) != 1 ||
		!containsString(report.Data.Tasks[0].Signals, "delivery_attention") ||
		!containsString(report.Data.Tasks[0].SuggestedTools, "retry_scheduled_task_delivery") ||
		!containsString(report.Data.Tasks[0].ManualRedeliveryRunIDs, runID) {
		t.Fatalf("日报应暴露失败投递的可恢复信号: %+v", report.Data.Tasks)
	}

	inboxKey := protocol.BuildAgentSessionKey(
		"nexus",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	updateBody := []byte(fmt.Sprintf(`{
		"delivery": {"mode": "explicit", "channel": "internal", "to": %q}
	}`, inboxKey))
	updateRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPatch,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s", created.Data.JobID),
		updateBody,
	)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("修正投递目标状态码不正确: got=%d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}

	retryRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPost,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s/runs/%s/delivery/retry", created.Data.JobID, runID),
		[]byte(`{}`),
	)
	if retryRecorder.Code != http.StatusOK {
		t.Fatalf("重试投递状态码不正确: got=%d body=%s", retryRecorder.Code, retryRecorder.Body.String())
	}
	var retry struct {
		Data struct {
			RunID                 string  `json:"run_id"`
			DeliveryStatus        string  `json:"delivery_status"`
			DeliveryTo            string  `json:"delivery_to"`
			DeliveryError         *string `json:"delivery_error"`
			DeliveryAttempts      int     `json:"delivery_attempts"`
			DeliveryNextAttemptAt *string `json:"delivery_next_attempt_at"`
		} `json:"data"`
	}
	if err = json.Unmarshal(retryRecorder.Body.Bytes(), &retry); err != nil {
		t.Fatalf("解析重试响应失败: %v", err)
	}
	if retry.Data.RunID != runID ||
		retry.Data.DeliveryStatus != automationdomain.DeliveryStatusSucceeded ||
		retry.Data.DeliveryTo != "explicit:internal:"+inboxKey ||
		retry.Data.DeliveryError != nil ||
		retry.Data.DeliveryAttempts != 2 ||
		retry.Data.DeliveryNextAttemptAt != nil {
		t.Fatalf("重试投递响应不完整: %+v", retry.Data)
	}

	assertHTTPDeliveryInboxMessage(t, agentHTTPWorkspacePath(t, cfg.DatabaseURL, "nexus"), inboxKey, "今日新闻摘要")
}

func serveAutomationJSON(t *testing.T, server *serverapp.Server, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	reader := bytes.NewReader(body)
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	return recorder
}

func insertHTTPFailedDeliveryRun(t *testing.T, databaseURL string, jobID string, runID string) {
	t.Helper()
	db := handlertest.OpenSQLite(t, databaseURL)
	defer func() { _ = db.Close() }()

	var ownerUserID string
	if err := db.QueryRow(`SELECT owner_user_id FROM automation_scheduled_tasks WHERE job_id = ?`, jobID).Scan(&ownerUserID); err != nil {
		t.Fatalf("读取任务 owner_user_id 失败: %v", err)
	}
	if strings.TrimSpace(ownerUserID) == "" {
		ownerUserID = authctx.SystemUserID
	}

	scheduledFor := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	deliveryNextAttemptAt := scheduledFor.Add(10 * time.Minute)
	_, err := db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    session_key, round_id, message_count,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at,
    scheduled_for, started_at, finished_at, attempts,
    result_summary, result_text, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		jobID,
		ownerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled,
		protocol.BuildAgentSessionKey("nexus", "automation", "dm", "scheduled-task:"+jobID+":"+runID, ""),
		"round-"+runID,
		1,
		automationdomain.DeliveryModeExplicit,
		"explicit:feishu:oc_missing_group",
		automationdomain.DeliveryStatusFailed,
		"feishu send message failed: bad chat_id",
		1,
		deliveryNextAttemptAt.Format(time.RFC3339Nano),
		scheduledFor.Format(time.RFC3339Nano),
		scheduledFor.Format(time.RFC3339Nano),
		scheduledFor.Add(time.Minute).Format(time.RFC3339Nano),
		1,
		"今日新闻摘要",
		"今日新闻摘要",
		scheduledFor.Format(time.RFC3339Nano),
		scheduledFor.Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("插入失败投递 run 失败: %v", err)
	}
}

func agentHTTPWorkspacePath(t *testing.T, databaseURL string, agentID string) string {
	t.Helper()
	db := handlertest.OpenSQLite(t, databaseURL)
	defer func() { _ = db.Close() }()
	var workspacePath string
	if err := db.QueryRow(`SELECT workspace_path FROM agents WHERE id = ?`, agentID).Scan(&workspacePath); err != nil {
		t.Fatalf("读取 agent workspace_path 失败: %v", err)
	}
	return workspacePath
}

func assertHTTPDeliveryInboxMessage(t *testing.T, workspacePath string, sessionKey string, expectedText string) {
	t.Helper()
	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionValue, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取投递收件箱 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("重试投递应自动创建定时任务收件箱")
	}
	history := workspacestore.NewAgentHistoryStore(workspacePath)
	messages, err := history.ReadMessages(workspacePath, *sessionValue, nil)
	if err != nil {
		t.Fatalf("读取投递收件箱消息失败: %v", err)
	}
	if len(messages) != 1 || extractHTTPAssistantText(messages[0]) != expectedText {
		t.Fatalf("投递收件箱消息不正确: %+v", messages)
	}
}

func extractHTTPAssistantText(message protocol.Message) string {
	items, ok := message["content"].([]map[string]any)
	if !ok {
		rawItems, ok := message["content"].([]any)
		if !ok {
			return ""
		}
		items = make([]map[string]any, 0, len(rawItems))
		for _, raw := range rawItems {
			if item, ok := raw.(map[string]any); ok {
				items = append(items, item)
			}
		}
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if text, _ := item["text"].(string); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func containsString(items []string, expected string) bool {
	return slices.Contains(items, expected)
}
