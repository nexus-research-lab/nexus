package automationmcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestListPassesAgentID(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{{JobID: "job-1", Schedule: protocol.Schedule{
			Kind: "every", IntervalSeconds: newInterval(300), Timezone: "Asia/Shanghai",
		}}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{IsMainAgent: true}, "list_scheduled_tasks", map[string]any{"agent_id": "agent-1"})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
}

func TestListCanFilterCandidatesByQueryAndEnabled(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-news",
				Name:        "每日新闻摘要",
				AgentID:     "agent-1",
				Instruction: "搜索今天的重要新闻并整理摘要",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
			},
			{
				JobID:       "job-old-news",
				Name:        "旧新闻推送",
				AgentID:     "agent-1",
				Instruction: "搜索旧闻",
				Enabled:     false,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
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
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "list_scheduled_tasks", map[string]any{
		"query":   "新闻",
		"enabled": true,
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.listAgentID != "agent-1" {
		t.Fatalf("普通 agent 应只查询自己的任务，实际 agent_id=%q", svc.listAgentID)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("list response 不是 JSON 数组: %v", err)
	}
	if len(decoded) != 1 || decoded[0]["job_id"] != "job-news" {
		t.Fatalf("expected only enabled news task, got %+v", decoded)
	}
}

func TestListCanFilterCandidatesByDeliveryChannelAndState(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-feishu",
				Name:        "每日播报",
				AgentID:     "agent-1",
				Instruction: "整理摘要并发送",
				Enabled:     true,
				Running:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_123",
				},
				SessionTarget: protocol.SessionTarget{
					Kind:            protocol.SessionTargetNamed,
					NamedSessionKey: "daily-news",
				},
			},
			{
				JobID:       "job-inbox",
				Name:        "内部收件箱日报",
				AgentID:     "agent-1",
				Instruction: "整理摘要",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelInternalSegment,
					To:      protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", protocol.AutomationInboxSessionRef, ""),
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "list_scheduled_tasks", map[string]any{
		"query": "飞书群",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("list response 不是 JSON 数组: %v", err)
	}
	if len(decoded) != 1 || decoded[0]["job_id"] != "job-feishu" {
		t.Fatalf("expected feishu task by delivery alias, got %+v", decoded)
	}

	result, isError = callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "list_scheduled_tasks", map[string]any{
		"query": "running",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	decoded = nil
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("list response 不是 JSON 数组: %v", err)
	}
	if len(decoded) != 1 || decoded[0]["job_id"] != "job-feishu" {
		t.Fatalf("expected running task by state alias, got %+v", decoded)
	}
}

func TestListCanFilterCandidatesByCurrentExternalGroupQuery(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-news",
				Name:        "本群新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发送",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_123",
				},
			},
			{
				JobID:       "job-other-news",
				Name:        "其他群新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发送",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_other",
				},
			},
			{
				JobID:       "job-current-weather",
				Name:        "本群天气",
				AgentID:     "agent-1",
				Instruction: "发送天气",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source: protocol.Source{
					SessionKey: "agent:agent-1:fs:group:oc_group_123",
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "list_scheduled_tasks", map[string]any{
		"query": "这个群",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("list response 不是 JSON 数组: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected current group delivery/source tasks, got %+v", decoded)
	}
	ids := map[string]bool{}
	for _, item := range decoded {
		ids[item["job_id"].(string)] = true
	}
	if !ids["job-current-news"] || !ids["job-current-weather"] || ids["job-other-news"] {
		t.Fatalf("current group filter returned wrong tasks: %+v", decoded)
	}

	result, isError = callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "list_scheduled_tasks", map[string]any{
		"query": "新闻",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	decoded = nil
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("list response 不是 JSON 数组: %v", err)
	}
	if len(decoded) != 1 || decoded[0]["job_id"] != "job-current-news" {
		t.Fatalf("external group list query should prefer current group matches, got %+v", decoded)
	}
}

func TestListDefaultsToCurrentExternalGroupWithoutQuery(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-news",
				Name:        "本群新闻",
				AgentID:     "agent-1",
				Instruction: "发送新闻",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Delivery: protocol.DeliveryTarget{
					Mode:    protocol.DeliveryModeExplicit,
					Channel: protocol.SessionChannelFeishu,
					To:      "oc_group_123",
				},
			},
			{
				JobID:       "job-current-disabled",
				Name:        "本群已停用任务",
				AgentID:     "agent-1",
				Instruction: "发送状态",
				Enabled:     false,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source: protocol.Source{
					SessionKey: "agent:agent-1:fs:group:oc_group_123",
				},
			},
			{
				JobID:       "job-other-news",
				Name:        "其他群新闻",
				AgentID:     "agent-1",
				Instruction: "发送新闻",
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
	}, "list_scheduled_tasks", map[string]any{})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("list response 不是 JSON 数组: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected current group tasks by default, got %+v", decoded)
	}
	for _, item := range decoded {
		if item["job_id"] == "job-other-news" {
			t.Fatalf("default current group list should not include other groups: %+v", decoded)
		}
	}

	result, isError = callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "list_scheduled_tasks", map[string]any{
		"enabled": false,
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	decoded = nil
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("list response 不是 JSON 数组: %v", err)
	}
	if len(decoded) != 1 || decoded[0]["job_id"] != "job-current-disabled" {
		t.Fatalf("enabled filter should apply inside current group scope, got %+v", decoded)
	}
}

