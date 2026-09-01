package automation

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/pressly/goose/v3"
)

type permissionProjectionRaceDMRunner struct {
	permission   *permissionctx.Context
	tools        []sdkpermission.Request
	beforeSecond func() error

	mu       sync.Mutex
	requests []dmsvc.Request
}

type permissionDispatchFailureDMRunner struct {
	err error
}

func (r *permissionDispatchFailureDMRunner) HandleChat(context.Context, dmsvc.Request) error {
	return r.err
}

func (r *permissionProjectionRaceDMRunner) HandleChat(_ context.Context, request dmsvc.Request) error {
	r.mu.Lock()
	requestIndex := len(r.requests)
	r.requests = append(r.requests, request)
	r.mu.Unlock()
	if requestIndex == 1 && r.beforeSecond != nil {
		if err := r.beforeSecond(); err != nil {
			return err
		}
	}

	for _, tool := range r.tools {
		decision, err := request.PermissionHandler(context.Background(), tool)
		if err != nil {
			return err
		}
		if decision.Behavior != sdkpermission.BehaviorAllow {
			r.emitTerminal(request, "permission blocked", "success", true)
			return nil
		}
	}
	r.emitTerminal(request, "ok", "success", false)
	return nil
}

func (r *permissionProjectionRaceDMRunner) HandleInterrupt(context.Context, dmsvc.InterruptRequest) error {
	return nil
}

func (r *permissionProjectionRaceDMRunner) emitTerminal(
	request dmsvc.Request,
	result string,
	subtype string,
	permissionDenied bool,
) {
	data := map[string]any{
		"message_id": "result_" + request.RoundID,
		"round_id":   request.RoundID,
		"role":       "result",
		"subtype":    subtype,
		"result":     result,
		"session_id": "sdk_" + request.RoundID,
	}
	if permissionDenied {
		data["permission_denials"] = []map[string]any{{"tool_name": "blocked"}}
	}
	r.permission.BroadcastEvent(context.Background(), request.SessionKey, protocol.EventMessage{
		ProtocolVersion: 2,
		DeliveryMode:    "durable",
		EventType:       protocol.EventTypeMessage,
		SessionKey:      request.SessionKey,
		Data:            data,
	})
	r.permission.BroadcastEvent(
		context.Background(),
		request.SessionKey,
		protocol.NewRoundStatusEvent(request.SessionKey, request.RoundID, "finished", subtype),
	)
}

func TestTaskPermissionPolicyReadPathsDoNotBackfill(t *testing.T) {
	db := newAutomationTestDB(t)
	createdBy := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := createdBy.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "旧任务只读投影",
		AgentID:     "agent-legacy-read",
		Instruction: "读取旧任务",
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
	resetTaskPermissionPolicyToLegacy(t, db, task.JobID)
	if _, err = db.Exec(`
CREATE TABLE permission_policy_update_audit (job_id TEXT NOT NULL);
CREATE TRIGGER audit_permission_policy_update
AFTER UPDATE OF permission_policy_revision ON automation_scheduled_tasks
WHEN NEW.job_id = '` + task.JobID + `'
BEGIN
    INSERT INTO permission_policy_update_audit(job_id) VALUES (NEW.job_id);
END;
`); err != nil {
		t.Fatalf("创建权限策略更新审计失败: %v", err)
	}

	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	loaded, err := service.GetTask(context.Background(), task.JobID)
	if err != nil || loaded == nil {
		t.Fatalf("GetTask 失败: task=%+v err=%v", loaded, err)
	}
	if loaded.PermissionPolicy.Revision != 0 ||
		loaded.PermissionState != automationdomain.TaskPermissionStateUninitialized {
		t.Fatalf("旧任务读取应保持未初始化投影: %+v", loaded)
	}
	listed, err := service.ListTasks(context.Background(), task.AgentID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListTasks 失败: tasks=%+v err=%v", listed, err)
	}
	if listed[0].PermissionPolicy.Revision != 0 ||
		listed[0].PermissionState != automationdomain.TaskPermissionStateUninitialized {
		t.Fatalf("旧任务列表读取应保持未初始化投影: %+v", listed[0])
	}
	if count := permissionPolicyUpdateAuditCount(t, db); count != 0 {
		t.Fatalf("只读路径不应回填权限策略，写入次数=%d", count)
	}

	name := "旧任务显式修改"
	updated, err := service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{Name: &name})
	if err != nil {
		t.Fatalf("旧任务显式修改失败: %v", err)
	}
	if updated.PermissionPolicy.Revision != 1 ||
		updated.PermissionState != automationdomain.TaskPermissionStateReady ||
		strings.TrimSpace(updated.Name) != name {
		t.Fatalf("显式修改未携带兼容策略一次性落库: %+v", updated)
	}
	if count := permissionPolicyUpdateAuditCount(t, db); count != 1 {
		t.Fatalf("旧任务修改只能随业务写入回填一次，写入次数=%d", count)
	}
}

