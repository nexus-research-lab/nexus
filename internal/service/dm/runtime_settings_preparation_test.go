package dm

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestConnectorRuntimePreparationCoalescesRapidChanges(t *testing.T) {
	service := NewService(config.Config{}, nil, nil, nil)
	service.connectorPreparationDelay = 20 * time.Millisecond
	var mu sync.Mutex
	runs := make([]int64, 0, 1)
	done := make(chan struct{}, 1)
	service.connectorPreparationRun = func(_ context.Context, session protocol.Session) error {
		mu.Lock()
		runs = append(runs, session.ConfigurationVersion)
		mu.Unlock()
		done <- struct{}{}
		return nil
	}
	ctx := contextWithExactOwner(context.Background(), "owner-a")
	for version := int64(1); version <= 3; version++ {
		service.ScheduleRuntimeSettingsPreparation(ctx, connectorPreparationTestSession(version))
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("latest Connector runtime preparation did not run")
	}
	time.Sleep(40 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(runs) != 1 || runs[0] != 3 {
		t.Fatalf("preparation versions = %v, want only [3]", runs)
	}
}

func TestConnectorRuntimePreparationCancelsRunningSupersededChange(t *testing.T) {
	service := NewService(config.Config{}, nil, nil, nil)
	service.connectorPreparationDelay = time.Millisecond
	firstStarted := make(chan struct{})
	firstCanceled := make(chan struct{})
	secondDone := make(chan struct{})
	service.connectorPreparationRun = func(ctx context.Context, session protocol.Session) error {
		if session.ConfigurationVersion == 1 {
			close(firstStarted)
			<-ctx.Done()
			close(firstCanceled)
			return ctx.Err()
		}
		close(secondDone)
		return nil
	}
	ctx := contextWithExactOwner(context.Background(), "owner-a")
	service.ScheduleRuntimeSettingsPreparation(ctx, connectorPreparationTestSession(1))
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first preparation did not start")
	}
	service.ScheduleRuntimeSettingsPreparation(ctx, connectorPreparationTestSession(2))
	select {
	case <-firstCanceled:
	case <-time.After(time.Second):
		t.Fatal("superseded running preparation was not canceled")
	}
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("latest preparation did not run")
	}
}

func TestConnectorRuntimePreparationIsCanceledByRealInput(t *testing.T) {
	service := NewService(config.Config{}, nil, nil, nil)
	service.connectorPreparationDelay = 40 * time.Millisecond
	runs := make(chan struct{}, 1)
	service.connectorPreparationRun = func(context.Context, protocol.Session) error {
		runs <- struct{}{}
		return nil
	}
	ctx := contextWithExactOwner(context.Background(), "owner-a")
	session := connectorPreparationTestSession(1)
	service.ScheduleRuntimeSettingsPreparation(ctx, session)
	service.cancelConnectorRuntimePreparation("owner-a", session.SessionKey)

	select {
	case <-runs:
		t.Fatal("real input should cancel pending preparation and use synchronous startup")
	case <-time.After(100 * time.Millisecond):
	}
}

func connectorPreparationTestSession(version int64) protocol.Session {
	return protocol.Session{
		SessionKey:           "agent:nexus:ws:dm:connector-preparation",
		AgentID:              "nexus",
		ChatType:             protocol.RoomTypeDM,
		ConfigurationVersion: version,
		Options:              map[string]any{},
	}
}
