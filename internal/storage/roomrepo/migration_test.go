package roomrepo

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestConversationDraftMigrationPreservesLegacyDataAndAddsUniqueDraft(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "conversation-drafts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	migrationDir := roomRepositoryMigrationDir(t, "sqlite")
	if err = goose.UpTo(db, migrationDir, 55); err != nil {
		t.Fatal(err)
	}
	seedConversationDraftMigrationFixture(t, db)
	if err = goose.UpTo(db, migrationDir, 57); err != nil {
		t.Fatal(err)
	}

	assertConversationDraftState(t, db, map[string]bool{
		"legacy-blank":           false,
		"legacy-generated-title": false,
		"legacy-explicit-title":  false,
		"legacy-message":         false,
		"legacy-sdk":             false,
	})

	var sessionCount int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE id IN ('session-empty', 'session-started')",
	).Scan(&sessionCount); err != nil {
		t.Fatal(err)
	}
	if sessionCount != 2 {
		t.Fatalf("legacy session count = %d, want 2", sessionCount)
	}
	var messageCount int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM messages WHERE id = 'message-1'",
	).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("legacy message count = %d, want 1", messageCount)
	}

	if _, err := db.Exec(`
INSERT INTO conversations (
    id, room_id, conversation_type, title, created_at, updated_at, last_activity_at, is_draft
) VALUES (
    'first-draft', 'room-main', 'topic', '', '2026-01-02 00:00:00',
    '2026-01-02 00:00:00', '2026-01-02 00:00:00', TRUE
)`); err != nil {
		t.Fatalf("first draft in room: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO conversations (
    id, room_id, conversation_type, title, created_at, updated_at, last_activity_at, is_draft
) VALUES (
    'second-draft', 'room-main', 'topic', '', '2026-01-02 00:01:00',
    '2026-01-02 00:01:00', '2026-01-02 00:01:00', TRUE
)`); err == nil {
		t.Fatal("second draft in one room should violate uq_conversations_room_draft")
	}
	if _, err := db.Exec(`
INSERT INTO conversations (
    id, room_id, conversation_type, title, created_at, updated_at, last_activity_at, is_draft
) VALUES (
    'other-room-draft', 'room-other', 'topic', '', '2026-01-02 00:00:00',
    '2026-01-02 00:00:00', '2026-01-02 00:00:00', TRUE
)`); err != nil {
		t.Fatalf("first draft in another room: %v", err)
	}

	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply remaining migrations: %v", err)
	}
	var version int64
	if err := db.QueryRow(
		"SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1",
	).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 88 {
		t.Fatalf("goose version = %d, want 88", version)
	}

	if err = goose.DownTo(db, migrationDir, 56); err != nil {
		t.Fatalf("roll back conversation draft migration: %v", err)
	}
	var draftColumnCount int
	if err := db.QueryRow(`
SELECT COUNT(*)
FROM pragma_table_info('conversations')
WHERE name = 'is_draft'
`).Scan(&draftColumnCount); err != nil {
		t.Fatal(err)
	}
	if draftColumnCount != 0 {
		t.Fatalf("is_draft column count after rollback = %d, want 0", draftColumnCount)
	}
	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatalf("reapply migrations after full rollback: %v", err)
	}
	if err = db.QueryRow(
		"SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1",
	).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 88 {
		t.Fatalf("goose version after reapply = %d, want 88", version)
	}
}

func TestConversationDraftMigrationsKeepSQLiteAndPostgresContract(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		path := filepath.Join(
			roomRepositoryMigrationDir(t, dialect),
			"00057_conversation_drafts.sql",
		)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sqlText := strings.ToLower(strings.Join(strings.Fields(string(body)), " "))
		for _, required := range []string{
			"add column",
			"is_draft boolean not null default false",
			"create unique index",
			"uq_conversations_room_draft",
			"where is_draft = true",
		} {
			if !strings.Contains(sqlText, required) {
				t.Fatalf("%s migration missing %q", dialect, required)
			}
		}
		for _, forbidden := range []string{
			"delete from",
			"update conversations",
			"row_number() over",
			"from messages",
			"from sessions",
		} {
			if strings.Contains(sqlText, forbidden) {
				t.Fatalf("%s migration must not infer or mutate legacy conversation state via %q", dialect, forbidden)
			}
		}
	}
}

func seedConversationDraftMigrationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO rooms (id, room_type, name, description, created_at, updated_at)
VALUES
    ('room-main', 'room', 'Main', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
    ('room-other', 'room', 'Other', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
		`INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, created_at, updated_at
) VALUES (
    'agent-draft-test', 'draft-test', 'Draft test', '', '', 'active', '',
    '2026-01-01 00:00:00', '2026-01-01 00:00:00'
)`,
		`INSERT INTO runtimes (
    id, agent_id, allowed_tools_json, disallowed_tools_json, mcp_servers_json,
    setting_sources_json, runtime_version, created_at, updated_at
) VALUES (
    'runtime-draft-test', 'agent-draft-test', '[]', '[]', '{}', '[]', 1,
    '2026-01-01 00:00:00', '2026-01-01 00:00:00'
)`,
		`INSERT INTO conversations (
    id, room_id, conversation_type, title, created_at, updated_at, last_activity_at
) VALUES
    ('legacy-blank', 'room-main', 'topic', '', '2026-01-01 09:00:00', '2026-01-01 09:00:00', '2026-01-01 09:00:00'),
    ('legacy-generated-title', 'room-main', 'topic', 'Main · 对话 2', '2026-01-01 10:00:00', '2026-01-01 10:00:00', '2026-01-01 10:00:00'),
    ('legacy-explicit-title', 'room-main', 'topic', 'Intentional research', '2026-01-01 11:00:00', '2026-01-01 11:00:00', '2026-01-01 11:00:00'),
    ('legacy-message', 'room-main', 'topic', '', '2026-01-01 12:00:00', '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    ('legacy-sdk', 'room-main', 'topic', '', '2026-01-01 13:00:00', '2026-01-01 13:00:00', '2026-01-01 13:00:00')`,
		`INSERT INTO sessions (
    id, conversation_id, agent_id, runtime_id, version_no, branch_key, is_primary,
    sdk_session_id, status, last_activity_at, created_at, updated_at
) VALUES
    (
        'session-empty', 'legacy-generated-title', 'agent-draft-test', 'runtime-draft-test',
        1, 'main', TRUE, '   ', 'idle', '2026-01-01 10:00:00',
        '2026-01-01 10:00:00', '2026-01-01 10:00:00'
    ),
    (
        'session-started', 'legacy-sdk', 'agent-draft-test', 'runtime-draft-test',
        1, 'main', TRUE, 'sdk-session-1', 'idle', '2026-01-01 13:00:00',
        '2026-01-01 13:00:00', '2026-01-01 13:00:00'
    )`,
		`INSERT INTO messages (
    id, conversation_id, sender_type, kind, status, jsonl_path, created_at, updated_at
) VALUES (
    'message-1', 'legacy-message', 'user', 'text', 'completed', 'room.jsonl',
    '2026-01-01 12:00:00', '2026-01-01 12:00:00'
)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed draft migration fixture: %v\n%s", err, statement)
		}
	}
}

func assertConversationDraftState(t *testing.T, db *sql.DB, expected map[string]bool) {
	t.Helper()
	rows, err := db.Query("SELECT id, is_draft FROM conversations")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	actual := make(map[string]bool)
	for rows.Next() {
		var (
			id      string
			isDraft bool
		)
		if err := rows.Scan(&id, &isDraft); err != nil {
			t.Fatal(err)
		}
		actual[id] = isDraft
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(actual) != len(expected) {
		t.Fatalf("conversation states = %#v, want %#v", actual, expected)
	}
	for id, wantDraft := range expected {
		if gotDraft, ok := actual[id]; !ok || gotDraft != wantDraft {
			t.Fatalf("conversation %s draft = %t, exists = %t, want %t", id, gotDraft, ok, wantDraft)
		}
	}
}

func roomRepositoryMigrationDir(t *testing.T, dialect string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration test file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", dialect)
}
