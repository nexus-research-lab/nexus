package agentrepo

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLRepositoryRuntimeVersionCompareAndSwap(t *testing.T) {
	db := newAgentRepositoryTestDB(t)
	repository := NewSQLRepository("sqlite", db)
	ctx := context.Background()

	created, err := repository.CreateAgent(ctx, testCreateRecord())
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if created.RuntimeVersion != 1 {
		t.Fatalf("created runtime version = %d, want 1", created.RuntimeVersion)
	}

	expectedVersion := created.RuntimeVersion
	record := testUpdateRecord()
	record.Name = "updated-agent"
	record.Model = "model-v2"
	record.ExpectedRuntimeVersion = &expectedVersion
	updated, err := repository.UpdateAgent(ctx, record)
	if err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}
	if updated.RuntimeVersion != 2 {
		t.Fatalf("updated runtime version = %d, want 2", updated.RuntimeVersion)
	}
	if updated.Options.Model != "model-v2" {
		t.Fatalf("updated model = %q, want model-v2", updated.Options.Model)
	}

	staleRecord := record
	staleRecord.Name = "stale-agent"
	staleRecord.Model = "stale-model"
	staleRecord.ExpectedRuntimeVersion = &expectedVersion
	if _, err = repository.UpdateAgent(ctx, staleRecord); !errors.Is(err, ErrRuntimeVersionConflict) {
		t.Fatalf("stale UpdateAgent() error = %v, want ErrRuntimeVersionConflict", err)
	}

	current, err := repository.GetAgent(ctx, record.AgentID, record.OwnerUserID)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if current == nil {
		t.Fatal("GetAgent() returned nil")
	}
	if current.RuntimeVersion != 2 || current.Name != "updated-agent" || current.Options.Model != "model-v2" {
		t.Fatalf("stale update was not rolled back atomically: %+v", current)
	}

	record.Name = "unconditional-agent"
	record.Model = "model-v3"
	record.ExpectedRuntimeVersion = nil
	unconditional, err := repository.UpdateAgent(ctx, record)
	if err != nil {
		t.Fatalf("unconditional UpdateAgent() error = %v", err)
	}
	if unconditional.RuntimeVersion != 3 {
		t.Fatalf("unconditional runtime version = %d, want 3", unconditional.RuntimeVersion)
	}
}

func TestSQLRepositorySkillSelectionCompareAndSwap(t *testing.T) {
	db := newAgentRepositoryTestDB(t)
	repository := NewSQLRepository("sqlite", db)
	ctx := context.Background()

	created, err := repository.CreateAgent(ctx, testCreateRecord())
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	updated, err := repository.UpdateAgentSkillIDsAtVersion(
		ctx,
		created.AgentID,
		created.OwnerUserID,
		`["global-skill"]`,
		created.RuntimeVersion,
	)
	if err != nil {
		t.Fatalf("UpdateAgentSkillIDsAtVersion() error = %v", err)
	}
	if updated.RuntimeVersion != created.RuntimeVersion+1 {
		t.Fatalf("updated runtime version = %d, want %d", updated.RuntimeVersion, created.RuntimeVersion+1)
	}
	withWorkspaceDisabled, err := repository.UpdateAgentDisabledSkillIDsAtVersion(
		ctx,
		created.AgentID,
		created.OwnerUserID,
		`["workspace-skill"]`,
		updated.RuntimeVersion,
	)
	if err != nil {
		t.Fatalf("UpdateAgentDisabledSkillIDsAtVersion() error = %v", err)
	}
	if _, err = repository.UpdateAgentSkillIDsAtVersion(
		ctx,
		created.AgentID,
		created.OwnerUserID,
		`["stale-global"]`,
		created.RuntimeVersion,
	); !errors.Is(err, ErrRuntimeVersionConflict) {
		t.Fatalf("stale skill selection error = %v, want ErrRuntimeVersionConflict", err)
	}
	current, err := repository.GetAgent(ctx, created.AgentID, created.OwnerUserID)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if current.RuntimeVersion != withWorkspaceDisabled.RuntimeVersion ||
		len(current.Options.SkillIDs) != 1 ||
		current.Options.SkillIDs[0] != "global-skill" ||
		len(current.Options.DisabledSkillIDs) != 1 ||
		current.Options.DisabledSkillIDs[0] != "workspace-skill" {
		t.Fatalf("stale skill selection changed state: %+v", current)
	}
}

