package automationmcp

import (
	"strings"
	"testing"

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
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit {
		t.Fatalf("expected explicit delivery, got %q", svc.createInput.Delivery.Mode)
	}
	if svc.createInput.Delivery.To != sctx.CurrentSessionKey {
		t.Fatalf("expected delivery.To=current_session_key, got %q", svc.createInput.Delivery.To)
	}
	if svc.createInput.Schedule.CronExpression == nil || *svc.createInput.Schedule.CronExpression != "0 9 * * *" {
		t.Fatalf("expected cron 0 9 * * *, got %+v", svc.createInput.Schedule.CronExpression)
	}
}

func TestCreateCanDeliverToExplicitChannel(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:ws:dm:current",
		SourceContextType: "agent",
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "新闻日报",
		"instruction":    "搜索今天的重要新闻并整理摘要",
		"execution_mode": "temporary",
		"reply_mode":     "channel",
		"reply_channel":  "feishu",
		"reply_to":       "oc_group_123",
		"schedule":       dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit ||
		svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
		svc.createInput.Delivery.To != "oc_group_123" {
		t.Fatalf("expected explicit feishu delivery, got %+v", svc.createInput.Delivery)
	}
}

func TestCreateInfersChannelReplyModeFromDeliveryFields(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:ws:dm:current",
		SourceContextType: "agent",
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "新闻日报",
		"instruction":    "搜索今天的重要新闻并整理摘要",
		"execution_mode": "temporary",
		"reply_channel":  "feishu",
		"reply_to":       "oc_group_123",
		"schedule":       dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit ||
		svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
		svc.createInput.Delivery.To != "oc_group_123" {
		t.Fatalf("expected explicit feishu delivery inferred from fields, got %+v", svc.createInput.Delivery)
	}
}

func TestCreateCanDeliverToAgentInbox(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{
		CurrentAgentID:    "main",
		CurrentSessionKey: "agent:main:ws:dm:current",
		SourceContextType: "agent",
		IsMainAgent:       true,
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "新闻日报",
		"agent_id":       "agent-2",
		"instruction":    "搜索今天的重要新闻并整理摘要",
		"execution_mode": "temporary",
		"reply_mode":     "agent",
		"schedule":       dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	expectedSessionKey := protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit ||
		svc.createInput.Delivery.Channel != protocol.SessionChannelInternalSegment ||
		svc.createInput.Delivery.To != expectedSessionKey {
		t.Fatalf("expected explicit internal agent delivery, got %+v", svc.createInput.Delivery)
	}
}

func TestCreateInfersAgentReplyModeFromReplyAgentID(t *testing.T) {
	svc := &stubService{}
	sctx := contract.ServerContext{
		CurrentAgentID:    "main",
		CurrentSessionKey: "agent:main:ws:dm:current",
		SourceContextType: "agent",
		IsMainAgent:       true,
	}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":           "新闻日报",
		"agent_id":       "agent-1",
		"instruction":    "搜索今天的重要新闻并整理摘要",
		"execution_mode": "temporary",
		"reply_agent_id": "agent-2",
		"schedule":       dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	expectedSessionKey := protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit ||
		svc.createInput.Delivery.Channel != protocol.SessionChannelInternalSegment ||
		svc.createInput.Delivery.To != expectedSessionKey {
		t.Fatalf("expected agent inbox delivery inferred from reply_agent_id, got %+v", svc.createInput.Delivery)
	}
}

func TestCreateCanDeriveDeliveryFromExternalSessionKey(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "create_scheduled_task", map[string]any{
		"name":                       "飞书群播报",
		"instruction":                "搜索今天的重要新闻并整理摘要",
		"execution_mode":             "temporary",
		"reply_mode":                 "selected",
		"selected_reply_session_key": "agent:agent-1:fs:group:oc_group_123",
		"schedule":                   dailySchedule("09:00"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
		svc.createInput.Delivery.To != "oc_group_123" {
		t.Fatalf("expected delivery derived from feishu session key, got %+v", svc.createInput.Delivery)
	}
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
	if svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
		svc.createInput.Delivery.To != "oc_group_123" {
		t.Fatalf("expected delivery defaulted from current feishu session, got %+v", svc.createInput.Delivery)
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
	if svc.createInput.SessionTarget.Kind != protocol.SessionTargetIsolated {
		t.Fatalf("expected temporary execution session from channel reply default, got %+v", svc.createInput.SessionTarget)
	}
	if svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
		svc.createInput.Delivery.To != "oc_group_123" {
		t.Fatalf("expected delivery defaulted from current feishu session, got %+v", svc.createInput.Delivery)
	}
}

func TestCreateChannelReplyFillsMissingTargetFromCurrentExternalSession(t *testing.T) {
	svc := &stubService{}
	sessionKey := "agent:agent-1:fs:group:oc_group_123"
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
	if svc.createInput.SessionTarget.Kind != protocol.SessionTargetIsolated {
		t.Fatalf("expected temporary execution session from channel reply default, got %+v", svc.createInput.SessionTarget)
	}
	if svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
		svc.createInput.Delivery.To != "oc_group_123" ||
		svc.createInput.Delivery.AccountID != "chat_id" {
		t.Fatalf("expected partial feishu target filled from current session, got %+v", svc.createInput.Delivery)
	}
}

func TestCreateSimpleReminderDefaultsToCurrentExternalSession(t *testing.T) {
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
	if svc.createInput.SessionTarget.BoundSessionKey != sessionKey {
		t.Fatalf("expected execution session=%s, got %+v", sessionKey, svc.createInput.SessionTarget)
	}
	if svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
		svc.createInput.Delivery.To != "oc_group_123" {
		t.Fatalf("expected default delivery derived from current feishu session, got %+v", svc.createInput.Delivery)
	}
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
	if svc.createInput.Delivery.Channel != protocol.SessionChannelFeishu ||
		svc.createInput.Delivery.To != "oc_group_123" {
		t.Fatalf("expected execution delivery derived from feishu session, got %+v", svc.createInput.Delivery)
	}
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
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeNone {
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
	if svc.createInput.Delivery.Mode != protocol.DeliveryModeExplicit {
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
