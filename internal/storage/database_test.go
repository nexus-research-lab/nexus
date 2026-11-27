package storage

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNullableTimeSupportsDriverAndSQLiteAggregateValues(t *testing.T) {
	want := time.Date(2026, 8, 17, 12, 34, 56, 123456789, time.UTC)
	values := []any{
		want,
		want.Format(time.RFC3339Nano),
		[]byte(want.Format("2006-01-02 15:04:05.999999999-07:00")),
	}
	for _, value := range values {
		got, err := NullableTime(value)
		if err != nil {
			t.Fatalf("NullableTime(%T): %v", value, err)
		}
		if got == nil || !got.Equal(want) {
			t.Fatalf("NullableTime(%T) = %v, want %v", value, got, want)
		}
	}
	got, err := NullableTime(nil)
	if err != nil || got != nil {
		t.Fatalf("NullableTime(nil) = %v, %v", got, err)
	}
}

func TestNormalizeDatabaseURLExpandsHomeAfterSQLiteScheme(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("读取用户目录失败: %v", err)
	}

	got := NormalizeDatabaseURL("sqlite:///~/.nexus/data/nexus.db")
	want := filepath.Join(home, ".nexus", "data", "nexus.db")
	if got != want {
		t.Fatalf("sqlite URL home 展开不正确: got=%q want=%q", got, want)
	}

	got = NormalizeDatabaseURL(`sqlite:///~\.nexus\data\nexus.db`)
	want = filepath.Join(home, ".nexus", "data", "nexus.db")
	if got != want {
		t.Fatalf("sqlite URL Windows home 展开不正确: got=%q want=%q", got, want)
	}
}

func TestSQLDialect(t *testing.T) {
	tests := []struct {
		name       string
		driver     string
		firstBind  string
		threeBinds string
		timestamp  string
		jsonText   string
		jsonValue  string
		insert     string
		suffix     string
	}{
		{
			name:       "postgres",
			driver:     "postgres",
			firstBind:  "$1",
			threeBinds: "$1,$2,$3",
			timestamp:  "now()",
			jsonText:   "payload::text",
			jsonValue:  "$2::json",
			insert:     "INSERT INTO sessions",
			suffix:     "\nON CONFLICT DO NOTHING",
		},
		{
			name:       "sqlite",
			driver:     "sqlite",
			firstBind:  "?",
			threeBinds: "?,?,?",
			timestamp:  "CURRENT_TIMESTAMP",
			jsonText:   "payload",
			jsonValue:  "json(?)",
			insert:     "INSERT OR IGNORE INTO sessions",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dialect := NewSQLDialect(test.driver)
			if got := dialect.Bind(1); got != test.firstBind {
				t.Fatalf("Bind(1) = %q, want %q", got, test.firstBind)
			}
			if got := dialect.BindList(3); got != test.threeBinds {
				t.Fatalf("BindList(3) = %q, want %q", got, test.threeBinds)
			}
			if got := dialect.CurrentTimestamp(); got != test.timestamp {
				t.Fatalf("CurrentTimestamp() = %q, want %q", got, test.timestamp)
			}
			if got := dialect.JSONText("payload"); got != test.jsonText {
				t.Fatalf("JSONText() = %q, want %q", got, test.jsonText)
			}
			if got := dialect.JSONValue(2); got != test.jsonValue {
				t.Fatalf("JSONValue() = %q, want %q", got, test.jsonValue)
			}
			if got := dialect.InsertIgnoreInto("sessions"); got != test.insert {
				t.Fatalf("InsertIgnoreInto() = %q, want %q", got, test.insert)
			}
			if got := dialect.InsertIgnoreSuffix(); got != test.suffix {
				t.Fatalf("InsertIgnoreSuffix() = %q, want %q", got, test.suffix)
			}
		})
	}
}

func TestSQLiteConnectionDSNDefaultsPageCacheAndKeepsExplicitValue(t *testing.T) {
	pragmasOf := func(dsn string) []string {
		_, query, _ := strings.Cut(dsn, "?")
		values, err := url.ParseQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		return values["_pragma"]
	}
	defaulted, err := sqliteConnectionDSN("nexus.db", true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(pragmasOf(defaulted), sqliteDefaultCacheSize) {
		t.Fatalf("default page cache missing: %s", defaulted)
	}
	explicit, err := sqliteConnectionDSN("nexus.db?_pragma=cache_size(-1000)", true)
	if err != nil {
		t.Fatal(err)
	}
	pragmas := pragmasOf(explicit)
	if slices.Contains(pragmas, sqliteDefaultCacheSize) || !slices.Contains(pragmas, "cache_size(-1000)") {
		t.Fatalf("explicit cache_size must win: %v", pragmas)
	}
}

func TestSQLDialectJSONSetsBindOneArray(t *testing.T) {
	postgres := NewSQLDialect("postgres")
	if got := postgres.JSONTextSet(3); got != "SELECT jsonb_array_elements_text($3::jsonb)" {
		t.Fatalf("postgres JSONTextSet = %q", got)
	}
	if got := postgres.JSONArrayRows(1, "tool"); got != "jsonb_array_elements($1::jsonb) AS tool(value)" {
		t.Fatalf("postgres JSONArrayRows = %q", got)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sqlite := NewSQLDialect("sqlite")
	var count int
	if err = db.QueryRow(
		"SELECT COUNT(*) FROM (SELECT 'a' AS id UNION ALL SELECT 'b' UNION ALL SELECT 'c') WHERE id IN ("+sqlite.JSONTextSet(1)+")",
		`["a","c","missing"]`,
	).Scan(&count); err != nil || count != 2 {
		t.Fatalf("sqlite JSONTextSet matched %d, err=%v", count, err)
	}
	var round, tool string
	if err = db.QueryRow(
		"SELECT pair.value ->> 0, pair.value ->> 1 FROM "+sqlite.JSONArrayRows(1, "pair"),
		`[["round","tool"]]`,
	).Scan(&round, &tool); err != nil || round != "round" || tool != "tool" {
		t.Fatalf("sqlite JSONArrayRows = %q/%q, err=%v", round, tool, err)
	}
}