func TestListPropagatesError(t *testing.T) {
	svc := &stubService{listErr: errors.New("boom")}
	result, isError := callTool(t, svc, contract.ServerContext{IsMainAgent: true}, "list_scheduled_tasks", nil)
	if !isError {
		t.Fatalf("expected error result")
	}
	if !strings.Contains(extractText(t, result), "boom") {
		t.Fatalf("expected error text to contain boom, got %q", extractText(t, result))
	}
}

func TestSearchScheduledTaskHistoryReturnsDeletedCandidates(t *testing.T) {
	deletedAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	svc := &stubService{
		historyItems: []protocol.CronTaskHistoryItem{
			{
				JobID:         "job-deleted",
				Name:          "旧新闻日报",
				AgentID:       "agent-1",
				Deleted:       true,
				LatestAction:  protocol.TaskEventActionDelete,
				LatestEventAt: &deletedAt,
				DeletedAt:     &deletedAt,
				RunCount:      1,
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "search_scheduled_task_history", map[string]any{
		"query": "新闻",
		"limit": 5,
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.historyInput.Query != "新闻" || svc.historyInput.AgentID != "agent-1" ||
		!svc.historyInput.IncludeActive || !svc.historyInput.IncludeDeleted || svc.historyInput.Limit != 5 {
		t.Fatalf("history search input 不正确: %+v", svc.historyInput)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("history response 不是 JSON 数组: %v", err)
	}
	if len(decoded) != 1 || decoded[0]["job_id"] != "job-deleted" || decoded[0]["deleted"] != true {
		t.Fatalf("history response 不正确: %+v", decoded)
	}
}

func TestSearchScheduledTaskHistoryCanFilterCurrentExternalGroupQuery(t *testing.T) {
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-news",
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
				JobID:       "job-other-news",
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
	}, "search_scheduled_task_history", map[string]any{
		"query": "这个群的新闻任务",
		"limit": 10,
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if strings.Contains(svc.historyInput.Query, "这个群") || svc.historyInput.IncludeActive || !svc.historyInput.IncludeDeleted {
		t.Fatalf("deleted history lookup should strip current group terms and only search deleted candidates: %+v", svc.historyInput)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("history response 不是 JSON 数组: %v", err)
	}
	ids := map[string]bool{}
	for _, item := range decoded {
		ids[item["job_id"].(string)] = true
	}
	if len(decoded) != 2 || !ids["job-current-news"] || !ids["job-current-deleted"] || ids["job-other-news"] || ids["job-other-deleted"] {
		t.Fatalf("current group history filter returned wrong tasks: %+v", decoded)
	}

	result, isError = callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "search_scheduled_task_history", map[string]any{
		"query": "新闻任务",
		"limit": 10,
	})
	if isError {
		t.Fatalf("unexpected error without explicit current group terms: %s", extractText(t, result))
	}
	decoded = nil
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("history response 不是 JSON 数组: %v", err)
	}
	ids = map[string]bool{}
	for _, item := range decoded {
		ids[item["job_id"].(string)] = true
	}
	if len(decoded) != 2 || !ids["job-current-news"] || !ids["job-current-deleted"] || ids["job-other-news"] || ids["job-other-deleted"] {
		t.Fatalf("external group history query should prefer current group matches, got %+v", decoded)
	}
}

func TestSearchScheduledTaskHistoryCanFilterCurrentInternalConversationQuery(t *testing.T) {
	currentSessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "operator", "")
	otherSessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "other", "")
	svc := &stubService{
		jobs: []protocol.CronJob{
			{
				JobID:       "job-current-news",
				Name:        "当前会话每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source: protocol.Source{
					SessionKey: currentSessionKey,
				},
			},
			{
				JobID:       "job-other-news",
				Name:        "其他会话每日新闻",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并发给我",
				Enabled:     true,
				Schedule:    protocol.Schedule{Timezone: "Asia/Shanghai"},
				Source: protocol.Source{
					SessionKey: otherSessionKey,
				},
			},
		},
		historyItems: []protocol.CronTaskHistoryItem{
			{
				JobID:   "job-current-deleted",
				Name:    "当前会话旧新闻",
				AgentID: "agent-1",
				Deleted: true,
			},
			{
				JobID:   "job-other-deleted",
				Name:    "其他会话旧新闻",
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
						"name":               "当前会话旧新闻",
						"source_session_key": currentSessionKey,
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
						"name":               "其他会话旧新闻",
						"source_session_key": otherSessionKey,
					},
				},
			},
		},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
	}, "search_scheduled_task_history", map[string]any{
		"query": "当前会话的新闻任务",
		"limit": 10,
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if strings.Contains(svc.historyInput.Query, "当前会话") || svc.historyInput.IncludeActive || !svc.historyInput.IncludeDeleted {
		t.Fatalf("deleted history lookup should strip current conversation terms and only search deleted candidates: %+v", svc.historyInput)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &decoded); err != nil {
		t.Fatalf("history response 不是 JSON 数组: %v", err)
	}
	ids := map[string]bool{}
	for _, item := range decoded {
		ids[item["job_id"].(string)] = true
	}
	if len(decoded) != 2 || !ids["job-current-news"] || !ids["job-current-deleted"] || ids["job-other-news"] || ids["job-other-deleted"] {
		t.Fatalf("current conversation history filter returned wrong tasks: %+v", decoded)
	}
}
