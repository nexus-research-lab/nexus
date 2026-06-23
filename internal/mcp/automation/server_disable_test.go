package automationmcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDisableScheduledTaskKeepsTaskAndPassesStatus(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Enabled:  true,
			Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "disable_scheduled_task", map[string]any{
		"job_id": "job-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-1" || svc.statusEnabled {
		t.Fatalf("disable should pass enabled=false for job-1, got job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}
	if svc.deletedJobID != "" {
		t.Fatalf("disable must not delete task, deleted=%q", svc.deletedJobID)
	}
}

func TestDisableScheduledTaskReportsPreservedActiveRun(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:        "job-1",
			AgentID:      "agent-1",
			Enabled:      true,
			Running:      true,
			RunningRunID: "run-active",
			Schedule:     protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "disable_scheduled_task", map[string]any{
		"job_id": "job-1",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("disable response 不是 JSON: %v", err)
	}
	if decoded["enabled"] != false || decoded["running_run_id"] != "run-active" {
		t.Fatalf("disable response should preserve active run: %+v", decoded)
	}
}

func TestDisableScheduledTaskCanCancelActiveRun(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:        "job-1",
			AgentID:      "agent-1",
			Enabled:      true,
			Running:      true,
			RunningRunID: "run-active",
			Schedule:     protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "disable_scheduled_task", map[string]any{
		"job_id":            "job-1",
		"cancel_active_run": true,
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-1" || svc.statusEnabled {
		t.Fatalf("disable should run before cancellation, job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}
	if svc.recoverJobID != "job-1" || svc.recoverRunID != "run-active" {
		t.Fatalf("disable cancel_active_run should recover active run, job=%q run=%q", svc.recoverJobID, svc.recoverRunID)
	}
}

func TestDisableScheduledTaskCanResolveUniqueQuery(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-feishu",
				Name:        "每日新闻摘要",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group",
				},
			},
			{
				JobID:       "job-water",
				Name:        "喝水提醒",
				AgentID:     "agent-1",
				Instruction: "提醒我喝水",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "disable_scheduled_task", map[string]any{
		"query": "飞书群",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-feishu" || svc.statusEnabled {
		t.Fatalf("disable by query should target job-feishu, job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}
}

func TestDisableScheduledTaskCanResolveCurrentExternalGroupQuery(t *testing.T) {
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
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "disable_scheduled_task", map[string]any{
		"query": "这个群的新闻任务",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-current-group-news" || svc.statusEnabled {
		t.Fatalf("current group query should target current group news task, job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}

	svc.statusJobID = ""
	result, isError = callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "disable_scheduled_task", map[string]any{
		"query": "每日新闻",
	})
	if isError {
		t.Fatalf("unexpected error without explicit current group terms: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-current-group-news" || svc.statusEnabled {
		t.Fatalf("external group query should prefer current group task, job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}
}

func TestDisableScheduledTaskCanResolveCurrentInternalConversationQuery(t *testing.T) {
	currentSessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "operator", "")
	otherSessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "other", "")
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-dm-news",
				Name:        "每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source: protocol.Source{
					SessionKey: currentSessionKey,
				},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelInternalSegment,
					To:      currentSessionKey,
				},
			},
			{
				JobID:       "job-other-dm-news",
				Name:        "每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source: protocol.Source{
					SessionKey: otherSessionKey,
				},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelInternalSegment,
					To:      otherSessionKey,
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
	}, "disable_scheduled_task", map[string]any{
		"query": "当前会话的新闻任务",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-current-dm-news" || svc.statusEnabled {
		t.Fatalf("current conversation query should target current dm task, job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}

	svc.statusJobID = ""
	result, isError = callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
	}, "disable_scheduled_task", map[string]any{
		"query": "每日新闻",
	})
	if isError {
		t.Fatalf("unexpected error without explicit current conversation terms: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-current-dm-news" || svc.statusEnabled {
		t.Fatalf("internal conversation query should prefer current conversation task, job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}

	svc.statusJobID = ""
	result, isError = callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
	}, "disable_scheduled_task", map[string]any{
		"query": "这个任务",
	})
	if isError {
		t.Fatalf("unexpected error for current task shorthand: %s", extractText(t, result))
	}
	if svc.statusJobID != "job-current-dm-news" || svc.statusEnabled {
		t.Fatalf("current task shorthand should target current conversation task, job=%q enabled=%v", svc.statusJobID, svc.statusEnabled)
	}
}

func TestRegularAgentCannotDisableAnotherAgentsTask(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{
			JobID:    "job-1",
			AgentID:  "agent-2",
			Enabled:  true,
			Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "disable_scheduled_task", map[string]any{
		"job_id": "job-1",
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
