package automation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func TestUnconfirmedScriptTreeTerminationNeverCallsDeliveryRouter(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "script-tree-unconfirmed")
	runID := "run-script-tree-unconfirmed"
	insertActiveDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
	terminationErr := errors.New("process group drain proof failed")

	err := service.commitScriptObservation(
		context.Background(),
		task,
		runID,
		time.Now(),
		automationexec.ExecutionObservation{
			Status:       automationdomain.RunStatusSucceeded,
			MessageCount: 1,
			ResultText:   "must not be delivered",
		},
		terminationErr,
	)
	if !errors.Is(err, terminationErr) {
		t.Fatalf("termination proof failure must remain visible: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("unconfirmed process tree must never enter delivery: %+v", calls)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("load terminal script run: %v", err)
	}
	if run.Status != automationdomain.RunStatusFailed ||
		run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted ||
		run.ErrorMessage == nil || !strings.Contains(*run.ErrorMessage, "manual review") {
		t.Fatalf("unconfirmed process tree must persist safe terminal facts: %+v", run)
	}
}

func TestTerminalCommitCASMissAndDeletionProduceNoDelivery(t *testing.T) {
	if !shouldCommitDeletingRunTerminal(automationdomain.ErrTaskDeleting) ||
		!shouldCommitDeletingRunTerminal(automationdomain.ErrRunCompletionConflict) {
		t.Fatal("deletion race sentinels 必须进入 exact suppressed terminal 分支")
	}
	for _, test := range []struct {
		name          string
		prepare       func(*testing.T, *sql.DB, automationdomain.ScheduledTask, string)
		wantCommitted bool
	}{
		{
			name: "run already terminal",
			prepare: func(t *testing.T, db *sql.DB, task automationdomain.ScheduledTask, runID string) {
				t.Helper()
				if _, err := db.Exec(`UPDATE automation_task_runs SET status = ? WHERE run_id = ?`, automationdomain.RunStatusCancelled, runID); err != nil {
					t.Fatalf("准备 terminal CAS miss: %v", err)
				}
			},
		},
		{
			name: "task deleting",
			prepare: func(t *testing.T, db *sql.DB, task automationdomain.ScheduledTask, _ string) {
				t.Helper()
				if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET deletion_state = 'deleting', deletion_token = 'delete-token' WHERE job_id = ?`, task.JobID); err != nil {
					t.Fatalf("准备 deleting task: %v", err)
				}
			},
			wantCommitted: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, db, delivery, task := newInitialDeliveryTestService(t, "terminal-cas-"+strings.ReplaceAll(test.name, " ", "-"))
			runID := "run-" + strings.ReplaceAll(test.name, " ", "-")
			insertActiveDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
			test.prepare(t, db, task, runID)

			observation := successfulRunFinish(runID, automationdomain.DeliveryStatusPending)
			updated, committed, err := service.commitObservedRunTerminal(context.Background(), task, observation)
			if committed != test.wantCommitted {
				t.Fatalf("terminal CAS miss = committed %v, want %v (err=%v)", committed, test.wantCommitted, err)
			}
			if test.wantCommitted {
				if err != nil || updated == nil || updated.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted ||
					updated.DeliveryDeadLetterAt == nil {
					t.Fatalf("deleting terminal 必须 suppressed+deadletter: run=%+v err=%v", updated, err)
				}
			} else if !errors.Is(err, automationdomain.ErrRunCompletionConflict) {
				t.Fatalf("ordinary terminal CAS miss error = %v", err)
			}
			if calls := delivery.Calls(); len(calls) != 0 {
				t.Fatalf("terminal commit 失败后不得外投: %+v", calls)
			}
			if test.wantCommitted {
				service.mu.Lock()
				_, retained := service.jobStates[task.JobID]
				service.mu.Unlock()
				if retained {
					t.Fatal("suppressed deleting terminal 不得把 deleting task 留在 scheduler 内存 catalog")
				}
			}
		})
	}
}

func TestPendingInitialDeliveryConcurrentClaimAndCrashRecovery(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "pending-concurrent")
	runID := "run-pending-concurrent"
	insertTerminalPendingDeliveryRun(t, db, service, task, runID, task.Delivery, "durable result")
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks SET enabled = FALSE WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatalf("停用任务: %v", err)
	}
	task.Enabled = false
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 pending run: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			_, deliveryErr := service.deliverPendingRun(context.Background(), task, *run)
			results <- deliveryErr
		}()
	}
	ready.Wait()
	close(start)
	firstErr := <-results
	secondErr := <-results
	if firstErr != nil && secondErr != nil {
		t.Fatalf("pending 并发 claim 至少一个应成功: %v / %v", firstErr, secondErr)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("pending 并发只能外投一次: %+v", calls)
	}
	updated, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取投递结果: %v", err)
	}
	if updated.DeliveryStatus != automationdomain.DeliveryStatusSucceeded || updated.DeliveryAttempts != 1 {
		t.Fatalf("pending crash recovery 未完成 durable 状态: %+v", updated)
	}
}

func TestDeletingTerminalRequiresExactPrivateClaimIdentity(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "deleting-terminal-identity")
	runID := "run-deleting-terminal-identity"
	insertActiveDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET deletion_state = 'review_required', deletion_token = 'authoritative-token' WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatalf("准备 deletion claim: %v", err)
	}
	finish := successfulRunFinish(runID, automationdomain.DeliveryStatusPending)
	for _, input := range []automationstore.DeletingRunTerminalCommitInput{
		{OwnerUserID: task.OwnerUserID, JobID: task.JobID, DeletionToken: "wrong-token", Finish: finish},
		{OwnerUserID: "wrong-owner", JobID: task.JobID, DeletionToken: "authoritative-token", Finish: finish},
	} {
		if err := service.repository.CommitDeletingRunTerminal(context.Background(), input); !errors.Is(err, automationdomain.ErrRunCompletionConflict) {
			t.Fatalf("wrong deletion identity 必须拒绝: %+v err=%v", input, err)
		}
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取未变更 run: %v", err)
	}
	if run.Status != automationdomain.RunStatusRunning || run.DeliveryStatus == automationdomain.DeliveryStatusNotAttempted {
		t.Fatalf("错误 deletion identity 不得修改 run: %+v", run)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("deleting storage CAS 不得外投: %+v", delivery.Calls())
	}
}

func TestTerminalCommitRejectsWrongOwnerJobAndNonTerminalStatus(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "terminal-identity")
	runID := "run-terminal-identity"
	insertActiveDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
	finishedAt := time.Now().UTC()
	for _, input := range []automationstore.RunTerminalCommitInput{
		{
			OwnerUserID: "wrong-owner", JobID: task.JobID,
			Finish: automationstore.RunFinishInput{RunID: runID, Status: automationdomain.RunStatusSucceeded, FinishedAt: finishedAt},
		},
		{
			OwnerUserID: task.OwnerUserID, JobID: "wrong-job",
			Finish: automationstore.RunFinishInput{RunID: runID, Status: automationdomain.RunStatusSucceeded, FinishedAt: finishedAt},
		},
		{
			OwnerUserID: task.OwnerUserID, JobID: task.JobID,
			Finish: automationstore.RunFinishInput{RunID: runID, Status: automationdomain.RunStatusRunning, FinishedAt: finishedAt},
		},
	} {
		if _, err := service.repository.CommitRunTerminalAndRuntime(context.Background(), input); !errors.Is(err, automationdomain.ErrRunCompletionConflict) {
			t.Fatalf("错误 terminal 身份/状态必须拒绝: %+v err=%v", input, err)
		}
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil || run.Status != automationdomain.RunStatusRunning {
		t.Fatalf("错误 terminal claim 不得修改 run: run=%+v err=%v", run, err)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("错误 terminal claim 不得外投: %+v", delivery.Calls())
	}
}

func TestScriptLaunchRegistrationFailureUsesAtomicTerminalPath(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "script registration conflict", AgentID: "agent-1", Instruction: "printf result",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	runID := "run-script-registration-conflict"
	service.idFactory = func(prefix string) string {
		if prefix == "run" {
			return runID
		}
		return prefix + "-fixed"
	}
	service.scriptAttempts[runID] = &scriptAttempt{
		ownerUserID: task.OwnerUserID,
		jobID:       task.JobID,
		done:        make(chan struct{}),
	}
	t.Cleanup(func() { delete(service.scriptAttempts, runID) })

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err == nil ||
		!strings.Contains(err.Error(), "already registered") {
		t.Fatalf("预置 script registration 应拒绝 launch: %v", err)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil || run.Status != automationdomain.RunStatusFailed ||
		run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted {
		t.Fatalf("script launch failure 必须原子落 terminal: run=%+v err=%v", run, err)
	}
	current, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || current == nil || current.RunningRunID != "" || current.Running ||
		current.LastRunStatus != automationdomain.RunStatusFailed {
		t.Fatalf("script launch failure 不得留下 running task: task=%+v err=%v", current, err)
	}
}

func TestPendingRouterErrorStaysUnverifiedAndNeverAutoRetries(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "pending-router-unknown")
	delivery.err = errors.New("transport disconnected after send")
	runID := "run-pending-router-unknown"
	insertTerminalPendingDeliveryRun(t, db, service, task, runID, task.Delivery, "possibly delivered")
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 pending run: %v", err)
	}
	updated, err := service.deliverPendingRun(context.Background(), task, *run)
	if !errors.Is(err, automationdomain.ErrDeliveryRetryCompletionUnconfirmed) {
		t.Fatalf("router 未知结果必须要求人工核对: %v", err)
	}
	if updated == nil || updated.DeliveryStatus != automationdomain.DeliveryStatusRetrying ||
		updated.DeliveryAttempts != 1 || updated.DeliveryNextAttemptAt != nil || updated.DeliveryDeadLetterAt != nil {
		t.Fatalf("未知投递状态不正确: %+v", updated)
	}
	if err = service.retryDueRunDelivery(context.Background(), *updated); err == nil {
		t.Fatal("retrying run 不应被自动 worker 重放")
	}
	due, err := service.repository.ListDueDeliveryRetries(context.Background(), time.Now().Add(24*time.Hour), maxAutoDeliveryAttempts, 20)
	if err != nil || len(due) != 0 {
		t.Fatalf("retrying 不得进入 due 查询: due=%+v err=%v", due, err)
	}
	if _, err = service.RetryRunDelivery(context.Background(), task.JobID, runID); !errors.Is(err, automationdomain.ErrDeliveryRetryUnverified) {
		t.Fatalf("普通人工 retry 也必须先核对: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("未知结果后不得自动产生第二次外投: %+v", calls)
	}
}

func TestOrphanPendingRecoveryDeadLettersWithoutDelivery(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "orphan-pending")
	runID := "run-orphan-pending"
	insertTerminalPendingDeliveryRun(t, db, service, task, runID, task.Delivery, "orphan result")
	if _, err := db.Exec(`DELETE FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatalf("删除 task definition: %v", err)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 orphan pending: %v", err)
	}
	if err = service.retryDueRunDelivery(context.Background(), *run); err != nil {
		t.Fatalf("收口 orphan pending: %v", err)
	}
	updated, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 orphan 收口结果: %v", err)
	}
	if updated.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted || updated.DeliveryDeadLetterAt == nil {
		t.Fatalf("orphan pending 必须 not_attempted+deadletter: %+v", updated)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("orphan pending 不得外投: %+v", delivery.Calls())
	}
}

func TestPendingDeliveryUsesFrozenRouteAndHonorsCurrentModeNone(t *testing.T) {
	t.Run("frozen route", func(t *testing.T) {
		service, db, delivery, task := newInitialDeliveryTestService(t, "frozen-route")
		frozen := fakeStructuredDelivery("agent-1", "frozen")
		current := fakeStructuredDelivery("agent-1", "current")
		runID := "run-frozen-route"
		insertTerminalPendingDeliveryRun(t, db, service, task, runID, frozen, "frozen result")
		if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET delivery_to = ?, delivery_session_key = ?, configuration_version = configuration_version + 1
WHERE job_id = ?`, current.To, current.SessionKey, task.JobID); err != nil {
			t.Fatalf("更新当前路由: %v", err)
		}
		run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
		if err != nil {
			t.Fatalf("读取 pending run: %v", err)
		}
		if _, err = service.deliverPendingRun(context.Background(), task, *run); err != nil {
			t.Fatalf("冻结路由首投失败: %v", err)
		}
		calls := delivery.Calls()
		if len(calls) != 1 || calls[0].SessionKey != frozen.SessionKey {
			t.Fatalf("首投必须使用 run 冻结路由: %+v", calls)
		}
	})

	t.Run("current mode none", func(t *testing.T) {
		service, db, delivery, task := newInitialDeliveryTestService(t, "current-none")
		runID := "run-current-none"
		insertTerminalPendingDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
		if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET delivery_mode = ?, configuration_version = configuration_version + 1
WHERE job_id = ?`, automationdomain.DeliveryModeNone, task.JobID); err != nil {
			t.Fatalf("关闭当前投递: %v", err)
		}
		run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
		if err != nil {
			t.Fatalf("读取 pending run: %v", err)
		}
		updated, err := service.deliverPendingRun(context.Background(), task, *run)
		if err != nil {
			t.Fatalf("关闭投递后的 durable completion: %v", err)
		}
		if updated.DeliveryStatus != automationdomain.DeliveryStatusNotRequired || len(delivery.Calls()) != 0 {
			t.Fatalf("当前 mode none 必须跳过 router: run=%+v calls=%+v", updated, delivery.Calls())
		}
	})
}

