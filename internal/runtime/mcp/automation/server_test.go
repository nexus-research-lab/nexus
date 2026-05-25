package automationmcp

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
)

type stubService struct {
	createInput       protocol.CreateJobInput
	updateInput       protocol.UpdateJobInput
	updateJobID       string
	statusJobID       string
	statusEnabled     bool
	deletedJobID      string
	runNowJobID       string
	created           *protocol.CronJob
	recoverJobID      string
	recoverRunID      string
	redeliverJobID    string
	redeliverRunID    string
	listErr           error
	updateErr         error
	jobs              []protocol.CronJob
	missingJobs       map[string]bool
	listAgentID       string
	runsByJob         map[string][]protocol.CronRun
	eventsByJob       map[string][]protocol.CronTaskEvent
	historyItems      []protocol.CronTaskHistoryItem
	historyInput      protocol.CronTaskHistorySearchInput
	taskStatus        *protocol.CronTaskStatus
	dailyReport       *protocol.CronDailyReport
	dailyReportsByJob map[string]*protocol.CronDailyReport
	dailyInput        protocol.CronDailyReportInput
	dailyInputs       []protocol.CronDailyReportInput
}

func (s *stubService) ListTasks(_ context.Context, agentID string) ([]protocol.CronJob, error) {
	s.listAgentID = agentID
	return s.jobs, s.listErr
}

func (s *stubService) CreateTask(_ context.Context, input protocol.CreateJobInput) (*protocol.CronJob, error) {
	s.createInput = input
	if s.created == nil {
		s.created = &protocol.CronJob{
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

func (s *stubService) UpdateTask(_ context.Context, jobID string, input protocol.UpdateJobInput) (*protocol.CronJob, error) {
	s.updateJobID = jobID
	s.updateInput = input
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	job := protocol.CronJob{
		JobID:    jobID,
		AgentID:  "agent-1",
		Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"},
	}
	if input.Delivery != nil {
		job.Delivery = *input.Delivery
	}
	return &job, nil
}

func (s *stubService) UpdateTaskStatus(_ context.Context, jobID string, enabled bool) (*protocol.CronJob, error) {
	s.statusJobID = jobID
	s.statusEnabled = enabled
	for _, job := range s.jobs {
		if job.JobID == jobID {
			job.Enabled = enabled
			return &job, nil
		}
	}
	return &protocol.CronJob{JobID: jobID, Enabled: enabled, Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"}}, nil
}

func (s *stubService) DeleteTask(_ context.Context, jobID string) (*protocol.DeleteJobResult, error) {
	s.deletedJobID = jobID
	result := &protocol.DeleteJobResult{JobID: jobID, Deleted: true}
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

func (s *stubService) RunTaskNow(_ context.Context, jobID string) (*protocol.ExecutionResult, error) {
	s.runNowJobID = jobID
	return &protocol.ExecutionResult{JobID: jobID, Status: "succeeded"}, nil
}

func (s *stubService) ListTaskRuns(_ context.Context, jobID string) ([]protocol.CronRun, error) {
	if s.runsByJob == nil {
		return nil, nil
	}
	return s.runsByJob[jobID], nil
}

func (s *stubService) ListTaskEvents(_ context.Context, jobID string, _ int) ([]protocol.CronTaskEvent, error) {
	if s.eventsByJob == nil {
		return nil, nil
	}
	return s.eventsByJob[jobID], nil
}

func (s *stubService) SearchTaskHistory(_ context.Context, input protocol.CronTaskHistorySearchInput) ([]protocol.CronTaskHistoryItem, error) {
	s.historyInput = input
	return s.historyItems, nil
}

func (s *stubService) GetTaskStatus(_ context.Context, jobID string, _ int, _ int) (*protocol.CronTaskStatus, error) {
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
	return &protocol.CronTaskStatus{
		Job:          *job,
		Health:       protocol.CronTaskHealth{State: "scheduled"},
		RecentRuns:   s.runsByJob[jobID],
		RecentEvents: s.eventsByJob[jobID],
	}, nil
}

func (s *stubService) GetDailyReport(_ context.Context, input protocol.CronDailyReportInput) (*protocol.CronDailyReport, error) {
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
	return &protocol.CronDailyReport{
		Date:     input.Date,
		Timezone: input.Timezone,
		AgentID:  input.AgentID,
		JobID:    input.JobID,
	}, nil
}

func (s *stubService) RetryRunDelivery(_ context.Context, jobID string, runID string) (*protocol.CronRun, error) {
	s.redeliverJobID = jobID
	s.redeliverRunID = runID
	return &protocol.CronRun{
		JobID:          jobID,
		RunID:          runID,
		Status:         protocol.RunStatusSucceeded,
		DeliveryStatus: protocol.DeliveryStatusSucceeded,
	}, nil
}

func (s *stubService) RecoverTaskRunningRun(_ context.Context, jobID string, runID string) (*protocol.CronJob, error) {
	s.recoverJobID = jobID
	s.recoverRunID = runID
	return &protocol.CronJob{JobID: jobID, AgentID: "agent-1", Schedule: protocol.Schedule{Timezone: "Asia/Shanghai"}}, nil
}

func (s *stubService) GetTask(_ context.Context, jobID string) (*protocol.CronJob, error) {
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
	return &protocol.CronJob{JobID: jobID}, nil
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
