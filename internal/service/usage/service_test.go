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

func TestServiceRecordsAndDeduplicatesMessageUsage(t *testing.T) {
	cfg, db := newUsageTestDB(t)
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()

	input := RecordInput{
		OwnerUserID: "user-1",
		Source:      "dm_runtime",
		SessionKey:  "agent:nexus:default:session-1",
		MessageID:   "result-1",
		RoundID:     "round-1",
		AgentID:     "nexus",
		Usage: map[string]any{
			"input_tokens":                100,
			"output_tokens":               20,
			"cache_creation_input_tokens": 3,
			"cache_read_input_tokens":     7,
		},
		OccurredAt: time.Unix(100, 0).UTC(),
	}
	if err := service.RecordMessageUsage(ctx, input); err != nil {
		t.Fatalf("写入 token usage 失败: %v", err)
	}
	if err := service.RecordMessageUsage(ctx, input); err != nil {
		t.Fatalf("重复写入 token usage 失败: %v", err)
	}

	summary, err := service.Summary(ctx, "user-1")
	if err != nil {
		t.Fatalf("汇总 token usage 失败: %v", err)
	}
	if summary.SessionCount != 1 || summary.MessageCount != 1 {
		t.Fatalf("去重计数不正确: %+v", summary)
	}
	if summary.InputTokens != 100 || summary.OutputTokens != 20 {
		t.Fatalf("输入输出 token 不正确: %+v", summary)
	}
	if summary.CacheCreationInputTokens != 3 || summary.CacheReadInputTokens != 7 || summary.TotalTokens != 130 {
		t.Fatalf("总 token 不正确: %+v", summary)
	}
}

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

func TestMessageHasUsage(t *testing.T) {
	t.Parallel()

	if MessageHasUsage(map[string]any{"usage": map[string]any{}}) {
		t.Fatal("空 usage 不应被判定为可入账")
	}
	if !MessageHasUsage(map[string]any{
		"usage": map[string]any{
			"cache_read_input_tokens": json.Number("59072"),
		},
	}) {
		t.Fatal("cache read usage 应被判定为可入账")
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

func TestServiceMissingCacheAttributionIsExplicitUnknown(t *testing.T) {
	cfg, db := newUsageTestDB(t)
	service := NewServiceWithDB(cfg, db)
	if err := service.RecordMessageUsage(context.Background(), RecordInput{
		OwnerUserID: "unknown-owner",
		Source:      "dm_runtime",
		SessionKey:  "agent:nexus:default:unknown",
		MessageID:   "result-unknown",
		Usage:       map[string]any{"cache_read_input_tokens": 12},
	}); err != nil {
		t.Fatalf("RecordMessageUsage() error = %v", err)
	}
	segments, err := service.CacheSegments(context.Background(), "unknown-owner")
	if err != nil {
		t.Fatalf("CacheSegments() error = %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("segments = %+v, want one", segments)
	}
	attribution := segments[0].CacheAttribution
	if attribution.GoalScope != ScopeUnknown ||
		attribution.ExecutionScope != ScopeUnknown ||
		attribution.ResponsibilityLane != ScopeUnknown {
		t.Fatalf("missing attribution = %+v, want explicit unknown", attribution)
	}
}

func TestServiceUsageSettlementSurvivesMissingAttributionColumns(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy-usage.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE token_usage_records (
		owner_user_id TEXT NOT NULL, usage_key TEXT NOT NULL, source TEXT NOT NULL,
		session_key TEXT NOT NULL, message_id TEXT NOT NULL, round_id TEXT NOT NULL,
		agent_id TEXT NOT NULL, room_id TEXT NOT NULL, conversation_id TEXT NOT NULL,
		input_tokens INTEGER NOT NULL, output_tokens INTEGER NOT NULL,
		cache_creation_input_tokens INTEGER NOT NULL, cache_read_input_tokens INTEGER NOT NULL,
		total_tokens INTEGER NOT NULL, occurred_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
		PRIMARY KEY (owner_user_id, usage_key)
	)`); err != nil {
		t.Fatalf("create legacy ledger: %v", err)
	}
	service := NewServiceWithDB(config.Config{DatabaseDriver: "sqlite"}, db)
	if err := service.RecordMessageUsage(context.Background(), RecordInput{
		OwnerUserID: "legacy-owner",
		Source:      "dm_runtime",
		SessionKey:  "agent:nexus:default:legacy",
		MessageID:   "result-legacy",
		Usage:       map[string]any{"input_tokens": 5},
	}); err != nil {
		t.Fatalf("settlement must ignore attribution update failure: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM token_usage_records`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("legacy usage count = %d, err=%v", count, err)
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