func TestTaskPermissionBoundaryUpdateRollsBackDefinitionAndInvalidationTogether(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "权限边界事务回滚",
		AgentID:     "agent-1",
		Instruction: "原始任务说明",
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
	runID := "run-permission-boundary-rollback"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:                    runID,
		JobID:                    task.JobID,
		OwnerUserID:              task.OwnerUserID,
		TriggerKind:              automationdomain.TriggerKindManual,
		Status:                   automationdomain.RunStatusPending,
		DeliveryStatus:           automationdomain.DeliveryStatusPending,
		PermissionPolicyRevision: task.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("创建阻塞 run 失败: %v", err)
	}
	requestID := "permission-boundary-rollback"
	if _, _, err = service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request: automationdomain.AutomationPermissionRequest{
				RequestID:      requestID,
				OwnerUserID:    task.OwnerUserID,
				JobID:          task.JobID,
				RunID:          runID,
				PolicyRevision: task.PermissionPolicy.Revision,
				Kind:           automationdomain.PermissionRequestKindTool,
				Capability: automationdomain.PermissionCapability{
					ToolName:         "Write",
					Effect:           automationdomain.PermissionEffectWrite,
					InputFingerprint: "sha256:boundary-rollback",
				},
			},
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	); err != nil {
		t.Fatalf("创建审批请求失败: %v", err)
	}
	if _, err = db.Exec(`
CREATE TRIGGER fail_permission_boundary_run_cancel
BEFORE UPDATE OF status ON automation_task_runs
WHEN OLD.run_id = '` + runID + `' AND NEW.status = 'cancelled'
BEGIN
    SELECT RAISE(FAIL, 'injected permission boundary cancellation failure');
END;
`); err != nil {
		t.Fatalf("创建回滚触发器失败: %v", err)
	}

	updatedInstruction := "不能局部提交的新任务说明"
	if _, err = service.UpdateTaskAtVersion(
		context.Background(),
		task.JobID,
		task.ConfigurationVersion,
		automationdomain.UpdateJobInput{Instruction: &updatedInstruction},
	); err == nil {
		t.Fatal("注入失效失败后 UpdateTaskAtVersion 应返回错误")
	}
	persisted, loadErr := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if loadErr != nil || persisted == nil {
		t.Fatalf("读取回滚后的任务失败: task=%+v err=%v", persisted, loadErr)
	}
	if persisted.Instruction != task.Instruction ||
		persisted.ConfigurationVersion != task.ConfigurationVersion ||
		persisted.PermissionPolicy.Revision != task.PermissionPolicy.Revision ||
		persisted.PendingPermissionRequestID != requestID ||
		persisted.PermissionState != automationdomain.TaskPermissionStateAwaitingApproval {
		t.Fatalf("任务定义或审批投影发生了部分提交: %+v", persisted)
	}
	storedRequest, requestErr := service.repository.GetPermissionRequest(context.Background(), task.OwnerUserID, requestID)
	if requestErr != nil || storedRequest == nil || storedRequest.Status != automationdomain.PermissionRequestStatusPending {
		t.Fatalf("审批请求没有随事务回滚: request=%+v err=%v", storedRequest, requestErr)
	}
	storedRun, runErr := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if runErr != nil || storedRun == nil ||
		storedRun.Status != automationdomain.RunStatusPending ||
		storedRun.DeliveryStatus != automationdomain.DeliveryStatusPending ||
		storedRun.BlockState != automationdomain.RunBlockStateAwaitingApproval ||
		storedRun.BlockedRequestID != requestID {
		t.Fatalf("阻塞 run 没有随事务回滚: run=%+v err=%v", storedRun, runErr)
	}
}

