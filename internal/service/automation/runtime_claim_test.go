// runtime_claim_test.go 覆盖自动化任务认领、运行时命令、调度策略与删除认领测试。
package automation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	_ "modernc.org/sqlite"
)

func TestScriptJobExternalClaimDoesNotRecordSkippedRun(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:          "脚本领取防重",
		AgentID:       "agent-1",
		Instruction:   "echo should-not-run",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		OverlapPolicy: automationdomain.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	startedAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	nextRunAt := startedAt.Add(time.Hour)
	claimed, err := service.repository.ClaimScheduledTaskRuntime(context.Background(), automationstore.JobRuntimeClaimInput{
		JobID:         task.JobID,
		RunID:         "run-script-1",
		StartedAt:     startedAt,
		NextRunAt:     &nextRunAt,
		OverlapPolicy: automationdomain.OverlapPolicySkip,
	})
	if err != nil {
		t.Fatalf("脚本任务外部领取失败: %v", err)
	}
	if !claimed {
		t.Fatal("脚本任务外部领取应成功")
	}
	result, err := service.startJobExecution(context.Background(), *task, automationdomain.TriggerKindScheduled, startedAt.Add(2*time.Second))
	if err != nil {
		t.Fatalf("脚本任务外部领取后触发失败: %v", err)
	}
	if result == nil || result.Status != automationdomain.RunStatusRunning || result.RunID == nil || *result.RunID != "run-script-1" {
		t.Fatalf("脚本任务外部领取后的触发结果 = %+v, 期望 running/run-script-1", result)
	}
	var runCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM automation_task_runs WHERE job_id = ?`, task.JobID).Scan(&runCount); err != nil {
		t.Fatalf("读取脚本 run 数量失败: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("脚本任务被其他调度器领取时，本进程不应写入 skipped run，实际 %d", runCount)
	}
}

func TestManualOverlapRequestUsesDatabaseAcrossRuntimeAndScriptPaths(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		executionKind string
	}{
		{name: "agent runtime", executionKind: automationdomain.ExecutionKindAgent},
		{name: "script", executionKind: automationdomain.ExecutionKindScript},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db := newAutomationTestDB(t)
			service := NewService(
				config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
				permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
			)
			task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
				Name: "database overlap " + testCase.name, AgentID: "agent-1", Instruction: "do not dispatch",
				Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
				SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
				Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
				OverlapPolicy: automationdomain.OverlapPolicySkip, Enabled: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`UPDATE automation_scheduled_tasks
SET execution_kind = ?, running_run_id = 'remote-active-run', running_started_at = CURRENT_TIMESTAMP
WHERE job_id = ?`, testCase.executionKind, task.JobID); err != nil {
				t.Fatal(err)
			}
			const requestID = "web-run:cross-path-overlap"
			first, err := service.RunTaskNowAtVersionWithRequest(
				context.Background(), task.JobID, task.ConfigurationVersion, requestID,
			)
			if err != nil || first == nil || first.RunID == nil || first.Status != automationdomain.RunStatusSkipped {
				t.Fatalf("first overlap receipt=%+v err=%v", first, err)
			}
			second, err := service.RunTaskNowAtVersionWithRequest(
				context.Background(), task.JobID, task.ConfigurationVersion, requestID,
			)
			if err != nil || second == nil || second.RunID == nil || *second.RunID != *first.RunID || !second.Replayed {
				t.Fatalf("replayed overlap receipt=%+v err=%v", second, err)
			}
			var runCount int
			if err = db.QueryRow(`SELECT COUNT(*) FROM automation_task_runs
WHERE job_id = ? AND client_request_id = ?`, task.JobID, requestID).Scan(&runCount); err != nil || runCount != 1 {
				t.Fatalf("request run count=%d err=%v", runCount, err)
			}
		})
	}
}

type fixedAutomationDeliveryAuthority struct {
	sessions []channels.AgentExternalSession
}

func (f fixedAutomationDeliveryAuthority) ValidateExternalSessionGrant(
	context.Context,
	string,
	string,
	string,
) error {
	return nil
}

func (f fixedAutomationDeliveryAuthority) ListAgentExternalSessions(
	context.Context,
	string,
	string,
	string,
) ([]channels.AgentExternalSession, error) {
	return append([]channels.AgentExternalSession(nil), f.sessions...), nil
}

type fixedAutomationDeliverySessionResolver map[string]protocol.Session

func (f fixedAutomationDeliverySessionResolver) ResolveDeliverySession(
	_ context.Context,
	sessionKey string,
) (*protocol.Session, error) {
	item, ok := f[sessionKey]
	if !ok {
		return nil, nil
	}
	return &item, nil
}

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

func TestRuntimeCommandSelectsSameAgentPairedDMSession(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	sessionKey := protocol.BuildAgentAccountSessionKey(
		actor.AgentID,
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"weixin-account",
		"weixin-user",
		"",
	)
	fixture.Service.SetDeliveryGrantResolver(fixedAutomationDeliveryAuthority{sessions: []channels.AgentExternalSession{{
		SessionKey: sessionKey,
		Channel:    protocol.SessionChannelWeixinPersonal,
		Label:      "捷哥",
		AgentID:    actor.AgentID,
	}}})
	fixture.Service.SetDeliverySessionResolver(fixedAutomationDeliverySessionResolver{
		sessionKey: {
			SessionKey:  sessionKey,
			AgentID:     actor.AgentID,
			ChannelType: protocol.SessionChannelWeixinPersonal,
			ChatType:    protocol.RoomTypeDM,
			Title:       "捷哥",
		},
	})

	listed, err := fixture.Service.InspectRuntimeCommand(
		context.Background(),
		actor,
		automationdomain.AutomationCommandOperationDeliveryTargets,
		automationdomain.AutomationCommandInput{DeliveryChannel: protocol.SessionChannelWeixinPersonal},
	)
	if err != nil {
		t.Fatal(err)
	}
	targets, ok := listed.([]automationdomain.AutomationCommandDeliverySession)
	if !ok || len(targets) != 1 || targets[0].SessionKey != sessionKey || targets[0].Label != "捷哥" {
		t.Fatalf("delivery_targets = %#v", listed)
	}

	input := automationdomain.AutomationCommandInput{
		Name: "微信提醒", Instruction: "提醒捷哥跟进客户",
		Schedule: &automationdomain.AutomationCommandSchedule{
			Kind: "daily", DailyTime: "09:00", Timezone: "Asia/Shanghai",
		},
		DeliverySessionKey: sessionKey,
	}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationCreate, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(),
		actor,
		automationdomain.AutomationCommandRequest{
			Action: automationdomain.AutomationCommandActionApply, Operation: automationdomain.AutomationCommandOperationCreate,
			Input: input, RequestID: "runtime-weixin-delivery",
			ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
		},
		RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatal(err)
	}
	created := result.Data.(*automationdomain.ScheduledTask)
	if created.Delivery.Mode != automationdomain.DeliveryModeLast ||
		created.Delivery.SessionKey != sessionKey ||
		created.DeliveryGrant.SessionKey != actor.SessionKey {
		t.Fatalf("created delivery = %+v grant=%+v", created.Delivery, created.DeliveryGrant)
	}
	otherAgentInput := input
	otherAgentInput.DeliverySessionKey = protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"other-user",
		"",
	)
	if _, err = fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationCreate, otherAgentInput,
	); err == nil {
		t.Fatal("ordinary Agent unexpectedly selected another Agent delivery Session")
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

func TestRuntimeCommandReplayRecoversAcceptedRunAfterReceiptLoss(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "accepted run", "runtime-accepted-create")
	request := automationdomain.AutomationCommandRequest{
		Action: automationdomain.AutomationCommandActionApply, Operation: automationdomain.AutomationCommandOperationRun,
		Input: automationdomain.AutomationCommandInput{JobID: created.JobID}, RequestID: "runtime-run-receipt-loss",
	}
	intentDigest, err := runtimeAutomationIntentDigest(actor, request.Operation, request.Input)
	if err != nil {
		t.Fatal(err)
	}
	if _, isNew, claimErr := fixture.Service.repository.ClaimRuntimeCommand(
		context.Background(),
		automationstore.RuntimeCommandRecord{
			OwnerUserID: actor.OwnerUserID, RequestID: request.RequestID,
			ActorAgentID: actor.AgentID, Operation: request.Operation, IntentDigest: intentDigest,
			ApprovalRequestID: "approval-receipt-loss",
		},
	); claimErr != nil || !isNew {
		t.Fatalf("claim runtime command isNew=%v err=%v", isNew, claimErr)
	}
	if err = fixture.Service.repository.InsertRunPending(
		context.Background(),
		automationstore.RunPendingInput{
			RunID: "run-receipt-loss", JobID: created.JobID, OwnerUserID: created.OwnerUserID,
			Status:          automationdomain.RunStatusRunning,
			ClientRequestID: request.RequestID, IntentDigest: intentDigest,
			PermissionPolicyRevision: created.PermissionPolicy.Revision,
		},
	); err != nil {
		t.Fatal(err)
	}
	replayed, found, err := fixture.Service.ReplayRuntimeCommand(context.Background(), actor, request)
	if err != nil || !found || replayed == nil || replayed.Outcome != "replayed" {
		t.Fatalf("receipt-loss replay=%+v found=%v err=%v", replayed, found, err)
	}
	run, ok := replayed.Data.(*automationdomain.ExecutionResult)
	if !ok || run.RunID == nil || *run.RunID != "run-receipt-loss" {
		t.Fatalf("replayed run=%#v", replayed.Data)
	}
	record, err := fixture.Service.repository.GetRuntimeCommand(context.Background(), actor.OwnerUserID, request.RequestID)
	if err != nil || record.Status != "applied" {
		t.Fatalf("reconciled command=%+v err=%v", record, err)
	}
}

func createRuntimeCommandTask(
	t *testing.T,
	fixture automationCommandFixture,
	actor command.Actor,
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

func TestSchedulerLeaseAllowsSingleLeaderAndExpiryTakeover(t *testing.T) {
	db := newAutomationTestDB(t)
	newService := func(ownerID string) *Service {
		service := NewService(
			config.Config{DatabaseDriver: "sqlite", AutomationSchedulerLeaseSeconds: 3},
			db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
		service.schedulerOwnerID = ownerID
		return service
	}

	first := newService("scheduler-a")
	second := newService("scheduler-b")
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	held, becameLeader, err := first.refreshSchedulerLease(context.Background(), now)
	if err != nil || !held || !becameLeader {
		t.Fatalf("第一个实例应获取租约: held=%v became=%v err=%v", held, becameLeader, err)
	}
	held, becameLeader, err = second.refreshSchedulerLease(context.Background(), now.Add(time.Second))
	if err != nil || held || becameLeader {
		t.Fatalf("租约有效期内第二个实例不应接管: held=%v became=%v err=%v", held, becameLeader, err)
	}
	held, becameLeader, err = second.refreshSchedulerLease(context.Background(), now.Add(4*time.Second))
	if err != nil || !held || !becameLeader {
		t.Fatalf("租约过期后第二个实例应接管: held=%v became=%v err=%v", held, becameLeader, err)
	}
	held, becameLeader, err = first.refreshSchedulerLease(context.Background(), now.Add(4*time.Second))
	if err != nil || held || becameLeader {
		t.Fatalf("原 leader 不应覆盖新租约: held=%v became=%v err=%v", held, becameLeader, err)
	}
}

func TestSchedulerSkipsMisfireAndAdvancesFromNow(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission}
	service := NewService(
		config.Config{
			DatabaseDriver:                "sqlite",
			AutomationMisfirePolicy:       "skip",
			AutomationMisfireGraceSeconds: 10,
		},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	task, err := service.CreateTask(context.Background(), schedulerPolicyTaskInput("错过窗口", true))
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	scheduledFor := now.Add(-5 * time.Minute)
	service.mu.Lock()
	service.jobStates[task.JobID].NextRunAt = &scheduledFor
	service.mu.Unlock()

	service.runDueOnce()
	if len(dm.Requests()) != 0 {
		t.Fatalf("misfire_policy=skip 不应启动 Agent: %+v", dm.Requests())
	}
	runs, err := service.repository.ListRunsByJob(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil {
		t.Fatalf("ListRunsByJob 失败: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != automationdomain.RunStatusSkipped || runs[0].TriggerKind != automationdomain.TriggerKindMisfire {
		t.Fatalf("应记录一条 misfire skipped run: %+v", runs)
	}
	current, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	wantNext := now.Add(time.Hour)
	if current.NextRunAt == nil || !current.NextRunAt.Equal(wantNext) {
		t.Fatalf("跳过后应从当前时间推进调度: got=%v want=%s", current.NextRunAt, wantNext)
	}
}

func TestScheduledTaskReadsDoNotPersistPastOneShotDisable(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	runAt := now.Add(time.Minute)
	input := schedulerPolicyTaskInput("只读的一次性任务", true)
	runAtText := runAt.Format(time.RFC3339)
	input.Schedule = automationdomain.Schedule{
		Kind:     automationdomain.ScheduleKindAt,
		RunAt:    &runAtText,
		Timezone: "UTC",
	}
	task, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = db.ExecContext(
		context.Background(),
		"UPDATE automation_scheduled_tasks SET next_run_at = NULL WHERE job_id = ?",
		task.JobID,
	); err != nil {
		t.Fatalf("准备无 next_run_at 的持久快照失败: %v", err)
	}
	service.mu.Lock()
	delete(service.jobStates, task.JobID)
	service.mu.Unlock()
	now = runAt.Add(time.Second)

	before, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || before == nil {
		t.Fatalf("读取测试前快照失败: task=%+v err=%v", before, err)
	}
	if _, err = service.ListTasks(context.Background(), ""); err != nil {
		t.Fatalf("ListTasks 失败: %v", err)
	}
	if _, err = service.GetTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	// 旧实现会在读取返回前登记异步整行 Upsert；等待后台任务可稳定捕获该回归。
	service.wg.Wait()
	after, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || after == nil {
		t.Fatalf("读取测试后快照失败: task=%+v err=%v", after, err)
	}
	if !after.Enabled || after.ConfigurationVersion != before.ConfigurationVersion {
		t.Fatalf("List/Get 只能投影，不能停用或推进版本: before=%+v after=%+v", before, after)
	}
}

func TestScheduledTaskDisableCASPreservesConcurrentConfiguration(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	expiresAt := now.Add(time.Minute)
	input := schedulerPolicyTaskInput("旧名称", true)
	input.ExpiresAt = &expiresAt
	task, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	stale := *task
	updatedInput := stale
	updatedInput.Name = "用户刚保存的新名称"
	updatedInput.Instruction = "使用新的用户配置执行"
	updated, err := service.repository.UpdateScheduledTaskAtVersion(
		context.Background(),
		updatedInput,
		stale.ConfigurationVersion,
	)
	if err != nil {
		t.Fatalf("模拟并发配置更新失败: %v", err)
	}
	now = expiresAt

	changed, err := service.repository.DisableScheduledTaskAtVersion(
		context.Background(),
		stale.OwnerUserID,
		stale.JobID,
		stale.ConfigurationVersion,
	)
	if changed || !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("旧版本自动停用必须冲突: changed=%v err=%v", changed, err)
	}
	if err = service.disableScheduledTaskAtVersion(
		context.Background(),
		scheduledTaskDisableCandidate{job: stale, reason: scheduledTaskDisableReasonExpired},
		now,
	); err != nil {
		t.Fatalf("scheduler 应安全吸收版本冲突并刷新投影: %v", err)
	}
	current, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || current == nil {
		t.Fatalf("读取并发更新后的任务失败: task=%+v err=%v", current, err)
	}
	if !current.Enabled || current.ConfigurationVersion != updated.ConfigurationVersion ||
		current.Name != updated.Name || current.Instruction != updated.Instruction {
		t.Fatalf("旧 scheduler 快照覆盖了用户配置: want=%+v got=%+v", updated, current)
	}
	service.mu.Lock()
	projected := service.jobStates[task.JobID]
	if projected == nil || projected.Job.ConfigurationVersion != updated.ConfigurationVersion ||
		projected.Job.Name != updated.Name {
		service.mu.Unlock()
		t.Fatalf("冲突后应刷新到权威配置: state=%+v updated=%+v", projected, updated)
	}
	service.mu.Unlock()

	service.runDueOnce()
	resolved, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || resolved == nil {
		t.Fatalf("读取重新核对后的任务失败: task=%+v err=%v", resolved, err)
	}
	if resolved.Enabled || resolved.ConfigurationVersion != updated.ConfigurationVersion+1 ||
		resolved.Name != updated.Name || resolved.Instruction != updated.Instruction {
		t.Fatalf("scheduler 应用新版本停用时仍须保留用户配置: updated=%+v resolved=%+v", updated, resolved)
	}
}

func schedulerPolicyTaskInput(name string, enabled bool) automationdomain.CreateJobInput {
	return automationdomain.CreateJobInput{
		Name:        name,
		AgentID:     "agent-1",
		Instruction: "执行检查",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       enabled,
	}
}

func TestOwnerConfirmedStoppedFinalizesReviewDeletionAtExactVersion(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil)
	task := createDeletionClaimTestTask(t, service)
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	runID := "review-confirm-run"
	requestID := "review-confirm-permission"
	if err := service.repository.InsertRunPending(ownerCtx, automationstore.RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		Status:         automationdomain.RunStatusRunning,
		SessionKey:     protocol.BuildAgentSessionKey(task.AgentID, "automation", "dm", "review-confirm", ""),
		RoundID:        "remote-round",
		DeliveryStatus: automationdomain.DeliveryStatusPending,
	}); err != nil {
		t.Fatalf("InsertRunPending(): %v", err)
	}
	if _, err := db.Exec(`UPDATE automation_task_runs
SET block_state = 'awaiting_approval', blocked_request_id = ?
WHERE run_id = ?`, requestID, runID); err != nil {
		t.Fatalf("block active run: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO automation_permission_requests (
request_id, owner_user_id, job_id, run_id, policy_revision, kind, status,
tool_name, effect, input_fingerprint, capability_json, input_summary_json
) VALUES (?, ?, ?, ?, 1, 'tool', 'pending', 'WebSearch', 'read', 'review-confirm', '{}', '{}')`,
		requestID, task.OwnerUserID, task.JobID, runID); err != nil {
		t.Fatalf("seed permission request: %v", err)
	}

	if _, err := service.DeleteTaskAtVersion(ownerCtx, task.JobID, task.ConfigurationVersion); !TaskDeletionPrepared(err) || !errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
		t.Fatalf("DeleteTaskAtVersion() must stop at review_required: %v", err)
	}
	claimed, err := service.repository.GetScheduledTask(ownerCtx, task.OwnerUserID, task.JobID)
	if err != nil || claimed == nil || claimed.DeletionState != automationdomain.TaskDeletionStateReviewRequired {
		t.Fatalf("review claim missing: task=%+v err=%v", claimed, err)
	}
	if _, err = service.DeleteTaskAtVersion(ownerCtx, task.JobID, claimed.ConfigurationVersion); !TaskDeletionPrepared(err) || !errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
		t.Fatalf("ordinary repeated DELETE must remain fail-closed: %v", err)
	}
	if _, err = service.ConfirmTaskDeletionStoppedAtVersion(ownerCtx, task.JobID, claimed.ConfigurationVersion-1); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale confirmation error=%v, want version conflict", err)
	}
	otherOwnerCtx := contextForOwner(context.Background(), "different-owner")
	if _, err = service.ConfirmTaskDeletionStoppedAtVersion(otherOwnerCtx, task.JobID, claimed.ConfigurationVersion); !errors.Is(err, automationdomain.ErrJobNotFound) {
		t.Fatalf("cross-owner confirmation error=%v, want not found", err)
	}

	result, err := service.ConfirmTaskDeletionStoppedAtVersion(ownerCtx, task.JobID, claimed.ConfigurationVersion)
	if err != nil {
		t.Fatalf("ConfirmTaskDeletionStoppedAtVersion(): %v", err)
	}
	if result == nil || !result.Deleted || !result.CancelledActiveRun || result.CancelledRunID != runID {
		t.Fatalf("unexpected confirmation result: %+v", result)
	}
	current, err := service.repository.GetScheduledTask(ownerCtx, task.OwnerUserID, task.JobID)
	if err != nil || current != nil {
		t.Fatalf("confirmed review task still exists: task=%+v err=%v", current, err)
	}
	run, err := service.repository.GetRun(ownerCtx, task.OwnerUserID, task.JobID, runID)
	if err != nil || run == nil || run.Status != automationdomain.RunStatusCancelled ||
		run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted || run.DeliveryDeadLetterAt == nil ||
		run.BlockState != "" || run.BlockedRequestID != "" {
		t.Fatalf("active run was not atomically closed: run=%+v err=%v", run, err)
	}
	request, err := service.repository.GetPermissionRequest(ownerCtx, task.OwnerUserID, requestID)
	if err != nil || request.Status != automationdomain.PermissionRequestStatusSuperseded {
		t.Fatalf("pending permission was not superseded: request=%+v err=%v", request, err)
	}
	events, err := service.repository.ListTaskEventsByJob(ownerCtx, task.OwnerUserID, task.JobID, 10)
	if err != nil || len(events) == 0 {
		t.Fatalf("confirmed deletion event missing: events=%+v err=%v", events, err)
	}
	var detail map[string]any
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionDelete {
			detail = event.Detail
			break
		}
	}
	if detail == nil {
		t.Fatalf("confirmed deletion event missing from event history: %+v", events)
	}
	if detail["execution_stop_confirmed_by_owner"] != true ||
		detail["review_required_finalized"] != true || detail["external_actions_replayed"] != false {
		t.Fatalf("confirmed deletion audit omitted safety facts: %+v", detail)
	}
}

