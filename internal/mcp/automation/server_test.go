package automationmcp

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
)

var automationToolRequestSequence atomic.Uint64

func requireLastDeliveryToSession(t *testing.T, target automationdomain.DeliveryTarget, sessionKey string) {
	t.Helper()
	if target.Mode != automationdomain.DeliveryModeLast ||
		target.SessionKey != sessionKey ||
		target.Channel != "" ||
		target.To != "" ||
		target.AccountID != "" ||
		target.ThreadID != "" {
		t.Fatalf("expected delivery routed through current session %q, got %+v", sessionKey, target)
	}
}

func requireExplicitSessionDelivery(
	t *testing.T,
	target automationdomain.DeliveryTarget,
	channel string,
	sessionKey string,
) {
	t.Helper()
	if target.Mode != automationdomain.DeliveryModeExplicit ||
		target.Channel != channel ||
		target.To != sessionKey ||
		target.SessionKey != "" {
		t.Fatalf("expected explicit %s delivery to %q, got %+v", channel, sessionKey, target)
	}
}

type stubService struct {
	createInput       automationdomain.CreateJobInput
	updateInput       automationdomain.UpdateJobInput
	updateJobID       string
	statusJobID       string
	statusEnabled     bool
	deletedJobID      string
	runNowJobID       string
	created           *automationdomain.ScheduledTask
	recoverJobID      string
	recoverRunID      string
	redeliverJobID    string
	redeliverRunID    string
	listErr           error
	updateErr         error
	jobs              []automationdomain.ScheduledTask
	missingJobs       map[string]bool
	listAgentID       string
	runsByJob         map[string][]automationdomain.ScheduledTaskRun
	eventsByJob       map[string][]automationdomain.ScheduledTaskEvent
	historyItems      []automationdomain.ScheduledTaskHistoryItem
	historyInput      automationdomain.ScheduledTaskHistorySearchInput
	taskStatus        *automationdomain.ScheduledTaskStatus
	dailyReport       *automationdomain.ScheduledTaskDailyReport
	dailyReportsByJob map[string]*automationdomain.ScheduledTaskDailyReport
	dailyInput        automationdomain.ScheduledTaskDailyReportInput
	dailyInputs       []automationdomain.ScheduledTaskDailyReportInput
	heartbeatStatus   *automationdomain.HeartbeatStatus
}

func (s *stubService) ListTasks(_ context.Context, agentID string) ([]automationdomain.ScheduledTask, error) {
	s.listAgentID = agentID
	for index := range s.jobs {
		if s.jobs[index].ConfigurationVersion < 1 {
			s.jobs[index].ConfigurationVersion = 1
		}
	}
	return s.jobs, s.listErr
}

func (s *stubService) CreateTask(_ context.Context, input automationdomain.CreateJobInput) (*automationdomain.ScheduledTask, error) {
	s.createInput = input
	if s.created == nil {
		s.created = &automationdomain.ScheduledTask{
			JobID:                "job-1",
			Name:                 input.Name,
			AgentID:              input.AgentID,
			Schedule:             input.Schedule,
			Instruction:          input.Instruction,
			SessionTarget:        input.SessionTarget,
			Delivery:             input.Delivery,
			Source:               input.Source,
			Enabled:              input.Enabled,
			ConfigurationVersion: 1,
		}
	}
	return s.created, nil
}

func (s *stubService) UpdateTask(_ context.Context, jobID string, input automationdomain.UpdateJobInput) (*automationdomain.ScheduledTask, error) {
	s.updateJobID = jobID
	s.updateInput = input
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	job := automationdomain.ScheduledTask{
		JobID:    jobID,
		AgentID:  "agent-1",
		Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"},
	}
	for _, current := range s.jobs {
		if current.JobID == jobID {
			job = current
			break
		}
	}
	if input.Enabled != nil {
		s.statusJobID = jobID
		s.statusEnabled = *input.Enabled
		job.Enabled = *input.Enabled
	}
	if input.Delivery != nil {
		job.Delivery = *input.Delivery
	}
	found := false
	for index := range s.jobs {
		if s.jobs[index].JobID == jobID {
			s.jobs[index] = job
			found = true
		}
	}
	if !found {
		s.jobs = append(s.jobs, job)
	}
	return &job, nil
}

