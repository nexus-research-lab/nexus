package goal

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceRunAndObserveAutoResumeLogsAggregatedFailure(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	repo.goals["goal-malformed"] = protocol.Goal{
		ID:         "goal-malformed",
		SessionKey: "malformed-session-key",
		Objective:  "cannot dispatch",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		UpdatedAt:  service.nowFn(),
	}
	var output bytes.Buffer
	service.SetLogger(slog.New(slog.NewTextHandler(&output, nil)))

	service.runAndObserveAutoResume(context.Background(), &fakeContinuationDispatcher{}, "periodic")

	logLine := output.String()
	if !strings.Contains(logLine, "Goal durable resume failed") ||
		!strings.Contains(logLine, "trigger=periodic") ||
		!strings.Contains(logLine, "goal-malformed") {
		t.Fatalf("resume log = %q", logLine)
	}
}
