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
	handlertest.CloseServer(t, server)

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name": "新闻日报",
		"agent_id": "nexus",
		"schedule": {"kind": "every", "interval_seconds": 3600, "timezone": "UTC"},
		"permission_mode": "plan",
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
			JobID          string `json:"job_id"`
			PermissionMode string `json:"permission_mode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	if created.Data.JobID == "" {
		t.Fatalf("创建响应缺少 job_id: %s", createRecorder.Body.String())
	}
	if created.Data.PermissionMode != automationdomain.PermissionModePlan {
		t.Fatalf("创建请求未保存 permission_mode: %+v", created.Data)
	}

	updateRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPatch,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s", created.Data.JobID),
		[]byte(`{"permission_mode":"dontAsk"}`),
	)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("更新权限模式状态码不正确: got=%d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated struct {
		Data struct {
			PermissionMode string `json:"permission_mode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	if updated.Data.PermissionMode != automationdomain.PermissionModeDontAsk {
		t.Fatalf("更新请求未保存 permission_mode: %+v", updated.Data)
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

func TestScheduledTaskHTTPRejectsScriptMutation(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name":"脚本任务","agent_id":"nexus",
		"schedule":{"kind":"every","interval_seconds":3600,"timezone":"UTC"},
		"execution_kind":"script","session_target":{"kind":"isolated"},
		"delivery":{"mode":"none"},"instruction":"echo unsafe","enabled":false
	}`))
	if createRecorder.Code != http.StatusBadRequest || !strings.Contains(createRecorder.Body.String(), "脚本任务") {
		t.Fatalf("页面创建脚本任务应失败: got=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}

	agentRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name":"Agent 任务","agent_id":"nexus",
		"schedule":{"kind":"every","interval_seconds":3600,"timezone":"UTC"},
		"session_target":{"kind":"isolated"},"delivery":{"mode":"none"},
		"instruction":"生成日报","enabled":false
	}`))
	if agentRecorder.Code != http.StatusOK {
		t.Fatalf("创建 Agent 任务失败: got=%d body=%s", agentRecorder.Code, agentRecorder.Body.String())
	}
	var created struct {
		Data struct {
			JobID string `json:"job_id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(agentRecorder.Body.Bytes(), &created); err != nil || created.Data.JobID == "" {
		t.Fatalf("解析 Agent 任务失败: err=%v body=%s", err, agentRecorder.Body.String())
	}
	updateRecorder := serveAutomationJSON(
		t, server, http.MethodPatch,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s", created.Data.JobID),
		[]byte(`{"execution_kind":"script","instruction":"echo unsafe"}`),
	)
	if updateRecorder.Code != http.StatusBadRequest || !strings.Contains(updateRecorder.Body.String(), "脚本任务") {
		t.Fatalf("页面改成脚本任务应失败: got=%d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
}

func TestScheduledTaskHTTPRejectsUnpairedIMRebindAsClientError(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name": "等待重绑",
		"agent_id": "nexus",
		"schedule": {"kind": "every", "interval_seconds": 3600, "timezone": "UTC"},
		"session_target": {"kind": "isolated"},
		"delivery": {"mode": "none"},
		"instruction": "测试未配对目标",
		"enabled": false
	}`))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("创建任务失败: got=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Data struct {
			JobID string `json:"job_id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil || created.Data.JobID == "" {
		t.Fatalf("解析创建响应失败: err=%v body=%s", err, createRecorder.Body.String())
	}

	unpairedSessionKey := protocol.BuildAgentAccountSessionKey(
		"nexus",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"removed-account",
		"removed-contact",
		"",
	)
	updateBody := []byte(fmt.Sprintf(`{
		"delivery": {"mode": "last", "session_key": %q}
	}`, unpairedSessionKey))
	updateRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPatch,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s", created.Data.JobID),
		updateBody,
	)
	if updateRecorder.Code != http.StatusBadRequest ||
		!strings.Contains(updateRecorder.Body.String(), automationdomain.ErrTaskDeliverySessionUnavailable.Error()) {
		t.Fatalf("未配对 IM 重绑应返回明确客户端错误: got=%d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
}

func TestScheduledTaskHTTPRejectsBareIMDeliveryTarget(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	recorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name":"裸 IM 目标","agent_id":"nexus",
		"schedule":{"kind":"every","interval_seconds":3600,"timezone":"UTC"},
		"session_target":{"kind":"isolated"},
		"delivery":{"mode":"explicit","channel":"feishu","to":"oc_model_guessed"},
		"instruction":"生成日报","enabled":false
	}`))
	if recorder.Code != http.StatusBadRequest ||
		!strings.Contains(recorder.Body.String(), automationdomain.ErrTaskDeliverySessionUnavailable.Error()) {
		t.Fatalf("裸 IM 目标必须被拒绝: got=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestScheduledTaskHTTPUpdatePreservesAgentCreatedProvenance(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name": "旧 Agent 任务",
		"agent_id": "nexus",
		"schedule": {"kind": "every", "interval_seconds": 3600, "timezone": "UTC"},
		"session_target": {"kind": "isolated"},
		"delivery": {"mode": "none"},
		"instruction": "保持原配置",
		"enabled": false
	}`))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("创建任务失败: got=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Data struct {
			JobID string `json:"job_id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil || created.Data.JobID == "" {
		t.Fatalf("解析创建响应失败: err=%v body=%s", err, createRecorder.Body.String())
	}

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	_, err = db.Exec(`
UPDATE automation_scheduled_tasks
SET source_kind = 'agent',
    source_creator_agent_id = 'nexus',
    source_context_type = 'agent',
    source_context_id = 'nexus',
    source_context_label = 'Nexus',
    delivery_grant_json = json_object(
        'kind', 'agent',
        'creator_agent_id', 'nexus',
        'context_type', 'agent',
        'context_id', 'nexus',
        'context_label', 'Nexus'
    )
WHERE job_id = ?`, created.Data.JobID)
	_ = db.Close()
	if err != nil {
		t.Fatalf("模拟旧 Agent 任务失败: %v", err)
	}

	updateRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPatch,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s", created.Data.JobID),
		[]byte(`{"name":"网页已修改","delivery":{"mode":"none"}}`),
	)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("网页编辑旧任务失败: got=%d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated struct {
		Data struct {
			Name   string                  `json:"name"`
			Source automationdomain.Source `json:"source"`
		} `json:"data"`
	}
	if err = json.Unmarshal(updateRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	if updated.Data.Name != "网页已修改" ||
		updated.Data.Source.Kind != automationdomain.SourceKindAgent ||
		updated.Data.Source.CreatorAgentID != "nexus" {
		t.Fatalf("网页更新破坏了创建 provenance: %+v", updated.Data)
	}
}

