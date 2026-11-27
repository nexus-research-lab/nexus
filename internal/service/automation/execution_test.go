// execution_test.go 覆盖自动化任务运行、目标解析、配置控制、命令 Schema、脚本边界与迁移测试。
package automation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	_ "modernc.org/sqlite"
)

func TestServiceStartRunsDueTask(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	service.nowFn = func() time.Time {
		return time.Now().UTC()
	}

	_, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "定时巡检",
		AgentID:     "agent-1",
		Instruction: "执行自动巡检",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(1),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "scheduler", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if err = service.Start(context.Background()); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer service.Stop()

	waitFor(t, 3*time.Second, func() bool {
		return len(dm.Requests()) > 0
	})
}

func TestServiceDispatchRoomTaskUsesNonInteractivePermissionHandler(t *testing.T) {
	roomRunner := &fakeRoomRunner{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		roomRunner,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	job := automationdomain.ScheduledTask{AgentID: "agent-1", Instruction: "整理 Room 今日进展"}
	if err := service.dispatchJobToSession(context.Background(), job, "", sessionKey, "round-room-1", nil, nil); err != nil {
		t.Fatalf("dispatchJobToSession 失败: %v", err)
	}

	requests := roomRunner.Requests()
	if len(requests) != 1 {
		t.Fatalf("期望 room runner 收到 1 次请求，实际 %d", len(requests))
	}
	if requests[0].SessionKey != sessionKey || requests[0].ConversationID != "conversation-1" {
		t.Fatalf("Room 请求路由不正确: %+v", requests[0])
	}
	if requests[0].PermissionHandler == nil {
		t.Fatal("Room 定时任务请求应使用非交互权限处理器")
	}
	if requests[0].PermissionMode != sdkpermission.ModeDefault {
		t.Fatalf("Room 定时任务请求应由后台权限处理器接管授权，实际 mode=%s", requests[0].PermissionMode)
	}
	if len(requests[0].TargetAgentIDs) != 1 || requests[0].TargetAgentIDs[0] != "agent-1" {
		t.Fatalf("Room 定时任务应显式指定目标成员: %+v", requests[0].TargetAgentIDs)
	}
	decision, err := requests[0].PermissionHandler(context.Background(), sdkpermission.Request{ToolName: "AskUserQuestion"})
	if err != nil {
		t.Fatalf("AskUserQuestion 权限处理失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("Room 后台定时任务不应等待交互式提问: %+v", decision)
	}
}

func TestDeleteTaskCancelsActiveScriptBeforeFinalizing(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath, AppMode: "desktop"},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	startedName := "delete-script-started"
	sideEffectName := "delete-script-side-effect"
	// The delayed write is performed by a background pipeline, not the shell
	// itself. Killing only /bin/sh would let that child outlive deletion.
	script := "touch " + startedName + "; (sleep 1; touch " + sideEffectName + ") | cat >/dev/null & wait"
	if runtime.GOOS == "windows" {
		script = "echo started>" + startedName + " & timeout /T 2 /NOBREAK >NUL & echo side-effect>" + sideEffectName
	}
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "cancel active script", AgentID: "agent-1", Instruction: script,
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result == nil || result.RunID == nil {
		t.Fatalf("RunTaskNow(): result=%+v err=%v", result, err)
	}
	startedPath := filepath.Join(workspacePath, startedName)
	sideEffectPath := filepath.Join(workspacePath, sideEffectName)
	waitFor(t, 2*time.Second, func() bool {
		_, statErr := os.Stat(startedPath)
		return statErr == nil
	})
	deleted, err := service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion)
	if err != nil {
		if !TaskDeletionPrepared(err) || !errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
			t.Fatalf("unconfirmed process-tree cancellation must require review: %v", err)
		}
		claimed, loadErr := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
		if loadErr != nil || claimed == nil || claimed.DeletionState != automationdomain.TaskDeletionStateReviewRequired {
			t.Fatalf("uncertain cancellation must retain review claim: task=%+v err=%v", claimed, loadErr)
		}
		waitDuration := 1200 * time.Millisecond
		if runtime.GOOS == "windows" {
			waitDuration = 2200 * time.Millisecond
		}
		time.Sleep(waitDuration)
		if _, statErr := os.Stat(sideEffectPath); !os.IsNotExist(statErr) {
			t.Fatalf("cancelled process-tree child continued side effects: stat err=%v", statErr)
		}
		return
	}
	if deleted == nil || !deleted.CancelledActiveRun || deleted.CancelledRunID != *result.RunID {
		t.Fatalf("delete result lost the script run stopped by cleanup: %+v", deleted)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, statErr := os.Stat(sideEffectPath); !os.IsNotExist(statErr) {
		t.Fatalf("script continued side effects after delete: stat err=%v", statErr)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, *result.RunID)
	if err != nil || run == nil || run.Status != automationdomain.RunStatusCancelled {
		t.Fatalf("deleted script run was not durably cancelled: run=%+v err=%v", run, err)
	}
}

