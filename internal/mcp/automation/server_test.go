package automationmcp

import (
	"context"
	"strings"
	"testing"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
)

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
}

func (s *stubService) ListTasks(_ context.Context, agentID string) ([]automationdomain.ScheduledTask, error) {
	s.listAgentID = agentID
	return s.jobs, s.listErr
}

func (s *stubService) CreateTask(_ context.Context, input automationdomain.CreateJobInput) (*automationdomain.ScheduledTask, error) {
	s.createInput = input
	if s.created == nil {
		s.created = &automationdomain.ScheduledTask{
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
	return &job, nil
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
	if s.listErr != nil {
		return nil, s.listErr
	}
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
	return &automationdomain.ScheduledTask{JobID: jobID, AgentID: "agent-1", Schedule: automationdomain.Schedule{Timezone: "Asia/Shanghai"}}, nil
}

func (s *stubService) GetTask(_ context.Context, jobID string) (*automationdomain.ScheduledTask, error) {
	if s.missingJobs[jobID] {
		return nil, nil
	}
	for i := range s.jobs {
		if s.jobs[i].JobID == jobID {
			return &s.jobs[i], nil
		}
	}
	if s.created != nil && s.created.JobID == jobID {
		return s.created, nil
	}
	return &automationdomain.ScheduledTask{JobID: jobID}, nil
}

func newInterval(v int) *int { return &v }

func callTool(t *testing.T, svc contract.Service, sctx contract.ServerContext, name string, args map[string]any) (map[string]any, bool) {
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

func listTools(t *testing.T, svc contract.Service, sctx contract.ServerContext) []map[string]any {
	t.Helper()
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
	wantNames := []string{
		"create_scheduled_task",
		"find_scheduled_tasks",
		"update_scheduled_task",
		"delete_scheduled_task",
		"inspect_scheduled_task",
		"get_scheduled_task_report",
		"run_scheduled_task",
		"repair_scheduled_task",
	}
	if len(tools) != len(wantNames) {
		t.Fatalf("tools count = %d, want %d", len(tools), len(wantNames))
	}
	for index, tool := range tools {
		name, _ := tool["name"].(string)
		if name != wantNames[index] {
			t.Fatalf("tool[%d] = %q, want %q", index, name, wantNames[index])
		}
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
