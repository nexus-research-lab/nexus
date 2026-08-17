package duework

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

const latencyMeasurementEnv = "NEXUS_MEASURE_BACKGROUND_LATENCY"

func TestMeasureDueworkLatency(t *testing.T) {
	if os.Getenv(latencyMeasurementEnv) != "1" {
		t.Skip("set " + latencyMeasurementEnv + "=1 to run wall-clock latency measurements")
	}
	measureWakeLatency(t, 100)
	measureDeadlineLateness(t, 100, 10*time.Millisecond)
}

func measureWakeLatency(t *testing.T, count int) {
	t.Helper()
	loop := New(Options{AuditInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reconciled := make(chan time.Time, 1)
	started := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		first := true
		done <- loop.Run(ctx, func(context.Context, time.Time) (Result, error) {
			if first {
				first = false
				started <- struct{}{}
				return Result{}, nil
			}
			reconciled <- time.Now()
			return Result{}, nil
		})
	}()
	<-started
	samples := make([]time.Duration, 0, count)
	for index := 0; index < count; index++ {
		start := time.Now()
		loop.Notify()
		fired := waitMeasurementTime(t, reconciled)
		samples = append(samples, fired.Sub(start))
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	t.Log(formatLatencyMeasurement("duework_wake_to_reconcile", samples))
}

func measureDeadlineLateness(t *testing.T, count int, delay time.Duration) {
	t.Helper()
	loop := New(Options{AuditInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deadlines := make(chan time.Time, 1)
	fired := make(chan time.Time, 1)
	done := make(chan error, 1)
	go func() {
		done <- loop.Run(ctx, func(context.Context, time.Time) (Result, error) {
			select {
			case deadline := <-deadlines:
				return Result{NextDueAt: &deadline}, nil
			default:
				fired <- time.Now()
				return Result{}, nil
			}
		})
	}()
	// Consume the startup reconcile.
	_ = waitMeasurementTime(t, fired)
	samples := make([]time.Duration, 0, count)
	for index := 0; index < count; index++ {
		deadline := time.Now().Add(delay)
		deadlines <- deadline
		loop.Notify()
		actual := waitMeasurementTime(t, fired)
		lateness := actual.Sub(deadline)
		if lateness < 0 {
			lateness = 0
		}
		samples = append(samples, lateness)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	t.Log(formatLatencyMeasurement(
		fmt.Sprintf("duework_deadline_lateness_%s", delay),
		samples,
	))
}

func waitMeasurementTime(t *testing.T, values <-chan time.Time) time.Time {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for latency measurement")
		return time.Time{}
	}
}

func formatLatencyMeasurement(name string, samples []time.Duration) string {
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return fmt.Sprintf(
		"LATENCY %s n=%d min=%s p50=%s p95=%s max=%s mean=%s",
		name,
		len(ordered),
		ordered[0],
		measurementPercentile(ordered, 0.50),
		measurementPercentile(ordered, 0.95),
		ordered[len(ordered)-1],
		measurementMean(ordered),
	)
}

func measurementPercentile(ordered []time.Duration, percentile float64) time.Duration {
	index := int(float64(len(ordered)-1) * percentile)
	return ordered[index]
}

func measurementMean(values []time.Duration) time.Duration {
	var total time.Duration
	for _, value := range values {
		total += value
	}
	return total / time.Duration(len(values))
}
