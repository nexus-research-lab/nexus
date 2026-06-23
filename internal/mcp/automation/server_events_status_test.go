package automationmcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