func TestServiceRunTaskNowNeverFallsBackToHostForServerScript(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	service := NewService(
		config.Config{
			DatabaseDriver:       "sqlite",
			WorkspacePath:        workspacePath,
			AppMode:              "server",
			RuntimeIsolationMode: "off",
		},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:          "server script isolation",
		AgentID:       "agent-1",
		Instruction:   "printf should-not-run > host-fallback-marker.txt",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	if result.Status != automationdomain.RunStatusRunning {
		t.Fatalf("期望脚本任务立即返回 running，实际 %s", result.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil &&
			len(runs) == 1 &&
			runs[0].Status == automationdomain.RunStatusFailed
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取 server 脚本 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].ErrorMessage == nil ||
		!strings.Contains(*runs[0].ErrorMessage, "requires runtime isolation enforce") {
		t.Fatalf("server 脚本应因隔离未启用而失败: %+v", runs[0].ErrorMessage)
	}
	if _, statErr := os.Stat(filepath.Join(workspacePath, "host-fallback-marker.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("server 脚本不应回退到宿主 shell，marker stat=%v", statErr)
	}
}

func TestCreateTaskRequestIDIsIdempotentBeforeCapacityCheck(t *testing.T) {
	service := NewService(
		config.Config{
			DatabaseDriver:                   "sqlite",
			AutomationMaxEnabledTasksPerUser: 1,
		},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	input := automationConfigurationTaskInput("stable-create")
	input.RequestID = "request-stable-create"

	first, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("first CreateTask: %v", err)
	}
	replayed, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("idempotent replay must bypass capacity check: %v", err)
	}
	if replayed.JobID != first.JobID || replayed.ConfigurationVersion != first.ConfigurationVersion {
		t.Fatalf("replay = %+v, want same task/version as %+v", replayed, first)
	}
	events, err := service.ListTaskEvents(context.Background(), first.JobID, 20)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	createEvents := 0
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionCreate {
			createEvents++
		}
	}
	if createEvents != 1 {
		t.Fatalf("create events = %d, want 1", createEvents)
	}

	conflicting := input
	conflicting.Name = "different intent"
	if _, err = service.CreateTask(context.Background(), conflicting); !errors.Is(err, automationdomain.ErrCreateRequestConflict) {
		t.Fatalf("conflicting replay error = %v, want ErrCreateRequestConflict", err)
	}
}

func TestCreateTaskReplayReportsCommittedTaskWasDeleted(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	input := automationConfigurationTaskInput("deleted-create-result")
	input.RequestID = "request-deleted-create-result"
	created, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	ownerCtx := contextForOwner(context.Background(), created.OwnerUserID)
	status, err := service.GetTaskCreateRequestStatus(ownerCtx, input.RequestID)
	if err != nil {
		t.Fatalf("GetTaskCreateRequestStatus committed: %v", err)
	}
	if status.Status != automationdomain.TaskCreateRequestStatusCommitted ||
		status.Task == nil || status.Task.JobID != created.JobID {
		t.Fatalf("committed create status = %+v", status)
	}
	if err = service.repository.DeleteScheduledTask(context.Background(), created.OwnerUserID, created.JobID); err != nil {
		t.Fatalf("DeleteScheduledTask: %v", err)
	}
	if _, err = service.CreateTask(context.Background(), input); !errors.Is(err, automationdomain.ErrCreateRequestResultGone) {
		t.Fatalf("deleted replay error = %v, want ErrCreateRequestResultGone", err)
	}
	status, err = service.GetTaskCreateRequestStatus(ownerCtx, input.RequestID)
	if err != nil {
		t.Fatalf("GetTaskCreateRequestStatus gone: %v", err)
	}
	if status.Status != automationdomain.TaskCreateRequestStatusGone || status.Task != nil {
		t.Fatalf("gone create status = %+v", status)
	}
	missing, err := service.GetTaskCreateRequestStatus(ownerCtx, "request-never-accepted")
	if err != nil {
		t.Fatalf("GetTaskCreateRequestStatus missing: %v", err)
	}
	if missing.Status != automationdomain.TaskCreateRequestStatusNotFound || missing.Task != nil {
		t.Fatalf("missing create status = %+v", missing)
	}
}

func TestAutomationCommandCreateRetryReplaysSameTask(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	createArgs := map[string]any{
		"request_id":     "mcp-create-stable",
		"name":           "stable",
		"instruction":    "run once",
		"execution_mode": "temporary",
		"reply_mode":     "none",
		"schedule": map[string]any{
			"kind":           "interval",
			"interval_value": 10,
			"interval_unit":  "minutes",
		},
	}
	firstResult, firstError := callAutomationCommand(
		t,
		fixture.Service,
		fixture.ServerContext,
		"create_scheduled_task",
		createArgs,
	)
	if firstError {
		t.Fatalf("first create_scheduled_task: %s", automationCommandText(t, firstResult))
	}
	secondResult, secondError := callAutomationCommand(
		t,
		fixture.Service,
		fixture.ServerContext,
		"create_scheduled_task",
		createArgs,
	)
	if secondError {
		t.Fatalf("replayed create_scheduled_task: %s", automationCommandText(t, secondResult))
	}
	first := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, firstResult)
	second := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, secondResult)
	if first.JobID != second.JobID || first.ConfigurationVersion != second.ConfigurationVersion {
		t.Fatalf("command replay created a different task: first=%+v second=%+v", first, second)
	}
	tasks, err := fixture.Service.ListTasks(
		automationCommandTestOwnerContext(fixture.ServerContext.OwnerUserID),
		fixture.ServerContext.AgentID,
	)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("task count after replay = %d, want 1", len(tasks))
	}
}

