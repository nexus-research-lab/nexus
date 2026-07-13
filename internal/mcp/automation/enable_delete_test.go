package automationmcp

import (
	"encoding/json"
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestEnableScheduledTaskCanResumeDisabledTaskByQuery(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-news",
				Name:        "暂停的每日新闻摘要",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     false,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group",
				},
			},
			{
				JobID:       "job-feishu-weather",
				Name:        "飞书群天气",
				AgentID:     "agent-1",
				Instruction: "发送天气",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: automationdomain.DeliveryTarget{
					Mode:    automationdomain.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group",
				},
			},
			{
				JobID:       "job-disabled-water",
				Name:        "暂停的喝水提醒",
				AgentID:     "agent-1",
				Instruction: "提醒我喝水",
				Enabled:     false,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
			},
			{
				JobID:       "job-water",
				Name:        "喝水提醒",
				AgentID:     "agent-1",
				Instruction: "提醒我喝水",
				Enabled:     true,
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "update_scheduled_task", map[string]any{
		"enabled": true,
		"query":   "飞书群暂停新闻",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-news" || !svc.statusEnabled {
		t.Fatalf("enable by query should target job-news with enabled=true, job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}
	if svc.recoverJobID != "" {
		t.Fatalf("enable must not recover a running run, got recover job=%q run=%q", svc.recoverJobID, svc.recoverRunID)
	}
}

func TestRegularAgentCannotEnableAnotherAgentsTask(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-2",
			Enabled:  false,
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "update_scheduled_task", map[string]any{
		"enabled": true,
		"job_id":  "job-1",
	})
	if !isError {
		t.Fatalf("expected ownership error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "another agent") {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.statusJobID != "" {
		t.Fatalf("status update should not be called for another agent task, got %q", svc.statusJobID)
	}
}

func TestDeleteRequiresJobID(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, contract.ServerContext{IsMainAgent: true}, "delete_scheduled_task", map[string]any{})
	if !isError {
		t.Fatalf("expected error, got %+v", result)
	}
}

func TestDeleteScheduledTaskPassesJobID(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "delete_scheduled_task", map[string]any{
		"job_id": "job-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.deletedJobID != "job-1" {
		t.Fatalf("expected deleted job_id=job-1, got %q", svc.deletedJobID)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("delete response 不是 JSON: %v", err)
	}
	if decoded["job_id"] != "job-1" || decoded["deleted"] != true {
		t.Fatalf("delete response 不正确: %+v", decoded)
	}
}

func TestDeleteScheduledTaskReportsCancelledActiveRun(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:        "job-1",
			AgentID:      "agent-1",
			RunningRunID: "run-active",
			Schedule:     automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "delete_scheduled_task", map[string]any{
		"job_id": "job-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("delete response 不是 JSON: %v", err)
	}
	if decoded["active_run_id"] != "run-active" ||
		decoded["cancelled_run_id"] != "run-active" ||
		decoded["cancelled_active_run"] != true {
		t.Fatalf("delete response should report active run cancellation: %+v", decoded)
	}
}

func TestDeleteScheduledTaskQueryRequiresUniqueMatch(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-news-a",
				Name:        "早间新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻",
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
			},
			{
				JobID:       "job-news-b",
				Name:        "晚间新闻",
				AgentID:     "agent-1",
				Instruction: "整理新闻",
				Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "delete_scheduled_task", map[string]any{
		"query": "新闻",
	})
	if !isError {
		t.Fatalf("expected ambiguous query error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "matched multiple current scheduled tasks") {
		t.Fatalf("unexpected ambiguity error: %s", extractText(t, result))
	}
	if svc.deletedJobID != "" {
		t.Fatalf("delete should not run for ambiguous query, got %q", svc.deletedJobID)
	}
}
