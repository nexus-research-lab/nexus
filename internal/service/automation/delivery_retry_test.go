package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"

	_ "modernc.org/sqlite"
)

type blockingDeliveryRouter struct {
	started     chan struct{}
	release     chan struct{}
	startedOnce sync.Once
	releaseOnce sync.Once
}

func newBlockingDeliveryRouter() *blockingDeliveryRouter {
	return &blockingDeliveryRouter{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (r *blockingDeliveryRouter) DeliverMessage(
	ctx context.Context,
	_ string,
	_ string,
	target channels.DeliveryTarget,
) (channels.DeliveryResult, error) {
	r.startedOnce.Do(func() { close(r.started) })
	select {
	case <-r.release:
		return channels.DeliveryResult{Target: target}, nil
	case <-ctx.Done():
		return channels.DeliveryResult{}, ctx.Err()
	}
}

func (r *blockingDeliveryRouter) unblock() {
	r.releaseOnce.Do(func() { close(r.release) })
}

func TestDeliveryRetryExactClaimAllowsOnlyOneAttemptAndLeavesUnknownOutOfAutoRetry(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, &fakeWorkspaceReader{}, delivery)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "exact-delivery-claim",
		AgentID:     "agent-1",
		Instruction: "deliver once",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "claim"},
		Delivery:      fakeStructuredDelivery("agent-1", "claim"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	runID := "run-exact-delivery-claim"
	if _, err = db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, delivery_status,
    delivery_attempts, result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, task.JobID, task.OwnerUserID, automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindManual, automationdomain.DeliveryStatusFailed, 1, "result", 1,
	); err != nil {
		t.Fatalf("准备 failed delivery 失败: %v", err)
	}

	type claimResult struct {
		attemptID string
		err       error
	}
	results := make(chan claimResult, 2)
	var start sync.WaitGroup
	start.Add(1)
	for _, attemptID := range []string{"attempt-a", "attempt-b"} {
		attemptID := attemptID
		go func() {
			start.Wait()
			claimErr := service.repository.ClaimRunDeliveryAttempt(context.Background(), automationstore.RunDeliveryAttemptClaimInput{
				OwnerUserID:                  task.OwnerUserID,
				JobID:                        task.JobID,
				RunID:                        runID,
				ExpectedDeliveryAttempts:     1,
				ExpectedConfigurationVersion: &task.ConfigurationVersion,
				ExpectedStatus:               automationdomain.DeliveryStatusFailed,
				AttemptID:                    attemptID,
			})
			results <- claimResult{attemptID: attemptID, err: claimErr}
		}()
	}
	start.Done()
	first := <-results
	second := <-results
	var winner string
	for _, result := range []claimResult{first, second} {
		if result.err == nil {
			if winner != "" {
				t.Fatalf("并发 claim 出现多个赢家: first=%v second=%v", first.err, second.err)
			}
			winner = result.attemptID
			continue
		}
		if !errors.Is(result.err, automationdomain.ErrDeliveryRetryConflict) {
			t.Fatalf("claim loser error = %v", result.err)
		}
	}
	if winner == "" {
		t.Fatalf("并发 claim 没有赢家: first=%v second=%v", first.err, second.err)
	}

	claimed, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 claimed run 失败: %v", err)
	}
	if claimed.DeliveryStatus != automationdomain.DeliveryStatusRetrying || claimed.DeliveryAttempts != 2 {
		t.Fatalf("claim 未原子推进状态和 attempts: %+v", claimed)
	}
	if err = service.repository.DeadLetterFailedRunDelivery(
		context.Background(), task.OwnerUserID, task.JobID, runID, 1, nil, time.Now(),
	); !errors.Is(err, automationdomain.ErrDeliveryRetryConflict) {
		t.Fatalf("过期自动 worker 不得覆盖 retrying claim: %v", err)
	}
	due, err := service.repository.ListDueDeliveryRetries(context.Background(), time.Now().Add(24*time.Hour), maxAutoDeliveryAttempts, 20)
	if err != nil || len(due) != 0 {
		t.Fatalf("retrying 不得进入自动重试: due=%+v err=%v", due, err)
	}
	if err = validateDeliveryRetry(*claimed); !errors.Is(err, automationdomain.ErrDeliveryRetryUnverified) {
		t.Fatalf("普通人工重试必须要求先核对未知结果: %v", err)
	}
	if _, err = service.RetryRunDelivery(context.Background(), task.JobID, runID); !errors.Is(err, automationdomain.ErrDeliveryRetryUnverified) {
		t.Fatalf("普通 retry API 不得重放未知外投: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("普通 retry API 不得产生外投: %+v", calls)
	}

	if err = service.repository.CompleteRunDeliveryAttempt(context.Background(), automationstore.RunDeliveryAttemptCompletionInput{
		OwnerUserID:    task.OwnerUserID,
		JobID:          task.JobID,
		RunID:          runID,
		AttemptID:      "loser-attempt",
		DeliveryStatus: automationdomain.DeliveryStatusSucceeded,
	}); !errors.Is(err, automationdomain.ErrDeliveryRetryCompletionUnconfirmed) {
		t.Fatalf("非持有者不得完成 attempt: %v", err)
	}
	confirmed, err := service.RetryUnverifiedRunDeliveryAtVersion(
		context.Background(), task.JobID, runID, task.ConfigurationVersion, 2,
	)
	if err != nil {
		t.Fatalf("用户核对后的 exact retry 失败: %v", err)
	}
	if confirmed.DeliveryStatus != automationdomain.DeliveryStatusSucceeded || confirmed.DeliveryAttempts != 3 {
		t.Fatalf("显式核对后的投递状态不正确: %+v", confirmed)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("显式核对后必须且只能产生一次外投: %+v", calls)
	}
}