func TestTerminalCommitReplacesCompletedOverlapRuntimeDeterministically(t *testing.T) {
	service, db, _, task := newInitialDeliveryTestService(t, "overlap-runtime")
	firstStarted := time.Now().UTC().Add(-2 * time.Minute)
	secondStarted := firstStarted.Add(time.Minute)
	for _, item := range []struct {
		runID     string
		startedAt time.Time
	}{
		{runID: "run-overlap-old", startedAt: firstStarted},
		{runID: "run-overlap-new", startedAt: secondStarted},
	} {
		if err := service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
			RunID: item.runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			TriggerKind: automationdomain.TriggerKindManual, DeliveryTarget: cloneDeliveryTargetPointer(task.Delivery),
		}); err != nil {
			t.Fatalf("创建 overlap run %s: %v", item.runID, err)
		}
		if err := service.repository.MarkRunRunning(context.Background(), item.runID, item.startedAt); err != nil {
			t.Fatalf("启动 overlap run %s: %v", item.runID, err)
		}
	}
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET running_run_id = ?, running_started_at = ? WHERE job_id = ?`, "run-overlap-old", firstStarted, task.JobID); err != nil {
		t.Fatalf("准备 overlap runtime: %v", err)
	}
	finishedAt := time.Now().UTC()
	result, err := service.repository.CommitRunTerminalAndRuntime(context.Background(), automationstore.RunTerminalCommitInput{
		OwnerUserID:                  task.OwnerUserID,
		JobID:                        task.JobID,
		ExpectedConfigurationVersion: task.ConfigurationVersion,
		Finish: automationstore.RunFinishInput{
			RunID: "run-overlap-old", Status: automationdomain.RunStatusSucceeded, FinishedAt: finishedAt,
			ResultText: stringPointer("result"), DeliveryStatus: automationdomain.DeliveryStatusPending,
		},
		Runtime: automationstore.JobRuntimeUpdateInput{
			JobID: task.JobID, LastRunAt: &finishedAt, LastRunStatus: automationdomain.RunStatusSucceeded,
			LastDeliveryStatus: automationdomain.DeliveryStatusPending,
		},
	})
	if err != nil || !result.Committed || !result.RuntimeUpdated {
		t.Fatalf("提交 overlap terminal: result=%+v err=%v", result, err)
	}
	current, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil {
		t.Fatalf("读取 overlap task: %v", err)
	}
	if current.RunningRunID != "run-overlap-new" || current.RunningStartedAt == nil || !current.RunningStartedAt.Equal(secondStarted) {
		t.Fatalf("完成旧 run 后必须回填确定的新 active run: %+v", current)
	}

	// A stale observation may finish after the task has moved on, but its older
	// finished_at must not replace the newer completion summary.
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: "run-overlap-stale", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual, DeliveryTarget: cloneDeliveryTargetPointer(task.Delivery),
	}); err != nil {
		t.Fatalf("创建 stale overlap run: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), "run-overlap-stale", firstStarted.Add(-time.Minute)); err != nil {
		t.Fatalf("启动 stale overlap run: %v", err)
	}
	staleFinishedAt := finishedAt.Add(-time.Hour)
	result, err = service.repository.CommitRunTerminalAndRuntime(context.Background(), automationstore.RunTerminalCommitInput{
		OwnerUserID:                  task.OwnerUserID,
		JobID:                        task.JobID,
		ExpectedConfigurationVersion: task.ConfigurationVersion,
		Finish: automationstore.RunFinishInput{
			RunID: "run-overlap-stale", Status: automationdomain.RunStatusSucceeded, FinishedAt: staleFinishedAt,
			ResultText: stringPointer("stale"), DeliveryStatus: automationdomain.DeliveryStatusPending,
		},
		Runtime: automationstore.JobRuntimeUpdateInput{
			JobID: task.JobID, LastRunAt: &staleFinishedAt, LastRunStatus: automationdomain.RunStatusSucceeded,
			LastDeliveryStatus: automationdomain.DeliveryStatusPending,
		},
	})
	if err != nil || !result.Committed || result.RuntimeUpdated {
		t.Fatalf("提交非当前 stale overlap: result=%+v err=%v", result, err)
	}
	var lastCompletedRunID string
	var storedLastRunAt time.Time
	if err = db.QueryRow(`SELECT last_completed_run_id, last_run_at FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).
		Scan(&lastCompletedRunID, &storedLastRunAt); err != nil {
		t.Fatalf("读取单调完成摘要: %v", err)
	}
	if lastCompletedRunID != "run-overlap-old" || !storedLastRunAt.Equal(finishedAt) {
		t.Fatalf("旧 finished_at 不得覆盖新完成摘要: last_completed=%s last_run_at=%v", lastCompletedRunID, storedLastRunAt)
	}
}

