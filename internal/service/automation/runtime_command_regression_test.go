package automation

import (
	"context"
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRuntimeCommandUpdateRejectsEmptyAndContradictoryControl(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "guarded task", "runtime-guarded-create")
	for _, input := range []automationdomain.AutomationCommandInput{
		{JobID: created.JobID},
		{JobID: created.JobID, RunID: "run-without-cancel"},
		{JobID: created.JobID, CancelActiveRun: true, Enabled: boolCommandPointer(true)},
	} {
		if _, err := fixture.Service.PlanRuntimeCommand(
			context.Background(), actor, automationdomain.AutomationCommandOperationUpdate, input,
		); err == nil {
			t.Fatalf("invalid update was planned: %+v", input)
		}
	}
}

func TestRuntimeCommandCancelRunDigestFencesRuntimeIdentityBeforeWrite(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "cancel fence", "runtime-cancel-fence-create")
	running := *created
	running.Running = true
	running.RunningRunID = "run-old"
	fixture.Service.replaceJobRuntimeState(running)
	input := automationdomain.AutomationCommandInput{
		JobID: created.JobID, Name: "must-not-commit", CancelActiveRun: true,
	}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationUpdate, input,
	)
	if err != nil || plan.Input.RunID != "run-old" {
		t.Fatalf("cancel plan = %+v err=%v", plan, err)
	}
	running.RunningRunID = "run-new"
	fixture.Service.replaceJobRuntimeState(running)
	_, err = fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor,
		automationdomain.AutomationCommandRequest{
			Action: automationdomain.AutomationCommandActionApply, Operation: automationdomain.AutomationCommandOperationUpdate,
			Input: input, RequestID: "runtime-cancel-fence-apply",
			ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
		},
		RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "approval-cancel-fence"},
	)
	if err == nil || !strings.Contains(err.Error(), "plan_digest") {
		t.Fatalf("stale active run error = %v", err)
	}
	stored, loadErr := fixture.Service.GetTask(runtimeAutomationCommandContext(context.Background(), actor), created.JobID)
	if loadErr != nil || stored.Name == "must-not-commit" || stored.RunningRunID != "run-new" {
		t.Fatalf("stale cancel mutated task: %+v err=%v", stored, loadErr)
	}
}

func TestRuntimeCommandDeletedHistoryAcceptsExactJobID(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "deleted history", "runtime-deleted-history-create")
	ownerCtx := runtimeAutomationCommandContext(context.Background(), actor)
	if _, err := fixture.Service.DeleteTaskAtVersion(ownerCtx, created.JobID, created.ConfigurationVersion); err != nil {
		t.Fatalf("DeleteTaskAtVersion() error = %v", err)
	}
	for _, operation := range []string{
		automationdomain.AutomationCommandOperationRuns,
		automationdomain.AutomationCommandOperationEvents,
	} {
		if _, err := fixture.Service.InspectRuntimeCommand(
			context.Background(), actor, operation,
			automationdomain.AutomationCommandInput{JobID: created.JobID},
		); err != nil {
			t.Fatalf("deleted %s by exact job_id: %v", operation, err)
		}
	}
}

func TestRuntimeCommandHistoryFiltersEnabledBeforeLimit(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	disabled := createRuntimeCommandTask(t, fixture, actor, "filter older", "runtime-filter-disabled-create")
	ownerCtx := runtimeAutomationCommandContext(context.Background(), actor)
	if _, err := fixture.Service.UpdateTaskAtVersion(
		ownerCtx, disabled.JobID, disabled.ConfigurationVersion,
		automationdomain.UpdateJobInput{Enabled: boolCommandPointer(false)},
	); err != nil {
		t.Fatalf("disable task: %v", err)
	}
	_ = createRuntimeCommandTask(t, fixture, actor, "filter newer", "runtime-filter-enabled-create")
	result, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationList,
		automationdomain.AutomationCommandInput{
			Query: "filter", IncludeDeleted: true, Enabled: boolCommandPointer(false), Limit: 1,
		},
	)
	items, ok := result.([]automationdomain.ScheduledTaskHistoryItem)
	if err != nil || !ok || len(items) != 1 || items[0].JobID != disabled.JobID {
		t.Fatalf("enabled-before-limit result = %#v err=%v", result, err)
	}
}

