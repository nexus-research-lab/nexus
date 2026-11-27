// delivery_test.go 覆盖自动化任务投递权威、成功、重试、死信、初始可靠性与 IM 反馈测试。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

type mutableAutomationAgentAuthority struct {
	mu     sync.Mutex
	agents map[string]protocol.Agent
}

type fakeAutomationDeliverySessionResolver struct {
	sessions map[string]protocol.Session
}

func (f fakeAutomationDeliverySessionResolver) ResolveDeliverySession(
	_ context.Context,
	sessionKey string,
) (*protocol.Session, error) {
	item, ok := f.sessions[strings.TrimSpace(sessionKey)]
	if !ok {
		return nil, nil
	}
	result := item
	return &result, nil
}

func (f *mutableAutomationAgentAuthority) EnsureReady(context.Context) error {
	return nil
}

func (f *mutableAutomationAgentAuthority) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.agents[strings.TrimSpace(agentID)]
	if !ok {
		return nil, nil
	}
	result := value
	return &result, nil
}

func (f *mutableAutomationAgentAuthority) setMain(agentID string, isMain bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value := f.agents[strings.TrimSpace(agentID)]
	value.IsMain = isMain
	f.agents[strings.TrimSpace(agentID)] = value
}

func TestServiceRejectsPageDeliveryToCrossOwnerAgent(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil, nil, nil, nil, nil, nil,
	)
	service.agents = &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"agent-a": {AgentID: "agent-a", OwnerUserID: "user-1", Status: "active"},
		"agent-b": {AgentID: "agent-b", OwnerUserID: "user-2", Status: "active"},
	}}
	recipientSession := protocol.BuildAgentSessionKey(
		"agent-b", protocol.SessionChannelInternalSegment, protocol.RoomTypeDM,
		"recipient-session", "",
	)
	_, err := service.CreateTask(automationCommandTestOwnerContext("user-1"), automationdomain.CreateJobInput{
		Name: "cross owner", AgentID: "agent-a", Instruction: "prepare report",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(60), Timezone: "Asia/Shanghai"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery: automationdomain.DeliveryTarget{
			Mode: automationdomain.DeliveryModeExplicit, Channel: protocol.SessionChannelInternalSegment,
			To: recipientSession, SessionKey: recipientSession,
		},
		Source:  automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled: true,
	})
	if err == nil || !strings.Contains(err.Error(), "must be owned") {
		t.Fatalf("cross-owner recipient must fail closed, got %v", err)
	}
}

func TestServiceOwnerMainGrantIsRevalidatedBeforeDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	workerWorkspace := newAutomationOwnerWorkspace(t, "user-1", "worker")
	authority := &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"main": {
			AgentID:       "main",
			OwnerUserID:   "user-1",
			Status:        "active",
			IsMain:        true,
			WorkspacePath: workerWorkspace,
		},
		"worker": {
			AgentID:       "worker",
			OwnerUserID:   "user-1",
			Status:        "active",
			WorkspacePath: workerWorkspace,
		},
	}}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workerWorkspace},
		db,
		nil,
		nil,
		nil,
		nil,
		nil,
		delivery,
	)
	service.agents = authority
	ownerCtx := automationCommandTestOwnerContext("user-1")
	mainCtx := automationexec.WithActorAgentID(ownerCtx, "main")
	mainSession := protocol.BuildAgentSessionKey(
		"main",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"owner",
		"",
	)
	workerSession := protocol.BuildAgentSessionKey(
		"worker",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"owner-selected",
		"",
	)
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		workerSession: {
			SessionKey: workerSession, AgentID: "worker", ChannelType: protocol.SessionChannelInternalSegment,
		},
	}})

	created, err := service.CreateTask(mainCtx, automationdomain.CreateJobInput{
		Name:        "main-granted-channel",
		AgentID:     "worker",
		Instruction: "send a report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "main-granted-channel",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeExplicit,
			Channel:    protocol.SessionChannelInternalSegment,
			To:         workerSession,
			SessionKey: workerSession,
		},
		Source: automationdomain.Source{
			Kind:           automationdomain.SourceKindAgent,
			CreatorAgentID: "main",
			ContextType:    "agent",
			ContextID:      "main",
			SessionKey:     mainSession,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("current owner-main should grant arbitrary owner-scoped delivery: %v", err)
	}

	authority.setMain("main", false)
	result := service.deliverJobObservation(
		ownerCtx,
		*created,
		"",
		automationexec.ExecutionObservation{ResultText: "sensitive report"},
	)
	if result.Status != automationdomain.DeliveryStatusFailed ||
		result.Error == nil ||
		!strings.Contains(*result.Error, "cannot grant automation delivery") {
		t.Fatalf("revoked owner-main authority should fail closed: %+v", result)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("revoked owner-main authority reached delivery router: %+v", calls)
	}
}