func (s *stubService) UpdateTaskAtVersion(
	ctx context.Context,
	jobID string,
	expectedVersion int64,
	input automationdomain.UpdateJobInput,
) (*automationdomain.ScheduledTask, error) {
	job, err := s.UpdateTask(ctx, jobID, input)
	if err != nil {
		return nil, err
	}
	job.ConfigurationVersion = expectedVersion + 1
	for index := range s.jobs {
		if s.jobs[index].JobID == jobID {
			s.jobs[index] = *job
		}
	}
	return job, nil
}

func (s *stubService) DeleteTask(_ context.Context, jobID string) (*automationdomain.DeleteJobResult, error) {
	s.deletedJobID = jobID
	result := &automationdomain.DeleteJobResult{JobID: jobID, Deleted: true}
	for _, job := range s.jobs {
		if job.JobID != jobID {
			continue
		}
		result.AgentID = job.AgentID
		result.ActiveRunID = job.RunningRunID
		if job.RunningRunID != "" {
			result.CancelledRunID = job.RunningRunID
			result.CancelledActiveRun = true
		}
		break
	}
	return result, nil
}

func (s *stubService) DeleteTaskAtVersion(
	ctx context.Context,
	jobID string,
	_ int64,
) (*automationdomain.DeleteJobResult, error) {
	result, err := s.DeleteTask(ctx, jobID)
	if err == nil {
		if s.missingJobs == nil {
			s.missingJobs = make(map[string]bool)
		}
		s.missingJobs[jobID] = true
	}
	return result, err
}

func (s *stubService) RunTaskNow(_ context.Context, jobID string) (*automationdomain.ExecutionResult, error) {
	s.runNowJobID = jobID
	return &automationdomain.ExecutionResult{JobID: jobID, Status: "succeeded"}, nil
}

func (s *stubService) ListTaskRuns(_ context.Context, jobID string) ([]automationdomain.ScheduledTaskRun, error) {
	if s.runsByJob == nil {
		return nil, nil
	}
	return s.runsByJob[jobID], nil
}

func (s *stubService) ListTaskEvents(_ context.Context, jobID string, _ int) ([]automationdomain.ScheduledTaskEvent, error) {
	if s.eventsByJob == nil {
		return nil, nil
	}
	return s.eventsByJob[jobID], nil
}

func (s *stubService) SearchTaskHistory(_ context.Context, input automationdomain.ScheduledTaskHistorySearchInput) ([]automationdomain.ScheduledTaskHistoryItem, error) {
	s.historyInput = input
	s.listAgentID = input.AgentID
	items := make([]automationdomain.ScheduledTaskHistoryItem, 0, len(s.jobs)+len(s.historyItems))
	if input.IncludeActive {
		for _, job := range s.jobs {
			if input.AgentID != "" && job.AgentID != input.AgentID {
				continue
			}
			if input.Query != "" && !automationexec.ScheduledTaskMatchesQuery(job, input.Query) {
				continue
			}
			enabled := job.Enabled
			items = append(items, automationdomain.ScheduledTaskHistoryItem{
				JobID:              job.JobID,
				Name:               job.Name,
				AgentID:            job.AgentID,
				Enabled:            &enabled,
				Running:            job.Running,
				NextRunAt:          job.NextRunAt,
				LastRunAt:          job.LastRunAt,
				LastRunStatus:      job.LastRunStatus,
				LastDeliveryStatus: job.LastDeliveryStatus,
			})
		}
	}
	if input.IncludeDeleted {
		for _, item := range s.historyItems {
			if item.Deleted {
				items = append(items, item)
			}
		}
	}
	if input.Limit > 0 && len(items) > input.Limit {
		items = items[:input.Limit]
	}
	return items, nil
}