func TestReviewRequiredDeletionFinalizesAfterRemoteScriptDurablySettles(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	newService := func() *Service {
		return NewService(
			config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath, AppMode: "desktop"},
			db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
	}
	executionOwner := newService()
	deleteReceiver := newService()
	startedName := "remote-review-script-started"
	finishedName := "remote-review-script-finished"
	script := "touch " + startedName + "; sleep 0.5; touch " + finishedName
	if runtime.GOOS == "windows" {
		script = "echo started>" + startedName + " & timeout /T 1 /NOBREAK >NUL & echo finished>" + finishedName
	}
	task, err := executionOwner.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "remote settling script", AgentID: "agent-1", Instruction: script,
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	result, err := executionOwner.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result == nil || result.RunID == nil {
		t.Fatalf("RunTaskNow(): result=%+v err=%v", result, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		_, statErr := os.Stat(filepath.Join(workspacePath, startedName))
		return statErr == nil
	})
	if _, err = deleteReceiver.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); !TaskDeletionPrepared(err) ||
		!errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
		t.Fatalf("non-owner delete must wait for durable terminal proof: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		run, loadErr := executionOwner.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, *result.RunID)
		return loadErr == nil && run != nil && run.Status == automationdomain.RunStatusSucceeded &&
			run.DeliveryStatus == automationdomain.DeliveryStatusNotAttempted && run.DeliveryDeadLetterAt != nil
	})
	if _, statErr := os.Stat(filepath.Join(workspacePath, finishedName)); statErr != nil {
		t.Fatalf("pre-existing remote script did not naturally settle: %v", statErr)
	}
	if err = deleteReceiver.bootstrapRuntime(context.Background()); err != nil {
		t.Fatalf("bootstrapRuntime(): %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		persisted, loadErr := deleteReceiver.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
		return loadErr == nil && persisted == nil
	})
}

func createDeletionClaimTestTask(t *testing.T, service *Service) *automationdomain.ScheduledTask {
	t.Helper()
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "durable delete", AgentID: "agent-1", Instruction: "delete safely",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetMain},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	return task
}