func TestVersionedDeliveryRetryRejectsStaleExpectedAttemptsBeforeExternalCall(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, &fakeWorkspaceReader{}, delivery)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "stale delivery attempts", AgentID: "agent-1", Instruction: "deliver",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "attempt-fence"},
		Delivery:      fakeStructuredDelivery("agent-1", "attempt-fence"), Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	runID := "run-stale-delivery-attempts"
	if _, err = db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, delivery_status,
    delivery_attempts, result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, task.JobID, task.OwnerUserID, automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindManual, automationdomain.DeliveryStatusFailed, 2, "result", 1,
	); err != nil {
		t.Fatalf("prepare failed delivery: %v", err)
	}

	if _, err = service.RetryRunDeliveryAtVersionAndAttempts(
		context.Background(), task.JobID, runID, task.ConfigurationVersion, 1,
	); !errors.Is(err, automationdomain.ErrDeliveryRetryConflict) {
		t.Fatalf("stale delivery attempts must conflict: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("stale delivery attempts must fail before external call: %+v", calls)
	}
	if _, err = db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, delivery_status,
    delivery_attempts, result_text, attempts, finished_at
) VALUES ('run-current-delivery-authority', ?, ?, ?, ?, ?, 0, 'newer result', 1, ?)`,
		task.JobID, task.OwnerUserID, automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindManual, automationdomain.DeliveryStatusPending, time.Now().UTC(),
	); err != nil {
		t.Fatalf("prepare latest completed run: %v", err)
	}
	if _, err = db.Exec(`UPDATE automation_scheduled_tasks
SET last_completed_run_id = 'run-current-delivery-authority', last_delivery_status = 'pending'
WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatalf("bind latest completed authority: %v", err)
	}
	if _, err = service.RetryRunDeliveryAtVersionAndAttempts(
		context.Background(), task.JobID, runID, task.ConfigurationVersion, 2,
	); err != nil {
		t.Fatalf("retry historical failed run: %v", err)
	}
	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("load task after historical retry: %v", err)
	}
	if updatedTask.LastDeliveryStatus != automationdomain.DeliveryStatusPending {
		t.Fatalf("historical service retry overwrote latest task summary: %+v", updatedTask)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("historical retry should update its own run exactly once: %+v", calls)
	}
}