func automationConfigurationTaskInput(name string) automationdomain.CreateJobInput {
	interval := 300
	return automationdomain.CreateJobInput{
		Name:        name,
		AgentID:     "agent-1",
		Schedule:    automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &interval, Timezone: "UTC"},
		Instruction: "run",
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source: automationdomain.Source{
			Kind: automationdomain.SourceKindAgent,
		},
		Enabled: true,
	}
}

func TestAutomationExactContractAndRawInputAgree(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	contract, err := fixture.Service.RuntimeCommandContract(context.Background(), fixture.ServerContext, "create")
	if err != nil || len(contract.Operations) != 1 {
		t.Fatalf("contract=%+v err=%v", contract, err)
	}
	schema := contract.Operations["create"].InputSchema
	valid := map[string]any{"name": "test", "instruction": "test", "schedule": map[string]any{"kind": "interval", "interval_value": 5}}
	if err := command.ValidateInput(schema, valid); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		field string
		value any
	}{
		{"agent_id", "other"}, {"enabled", "true"}, {"context_mode", "unknown"},
		{"schedule", map[string]any{"kind": "interval", "interval_value": 0}},
		{"schedule", map[string]any{"kind": "interval", "interval_value": 5, "extra": true}},
	} {
		input := map[string]any{}
		for k, v := range valid {
			input[k] = v
		}
		input[test.field] = test.value
		if err := ValidateRuntimeCommandInput(fixture.ServerContext, "plan", "create", input); err == nil {
			t.Fatalf("accepted invalid %s=%v", test.field, test.value)
		}
	}
	if _, err := fixture.Service.RuntimeCommandContract(context.Background(), fixture.ServerContext, "unknown"); err == nil {
		t.Fatal("unknown operation accepted")
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "invoke", "list", nil); err == nil || !strings.Contains(err.Error(), "action=inspect") {
		t.Fatalf("query correction: %v", err)
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "invoke", "create", valid); err == nil || !strings.Contains(err.Error(), "action=plan") {
		t.Fatalf("mutation correction: %v", err)
	}
}

func TestAutomationPlanViewCanBeReusedWithoutHiddenFields(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	input := automationdomain.AutomationCommandInput{Name: "test", Instruction: "test", Schedule: &automationdomain.AutomationCommandSchedule{Kind: "interval", IntervalValue: 60}}
	plan, err := fixture.Service.PlanRuntimeCommand(context.Background(), fixture.ServerContext, "create", input)
	if err != nil {
		t.Fatal(err)
	}
	view, err := RuntimeCommandPlanView(fixture.ServerContext, *plan)
	if err != nil {
		t.Fatal(err)
	}
	raw := view["input"].(map[string]any)
	if _, ok := raw["agent_id"]; ok {
		t.Fatal("hidden agent_id leaked into reusable input")
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "apply", "create", raw); err != nil {
		t.Fatal(err)
	}
	bytes, _ := json.Marshal(raw)
	var reused automationdomain.AutomationCommandInput
	if err := json.Unmarshal(bytes, &reused); err != nil {
		t.Fatal(err)
	}
	replanned, err := fixture.Service.PlanRuntimeCommand(context.Background(), fixture.ServerContext, "create", reused)
	if err != nil {
		t.Fatal(err)
	}
	request := automationdomain.AutomationCommandRequest{RequestID: "schema-test-001", ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest}
	if err := ValidateRuntimeCommandPlan(request, *replanned); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"request_id", "expected_revision", "plan_digest", "stale_revision", "stale_digest"} {
		bad := request
		switch field {
		case "request_id":
			bad.RequestID = ""
		case "expected_revision":
			bad.ExpectedRevision = ""
		case "plan_digest":
			bad.PlanDigest = ""
		case "stale_revision":
			bad.ExpectedRevision = "old"
		case "stale_digest":
			bad.PlanDigest = "old"
		}
		if err := ValidateRuntimeCommandPlan(bad, *replanned); err == nil {
			t.Fatalf("accepted %s", field)
		}
	}
}