func TestScheduledTaskDeliveryRecoveryHTTP(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name": "飞书日报",
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
	if err = json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	if created.Data.JobID == "" {
		t.Fatalf("创建响应缺少 job_id: %s", createRecorder.Body.String())
	}

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	_, err = db.Exec(`
UPDATE automation_scheduled_tasks
SET delivery_mode = 'explicit',
    delivery_channel = 'feishu',
    delivery_to = 'oc_missing_group',
    delivery_session_key = NULL
WHERE job_id = ?`, created.Data.JobID)
	_ = db.Close()
	if err != nil {
		t.Fatalf("模拟旧版裸 IM 投递任务失败: %v", err)
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
		!containsString(report.Data.Tasks[0].SuggestedTools, "nexus.command automation apply") ||
		!containsString(report.Data.Tasks[0].ManualRedeliveryRunIDs, runID) {
		t.Fatalf("日报应暴露失败投递的可恢复信号: %+v", report.Data.Tasks)
	}

	recipientSessionKey := protocol.BuildAgentSessionKey(
		"nexus",
		protocol.SessionChannelInternalSegment,
		"dm",
		"delivery-recovery",
		"",
	)
	workspacePath := agentHTTPWorkspacePath(t, cfg.DatabaseURL, "nexus")
	now := time.Now().UTC()
	if _, err = workspacestore.NewSessionFileStore(workspacePath).UpsertSession(workspacePath, protocol.Session{
		SessionKey:   recipientSessionKey,
		AgentID:      "nexus",
		ChannelType:  protocol.SessionChannelInternalSegment,
		ChatType:     protocol.RoomTypeDM,
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "投递恢复会话",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("准备真实投递会话失败: %v", err)
	}
	updateBody := []byte(fmt.Sprintf(`{
		"delivery": {"mode": "explicit", "channel": "internal", "to": %q}
	}`, recipientSessionKey))
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
		retry.Data.DeliveryTo != "explicit:internal:"+recipientSessionKey ||
		retry.Data.DeliveryError != nil ||
		retry.Data.DeliveryAttempts != 2 ||
		retry.Data.DeliveryNextAttemptAt != nil {
		t.Fatalf("重试投递响应不完整: %+v", retry.Data)
	}

	assertHTTPDeliverySessionMessage(t, workspacePath, recipientSessionKey, "今日新闻摘要")
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

func assertHTTPDeliverySessionMessage(t *testing.T, workspacePath string, sessionKey string, expectedText string) {
	t.Helper()
	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionValue, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取投递会话失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("重试投递应保留真实接收会话")
	}
	history := workspacestore.NewAgentHistoryStore(workspacePath)
	messages, err := history.ReadMessages(workspacePath, *sessionValue, nil)
	if err != nil {
		t.Fatalf("读取投递会话消息失败: %v", err)
	}
	if len(messages) != 1 || extractHTTPAssistantText(messages[0]) != expectedText {
		t.Fatalf("投递会话消息不正确: %+v", messages)
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