func (s *stubService) GetTaskStatus(_ context.Context, jobID string, _ int, _ int) (*automationdomain.ScheduledTaskStatus, error) {
	if s.taskStatus != nil {
		return s.taskStatus, nil
	}
	job, err := s.GetTask(context.Background(), jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	return &automationdomain.ScheduledTaskStatus{
		Job:          *job,
		Health:       automationdomain.ScheduledTaskHealth{State: "scheduled"},
		RecentRuns:   s.runsByJob[jobID],
		RecentEvents: s.eventsByJob[jobID],
	}, nil
}

func (s *stubService) GetDailyReport(_ context.Context, input automationdomain.ScheduledTaskDailyReportInput) (*automationdomain.ScheduledTaskDailyReport, error) {
	s.dailyInput = input
	s.dailyInputs = append(s.dailyInputs, input)
	s.listAgentID = input.AgentID
	if s.dailyReportsByJob != nil {
		if report, ok := s.dailyReportsByJob[input.JobID]; ok {
			return report, nil
		}
	}
	if s.dailyReport != nil {
		return s.dailyReport, nil
	}
	return &automationdomain.ScheduledTaskDailyReport{
		Date:     input.Date,
		Timezone: input.Timezone,
		AgentID:  input.AgentID,
		JobID:    input.JobID,
	}, nil
}

func (s *stubService) RetryRunDelivery(_ context.Context, jobID string, runID string) (*automationdomain.ScheduledTaskRun, error) {
	s.redeliverJobID = jobID
	s.redeliverRunID = runID
	return &automationdomain.ScheduledTaskRun{
		JobID:          jobID,
		RunID:          runID,
		Status:         automationdomain.RunStatusSucceeded,
		DeliveryStatus: automationdomain.DeliveryStatusSucceeded,
	}, nil
}

func (s *stubService) RecoverTaskRunningRun(_ context.Context, jobID string, runID string) (*automationdomain.ScheduledTask, error) {
	s.recoverJobID = jobID
	s.recoverRunID = runID
	job, err := s.GetTask(context.Background(), jobID)
	if err != nil {
		return nil, err
	}
	job.Running = false
	job.RunningRunID = ""
	return job, nil
}

func (s *stubService) GetHeartbeatStatus(_ context.Context, agentID string) (*automationdomain.HeartbeatStatus, error) {
	if s.heartbeatStatus != nil && s.heartbeatStatus.AgentID == agentID {
		result := *s.heartbeatStatus
		return &result, nil
	}
	result := &automationdomain.HeartbeatStatus{
		AgentID:              agentID,
		EverySeconds:         1800,
		TargetMode:           automationdomain.HeartbeatTargetNone,
		AckMaxChars:          300,
		ConfigurationVersion: 1,
	}
	s.heartbeatStatus = result
	copyValue := *result
	return &copyValue, nil
}

func (s *stubService) UpdateHeartbeatAtVersion(
	_ context.Context,
	agentID string,
	expectedVersion int64,
	input automationdomain.HeartbeatUpdateInput,
) (*automationdomain.HeartbeatStatus, error) {
	result := &automationdomain.HeartbeatStatus{
		AgentID:              agentID,
		Enabled:              input.Enabled,
		EverySeconds:         input.EverySeconds,
		TargetMode:           input.TargetMode,
		AckMaxChars:          input.AckMaxChars,
		ConfigurationVersion: expectedVersion + 1,
	}
	s.heartbeatStatus = result
	copyValue := *result
	return &copyValue, nil
}

func (s *stubService) WakeHeartbeat(_ context.Context, agentID string, input automationdomain.HeartbeatWakeInput) (*automationdomain.HeartbeatWakeResult, error) {
	return &automationdomain.HeartbeatWakeResult{
		AgentID:   agentID,
		Mode:      input.Mode,
		Scheduled: true,
	}, nil
}

func (s *stubService) GetTask(_ context.Context, jobID string) (*automationdomain.ScheduledTask, error) {
	if s.missingJobs[jobID] {
		return nil, nil
	}
	for i := range s.jobs {
		if s.jobs[i].JobID == jobID {
			if s.jobs[i].ConfigurationVersion < 1 {
				s.jobs[i].ConfigurationVersion = 1
			}
			return &s.jobs[i], nil
		}
	}
	if s.created != nil && s.created.JobID == jobID {
		return s.created, nil
	}
	return &automationdomain.ScheduledTask{JobID: jobID, ConfigurationVersion: 1}, nil
}

func newInterval(v int) *int { return &v }

func callTool(t *testing.T, svc contract.Service, sctx contract.ServerContext, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	if strings.TrimSpace(sctx.SourceContextType) == "" {
		sctx.SourceContextType = "agent"
	}
	name, args = automationTestRoute(name, args)
	if args["operation"] == "create" {
		requestID, hasRequestID := args["request_id"]
		if !hasRequestID || strings.TrimSpace(fmt.Sprint(requestID)) == "" {
			args["request_id"] = fmt.Sprintf(
				"test-%s-%d",
				strings.ReplaceAll(t.Name(), "/", "-"),
				automationToolRequestSequence.Add(1),
			)
		}
	}
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

func automationTestRoute(name string, args map[string]any) (string, map[string]any) {
	routed := make(map[string]any, len(args)+1)
	for key, value := range args {
		routed[key] = value
	}
	operation := ""
	switch name {
	case "create_scheduled_task":
		operation = "create"
	case "find_scheduled_tasks":
		operation = "list"
	case "inspect_scheduled_task":
		operation, _ = routed["view"].(string)
		if operation == "" || operation == "status" {
			operation = "get"
		}
	case "update_scheduled_task":
		operation = "update"
	case "delete_scheduled_task":
		operation = "delete"
	case "get_scheduled_task_report":
		operation = "report"
	case "get_heartbeat":
		operation = "heartbeat"
	case "run_scheduled_task":
		operation = "run"
	case "repair_scheduled_task":
		if routed["action"] == "recover" {
			operation = "update"
			routed["enabled"] = false
			routed["cancel_active_run"] = true
		} else {
			operation = "retry_delivery"
		}
		delete(routed, "action")
	case "update_heartbeat":
		operation = "set_heartbeat"
	case "wake_heartbeat":
		operation = "wake"
	case "automation_query", "automation_update":
		return name, routed
	}
	routed["operation"] = operation
	if operation == "list" || operation == "get" || operation == "runs" || operation == "events" || operation == "report" || operation == "heartbeat" {
		return "automation_query", routed
	}
	return "automation_update", routed
}

func listTools(t *testing.T, svc contract.Service, sctx contract.ServerContext) []map[string]any {
	t.Helper()
	if strings.TrimSpace(sctx.SourceContextType) == "" {
		sctx.SourceContextType = "agent"
	}
	server := NewServer(svc, sctx)
	resp, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result, got %+v", resp)
	}
	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatalf("tools not []map, got %T", result["tools"])
	}
	return tools
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

func firstString(value any) string {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	text, _ := items[0].(string)
	return text
}

func stringSliceContains(value any, want string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if text, _ := item.(string); text == want {
			return true
		}
	}
	return false
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

func TestToolsListIncludesSearchHints(t *testing.T) {
	tools := listTools(t, &stubService{}, contract.ServerContext{})
	if len(tools) == 0 {
		t.Fatal("tools/list 返回空列表")
	}
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		meta, ok := tool["_meta"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing _meta", name)
		}
		hint, _ := meta["anthropic/searchHint"].(string)
		if strings.TrimSpace(hint) == "" {
			t.Fatalf("%s missing anthropic/searchHint", name)
		}
		if _, ok := meta["anthropic/alwaysLoad"]; ok {
			t.Fatalf("%s should stay deferred", name)
		}
	}
}
