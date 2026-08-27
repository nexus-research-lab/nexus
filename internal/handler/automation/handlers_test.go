package automation_test

import (
	"bytes"
	"database/sql"
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

	insertHTTPFailedDeliveryRun(t, cfg.DatabaseURL, created.Data.JobID, "run-history-a")
	insertHTTPFailedDeliveryRun(t, cfg.DatabaseURL, created.Data.JobID, "run-history-b")
	runsRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/nexus/v1/capability/scheduled/tasks/%s/runs", created.Data.JobID),
		nil,
	)
	if runsRecorder.Code != http.StatusOK {
		t.Fatalf("查询运行历史状态码不正确: got=%d body=%s", runsRecorder.Code, runsRecorder.Body.String())
	}
	var runs struct {
		Code    string                              `json:"code"`
		Success bool                                `json:"success"`
		Data    []automationdomain.ScheduledTaskRun `json:"data"`
	}
	if err := json.Unmarshal(runsRecorder.Body.Bytes(), &runs); err != nil {
		t.Fatalf("解析运行历史响应失败: %v", err)
	}
	if runs.Code != "0000" || !runs.Success || len(runs.Data) != 2 {
		t.Fatalf("运行历史成功 envelope 被改变: %+v", runs)
	}
	for index, expectedRunID := range []string{"run-history-b", "run-history-a"} {
		run := runs.Data[index]
		if run.RunID != expectedRunID ||
			run.JobID != created.Data.JobID ||
			run.Status != automationdomain.RunStatusSucceeded ||
			run.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
			run.DeliveryAttempts != 1 {
			t.Fatalf("运行历史身份、阶段或顺序被改变: index=%d run=%+v", index, run)
		}
	}

	missingRequest := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/capability/scheduled/tasks/missing-job/runs",
		nil,
	)
	missingRequest.Header.Set("X-Request-ID", "automation-http-attempt")
	missingRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(missingRecorder, missingRequest)
	if missingRecorder.Code != http.StatusNotFound {
		t.Fatalf("缺失任务运行历史状态码不正确: got=%d body=%s", missingRecorder.Code, missingRecorder.Body.String())
	}
	var missing struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Success bool   `json:"success"`
		Data    struct {
			Detail    string               `json:"detail"`
			RequestID string               `json:"request_id"`
			Failure   protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(missingRecorder.Body.Bytes(), &missing); err != nil {
		t.Fatalf("解析缺失任务运行历史响应失败: %v", err)
	}
	if missing.Code != "404" || missing.Message != "failed" || missing.Success || missing.Data.Detail != "资源不存在" {
		t.Fatalf("运行历史 404 envelope 被改变: %+v", missing)
	}
	if missing.Data.RequestID != "automation-http-attempt" ||
		missing.Data.Failure.TransportRequestID != "automation-http-attempt" ||
		missing.Data.Failure.Code != "automation.run_history_not_found" ||
		missing.Data.Failure.Effect != protocol.FailureEffectNotApplicable {
		t.Fatalf("运行历史 FailureCore 不正确: %+v", missing.Data)
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

func TestScheduledTaskCreateHTTPUsesDomainRequestIDWithoutChangingLegacyCalls(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	payload := []byte(`{
		"request_id":"page-create-intent-01","name":"幂等页面任务","agent_id":"nexus",
		"schedule":{"kind":"every","interval_seconds":3600,"timezone":"UTC"},
		"session_target":{"kind":"isolated"},"delivery":{"mode":"none"},
		"instruction":"验证页面创建重放","enabled":false
	}`)
	firstRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPost,
		"/nexus/v1/capability/scheduled/tasks",
		payload,
	)
	secondRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPost,
		"/nexus/v1/capability/scheduled/tasks",
		payload,
	)
	if firstRecorder.Code != http.StatusOK || secondRecorder.Code != http.StatusOK {
		t.Fatalf(
			"同一页面创建意图应可安全重放: first=%d %s second=%d %s",
			firstRecorder.Code,
			firstRecorder.Body.String(),
			secondRecorder.Code,
			secondRecorder.Body.String(),
		)
	}
	var first, second struct {
		Data automationdomain.ScheduledTask `json:"data"`
	}
	if err = json.Unmarshal(firstRecorder.Body.Bytes(), &first); err != nil {
		t.Fatalf("解析首次创建响应失败: %v", err)
	}
	if err = json.Unmarshal(secondRecorder.Body.Bytes(), &second); err != nil {
		t.Fatalf("解析重放创建响应失败: %v", err)
	}
	if first.Data.JobID == "" || second.Data.JobID != first.Data.JobID {
		t.Fatalf("相同创建意图产生了重复任务: first=%q second=%q", first.Data.JobID, second.Data.JobID)
	}
	receiptRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		"/nexus/v1/capability/scheduled/tasks/create-requests/page-create-intent-01",
		nil,
	)
	if receiptRecorder.Code != http.StatusOK {
		t.Fatalf("读取创建回执失败: got=%d body=%s", receiptRecorder.Code, receiptRecorder.Body.String())
	}
	var receipt struct {
		Data automationdomain.ScheduledTaskCreateRequestStatus `json:"data"`
	}
	if err = json.Unmarshal(receiptRecorder.Body.Bytes(), &receipt); err != nil {
		t.Fatalf("解析创建回执失败: %v", err)
	}
	if receipt.Data.Status != automationdomain.TaskCreateRequestStatusCommitted ||
		receipt.Data.Task == nil || receipt.Data.Task.JobID != first.Data.JobID {
		t.Fatalf("创建回执与任务不一致: %+v", receipt.Data)
	}
	missingReceiptRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		"/nexus/v1/capability/scheduled/tasks/create-requests/page-create-never-accepted",
		nil,
	)
	var missingReceipt struct {
		Data automationdomain.ScheduledTaskCreateRequestStatus `json:"data"`
	}
	if err = json.Unmarshal(missingReceiptRecorder.Body.Bytes(), &missingReceipt); err != nil {
		t.Fatalf("解析未受理创建回执失败: %v", err)
	}
	if missingReceiptRecorder.Code != http.StatusOK ||
		missingReceipt.Data.Status != automationdomain.TaskCreateRequestStatusNotFound {
		t.Fatalf("未受理创建回执不正确: got=%d data=%+v", missingReceiptRecorder.Code, missingReceipt.Data)
	}

	conflictRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPost,
		"/nexus/v1/capability/scheduled/tasks",
		[]byte(`{
			"request_id":"page-create-intent-01","name":"不同页面任务","agent_id":"nexus",
			"schedule":{"kind":"every","interval_seconds":3600,"timezone":"UTC"},
			"session_target":{"kind":"isolated"},"delivery":{"mode":"none"},
			"instruction":"同一标识不得换意图","enabled":false
		}`),
	)
	assertAutomationHTTPFailure(
		t,
		conflictRecorder,
		http.StatusConflict,
		"automation.task_create_conflict",
		protocol.FailureEffectNotApplied,
	)

	legacyPayload := []byte(`{
		"name":"旧页面调用","agent_id":"nexus",
		"schedule":{"kind":"every","interval_seconds":3600,"timezone":"UTC"},
		"session_target":{"kind":"isolated"},"delivery":{"mode":"none"},
		"instruction":"不携带创建标识","enabled":false
	}`)
	legacyFirstRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPost,
		"/nexus/v1/capability/scheduled/tasks",
		legacyPayload,
	)
	legacySecondRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPost,
		"/nexus/v1/capability/scheduled/tasks",
		legacyPayload,
	)
	var legacyFirst, legacySecond struct {
		Data automationdomain.ScheduledTask `json:"data"`
	}
	if err = json.Unmarshal(legacyFirstRecorder.Body.Bytes(), &legacyFirst); err != nil {
		t.Fatalf("解析旧调用首次响应失败: %v", err)
	}
	if err = json.Unmarshal(legacySecondRecorder.Body.Bytes(), &legacySecond); err != nil {
		t.Fatalf("解析旧调用再次响应失败: %v", err)
	}
	if legacyFirst.Data.JobID == "" || legacySecond.Data.JobID == "" ||
		legacyFirst.Data.JobID == legacySecond.Data.JobID {
		t.Fatalf("不携带 request_id 的旧调用语义被改变: first=%q second=%q", legacyFirst.Data.JobID, legacySecond.Data.JobID)
	}
}