func TestSlowDeliveryRetryDoesNotBlockUnrelatedTaskConfiguration(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := newBlockingDeliveryRouter()
	defer delivery.unblock()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	createTask := func(name string, target automationdomain.DeliveryTarget) *automationdomain.ScheduledTask {
		t.Helper()
		task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
			Name: name, AgentID: "agent-1", Instruction: "deliver",
			Schedule: automationdomain.Schedule{
				Kind:            automationdomain.ScheduleKindEvery,
				IntervalSeconds: intRef(3600),
				Timezone:        "UTC",
			},
			SessionTarget: automationdomain.SessionTarget{
				Kind:            automationdomain.SessionTargetNamed,
				NamedSessionKey: name,
			},
			Delivery: target,
			Enabled:  true,
		})
		if err != nil {
			t.Fatalf("CreateTask(%s): %v", name, err)
		}
		return task
	}
	deliveryTask := createTask(
		"slow-delivery",
		fakeStructuredDelivery("agent-1", "slow-delivery"),
	)
	unrelatedTask := createTask(
		"unrelated-config",
		automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
	)
	runID := "run-slow-delivery"
	if _, err := db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, delivery_status,
    delivery_attempts, result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		deliveryTask.JobID,
		deliveryTask.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindManual,
		automationdomain.DeliveryStatusFailed,
		1,
		"result",
		1,
	); err != nil {
		t.Fatalf("prepare failed delivery: %v", err)
	}

	retryResult := make(chan error, 1)
	go func() {
		_, err := service.RetryRunDeliveryAtVersionAndAttempts(
			context.Background(),
			deliveryTask.JobID,
			runID,
			deliveryTask.ConfigurationVersion,
			1,
		)
		retryResult <- err
	}()
	select {
	case <-delivery.started:
	case <-time.After(time.Second):
		t.Fatal("delivery router did not start")
	}

	updatedName := "unrelated-config-updated"
	updateResult := make(chan error, 1)
	go func() {
		_, err := service.UpdateTaskAtVersion(
			context.Background(),
			unrelatedTask.JobID,
			unrelatedTask.ConfigurationVersion,
			automationdomain.UpdateJobInput{Name: &updatedName},
		)
		updateResult <- err
	}()
	select {
	case err := <-updateResult:
		if err != nil {
			t.Fatalf("unrelated task update failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("slow external delivery blocked an unrelated task configuration")
	}

	delivery.unblock()
	select {
	case err := <-retryResult:
		if err != nil {
			t.Fatalf("delivery retry failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("delivery retry did not settle after router release")
	}
}

func TestAutomaticAndManualDeliveryRetryCannotBothProduceExternalSideEffects(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, &fakeWorkspaceReader{}, delivery)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "manual-auto-retry-race",
		AgentID:     "agent-1",
		Instruction: "deliver once",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "retry-race"},
		Delivery:      fakeStructuredDelivery("agent-1", "retry-race"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	runID := "run-manual-auto-retry-race"
	dueAt := time.Now().UTC().Add(-time.Minute)
	if _, err = db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, delivery_status,
    delivery_attempts, delivery_next_attempt_at, result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, task.JobID, task.OwnerUserID, automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindManual, automationdomain.DeliveryStatusFailed, 1, dueAt, "result", 1,
	); err != nil {
		t.Fatalf("准备 failed delivery 失败: %v", err)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 race run 失败: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, retryErr := service.RetryRunDelivery(context.Background(), task.JobID, runID)
		results <- retryErr
	}()
	go func() {
		<-start
		results <- service.retryDueRunDelivery(context.Background(), *run)
	}()
	close(start)
	firstErr := <-results
	secondErr := <-results
	if firstErr != nil && secondErr != nil {
		t.Fatalf("并发 retry 至少应有一个成功: first=%v second=%v", firstErr, secondErr)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("人工与自动并发只能产生一次外投: %+v", calls)
	}
	updated, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 race 结果失败: %v", err)
	}
	if updated.DeliveryStatus != automationdomain.DeliveryStatusSucceeded || updated.DeliveryAttempts != 2 {
		t.Fatalf("并发 retry durable 结果不正确: %+v", updated)
	}
}

