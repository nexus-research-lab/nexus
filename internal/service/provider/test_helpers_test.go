package provider

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func newTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()

	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	t.Cleanup(func() { _ = db.Close() })
	return NewServiceWithDB(cfg, db), db
}

func insertProviderUsageAgent(
	t *testing.T,
	db *sql.DB,
	agentID string,
	slug string,
	name string,
	displayName string,
	isMain bool,
	provider string,
	status string,
) {
	t.Helper()
	insertProviderUsageAgentForOwner(t, db, authctx.SystemUserID, agentID, slug, name, displayName, isMain, provider, status)
}

func insertProviderUsageAgentForOwner(
	t *testing.T,
	db *sql.DB,
	ownerUserID string,
	agentID string,
	slug string,
	name string,
	displayName string,
	isMain bool,
	provider string,
	status string,
) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES (?, ?, ?, '', '', ?, ?, ?, ?)`,
		agentID,
		slug,
		name,
		status,
		"/tmp/"+slug,
		ownerUserID,
		isMain,
	)
	if err != nil {
		t.Fatalf("插入 agent 失败: %v", err)
	}
	_, err = db.Exec(`
INSERT INTO profiles (
    id, agent_id, display_name, headline, profile_markdown
) VALUES (?, ?, ?, '', '')`,
		"profile-"+agentID,
		agentID,
		displayName,
	)
	if err != nil {
		t.Fatalf("插入 profile 失败: %v", err)
	}
	_, err = db.Exec(`
INSERT INTO runtimes (
    id, agent_id, provider, permission_mode, allowed_tools_json, disallowed_tools_json,
    mcp_servers_json, setting_sources_json, runtime_version
) VALUES (?, ?, ?, '', '[]', '[]', '{}', '[]', 1)`,
		"runtime-"+agentID,
		agentID,
		provider,
	)
	if err != nil {
		t.Fatalf("插入 runtime 失败: %v", err)
	}
}

func providerTestContext(userID string, role string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     userID,
		Username:   userID,
		Role:       role,
		AuthMethod: authctx.AuthMethodPassword,
	})
}

func stringPointer(value string) *string {
	return &value
}

func setTestDefaultAgentSelection(service *Service, selection DefaultAgentSelection) {
	service.SetDefaultAgentSelectionResolver(func(context.Context, string) (DefaultAgentSelection, error) {
		return selection, nil
	})
}

type runtimeSelection struct {
	provider string
	model    string
	version  int64
}

func runtimeSelectionsByAgent(t *testing.T, db *sql.DB, agentIDs ...string) map[string]runtimeSelection {
	t.Helper()
	result := map[string]runtimeSelection{}
	for _, agentID := range agentIDs {
		row := db.QueryRow(
			`SELECT COALESCE(provider, ''), COALESCE(model, ''), runtime_version
			 FROM runtimes WHERE agent_id = ? LIMIT 1`,
			agentID,
		)
		var item runtimeSelection
		if err := row.Scan(&item.provider, &item.model, &item.version); err != nil {
			t.Fatalf("读取 runtime provider/model 失败: %v", err)
		}
		result[agentID] = item
	}
	return result
}

func optionByProvider(items []Option, provider string) *Option {
	for index := range items {
		if items[index].Provider == provider {
			return &items[index]
		}
	}
	return nil
}