func TestRoomDeliveryRevalidatesCurrentMembership(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	room := &fakeRoomRunner{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		room,
		nil,
		nil,
		delivery,
	)
	ownerCtx := automationCommandTestOwnerContext("user-1")
	agentCtx := automationexec.WithActorAgentID(ownerCtx, "agent-1")
	roomSession := protocol.BuildRoomSharedSessionKey("conversation-1")
	created, err := service.CreateTask(agentCtx, automationdomain.CreateJobInput{
		Name:        "room-report",
		AgentID:     "agent-1",
		Instruction: "send to current room",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "room-report",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: protocol.SessionChannelWebSocket,
			To:      roomSession,
		},
		Source: automationdomain.Source{
			Kind:           automationdomain.SourceKindAgent,
			CreatorAgentID: "agent-1",
			ContextType:    "room",
			ContextID:      "room-1",
			SessionKey:     roomSession,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("current Room member should grant delivery to that Room: %v", err)
	}
	if created.Delivery.AgentID != "agent-1" {
		t.Fatalf("blank Room reply agent should resolve to host: %+v", created.Delivery)
	}

	room.contexts = map[string]*protocol.ConversationContextAggregate{
		"conversation-1": {
			Room:         protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
			Conversation: protocol.ConversationRecord{ID: "conversation-1", RoomID: "room-1"},
		},
	}
	result := service.deliverJobObservation(
		ownerCtx,
		*created,
		"",
		automationexec.ExecutionObservation{ResultText: "former member output"},
	)
	if result.Status != automationdomain.DeliveryStatusFailed ||
		result.Error == nil ||
		!strings.Contains(*result.Error, "no longer a member") {
		t.Fatalf("revoked Room membership should fail closed: %+v", result)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("revoked Room member reached delivery router: %+v", calls)
	}
}

func TestServiceAutoRetryDeliveryDeadLettersDisabledTask(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "paused-delivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery:      fakeStructuredDelivery("agent-1", "paused-delivery"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = service.UpdateTaskStatus(context.Background(), task.JobID, false); err != nil {
		t.Fatalf("停用任务失败: %v", err)
	}

	runID := "run-disabled-delivery"
	dueAt := base.Add(time.Minute)
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
		"explicit:feishu:oc_group",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		dueAt,
		dueAt.Add(-time.Minute),
		dueAt,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 disabled due delivery run 失败: %v", err)
	}

	service.nowFn = func() time.Time { return dueAt.Add(time.Second) }
	service.retryDueDeliveries(context.Background(), dueAt.Add(time.Second))

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取自动重试跳过后的 run 失败: runs=%+v err=%v", runs, err)
	}
	updated := runs[0]
	if updated.RunID != runID ||
		updated.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		updated.DeliveryDeadLetterAt == nil ||
		updated.DeliveryNextAttemptAt != nil {
		t.Fatalf("停用任务的 due delivery 应进入死信并清理下一次重试: %+v", updated)
	}
	if updated.DeliveryAttempts != 1 {
		t.Fatalf("停用任务不应发生新的投递尝试，attempts=%d", updated.DeliveryAttempts)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("停用任务不应继续自动投递，calls=%+v", delivery.Calls())
	}
	dueRuns, err := service.repository.ListDueDeliveryRetries(context.Background(), dueAt.Add(2*time.Second), maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		t.Fatalf("重新读取 due delivery 失败: %v", err)
	}
	for _, dueRun := range dueRuns {
		if dueRun.RunID == runID {
			t.Fatalf("死信后的 disabled delivery 不应再次进入自动重试队列: %+v", dueRuns)
		}
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试跳过事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionAutoRetryDelivery &&
			event.RunID == runID &&
			event.Detail["auto_retry_skipped_reason"] == "task_disabled" {
			return
		}
	}
	t.Fatalf("停用任务的自动重试跳过应写入审计事件: %+v", events)
}

