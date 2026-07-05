package automationmcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestCreateRejectsMissingDefaultContext(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
	}{
		{
			name: "complex task missing execution mode",
			input: map[string]any{
				"name":        "每五分钟总结一次昨天的错误日志",
				"instruction": "请总结昨天的错误日志",
				"schedule":    intervalSchedule(5, "minutes"),
			},
		},
		{
			name: "simple default missing current session",
			input: map[string]any{
				"name":        "简单提醒",
				"instruction": "喝水",
				"schedule":    intervalSchedule(15, "minutes"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, isError := callTool(t, &stubService{}, contract.ServerContext{CurrentAgentID: "agent-1"}, "create_scheduled_task", test.input)
			if !isError {
				t.Fatalf("expected error result, got %+v", result)
			}
			if !strings.Contains(extractText(t, result), "execution_mode") {
				t.Fatalf("error must mention execution_mode: %s", extractText(t, result))
			}
		})
	}
}

func TestCreateAllowsSimpleDefaults(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":        "简单提醒",
		"instruction": "喝水",
		"schedule":    intervalSchedule(15, "minutes"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.SessionTarget.Kind != protocol.SessionTargetBound ||
		svc.createInput.SessionTarget.BoundSessionKey != sctx.CurrentSessionKey {
		t.Fatalf("expected current bound target from default, got %+v", svc.createInput.SessionTarget)
	}
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit ||
		svc.createInput.Delivery.To != sctx.CurrentSessionKey {
		t.Fatalf("expected visible current-session delivery from default, got %+v", svc.createInput.Delivery)
	}
	if svc.createInput.Schedule.IntervalSeconds == nil || *svc.createInput.Schedule.IntervalSeconds != 15*60 {
		t.Fatalf("expected 900s interval, got %+v", svc.createInput.Schedule.IntervalSeconds)
	}
}

func TestCreateDefaultsCurrentExternalChannel(t *testing.T) {
	tests := []struct {
		name        string
		taskName    string
		instruction string
	}{
		{name: "explicit this group", taskName: "飞书群每日新闻", instruction: "每天 9 点搜索重要新闻并发到这个飞书群"},
		{name: "broadcast intent", taskName: "每日新闻推送", instruction: "每天 9 点搜索重要新闻并推送摘要"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := &stubService{}
			result, isError := callTool(t, svc, contract.ServerContext{
				CurrentAgentID:    "agent-1",
				CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
				SourceContextType: "agent",
			}, "create_scheduled_task", map[string]any{
				"name":        test.taskName,
				"instruction": test.instruction,
				"schedule":    dailySchedule("09:00"),
			})
			if isError {
				t.Fatalf("unexpected error: %s", extractText(t, result))
			}
			if svc.createInput.SessionTarget.Kind != protocol.SessionTargetIsolated {
				t.Fatalf("expected temporary execution session from channel default, got %+v", svc.createInput.SessionTarget)
			}
			if svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
				svc.createInput.Delivery.To != "oc_group_123" {
				t.Fatalf("expected default delivery to current feishu group, got %+v", svc.createInput.Delivery)
			}
		})
	}
}

func TestCreateRejectsAmbiguousVisibleComplexTasks(t *testing.T) {
	tests := []struct {
		name        string
		sessionKey  string
		taskName    string
		instruction string
	}{
		{
			name:        "external search news without broadcast intent",
			sessionKey:  "agent:agent-1:fs:group:oc_group_123",
			taskName:    "每日新闻",
			instruction: "每天 9 点搜索重要新闻并整理摘要",
		},
		{
			name:        "external broadcast intent opted out",
			sessionKey:  "agent:agent-1:fs:group:oc_group_123",
			taskName:    "每日新闻静默任务",
			instruction: "每天 9 点搜索重要新闻，不要推送到群里",
		},
		{
			name:        "search news without explicit delivery",
			sessionKey:  "agent:agent-1:dm:dm-user:main:",
			taskName:    "每日新闻",
			instruction: "搜索今天的重要新闻",
		},
		{
			name:        "current context required",
			sessionKey:  "agent:agent-1:dm:dm-user:main:",
			taskName:    "每日对话总结",
			instruction: "每天 9 点总结这个对话并告诉我",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, isError := callTool(t, &stubService{}, contract.ServerContext{
				CurrentAgentID:    "agent-1",
				CurrentSessionKey: test.sessionKey,
				SourceContextType: "agent",
			}, "create_scheduled_task", map[string]any{
				"name":        test.taskName,
				"instruction": test.instruction,
				"schedule":    dailySchedule("09:00"),
			})
			if !isError {
				t.Fatalf("expected error result, got %+v", result)
			}
			if !strings.Contains(extractText(t, result), "execution_mode") ||
				!strings.Contains(extractText(t, result), "reply_mode") {
				t.Fatalf("error must mention execution and reply modes: %s", extractText(t, result))
			}
		})
	}
}

