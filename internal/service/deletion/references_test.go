package deletion

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	_ "modernc.org/sqlite"
)

func TestSessionCleanupRollbackAndOwnerIsolation(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	run := func(query string) {
		t.Helper()
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	// 仅建立本测试涉及的列，故障注入验证跨领域事务，不依赖领域创建流程。
	run(`CREATE TABLE agents (id TEXT, owner_user_id TEXT)`)
	run(`INSERT INTO agents VALUES ('agent-a','owner-a'),('agent-b','owner-b')`)
	run(`CREATE TABLE automation_delivery_routes (agent_id TEXT, session_key TEXT)`)
	run(`INSERT INTO automation_delivery_routes VALUES ('agent-a','session'),('agent-b','session')`)
	tables := []string{"runtime_graph_artifact_refs", "runtime_graph_edge_runs", "runtime_graph_node_runs", "execution_plan_proposals", "executions"}
	for _, table := range tables {
		run("CREATE TABLE " + table + " (owner_user_id TEXT, session_key TEXT)")
		run("INSERT INTO " + table + " VALUES ('owner-a','session'),('owner-b','session'),('owner-a','other')")
	}
	run(`CREATE TRIGGER reject_cleanup BEFORE DELETE ON executions BEGIN SELECT RAISE(ABORT,'injected failure'); END`)
	c := NewCoordinator(config.Config{DatabaseDriver: "sqlite"}, db)
	if err := c.CleanupSessionReferences(context.Background(), "owner-a", []string{" session ", "session", ""}); err == nil {
		t.Fatal("注入失败未传递")
	}
	count := func(table string, want int) {
		t.Helper()
		var n int
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Fatalf("%s count=%d want=%d", table, n, want)
		}
	}
	count("automation_delivery_routes", 2)
	for _, table := range tables {
		count(table, 3)
	}
	run(`DROP TRIGGER reject_cleanup`)
	if err := c.CleanupSessionReferences(context.Background(), "owner-a", []string{"session"}); err != nil {
		t.Fatal(err)
	}
	count("automation_delivery_routes", 1)
	for _, table := range tables {
		count(table, 2)
	}
	if err := c.CleanupSessionReferences(context.Background(), "owner-a", []string{"session"}); err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		count(table, 2)
	}
	var owner string
	if err := db.QueryRow(`SELECT a.owner_user_id FROM automation_delivery_routes r JOIN agents a ON a.id=r.agent_id`).Scan(&owner); err != nil || owner != "owner-b" {
		t.Fatalf("剩余 owner=%q err=%v", owner, err)
	}
}