func TestServiceRejectsAgentActorAtEveryScriptControlEntry(t *testing.T) {
	service := newScriptControlBoundaryService(t)
	agentCtx := automationexec.WithActorAgentID(context.Background(), "agent-1")

	if _, err := service.CreateTask(agentCtx, scriptControlTaskInput("agent-script-create", automationdomain.ExecutionKindScript)); !errors.Is(err, errAgentScriptControl) {
		t.Fatalf("Agent script create error = %v, want boundary rejection", err)
	}

	for _, test := range []struct {
		name string
		call func(string) error
	}{
		{
			name: "update",
			call: func(jobID string) error {
				name := "changed"
				_, err := service.UpdateTask(agentCtx, jobID, automationdomain.UpdateJobInput{Name: &name})
				return err
			},
		},
		{
			name: "delete",
			call: func(jobID string) error {
				_, err := service.DeleteTask(agentCtx, jobID)
				return err
			},
		},
		{
			name: "run",
			call: func(jobID string) error {
				_, err := service.RunTaskNow(agentCtx, jobID)
				return err
			},
		},
		{
			name: "retry_delivery",
			call: func(jobID string) error {
				_, err := service.RetryRunDelivery(agentCtx, jobID, "missing-run")
				return err
			},
		},
		{
			name: "recover",
			call: func(jobID string) error {
				_, err := service.RecoverTaskRunningRun(agentCtx, jobID, "")
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			task, err := service.CreateTask(context.Background(), scriptControlTaskInput("script-"+test.name, automationdomain.ExecutionKindScript))
			if err != nil {
				t.Fatalf("human script create failed: %v", err)
			}
			if err = test.call(task.JobID); !errors.Is(err, errAgentScriptControl) {
				t.Fatalf("%s error = %v, want boundary rejection", test.name, err)
			}
		})
	}
}

func TestServiceSerializesConcurrentHumanScriptTransitionBeforeAgentRun(t *testing.T) {
	service := newScriptControlBoundaryService(t)
	task, err := service.CreateTask(context.Background(), scriptControlTaskInput("concurrent-agent", automationdomain.ExecutionKindAgent))
	if err != nil {
		t.Fatalf("create Agent task: %v", err)
	}

	humanPersisted := make(chan struct{})
	releaseHuman := make(chan struct{})
	service.SetTaskEventNotifier(TaskEventNotifierFunc(func(_ context.Context, event automationdomain.ScheduledTaskEvent) {
		if event.JobID != task.JobID || event.Action != automationdomain.TaskEventActionUpdate {
			return
		}
		close(humanPersisted)
		<-releaseHuman
	}))

	humanDone := make(chan error, 1)
	go func() {
		scriptKind := automationdomain.ExecutionKindScript
		_, updateErr := service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{
			ExecutionKind: &scriptKind,
		})
		humanDone <- updateErr
	}()
	<-humanPersisted

	agentDone := make(chan error, 1)
	go func() {
		agentCtx := automationexec.WithActorAgentID(context.Background(), "agent-1")
		_, runErr := service.RunTaskNow(agentCtx, task.JobID)
		agentDone <- runErr
	}()
	select {
	case runErr := <-agentDone:
		close(releaseHuman)
		<-humanDone
		t.Fatalf("Agent run crossed the in-flight human control boundary: %v", runErr)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseHuman)
	if err = <-humanDone; err != nil {
		t.Fatalf("human script transition failed: %v", err)
	}
	if err = <-agentDone; !errors.Is(err, errAgentScriptControl) {
		t.Fatalf("Agent run after script transition error = %v, want boundary rejection", err)
	}
}

func newScriptControlBoundaryService(t *testing.T) *Service {
	t.Helper()
	return NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
}

func scriptControlTaskInput(name string, executionKind string) automationdomain.CreateJobInput {
	return automationdomain.CreateJobInput{
		Name:          name,
		AgentID:       "agent-1",
		Instruction:   "echo safe",
		ExecutionKind: executionKind,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	}
}
