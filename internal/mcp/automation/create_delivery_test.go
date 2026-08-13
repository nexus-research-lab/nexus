package automationmcp

import (
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestCreatePageSemanticsForbidsMainWithReply(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "长期监控",
		"instruction":    "持续监控生产告警",
		"execution_mode": "main",
		"reply_mode":     "selected",
		"schedule":       intervalSchedule(5, "minutes"),
	})
	if !isError {
		t.Fatalf("expected error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "execution_mode=main") {
		t.Fatalf("error must mention execution_mode=main: %s", extractText(t, result))
	}
}

func TestCreateResolvesDeliveryFromReplyModeSelected(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:dm:dm-user:main:",
		SourceContextType: "agent",
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":                       "定点播报",
		"instruction":                "每天 9 点说早安",
		"execution_mode":             "temporary",
		"reply_mode":                 "selected",
		"selected_reply_session_key": "agent:agent-1:dm:dm-user:main:",
		"schedule":                   dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.Delivery.Mode != automationdomain.DeliveryModeExplicit {
		t.Fatalf("expected explicit delivery, got %q", svc.createInput.Delivery.Mode)
	}
	if svc.createInput.Delivery.To != sctx.CurrentSessionKey {
		t.Fatalf("expected delivery.To=current_session_key, got %q", svc.createInput.Delivery.To)
	}
	if svc.createInput.Schedule.CronExpression == nil || *svc.createInput.Schedule.CronExpression != "0 9 * * *" {
		t.Fatalf("expected cron 0 9 * * *, got %+v", svc.createInput.Schedule.CronExpression)
	}
}

func TestCreateOwnerMainRequiresExistingSessionForChannelDelivery(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:ws:dm:current",
		SourceContextType: "agent",
		IsMainAgent:       true,
	}, "create_scheduled_task", map[string]any{
		"name":           "新闻日报",
		"instruction":    "搜索今天的重要新闻并整理摘要",
		"execution_mode": "temporary",
		"reply_mode":     "channel",
		"reply_channel":  "feishu",
		"reply_to":       "oc_group_123",
		"schedule":       dailySchedule("09:00"),
	})
	if !isError || !strings.Contains(extractText(t, result), "existing authorized reply_session_key") {
		t.Fatalf("owner-main bare channel target should require a real session: %+v", result)
	}
	if svc.createInput.Name != "" {
		t.Fatalf("rejected bare target reached service: %+v", svc.createInput)
	}
}

func TestCreateOrdinaryAgentRejectsArbitraryChannel(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:ws:dm:current",
		SourceContextType: "agent",
	}, "create_scheduled_task", map[string]any{
		"name":           "越界投递",
		"instruction":    "把结果发到任意群",
		"execution_mode": "temporary",
		"reply_mode":     "channel",
		"reply_channel":  "feishu",
		"reply_to":       "oc_other_group",
		"schedule":       dailySchedule("09:00"),
	})
	if !isError {
		t.Fatalf("ordinary Agent arbitrary Channel delivery should fail: %+v", result)
	}
	if svc.createInput.AgentID != "" {
		t.Fatalf("rejected delivery reached service: %+v", svc.createInput)
	}
}

func TestCreateRejectsSyntheticAgentInboxModes(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
	}{
		{
			name: "explicit mode",
			input: map[string]any{
				"agent_id":   "agent-2",
				"reply_mode": "agent",
			},
		},
		{
			name: "inferred mode",
			input: map[string]any{
				"agent_id":       "agent-1",
				"reply_agent_id": "agent-2",
				"reply_mode":     "agent",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := &stubService{}
			input := map[string]any{
				"name":           "新闻日报",
				"instruction":    "搜索今天的重要新闻并整理摘要",
				"execution_mode": "temporary",
				"schedule":       dailySchedule("09:00"),
			}
			for key, value := range test.input {
				input[key] = value
			}
			result, isError := callTool(t, svc, contract.ServerContext{
				CurrentAgentID:    "main",
				CurrentSessionKey: "agent:main:ws:dm:current",
				SourceContextType: "agent",
				IsMainAgent:       true,
			}, "create_scheduled_task", input)
			if !isError {
				t.Fatalf("synthetic Agent inbox mode should be rejected: %+v", result)
			}
			if svc.createInput.Name != "" {
				t.Fatalf("rejected synthetic inbox reached service: %+v", svc.createInput)
			}
		})
	}
}

func TestCreateCanDeriveDeliveryFromExternalSessionKey(t *testing.T) {
	svc := &stubService{}
	sessionKey := "agent:agent-1:fs:group:oc_group_123"
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		SourceContextType: "agent",
	}, "create_scheduled_task", map[string]any{
		"name":                       "飞书群播报",
		"instruction":                "搜索今天的重要新闻并整理摘要",
		"execution_mode":             "temporary",
		"reply_mode":                 "selected",
		"selected_reply_session_key": sessionKey,
		"schedule":                   dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	requireLastDeliveryToSession(t, svc.createInput.Delivery, sessionKey)
}

