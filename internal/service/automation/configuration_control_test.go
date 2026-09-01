package automation

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

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

func TestCreateTaskReplayPrecedesMutableExpirationValidation(t *testing.T) {
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
	createdAt := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return createdAt }
	expiresAt := createdAt.Add(time.Hour)
	input := automationConfigurationTaskInput("replay-after-expiry")
	input.RequestID = "request-replay-after-expiry"
	input.ExpiresAt = &expiresAt

	first, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("first CreateTask: %v", err)
	}
	service.nowFn = func() time.Time { return expiresAt.Add(time.Hour) }
	replayed, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("committed replay must not re-run expiration validation: %v", err)
	}
	if replayed.JobID != first.JobID {
		t.Fatalf("replay job_id = %q, want %q", replayed.JobID, first.JobID)
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

func TestScheduledTaskCASRejectsStaleUpdateAndDelete(t *testing.T) {
	db := newAutomationTestDB(t)
	firstService := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	secondService := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)

	created, err := firstService.CreateTask(context.Background(), automationConfigurationTaskInput("versioned"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if created.ConfigurationVersion != 1 {
		t.Fatalf("initial version = %d, want 1", created.ConfigurationVersion)
	}
	stale, err := secondService.GetTask(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("GetTask stale snapshot: %v", err)
	}
	name := "updated"
	updated, err := firstService.UpdateTaskAtVersion(
		context.Background(),
		created.JobID,
		created.ConfigurationVersion,
		automationdomain.UpdateJobInput{Name: &name},
	)
	if err != nil {
		t.Fatalf("UpdateTaskAtVersion: %v", err)
	}
	if updated.ConfigurationVersion != created.ConfigurationVersion+1 {
		t.Fatalf("updated version = %d, want %d", updated.ConfigurationVersion, created.ConfigurationVersion+1)
	}
	staleName := "stale overwrite"
	if _, err = secondService.UpdateTaskAtVersion(
		context.Background(),
		created.JobID,
		stale.ConfigurationVersion,
		automationdomain.UpdateJobInput{Name: &staleName},
	); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale update error = %v, want version conflict", err)
	}
	if _, err = secondService.DeleteTaskAtVersion(
		context.Background(),
		created.JobID,
		stale.ConfigurationVersion,
	); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale delete error = %v, want version conflict", err)
	}
	if _, err = secondService.DeleteTaskAtVersion(
		context.Background(),
		created.JobID,
		updated.ConfigurationVersion,
	); err != nil {
		t.Fatalf("current-version delete: %v", err)
	}
	persisted, err := firstService.GetTask(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("GetTask after delete: %v", err)
	}
	if persisted != nil {
		t.Fatalf("task still exists after versioned delete: %+v", persisted)
	}
}

func TestVersionedManualRunRechecksConfigurationBeforeRuntimeClaim(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	created, err := service.CreateTask(context.Background(), automationConfigurationTaskInput("run-version"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	name := "new configuration"
	if _, err = service.UpdateTaskAtVersion(
		context.Background(),
		created.JobID,
		created.ConfigurationVersion,
		automationdomain.UpdateJobInput{Name: &name},
	); err != nil {
		t.Fatalf("UpdateTaskAtVersion: %v", err)
	}

	staleVersion := created.ConfigurationVersion
	result, err := service.startJobExecutionControlledAtVersion(
		context.Background(),
		*created,
		automationdomain.TriggerKindManual,
		time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC),
		&staleVersion,
	)
	if result != nil || !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale manual run = (%+v, %v), want nil/version conflict", result, err)
	}
	var runningRunID sql.NullString
	if err = db.QueryRow(
		`SELECT running_run_id FROM automation_scheduled_tasks WHERE job_id = ?`,
		created.JobID,
	).Scan(&runningRunID); err != nil {
		t.Fatalf("read runtime after stale run: %v", err)
	}
	if runningRunID.Valid && runningRunID.String != "" {
		t.Fatalf("stale manual run changed running_run_id: %+v", runningRunID)
	}
}