func TestServiceRevalidatesIMPairingAtCreateDeliveryAndRetry(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("temporary IM send failure")}
	grant := &mutableAutomationDeliveryGrant{allowed: true}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "定时结果"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	service.SetDeliveryGrantResolver(grant)
	ownerCtx := contextForOwner(context.Background(), "user-1")
	sessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-1",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"weixin-account",
		"weixin-user",
		"",
	)
	create := func(name string) (*automationdomain.ScheduledTask, error) {
		return service.CreateTask(ownerCtx, automationdomain.CreateJobInput{
			Name:        name,
			AgentID:     "agent-1",
			Instruction: "执行后回传",
			Schedule: automationdomain.Schedule{
				Kind:            automationdomain.ScheduleKindEvery,
				IntervalSeconds: intRef(3600),
				Timezone:        "Asia/Shanghai",
			},
			SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: name},
			Delivery: automationdomain.DeliveryTarget{
				Mode:       automationdomain.DeliveryModeLast,
				SessionKey: sessionKey,
			},
			Source:  automationdomain.Source{Kind: automationdomain.SourceKindCLI, SessionKey: sessionKey},
			Enabled: true,
		})
	}
	task, err := create("pairing-runtime")
	if err != nil {
		t.Fatalf("active pairing 下创建任务失败: %v", err)
	}
	service.delivery = nil
	if _, err = service.RunTaskNow(ownerCtx, task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	var run automationdomain.ScheduledTaskRun
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(ownerCtx, task.JobID)
		if listErr != nil || len(runs) != 1 || runs[0].DeliveryStatus != automationdomain.DeliveryStatusFailed {
			return false
		}
		run = runs[0]
		return true
	})
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("缺少 router 的确定失败不得调用 IM adapter: %+v", calls)
	}

	service.delivery = delivery
	grant.setAllowed(false)
	if _, err = create("pairing-create-revoked"); !errors.Is(err, automationdomain.ErrTaskDeliverySessionUnavailable) {
		t.Fatalf("pairing 撤销后新建任务必须 fail closed: %v", err)
	}
	delivery.err = nil
	retried, err := service.RetryRunDelivery(ownerCtx, task.JobID, run.RunID)
	if err != nil {
		t.Fatalf("重试 ledger 更新失败: %v", err)
	}
	if retried.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		retried.DeliveryError == nil ||
		!strings.Contains(*retried.DeliveryError, automationdomain.ErrTaskDeliverySessionUnavailable.Error()) {
		t.Fatalf("pairing 撤销后重试必须记录授权失败: %+v", retried)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("撤销 pairing 后重试不得到达 IM adapter: %+v", calls)
	}
}

func TestServiceDeliveryRouterErrorDoesNotFailExecutionAndRequiresVerification(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("feishu send message failed: bad chat_id")}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "今日新闻摘要"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "news",
		AgentID:     "agent-1",
		Instruction: "search news",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "news"},
		Delivery:      fakeStructuredDelivery("agent-1", "bad-delivery-session"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		current, taskErr := service.GetTask(context.Background(), task.JobID)
		return listErr == nil && taskErr == nil && len(items) > 0 &&
			items[0].DeliveryStatus == automationdomain.DeliveryStatusRetrying &&
			current.LastRunStatus == automationdomain.RunStatusSucceeded && !current.Running
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].Status != automationdomain.RunStatusSucceeded {
		t.Fatalf("投递失败不应把执行状态改成 failed: %+v", runs[0])
	}
	if runs[0].DeliveryError == nil || !strings.Contains(*runs[0].DeliveryError, "bad chat_id") {
		t.Fatalf("delivery_error 未记录失败原因: %+v", runs[0])
	}
	if runs[0].DeliveryAttempts != 1 {
		t.Fatalf("delivery_attempts = %d, 期望 1", runs[0].DeliveryAttempts)
	}
	if runs[0].DeliveryNextAttemptAt != nil || runs[0].DeliveryDeadLetterAt != nil {
		t.Fatalf("结果未知时不得安排自动重试或伪造死信: %+v", runs[0])
	}
	deadline, err := service.repository.NextDeliveryRetryAt(
		context.Background(),
		maxAutoDeliveryAttempts,
	)
	if err != nil || deadline != nil {
		t.Fatalf("unverified delivery 不得进入自动 deadline: %v, err=%v", deadline, err)
	}
	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if updatedTask.LastRunStatus != automationdomain.RunStatusSucceeded || updatedTask.FailureStreak != 0 {
		t.Fatalf("投递失败不应触发任务级执行失败退避: %+v", updatedTask)
	}
	if updatedTask.LastDeliveryStatus != automationdomain.DeliveryStatusRetrying {
		t.Fatalf("last_delivery_status 未记录需要人工核对: %+v", updatedTask)
	}
	delivery.err = nil
	service.retryDueDeliveries(context.Background(), time.Now().Add(24*time.Hour))
	if _, err = service.RetryRunDelivery(context.Background(), task.JobID, runs[0].RunID); !errors.Is(err, automationdomain.ErrDeliveryRetryUnverified) {
		t.Fatalf("普通人工 retry 也必须先核对接收端: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("未知结果后不得产生第二次外投: %+v", calls)
	}
}