func TestDeniedPermissionRunClosesDeliveryAndObservabilityPendingState(t *testing.T) {
	db := newPermissionReliabilityMigratedDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "权限拒绝投递收口",
		AgentID:     "agent-1",
		Instruction: "等待用户批准后执行",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      fakeStructuredDelivery("agent-1", "permission-denied"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	runID := "run-permission-denied-delivery"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual, Status: automationdomain.RunStatusPending,
		DeliveryStatus:           automationdomain.DeliveryStatusPending,
		PermissionPolicyRevision: task.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("创建阻塞 run 失败: %v", err)
	}
	request := automationdomain.AutomationPermissionRequest{
		RequestID: "permission-denied-delivery", OwnerUserID: task.OwnerUserID,
		JobID: task.JobID, RunID: runID, PolicyRevision: task.PermissionPolicy.Revision,
		Kind: automationdomain.PermissionRequestKindTool,
		Capability: automationdomain.PermissionCapability{
			ToolName: "Write", Effect: automationdomain.PermissionEffectWrite,
			InputFingerprint: "sha256:permission-denied-delivery",
		},
	}
	storedRequest, _, err := service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request: request, TaskState: automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("创建权限请求失败: %v", err)
	}
	if _, err = db.Exec(`UPDATE automation_task_runs
SET delivery_attempts = 2, delivery_attempt_id = 'stale-attempt',
    delivery_attempt_started_at = CURRENT_TIMESTAMP,
    delivery_next_attempt_at = CURRENT_TIMESTAMP,
    delivery_dead_letter_at = CURRENT_TIMESTAMP
WHERE run_id = ?`, runID); err != nil {
		t.Fatalf("预置过期投递状态失败: %v", err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		storedRequest.RequestID,
		permissionDecisionInputForRequest(*storedRequest, automationdomain.PermissionDecisionDeny),
	)
	if err != nil {
		t.Fatalf("拒绝权限请求失败: %v", err)
	}
	if decision.Run == nil || decision.Run.Status != automationdomain.RunStatusFailed ||
		decision.Run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted ||
		decision.Run.DeliveryAttempts != 0 || decision.Run.DeliveryNextAttemptAt != nil ||
		decision.Run.DeliveryDeadLetterAt != nil {
		t.Fatalf("拒绝后的 run 未完整收口: %+v", decision.Run)
	}
	if decision.Task.LastRunStatus != automationdomain.RunStatusFailed ||
		decision.Task.LastDeliveryStatus != automationdomain.DeliveryStatusNotAttempted {
		t.Fatalf("拒绝后的任务摘要不是权威终态: %+v", decision.Task)
	}
	status, err := service.GetTaskStatus(ownerCtx, task.JobID, 10, 10)
	if err != nil {
		t.Fatalf("GetTaskStatus 失败: %v", err)
	}
	if status.Health.DeliveryPendingRunCount != 0 || len(status.Health.DeliveryPendingRunIDs) != 0 ||
		status.Health.DeliveryFailedRunCount != 0 || status.Health.ManualRedeliveryAvailable {
		t.Fatalf("无可投递结果不应暴露等待或补投动作: %+v", status.Health)
	}
	if status.Health.FailedRunCount != 1 || len(status.Health.ExecutionFailedRunIDs) != 1 ||
		status.Health.ExecutionFailedRunIDs[0] != runID {
		t.Fatalf("权限拒绝仍应保留执行失败事实: %+v", status.Health)
	}
}

func newPermissionReliabilityMigratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "permission-reliability.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO agents (
id, slug, name, description, definition, status, workspace_path
) VALUES ('agent-1', 'agent-1', 'Agent 1', '', '', 'active', '/tmp/agent-1')`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestResumePermissionRunFailureKeepsExactActionableRequest(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "重试补偿保留审批身份",
		AgentID:     "agent-1",
		Instruction: "执行已批准操作",
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

	runID := "run-resume-compensation"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:                    runID,
		JobID:                    task.JobID,
		OwnerUserID:              task.OwnerUserID,
		TriggerKind:              automationdomain.TriggerKindManual,
		Status:                   automationdomain.RunStatusPending,
		PermissionPolicyRevision: task.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("创建阻塞 run 失败: %v", err)
	}
	request := automationdomain.AutomationPermissionRequest{
		RequestID:      "permission-resume-compensation",
		OwnerUserID:    task.OwnerUserID,
		JobID:          task.JobID,
		RunID:          runID,
		PolicyRevision: task.PermissionPolicy.Revision,
		Kind:           automationdomain.PermissionRequestKindTool,
		Capability: automationdomain.PermissionCapability{
			ToolName:         "Write",
			Effect:           automationdomain.PermissionEffectWrite,
			InputFingerprint: "sha256:resume-compensation",
		},
		ResumeSafe: false,
	}
	storedRequest, _, err := service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request:    request,
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("创建权限请求失败: %v", err)
	}
	request = *storedRequest
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		request.RequestID,
		permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowOnce),
	)
	if err != nil || decision.Task.PermissionState != automationdomain.TaskPermissionStateReadyToRetry {
		t.Fatalf("权限批准未进入显式重试状态: result=%+v err=%v", decision, err)
	}
	historicalRequest := request
	if err = service.repository.SetTaskPermissionState(
		context.Background(),
		task.OwnerUserID,
		task.JobID,
		automationdomain.TaskPermissionStateReady,
		"",
	); err != nil {
		t.Fatalf("模拟下一次权限阻塞前重置任务失败: %v", err)
	}
	if _, err = db.Exec(
		`UPDATE automation_task_runs
SET block_state = '', blocked_request_id = NULL
WHERE run_id = ? AND owner_user_id = ?`,
		runID,
		task.OwnerUserID,
	); err != nil {
		t.Fatalf("模拟下一次权限阻塞前重置 run 失败: %v", err)
	}
	request.RequestID = "permission-resume-current"
	request.Capability.ToolName = "WebFetch"
	request.Capability.InputFingerprint = "sha256:resume-current"
	storedRequest, _, err = service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request:    request,
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("创建同 run/revision 的后继权限请求失败: %v", err)
	}
	request = *storedRequest
	decision, err = service.ResolvePermissionRequest(
		ownerCtx,
		request.RequestID,
		permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowOnce),
	)
	if err != nil || decision.Run == nil || decision.Run.BlockedRequestID != request.RequestID {
		t.Fatalf("后继审批没有成为 run 的 exact retry identity: result=%+v err=%v", decision, err)
	}

	if _, err = db.Exec(
		`UPDATE automation_task_runs SET blocked_request_id = NULL WHERE run_id = ?`,
		runID,
	); err != nil {
		t.Fatalf("制造旧版 run 请求投影失败: %v", err)
	}
	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks SET running_run_id = ? WHERE job_id = ?`,
		"competing-run",
		task.JobID,
	); err != nil {
		t.Fatalf("制造运行占用冲突失败: %v", err)
	}
	actionable, err := service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
	if err != nil || len(actionable) != 1 || actionable[0].RequestID != request.RequestID {
		t.Fatalf("旧版 task exact request 应能定位同一 run: requests=%+v err=%v", actionable, err)
	}
	if _, err = service.ResumePermissionRun(
		ownerCtx,
		task.JobID,
		runID,
		permissionResumeInputForRequest(historicalRequest),
	); !errors.Is(err, automationdomain.ErrPermissionRequestStale) {
		t.Fatalf("历史 approved request 不得恢复当前阻塞: %v", err)
	}
	if _, err = service.ResumePermissionRun(
		ownerCtx,
		task.JobID,
		runID,
		permissionResumeInputForRequest(request),
	); err == nil {
		t.Fatal("运行占用冲突时 ResumePermissionRun 应失败")
	}
	var boundRequestID sql.NullString
	if err = db.QueryRow(
		`SELECT blocked_request_id FROM automation_task_runs WHERE run_id = ?`,
		runID,
	).Scan(&boundRequestID); err != nil {
		t.Fatalf("读取旧版 run 修复结果失败: %v", err)
	}
	if !boundRequestID.Valid || boundRequestID.String != request.RequestID {
		t.Fatalf("旧版 run 未绑定 exact request_id: %+v", boundRequestID)
	}

	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks
