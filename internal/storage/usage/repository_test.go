package usage

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	_ "modernc.org/sqlite"
)

func TestRepositoryUpdatesAndAggregatesCacheAttribution(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: filepath.Join(root, "usage.db")}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	migrations := filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
	handlertest.MigrateSQLiteFromDir(t, cfg.DatabaseURL, migrations)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(cfg, db)
	record := Record{
		OwnerUserID: "owner", UsageKey: "usage", Source: "room_runtime",
		SessionKey: "session", MessageID: "message", RoundID: "round",
		InputTokens: 20, CacheReadInputTokens: 15, TotalTokens: 35,
		OccurredAt: time.Unix(100, 0).UTC(),
		GoalScope:  "bound", ExecutionScope: "bound", ResponsibilityLane: "review",
		RuntimeKind:             "nxs",
		HostToolSurfaceComplete: true,
		ToolSurfaceFingerprint:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if err := repository.Upsert(context.Background(), record); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := repository.UpdateCacheAttribution(context.Background(), record); err != nil {
		t.Fatalf("UpdateCacheAttribution() error = %v", err)
	}
	segments, err := repository.CacheSegments(context.Background(), "owner")
	if err != nil {
		t.Fatalf("CacheSegments() error = %v", err)
	}
	if len(segments) != 1 || segments[0].ResponsibilityLane != "review" ||
		segments[0].CacheReadInputTokens != 15 || !segments[0].HostToolSurfaceComplete {
		t.Fatalf("segments = %+v", segments)
	}

	unknownRetry := record
	unknownRetry.GoalScope = "unknown"
	unknownRetry.ExecutionScope = "unknown"
	unknownRetry.ResponsibilityLane = "unknown"
	unknownRetry.RuntimeKind = "unknown"
	unknownRetry.HostToolSurfaceComplete = false
	unknownRetry.ToolSurfaceFingerprint = ""
	if err := repository.Upsert(context.Background(), unknownRetry); err != nil {
		t.Fatalf("retry Upsert() error = %v", err)
	}
	if err := repository.UpdateCacheAttribution(context.Background(), unknownRetry); err != nil {
		t.Fatalf("retry UpdateCacheAttribution() error = %v", err)
	}
	segments, err = repository.CacheSegments(context.Background(), "owner")
	if err != nil {
		t.Fatalf("CacheSegments() after retry error = %v", err)
	}
	if len(segments) != 1 || segments[0].ResponsibilityLane != "review" ||
		segments[0].ToolSurfaceFingerprint != record.ToolSurfaceFingerprint ||
		!segments[0].HostToolSurfaceComplete {
		t.Fatalf("unknown retry degraded attribution: %+v", segments)
	}

	conflictingRetry := record
	conflictingRetry.GoalScope = "none"
	conflictingRetry.ExecutionScope = "none"
	conflictingRetry.ResponsibilityLane = "unbound"
	conflictingRetry.RuntimeKind = "claude"
	conflictingRetry.ToolSurfaceFingerprint = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := repository.UpdateCacheAttribution(context.Background(), conflictingRetry); err != nil {
		t.Fatalf("conflicting retry UpdateCacheAttribution() error = %v", err)
	}
	segments, err = repository.CacheSegments(context.Background(), "owner")
	if err != nil {
		t.Fatalf("CacheSegments() after conflicting retry error = %v", err)
	}
	if len(segments) != 1 || segments[0].GoalScope != "bound" ||
		segments[0].ResponsibilityLane != "review" ||
		segments[0].ToolSurfaceFingerprint != record.ToolSurfaceFingerprint {
		t.Fatalf("conflicting retry replaced first known attribution: %+v", segments)
	}
}