func TestScheduledTaskDeleteRollsBackRunFinalizationWhenDeleteFails(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationConfigurationTaskInput("delete-atomic"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	activeRunID := "run-delete-active"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       activeRunID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual,
		Status:      automationdomain.RunStatusPending,
	}); err != nil {
		t.Fatalf("InsertRunPending active: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), activeRunID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkRunRunning: %v", err)
	}
	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks SET running_run_id = ?, running_started_at = CURRENT_TIMESTAMP WHERE job_id = ?`,
		activeRunID,
		task.JobID,
	); err != nil {
		t.Fatalf("set active task runtime: %v", err)
	}
	deliveryRunID := "run-delete-delivery"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:          deliveryRunID,
		JobID:          task.JobID,
		OwnerUserID:    task.OwnerUserID,
		TriggerKind:    automationdomain.TriggerKindScheduled,
		Status:         automationdomain.RunStatusSucceeded,
		DeliveryMode:   automationdomain.DeliveryModeExplicit,
		DeliveryStatus: automationdomain.DeliveryStatusFailed,
	}); err != nil {
		t.Fatalf("InsertRunPending delivery: %v", err)
	}
	if _, err = db.Exec(`
CREATE TRIGGER fail_atomic_task_delete
BEFORE DELETE ON automation_scheduled_tasks
WHEN OLD.job_id = '` + task.JobID + `'
BEGIN
    SELECT RAISE(FAIL, 'injected task delete failure');
END;
`); err != nil {
		t.Fatalf("create delete failure trigger: %v", err)
	}

	if _, err = service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); err == nil {
		t.Fatal("DeleteTaskAtVersion should fail when delete trigger aborts")
	} else if !TaskDeletionPrepared(err) {
		t.Fatalf("delete failure should preserve prepared-stage evidence: %v", err)
	}
	persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted == nil {
		t.Fatalf("durable deletion claim must remain: task=%+v err=%v", persisted, err)
	}
	if persisted.DeletionState != automationdomain.TaskDeletionStateDeleting ||
		persisted.DeletionToken == "" || persisted.Enabled ||
		persisted.ConfigurationVersion != task.ConfigurationVersion+1 {
		t.Fatalf("delete failure lost its exact durable claim: %+v", persisted)
	}
	activeRun, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, activeRunID)
	if err != nil || activeRun == nil || activeRun.Status != automationdomain.RunStatusRunning {
		t.Fatalf("active run cancellation must roll back: run=%+v err=%v", activeRun, err)
	}
	deliveryRun, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, deliveryRunID)
	if err != nil || deliveryRun == nil ||
		deliveryRun.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		deliveryRun.DeliveryDeadLetterAt != nil {
		t.Fatalf("delivery dead-letter must roll back: run=%+v err=%v", deliveryRun, err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_atomic_task_delete`); err != nil {
		t.Fatalf("drop delete failure trigger: %v", err)
	}
	if _, err = service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); err != nil {
		t.Fatalf("repeated DELETE must resume the exact deletion claim: %v", err)
	}
	persisted, err = service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted != nil {
		t.Fatalf("repeated DELETE did not finish durable cleanup: task=%+v err=%v", persisted, err)
	}
}

func TestScheduledTaskUpdateAcceptsUnchangedHistoricalIMDelivery(t *testing.T) {
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
	grant := &mutableAutomationDeliveryGrant{allowed: true}
	service.SetDeliveryGrantResolver(grant)
	input := automationConfigurationTaskInput("historical-im")
	input.Source = automationdomain.Source{Kind: automationdomain.SourceKindUserPage}
	input.Delivery = automationdomain.DeliveryTarget{
		Mode:       automationdomain.DeliveryModeLast,
		SessionKey: "agent:agent-1:weixin-personal:dm:acct:old-account:old-contact",
	}
	created, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	grant.setAllowed(false)
	updatedName := "historical-im-edited"
	updated, err := service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{
			Name:     &updatedName,
			Delivery: &created.Delivery,
			Source:   &created.Source,
		},
	)
	if err != nil {
		t.Fatalf("unchanged historical IM route should remain editable: %v", err)
	}
	if updated.Name != updatedName || updated.Delivery != created.Delivery ||
		updated.Source != created.Source {
		t.Fatalf("historical task update changed route/source: %+v", updated)
	}

	changedDelivery := created.Delivery
	changedDelivery.SessionKey = "agent:agent-1:weixin-personal:dm:acct:new-account:new-contact"
	if _, err = service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{Delivery: &changedDelivery},
	); !errors.Is(err, automationdomain.ErrTaskDeliverySessionUnavailable) {
		t.Fatalf("changed IM route must return the stable unavailable-session error, got %v", err)
	}
}