func TestCreateDefaultsVisibleComplexTaskToCurrentConversation(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:dm:dm-user:main:",
		SourceContextType: "agent",
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":        "每日新闻摘要",
		"instruction": "每天 9 点搜索重要新闻并发给我摘要",
		"schedule":    dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.SessionTarget.Kind != protocol.SessionTargetIsolated {
		t.Fatalf("expected isolated target for visible complex default, got %+v", svc.createInput.SessionTarget)
	}
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit ||
		svc.createInput.Delivery.To != sctx.CurrentSessionKey {
		t.Fatalf("expected current conversation delivery for visible complex default, got %+v", svc.createInput.Delivery)
	}
}

func TestCreateAllowsSimpleDefaultsWithJSONNumberAndDottedSchedule(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":                    "test",
		"instruction":             "提醒我喝水",
		"schedule.kind":           "interval",
		"schedule.interval_value": json.Number("1"),
		"schedule.interval_unit":  "minutes",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.SessionTarget.Kind != protocol.SessionTargetBound ||
		svc.createInput.SessionTarget.BoundSessionKey != sctx.CurrentSessionKey {
		t.Fatalf("expected current bound target from default, got %+v", svc.createInput.SessionTarget)
	}
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit ||
		svc.createInput.Delivery.To != sctx.CurrentSessionKey {
		t.Fatalf("expected visible current-session delivery from default, got %+v", svc.createInput.Delivery)
	}
	if svc.createInput.Schedule.IntervalSeconds == nil || *svc.createInput.Schedule.IntervalSeconds != 60 {
		t.Fatalf("expected 60s interval, got %+v", svc.createInput.Schedule.IntervalSeconds)
	}
}

func TestCreateExecutionModeExistingRequiresSession(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{CurrentAgentID: "agent-1"}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "跟进订单",
		"instruction":    "跟进订单状态并汇总",
		"execution_mode": "existing",
		"reply_mode":     "none",
		"schedule":       intervalSchedule(10, "minutes"),
	})
	if !isError {
		t.Fatalf("expected error result, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "selected_session_key") {
		t.Fatalf("expected hint about selected_session_key, got %q", extractText(t, result))
	}
}

func TestCreateExistingExecutionMatchesUIPayloadShape(t *testing.T) {
	svc := &stubService{}
	sessionKey := "agent:agent-1:ws:dm:current"
	sctx := contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		SourceContextType: "agent",
		SourceContextID:   "agent-1",
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":                 "半小时同步",
		"instruction":          "同步当前进展",
		"execution_mode":       "existing",
		"reply_mode":           "execution",
		"selected_session_key": sessionKey,
		"schedule":             intervalSchedule(30, "minutes"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	input := svc.createInput
	if input.Schedule.Kind != protocol.ScheduleKindEvery ||
		input.Schedule.IntervalSeconds == nil ||
		*input.Schedule.IntervalSeconds != 1800 {
		t.Fatalf("schedule should match UI every payload, got %+v", input.Schedule)
	}
	if input.SessionTarget.Kind != protocol.SessionTargetBound || input.SessionTarget.BoundSessionKey != sessionKey {
		t.Fatalf("session target should match UI existing payload, got %+v", input.SessionTarget)
	}
	if input.Delivery.Mode != protocol.DeliveryModeExplicit ||
		input.Delivery.Channel != "websocket" ||
		input.Delivery.To != sessionKey {
		t.Fatalf("delivery should match UI execution payload, got %+v", input.Delivery)
	}
	if input.Source.Kind != protocol.SourceKindAgent || input.Source.ContextType != "agent" || input.Source.ContextID != "agent-1" {
		t.Fatalf("source should preserve agent snapshot, got %+v", input.Source)
	}
}

func TestCreateDailyWithWeekdaysBuildsCron(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
	schedule := map[string]any{
		"kind":       "daily",
		"daily_time": "08:30",
		"weekdays":   []any{"mon", "wed", "fri"},
		"timezone":   "Asia/Shanghai",
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "工作日早会提醒",
		"instruction":    "提醒参加每日站会",
		"execution_mode": "temporary",
		"reply_mode":     "none",
		"schedule":       schedule,
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.Schedule.CronExpression == nil {
		t.Fatalf("expected cron expression to be generated")
	}
	if *svc.createInput.Schedule.CronExpression != "30 8 * * 1,3,5" {
		t.Fatalf("expected cron '30 8 * * 1,3,5', got %q", *svc.createInput.Schedule.CronExpression)
	}
}

func TestCreateRejectsUnsupportedScheduleKind(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "无效参数",
		"instruction":    "喝水",
		"execution_mode": "temporary",
		"reply_mode":     "none",
		"schedule": map[string]any{
			"kind":             "every",
			"interval_seconds": 300,
			"timezone":         "Asia/Shanghai",
		},
	})
	if !isError {
		t.Fatalf("expected error for unsupported kind=every, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "single") {
		t.Fatalf("error should hint at new kinds, got %q", extractText(t, result))
	}
}
