package automationmcp

import (
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestUpdateCanDisableDeliveryWithoutExecutionMode(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, contract.ServerContext{IsMainAgent: true}, "update_scheduled_task", map[string]any{
		"job_id":     "job-1",
		"reply_mode": "none",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateJobID != "job-1" {
		t.Fatalf("expected update job_id=job-1, got %q", svc.updateJobID)
	}
	if svc.updateInput.SessionTarget != nil {
		t.Fatalf("delivery-only update must not rewrite execution target, got %+v", svc.updateInput.SessionTarget)
	}
	if svc.updateInput.Delivery == nil || svc.updateInput.Delivery.Mode != automationdomain.DeliveryModeNone {
		t.Fatalf("expected delivery.mode=none, got %+v", svc.updateInput.Delivery)
	}
}

func TestUpdateMapsPermissionMode(t *testing.T) {
	svc := &stubService{jobs: []automationdomain.ScheduledTask{{
		JobID:    "job-1",
		AgentID:  "agent-1",
		Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
	}}}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:ws:dm:current",
	}, "update_scheduled_task", map[string]any{
		"job_id":          "job-1",
		"permission_mode": "dontAsk",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateInput.PermissionMode == nil || *svc.updateInput.PermissionMode != automationdomain.PermissionModeDontAsk {
		t.Fatalf("update_scheduled_task 未透传 permission_mode: %+v", svc.updateInput)
	}
}

func TestUpdateRequiresExistingSessionForChannelDelivery(t *testing.T) {
	svc := &stubService{jobs: []automationdomain.ScheduledTask{{
		JobID: "job-1", AgentID: "agent-1",
		Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
	}}}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "main",
		CurrentSessionKey: "agent:main:ws:dm:current",
		SourceContextType: "agent",
		IsMainAgent:       true,
	}, "update_scheduled_task", map[string]any{
		"job_id":        "job-1",
		"reply_mode":    "channel",
		"reply_channel": "feishu",
		"reply_to":      "oc_group_123",
	})
	if !isError || !strings.Contains(extractText(t, result), "existing authorized reply_session_key") {
		t.Fatalf("owner-main bare channel update should require a real session: %+v", result)
	}
	if svc.updateJobID != "" {
		t.Fatalf("rejected bare target reached service: %+v", svc.updateInput)
	}
}

func TestUpdateRejectsSyntheticReplyAgentField(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:ws:dm:current",
		SourceContextType: "agent",
		IsMainAgent:       true,
	}, "update_scheduled_task", map[string]any{
		"job_id":         "job-1",
		"reply_agent_id": "agent-2",
	})
	if !isError {
		t.Fatalf("synthetic reply_agent_id should not be accepted: %+v", result)
	}
	if svc.updateJobID != "" {
		t.Fatalf("rejected synthetic inbox reached service: %q", svc.updateJobID)
	}
}

func TestUpdateOrdinaryAgentRejectsOtherAgentInbox(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID: "job-1", AgentID: "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:ws:dm:current",
		SourceContextType: "agent",
	}, "update_scheduled_task", map[string]any{
		"job_id":         "job-1",
		"reply_agent_id": "agent-2",
	})
	if !isError {
		t.Fatalf("ordinary Agent cross-Agent delivery should fail: %+v", result)
	}
	if svc.updateJobID != "" {
		t.Fatalf("rejected delivery reached service: %q", svc.updateJobID)
	}
}

