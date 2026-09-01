package shared

import (
	"net/http/httptest"
	"testing"
)

func TestStrongETagRoundTrip(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteStrongETag(recorder, "resource-", 7)
	if got := recorder.Header().Get("ETag"); got != `"resource-7"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	version, err := ParseStrongIfMatch(
		recorder.Header().Get("ETag"),
		"resource-",
		"Resource",
		"资源",
	)
	if err != nil || version == nil || *version != 7 {
		t.Fatalf("ParseStrongIfMatch() = %v, %v", version, err)
	}
}

func TestParseStrongIfMatchRejectsAmbiguousValues(t *testing.T) {
	for _, value := range []string{
		`W/"resource-7"`,
		`"resource-7", "resource-8"`,
		`"other-7"`,
		`"resource-0"`,
		`resource-7`,
	} {
		if _, err := ParseStrongIfMatch(value, "resource-", "Resource", "资源"); err == nil {
			t.Fatalf("ParseStrongIfMatch(%q) unexpectedly succeeded", value)
		}
	}
}