func newInitialDeliveryTestService(t *testing.T, name string) (*Service, *sql.DB, *fakeDeliveryRouter, automationdomain.ScheduledTask) {
	t.Helper()
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, &fakeWorkspaceReader{}, delivery)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        name,
		AgentID:     "agent-1",
		Instruction: "deliver durable result",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: name},
		Delivery:      fakeStructuredDelivery("agent-1", name),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	return service, db, delivery, *task
}

func insertActiveDeliveryRun(
	t *testing.T,
	db *sql.DB,
	service *Service,
	task automationdomain.ScheduledTask,
	runID string,
	target automationdomain.DeliveryTarget,
	result string,
) {
	t.Helper()
	if err := service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual, DeliveryTarget: cloneDeliveryTargetPointer(target),
	}); err != nil {
		t.Fatalf("InsertRunPending: %v", err)
	}
	startedAt := time.Now().UTC().Add(-time.Second)
	if err := service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("MarkRunRunning: %v", err)
	}
	if _, err := db.Exec(`UPDATE automation_task_runs SET result_text = ? WHERE run_id = ?`, result, runID); err != nil {
		t.Fatalf("写入 run result: %v", err)
	}
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET running_run_id = ?, running_started_at = ? WHERE job_id = ?`, runID, startedAt, task.JobID); err != nil {
		t.Fatalf("写入 task runtime: %v", err)
	}
}

func insertTerminalPendingDeliveryRun(
	t *testing.T,
	db *sql.DB,
	service *Service,
	task automationdomain.ScheduledTask,
	runID string,
	target automationdomain.DeliveryTarget,
	result string,
) {
	t.Helper()
	if err := service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual, Status: automationdomain.RunStatusSucceeded,
		DeliveryTarget: cloneDeliveryTargetPointer(target), DeliveryStatus: automationdomain.DeliveryStatusPending,
	}); err != nil {
		t.Fatalf("InsertRunPending terminal: %v", err)
	}
	finishedAt := time.Now().UTC().Add(-time.Second)
	if _, err := db.Exec(`UPDATE automation_task_runs
SET result_text = ?, finished_at = ?, delivery_attempts = 0 WHERE run_id = ?`, result, finishedAt, runID); err != nil {
		t.Fatalf("写入 terminal pending result: %v", err)
	}
}

func successfulRunFinish(runID string, deliveryStatus string) automationstore.RunFinishInput {
	finishedAt := time.Now().UTC()
	return automationstore.RunFinishInput{
		RunID: runID, Status: automationdomain.RunStatusSucceeded, FinishedAt: finishedAt,
		ResultText: stringPointer("result"), ResultSummary: stringPointer("result"), DeliveryStatus: deliveryStatus,
	}
}
