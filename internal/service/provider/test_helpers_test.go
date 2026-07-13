package provider

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
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

type runtimeSelection struct {
	provider string
	model    string
}

func runtimeSelectionsByAgent(t *testing.T, db *sql.DB, agentIDs ...string) map[string]runtimeSelection {
	t.Helper()
	result := map[string]runtimeSelection{}
	for _, agentID := range agentIDs {
		row := db.QueryRow(`SELECT COALESCE(provider, ''), COALESCE(model, '') FROM runtimes WHERE agent_id = ? LIMIT 1`, agentID)
		var item runtimeSelection
		if err := row.Scan(&item.provider, &item.model); err != nil {
			t.Fatalf("读取 runtime provider/model 失败: %v", err)
		}
		result[agentID] = item
	}
	return result
}

type capturedLogRecord struct {
	message string
	attrs   map[string]any
}

type captureSlogHandler struct {
	mu      sync.Mutex
	records []capturedLogRecord
}

func (h *captureSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureSlogHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := map[string]any{}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, capturedLogRecord{
		message: record.Message,
		attrs:   attrs,
	})
	return nil
}

func (h *captureSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureSlogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *captureSlogHandler) find(message string) *capturedLogRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	for index := range h.records {
		if h.records[index].message == message {
			record := h.records[index]
			return &record
		}
	}
	return nil
}

func (h *captureSlogHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]string, 0, len(h.records))
	for _, record := range h.records {
		result = append(result, record.message)
	}
	return result
}

func hasOptionProvider(items []Option, provider string) bool {
	return optionByProvider(items, provider) != nil
}

func optionByProvider(items []Option, provider string) *Option {
	for index := range items {
		if items[index].Provider == provider {
			return &items[index]
		}
	}
	return nil
}

func hasModelOption(items []ModelOption, modelID string) bool {
	for _, item := range items {
		if item.ModelID == modelID {
			return true
		}
	}
	return false
}