func TestVersionedDeliveryRetryKeepsTheRouteFromItsCheckedTaskSnapshot(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("temporary delivery failure")}
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "需要补投递的结果"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	originalDelivery := fakeStructuredDelivery("agent-1", "versioned-route-original")
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "版本化补投递",
		AgentID:     "agent-1",
		Instruction: "生成结果",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "versioned-route"},
		Delivery:      originalDelivery,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	service.delivery = nil
	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	var failedRun automationdomain.ScheduledTaskRun
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(runs) != 1 || runs[0].DeliveryStatus != automationdomain.DeliveryStatusFailed {
			return false
		}
		failedRun = runs[0]
		return true
	})
	checkedSnapshot, err := service.GetTask(context.Background(), task.JobID)
	if err != nil || checkedSnapshot == nil {
		t.Fatalf("读取已核对任务快照失败: task=%+v err=%v", checkedSnapshot, err)
	}
	service.delivery = delivery
	newDelivery := fakeStructuredDelivery("agent-1", "versioned-route-newer")
	if _, err = service.UpdateTask(
		context.Background(),
		task.JobID,
		automationdomain.UpdateJobInput{Delivery: &newDelivery},
	); err != nil {
		t.Fatalf("并发更新投递目标失败: %v", err)
	}
	if _, err = service.RetryRunDeliveryAtVersion(
		context.Background(),
		task.JobID,
		failedRun.RunID,
		checkedSnapshot.ConfigurationVersion,
	); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("旧版本重试必须在投递前失败: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("版本冲突不应触发额外投递: %+v", calls)
	}
	delivery.mu.Lock()
	delivery.err = nil
	delivery.mu.Unlock()
	if _, err = service.retryLoadedRunDelivery(
		context.Background(),
		task.OwnerUserID,
		*checkedSnapshot,
		failedRun,
		true,
	); !errors.Is(err, automationdomain.ErrDeliveryRetryConflict) {
		t.Fatalf("过期服务快照必须被 DB 配置 fence 拒绝: %v", err)
	}
	calls := delivery.Calls()
	if len(calls) != 0 {
		t.Fatalf("DB 配置 fence 失败时不得产生外投副作用: calls=%+v", calls)
	}
}

func TestServiceRunDueOnceRetriesDueDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return base }
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "auto-redelivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery:      fakeStructuredDelivery("agent-1", "auto-redelivery"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-due-delivery"
	dueAt := base.Add(5 * time.Minute)
	scheduledFor := base.Add(-time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"explicit:feishu:oc_old",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		dueAt,
		scheduledFor,
		scheduledFor.Add(time.Minute),
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 due delivery run 失败: %v", err)
	}

	service.nowFn = func() time.Time { return dueAt.Add(time.Second) }
	service.runDueOnce()

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil &&
			len(runs) > 0 &&
			runs[0].RunID == runID &&
			runs[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取调度自动重试后的 run 失败: runs=%+v err=%v", runs, err)
	}
	redelivered := runs[0]
	if redelivered.DeliveryAttempts != 2 || redelivered.DeliveryError != nil || redelivered.DeliveryNextAttemptAt != nil {
		t.Fatalf("调度 tick 自动重试后状态不正确: %+v", redelivered)
	}
	if redelivered.DeliveryTo != "explicit:internal:"+task.Delivery.SessionKey {
		t.Fatalf("自动重试应使用任务当前投递目标，实际 delivery_to=%q", redelivered.DeliveryTo)
	}
	calls := delivery.Calls()
	if len(calls) != 1 || calls[0].To != task.Delivery.SessionKey {
		t.Fatalf("调度 tick 应自动投递到当前目标，calls=%+v", calls)
	}
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionAutoRetryDelivery && event.RunID == runID {
			return
		}
	}
	t.Fatalf("调度 tick 自动重试应写入审计事件: %+v", events)
}
