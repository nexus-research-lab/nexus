package automation

import (
	"context"
	"errors"
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
)

func TestAutomationCommandCronScheduleAcceptsStandardFiveField(t *testing.T) {
	for _, expression := range []string{"0 9 15 * *", "*/15 9-17 * * 1-5"} {
		schedule, err := automationCommandCronSchedule(expression, "Asia/Shanghai")
		if err != nil {
			t.Fatalf("automationCommandCronSchedule(%q): %v", expression, err)
		}
		if schedule.CronExpression == nil || *schedule.CronExpression != expression {
			t.Fatalf("cron_expression = %#v, want %q", schedule.CronExpression, expression)
		}
	}
	if _, err := automationCommandCronSchedule("0 25 15 * *", "Asia/Shanghai"); err == nil {
		t.Fatal("invalid cron expression unexpectedly accepted")
	}
	fixture := newAutomationCommandFixture(t, "ok")
	input := automationConfigurationTaskInput("invalid cron")
	expression := "0 25 15 * *"
	input.Schedule = automationdomain.Schedule{
		Kind: automationdomain.ScheduleKindCron, CronExpression: &expression, Timezone: "Asia/Shanghai",
	}
	if _, err := fixture.Service.CreateTask(context.Background(), input); err == nil {
		t.Fatal("CreateTask unexpectedly persisted invalid cron expression")
	}
}

func TestRuntimeCommandCreateUsesPlanConfirmationAndIdempotency(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	input := automationdomain.AutomationCommandInput{
		Name: "CLI 日报", Instruction: "生成工作日报",
		Schedule: &automationdomain.AutomationCommandSchedule{
			Kind: "daily", DailyTime: "09:00", Timezone: "Asia/Shanghai",
		},
	}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationCreate, input,
	)
	if err != nil {
		t.Fatalf("PlanRuntimeCommand(create): %v", err)
	}
	if !plan.RequiresConfirmation || plan.PlanDigest == "" || plan.CurrentRevision != "new:agent-1" {
		t.Fatalf("create plan = %+v", plan)
	}
	request := automationdomain.AutomationCommandRequest{
		Action:    automationdomain.AutomationCommandActionApply,
		Operation: automationdomain.AutomationCommandOperationCreate,
		Input:     input, RequestID: "runtime-create-stable",
		ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	}
	if _, err = fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, request, RuntimeCommandApplyOptions{},
	); err == nil || !strings.Contains(err.Error(), "真人确认") {
		t.Fatalf("unconfirmed create error = %v", err)
	}
	first, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, request, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatalf("ApplyRuntimeCommand(create): %v", err)
	}
	created, ok := first.Data.(*automationdomain.ScheduledTask)
	if !ok || created == nil || created.JobID == "" || created.Source.CreatorAgentID != actor.AgentID {
		t.Fatalf("created task = %#v", first.Data)
	}
	commandRecord, err := fixture.Service.repository.GetRuntimeCommand(
		context.Background(), actor.OwnerUserID, request.RequestID,
	)
	if err != nil || commandRecord.ApprovalRequestID != "test-approval" {
		t.Fatalf("command approval audit = %+v err=%v", commandRecord, err)
	}
	replayed, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, request, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatalf("replayed create: %v", err)
	}
	replayedTask, ok := replayed.Data.(*automationdomain.ScheduledTask)
	if !ok || replayedTask.JobID != created.JobID {
		t.Fatalf("create replay = %#v, want job %s", replayed.Data, created.JobID)
	}
	conflicting := request
	conflicting.Input.Name = "different intent"
	if _, err = fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, conflicting, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	); !errors.Is(err, automationdomain.ErrRuntimeCommandConflict) {
		t.Fatalf("conflicting request_id error = %v", err)
	}
	listed, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationList,
		automationdomain.AutomationCommandInput{Query: "CLI 日报"},
	)
	if err != nil {
		t.Fatalf("InspectRuntimeCommand(list): %v", err)
	}
	items, ok := listed.([]automationdomain.ScheduledTask)
	if !ok || len(items) != 1 || items[0].JobID != created.JobID {
		t.Fatalf("listed tasks = %#v", listed)
	}
}

