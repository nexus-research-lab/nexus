package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"

	_ "modernc.org/sqlite"
)

func TestServiceRecordsJSONNumberUsage(t *testing.T) {
	cfg, db := newUsageTestDB(t)
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()

	input := MessageRecordInput("user-json-number", "room_runtime", map[string]any{
		"session_key": "room:group:conversation-1",
		"message_id":  "result-1",
		"round_id":    "round-1",
		"role":        "result",
		"timestamp":   json.Number("1777106383751"),
		"usage": map[string]any{
			"input_tokens":                json.Number("24777"),
			"output_tokens":               json.Number("727"),
			"cache_creation_input_tokens": json.Number("0"),
			"cache_read_input_tokens":     json.Number("15296"),
		},
	})
	if err := service.RecordMessageUsage(ctx, input); err != nil {
		t.Fatalf("写入 json.Number token usage 失败: %v", err)
	}

	summary, err := service.Summary(ctx, "user-json-number")
	if err != nil {
		t.Fatalf("汇总 json.Number token usage 失败: %v", err)
	}
	if summary.InputTokens != 24777 || summary.OutputTokens != 727 || summary.CacheReadInputTokens != 15296 {
		t.Fatalf("json.Number token 解析不正确: %+v", summary)
	}
	if summary.TotalTokens != 40800 {
		t.Fatalf("json.Number 总 token 不正确: %+v", summary)
	}
}

func TestServicePersistsNormalizedCacheAttributionAndSegments(t *testing.T) {
	cfg, db := newUsageTestDB(t)
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()
	fingerprint := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	input := RecordInput{
		OwnerUserID: "cache-owner",
		Source:      "room_runtime",
		SessionKey:  "room:cache:conversation",
		MessageID:   "result-cache",
		RoundID:     "round-cache",
		CacheAttribution: CacheAttribution{
			GoalScope:               ScopeBound,
			ExecutionScope:          ScopeNone,
			ResponsibilityLane:      "WORK",
			RuntimeKind:             "NXS",
			ProviderFingerprint:     fingerprint,
			ModelFingerprint:        "not-a-fingerprint",
			HostToolSurfaceComplete: true,
			ToolPolicyFingerprint:   fingerprint,
			MCPServersFingerprint:   fingerprint,
			ToolSurfaceFingerprint:  fingerprint,
		},
		Usage: map[string]any{
			"input_tokens":                100,
			"output_tokens":               10,
			"cache_creation_input_tokens": 20,
			"cache_read_input_tokens":     70,
		},
	}
	if err := service.RecordMessageUsage(ctx, input); err != nil {
		t.Fatalf("RecordMessageUsage() error = %v", err)
	}

	segments, err := service.CacheSegments(ctx, "cache-owner")
	if err != nil {
		t.Fatalf("CacheSegments() error = %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("segments = %+v, want one", segments)
	}
	segment := segments[0]
	if segment.CacheAttribution.GoalScope != ScopeBound ||
		segment.CacheAttribution.ExecutionScope != ScopeNone ||
		segment.CacheAttribution.ResponsibilityLane != "work" ||
		segment.CacheAttribution.RuntimeKind != "nxs" {
		t.Fatalf("normalized attribution = %+v", segment.CacheAttribution)
	}
	if segment.CacheAttribution.ModelFingerprint != "" {
		t.Fatalf("invalid fingerprint persisted: %q", segment.CacheAttribution.ModelFingerprint)
	}
	if segment.CacheReadInputTokens != 70 || segment.CacheCreationInputTokens != 20 || segment.MessageCount != 1 {
		t.Fatalf("cache provider usage = %+v", segment)
	}
	if share, ok := segment.CacheReadShare(); !ok || share < 0.777 || share > 0.778 {
		t.Fatalf("CacheReadShare() = %f, %v", share, ok)
	}
}

func newUsageTestDB(t *testing.T) (config.Config, *sql.DB) {
	t.Helper()

	root := t.TempDir()
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "usage.db"),
	}

	handlertest.MigrateSQLiteFromDir(t, cfg.DatabaseURL, usageMigrationDir(t))
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开 usage 测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return cfg, db
}

func usageMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位 usage 测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}

func TestSummaryDailyUsage(t *testing.T) {
	cfg, db := newUsageTestDB(t)
	service := NewServiceWithDB(cfg, db)
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	for _, item := range []struct {
		owner, id string
		day       int
	}{{"daily-owner", "a", 0}, {"daily-owner", "b", -1}, {"other-owner", "c", 0}, {"daily-owner", "old", -365}} {
		err := service.RecordMessageUsage(context.Background(), RecordInput{OwnerUserID: item.owner, SessionKey: "s", MessageID: item.id, OccurredAt: now.AddDate(0, 0, item.day), Usage: map[string]any{"input_tokens": int64(10), "output_tokens": int64(5), "cache_read_input_tokens": int64(20)}})
		if err != nil {
			t.Fatal(err)
		}
	}
	summary, err := service.Summary(context.Background(), "daily-owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Daily) != 365 || summary.Daily[0].Date != "2025-09-10" || summary.Daily[0].TotalTokens != 0 || summary.Daily[364].InputTokens != 10 || summary.Daily[363].CacheTokens != 20 {
		t.Fatalf("daily = %+v", summary.Daily)
	}
}
