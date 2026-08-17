package goal

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
)

type goalLatencyDispatch struct {
	service *Service
	calls   chan goalLatencyDispatchCall
}

type goalLatencyDispatchCall struct {
	at   time.Time
	plan protocol.GoalContinuation
}

func (d *goalLatencyDispatch) ShouldDeferGoalContinuation(context.Context, string) bool {
	return false
}

func (d *goalLatencyDispatch) DispatchGoalContinuation(
	ctx context.Context,
	plan protocol.GoalContinuation,
) error {
	if _, err := d.service.ClaimContinuationPlan(ctx, plan); err != nil {
		return err
	}
	d.calls <- goalLatencyDispatchCall{at: time.Now(), plan: plan}
	return nil
}

func TestMeasureGoalAutoResumeLatency(t *testing.T) {
	if os.Getenv("NEXUS_MEASURE_BACKGROUND_LATENCY") != "1" {
		t.Skip("set NEXUS_MEASURE_BACKGROUND_LATENCY=1 to run wall-clock latency measurements")
	}
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	cfg.GoalAutoContinueEnabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := goalstore.NewRepository(cfg, db)
	service := NewService(cfg, repository)
	dispatcher := &goalLatencyDispatch{
		service: service,
		calls:   make(chan goalLatencyDispatchCall, 1),
	}
	stop, err := service.StartAutoResume(context.Background(), dispatcher)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	const sampleCount = 30
	samples := make([]time.Duration, 0, sampleCount)
	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-%02d", index)
		now := time.Now().UTC()
		_, err = repository.CreateGoal(context.Background(), protocol.Goal{
			ID:         "goal-" + suffix,
			SessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", protocol.RoomTypeDM, suffix, ""),
			Objective:  "measure goal recovery " + suffix,
			Status:     protocol.GoalStatusActive,
			Version:    1,
			CreatedBy:  "latency-test",
			CreatedAt:  now,
			UpdatedAt:  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		service.WakeAutoResume()
		call := waitGoalMeasurement(t, dispatcher.calls)
		samples = append(samples, call.at.Sub(start))
		if deleted, deleteErr := repository.DeleteGoal(context.Background(), call.plan.Goal.ID); deleteErr != nil || !deleted {
			t.Fatalf("DeleteGoal() = %v, %v", deleted, deleteErr)
		}
	}
	t.Log(formatGoalLatency("goal_durable_wake_to_claimed_dispatch", samples))
}

func waitGoalMeasurement(t *testing.T, values <-chan goalLatencyDispatchCall) goalLatencyDispatchCall {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Goal latency measurement")
		return goalLatencyDispatchCall{}
	}
}

func formatGoalLatency(name string, samples []time.Duration) string {
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
