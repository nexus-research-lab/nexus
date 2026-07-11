package tool

import (
	"testing"
	"time"
)

func TestParseExpiresAtNormalizesRFC3339ToUTC(t *testing.T) {
	expiresAt, err := parseExpiresAt(map[string]any{"expires_at": "2026-07-11T18:30:00+08:00"})
	if err != nil {
		t.Fatalf("parseExpiresAt 失败: %v", err)
	}
	want := time.Date(2026, 7, 11, 10, 30, 0, 0, time.UTC)
	if expiresAt == nil || !expiresAt.Equal(want) {
		t.Fatalf("expires_at = %v, want %s", expiresAt, want)
	}
}

func TestParseExpiresAtRejectsEmptyAndInvalidValues(t *testing.T) {
	for _, value := range []any{"", "tomorrow"} {
		if _, err := parseExpiresAt(map[string]any{"expires_at": value}); err == nil {
			t.Fatalf("parseExpiresAt(%q) 应失败", value)
		}
	}
}