func TestSQLRepositoryListsMainAgentThenNewestAgents(t *testing.T) {
	db := newAgentRepositoryTestDB(t)
	repository := NewSQLRepository("sqlite", db)
	ctx := context.Background()

	main := testCreateRecord()
	main.AgentID = "agent-main"
	main.ProfileID = "profile-main"
	main.RuntimeID = "runtime-main"
	main.IsMain = true
	older := testCreateRecord()
	older.AgentID = "agent-older"
	older.ProfileID = "profile-older"
	older.RuntimeID = "runtime-older"
	newer := testCreateRecord()
	newer.AgentID = "agent-newer"
	newer.ProfileID = "profile-newer"
	newer.RuntimeID = "runtime-newer"
	for _, record := range []CreateRecord{main, older, newer} {
		if _, err := repository.CreateAgent(ctx, record); err != nil {
			t.Fatalf("CreateAgent(%q) error = %v", record.AgentID, err)
		}
	}
	if _, err := db.Exec(`
UPDATE agents
SET created_at = CASE id
    WHEN 'agent-main' THEN '2026-08-09 00:00:00'
    WHEN 'agent-older' THEN '2026-08-10 00:00:00'
    WHEN 'agent-newer' THEN '2026-08-11 00:00:00'
END`); err != nil {
		t.Fatalf("set agent creation times: %v", err)
	}

	agents, err := repository.ListActiveAgents(ctx, older.OwnerUserID)
	if err != nil {
		t.Fatalf("ListActiveAgents() error = %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("agent count = %d, want 3", len(agents))
	}
	if agents[0].AgentID != main.AgentID || agents[1].AgentID != newer.AgentID || agents[2].AgentID != older.AgentID {
		t.Fatalf("agent order = %v, want [%s %s %s]", []string{agents[0].AgentID, agents[1].AgentID, agents[2].AgentID}, main.AgentID, newer.AgentID, older.AgentID)
	}
}

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

func testCreateRecord() CreateRecord {
	return CreateRecord{
		AgentID:              "agent-1",
		OwnerUserID:          "owner-1",
		Slug:                 "agent-1",
		Name:                 "test-agent",
		WorkspacePath:        "/tmp/agent-1",
		Status:               "active",
		BusinessTagsJSON:     "[]",
		VibeTagsJSON:         "[]",
		DisplayName:          "test-agent",
		RuntimeID:            "runtime-1",
		ProfileID:            "profile-1",
		Provider:             "provider-1",
		Model:                "model-v1",
		PermissionMode:       "default",
		AllowedToolsJSON:     "[]",
		DisallowedToolsJSON:  "[]",
		MCPServersJSON:       "{}",
		ConnectorIDsJSON:     "[]",
		SkillIDsJSON:         "[]",
		DisabledSkillIDsJSON: "[]",
		SettingSourcesJSON:   "[]",
		RuntimeVersion:       1,
	}
}

func testUpdateRecord() UpdateRecord {
	return UpdateRecord{
		AgentID:              "agent-1",
		OwnerUserID:          "owner-1",
		Name:                 "test-agent",
		WorkspacePath:        "/tmp/agent-1",
		BusinessTagsJSON:     "[]",
		VibeTagsJSON:         "[]",
		Provider:             "provider-1",
		Model:                "model-v1",
		PermissionMode:       "default",
		AllowedToolsJSON:     "[]",
		DisallowedToolsJSON:  "[]",
		MCPServersJSON:       "{}",
		ConnectorIDsJSON:     "[]",
		SkillIDsJSON:         "[]",
		DisabledSkillIDsJSON: "[]",
		SettingSourcesJSON:   "[]",
	}
}