func TestCreateChannelReplyDefaultsToCurrentExternalSession(t *testing.T) {
	svc := &stubService{}
	sessionKey := "agent:agent-1:fs:group:oc_group_123"
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		SourceContextType: "agent",
	}, "create_scheduled_task", map[string]any{
		"name":           "飞书群播报",
		"instruction":    "搜索今天的重要新闻并整理摘要",
		"execution_mode": "temporary",
		"reply_mode":     "channel",
		"schedule":       dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	requireLastDeliveryToSession(t, svc.createInput.Delivery, sessionKey)
}

func TestCreateCurrentExternalSessionHostBindsEveryIMChannel(t *testing.T) {
	channels := []string{
		protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu,
	}
	for _, channel := range channels {
		t.Run(channel, func(t *testing.T) {
			svc := &stubService{}
			sessionKey := protocol.BuildAgentAccountSessionKey(
				"agent-1",
				channel,
				protocol.RoomTypeDM,
				"account-1",
				"external-user",
				"thread-1",
			)
			result, isError := callTool(t, svc, contract.ServerContext{
				CurrentAgentID:    "agent-1",
				CurrentSessionKey: sessionKey,
				SourceContextType: "agent_paired",
			}, "create_scheduled_task", map[string]any{
				"name":             "当前 IM 回传",
				"instruction":      "返回测试结果",
				"execution_mode":   "temporary",
				"reply_mode":       "channel",
				"reply_channel":    "model-guessed-wrong-channel",
				"reply_to":         "model-guessed-target",
				"reply_account_id": "model-guessed-account",
				"reply_thread_id":  "model-guessed-thread",
				"schedule":         intervalSchedule(5, "minutes"),
			})
			if isError {
				t.Fatalf("%s current IM route should be host-bound: %s", channel, extractText(t, result))
			}
			requireLastDeliveryToSession(t, svc.createInput.Delivery, sessionKey)
		})
	}
}

func TestExternalIMAutomationSchemaHidesRouteIdentifiers(t *testing.T) {
	sctx := contract.ServerContext{
		CurrentAgentID: "agent-1",
		CurrentSessionKey: protocol.BuildAgentAccountSessionKey(
			"agent-1",
			protocol.SessionChannelWeixinPersonal,
			protocol.RoomTypeDM,
			"account-1",
			"external-user",
			"",
		),
		SourceContextType: "agent_paired",
	}
	tools := listTools(t, &stubService{}, sctx)
	matched := 0
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if name != "create_scheduled_task" && name != "update_scheduled_task" {
			continue
		}
		matched++
		schema, _ := tool["inputSchema"].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		for _, field := range []string{
			"reply_session_key",
			"reply_channel",
			"reply_to",
			"reply_account_id",
			"reply_thread_id",
		} {
			if _, exists := properties[field]; exists {
				t.Fatalf("%s schema must hide host-owned %s", name, field)
			}
		}
		description, _ := tool["description"].(string)
		if !strings.Contains(description, "weixin-personal") || !strings.Contains(description, "deliver_result") {
			t.Fatalf("%s description missing safe current IM context: %q", name, description)
		}
		for _, field := range []string{"execution_mode", "reply_mode", "selected_session_key", "selected_reply_session_key"} {
			if _, exists := properties[field]; exists {
				t.Fatalf("%s ordinary IM schema must hide legacy routing field %s", name, field)
			}
		}
	}
	if matched != 2 {
		t.Fatalf("matched %d create/update tools, want 2", matched)
	}
}

func TestOwnerMainPrivateDMSchemaOnlyExposesExistingSessionRouting(t *testing.T) {
	tools := listTools(t, &stubService{}, contract.ServerContext{
		CurrentAgentID:    "main",
		CurrentSessionKey: protocol.BuildAgentSessionKey("main", protocol.SessionChannelWebSocket, protocol.RoomTypeDM, "current", ""),
		SourceContextType: "agent",
		IsMainAgent:       true,
	})
	matched := 0
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if name != "create_scheduled_task" && name != "update_scheduled_task" {
			continue
		}
		matched++
		schema, _ := tool["inputSchema"].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		if _, exists := properties["reply_session_key"]; !exists {
			t.Fatalf("%s owner-main schema missing existing reply_session_key", name)
		}
		for _, field := range []string{"reply_channel", "reply_to", "reply_account_id", "reply_thread_id"} {
			if _, exists := properties[field]; exists {
				t.Fatalf("%s owner-main schema must hide bare route field %s", name, field)
			}
		}
	}
	if matched != 2 {
		t.Fatalf("matched %d create/update tools, want 2", matched)
	}
}

