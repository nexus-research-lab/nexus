package protocol

import "testing"

func TestParseNexusManagedToolIdentity(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]any
		domain    string
		action    string
		operation string
	}{
		{name: "mcp__nexus__goal_read", domain: "goal", action: "inspect", operation: "get_goal"},
		{name: "nexus.goal_write", input: map[string]any{"operation": "update_goal"}, domain: "goal", action: "invoke", operation: "update_goal"},
		{name: "execution_read", input: map[string]any{"operation": "get_execution"}, domain: "execution", action: "inspect", operation: "get_execution"},
		{name: "nexus/execution_write", input: map[string]any{"operation": "assign_work"}, domain: "execution", action: "invoke", operation: "assign_work"},
		{name: "mcp__nexus__automation_apply", input: map[string]any{"operation": "create"}, domain: "automation", action: "apply", operation: "create"},
	}
	for _, test := range tests {
		identity, ok := ParseNexusManagedToolIdentity(test.name, test.input)
		if !ok || identity.Domain != test.domain || identity.Action != test.action || identity.Operation != test.operation {
			t.Fatalf("ParseNexusManagedToolIdentity(%q) = %+v, %t", test.name, identity, ok)
		}
	}
	for _, name := range []string{"mcp__nexus__command", "mcp__external__goal_read", "nexus.goal_write"} {
		if _, ok := ParseNexusManagedToolIdentity(name, nil); ok {
			t.Fatalf("incomplete or retired tool must not resolve: %q", name)
		}
	}
}

func TestParseLegacyNexusCommandToolIdentityIsReadOnlyCompatibility(t *testing.T) {
	identity, ok := ParseLegacyNexusCommandToolIdentity("mcp__nexus__command", map[string]any{
		"domain": "execution", "action": "invoke", "operation": "assign_work",
		"request_id": "request-1",
	})
	if !ok || identity.Operation != "assign_work" || identity.RequestID != "request-1" {
		t.Fatalf("legacy identity = %+v, %t", identity, ok)
	}
	if IsNexusManagedToolName("mcp__nexus__command") {
		t.Fatal("legacy command must not regain current authorization")
	}
}
