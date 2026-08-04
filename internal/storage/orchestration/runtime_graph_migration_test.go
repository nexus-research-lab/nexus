package orchestration

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRuntimeGraphGateAndSummaryMigrationsPreserveGraphAndAcceptControlFlow(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "runtime-graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	migrationDir := "../../../db/migrations/sqlite"
	if err = goose.UpTo(db, migrationDir, 65); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO runtime_graph_node_runs (
    node_run_id, graph_id, owner_user_id, session_key, execution_id,
    node_kind, subject_id, root_round_id, runtime_round_id, agent_round_id,
    status, started_at, updated_at
) VALUES
    ('node-agent', 'graph-1', 'owner-1', 'session-1', 'execution-1',
     'agent', 'agent-round-1', 'round-1', 'round-1', 'agent-round-1',
     'running', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('node-tool', 'graph-1', 'owner-1', 'session-1', 'execution-1',
     'tool', 'tool-1', 'round-1', 'round-1', 'agent-round-1',
     'succeeded', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO runtime_graph_edge_runs (
    edge_run_id, graph_id, owner_user_id, session_key,
    source_node_run_id, target_node_run_id, edge_kind, created_at
) VALUES (
    'edge-invoke', 'graph-1', 'owner-1', 'session-1',
    'node-agent', 'node-tool', 'invoke', CURRENT_TIMESTAMP
);`); err != nil {
		t.Fatalf("seed runtime graph at version 65: %v", err)
	}

	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatalf("upgrade runtime graph schema: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO runtime_graph_node_runs (
    node_run_id, graph_id, owner_user_id, session_key, execution_id,
    node_kind, subject_id, root_round_id, runtime_round_id, agent_round_id,
    status, result_summary, error_code, error_summary, summary_truncated,
    duration_ms, started_at, updated_at, metadata_json
) VALUES
(
    'node-gate', 'graph-1', 'owner-1', 'session-1', 'execution-1',
    'gate', 'alignment-1', 'round-1', 'round-1', 'agent-round-1',
    'succeeded', NULL, NULL, NULL, FALSE, 0,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '{"decision":"not_aligned"}'
),
(
    'node-tool-retry', 'graph-1', 'owner-1', 'session-1', 'execution-1',
    'tool', 'tool-2', 'round-1', 'round-1', 'agent-round-1',
    'succeeded', 'Found the page', NULL, NULL, FALSE, 42,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '{}'
);
INSERT INTO runtime_graph_edge_runs (
    edge_run_id, graph_id, owner_user_id, session_key,
    source_node_run_id, target_node_run_id, edge_kind, created_at
) VALUES
    ('edge-guard', 'graph-1', 'owner-1', 'session-1',
     'node-agent', 'node-gate', 'guard', CURRENT_TIMESTAMP),
    ('edge-loop', 'graph-1', 'owner-1', 'session-1',
     'node-gate', 'node-agent', 'loop_back', CURRENT_TIMESTAMP),
    ('edge-retry', 'graph-1', 'owner-1', 'session-1',
     'node-tool', 'node-tool-retry', 'retry', CURRENT_TIMESTAMP);
`); err != nil {
		t.Fatalf("insert Gate control flow after migration: %v", err)
	}

	var nodeCount, edgeCount, foreignKeyViolations, version int
	if err = db.QueryRow(`SELECT COUNT(*) FROM runtime_graph_node_runs`).Scan(&nodeCount); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM runtime_graph_edge_runs`).Scan(&edgeCount); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&foreignKeyViolations); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if nodeCount != 4 || edgeCount != 4 || foreignKeyViolations != 0 || version != 68 {
		t.Fatalf(
			"migration result nodes=%d edges=%d fk=%d version=%d",
			nodeCount,
			edgeCount,
			foreignKeyViolations,
			version,
		)
	}
}