func TestHumanUpdatePreservesAgentCreatedTaskProvenance(t *testing.T) {
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
	agentCtx := automationexec.WithActorAgentID(context.Background(), "agent-1")
	source := automationdomain.Source{
		Kind:           automationdomain.SourceKindAgent,
		CreatorAgentID: "agent-1",
		ContextType:    "agent",
		ContextID:      "agent-1",
		SessionKey: protocol.BuildAgentSessionKey(
			"agent-1",
			protocol.SessionChannelInternalSegment,
			protocol.RoomTypeDM,
			"operator",
			"",
		),
		SessionLabel: "最初会话",
	}
	input := automationConfigurationTaskInput("agent-created")
	input.Source = source
	input.Delivery = automationdomain.DeliveryTarget{
		Mode:    automationdomain.DeliveryModeExplicit,
		Channel: protocol.SessionChannelInternalSegment,
		To:      source.SessionKey,
	}
	created, err := service.CreateTask(agentCtx, input)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	updatedName := "human-edited"
	updated, err := service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{Name: &updatedName},
	)
	if err != nil {
		t.Fatalf("human control-plane update should not require Agent actor: %v", err)
	}
	if updated.Source != source.Normalized() {
		t.Fatalf("creation provenance changed: got=%+v want=%+v", updated.Source, source.Normalized())
	}
	if updated.DeliveryGrant != source.Normalized() {
		t.Fatalf("unchanged delivery grant changed: got=%+v want=%+v", updated.DeliveryGrant, source.Normalized())
	}

	none := automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}
	pageSource := automationdomain.Source{Kind: automationdomain.SourceKindUserPage}
	updated, err = service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{Delivery: &none, Source: &pageSource},
	)
	if err != nil {
		t.Fatalf("human delivery update: %v", err)
	}
	if updated.Source != source.Normalized() {
		t.Fatalf("delivery edit rewrote creation provenance: %+v", updated.Source)
	}
	if updated.DeliveryGrant.Kind != automationdomain.SourceKindUserPage ||
		updated.DeliveryGrant.CreatorAgentID != "" {
		t.Fatalf("delivery grant did not transfer to page control plane: %+v", updated.DeliveryGrant)
	}
}

func TestScheduledTaskConcurrentCASHasSingleWinner(t *testing.T) {
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
	created, err := service.CreateTask(context.Background(), automationConfigurationTaskInput("race"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, value := range []string{"left", "right"} {
		value := value
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, updateErr := service.UpdateTaskAtVersion(
				context.Background(),
				created.JobID,
				created.ConfigurationVersion,
				automationdomain.UpdateJobInput{Name: &value},
			)
			results <- updateErr
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, automationdomain.ErrConfigurationVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
}

func TestHeartbeatCASAndRuntimePersistenceKeepConfigurationVersion(t *testing.T) {
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
	initial, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus: %v", err)
	}
	enabled := automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 60,
		TargetMode:   automationdomain.HeartbeatTargetNone,
		AckMaxChars:  120,
	}
	updated, err := service.UpdateHeartbeatAtVersion(
		context.Background(),
		"agent-1",
		initial.ConfigurationVersion,
		enabled,
	)
	if err != nil {
		t.Fatalf("UpdateHeartbeatAtVersion: %v", err)
	}
	if updated.ConfigurationVersion != initial.ConfigurationVersion+1 {
		t.Fatalf("heartbeat version = %d, want %d", updated.ConfigurationVersion, initial.ConfigurationVersion+1)
	}
	if _, err = service.UpdateHeartbeatAtVersion(
		context.Background(),
		"agent-1",
		initial.ConfigurationVersion,
		enabled,
	); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale heartbeat update error = %v, want version conflict", err)
	}

	if err = service.repository.PersistHeartbeatRuntimeState(
		context.Background(),
		"runtime-state",
		automationdomain.HeartbeatConfig{
			AgentID:              updated.AgentID,
			Enabled:              updated.Enabled,
			EverySeconds:         updated.EverySeconds,
			TargetMode:           updated.TargetMode,
			AckMaxChars:          updated.AckMaxChars,
			ConfigurationVersion: updated.ConfigurationVersion,
		},
		nil,
		nil,
	); err != nil {
		t.Fatalf("PersistHeartbeatRuntimeState: %v", err)
	}
	afterRuntime, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus after runtime write: %v", err)
	}
	if afterRuntime.ConfigurationVersion != updated.ConfigurationVersion {
		t.Fatalf(
			"runtime persistence changed version: got %d want %d",
			afterRuntime.ConfigurationVersion,
			updated.ConfigurationVersion,
		)
	}

	staleRuntimeAt := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	freshRuntimeAt := staleRuntimeAt.Add(time.Minute)
	configValue := automationdomain.HeartbeatConfig{
		AgentID:              updated.AgentID,
		Enabled:              updated.Enabled,
		EverySeconds:         updated.EverySeconds,
		TargetMode:           updated.TargetMode,
		AckMaxChars:          updated.AckMaxChars,
		ConfigurationVersion: updated.ConfigurationVersion,
	}
	if err = service.repository.PersistHeartbeatRuntimeState(
		context.Background(),
		"runtime-state",
		configValue,
		&freshRuntimeAt,
		&freshRuntimeAt,
	); err != nil {
		t.Fatalf("persist fresh heartbeat runtime: %v", err)
	}
	configValue.EverySeconds = 90
	if err = service.repository.UpsertHeartbeatStateAtVersion(
		context.Background(),
		"config-state",
		configValue,
		&staleRuntimeAt,
		&staleRuntimeAt,
		updated.ConfigurationVersion,
	); err != nil {
		t.Fatalf("versioned heartbeat config update: %v", err)
	}
	persisted, lastHeartbeatAt, lastAckAt, err := service.repository.GetHeartbeatState(
		context.Background(),
		updated.AgentID,
	)
	if err != nil {
		t.Fatalf("GetHeartbeatState: %v", err)
	}
	if persisted == nil ||
		persisted.ConfigurationVersion != updated.ConfigurationVersion+1 ||
		persisted.EverySeconds != 90 {
		t.Fatalf("persisted heartbeat config = %+v", persisted)
	}
	if lastHeartbeatAt == nil || !lastHeartbeatAt.Equal(freshRuntimeAt) ||
		lastAckAt == nil || !lastAckAt.Equal(freshRuntimeAt) {
		t.Fatalf(
			"config CAS overwrote fresher runtime timestamps: heartbeat=%v ack=%v",
			lastHeartbeatAt,
			lastAckAt,
		)
	}
}

