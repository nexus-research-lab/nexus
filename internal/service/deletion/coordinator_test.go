package deletion

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "modernc.org/sqlite"
)

func TestCoordinatorKeepsOriginalPayloadUntilDeletionCompletes(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "deletion.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err = db.Exec(`
CREATE TABLE deletion_jobs (
    job_id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    target_id TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE (owner_user_id, kind, target_id)
)`); err != nil {
		t.Fatal(err)
	}

	coordinator := NewCoordinator(config.Config{DatabaseDriver: "sqlite"}, db)
	ctx := context.Background()
	first, err := coordinator.Ensure(ctx, "owner-a", KindSession, "session-a", map[string]string{
		"workspace": "workspace-a",
	})
	if err != nil {
		t.Fatalf("登记删除任务失败: %v", err)
	}
	second, err := coordinator.Ensure(ctx, "owner-a", KindSession, "session-a", map[string]string{
		"workspace": "workspace-overwritten",
	})
	if err != nil {
		t.Fatalf("重复登记删除任务失败: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("同一删除目标应复用稳定 job id: %s != %s", first.ID, second.ID)
	}
	var payload map[string]string
	if err = DecodePayload(second, &payload); err != nil {
		t.Fatalf("解码删除 payload 失败: %v", err)
	}
	if payload["workspace"] != "workspace-a" {
		t.Fatalf("重复登记不应覆盖最初完整 payload: %+v", payload)
	}

	failure := errors.New("transcript 暂时不可删除")
	if err = coordinator.Fail(ctx, first, failure); !errors.Is(err, failure) {
		t.Fatalf("Fail 应保留原始错误: %v", err)
	}
	pending, err := coordinator.ListPending(ctx, KindSession)
	if err != nil {
		t.Fatalf("列出待恢复删除失败: %v", err)
	}
	if len(pending) != 1 || pending[0].Attempts != 1 || pending[0].LastError != failure.Error() {
		t.Fatalf("失败状态未持久化: %+v", pending)
	}
	if err = coordinator.Complete(ctx, pending[0]); err != nil {
		t.Fatalf("完成删除任务失败: %v", err)
	}
	pending, err = coordinator.ListPending(ctx, KindSession)
	if err != nil || len(pending) != 0 {
		t.Fatalf("完成后删除任务仍残留: jobs=%+v err=%v", pending, err)
	}
}