func TestRuntimeCommandScopesCrossAgentAndBackgroundRun(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	if _, err := runtimeCommandAgentID(actor, "agent-2"); err == nil {
		t.Fatal("ordinary Agent unexpectedly received cross-Agent authority")
	}
	main := actor
	main.IsMainAgent = true
	if target, err := runtimeCommandAgentID(main, "agent-2"); err != nil || target != "agent-2" {
		t.Fatalf("main cross-Agent target = %q err=%v", target, err)
	}

	created := createRuntimeCommandTask(t, fixture, actor, "background task", "runtime-background-create")
	background := actor
	background.SourceContextType = "automation_run"
	background.CurrentJobID = created.JobID
	background.CurrentRunID = "run-current"
	if background.MutationAllowed() {
		t.Fatal("background scheduled run unexpectedly received mutation authority")
	}
	if _, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), background, automationdomain.AutomationCommandOperationDelete,
		automationdomain.AutomationCommandInput{JobID: created.JobID},
	); err == nil {
		t.Fatal("background scheduled run planned a mutation")
	}
	if _, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), background, automationdomain.AutomationCommandOperationGet,
		automationdomain.AutomationCommandInput{},
	); err != nil {
		t.Fatalf("background scheduled run could not inspect bound task: %v", err)
	}
	if _, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), background, automationdomain.AutomationCommandOperationGet,
		automationdomain.AutomationCommandInput{JobID: "other-job"},
	); err == nil {
		t.Fatal("background scheduled run inspected another task")
	}
}

func TestRuntimeCommandApplyRejectsStaleTaskRevision(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "versioned task", "runtime-version-create")
	input := automationdomain.AutomationCommandInput{JobID: created.JobID, Enabled: boolCommandPointer(false)}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationUpdate, input,
	)
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	name := "concurrent update"
	if _, err = fixture.Service.UpdateTaskAtVersion(
		runtimeAutomationCommandContext(context.Background(), actor),
		created.JobID,
		created.ConfigurationVersion,
		automationdomain.UpdateJobInput{Name: &name},
	); err != nil {
		t.Fatalf("concurrent update: %v", err)
	}
	_, err = fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor,
		automationdomain.AutomationCommandRequest{
			Action:    automationdomain.AutomationCommandActionApply,
			Operation: automationdomain.AutomationCommandOperationUpdate,
			Input:     input, RequestID: "runtime-update-stale",
			ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
		},
		RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err == nil || !strings.Contains(err.Error(), "状态已变化") {
		t.Fatalf("stale update error = %v", err)
	}
}