SET permission_state = ?, pending_permission_request_id = NULL
WHERE job_id = ?`,
		automationdomain.TaskPermissionStateReady,
		task.JobID,
	); err != nil {
		t.Fatalf("制造 task 补偿投影丢失失败: %v", err)
	}
	actionable, err = service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
	if err != nil || len(actionable) != 1 || actionable[0].RequestID != request.RequestID {
		t.Fatalf("exact run/request 应能修复丢失的任务投影: requests=%+v err=%v", actionable, err)
	}
	if _, err = service.ResumePermissionRun(
		ownerCtx,
		task.JobID,
		runID,
		permissionResumeInputForRequest(request),
	); err == nil {
		t.Fatal("投影修复后仍有运行占用冲突时 ResumePermissionRun 应失败")
	}

	persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted == nil {
		t.Fatalf("读取补偿后的任务失败: task=%+v err=%v", persisted, err)
	}
	if persisted.PermissionState != automationdomain.TaskPermissionStateReadyToRetry ||
		persisted.PendingPermissionRequestID != request.RequestID {
		t.Fatalf("失败补偿丢失 exact request_id: %+v", persisted)
	}
	actionable, err = service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
	if err != nil || len(actionable) != 1 || actionable[0].RequestID != request.RequestID {
		t.Fatalf("失败补偿后的请求不可再次操作: requests=%+v err=%v", actionable, err)
	}
}

func TestResumePermissionRunDispatchFailureRestoresExactRunRequest(t *testing.T) {
	db := newAutomationTestDB(t)
	dispatchErr := errors.New("injected resume dispatch failure")
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&permissionDispatchFailureDMRunner{err: dispatchErr},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "恢复派发失败仍可重试",
		AgentID:     "agent-1",
		Instruction: "执行已批准操作",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-dispatch-failure", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	runID := "run-resume-dispatch-failure"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:                    runID,
		JobID:                    task.JobID,
		OwnerUserID:              task.OwnerUserID,
		TriggerKind:              automationdomain.TriggerKindManual,
		Status:                   automationdomain.RunStatusPending,
		PermissionPolicyRevision: task.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("创建阻塞 run 失败: %v", err)
	}
	request := automationdomain.AutomationPermissionRequest{
		RequestID:      "permission-resume-dispatch-failure",
		OwnerUserID:    task.OwnerUserID,
		JobID:          task.JobID,
		RunID:          runID,
		PolicyRevision: task.PermissionPolicy.Revision,
		Kind:           automationdomain.PermissionRequestKindTool,
		Capability: automationdomain.PermissionCapability{
			ToolName:         "Write",
			Effect:           automationdomain.PermissionEffectWrite,
			InputFingerprint: "sha256:resume-dispatch-failure",
		},
		ResumeSafe: false,
	}
	storedRequest, _, err := service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request:    request,
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("创建权限请求失败: %v", err)
	}
	request = *storedRequest
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		request.RequestID,
		permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowOnce),
	)
	if err != nil || decision.Task.PermissionState != automationdomain.TaskPermissionStateReadyToRetry {
		t.Fatalf("权限批准未进入显式重试状态: result=%+v err=%v", decision, err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = service.ResumePermissionRun(
			ownerCtx,
			task.JobID,
			runID,
			permissionResumeInputForRequest(request),
		); !errors.Is(err, dispatchErr) {
			t.Fatalf("第 %d 次恢复没有返回派发错误: %v", attempt, err)
		}
		persisted, taskErr := service.repository.GetScheduledTask(
			context.Background(),
			task.OwnerUserID,
			task.JobID,
		)
		persistedRun, runErr := service.repository.GetRun(
			context.Background(),
			task.OwnerUserID,
			task.JobID,
			runID,
		)
		if taskErr != nil || persisted == nil ||
			persisted.PermissionState != automationdomain.TaskPermissionStateReadyToRetry ||
			persisted.PendingPermissionRequestID != request.RequestID {
			t.Fatalf("第 %d 次失败后 task 丢失 exact request: task=%+v err=%v", attempt, persisted, taskErr)
		}
		if runErr != nil || persistedRun == nil ||
			persistedRun.Status != automationdomain.RunStatusPending ||
			persistedRun.BlockState != automationdomain.RunBlockStateReadyToRetry ||
			persistedRun.BlockedRequestID != request.RequestID {
			t.Fatalf("第 %d 次失败后 run 不可再次恢复: run=%+v err=%v", attempt, persistedRun, runErr)
		}
		actionable, listErr := service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
		if listErr != nil || len(actionable) != 1 || actionable[0].RequestID != request.RequestID {
			t.Fatalf("第 %d 次失败后审批卡不可操作: requests=%+v err=%v", attempt, actionable, listErr)
		}
	}
}

func TestPermissionPauseDoesNotClearACompetingRunClaim(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "权限暂停不覆盖新运行",
		AgentID:     "agent-1",
		Instruction: "验证 exact runtime claim",
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
	oldRunID := "run-permission-old"
	competingRunID := "run-permission-new"
	service.mu.Lock()
	state := service.jobStates[task.JobID]
	state.Running = true
	state.RunningCount = 1
	state.RunningRunID = oldRunID
	service.mu.Unlock()
	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks SET running_run_id = ?, running_started_at = CURRENT_TIMESTAMP WHERE job_id = ?`,
		competingRunID,
		task.JobID,
	); err != nil {
		t.Fatalf("预置竞争 run 失败: %v", err)
	}
	reason := "旧 run 等待权限"
	service.pauseJobRuntimeForPermission(
		*task,
		oldRunID,
		automationdomain.TaskPermissionStateReadyToRetry,
		&reason,
	)
	persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted == nil || persisted.RunningRunID != competingRunID {
		t.Fatalf("旧权限暂停覆盖了新 run: task=%+v err=%v", persisted, err)
	}
	service.mu.Lock()
	projected := service.jobStates[task.JobID]
	if projected == nil || projected.RunningRunID != competingRunID || !projected.Running {
		service.mu.Unlock()
		t.Fatalf("内存投影没有刷新到新 run: %+v", projected)
	}
	service.mu.Unlock()
}