func TestScheduledTaskStatusHTTPUsesOptionalConfigurationVersion(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name":"版本栅栏任务","agent_id":"nexus",
		"schedule":{"kind":"every","interval_seconds":3600,"timezone":"UTC"},
		"session_target":{"kind":"isolated"},"delivery":{"mode":"none"},
		"instruction":"验证启停状态","enabled":false
	}`))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("创建任务失败: got=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Data automationdomain.ScheduledTask `json:"data"`
	}
	if err = json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	if created.Data.JobID == "" || created.Data.ConfigurationVersion < 1 || created.Data.Enabled {
		t.Fatalf("创建响应缺少初始版本事实: %+v", created.Data)
	}

	statusPath := fmt.Sprintf(
		"/nexus/v1/capability/scheduled/tasks/%s/status",
		created.Data.JobID,
	)
	missingEnabledRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPatch,
		statusPath,
		[]byte(`{}`),
	)
	assertAutomationHTTPFailure(
		t,
		missingEnabledRecorder,
		http.StatusBadRequest,
		"automation.task_status_invalid",
		protocol.FailureEffectNotApplied,
	)
	afterMissingRecorder := serveAutomationJSON(t, server, http.MethodGet, statusPath, nil)
	if afterMissingRecorder.Code != http.StatusOK {
		t.Fatalf("读取缺少 enabled 后的任务失败: got=%d body=%s", afterMissingRecorder.Code, afterMissingRecorder.Body.String())
	}
	var afterMissing struct {
		Data struct {
			Job automationdomain.ScheduledTask `json:"job"`
		} `json:"data"`
	}
	if err = json.Unmarshal(afterMissingRecorder.Body.Bytes(), &afterMissing); err != nil {
		t.Fatalf("解析缺少 enabled 后的任务失败: %v", err)
	}
	if afterMissing.Data.Job.Enabled ||
		afterMissing.Data.Job.ConfigurationVersion != created.Data.ConfigurationVersion {
		t.Fatalf("缺少 enabled 不得隐式暂停或推进版本: before=%+v after=%+v", created.Data, afterMissing.Data.Job)
	}

	enableRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPatch,
		statusPath,
		[]byte(fmt.Sprintf(
			`{"enabled":true,"expected_configuration_version":%d}`,
			created.Data.ConfigurationVersion,
		)),
	)
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("按版本启用任务失败: got=%d body=%s", enableRecorder.Code, enableRecorder.Body.String())
	}
	var enabled struct {
		Data automationdomain.ScheduledTask `json:"data"`
	}
	if err = json.Unmarshal(enableRecorder.Body.Bytes(), &enabled); err != nil {
		t.Fatalf("解析启用响应失败: %v", err)
	}
	if !enabled.Data.Enabled || enabled.Data.ConfigurationVersion <= created.Data.ConfigurationVersion {
		t.Fatalf("启用未推进任务版本: before=%+v after=%+v", created.Data, enabled.Data)
	}

	staleRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPatch,
		statusPath,
		[]byte(fmt.Sprintf(
			`{"enabled":false,"expected_configuration_version":%d}`,
			created.Data.ConfigurationVersion,
		)),
	)
	if staleRecorder.Code != http.StatusConflict {
		t.Fatalf("过期版本应被拒绝: got=%d body=%s", staleRecorder.Code, staleRecorder.Body.String())
	}
	var stale struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err = json.Unmarshal(staleRecorder.Body.Bytes(), &stale); err != nil {
		t.Fatalf("解析过期版本响应失败: %v", err)
	}
	if stale.Data.Failure.Code != "automation.configuration_conflict" ||
		stale.Data.Failure.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf("过期版本 FailureCore 不正确: %+v", stale.Data.Failure)
	}

	getRecorder := serveAutomationJSON(t, server, http.MethodGet, statusPath, nil)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("对账任务状态失败: got=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	var current struct {
		Data struct {
			Job automationdomain.ScheduledTask `json:"job"`
		} `json:"data"`
	}
	if err = json.Unmarshal(getRecorder.Body.Bytes(), &current); err != nil {
		t.Fatalf("解析对账响应失败: %v", err)
	}
	if !current.Data.Job.Enabled || current.Data.Job.ConfigurationVersion != enabled.Data.ConfigurationVersion {
		t.Fatalf("过期请求不应改写已保存状态: enabled=%+v current=%+v", enabled.Data, current.Data.Job)
	}

	legacyRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPatch,
		statusPath,
		[]byte(`{"enabled":false}`),
	)
	if legacyRecorder.Code != http.StatusOK {
		t.Fatalf("旧调用方不携版本仍应兼容: got=%d body=%s", legacyRecorder.Code, legacyRecorder.Body.String())
	}
	var legacy struct {
		Data automationdomain.ScheduledTask `json:"data"`
	}
	if err = json.Unmarshal(legacyRecorder.Body.Bytes(), &legacy); err != nil {
		t.Fatalf("解析兼容更新响应失败: %v", err)
	}
	if legacy.Data.Enabled || legacy.Data.ConfigurationVersion <= enabled.Data.ConfigurationVersion {
		t.Fatalf("旧启停调用没有正常生效: %+v", legacy.Data)
	}

	deletePath := fmt.Sprintf(
		"/nexus/v1/capability/scheduled/tasks/%s",
		created.Data.JobID,
	)
	staleDeleteRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodDelete,
		deletePath,
		[]byte(fmt.Sprintf(
			`{"expected_configuration_version":%d}`,
			enabled.Data.ConfigurationVersion,
		)),
	)
	assertAutomationHTTPFailure(
		t,
		staleDeleteRecorder,
		http.StatusConflict,
		"automation.delete_configuration_conflict",
		protocol.FailureEffectNotApplied,
	)
	currentDeleteRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodDelete,
		deletePath,
		[]byte(fmt.Sprintf(
			`{"expected_configuration_version":%d}`,
			legacy.Data.ConfigurationVersion,
		)),
	)
	if currentDeleteRecorder.Code != http.StatusOK {
		t.Fatalf("按当前版本删除任务失败: got=%d body=%s", currentDeleteRecorder.Code, currentDeleteRecorder.Body.String())
	}
}

func TestScheduledTaskDeletionStopConfirmationHTTPIsVersionedAndTokenPrivate(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	createRecorder := serveAutomationJSON(t, server, http.MethodPost, "/nexus/v1/capability/scheduled/tasks", []byte(`{
		"name":"等待停止确认的任务","agent_id":"nexus",
		"schedule":{"kind":"every","interval_seconds":3600,"timezone":"UTC"},
		"session_target":{"kind":"isolated"},"delivery":{"mode":"none"},
		"instruction":"验证停止确认收口","enabled":false
	}`))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("创建任务失败: got=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Data automationdomain.ScheduledTask `json:"data"`
	}
	if err = json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil || created.Data.JobID == "" {
		t.Fatalf("解析创建响应失败: err=%v body=%s", err, createRecorder.Body.String())
	}

	const privateToken = "server-private-deletion-token"
	const runID = "review-confirm-http-run"
	const requestID = "review-confirm-http-request"
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()
	if _, err = db.Exec(`UPDATE automation_scheduled_tasks
