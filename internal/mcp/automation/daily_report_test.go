package automationmcp

import (
	"encoding/json"
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDailyReportUsesServiceObservability(t *testing.T) {
	deliveryError := "feishu send failed"
	executionError := "WebSearch permission denied"
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
		dailyReport: &automationdomain.ScheduledTaskDailyReport{
			Date:     "2026-05-21",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			Totals: automationdomain.ScheduledTaskDailyReportTotals{
				RunCount:                  3,
				DeliveredRunCount:         1,
				DeliveryFailedRunCount:    1,
				DeliverySkippedRunCount:   1,
				DeliveryNotNeededCount:    0,
				DeliveryNotAttemptedCount: 0,
			},
			Tasks: []automationdomain.ScheduledTaskDailyReportItem{
				{
					JobID:                    "job-1",
					Name:                     "新闻日报",
					Signals:                  []string{"delivery_attention"},
					SuggestedTools:           []string{"repair_scheduled_task", "update_scheduled_task", "run_scheduled_task"},
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
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_report", map[string]any{
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
		!stringSliceContains(task["suggested_tools"], "repair_scheduled_task") ||
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
		eventsByJob: map[string][]automationdomain.ScheduledTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-1",
					Action:  automationdomain.TaskEventActionDelete,
				},
			},
		},
		runsByJob: map[string][]automationdomain.ScheduledTaskRun{
			"job-deleted": {{RunID: "run-before-delete", JobID: "job-deleted", Status: automationdomain.RunStatusSucceeded}},
		},
		dailyReport: &automationdomain.ScheduledTaskDailyReport{
			Date:     "2026-05-21",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			JobID:    "job-deleted",
			Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1, RunCount: 1},
			Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
				JobID:   "job-deleted",
				Name:    "已删日报",
				AgentID: "agent-1",
				Deleted: true,
				Runs:    []automationdomain.ScheduledTaskRun{{RunID: "run-before-delete", JobID: "job-deleted", Status: automationdomain.RunStatusSucceeded}},
			}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_report", map[string]any{
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
		historyItems: []automationdomain.ScheduledTaskHistoryItem{
			{
				JobID:   "job-deleted",
				Name:    "旧新闻日报",
				AgentID: "agent-1",
				Deleted: true,
			},
		},
		eventsByJob: map[string][]automationdomain.ScheduledTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-1",
					Action:  automationdomain.TaskEventActionDelete,
				},
			},
		},
		dailyReport: &automationdomain.ScheduledTaskDailyReport{
			Date:     "2026-05-21",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			JobID:    "job-deleted",
			Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1},
			Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
				JobID:   "job-deleted",
				Name:    "旧新闻日报",
				AgentID: "agent-1",
				Deleted: true,
			}},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "get_scheduled_task_report", map[string]any{
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
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-current-group-news",
				Name:        "本群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
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
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		dailyReport: &automationdomain.ScheduledTaskDailyReport{
			Date:     "2026-05-22",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			JobID:    "job-current-group-news",
			Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1},
			Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
	}, "get_scheduled_task_report", map[string]any{
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
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-current-group-news",
				Name:        "本群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
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
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		dailyReportsByJob: map[string]*automationdomain.ScheduledTaskDailyReport{
			"job-current-group-news": {
				Date:     "2026-05-22",
				Timezone: "Asia/Shanghai",
				AgentID:  "agent-1",
				JobID:    "job-current-group-news",
				Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1, RunCount: 1, DeliveredRunCount: 1},
				Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
	}, "get_scheduled_task_report", map[string]any{
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
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-other-group-news",
				Name:        "其他群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		dailyReport: &automationdomain.ScheduledTaskDailyReport{
			Date:     "2026-05-22",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1, RunCount: 3},
			Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
	}, "get_scheduled_task_report", map[string]any{
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
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-current-delivery",
				Name:        "本群新闻推送",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
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
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Source:      automationdomain.Source{SessionKey: sessionKey},
			},
			{
				JobID:       "job-other-group",
				Name:        "其他群任务",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		dailyReportsByJob: map[string]*automationdomain.ScheduledTaskDailyReport{
			"job-current-delivery": {
				Date:     "2026-05-22",
				Timezone: "Asia/Shanghai",
				AgentID:  "agent-1",
				JobID:    "job-current-delivery",
				Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1, RunCount: 2, DeliveredRunCount: 2},
				Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
				Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1, RunCount: 1, DeliveryFailedRunCount: 1},
				Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
	}, "get_scheduled_task_report", map[string]any{
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
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-current-source",
				Name:        "当前会话新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Source:      automationdomain.Source{SessionKey: currentSessionKey},
			},
			{
				JobID:       "job-current-delivery",
				Name:        "当前会话告警",
				AgentID:     "agent-1",
				Instruction: "检查状态并通知我",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
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
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Source:      automationdomain.Source{SessionKey: otherSessionKey},
			},
		},
		dailyReportsByJob: map[string]*automationdomain.ScheduledTaskDailyReport{
			"job-current-source": {
				Date:     "2026-05-22",
				Timezone: "Asia/Shanghai",
				AgentID:  "agent-1",
				JobID:    "job-current-source",
				Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1, RunCount: 1, DeliveredRunCount: 1},
				Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
				Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1, RunCount: 2, DeliveryFailedRunCount: 1},
				Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
	}, "get_scheduled_task_report", map[string]any{
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
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-other-conversation",
				Name:        "其他会话新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Source:      automationdomain.Source{SessionKey: otherSessionKey},
			},
		},
		dailyReport: &automationdomain.ScheduledTaskDailyReport{
			Date:     "2026-05-22",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1, RunCount: 3},
			Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
	}, "get_scheduled_task_report", map[string]any{
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
		historyItems: []automationdomain.ScheduledTaskHistoryItem{
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
		eventsByJob: map[string][]automationdomain.ScheduledTaskEvent{
			"job-current-deleted": {
				{
					EventID: "evt-current-delete",
					JobID:   "job-current-deleted",
					AgentID: "agent-1",
					Action:  automationdomain.TaskEventActionDelete,
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
					Action:  automationdomain.TaskEventActionDelete,
					Detail: map[string]any{
						"name":             "旧新闻日报",
						"delivery_channel": protocol.SessionChannelFeishu,
						"delivery_to":      "oc_group_other",
					},
				},
			},
		},
		dailyReport: &automationdomain.ScheduledTaskDailyReport{
			Date:     "2026-05-22",
			Timezone: "Asia/Shanghai",
			AgentID:  "agent-1",
			JobID:    "job-current-deleted",
			Totals:   automationdomain.ScheduledTaskDailyReportTotals{TaskCount: 1},
			Tasks: []automationdomain.ScheduledTaskDailyReportItem{{
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
	}, "get_scheduled_task_report", map[string]any{
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
