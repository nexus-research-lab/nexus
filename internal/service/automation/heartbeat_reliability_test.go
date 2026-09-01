// INPUT: stale in-memory heartbeat version、durable wake outbox、command ledger gap 与并发消费者。
// OUTPUT: DB CAS、restart recovery、exact replay 和单次 dispatch 的服务级保证。
// POS: Heartbeat wake reliability 的跨 service/storage 回归。
package automation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/dm"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func TestWakeHeartbeatAtVersionUsesDurableConfigurationFence(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil,
		&fakeWorkspaceReader{}, nil,
	)
	status, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled: true, EverySeconds: 60,
		TargetMode: automationdomain.HeartbeatTargetNone, AckMaxChars: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	staleVersion := status.ConfigurationVersion
	persisted, lastHeartbeatAt, lastAckAt, err := service.repository.GetHeartbeatState(context.Background(), "agent-1")
	if err != nil || persisted == nil {
		t.Fatalf("read heartbeat: config=%+v err=%v", persisted, err)
	}
	persisted.AckMaxChars++
	if err = service.repository.UpsertHeartbeatStateAtVersion(
		context.Background(), "hb-concurrent", *persisted, lastHeartbeatAt, lastAckAt, staleVersion,
	); err != nil {
		t.Fatal(err)
	}
	_, err = service.WakeHeartbeatAtVersion(
		context.Background(), "agent-1", staleVersion,
		automationdomain.HeartbeatWakeInput{Mode: automationdomain.WakeModeNextHeartbeat},
	)
	if !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale in-memory wake error = %v", err)
	}
	events, listErr := service.repository.ListNewSystemEventsByAgent(context.Background(), "agent-1")
	if listErr != nil || len(events) != 0 {
		t.Fatalf("stale wake wrote outbox: events=%+v err=%v", events, listErr)
	}
}

func TestHeartbeatWakeWithoutTextIsDurableAndRestartRecoverable(t *testing.T) {
	db := newAutomationTestDB(t)
	seed := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil,
		&fakeWorkspaceReader{}, nil,
	)
	accepted, err := seed.repository.AcceptHeartbeatWake(context.Background(), automationstore.HeartbeatWakeAcceptanceInput{
		EventID: "wake-restart", AgentID: "agent-1", OwnerUserID: "user-1",
		RequestID: "wake-restart-request", IntentDigest: "wake-restart-intent",
		Mode: automationdomain.WakeModeNow, AcceptedAt: time.Now().UTC(),
	})
	if err != nil || accepted.Event.Payload == "" {
		t.Fatalf("seed durable wake: result=%+v err=%v", accepted, err)
	}

	permission := permissionctx.NewContext()
	runner := &fakeDMRunner{permission: permission}
	restarted := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, runner, nil, permission,
		&fakeWorkspaceReader{}, nil,
	)
	if err = restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer restarted.Stop()
	waitFor(t, 2*time.Second, func() bool { return len(runner.Requests()) == 1 })
	if content := runner.Requests()[0].Content; content == "" {
		t.Fatal("no-text durable wake produced empty instruction")
	}
	waitFor(t, 2*time.Second, func() bool {
		event, getErr := restarted.repository.GetHeartbeatWakeByRequest(
			context.Background(), "user-1", "wake-restart-request",
		)
		return getErr == nil && event.Status == "processed"
	})
}

func TestConcurrentHeartbeatWakeConsumersDispatchOnce(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	runner := &fakeDMRunner{permission: permission}
	first := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, runner, nil, permission,
		&fakeWorkspaceReader{}, nil,
	)
	second := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, runner, nil, permission,
		&fakeWorkspaceReader{}, nil,
	)
	if _, err := first.repository.AcceptHeartbeatWake(context.Background(), automationstore.HeartbeatWakeAcceptanceInput{
		EventID: "wake-concurrent", AgentID: "agent-1", Mode: automationdomain.WakeModeNow,
		AcceptedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); first.dispatchHeartbeat("agent-1", "durable-wake-recovery") }()
	go func() { defer wg.Done(); second.dispatchHeartbeat("agent-1", "durable-wake-recovery") }()
	wg.Wait()
	waitFor(t, 2*time.Second, func() bool { return len(runner.Requests()) > 0 })
	time.Sleep(100 * time.Millisecond)
	if requests := runner.Requests(); len(requests) != 1 {
		t.Fatalf("concurrent durable wake dispatched %d rounds: %+v", len(requests), requests)
	}
}

