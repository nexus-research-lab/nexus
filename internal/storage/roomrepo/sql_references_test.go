package roomrepo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

func TestConversationReferenceQueriesUseSQLiteAndPostgresBindings(t *testing.T) {
	sessionKeys := []string{
		"room:group:conversation-1",
		"agent:agent-1:ws:group:conversation-1",
	}
	expectedSources := map[string]struct{}{
		"automation_scheduled_tasks":    {},
		"automation_task_runs":          {},
		"automation_delivery_routes":    {},
		"im_ingress_messages":           {},
		"token_usage_records":           {},
		"session_goals":                 {},
		"goal_events":                   {},
		"goal_usage_source_checkpoints": {},
		"goal_usage_scope_bindings":     {},
		"goal_usage_source_pending":     {},
		"goal_usage_source_evidence":    {},
		"goal_usage_parent_ledger":      {},
		"rounds":                        {},
	}

	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			repository := &SQLRepository{dialect: storage.NewSQLDialect(driver)}
			queries := repository.buildConversationReferenceQueries(
				"owner-1",
				"conversation-1",
				sessionKeys,
			)
			if len(queries) != len(expectedSources) {
				t.Fatalf("reference query count = %d, want %d", len(queries), len(expectedSources))
			}
			seen := make(map[string]struct{}, len(queries))
			for _, query := range queries {
				if _, expected := expectedSources[query.source]; !expected {
					t.Fatalf("unexpected reference source %q", query.source)
				}
				seen[query.source] = struct{}{}
				if !strings.Contains(query.sql, "SELECT 1") ||
					!strings.Contains(query.sql, "LIMIT 1") {
					t.Fatalf("%s query is not a bounded existence probe:\n%s", query.source, query.sql)
				}
				assertConversationReferenceBindings(t, driver, query)
			}
			if len(seen) != len(expectedSources) {
				t.Fatalf("reference sources = %#v, want %#v", seen, expectedSources)
			}
		})
	}
}

func assertConversationReferenceBindings(
	t *testing.T,
	driver string,
	query conversationReferenceQuery,
) {
	t.Helper()
	if driver == "sqlite" {
		if got := strings.Count(query.sql, "?"); got != len(query.args) {
			t.Fatalf(
				"%s SQLite bind count = %d, args = %d\n%s",
				query.source,
				got,
				len(query.args),
				query.sql,
			)
		}
		if strings.Contains(query.sql, "$1") {
			t.Fatalf("%s SQLite query contains PostgreSQL bind:\n%s", query.source, query.sql)
		}
		return
	}

	if strings.Contains(query.sql, "?") {
		t.Fatalf("%s PostgreSQL query contains SQLite bind:\n%s", query.source, query.sql)
	}
	for index := range query.args {
		bind := fmt.Sprintf("$%d", index+1)
		if !strings.Contains(query.sql, bind) {
			t.Fatalf("%s PostgreSQL query misses %s:\n%s", query.source, bind, query.sql)
		}
	}
	extraBind := fmt.Sprintf("$%d", len(query.args)+1)
	if strings.Contains(query.sql, extraBind) {
		t.Fatalf("%s PostgreSQL query has extra bind %s:\n%s", query.source, extraBind, query.sql)
	}
}

func TestNormalizeConversationReferenceKeys(t *testing.T) {
	got := normalizeConversationReferenceKeys([]string{
		" room:group:b ",
		"",
		"room:group:a",
		"room:group:b",
	})
	want := []string{"room:group:a", "room:group:b"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("normalized keys = %#v, want %#v", got, want)
	}
}
