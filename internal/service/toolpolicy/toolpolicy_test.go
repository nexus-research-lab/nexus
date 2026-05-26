package toolpolicy

import "testing"

func TestContainsMatchesCommonWebSearchAliases(t *testing.T) {
	approved := NormalizeSet([]string{"WebSearch"})

	for _, toolName := range []string{
		"WebSearch",
		"web_search",
		"mcp__brave_search__brave_web_search",
		"brave.web-search",
		"search",
	} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected WebSearch approval to match %q", toolName)
		}
	}
}

func TestContainsMatchesCommonWebFetchAliases(t *testing.T) {
	approved := NormalizeSet([]string{"WebFetch"})

	for _, toolName := range []string{
		"WebFetch",
		"web_fetch",
		"mcp__fetch__fetch",
		"browser.web-fetch",
	} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected WebFetch approval to match %q", toolName)
		}
	}
}

func TestContainsDoesNotBroadenUnrelatedTools(t *testing.T) {
	approved := NormalizeSet([]string{"WebSearch"})

	for _, toolName := range []string{"Write", "mcp__filesystem__write_file", "Research"} {
		if Contains(approved, toolName) {
			t.Fatalf("did not expect WebSearch approval to match %q", toolName)
		}
	}
}
