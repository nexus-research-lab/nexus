package automationmcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
)

func TestRunNowReturnsStatus(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, contract.ServerContext{IsMainAgent: true}, "run_scheduled_task", map[string]any{"job_id": "job-1"})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	payload := extractText(t, result)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if decoded["status"] != "succeeded" {
		t.Fatalf("expected status=succeeded, got %v", decoded["status"])
	}
}

func TestRunNowQueryNoMatchDoesNotRun(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:       "job-water",
			Name:        "喝水提醒",
			AgentID:     "agent-1",
			Instruction: "提醒我喝水",
			Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "run_scheduled_task", map[string]any{
		"query": "新闻",
	})
	if !isError {
		t.Fatalf("expected no-match query error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "no current scheduled task matched query") {
		t.Fatalf("unexpected no-match error: %s", extractText(t, result))
	}
	if svc.runNowJobID != "" {
		t.Fatalf("run should not start for no-match query, got %q", svc.runNowJobID)
	}
}

func TestDailyReportUsesServiceObservability(t *testing.T) {
	deliveryError := "feishu send failed"
	executionError := "WebSearch permission denied"
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
		dailyReport: &protocol.CronDailyReport{
			Date:     "2026-05-21",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			Totals: protocol.CronDailyReportTotals{
				RunCount:                  3,
				DeliveredRunCount:         1,
				DeliveryFailedRunCount:    1,
				DeliverySkippedRunCount:   1,
				DeliveryNotNeededCount:    0,
				DeliveryNotAttemptedCount: 0,
			},
			Tasks: []protocol.CronDailyReportTask{
				{
					JobID:                    "job-1",
					Name:                     "新闻日报",
					Signals:                  []string{"delivery_attention"},
					SuggestedTools:           []string{"retry_scheduled_task_delivery", "update_scheduled_task", "run_scheduled_task"},
					LatestExecutionError:     &executionError,
					LatestDeliveryError:      &deliveryError,
					ExecutionFailedRunIDs:    []string{"run-exec-failed"},
					ManualRedeliveryRunIDs:   []string{"run-failed"},
					DeliveryPendingRunIDs:    []string{"run-pending"},
					DeliverySkippedRunIDs:    []string{"run-skipped"},
					DeliveryDeadLetterRunIDs: []string{"run-dead"},
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_daily_report", map[string]any{
		"date":     "2026-05-21",
		"timezone": "Asia/Shanghai",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.listAgentID != "agent-1" {
		t.Fatalf("普通 agent 应只查询自己的任务，实际 agent_id=%q", svc.listAgentID)
	}
	if svc.dailyInput.Date != "2026-05-21" || svc.dailyInput.Timezone != "Asia/Shanghai" || svc.dailyInput.AgentID != "agent-1" {
		t.Fatalf("日报查询入参不正确: %+v", svc.dailyInput)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("daily report 不是 JSON: %v", err)
	}
	totals, ok := decoded["totals"].(map[string]any)
	if !ok {
		t.Fatalf("missing totals: %+v", decoded)
	}
	if totals["run_count"] != float64(3) ||
		totals["delivered_run_count"] != float64(1) ||
		totals["delivery_failed_run_count"] != float64(1) ||
		totals["delivery_skipped_run_count"] != float64(1) ||
		totals["delivery_not_needed_count"] != float64(0) {
		t.Fatalf("daily report totals 不正确: %+v", totals)
	}
	tasks, ok := decoded["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("missing tasks: %+v", decoded)
	}
	task, ok := tasks[0].(map[string]any)
	if !ok {
		t.Fatalf("daily report task 不是 object: %+v", tasks[0])
	}
	if firstString(task["signals"]) != "delivery_attention" ||
		!stringSliceContains(task["suggested_tools"], "retry_scheduled_task_delivery") ||
		!stringSliceContains(task["suggested_tools"], "update_scheduled_task") ||
		!stringSliceContains(task["suggested_tools"], "run_scheduled_task") ||
		task["latest_execution_error"] != "WebSearch permission denied" ||
		task["latest_delivery_error"] != "feishu send failed" ||
		firstString(task["execution_failed_run_ids"]) != "run-exec-failed" ||
		firstString(task["manual_redelivery_run_ids"]) != "run-failed" ||
		firstString(task["delivery_pending_run_ids"]) != "run-pending" ||
		firstString(task["delivery_skipped_run_ids"]) != "run-skipped" ||
		firstString(task["delivery_dead_letter_run_ids"]) != "run-dead" {
		t.Fatalf("daily report should expose actionable fields to agent: %+v", task)
	}
}

func TestDailyReportAllowsDeletedOwnedTaskHistory(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{"job-deleted": true},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDelete,
				},
			},
		},
		runsByJob: map[string][]protocol.CronRun{
			"job-deleted": {{RunID: "run-before-delete", JobID: "job-deleted", Status: protocol.RunStatusSucceeded}},
		},
		dailyReport: &protocol.CronDailyReport{
			Date:     "2026-05-21",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			JobID:    "job-deleted",
			Totals:   protocol.CronDailyReportTotals{TaskCount: 1, RunCount: 1},
			Tasks: []protocol.CronDailyReportTask{{
				JobID:   "job-deleted",
				Name:    "已删日报",
				AgentID: "agent-1",
				Deleted: true,
				Runs:    []protocol.CronRun{{RunID: "run-before-delete", JobID: "job-deleted", Status: protocol.RunStatusSucceeded}},
			}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_daily_report", map[string]any{
		"job_id": "job-deleted",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.dailyInput.JobID != "job-deleted" {
		t.Fatalf("deleted task report should pass job_id through: %+v", svc.dailyInput)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("daily report 不是 JSON: %v", err)
	}
	tasks, ok := decoded["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("missing tasks: %+v", decoded)
	}
	task, ok := tasks[0].(map[string]any)
	if !ok || task["deleted"] != true {
		t.Fatalf("deleted task history should be marked deleted: %+v", tasks[0])
	}
}

func TestDailyReportCanResolveDeletedTaskByQuery(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{"job-deleted": true},
		historyItems: []protocol.CronTaskHistoryItem{
			{
				JobID:   "job-deleted",
				Name:    "旧新闻日报",
				AgentID: "agent-1",
				Deleted: true,
			},
		},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDelete,
				},
			},
		},
		dailyReport: &protocol.CronDailyReport{
			Date:     "2026-05-21",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			JobID:    "job-deleted",
			Totals:   protocol.CronDailyReportTotals{TaskCount: 1},
			Tasks: []protocol.CronDailyReportTask{{
				JobID:   "job-deleted",
				Name:    "旧新闻日报",
				AgentID: "agent-1",
				Deleted: true,
			}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_daily_report", map[string]any{
		"query": "旧新闻",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.historyInput.Query != "旧新闻" || !svc.historyInput.IncludeActive || !svc.historyInput.IncludeDeleted {
		t.Fatalf("daily report query should search task history first: %+v", svc.historyInput)
	}
	if svc.dailyInput.JobID != "job-deleted" {
		t.Fatalf("daily report should resolve query to job_id, got %+v", svc.dailyInput)
	}
}

func TestDailyReportCanResolveCurrentExternalGroupQuery(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-group-news",
				Name:        "本群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_123",
				},
			},
			{
				JobID:       "job-other-group-news",
				Name:        "其他群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		dailyReport: &protocol.CronDailyReport{
			Date:     "2026-05-22",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			JobID:    "job-current-group-news",
			Totals:   protocol.CronDailyReportTotals{TaskCount: 1},
			Tasks: []protocol.CronDailyReportTask{{
				JobID:   "job-current-group-news",
				Name:    "本群每日新闻",
				AgentID: "agent-1",
			}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
		DefaultTimezone:   "Asia/Shanghai",
	}, "get_scheduled_task_daily_report", map[string]any{
		"query": "这个群的新闻任务",
		"date":  "2026-05-22",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.dailyInput.JobID != "job-current-group-news" {
		t.Fatalf("daily report should resolve current group query to job_id, got %+v", svc.dailyInput)
	}
	if svc.historyInput.Query != "" {
		t.Fatalf("active current group task should resolve before history search: %+v", svc.historyInput)
	}
	if !strings.Contains(extractText(t, result), "job-current-group-news") {
		t.Fatalf("current group report missing selected job: %s", extractText(t, result))
	}
}

func TestDailyReportDefaultsToCurrentExternalGroupWithoutQuery(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-group-news",
				Name:        "本群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_123",
				},
			},
			{
				JobID:       "job-other-group-news",
				Name:        "其他群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		dailyReportsByJob: map[string]*protocol.CronDailyReport{
			"job-current-group-news": {
				Date:     "2026-05-22",
				Timezone: "Asia/Shanghai",
				AgentID:  "agent-1",
				JobID:    "job-current-group-news",
				Totals:   protocol.CronDailyReportTotals{TaskCount: 1, RunCount: 1, DeliveredRunCount: 1},
				Tasks: []protocol.CronDailyReportTask{{
					JobID:   "job-current-group-news",
					Name:    "本群每日新闻",
					AgentID: "agent-1",
				}},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
		DefaultTimezone:   "Asia/Shanghai",
	}, "get_scheduled_task_daily_report", map[string]any{
		"date": "2026-05-22",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if len(svc.dailyInputs) != 1 || svc.dailyInputs[0].JobID != "job-current-group-news" {
		t.Fatalf("daily report should default to current group tasks, got %+v", svc.dailyInputs)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "job-current-group-news") || strings.Contains(text, "job-other-group-news") {
		t.Fatalf("current group default report returned wrong tasks: %s", text)
	}
}

func TestDailyReportDefaultsToEmptyCurrentExternalGroup(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-other-group-news",
				Name:        "其他群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		dailyReport: &protocol.CronDailyReport{
			Date:     "2026-05-22",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			Totals:   protocol.CronDailyReportTotals{TaskCount: 1, RunCount: 3},
			Tasks: []protocol.CronDailyReportTask{{
				JobID:   "job-other-group-news",
				Name:    "其他群每日新闻",
				AgentID: "agent-1",
			}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
		DefaultTimezone:   "Asia/Shanghai",
	}, "get_scheduled_task_daily_report", map[string]any{
		"date": "2026-05-22",
	})
	if isError {
		t.Fatalf("unexpected error for empty current group report: %s", extractText(t, result))
	}
	if len(svc.dailyInputs) != 1 || svc.dailyInputs[0].JobID != "" || svc.dailyInputs[0].AgentID != "agent-1" {
		t.Fatalf("empty current group report should use scoped agent report metadata, got %+v", svc.dailyInputs)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	tasks, ok := decoded["tasks"].([]any)
	if !ok || len(tasks) != 0 {
		t.Fatalf("empty current group report should return no tasks, got %+v", decoded["tasks"])
	}
	totals, ok := decoded["totals"].(map[string]any)
	if !ok || totals["task_count"] != float64(0) || totals["run_count"] != float64(0) {
		t.Fatalf("empty current group report should reset totals, got %+v", decoded["totals"])
	}
	if strings.Contains(extractText(t, result), "job-other-group-news") {
		t.Fatalf("empty current group report leaked other group task: %s", extractText(t, result))
	}
}

func TestDailyReportAggregatesCurrentExternalGroupGenericQuery(t *testing.T) {
	sessionKey := "agent:agent-1:fs:group:oc_group_123"
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-delivery",
				Name:        "本群新闻推送",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_123",
				},
			},
			{
				JobID:       "job-current-source",
				Name:        "本群状态检查",
				AgentID:     "agent-1",
				Instruction: "检查状态",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source:      protocol.Source{SessionKey: sessionKey},
			},
			{
				JobID:       "job-other-group",
				Name:        "其他群任务",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		dailyReportsByJob: map[string]*protocol.CronDailyReport{
			"job-current-delivery": {
				Date:     "2026-05-22",
				Timezone: "Asia/Shanghai",
				AgentID:  "agent-1",
				JobID:    "job-current-delivery",
				Totals:   protocol.CronDailyReportTotals{TaskCount: 1, RunCount: 2, DeliveredRunCount: 2},
				Tasks: []protocol.CronDailyReportTask{{
					JobID:   "job-current-delivery",
					Name:    "本群新闻推送",
					AgentID: "agent-1",
				}},
			},
			"job-current-source": {
				Date:     "2026-05-22",
				Timezone: "Asia/Shanghai",
				AgentID:  "agent-1",
				JobID:    "job-current-source",
				Totals:   protocol.CronDailyReportTotals{TaskCount: 1, RunCount: 1, DeliveryFailedRunCount: 1},
				Tasks: []protocol.CronDailyReportTask{{
					JobID:   "job-current-source",
					Name:    "本群状态检查",
					AgentID: "agent-1",
				}},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		DefaultTimezone:   "Asia/Shanghai",
	}, "get_scheduled_task_daily_report", map[string]any{
		"query": "这个群的定时任务发送情况",
		"date":  "2026-05-22",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if len(svc.dailyInputs) != 2 ||
		svc.dailyInputs[0].JobID != "job-current-delivery" ||
		svc.dailyInputs[1].JobID != "job-current-source" {
		t.Fatalf("generic current group report should aggregate current tasks, got %+v", svc.dailyInputs)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	tasks, ok := decoded["tasks"].([]any)
	if !ok || len(tasks) != 2 {
		t.Fatalf("expected two current group tasks, got %+v", decoded)
	}
	totals, ok := decoded["totals"].(map[string]any)
	if !ok ||
		totals["task_count"] != float64(2) ||
		totals["run_count"] != float64(3) ||
		totals["delivered_run_count"] != float64(2) ||
		totals["delivery_failed_run_count"] != float64(1) {
		t.Fatalf("aggregated totals are wrong: %+v", decoded["totals"])
	}
	if strings.Contains(extractText(t, result), "job-other-group") {
		t.Fatalf("current group aggregate should not include other groups: %s", extractText(t, result))
	}
}

func TestDailyReportAggregatesCurrentInternalConversationGenericQuery(t *testing.T) {
	currentSessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "operator", "")
	otherSessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "other", "")
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-source",
				Name:        "当前会话新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source:      protocol.Source{SessionKey: currentSessionKey},
			},
			{
				JobID:       "job-current-delivery",
				Name:        "当前会话告警",
				AgentID:     "agent-1",
				Instruction: "检查状态并通知我",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelInternalSegment,
					To:      currentSessionKey,
				},
			},
			{
				JobID:       "job-other-conversation",
				Name:        "其他会话新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source:      protocol.Source{SessionKey: otherSessionKey},
			},
		},
		dailyReportsByJob: map[string]*protocol.CronDailyReport{
			"job-current-source": {
				Date:     "2026-05-22",
				Timezone: "Asia/Shanghai",
				AgentID:  "agent-1",
				JobID:    "job-current-source",
				Totals:   protocol.CronDailyReportTotals{TaskCount: 1, RunCount: 1, DeliveredRunCount: 1},
				Tasks: []protocol.CronDailyReportTask{{
					JobID:   "job-current-source",
					Name:    "当前会话新闻",
					AgentID: "agent-1",
				}},
			},
			"job-current-delivery": {
				Date:     "2026-05-22",
				Timezone: "Asia/Shanghai",
				AgentID:  "agent-1",
				JobID:    "job-current-delivery",
				Totals:   protocol.CronDailyReportTotals{TaskCount: 1, RunCount: 2, DeliveryFailedRunCount: 1},
				Tasks: []protocol.CronDailyReportTask{{
					JobID:   "job-current-delivery",
					Name:    "当前会话告警",
					AgentID: "agent-1",
				}},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
		DefaultTimezone:   "Asia/Shanghai",
	}, "get_scheduled_task_daily_report", map[string]any{
		"query": "当前会话的定时任务发送情况",
		"date":  "2026-05-22",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if len(svc.dailyInputs) != 2 ||
		svc.dailyInputs[0].JobID != "job-current-source" ||
		svc.dailyInputs[1].JobID != "job-current-delivery" {
		t.Fatalf("generic current conversation report should aggregate current tasks, got %+v", svc.dailyInputs)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	totals, ok := decoded["totals"].(map[string]any)
	if !ok ||
		totals["task_count"] != float64(2) ||
		totals["run_count"] != float64(3) ||
		totals["delivered_run_count"] != float64(1) ||
		totals["delivery_failed_run_count"] != float64(1) {
		t.Fatalf("aggregated totals are wrong: %+v", decoded["totals"])
	}
	if strings.Contains(extractText(t, result), "job-other-conversation") {
		t.Fatalf("current conversation aggregate should not include other conversations: %s", extractText(t, result))
	}
}

func TestDailyReportCurrentInternalConversationGenericQueryCanReturnEmptyReport(t *testing.T) {
	currentSessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "operator", "")
	otherSessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "other", "")
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-other-conversation",
				Name:        "其他会话新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source:      protocol.Source{SessionKey: otherSessionKey},
			},
		},
		dailyReport: &protocol.CronDailyReport{
			Date:     "2026-05-22",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			Totals:   protocol.CronDailyReportTotals{TaskCount: 1, RunCount: 3},
			Tasks: []protocol.CronDailyReportTask{{
				JobID:   "job-other-conversation",
				Name:    "其他会话新闻",
				AgentID: "agent-1",
			}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
		DefaultTimezone:   "Asia/Shanghai",
	}, "get_scheduled_task_daily_report", map[string]any{
		"query": "当前会话的定时任务发送情况",
		"date":  "2026-05-22",
	})
	if isError {
		t.Fatalf("unexpected error for empty current conversation report: %s", extractText(t, result))
	}
	if len(svc.dailyInputs) != 1 || svc.dailyInputs[0].JobID != "" || svc.dailyInputs[0].AgentID != "agent-1" {
		t.Fatalf("empty current conversation report should use scoped agent report metadata, got %+v", svc.dailyInputs)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	tasks, ok := decoded["tasks"].([]any)
	if !ok || len(tasks) != 0 {
		t.Fatalf("empty current conversation report should return no tasks, got %+v", decoded["tasks"])
	}
	totals, ok := decoded["totals"].(map[string]any)
	if !ok || totals["task_count"] != float64(0) || totals["run_count"] != float64(0) {
		t.Fatalf("empty current conversation report should reset totals, got %+v", decoded["totals"])
	}
	if strings.Contains(extractText(t, result), "job-other-conversation") {
		t.Fatalf("empty current conversation report leaked other conversation task: %s", extractText(t, result))
	}
}

