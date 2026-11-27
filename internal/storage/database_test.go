package storage

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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
