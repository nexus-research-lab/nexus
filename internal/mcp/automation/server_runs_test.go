package automationmcp

import (
	"encoding/json"
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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
		jobs: []automationdomain.CronJob{{
			JobID:       "job-water",
			Name:        "喝水提醒",
			AgentID:     "agent-1",
			Instruction: "提醒我喝水",
			Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
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

func TestGetScheduledTaskRunsAllowsDeletedOwnedTaskHistory(t *testing.T) {
	svc := &stubService{
		missingJobs: map[string]bool{"job-deleted": true},
		eventsByJob: map[string][]automationdomain.CronTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-1",
					Action:  automationdomain.TaskEventActionDelete,
				},
			},
		},
		runsByJob: map[string][]automationdomain.CronRun{
			"job-deleted": {{RunID: "run-before-delete", JobID: "job-deleted", Status: automationdomain.RunStatusSucceeded}},
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
		jobs: []automationdomain.CronJob{
			{
				JobID:       "job-current-group-news",
				Name:        "本群每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
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
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
		},
		runsByJob: map[string][]automationdomain.CronRun{
			"job-current-group-news": {{RunID: "run-current-group", JobID: "job-current-group-news", Status: automationdomain.RunStatusSucceeded}},
			"job-other-group-news":   {{RunID: "run-other-group", JobID: "job-other-group-news", Status: automationdomain.RunStatusSucceeded}},
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
		eventsByJob: map[string][]automationdomain.CronTaskEvent{
			"job-deleted": {
				{
					EventID: "evt-delete",
					JobID:   "job-deleted",
					AgentID: "agent-2",
					Action:  automationdomain.TaskEventActionDelete,
				},
			},
		},
		runsByJob: map[string][]automationdomain.CronRun{
			"job-deleted": {{RunID: "run-before-delete", JobID: "job-deleted", Status: automationdomain.RunStatusSucceeded}},
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
		jobs: []automationdomain.CronJob{{
			JobID:        "job-1",
			AgentID:      "agent-1",
			RunningRunID: "run-1",
			Schedule:     automationdomain.Schedule{Timezone: "Asia/Shanghai"},
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
		jobs: []automationdomain.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
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
		jobs: []automationdomain.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
		taskStatus: &automationdomain.CronTaskStatus{
			Job: automationdomain.CronJob{
				JobID:    "job-1",
				AgentID:  "agent-1",
				Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
			},
			Health: automationdomain.CronTaskHealth{
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
		jobs: []automationdomain.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
		taskStatus: &automationdomain.CronTaskStatus{
			Job: automationdomain.CronJob{
				JobID:    "job-1",
				AgentID:  "agent-1",
				Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
			},
			Health: automationdomain.CronTaskHealth{
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