func TestDeleteTaskDeadLettersPendingDeliveryRetries(t *testing.T) {
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
		Name:        "delete-with-failed-delivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery:      fakeStructuredDelivery("agent-1", "delete-with-failed-delivery"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-delete-delivery"
	nextAttemptAt := base.Add(10 * time.Minute)
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
		"explicit:feishu:oc_group",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		nextAttemptAt,
		base,
		base.Add(time.Minute),
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备待补投递 run 失败: %v", err)
	}

	deletedAt := base.Add(time.Minute)
	service.nowFn = func() time.Time { return deletedAt }
	if _, err = service.DeleteTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("删除后读取 run ledger 失败: runs=%+v err=%v", runs, err)
	}
	updated := runs[0]
	if updated.RunID != runID ||
		updated.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		updated.DeliveryDeadLetterAt == nil ||
		updated.DeliveryNextAttemptAt != nil ||
		updated.DeliveryError == nil ||
		!strings.Contains(*updated.DeliveryError, "deleted") {
		t.Fatalf("删除任务应立即把待补投递 run 标记为死信: %+v", updated)
	}
	if updated.DeliveryAttempts != 1 {
		t.Fatalf("删除任务不应新增投递尝试，attempts=%d", updated.DeliveryAttempts)
	}
	dueRuns, err := service.repository.ListDueDeliveryRetries(context.Background(), deletedAt.Add(time.Hour), maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		t.Fatalf("读取 due delivery 失败: %v", err)
	}
	for _, dueRun := range dueRuns {
		if dueRun.RunID == runID {
			t.Fatalf("删除任务后的死信 run 不应进入自动重试队列: %+v", dueRuns)
		}
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("删除任务不应触发投递，calls=%+v", delivery.Calls())
	}
	report, err := service.GetDailyReport(context.Background(), automationdomain.ScheduledTaskDailyReportInput{
		Date:     "2026-05-21",
		Timezone: "UTC",
		JobID:    task.JobID,
	})
	if err != nil {
		t.Fatalf("删除任务后读取日报失败: %v", err)
	}
	if len(report.Tasks) != 1 {
		t.Fatalf("删除任务日报应返回一条任务明细: %+v", report)
	}
	dailyTask := report.Tasks[0]
	if !dailyTask.Deleted ||
		!containsString(dailyTask.Signals, "deleted") ||
		!containsString(dailyTask.Signals, "delivery_attention") ||
		!containsString(dailyTask.DeliveryDeadLetterRunIDs, runID) ||
		containsString(dailyTask.ManualRedeliveryRunIDs, runID) ||
		!containsString(dailyTask.SuggestedTools, runtimeAutomationInspectSuggestion) ||
		containsString(dailyTask.SuggestedTools, runtimeAutomationApplySuggestion) {
		t.Fatalf("删除任务日报应保留失败信号但不建议不可执行补投递: %+v", dailyTask)
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("删除后读取事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action != automationdomain.TaskEventActionDelete {
			continue
		}
		values, ok := event.Detail["dead_lettered_delivery_run_ids"].([]any)
		if !ok || len(values) != 1 || values[0] != runID {
			t.Fatalf("delete 事件应记录被死信的投递 run: %+v", event.Detail)
		}
		return
	}
	t.Fatalf("删除任务应写入 delete 事件: %+v", events)
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