func TestUpdateCanRetargetDeliveryToCurrentExternalSession(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "update_scheduled_task", map[string]any{
		"job_id":     "job-1",
		"reply_mode": "channel",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateInput.SessionTarget != nil {
		t.Fatalf("delivery-only update must not rewrite execution target, got %+v", svc.updateInput.SessionTarget)
	}
	if svc.updateInput.Delivery == nil {
		t.Fatal("expected delivery retargeted to current feishu session")
	}
	requireLastDeliveryToSession(t, *svc.updateInput.Delivery, "agent:agent-1:fs:group:oc_group_123")
}

func TestUpdateCanFillPartialChannelTargetFromCurrentExternalSession(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID: "agent-1",
		CurrentSessionKey: protocol.BuildAgentAccountSessionKey(
			"agent-1",
			protocol.SessionChannelFeishu,
			"group",
			"chat_id",
			"oc_group_123",
			"",
		),
	}, "update_scheduled_task", map[string]any{
		"job_id":           "job-1",
		"reply_channel":    "feishu",
		"reply_account_id": "chat_id",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateInput.SessionTarget != nil {
		t.Fatalf("delivery-only update must not rewrite execution target, got %+v", svc.updateInput.SessionTarget)
	}
	if svc.updateInput.Delivery == nil {
		t.Fatal("expected partial feishu target filled from current session")
	}
	requireLastDeliveryToSession(t, *svc.updateInput.Delivery, protocol.BuildAgentAccountSessionKey(
		"agent-1",
		protocol.SessionChannelFeishu,
		"group",
		"chat_id",
		"oc_group_123",
		"",
	))
}

func TestUpdateCurrentExternalSessionIgnoresModelSuppliedChannelTarget(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123",
	}, "update_scheduled_task", map[string]any{
		"job_id":        "job-1",
		"reply_mode":    "channel",
		"reply_channel": "telegram",
		"reply_to":      "model-guessed-target",
	})
	if isError {
		t.Fatalf("current IM route should be host-bound, got %s", extractText(t, result))
	}
	if svc.updateInput.Delivery == nil {
		t.Fatal("expected host-bound current IM delivery")
	}
	requireLastDeliveryToSession(t, *svc.updateInput.Delivery, "agent:agent-1:fs:group:oc_group_123")
}

func TestUpdateSelectedReplyDefaultsToCurrentConversation(t *testing.T) {
	currentSessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		"dm",
		"user-123",
		"",
	)
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{
		CurrentAgentID:    "agent-1",
		CurrentSessionKey: currentSessionKey,
	}, "update_scheduled_task", map[string]any{
		"job_id":     "job-1",
		"reply_mode": "selected",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateInput.SessionTarget != nil {
		t.Fatalf("delivery-only update must not rewrite execution target, got %+v", svc.updateInput.SessionTarget)
	}
	if svc.updateInput.Delivery == nil {
		t.Fatal("expected selected delivery to current conversation")
	}
	requireExplicitSessionDelivery(t, *svc.updateInput.Delivery, protocol.SessionChannelInternalSegment, currentSessionKey)
}

func TestUpdateSelectedReplyRequiresConversationTarget(t *testing.T) {
	tests := []contract.ServerContext{
		{CurrentAgentID: "agent-1"},
		{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:fs:group:oc_group_123"},
	}

	for _, sctx := range tests {
		t.Run(sctx.CurrentSessionKey, func(t *testing.T) {
			svc := &stubService{
				jobs: []automationdomain.ScheduledTask{{
					JobID:    "job-1",
					AgentID:  "agent-1",
					Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
				}},
			}
			result, isError := callTool(t, svc, sctx, "update_scheduled_task", map[string]any{
				"job_id":     "job-1",
				"reply_mode": "selected",
			})
			if !isError {
				t.Fatalf("expected missing selected reply target error, got %+v", result)
			}
			if !strings.Contains(extractText(t, result), "selected_reply_session_key") {
				t.Fatalf("error should mention selected_reply_session_key, got %q", extractText(t, result))
			}
			if svc.updateJobID != "" {
				t.Fatalf("invalid selected update should not reach service, got job_id=%q", svc.updateJobID)
			}
		})
	}
}

func TestUpdateRejectsSyntheticAgentDeliveryMode(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-2",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "main", IsMainAgent: true}, "update_scheduled_task", map[string]any{
		"job_id":     "job-1",
		"reply_mode": "agent",
	})
	if !isError {
		t.Fatalf("synthetic Agent inbox mode should be rejected: %+v", result)
	}
	if svc.updateJobID != "" {
		t.Fatalf("rejected synthetic inbox reached service: %q", svc.updateJobID)
	}
}

func TestUpdateExecutionReplyRequiresExecutionMode(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, contract.ServerContext{IsMainAgent: true}, "update_scheduled_task", map[string]any{
		"job_id":     "job-1",
		"reply_mode": "execution",
	})
	if !isError {
		t.Fatalf("expected error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "reply_mode=execution") {
		t.Fatalf("error should mention reply_mode=execution, got %q", extractText(t, result))
	}
}

