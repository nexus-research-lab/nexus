package automationmcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

type stubService struct {
	createInput automationsvc.CreateJobInput
	created     *automationsvc.CronJob
	listErr     error
	updateErr   error
	jobs        []automationsvc.CronJob
}

func (s *stubService) ListTasks(_ context.Context, _ string) ([]automationsvc.CronJob, error) {
	return s.jobs, s.listErr
}

func (s *stubService) CreateTask(_ context.Context, input automationsvc.CreateJobInput) (*automationsvc.CronJob, error) {
	s.createInput = input
	if s.created == nil {
		s.created = &automationsvc.CronJob{
			JobID:         "job-1",
			Name:          input.Name,
			AgentID:       input.AgentID,
			Schedule:      input.Schedule,
			Instruction:   input.Instruction,
			SessionTarget: input.SessionTarget,
			Delivery:      input.Delivery,
			Source:        input.Source,
			Enabled:       input.Enabled,
		}
	}
	return s.created, nil
}

func (s *stubService) UpdateTask(_ context.Context, _ string, _ automationsvc.UpdateJobInput) (*automationsvc.CronJob, error) {
	return nil, s.updateErr
}

func (s *stubService) UpdateTaskStatus(_ context.Context, jobID string, enabled bool) (*automationsvc.CronJob, error) {
	return &automationsvc.CronJob{JobID: jobID, Enabled: enabled, Schedule: automationsvc.Schedule{Timezone: "Asia/Shanghai"}}, nil
}

func (s *stubService) DeleteTask(_ context.Context, _ string) error { return nil }

func (s *stubService) RunTaskNow(_ context.Context, jobID string) (*automationsvc.ExecutionResult, error) {
	return &automationsvc.ExecutionResult{JobID: jobID, Status: "succeeded"}, nil
}

func (s *stubService) ListTaskRuns(_ context.Context, _ string) ([]automationsvc.CronRun, error) {
	return nil, nil
}

func (s *stubService) GetTask(_ context.Context, jobID string) (*automationsvc.CronJob, error) {
	for i := range s.jobs {
		if s.jobs[i].JobID == jobID {
			return &s.jobs[i], nil
		}
	}
	if s.created != nil && s.created.JobID == jobID {
		return s.created, nil
	}
	return &automationsvc.CronJob{JobID: jobID}, nil
}

func newInterval(v int) *int { return &v }

func callTool(t *testing.T, svc Service, sctx ServerContext, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	server := NewServer(svc, sctx)
	resp, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result, got %+v", resp)
	}
	isError, _ := result["isError"].(bool)
	return result, isError
}

func extractText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok {
		t.Fatalf("content not []map, got %T", result["content"])
	}
	if len(content) == 0 {
		t.Fatalf("empty content")
	}
	if s, ok := content[0]["text"].(string); ok {
		return s
	}
	t.Fatalf("text is not string, got %T", content[0]["text"])
	return ""
}

func intervalSchedule(value int, unit string) map[string]any {
	return map[string]any{
		"kind":           "interval",
		"interval_value": value,
		"interval_unit":  unit,
		"timezone":       "Asia/Shanghai",
	}
}

func dailySchedule(hhmm string) map[string]any {
	return map[string]any{
		"kind":       "daily",
		"daily_time": hhmm,
		"timezone":   "Asia/Shanghai",
	}
}

func TestCreateRejectsMissingExecutionMode(t *testing.T) {
	svc := &stubService{}
	sctx := ServerContext{CurrentAgentID: "agent-1"}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":        "每五分钟总结一次昨天的错误日志",
		"instruction": "请总结昨天的错误日志",
		"schedule":    intervalSchedule(5, "minutes"),
	})
	if !isError {
		t.Fatalf("expected error result, got %+v", result)
	}
	if !strings.Contains(extractText(t, result), "execution_mode") {
		t.Fatalf("error must mention execution_mode: %s", extractText(t, result))
	}
}

