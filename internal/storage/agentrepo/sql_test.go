package agentrepo

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newAgentRepositoryTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})

	schema := []string{
		`CREATE TABLE agent_creation_requests (
			owner_user_id TEXT NOT NULL,
			creation_request_id TEXT NOT NULL,
			intent_digest TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			workspace_path TEXT NOT NULL,
			status TEXT NOT NULL,
			stage TEXT NOT NULL DEFAULT 'reserved',
			claim_token TEXT,
			lease_expires_at_ms INTEGER NOT NULL DEFAULT 0,
			failure_code TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (owner_user_id, creation_request_id),
			UNIQUE (owner_user_id, agent_id)
		)`,
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL,
			slug TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			definition TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			workspace_path TEXT NOT NULL,
			is_main BOOLEAN NOT NULL DEFAULT FALSE,
			avatar TEXT,
			vibe_tags TEXT NOT NULL DEFAULT '[]',
			business_tags TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE profiles (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			avatar_url TEXT,
			headline TEXT,
			profile_markdown TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE runtimes (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL UNIQUE,
			provider TEXT,
			model TEXT,
			permission_mode TEXT,
			allowed_tools_json TEXT NOT NULL DEFAULT '[]',
			disallowed_tools_json TEXT NOT NULL DEFAULT '[]',
			mcp_servers_json TEXT NOT NULL DEFAULT '{}',
			connector_ids_json TEXT NOT NULL DEFAULT '[]',
			skill_ids_json TEXT NOT NULL DEFAULT '[]',
			disabled_skill_ids_json TEXT NOT NULL DEFAULT '[]',
			max_turns INTEGER,
			max_thinking_tokens INTEGER,
			setting_sources_json TEXT NOT NULL DEFAULT '[]',
			runtime_version INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, statement := range schema {
		if _, err = db.Exec(statement); err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}
