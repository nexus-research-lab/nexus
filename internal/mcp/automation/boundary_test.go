package automationmcp

import (
	"encoding/json"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestExternalAndBackgroundContextsOnlyExposeReadTools(t *testing.T) {
	for _, source := range []string{"", "agent_channel", "agent_external", "agent_automation", "room_queue", "room_internal"} {
		t.Run(source, func(t *testing.T) {
			server := NewServer(&stubService{}, contract.ServerContext{SourceContextType: source})
			response, err := server.HandleMessage(t.Context(), map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "tools/list",
			})
			if err != nil {
				t.Fatalf("HandleMessage error: %v", err)
			}
			result, ok := response["result"].(map[string]any)
			if !ok {
				t.Fatalf("missing result: %+v", response)
			}
			tools, ok := result["tools"].([]map[string]any)
			if !ok {
				t.Fatalf("tools type = %T", result["tools"])
			}
			want := []string{"automation_query"}
			if len(tools) != len(want) {
				t.Fatalf("tools = %+v, want exactly %v", tools, want)
			}
			for index, item := range tools {
				if item["name"] != want[index] {
					t.Fatalf("tool[%d] = %v, want %q", index, item["name"], want[index])
				}
			}
		})
	}
}

func TestPairedAgentContextExposesMutationsWithoutOwnerWideAuthority(t *testing.T) {
	sctx := contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelTelegramSegment, protocol.RoomTypeDM, "user-1", ""),
		SourceContextType: "agent_paired",
		IsMainAgent:       true,
	}
	server := NewServer(&stubService{}, sctx)
	response, err := server.HandleMessage(t.Context(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result := response["result"].(map[string]any)
	tools := result["tools"].([]map[string]any)
	if len(tools) != 2 || tools[0]["name"] != "automation_query" || tools[1]["name"] != "automation_update" {
		t.Fatalf("paired Agent DM 未获得完整 Automation 工具: %+v", tools)
	}

	svc := &stubService{}
	_, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"request_id":     "paired-cross-agent",
		"name":           "越权任务",
		"agent_id":       "agent-2",
		"instruction":    "不应创建",
		"schedule":       intervalSchedule(1, "hours"),
		"execution_mode": "temporary",
		"reply_mode":     "none",
	})
	if !isError || svc.createInput.AgentID != "" {
		t.Fatalf("paired main Agent 不得获得跨 Agent owner scope: input=%+v", svc.createInput)
	}
}

func TestConversationalAutomationCannotCreateScriptTask(t *testing.T) {
	svc := &stubService{}
	_, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		SourceContextType: "agent",
	}, "create_scheduled_task", map[string]any{
		"name":           "unsafe",
		"instruction":    "env",
		"execution_kind": "script",
		"schedule":       intervalSchedule(1, "hours"),
	})
	if !isError {
		t.Fatal("script task creation through conversation should fail")
	}
	if svc.createInput.ExecutionKind != "" {
		t.Fatalf("service received script create input: %+v", svc.createInput)
	}
}

func TestConversationalAutomationCannotMutateOrRunExistingScriptTask(t *testing.T) {
	for _, test := range []struct {
		name string
		args map[string]any
	}{
		{name: "update_scheduled_task", args: map[string]any{"job_id": "script-1", "enabled": false}},
		{name: "delete_scheduled_task", args: map[string]any{"job_id": "script-1"}},
		{name: "run_scheduled_task", args: map[string]any{"job_id": "script-1"}},
		{name: "repair_scheduled_task", args: map[string]any{"job_id": "script-1", "action": "recover"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := &stubService{jobs: []automationdomain.ScheduledTask{{
				JobID:         "script-1",
				AgentID:       "agent-1",
				ExecutionKind: automationdomain.ExecutionKindScript,
			}}}
			_, isError := callTool(t, svc, contract.ServerContext{
				CurrentAgentID:    "agent-1",
				SourceContextType: "agent",
			}, test.name, test.args)
			if !isError {
				t.Fatalf("%s should reject a script task", test.name)
			}
			if svc.updateJobID != "" || svc.deletedJobID != "" || svc.runNowJobID != "" || svc.recoverJobID != "" {
				t.Fatalf("mutation reached service: %+v", svc)
			}
		})
	}
}