func TestRuntimeCommandReconcilesAcceptedWakeAfterLedgerGap(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	request := automationdomain.AutomationCommandRequest{
		Operation: automationdomain.AutomationCommandOperationWake,
		RequestID: "runtime-wake-ledger-gap",
		Input:     automationdomain.AutomationCommandInput{Mode: automationdomain.WakeModeNextHeartbeat},
	}
	intentDigest, err := runtimeAutomationIntentDigest(actor, request.Operation, request.Input)
	if err != nil {
		t.Fatal(err)
	}
	if _, isNew, claimErr := fixture.Service.repository.ClaimRuntimeCommand(
		context.Background(), automationstore.RuntimeCommandRecord{
			OwnerUserID: actor.OwnerUserID, RequestID: request.RequestID,
			ActorAgentID: actor.AgentID, Operation: request.Operation,
			IntentDigest: intentDigest, ApprovalRequestID: "approval-ledger-gap",
		},
	); claimErr != nil || !isNew {
		t.Fatalf("claim command: new=%v err=%v", isNew, claimErr)
	}
	if _, err = fixture.Service.repository.AcceptHeartbeatWake(
		context.Background(), automationstore.HeartbeatWakeAcceptanceInput{
			EventID: "wake-ledger-gap", AgentID: actor.AgentID, OwnerUserID: actor.OwnerUserID,
			RequestID: request.RequestID, IntentDigest: intentDigest,
			Mode: automationdomain.WakeModeNextHeartbeat, AcceptedAt: time.Now().UTC(),
		},
	); err != nil {
		t.Fatal(err)
	}
	replayed, found, err := fixture.Service.ReplayRuntimeCommand(context.Background(), actor, request)
	if err != nil || !found || replayed.Outcome != "replayed" {
		t.Fatalf("reconcile accepted wake: result=%+v found=%v err=%v", replayed, found, err)
	}
	wake, ok := replayed.Data.(*automationdomain.HeartbeatWakeResult)
	if !ok || wake.AgentID != actor.AgentID || wake.Mode != automationdomain.WakeModeNextHeartbeat || wake.Scheduled {
		t.Fatalf("replayed wake receipt = %+v", replayed.Data)
	}
	record, err := fixture.Service.repository.GetRuntimeCommand(context.Background(), actor.OwnerUserID, request.RequestID)
	if err != nil || record.Status != "applied" {
		t.Fatalf("reconciled command ledger = %+v err=%v", record, err)
	}
}

type heartbeatReentrantDMRunner struct {
	service *Service
	done    chan error
}

func (r *heartbeatReentrantDMRunner) HandleChat(_ context.Context, _ dm.Request) error {
	_, err := r.service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled: true, EverySeconds: 120,
		TargetMode: automationdomain.HeartbeatTargetNone, AckMaxChars: 300,
	})
	r.done <- err
	return errors.New("stop test dispatch after reentrant configuration update")
}

func TestWakeHeartbeatDoesNotHoldControlLockAcrossDispatch(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	runner := &heartbeatReentrantDMRunner{done: make(chan error, 1)}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, runner, nil, permission,
		&fakeWorkspaceReader{}, nil,
	)
	runner.service = service
	if _, err := service.WakeHeartbeat(
		context.Background(), "agent-1", automationdomain.HeartbeatWakeInput{Mode: automationdomain.WakeModeNow},
	); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-runner.done:
		if err != nil {
			t.Fatalf("reentrant heartbeat update: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat dispatch retained heartbeatControlMu")
	}
}

var _ dmRunner = (*heartbeatReentrantDMRunner)(nil)
