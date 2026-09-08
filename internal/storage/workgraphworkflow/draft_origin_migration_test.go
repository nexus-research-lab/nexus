// INPUT: 旧版 Draft 表中的完整版本历史、选择和后台保存标记。
// OUTPUT: 原样保留草稿和编辑身份的迁移验证；退休 claim 不再阻塞恢复。
// POS: 保存统一化的 SQLite 跨版本升级回归。
package workgraphworkflow

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/pressly/goose/v3"
)

func TestDraftOriginMigrationPreservesExistingVersionsAndEditor(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "upgrade.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ensureGooseSQLiteDialect(t)
	if err = goose.UpTo(db, "../../../db/migrations/sqlite", 133); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO workgraph_workflow_drafts (
preview_id, owner_user_id, source_execution_id, source_session_key, source_agent_id, output_language,
head_revision, selected_revision, editor_id, editor_agent_id, editor_session_key, save_scheduled, expires_at)
VALUES ('preview-a','owner-a','execution-a','session-a','agent-a','zh',2,1,'editor-a','main','editor-session',TRUE,'2026-10-01 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	for revision, name := range map[int]string{1: "report", 2: "chain"} {
		if _, err = db.Exec(`INSERT INTO workgraph_workflow_draft_versions (preview_id, revision, preview_json) VALUES ('preview-a',?,?)`,
			revision, `{"preview_id":"preview-a","slash_name":"`+name+`","source_execution_id":"execution-a","source_session_key":"session-a"}`); err != nil {
			t.Fatal(err)
		}
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	draft, err := r.GetDraftByID(context.Background(), "owner-a", "preview-a")
	if err != nil || draft == nil || len(draft.Versions) != 2 || draft.HeadRevision != 2 || draft.SelectedRevision != 1 ||
		draft.Preview.SlashName != "report" || draft.Versions[1].Preview.SlashName != "chain" || draft.EditorID != "editor-a" || draft.EditorSessionKey != "editor-session" || draft.SaveScheduled || draft.OriginWorkflowID != "" {
		t.Fatalf("upgrade changed existing Draft: %#v, %v", draft, err)
	}
	var foreignKeyViolations int
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		foreignKeyViolations++
	}
	if rows.Err() != nil || foreignKeyViolations != 0 {
		t.Fatalf("broken Draft history foreign keys: %d, %v", foreignKeyViolations, rows.Err())
	}
}