func TestDailyReportCanResolveDeletedCurrentExternalGroupQuery(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{
			"job-current-deleted": true,
			"job-other-deleted":   true,
		},
		historyItems: []protocol.CronTaskHistoryItem{
			{
				JobID:   "job-current-deleted",
				Name:    "旧新闻日报",
				AgentID: "agent-1",
				Deleted: true,
			},
			{
				JobID:   "job-other-deleted",
				Name:    "旧新闻日报",
				AgentID: "agent-1",
				Deleted: true,
			},
		},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-current-deleted": {
				{
					EventID: "evt-current-delete",
					JobID:   "job-current-deleted",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDelete,
					Detail: map[string]any{
						"name":             "旧新闻日报",
						"delivery_channel": protocol.SessionChannelFeishu,
						"delivery_to":      "oc_group_123",
					},
				},
			},
			"job-other-deleted": {
				{
					EventID: "evt-other-delete",
					JobID:   "job-other-deleted",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDelete,
					Detail: map[string]any{
						"name":             "旧新闻日报",
						"delivery_channel": protocol.SessionChannelFeishu,
						"delivery_to":      "oc_group_other",
					},
				},
			},
		},
		dailyReport: &protocol.CronDailyReport{
			Date:     "2026-05-22",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			JobID:    "job-current-deleted",
			Totals:   protocol.CronDailyReportTotals{TaskCount: 1},
			Tasks: []protocol.CronDailyReportTask{{
				JobID:   "job-current-deleted",
				Name:    "旧新闻日报",
				AgentID: "agent-1",
				Deleted: true,
			}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
		DefaultTimezone:   "Asia/Shanghai",
	}, "get_scheduled_task_daily_report", map[string]any{
		"query": "这个群的旧新闻任务",
		"date":  "2026-05-22",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if strings.Contains(svc.historyInput.Query, "这个群") {
		t.Fatalf("history search should strip current conversation terms: %+v", svc.historyInput)
	}
	if svc.historyInput.IncludeActive || !svc.historyInput.IncludeDeleted {
		t.Fatalf("deleted current group fallback should only search deleted history: %+v", svc.historyInput)
	}
	if svc.dailyInput.JobID != "job-current-deleted" {
		t.Fatalf("daily report should resolve deleted current group query to job_id, got %+v", svc.dailyInput)
	}
}

func TestGetScheduledTaskRunsAllowsDeletedOwnedTaskHistory(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{"job-deleted": true},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDelete,
				},
			},
		},
		runsByJob: map[string][]protocol.CronRun{
			"job-deleted": {{RunID: "run-before-delete", JobID: "job-deleted", Status: protocol.RunStatusSucceeded}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_runs", map[string]any{
		"job_id": "job-deleted",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if !strings.Contains(extractText(t, result), "run-before-delete") {
		t.Fatalf("deleted task run history missing: %s", extractText(t, result))
	}
}

func TestGetScheduledTaskRunsCanResolveCurrentExternalGroupQuery(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-group-news",
				Name:        "本群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_123",
				},
			},
			{
				JobID:       "job-other-group-news",
				Name:        "其他群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		runsByJob: map[string][]protocol.CronRun{
			"job-current-group-news": {{RunID: "run-current-group", JobID: "job-current-group-news", Status: protocol.RunStatusSucceeded}},
			"job-other-group-news":   {{RunID: "run-other-group", JobID: "job-other-group-news", Status: protocol.RunStatusSucceeded}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "get_scheduled_task_runs", map[string]any{
		"query": "这个群的新闻任务",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	text := extractText(t, result)
	if !strings.Contains(text, "run-current-group") || strings.Contains(text, "run-other-group") {
		t.Fatalf("current group run history mismatch: %s", text)
	}
	if svc.historyInput.Query != "" {
		t.Fatalf("active current group run query should resolve before history search: %+v", svc.historyInput)
	}
}

func TestGetScheduledTaskRunsRejectsDeletedOtherAgentTask(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{"job-deleted": true},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-2",
					Action:  protocol.TaskEventActionDelete,
				},
			},
		},
		runsByJob: map[string][]protocol.CronRun{
			"job-deleted": {{RunID: "run-before-delete", JobID: "job-deleted", Status: protocol.RunStatusSucceeded}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_runs", map[string]any{
		"job_id": "job-deleted",
	})
	if !isError {
		t.Fatalf("expected error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "another agent") {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
}

func TestRecoverScheduledTaskPassesRunID(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:        "job-1",
			AgentID:      "agent-1",
			RunningRunID: "run-1",
			Schedule:     protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "recover_scheduled_task", map[string]any{
		"job_id": "job-1",
		"run_id": "run-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.recoverJobID != "job-1" || svc.recoverRunID != "run-1" {
		t.Fatalf("recover args not passed through: job=%q run=%q", svc.recoverJobID, svc.recoverRunID)
	}
}

func TestRetryScheduledTaskDeliveryPassesRunID(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "retry_scheduled_task_delivery", map[string]any{
		"job_id": "job-1",
		"run_id": "run-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.redeliverJobID != "job-1" || svc.redeliverRunID != "run-1" {
		t.Fatalf("redeliver args not passed through: job=%q run=%q", svc.redeliverJobID, svc.redeliverRunID)
	}
}

func TestRetryScheduledTaskDeliveryCanInferUniqueFailedRunID(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
		taskStatus: &protocol.CronTaskStatus{
			Job: protocol.CronJob{
				JobID:    "job-1",
				AgentID:  "agent-1",
				Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
			},
			Health: protocol.CronTaskHealth{
				ManualRedeliveryAvailable: true,
				ManualRedeliveryRunIDs:    []string{"run-delivery-failed"},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "retry_scheduled_task_delivery", map[string]any{
		"job_id": "job-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.redeliverJobID != "job-1" || svc.redeliverRunID != "run-delivery-failed" {
		t.Fatalf("redeliver should infer unique failed run: job=%q run=%q", svc.redeliverJobID, svc.redeliverRunID)
	}
}

func TestRetryScheduledTaskDeliveryRequiresRunIDWhenMultipleFailedRuns(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
		taskStatus: &protocol.CronTaskStatus{
			Job: protocol.CronJob{
				JobID:    "job-1",
				AgentID:  "agent-1",
				Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
			},
			Health: protocol.CronTaskHealth{
				ManualRedeliveryAvailable: true,
				ManualRedeliveryRunIDs:    []string{"run-failed-1", "run-failed-2"},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "retry_scheduled_task_delivery", map[string]any{
		"job_id": "job-1",
	})
	if !isError {
		t.Fatalf("expected multiple-run error, got %+v", result)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "multiple failed delivery runs") ||
		!strings.Contains(text, "run-failed-1") ||
		!strings.Contains(text, "run-failed-2") {
		t.Fatalf("unexpected multiple-run error: %s", text)
	}
	if svc.redeliverRunID != "" {
		t.Fatalf("redeliver should not run without explicit run_id when multiple candidates exist, got %q", svc.redeliverRunID)
	}
}

func TestGetScheduledTaskEventsReturnsAuditTrail(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-1": {
				{
					EventID: "evt-1",
					JobID:   "job-1",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDisable,
					Detail:  map[string]any{"enabled": false},
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_events", map[string]any{
		"job_id": "job-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if !strings.Contains(extractText(t, result), protocol.TaskEventActionDisable) {
		t.Fatalf("events response missing disable action: %s", extractText(t, result))
	}
}

func TestGetScheduledTaskEventsCanResolveDeletedCurrentExternalGroupQuery(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{
			"job-current-deleted": true,
			"job-other-deleted":   true,
		},
		historyItems: []protocol.CronTaskHistoryItem{
			{
				JobID:   "job-current-deleted",
				Name:    "本群旧新闻",
				AgentID: "agent-1",
				Deleted: true,
			},
			{
				JobID:   "job-other-deleted",
				Name:    "其他群旧新闻",
				AgentID: "agent-1",
				Deleted: true,
			},
		},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-current-deleted": {
				{
					EventID: "evt-current-delete",
					JobID:   "job-current-deleted",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDelete,
					Detail: map[string]any{
						"name":             "本群旧新闻",
						"delivery_channel": protocol.SessionChannelFeishu,
						"delivery_to":      "oc_group_123",
					},
				},
			},
			"job-other-deleted": {
				{
					EventID: "evt-other-delete",
					JobID:   "job-other-deleted",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDelete,
					Detail: map[string]any{
						"name":             "其他群旧新闻",
						"delivery_channel": protocol.SessionChannelFeishu,
						"delivery_to":      "oc_group_other",
					},
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "get_scheduled_task_events", map[string]any{
		"query": "这个群的旧新闻任务",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	text := extractText(t, result)
	if !strings.Contains(text, "evt-current-delete") || strings.Contains(text, "evt-other-delete") {
		t.Fatalf("current group event history mismatch: %s", text)
	}
	if strings.Contains(svc.historyInput.Query, "这个群") || svc.historyInput.IncludeActive || !svc.historyInput.IncludeDeleted {
		t.Fatalf("deleted current group event query should strip current group terms and only search deleted history: %+v", svc.historyInput)
	}
}

func TestGetScheduledTaskEventsAllowsDeletedOwnedTask(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{"job-deleted": true},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-1",
					Action:  protocol.TaskEventActionDelete,
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_events", map[string]any{
		"job_id": "job-deleted",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if !strings.Contains(extractText(t, result), protocol.TaskEventActionDelete) {
		t.Fatalf("events response missing delete action: %s", extractText(t, result))
	}
}

func TestGetScheduledTaskEventsRejectsDeletedOtherAgentTask(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{"job-deleted": true},
		eventsByJob: map[string][]protocol.CronTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-2",
					Action:  protocol.TaskEventActionDelete,
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_events", map[string]any{
		"job_id": "job-deleted",
	})
	if !isError {
		t.Fatalf("expected error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "another agent") {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
}

func TestGetScheduledTaskStatusReturnsHealthRunsAndEvents(t *testing.T) {
	deliveryError := "feishu send message failed"
	deadLetterAt := time.Date(2026, 5, 25, 13, 30, 0, 0, time.UTC)
	eventAt := time.Date(2026, 5, 25, 14, 0, 0, 0, time.UTC)
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:              "job-1",
			Name:               "新闻日报",
			AgentID:            "agent-1",
			Schedule:           protocol.Schedule{Timezone: "Asia/Shanghai"},
			Enabled:            true,
			LastRunStatus:      protocol.RunStatusSucceeded,
			LastDeliveryStatus: protocol.DeliveryStatusFailed,
		}},
		taskStatus: &protocol.CronTaskStatus{
			Job: protocol.CronJob{
				JobID:              "job-1",
				Name:               "新闻日报",
				AgentID:            "agent-1",
				Schedule:           protocol.Schedule{Timezone: "Asia/Shanghai"},
				Enabled:            true,
				LastRunStatus:      protocol.RunStatusSucceeded,
				LastDeliveryStatus: protocol.DeliveryStatusFailed,
			},
			Health: protocol.CronTaskHealth{
				State:                     "attention",
				Signals:                   []string{"delivery_attention"},
				SuggestedTools:            []string{"retry_scheduled_task_delivery"},
				ManualRedeliveryAvailable: true,
				DeliveryFailedRunCount:    1,
				ManualRedeliveryRunIDs:    []string{"run-delivery-failed"},
				DeliveryDeadLetterCount:   1,
				DeliveryDeadLetterRunIDs:  []string{"run-delivery-failed"},
				LatestDeliveryError:       &deliveryError,
			},
			RecentRuns: []protocol.CronRun{
				{
					RunID:                "run-delivery-failed",
					JobID:                "job-1",
					Status:               protocol.RunStatusSucceeded,
					DeliveryStatus:       protocol.DeliveryStatusFailed,
					DeliveryError:        &deliveryError,
					DeliveryDeadLetterAt: &deadLetterAt,
				},
			},
			RecentEvents: []protocol.CronTaskEvent{
				{
					EventID:   "evt-update",
					JobID:     "job-1",
					AgentID:   "agent-1",
					Action:    protocol.TaskEventActionUpdate,
					CreatedAt: eventAt,
					Detail: map[string]any{
						"delivery_dead_letter_at": deadLetterAt,
					},
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_status", map[string]any{
		"job_id": "job-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	text := extractText(t, result)
	for _, want := range []string{"delivery_attention", "retry_scheduled_task_delivery", "run-delivery-failed", deliveryError, protocol.TaskEventActionUpdate} {
		if !strings.Contains(text, want) {
			t.Fatalf("status response missing %q: %s", want, text)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("status response 不是 JSON: %v", err)
	}
	runs := decoded["recent_runs"].([]any)
	run := runs[0].(map[string]any)
	if run["delivery_dead_letter_at_display"] != "2026-05-25 21:30:00 CST" {
		t.Fatalf("recent run time display missing or wrong: %+v", run)
	}
	events := decoded["recent_events"].([]any)
	event := events[0].(map[string]any)
	if event["created_at_display"] != "2026-05-25 22:00:00 CST" {
		t.Fatalf("recent event time display missing or wrong: %+v", event)
	}
	detail := event["detail"].(map[string]any)
	if detail["delivery_dead_letter_at_display"] != "2026-05-25 21:30:00 CST" {
		t.Fatalf("event detail time display missing or wrong: %+v", detail)
	}
}