func TestCreateChannelReplyDefaultsMissingExecutionModeToTemporary(t *testing.T) {
	svc := &stubService{}
	sessionKey := "agent:agent-1:fs:group:oc_group_123"
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		SourceContextType: "agent",
	}, "create_scheduled_task", map[string]any{
		"name":        "飞书群播报",
		"instruction": "搜索今天的重要新闻并整理摘要",
		"reply_mode":  "channel",
		"schedule":    dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.SessionTarget.Kind != automationdomain.SessionTargetIsolated {
		t.Fatalf("expected temporary execution session from channel reply default, got %+v", svc.createInput.SessionTarget)
	}
	requireLastDeliveryToSession(t, svc.createInput.Delivery, sessionKey)
}

func TestCreateChannelReplyFillsMissingTargetFromCurrentExternalSession(t *testing.T) {
	svc := &stubService{}
	sessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-1",
		protocol.SessionChannelFeishu,
		"group",
		"chat_id",
		"oc_group_123",
		"",
	)
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		SourceContextType: "agent",
	}, "create_scheduled_task", map[string]any{
		"name":             "飞书群播报",
		"instruction":      "搜索今天的重要新闻并整理摘要",
		"reply_channel":    "feishu",
		"reply_account_id": "chat_id",
		"schedule":         dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.SessionTarget.Kind != automationdomain.SessionTargetIsolated {
		t.Fatalf("expected temporary execution session from channel reply default, got %+v", svc.createInput.SessionTarget)
	}
	requireLastDeliveryToSession(t, svc.createInput.Delivery, sessionKey)
}

func TestCreateSimpleReminderDefaultsToIsolatedExternalDelivery(t *testing.T) {
	svc := &stubService{}
	sessionKey := "agent:agent-1:fs:group:oc_group_123"
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		SourceContextType: "agent",
	}, "create_scheduled_task", map[string]any{
		"name":        "喝水提醒",
		"instruction": "喝水",
		"schedule":    intervalSchedule(30, "minutes"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.SessionTarget.Kind != automationdomain.SessionTargetIsolated {
		t.Fatalf("expected isolated execution, got %+v", svc.createInput.SessionTarget)
	}
	requireLastDeliveryToSession(t, svc.createInput.Delivery, sessionKey)
}

func TestCreateExecutionReplyDerivesDeliveryFromExternalSession(t *testing.T) {
	svc := &stubService{}
	sessionKey := "agent:agent-1:fs:group:oc_group_123"
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		SourceContextType: "agent",
	}, "create_scheduled_task", map[string]any{
		"name":                 "飞书群提醒",
		"instruction":          "每天 9 点提醒大家看日报",
		"execution_mode":       "existing",
		"reply_mode":           "execution",
		"selected_session_key": sessionKey,
		"schedule":             dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.SessionTarget.BoundSessionKey != sessionKey {
		t.Fatalf("expected execution session=%s, got %+v", sessionKey, svc.createInput.SessionTarget)
	}
	requireLastDeliveryToSession(t, svc.createInput.Delivery, sessionKey)
}

func TestCreateExecutionReplyTemporaryFromAgentContextFallsBackToNone(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:dm:dm-user:main:",
		SourceContextType: "agent",
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "定点播报",
		"instruction":    "每天 9 点说早安",
		"execution_mode": "temporary",
		"reply_mode":     "execution",
		"schedule":       dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.Delivery.Mode != automationdomain.DeliveryModeNone {
		t.Fatalf("expected delivery.mode=none for temporary+execution in agent context, got %q", svc.createInput.Delivery.Mode)
	}
}

func TestCreateExecutionReplyTemporaryFromRoomContextTargetsCurrentSession(t *testing.T) {
	svc := &stubService{}
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	sctx := contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: sessionKey,
		SourceContextType: "room",
		SourceContextID:   "room-1",
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "定点播报",
		"instruction":    "每天 9 点说早安",
		"execution_mode": "temporary",
		"reply_mode":     "execution",
		"schedule":       dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.Delivery.Mode != automationdomain.DeliveryModeExplicit {
		t.Fatalf("expected delivery.mode=explicit for temporary+execution in room context, got %q", svc.createInput.Delivery.Mode)
	}
	if svc.createInput.Delivery.To != sessionKey {
		t.Fatalf("expected delivery.To=current room session, got %q", svc.createInput.Delivery.To)
	}
	if svc.createInput.Source.ContextType != "room" ||
		svc.createInput.Source.ContextID != "room-1" ||
		svc.createInput.Source.SessionKey != sessionKey {
		t.Fatalf("expected room source snapshot, got %+v", svc.createInput.Source)
	}
}