func TestServiceRunTaskNowDeliversLegacyAgentAutomationInbox(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: "今日新闻摘要",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)

	inboxKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "新闻投递到智能体",
		AgentID:     "agent-1",
		Instruction: "搜索新闻并输出摘要",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "news",
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	legacyTask := *task
	legacyTask.Delivery = automationdomain.DeliveryTarget{
		Mode:       automationdomain.DeliveryModeExplicit,
		Channel:    protocol.SessionChannelInternalSegment,
		To:         inboxKey,
		SessionKey: inboxKey,
	}
	legacyTask.DeliveryGrant = automationdomain.Source{Kind: automationdomain.SourceKindUserPage}.Normalized()
	task, err = service.repository.UpsertScheduledTask(context.Background(), legacyTask)
	if err != nil {
		t.Fatalf("准备历史收件箱任务失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})

	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionValue, _, err := store.FindSession([]string{workspacePath}, inboxKey)
	if err != nil {
		t.Fatalf("读取智能体收件箱 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("投递到智能体时应自动创建定时任务收件箱")
	}
	if sessionValue.AgentID != "agent-1" {
		t.Fatalf("收件箱应归属目标智能体，实际 %+v", sessionValue)
	}
	if sessionValue.Title != "定时任务收件箱" || sessionValue.ChannelType != protocol.SessionChannelInternalSegment {
		t.Fatalf("收件箱元数据不正确: %+v", sessionValue)
	}

	assertDeliveredAgentMessage(t, workspacePath, *sessionValue, "今日新闻摘要", "智能体收件箱")
	assertRunDeliveredTo(t, service, task.JobID, "explicit:internal:"+inboxKey)
}

func TestAutomationCommandCreateRunAndInspectDeliversToCurrentSession(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, "user-1", "agent-1")
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: "今日新闻摘要",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)
	sctx := command.Actor{
		AgentID:            "agent-1",
		AgentName:          "新闻智能体",
		OwnerUserID:        "user-1",
		SessionKey:         protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "operator", ""),
		SessionLabel:       "用户对话",
		SourceContextType:  "agent",
		SourceContextID:    "agent-1",
		SourceContextLabel: "新闻智能体",
		DefaultTimezone:    "Asia/Shanghai",
	}
	recipientSessionKey := sctx.SessionKey
	prepareAutomationDeliverySession(t, workspacePath, "user-1", "agent-1", recipientSessionKey)

	createResult, isError := callAutomationCommand(t, service, sctx, "create_scheduled_task", map[string]any{
		"name":                       "新闻投递到智能体",
		"instruction":                "每天搜索新闻并输出摘要",
		"execution_mode":             "dedicated",
		"named_session_key":          "news-search",
		"reply_mode":                 "selected",
		"selected_reply_session_key": recipientSessionKey,
		"schedule": map[string]any{
			"kind":       "daily",
			"daily_time": "09:00",
			"timezone":   "Asia/Shanghai",
		},
	})
	if isError {
		t.Fatalf("create_scheduled_task 不应失败: %s", automationCommandText(t, createResult))
	}
	created := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, createResult)
	if created.AgentID != "agent-1" {
		t.Fatalf("CLI 创建任务应归属调用智能体，实际 %+v", created)
	}
	if created.Delivery.Mode != automationdomain.DeliveryModeExplicit ||
		created.Delivery.Channel != protocol.SessionChannelInternalSegment ||
		created.Delivery.To != recipientSessionKey {
		t.Fatalf("CLI selected 应绑定真实当前会话，实际 %+v", created.Delivery)
	}
	if created.Source.Kind != automationdomain.SourceKindAgent || created.Source.CreatorAgentID != "agent-1" {
		t.Fatalf("CLI 创建任务应记录 Agent 来源，实际 %+v", created.Source)
	}

	runResult, isError := callAutomationCommand(t, service, sctx, "run_scheduled_task", map[string]any{
		"query": "新闻投递到智能体",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationCommandText(t, runResult))
	}
	runNow := decodeAutomationCommandJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.JobID != created.JobID {
		t.Fatalf("query 应定位到刚创建的任务，run=%+v created=%+v", runNow, created)
	}

	ownerCtx := automationCommandTestOwnerContext(sctx.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(ownerCtx, created.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})

	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionValue, _, err := store.FindSession([]string{workspacePath}, recipientSessionKey)
	if err != nil {
		t.Fatalf("读取 CLI 真实当前 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("CLI 创建并运行后应写入真实当前会话")
	}
	if sessionValue.AgentID != "agent-1" {
		t.Fatalf("CLI 投递会话应归属目标智能体，实际 %+v", sessionValue)
	}
	assertDeliveredAgentMessage(t, workspacePath, *sessionValue, "今日新闻摘要", "CLI 真实当前会话")
	assertRunDeliveredToContext(t, ownerCtx, service, created.JobID, "explicit:internal:"+recipientSessionKey)

	statusResult, isError := callAutomationCommand(t, service, sctx, "inspect_scheduled_task", map[string]any{
		"query":       "新闻投递到智能体",
		"run_limit":   5,
		"event_limit": 5,
	})
	if isError {
		t.Fatalf("inspect_scheduled_task by query 不应失败: %s", automationCommandText(t, statusResult))
	}
	status := decodeAutomationCommandJSON[automationdomain.ScheduledTaskStatus](t, statusResult)
	if status.Job.JobID != created.JobID || status.Job.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("CLI 状态应能看到任务最新投递成功，实际 %+v", status.Job)
	}
	if len(status.RecentRuns) == 0 || status.RecentRuns[0].DeliveryTo != "explicit:internal:"+recipientSessionKey {
		t.Fatalf("CLI 状态应返回最近投递目标，实际 %+v", status.RecentRuns)
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

func TestIMFeedbackPolicyUsesExactRunAndCurrentRestrictions(t *testing.T) {
	service, db, _, task := newInitialDeliveryTestService(t, "im-feedback")
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: task.OwnerUserID, Role: authctx.RoleOwner})
	source := imdelivery.Source{Kind: "automation", AgentID: task.AgentID, JobID: task.JobID, RunID: "im-run", SessionKey: "source-session", RoundID: "source-round"}
	if err := service.repository.InsertRunPending(ctx, automationstore.RunPendingInput{RunID: source.RunID, JobID: source.JobID, OwnerUserID: task.OwnerUserID, SessionKey: source.SessionKey, RoundID: source.RoundID, PermissionPolicyRevision: task.PermissionPolicy.Revision, Status: automationdomain.RunStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	policy, err := service.IMFeedbackPolicy(ctx, source)
	if err != nil || policy == nil || !reflect.DeepEqual(policy, taskRuntimeToolPolicy(task)) {
		t.Fatalf("policy %+v %v", policy, err)
	}
	for _, mutate := range []func(*imdelivery.Source){func(s *imdelivery.Source) { s.RoundID = "other" }, func(s *imdelivery.Source) { s.SessionKey = "other" }, func(s *imdelivery.Source) { s.AgentID = "other" }, func(s *imdelivery.Source) { s.RunID = "other" }} {
		bad := source
		mutate(&bad)
		if _, err := service.IMFeedbackPolicy(ctx, bad); err == nil {
			t.Fatal("foreign producer accepted")
		}
	}
	if _, err = db.Exec(`UPDATE automation_task_runs SET permission_policy_revision=permission_policy_revision+1 WHERE run_id=?`, source.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.IMFeedbackPolicy(ctx, source); err == nil {
		t.Fatal("stale policy accepted")
	}
	run, err := service.repository.GetRun(ctx, task.OwnerUserID, task.JobID, source.RunID)
	if err != nil || run.Status != automationdomain.RunStatusSucceeded {
		t.Fatalf("feedback changed old run %+v %v", run, err)
	}
}