func TestPermissionPauseDoesNotOverwriteACompletedCompetingRun(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "权限暂停不覆盖已完成运行",
		AgentID:     "agent-1",
		Instruction: "验证完成态 exact runtime fence",
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
	oldRunID := "run-permission-completed-old"
	service.mu.Lock()
	state := service.jobStates[task.JobID]
	state.Running = true
	state.RunningCount = 1
	state.RunningRunID = oldRunID
	service.mu.Unlock()
	completedAt := time.Now().UTC().Truncate(time.Second)
	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks
SET running_run_id = NULL,
    running_started_at = NULL,
    last_run_at = ?,
    last_run_status = ?,
    failure_streak = 0,
    last_error = NULL,
    last_delivery_status = ?
WHERE job_id = ?`,
		completedAt,
		automationdomain.RunStatusSucceeded,
		automationdomain.DeliveryStatusSucceeded,
		task.JobID,
	); err != nil {
		t.Fatalf("预置新 run 完成态失败: %v", err)
	}
	reason := "旧 run 等待权限"
	service.pauseJobRuntimeForPermission(
		*task,
		oldRunID,
		automationdomain.TaskPermissionStateReadyToRetry,
		&reason,
	)
	persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted == nil {
		t.Fatalf("读取权威任务失败: task=%+v err=%v", persisted, err)
	}
	if persisted.Running || persisted.RunningRunID != "" ||
		persisted.LastRunStatus != automationdomain.RunStatusSucceeded ||
		persisted.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded ||
		persisted.LastRunAt == nil {
		t.Fatalf("旧权限暂停覆盖了新 run 完成事实: %+v", persisted)
	}
	service.mu.Lock()
	projected := service.jobStates[task.JobID]
	if projected == nil || projected.Running || projected.RunningRunID != "" ||
		projected.LastRunStatus != automationdomain.RunStatusSucceeded ||
		projected.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded {
		service.mu.Unlock()
		t.Fatalf("内存投影没有收敛到新 run 完成事实: %+v", projected)
	}
	service.mu.Unlock()
}

func TestResumePermissionRunDoesNotClearNewRequestCreatedDuringDispatch(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	runner := &permissionProjectionRaceDMRunner{
		permission: permission,
		tools: []sdkpermission.Request{
			{ToolName: "Write", Input: map[string]any{"file_path": "/tmp/permission-race"}},
			{ToolName: "WebSearch", Input: map[string]any{"query": "first approval"}},
			{ToolName: "Read", Input: map[string]any{"file_path": "/tmp/next-request"}},
		},
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		runner,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "恢复尾部保留下一条审批",
		AgentID:     "agent-1",
		Instruction: "依次执行三个工具",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-projection-race", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	policy := appendTaskPermissionGrant(task.PermissionPolicy, automationdomain.TaskPermissionGrant{
		GrantID: "grant-write-before-resume",
		Capability: automationdomain.PermissionCapability{
			ToolName: "Write",
			Effect:   automationdomain.PermissionEffectWrite,
		},
		Source: automationdomain.PermissionGrantSourceUserApproval,
	})
	updated, err := service.repository.UpdateTaskPermissionPolicyIfRevision(
		context.Background(),
		task.OwnerUserID,
		task.JobID,
		task.PermissionPolicy.Revision,
		policy,
		automationdomain.TaskPermissionStateReady,
	)
	if err != nil || !updated {
		t.Fatalf("预置写权限失败: updated=%v err=%v", updated, err)
	}

	runResult, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || runResult.RunID == nil {
		t.Fatalf("RunTaskNow 失败: result=%+v err=%v", runResult, err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	pending, err := service.ListPermissionRequests(
		ownerCtx,
		automationdomain.PermissionRequestStatusPending,
		task.JobID,
	)
	if err != nil || len(pending) != 1 || pending[0].Capability.ToolName != "WebSearch" || pending[0].ResumeSafe {
		t.Fatalf("首次运行没有停在副作用后的 WebSearch 审批: requests=%+v err=%v", pending, err)
	}
	firstRequest := pending[0]
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		firstRequest.RequestID,
		permissionDecisionInputForRequest(firstRequest, automationdomain.PermissionDecisionAllowOnce),
	)
	if err != nil || decision.ResumeStarted ||
		decision.Task.PermissionState != automationdomain.TaskPermissionStateReadyToRetry {
		t.Fatalf("首次审批没有进入显式恢复: result=%+v err=%v", decision, err)
	}
	// 模拟新 attempt 在 dispatch 返回后的极窄窗口接管旧 task 投影：
	// 它先用 exact old request 释放旧绑定，随后同步创建下一条请求；
	// ResumePermissionRun 的尾部清理必须因 request_id 已变化而 CAS 失败。
	runner.beforeSecond = func() error {
		cleared, clearErr := service.repository.ClearTaskPermissionRetryState(
			context.Background(),
			task.OwnerUserID,
			task.JobID,
			firstRequest.PolicyRevision,
			firstRequest.RequestID,
		)
		if clearErr != nil {
			return clearErr
		}
		if !cleared {
			return errors.New("old permission projection was not available for the race setup")
		}
		return nil
	}

	resumed, err := service.ResumePermissionRun(
		ownerCtx,
		task.JobID,
		*runResult.RunID,
		permissionResumeInputForRequest(firstRequest),
	)
	if err != nil || resumed == nil || !resumed.ResumeStarted {
		failedTask, _ := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
		failedRun, _ := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, *runResult.RunID)
		failedRequest, _ := service.repository.GetPermissionRequest(context.Background(), task.OwnerUserID, firstRequest.RequestID)
		t.Fatalf(
			"显式恢复失败: result=%+v err=%v task=%+v run=%+v request=%+v",
			resumed,
			err,
			failedTask,
			failedRun,
			failedRequest,
		)
	}
	pending, err = service.ListPermissionRequests(
		ownerCtx,
		automationdomain.PermissionRequestStatusPending,
		task.JobID,
	)
	if err != nil || len(pending) != 1 || pending[0].Capability.ToolName != "Read" {
		t.Fatalf("恢复派发中产生的新请求被尾部清理: requests=%+v err=%v", pending, err)
	}
	nextRequest := pending[0]
	if nextRequest.RequestID == firstRequest.RequestID {
		t.Fatalf("后继请求错误复用了旧 request_id: %q", nextRequest.RequestID)
	}
	persisted, err := service.repository.GetScheduledTask(
		context.Background(),
		task.OwnerUserID,
		task.JobID,
	)
	if err != nil || persisted == nil ||
		persisted.PermissionState != automationdomain.TaskPermissionStateAwaitingApproval ||
		persisted.PendingPermissionRequestID != nextRequest.RequestID {
		t.Fatalf("旧 Resume 尾部覆盖了新任务投影: task=%+v err=%v", persisted, err)
	}
	persistedRun, err := service.repository.GetRun(
		context.Background(),
		task.OwnerUserID,
		task.JobID,
		*runResult.RunID,
	)
	if err != nil || persistedRun == nil ||
		persistedRun.BlockState != automationdomain.RunBlockStateAwaitingApproval ||
		persistedRun.BlockedRequestID != nextRequest.RequestID {
		t.Fatalf("后继请求没有保持 exact run 绑定: run=%+v err=%v", persistedRun, err)
	}
}

func TestPermissionDecisionCommittedErrorPreservesRootCause(t *testing.T) {
	wrapped := MarkPermissionDecisionCommitted(automationdomain.ErrPermissionRequestStale)
	if !PermissionDecisionCommitted(wrapped) {
		t.Fatalf("审批提交阶段标记丢失: %v", wrapped)
	}
	if !errors.Is(wrapped, automationdomain.ErrPermissionRequestStale) {
		t.Fatalf("审批提交阶段错误没有保留根因: %v", wrapped)
	}
}

func permissionPolicyUpdateAuditCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM permission_policy_update_audit`).Scan(&count); err != nil {
		t.Fatalf("读取权限策略更新审计失败: %v", err)
	}
	return count
}