SET enabled = 0, next_run_at = NULL, deletion_state = 'review_required',
    deletion_token = ?, deletion_claimed_at = CURRENT_TIMESTAMP,
    configuration_version = configuration_version + 1
WHERE job_id = ?`, privateToken, created.Data.JobID); err != nil {
		t.Fatalf("准备 review_required 任务失败: %v", err)
	}
	var ownerUserID string
	var reviewVersion int64
	if err = db.QueryRow(`SELECT owner_user_id, configuration_version
FROM automation_scheduled_tasks WHERE job_id = ?`, created.Data.JobID).Scan(&ownerUserID, &reviewVersion); err != nil {
		t.Fatalf("读取 review 快照失败: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO automation_task_runs (
run_id, job_id, owner_user_id, status, trigger_kind, delivery_status, attempts,
block_state, blocked_request_id, effect_started
) VALUES (?, ?, ?, 'running', 'scheduled', 'pending', 1, 'awaiting_approval', ?, 1)`,
		runID, created.Data.JobID, ownerUserID, requestID); err != nil {
		t.Fatalf("准备活动 run 失败: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO automation_permission_requests (
request_id, owner_user_id, job_id, run_id, policy_revision, kind, status,
tool_name, effect, input_fingerprint, capability_json, input_summary_json
) VALUES (?, ?, ?, ?, 1, 'tool', 'pending', 'WebSearch', 'read',
          'review-confirm-http', '{}', '{}')`,
		requestID, ownerUserID, created.Data.JobID, runID); err != nil {
		t.Fatalf("准备待确认权限失败: %v", err)
	}

	legacyPath := fmt.Sprintf(
		"/nexus/v1/scheduled/tasks/%s/deletion/confirm-stopped",
		created.Data.JobID,
	)
	missingVersionRecorder := serveAutomationJSON(t, server, http.MethodPost, legacyPath, []byte(`{}`))
	assertAutomationHTTPFailure(
		t,
		missingVersionRecorder,
		http.StatusBadRequest,
		"automation.deletion_confirmation_invalid",
		protocol.FailureEffectNotApplied,
	)

	capabilityPath := fmt.Sprintf(
		"/nexus/v1/capability/scheduled/tasks/%s/deletion/confirm-stopped",
		created.Data.JobID,
	)
	staleRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPost,
		capabilityPath,
		[]byte(fmt.Sprintf(`{"expected_configuration_version":%d}`, reviewVersion-1)),
	)
	assertAutomationHTTPFailure(
		t,
		staleRecorder,
		http.StatusConflict,
		"automation.deletion_confirmation_conflict",
		protocol.FailureEffectNotApplied,
	)

	confirmRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodPost,
		capabilityPath,
		[]byte(fmt.Sprintf(`{"expected_configuration_version":%d}`, reviewVersion)),
	)
	if confirmRecorder.Code != http.StatusOK {
		t.Fatalf("停止确认收口失败: got=%d body=%s", confirmRecorder.Code, confirmRecorder.Body.String())
	}
	if strings.Contains(confirmRecorder.Body.String(), privateToken) {
		t.Fatalf("HTTP 响应泄漏私有 deletion token: %s", confirmRecorder.Body.String())
	}
	var confirmed struct {
		Data automationdomain.DeleteJobResult `json:"data"`
	}
	if err = json.Unmarshal(confirmRecorder.Body.Bytes(), &confirmed); err != nil {
		t.Fatalf("解析停止确认响应失败: %v", err)
	}
	if !confirmed.Data.Deleted || !confirmed.Data.CancelledActiveRun ||
		confirmed.Data.JobID != created.Data.JobID || confirmed.Data.CancelledRunID != runID {
		t.Fatalf("停止确认响应不完整: %+v", confirmed.Data)
	}

	var taskCount int
	if err = db.QueryRow(`SELECT COUNT(1) FROM automation_scheduled_tasks WHERE job_id = ?`, created.Data.JobID).Scan(&taskCount); err != nil || taskCount != 0 {
		t.Fatalf("任务未删除: count=%d err=%v", taskCount, err)
	}
	var runStatus, deliveryStatus, blockState, permissionStatus string
	var deliveryDeadLetterAt sql.NullTime
	if err = db.QueryRow(`SELECT status, delivery_status, block_state, delivery_dead_letter_at
FROM automation_task_runs WHERE run_id = ?`, runID).Scan(
		&runStatus, &deliveryStatus, &blockState, &deliveryDeadLetterAt,
	); err != nil {
		t.Fatalf("读取收口 run 失败: %v", err)
	}
	if err = db.QueryRow(`SELECT status FROM automation_permission_requests WHERE request_id = ?`, requestID).Scan(&permissionStatus); err != nil {
		t.Fatalf("读取收口权限失败: %v", err)
	}
	if runStatus != automationdomain.RunStatusCancelled ||
		deliveryStatus != automationdomain.DeliveryStatusNotAttempted ||
		blockState != "" || !deliveryDeadLetterAt.Valid ||
		permissionStatus != automationdomain.PermissionRequestStatusSuperseded {
		t.Fatalf("停止确认没有原子收口: run=%q delivery=%q block=%q deadletter=%v permission=%q",
			runStatus, deliveryStatus, blockState, deliveryDeadLetterAt, permissionStatus)
	}
	var detailJSON string
	if err = db.QueryRow(`SELECT detail_json FROM automation_task_events
WHERE job_id = ? AND action = 'delete' ORDER BY created_at DESC, event_id DESC LIMIT 1`,
		created.Data.JobID).Scan(&detailJSON); err != nil {
		t.Fatalf("读取删除审计失败: %v", err)
	}
	for _, fact := range []string{
		`"execution_stop_confirmed_by_owner":true`,
		`"review_required_finalized":true`,
		`"external_actions_replayed":false`,
	} {
		if !strings.Contains(detailJSON, fact) {
			t.Fatalf("删除审计缺少 %s: %s", fact, detailJSON)
		}
	}
}

func TestScheduledTaskReadFailuresExposeNotApplicableEffect(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	if _, err = db.Exec(`DROP TABLE automation_permission_requests`); err != nil {
		t.Fatalf("准备权限列表读失败场景失败: %v", err)
	}
	permissionRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		"/nexus/v1/capability/scheduled/permission-requests",
		nil,
	)
	assertAutomationHTTPFailure(
		t,
		permissionRecorder,
		http.StatusInternalServerError,
		"automation.permission_list_unavailable",
		protocol.FailureEffectNotApplicable,
	)

	if _, err = db.Exec(`DROP TABLE automation_scheduled_tasks`); err != nil {
		t.Fatalf("准备任务列表读失败场景失败: %v", err)
	}
	taskRecorder := serveAutomationJSON(
		t,
		server,
		http.MethodGet,
		"/nexus/v1/capability/scheduled/tasks",
		nil,
	)
	assertAutomationHTTPFailure(
		t,
		taskRecorder,
		http.StatusInternalServerError,
		"automation.task_list_unavailable",
		protocol.FailureEffectNotApplicable,
	)
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
		!containsString(report.Data.Tasks[0].SuggestedTools, "nexus automation apply") ||
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

func assertAutomationHTTPFailure(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	status int,
	code string,
	effect protocol.FailureEffect,
) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("失败状态码不正确: got=%d want=%d body=%s", recorder.Code, status, recorder.Body.String())
	}
	var response struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Success bool   `json:"success"`
		Data    struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析失败响应失败: %v", err)
	}
	if response.Code != fmt.Sprint(status) || response.Message != "failed" || response.Success {
		t.Fatalf("旧 HTTP envelope 被改变: %+v", response)
	}
	if response.Data.Failure.Code != code || response.Data.Failure.Effect != effect {
		t.Fatalf("FailureCore 不正确: got=%+v want_code=%q want_effect=%q", response.Data.Failure, code, effect)
	}
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