func TestUpdateNameOnlyDoesNotRewriteExecutionOrDelivery(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "update_scheduled_task", map[string]any{
		"job_id": "job-1",
		"name":   "每日新闻摘要",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateJobID != "job-1" {
		t.Fatalf("expected update job_id=job-1, got %q", svc.updateJobID)
	}
	if svc.updateInput.Name == nil || *svc.updateInput.Name != "每日新闻摘要" {
		t.Fatalf("expected name-only update, got %+v", svc.updateInput.Name)
	}
	if svc.updateInput.SessionTarget != nil || svc.updateInput.Delivery != nil || svc.updateInput.Schedule != nil {
		t.Fatalf("name-only update must not rewrite schedule/session/delivery, got %+v", svc.updateInput)
	}
}

func TestUpdateCanAppendInstruction(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:       "job-1",
			AgentID:     "agent-1",
			Instruction: "搜索新闻并整理摘要",
			Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "update_scheduled_task", map[string]any{
		"job_id":             "job-1",
		"instruction_append": "附带来源链接",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateInput.Instruction == nil ||
		*svc.updateInput.Instruction != "搜索新闻并整理摘要\n\n附带来源链接" {
		t.Fatalf("expected appended instruction, got %+v", svc.updateInput.Instruction)
	}
	if svc.updateInput.SessionTarget != nil || svc.updateInput.Delivery != nil || svc.updateInput.Schedule != nil {
		t.Fatalf("instruction append must not rewrite schedule/session/delivery, got %+v", svc.updateInput)
	}
}

func TestUpdateRejectsInstructionReplaceAndAppendTogether(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:       "job-1",
			AgentID:     "agent-1",
			Instruction: "搜索新闻并整理摘要",
			Schedule:    automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "update_scheduled_task", map[string]any{
		"job_id":             "job-1",
		"instruction":        "重写后的任务",
		"instruction_append": "附带来源链接",
	})
	if !isError {
		t.Fatalf("expected instruction conflict error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "instruction_append") {
		t.Fatalf("error should mention instruction_append, got %q", extractText(t, result))
	}
	if svc.updateJobID != "" {
		t.Fatalf("invalid instruction update should not reach service, got job_id=%q", svc.updateJobID)
	}
}

func TestUpdateScheduleOnlyDoesNotRewriteExecutionOrDelivery(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1", DefaultTimezone: "Asia/Shanghai"}, "update_scheduled_task", map[string]any{
		"job_id":   "job-1",
		"schedule": dailySchedule("08:30"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateInput.Schedule == nil ||
		svc.updateInput.Schedule.CronExpression == nil ||
		*svc.updateInput.Schedule.CronExpression != "30 8 * * *" {
		t.Fatalf("expected schedule-only cron update, got %+v", svc.updateInput.Schedule)
	}
	if svc.updateInput.SessionTarget != nil || svc.updateInput.Delivery != nil {
		t.Fatalf("schedule-only update must not rewrite session/delivery, got %+v", svc.updateInput)
	}
}

func TestUpdateScheduledTaskCanResolveUniqueQuery(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{
			{
				JobID:       "job-news",
				Name:        "每日新闻摘要",
				AgentID:     "agent-1",
				Instruction: "搜索新闻并投递",
				Enabled:     true,
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
		"query": "每日新闻",
		"name":  "早间新闻摘要",
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateJobID != "job-news" {
		t.Fatalf("update by query should target job-news, got %q", svc.updateJobID)
	}
	if svc.updateInput.Name == nil || *svc.updateInput.Name != "早间新闻摘要" {
		t.Fatalf("expected query update to pass new name, got %+v", svc.updateInput.Name)
	}
}

func TestUpdateRejectsEmptyPatch(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-1",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "update_scheduled_task", map[string]any{
		"job_id": "job-1",
	})
	if !isError {
		t.Fatalf("expected empty update error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "at least one field") {
		t.Fatalf("error should explain missing update field, got %q", extractText(t, result))
	}
	if svc.updateJobID != "" {
		t.Fatalf("empty update should not reach service, got job_id=%q", svc.updateJobID)
	}
}

func TestRegularAgentCannotUpdateAnotherAgentsTask(t *testing.T) {
	svc := &stubService{
		jobs: []automationdomain.ScheduledTask{{
			JobID:    "job-1",
			AgentID:  "agent-2",
			Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
		}},
	}
	result, isError := callTool(t, svc, contract.ServerContext{CurrentAgentID: "agent-1"}, "update_scheduled_task", map[string]any{
		"job_id": "job-1",
		"name":   "不该修改",
	})
	if !isError {
		t.Fatalf("expected ownership error, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "another agent") {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.updateJobID != "" {
		t.Fatalf("update should not be called for another agent task, got %q", svc.updateJobID)
	}
}
