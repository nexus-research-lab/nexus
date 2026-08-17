package automation

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
)

type latencyDMRunner struct {
	base  *fakeDMRunner
	calls chan time.Time
}

type latencyDeliveryRouter struct {
	base  *fakeDeliveryRouter
	calls chan time.Time
}

func (r *latencyDeliveryRouter) DeliverMessage(
	ctx context.Context,
	agentID string,
	message string,
	target channels.DeliveryTarget,
) (channels.DeliveryResult, error) {
	r.calls <- time.Now()
	return r.base.DeliverMessage(ctx, agentID, message, target)
}

func (r *latencyDMRunner) HandleChat(ctx context.Context, request dmsvc.Request) error {
	r.calls <- time.Now()
	return r.base.HandleChat(ctx, request)
}

func TestMeasureAutomationSchedulerLatency(t *testing.T) {
	if os.Getenv("NEXUS_MEASURE_BACKGROUND_LATENCY") != "1" {
		t.Skip("set NEXUS_MEASURE_BACKGROUND_LATENCY=1 to run wall-clock latency measurements")
	}
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	runner := &latencyDMRunner{
		base:  &fakeDMRunner{permission: permission, resultText: "latency result"},
		calls: make(chan time.Time, 1),
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
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()

	const sampleCount = 30
	samples := make([]time.Duration, 0, sampleCount)
	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-%02d", index)
		task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
			Name:        suffix,
			AgentID:     "agent-1",
			Instruction: "measure scheduler deadline",
			Schedule: automationdomain.Schedule{
				Kind:            automationdomain.ScheduleKindEvery,
				IntervalSeconds: intRef(3600),
				Timezone:        "UTC",
			},
			SessionTarget: automationdomain.SessionTarget{
				Kind: automationdomain.SessionTargetBound,
				BoundSessionKey: protocol.BuildAgentSessionKey(
					"agent-1", "ws", protocol.RoomTypeDM, suffix, "",
				),
			},
			Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
			Enabled:  true,
		})
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(10 * time.Millisecond)
		task.NextRunAt = &deadline
		service.replaceJobRuntimeState(*task)
		actual := waitAutomationMeasurement(t, runner.calls)
		lateness := actual.Sub(deadline)
		if lateness < 0 {
			lateness = 0
		}
		samples = append(samples, lateness)
		waitFor(t, 2*time.Second, func() bool {
			current, getErr := service.GetTask(context.Background(), task.JobID)
			return getErr == nil && !current.Running
		})
	}
	t.Log(formatAutomationLatency("automation_task_deadline_to_runtime_dispatch", samples))
}

func TestMeasureAutomationDeliveryRetryLatency(t *testing.T) {
	if os.Getenv("NEXUS_MEASURE_BACKGROUND_LATENCY") != "1" {
		t.Skip("set NEXUS_MEASURE_BACKGROUND_LATENCY=1 to run wall-clock latency measurements")
	}
	db := newAutomationTestDB(t)
	router := &latencyDeliveryRouter{
		base:  &fakeDeliveryRouter{},
		calls: make(chan time.Time, 1),
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		router,
	)
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "latency-delivery-retry",
		AgentID:     "agent-1",
		Instruction: "measure delivery retry",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "latency-delivery",
		},
		Delivery: fakeStructuredDelivery("agent-1", "latency-delivery"),
		Enabled:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	const sampleCount = 30
	samples := make([]time.Duration, 0, sampleCount)
	for index := 0; index < sampleCount; index++ {
		runID := fmt.Sprintf("run-latency-delivery-%02d", index)
		deadline := time.Now().UTC().Add(10 * time.Millisecond)
		if _, err = db.ExecContext(context.Background(), `
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
			automationdomain.TriggerKindScheduled,
			automationdomain.DeliveryModeExplicit,
			"explicit:internal:latency",
			automationdomain.DeliveryStatusFailed,
			"temporary delivery failure",
			1,
			deadline,
			deadline.Add(-time.Minute),
			deadline.Add(-time.Second),
			"latency result",
			1,
		); err != nil {
			t.Fatal(err)
		}
		service.invalidateDeliveryRetryDeadline()
		actual := waitAutomationMeasurement(t, router.calls)
		lateness := actual.Sub(deadline)
		if lateness < 0 {
			lateness = 0
		}
		samples = append(samples, lateness)
		waitFor(t, 2*time.Second, func() bool {
			runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
			if listErr != nil {
				return false
			}
			for _, run := range runs {
				if run.RunID == runID {
					return run.DeliveryStatus == automationdomain.DeliveryStatusSucceeded
				}
			}
			return false
		})
	}
	t.Log(formatAutomationLatency("automation_delivery_retry_deadline_to_adapter", samples))
}

func TestMeasureAutomationLeaseTakeoverLatency(t *testing.T) {
	if os.Getenv("NEXUS_MEASURE_BACKGROUND_LATENCY") != "1" {
		t.Skip("set NEXUS_MEASURE_BACKGROUND_LATENCY=1 to run wall-clock latency measurements")
	}
	db := newAutomationTestDB(t)
	newScheduler := func() *Service {
		return NewService(
			config.Config{DatabaseDriver: "sqlite"},
			db,
			nil,
			nil,
			nil,
			nil,
			&fakeWorkspaceReader{},
			nil,
		)
	}
	leader := newScheduler()
	follower := newScheduler()
	if err := leader.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := follower.Start(context.Background()); err != nil {
		leader.Stop()
		t.Fatal(err)
	}
	defer follower.Stop()
	// Let the follower cache the current lease expiry, then stop the leader
	// without sending any cross-process wake to the follower.
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	leader.Stop()
	deadline := start.Add(35 * time.Second)
	for time.Now().Before(deadline) {
		follower.mu.Lock()
		held := follower.schedulerLeaseHeld
		follower.mu.Unlock()
		if held {
			t.Log(formatAutomationLatency(
				"automation_follower_lease_takeover",
				[]time.Duration{time.Since(start)},
			))
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("Automation follower did not acquire the scheduler lease within 35 seconds")
}

func waitAutomationMeasurement(t *testing.T, values <-chan time.Time) time.Time {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for automation latency measurement")
		return time.Time{}
	}
}

func formatAutomationLatency(name string, samples []time.Duration) string {
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var total time.Duration
	for _, sample := range ordered {
		total += sample
	}
	return fmt.Sprintf(
		"LATENCY %s n=%d min=%s p50=%s p95=%s max=%s mean=%s",
		name, len(ordered), ordered[0],
		ordered[int(float64(len(ordered)-1)*0.50)],
		ordered[int(float64(len(ordered)-1)*0.95)],
		ordered[len(ordered)-1], total/time.Duration(len(ordered)),
	)
}