func TestAutomationCommandCreateRetryAndHeartbeatControl(t *testing.T) {
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

	updateResult, updateError := callAutomationCommand(
		t,
		fixture.Service,
		fixture.ServerContext,
		"update_heartbeat",
		map[string]any{
			"enabled":       true,
			"every_seconds": 120,
			"target_mode":   automationdomain.HeartbeatTargetNone,
			"ack_max_chars": 200,
		},
	)
	if updateError {
		t.Fatalf("update_heartbeat: %s", automationCommandText(t, updateResult))
	}
	heartbeat := decodeAutomationCommandJSON[automationdomain.HeartbeatStatus](t, updateResult)
	if heartbeat.ConfigurationVersion != 1 || !heartbeat.Enabled || heartbeat.EverySeconds != 120 {
		t.Fatalf("heartbeat update = %+v", heartbeat)
	}
	wakeResult, wakeError := callAutomationCommand(
		t,
		fixture.Service,
		command.Actor{
			AgentID:           fixture.ServerContext.AgentID,
			OwnerUserID:       fixture.ServerContext.OwnerUserID,
			SourceContextType: "agent",
		},
		"wake_heartbeat",
		map[string]any{"mode": automationdomain.WakeModeNextHeartbeat},
	)
	if wakeError {
		t.Fatalf("wake_heartbeat: %s", automationCommandText(t, wakeResult))
	}
	wake := decodeAutomationCommandJSON[automationdomain.HeartbeatWakeResult](t, wakeResult)
	if wake.AgentID != fixture.ServerContext.AgentID ||
		wake.Mode != automationdomain.WakeModeNextHeartbeat ||
		wake.Scheduled {
		t.Fatalf("wake result = %+v", wake)
	}
	afterWake, err := fixture.Service.GetHeartbeatStatus(
		automationCommandTestOwnerContext(fixture.ServerContext.OwnerUserID),
		fixture.ServerContext.AgentID,
	)
	if err != nil {
		t.Fatalf("GetHeartbeatStatus after wake: %v", err)
	}
	if afterWake.ConfigurationVersion != heartbeat.ConfigurationVersion {
		t.Fatalf(
			"wake changed configuration version: before=%d after=%d",
			heartbeat.ConfigurationVersion,
			afterWake.ConfigurationVersion,
		)
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
			Kind:     automationdomain.SessionTargetIsolated,
			WakeMode: automationdomain.WakeModeNow,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source: automationdomain.Source{
			Kind: automationdomain.SourceKindAgent,
		},
		Enabled: true,
	}
}
