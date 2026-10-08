package goal

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func TestServiceCreatesCompletedExternalGoalWithFinalizedZeroUsage(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	complete := goalappserver.ThreadGoalStatusComplete
	objective := "Already complete"

	completed, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  "agent:nexus:ws:dm:external-complete-idle",
		Objective: &objective,
		Status:    &complete,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !completed.UsageFinalized || completed.Usage.ActualTokens() != 0 {
		t.Fatalf("completed = %#v, want authoritative finalized zero usage", completed)
	}
}

func testConfig() config.Config {
	return config.Config{GoalEnabled: true}
}
