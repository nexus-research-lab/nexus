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

func TestContainsMatchesNexusRoomServerTools(t *testing.T) {
	approved := NormalizeSet([]string{"nexus_room"})

	for _, toolName := range []string{
		"mcp__nexus_room__send_directed_message",
		"nexus_room__publish_public_message",
		"nexus_room.send_directed_message",
	} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected nexus_room approval to match %q", toolName)
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