func TestRuntimeCommandExternalSessionDefaultsStayInCurrentConversation(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	ownerCtx := runtimeAutomationCommandContext(context.Background(), fixture.ServerContext)
	firstSession := protocol.BuildAgentAccountSessionKey(
		"agent-1", protocol.SessionChannelWeixinPersonal, protocol.RoomTypeDM,
		"account-1", "user-1", "",
	)
	secondSession := protocol.BuildAgentAccountSessionKey(
		"agent-1", protocol.SessionChannelWeixinPersonal, protocol.RoomTypeDM,
		"account-1", "user-2", "",
	)
	first := createRuntimeSessionTask(t, fixture.Service, ownerCtx, firstSession, "first conversation")
	_ = createRuntimeSessionTask(t, fixture.Service, ownerCtx, secondSession, "second conversation")
	external := fixture.ServerContext
	external.SessionKey = firstSession
	external.LeaseSessionKey = firstSession
	external.SourceContextType = "agent_paired"
	external.SourceContextID = "user-1"
	for _, query := range []string{"", "这里的任务"} {
		result, err := fixture.Service.InspectRuntimeCommand(
			context.Background(), external, automationdomain.AutomationCommandOperationList,
			automationdomain.AutomationCommandInput{Query: query},
		)
		items, ok := result.([]automationdomain.ScheduledTask)
		if err != nil || !ok || len(items) != 1 || items[0].JobID != first.JobID {
			t.Fatalf("external list query %q = %#v err=%v", query, result, err)
		}
	}
	historyResult, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), external, automationdomain.AutomationCommandOperationList,
		automationdomain.AutomationCommandInput{IncludeDeleted: true, Limit: 1},
	)
	history, ok := historyResult.([]automationdomain.ScheduledTaskHistoryItem)
	if err != nil || !ok || len(history) != 1 || history[0].JobID != first.JobID {
		t.Fatalf("external history escaped current conversation: %#v err=%v", historyResult, err)
	}
	contract, err := fixture.Service.RuntimeCommandContract(context.Background(), external)
	if err != nil {
		t.Fatal(err)
	}
	if _, exposed := contract.Operations[automationdomain.AutomationCommandOperationHeartbeat]; exposed {
		t.Fatalf("external contract exposed heartbeat: %+v", contract)
	}
	if _, err = fixture.Service.InspectRuntimeCommand(
		context.Background(), external, automationdomain.AutomationCommandOperationHeartbeat,
		automationdomain.AutomationCommandInput{},
	); err == nil {
		t.Fatal("external IM inspected heartbeat configuration")
	}
	report, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), external, automationdomain.AutomationCommandOperationReport,
		automationdomain.AutomationCommandInput{},
	)
	daily, ok := report.(*automationdomain.ScheduledTaskDailyReport)
	if err != nil || !ok || daily.Totals.TaskCount > 1 {
		t.Fatalf("external report escaped current conversation: %#v err=%v", report, err)
	}
}

func createRuntimeSessionTask(
	t *testing.T,
	service *Service,
	ctx context.Context,
	sessionKey string,
	name string,
) *automationdomain.ScheduledTask {
	t.Helper()
	task, err := service.CreateTask(ctx, automationdomain.CreateJobInput{
		Name: name, AgentID: "agent-1", Instruction: "do work",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source: automationdomain.Source{
			Kind: automationdomain.SourceKindAgent, CreatorAgentID: "agent-1",
			ContextType: "agent", ContextID: "agent-1", SessionKey: sessionKey,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return task
}