func TestRuntimeCommandReplayDoesNotDuplicateRunOrWake(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "replay task", "runtime-replay-create")
	runInput := automationdomain.AutomationCommandInput{JobID: created.JobID}
	runPlan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationRun, runInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	runRequest := automationdomain.AutomationCommandRequest{
		Action:    automationdomain.AutomationCommandActionApply,
		Operation: automationdomain.AutomationCommandOperationRun,
		Input:     runInput, RequestID: "runtime-run-replay",
		ExpectedRevision: runPlan.CurrentRevision, PlanDigest: runPlan.PlanDigest,
	}
	first, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, runRequest, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, runRequest, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil || second.Outcome != "replayed" {
		t.Fatalf("run replay = %+v err=%v", second, err)
	}
	firstRun := first.Data.(*automationdomain.ExecutionResult)
	secondRun := second.Data.(*automationdomain.ExecutionResult)
	if firstRun.RunID == nil || secondRun.RunID == nil || *firstRun.RunID != *secondRun.RunID {
		t.Fatalf("run replay IDs: first=%+v second=%+v", firstRun, secondRun)
	}

	wakeInput := automationdomain.AutomationCommandInput{Mode: automationdomain.WakeModeNextHeartbeat}
	wakePlan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationWake, wakeInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	wakeRequest := automationdomain.AutomationCommandRequest{
		Action:    automationdomain.AutomationCommandActionApply,
		Operation: automationdomain.AutomationCommandOperationWake,
		Input:     wakeInput, RequestID: "runtime-wake-replay",
		ExpectedRevision: wakePlan.CurrentRevision, PlanDigest: wakePlan.PlanDigest,
	}
	if _, err = fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, wakeRequest, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	); err != nil {
		t.Fatal(err)
	}
	if replayed, replayErr := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, wakeRequest, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	); replayErr != nil || replayed.Outcome != "replayed" {
		t.Fatalf("wake replay = %+v err=%v", replayed, replayErr)
	}
	fixture.Service.mu.Lock()
	wakeCount := 0
	for _, requests := range fixture.Service.wakeRequests {
		wakeCount += len(requests)
	}
	fixture.Service.mu.Unlock()
	if wakeCount != 1 {
		t.Fatalf("wake requests = %d, want 1", wakeCount)
	}
}

type runtimeCommandRoundStub struct {
	rounds map[string][]string
}

func (s runtimeCommandRoundStub) GetRunningRoundIDs(sessionKey string) []string {
	return append([]string(nil), s.rounds[sessionKey]...)
}

func TestRuntimeCommandCapabilityReusesSessionTokenAndRequiresUniqueActiveRound(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	resolver := runtimeCommandRoundStub{rounds: map[string][]string{"runtime-session": {"round-1"}}}
	registry := runtimecommand.NewRegistry(resolver)
	actor := fixture.ServerContext
	actor.LeaseSessionKey = "runtime-session"
	actor.LeaseRoundID = "round-1"
	token, err := registry.Issue(actor)
	if err != nil {
		t.Fatalf("IssueRuntimeCommandCapability: %v", err)
	}
	actor.LeaseRoundID = "round-2"
	secondToken, err := registry.Issue(actor)
	if err != nil || secondToken != token {
		t.Fatalf("session token = %q second=%q err=%v", token, secondToken, err)
	}
	resolved, err := registry.Resolve(token)
	if err != nil || resolved.LeaseRoundID != "round-1" {
		t.Fatalf("resolved actor = %+v err=%v", resolved, err)
	}
	resolver.rounds["runtime-session"] = []string{"round-1", "round-2"}
	if _, err = registry.Resolve(token); err == nil ||
		!strings.Contains(err.Error(), "并发 round") {
		t.Fatalf("concurrent resolve error = %v", err)
	}
	resolver.rounds["runtime-session"] = nil
	if _, err = registry.Resolve(token); err == nil ||
		!strings.Contains(err.Error(), "已结束") {
		t.Fatalf("inactive resolve error = %v", err)
	}
}

func createRuntimeCommandTask(
	t *testing.T,
	fixture automationCommandFixture,
	actor runtimecommand.Actor,
	name string,
	requestID string,
) *automationdomain.ScheduledTask {
	t.Helper()
	input := automationdomain.AutomationCommandInput{
		Name: name, Instruction: "do work",
		Schedule: &automationdomain.AutomationCommandSchedule{
			Kind: "interval", IntervalValue: 1, IntervalUnit: "hours",
		},
	}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationCreate, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor,
		automationdomain.AutomationCommandRequest{
			Action:    automationdomain.AutomationCommandActionApply,
			Operation: automationdomain.AutomationCommandOperationCreate,
			Input:     input, RequestID: requestID,
			ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
		},
		RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatal(err)
	}
	task, ok := result.Data.(*automationdomain.ScheduledTask)
	if !ok || task == nil {
		t.Fatalf("created data = %#v", result.Data)
	}
	return task
}

func boolCommandPointer(value bool) *bool { return &value }