func TestExternalReadToolsStayWithinCurrentAgentAndConversation(t *testing.T) {
	currentSessionKey := protocol.BuildAgentSessionKey(
		"main",
		protocol.SessionChannelFeishuSegment,
		"group",
		"chat-current",
		"",
	)
	sctx := contract.ServerContext{
		CurrentAgentID:    "main",
		IsMainAgent:       true,
		CurrentSessionKey: currentSessionKey,
		SourceContextType: "agent_channel",
	}
	svc := &stubService{jobs: []automationdomain.ScheduledTask{
		{
			JobID:   "current-task",
			Name:    "当前群日报",
			AgentID: "main",
			Source:  automationdomain.Source{SessionKey: currentSessionKey},
		},
		{
			JobID:   "private-task",
			Name:    "私有任务",
			AgentID: "main",
			Source:  automationdomain.Source{SessionKey: "agent:main:ws:dm:private:"},
		},
	}}

	result, isError := callTool(t, svc, sctx, "find_scheduled_tasks", map[string]any{})
	if isError {
		t.Fatalf("current-conversation find failed: %s", extractText(t, result))
	}
	var items []automationdomain.ScheduledTaskHistoryItem
	if err := json.Unmarshal([]byte(extractText(t, result)), &items); err != nil {
		t.Fatalf("decode find result: %v", err)
	}
	if len(items) != 1 || items[0].JobID != "current-task" {
		t.Fatalf("external find leaked unrelated tasks: %+v", items)
	}

	_, isError = callTool(t, svc, sctx, "inspect_scheduled_task", map[string]any{
		"job_id": "private-task",
	})
	if !isError {
		t.Fatal("external inspect should reject a task outside the current conversation")
	}
	_, isError = callTool(t, svc, sctx, "find_scheduled_tasks", map[string]any{
		"agent_id": "other-agent",
	})
	if !isError {
		t.Fatal("external main Agent context should not retain cross-Agent read authority")
	}
}

func TestPersonalWeixinReadToolsUseCurrentConversationScope(t *testing.T) {
	currentSessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-1",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"account-1",
		"user-1",
		"",
	)
	svc := &stubService{jobs: []automationdomain.ScheduledTask{
		{
			JobID:   "current-task",
			Name:    "当前微信提醒",
			AgentID: "agent-1",
			Source:  automationdomain.Source{SessionKey: currentSessionKey},
		},
		{
			JobID:   "unrelated-task",
			Name:    "其他对话提醒",
			AgentID: "agent-1",
			Source:  automationdomain.Source{SessionKey: "agent:agent-1:ws:dm:private:"},
		},
	}}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
		SourceContextType: "agent_channel",
	}, "find_scheduled_tasks", map[string]any{})
	if isError {
		t.Fatalf("personal Weixin find failed: %s", extractText(t, result))
	}
	var items []automationdomain.ScheduledTaskHistoryItem
	if err := json.Unmarshal([]byte(extractText(t, result)), &items); err != nil {
		t.Fatalf("decode find result: %v", err)
	}
	if len(items) != 1 || items[0].JobID != "current-task" {
		t.Fatalf("personal Weixin find leaked unrelated tasks: %+v", items)
	}
}

func TestUnknownExternalChannelReadScopeFailsClosed(t *testing.T) {
	currentSessionKey := "agent:agent-1:future-chat:group:room-1"
	svc := &stubService{jobs: []automationdomain.ScheduledTask{
		{
			JobID:   "current-task",
			AgentID: "agent-1",
			Source:  automationdomain.Source{SessionKey: currentSessionKey},
		},
		{
			JobID:   "unrelated-task",
			AgentID: "agent-1",
			Source:  automationdomain.Source{SessionKey: "agent:agent-1:future-chat:group:room-2"},
		},
	}}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
		SourceContextType: "agent_channel",
	}, "find_scheduled_tasks", map[string]any{})
	if isError {
		t.Fatalf("future channel find failed: %s", extractText(t, result))
	}
	var items []automationdomain.ScheduledTaskHistoryItem
	if err := json.Unmarshal([]byte(extractText(t, result)), &items); err != nil {
		t.Fatalf("decode find result: %v", err)
	}
	if len(items) != 1 || items[0].JobID != "current-task" {
		t.Fatalf("unknown external channel should fail closed to current conversation: %+v", items)
	}
}

func TestExternalReadScopeSeparatesAccountsAndThreads(t *testing.T) {
	currentSessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-1",
		protocol.SessionChannelTelegram,
		"group",
		"account-a",
		"group-1",
		"topic-1",
	)
	taskWithDelivery := func(jobID, accountID, threadID string) automationdomain.ScheduledTask {
		return automationdomain.ScheduledTask{
			JobID:   jobID,
			AgentID: "agent-1",
			Delivery: automationdomain.DeliveryTarget{
				Mode:      automationdomain.DeliveryModeExplicit,
				Channel:   protocol.SessionChannelTelegram,
				To:        "group-1",
				AccountID: accountID,
				ThreadID:  threadID,
			},
		}
	}
	svc := &stubService{jobs: []automationdomain.ScheduledTask{
		taskWithDelivery("current-topic", "account-a", "topic-1"),
		taskWithDelivery("group-wide", "account-a", ""),
		taskWithDelivery("other-account", "account-b", "topic-1"),
		taskWithDelivery("other-topic", "account-a", "topic-2"),
	}}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
		SourceContextType: "agent_channel",
	}, "find_scheduled_tasks", map[string]any{})
	if isError {
		t.Fatalf("account/thread scoped find failed: %s", extractText(t, result))
	}
	var items []automationdomain.ScheduledTaskHistoryItem
	if err := json.Unmarshal([]byte(extractText(t, result)), &items); err != nil {
		t.Fatalf("decode find result: %v", err)
	}
	if len(items) != 2 || items[0].JobID != "current-topic" || items[1].JobID != "group-wide" {
		t.Fatalf("external find crossed account/thread boundary: %+v", items)
	}
}