func TestCreateAllowsSimpleDefaults(t *testing.T) {
	svc := &stubService{}
	sctx := ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
	result, isError := callTool(t, svc, sctx, "create_scheduled_task", map[string]any{
		"name":        "简单提醒",
		"instruction": "喝水",
		"schedule":    intervalSchedule(15, "minutes"),
	})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
	if svc.createInput.SessionTarget.Kind != automationsvc.SessionTargetIsolated {
		t.Fatalf("expected isolated target from default, got %q", svc.createInput.SessionTarget.Kind)
	}
	if svc.createInput.Delivery.Mode != automationsvc.DeliveryModeNone {
		t.Fatalf("expected none delivery from default, got %q", svc.createInput.Delivery.Mode)
	}
	if svc.createInput.Schedule.IntervalSeconds == nil || *svc.createInput.Schedule.IntervalSeconds != 15*60 {
		t.Fatalf("expected 900s interval, got %+v", svc.createInput.Schedule.IntervalSeconds)
	}
}

func TestCreateExecutionModeExistingRequiresSession(t *testing.T) {
	svc := &stubService{}
	sctx := ServerContext{CurrentAgentID: "agent-1"}
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

func TestCreatePageSemanticsForbidsMainWithReply(t *testing.T) {
	svc := &stubService{}
	sctx := ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
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
	sctx := ServerContext{
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
	if svc.createInput.Delivery.Mode != automationsvc.DeliveryModeExplicit {
		t.Fatalf("expected explicit delivery, got %q", svc.createInput.Delivery.Mode)
	}
	if svc.createInput.Delivery.To != sctx.CurrentSessionKey {
		t.Fatalf("expected delivery.To=current_session_key, got %q", svc.createInput.Delivery.To)
	}
	if svc.createInput.Schedule.CronExpression == nil || *svc.createInput.Schedule.CronExpression != "0 9 * * *" {
		t.Fatalf("expected cron 0 9 * * *, got %+v", svc.createInput.Schedule.CronExpression)
	}
}

func TestCreateExecutionReplyTemporaryFromAgentContextFallsBackToNone(t *testing.T) {
	svc := &stubService{}
	sctx := ServerContext{
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
	if svc.createInput.Delivery.Mode != automationsvc.DeliveryModeNone {
		t.Fatalf("expected delivery.mode=none for temporary+execution in agent context, got %q", svc.createInput.Delivery.Mode)
	}
}

func TestCreateDailyWithWeekdaysBuildsCron(t *testing.T) {
	svc := &stubService{}
	sctx := ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
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
	sctx := ServerContext{CurrentAgentID: "agent-1", CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
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

func TestRunNowReturnsStatus(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, ServerContext{IsMainAgent: true}, "run_scheduled_task", map[string]any{"job_id": "job-1"})
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

func TestDeleteRequiresJobID(t *testing.T) {
	svc := &stubService{}
	result, isError := callTool(t, svc, ServerContext{IsMainAgent: true}, "delete_scheduled_task", map[string]any{})
	if !isError {
		t.Fatalf("expected error, got %+v", result)
	}
}

func TestListPassesAgentID(t *testing.T) {
	svc := &stubService{
		jobs: []automationsvc.CronJob{{JobID: "job-1", Schedule: automationsvc.Schedule{
			Kind: "every", IntervalSeconds: newInterval(300), Timezone: "Asia/Shanghai",
		}}},
	}
	result, isError := callTool(t, svc, ServerContext{IsMainAgent: true}, "list_scheduled_tasks", map[string]any{"agent_id": "agent-1"})
	if isError {
		t.Fatalf("unexpected error: %s", extractText(t, result))
	}
}

func TestListPropagatesError(t *testing.T) {
	svc := &stubService{listErr: errors.New("boom")}
	result, isError := callTool(t, svc, ServerContext{IsMainAgent: true}, "list_scheduled_tasks", nil)
	if !isError {
		t.Fatalf("expected error result")
	}
	if !strings.Contains(extractText(t, result), "boom") {
		t.Fatalf("expected error text to contain boom, got %q", extractText(t, result))
	}
}
