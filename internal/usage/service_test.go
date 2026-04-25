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

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
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

func newUsageTestDB(t *testing.T) (config.Config, *sql.DB) {
	t.Helper()

	root := t.TempDir()
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "usage.db"),
	}

	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开 usage 测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, usageMigrationDir(t)); err != nil {
		t.Fatalf("执行 usage migration 失败: %v", err)
	}
	return cfg, db
}

func usageMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位 usage 测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}
