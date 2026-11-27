package storage

import (
	"net/url"
	"slices"
	"strings"
	"testing"
)

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
