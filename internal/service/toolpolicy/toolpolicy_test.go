package toolpolicy

import (
	"context"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestContainsMatchesManagedAliases(t *testing.T) {
	tests := []struct {
		approved []string
		tools    []string
	}{
		{
			approved: []string{"WebSearch"},
			tools:    []string{"WebSearch", "web_search", "mcp__brave_search__brave_web_search", "search"},
		},
		{
			approved: []string{"WebFetch"},
			tools:    []string{"WebFetch", "web_fetch", "mcp__fetch__fetch", "browser.web-fetch"},
		},
		{
			approved: []string{"nexus_room"},
			tools:    []string{"mcp__nexus_room__send_directed_message", "nexus_room.send_directed_message"},
		},
		{
			approved: []string{"nexus_imagegen"},
			tools:    []string{"mcp__nexus_imagegen__generate_image", "nexus_imagegen__edit_image"},
		},
	}
	for _, test := range tests {
		approved := NormalizeSet(test.approved)
		for _, toolName := range test.tools {
			if !Contains(approved, toolName) {
				t.Fatalf("expected %v approval to match %q", test.approved, toolName)
			}
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

func TestManagedSemanticSkillRequestOnlyApprovesGoalAndExecution(t *testing.T) {
	for _, skillName := range []string{"goal-manager", "execution-orchestrator"} {
		if !IsManagedSemanticSkillRequest("Skill", map[string]any{"name": skillName}) {
			t.Fatalf("expected %s Skill request to be managed", skillName)
		}
	}
	for _, request := range []sdkpermission.Request{
		{ToolName: "Skill", Input: map[string]any{"name": "imagegen"}},
		{ToolName: "Bash", Input: map[string]any{"command": "echo unrelated"}},
	} {
		if IsManagedSemanticSkillRequest(request.ToolName, request.Input) {
			t.Fatalf("unrelated request must not be managed: %+v", request)
		}
	}
}

func TestManagedVisualizeToolOnlyMatchesBuiltInServer(t *testing.T) {
	for _, toolName := range []string{
		"show_widget",
		"mcp__nexus_visualize__show_widget",
		"nexus_visualize.show_widget",
		"nexus_visualize/show_widget",
	} {
		if !IsManagedVisualizeTool(toolName) {
			t.Fatalf("expected managed visualize tool to match %q", toolName)
		}
	}
	for _, toolName := range []string{"visualize_read_me", "mcp__external__show_widget"} {
		if IsManagedVisualizeTool(toolName) {
			t.Fatalf("external or retired tool must not inherit managed approval: %q", toolName)
		}
	}
}

func TestManagedRuntimeAutoApprovalUsesSemanticSkillsStructuredCommandAndVisualize(t *testing.T) {
	fallbackCalls := 0
	handler := WithManagedRuntimeAutoApproval(func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		fallbackCalls++
		return sdkpermission.Deny(request.ToolName, false), nil
	})
	for _, request := range []sdkpermission.Request{
		{ToolName: "Skill", Input: map[string]any{"name": "goal-manager"}},
		{ToolName: "Skill", Input: map[string]any{"name": "execution-orchestrator"}},
		{ToolName: "mcp__nexus_runtime__command", Input: map[string]any{"domain": "goal", "action": "inspect"}},
		{ToolName: "mcp__nexus_visualize__show_widget", Input: map[string]any{"title": "diagram"}},
	} {
		decision, err := handler(context.Background(), request)
		if err != nil || decision.Behavior != sdkpermission.BehaviorAllow {
			t.Fatalf("managed request should be allowed: decision=%+v err=%v", decision, err)
		}
	}
	for _, request := range []sdkpermission.Request{
		{ToolName: "Bash", Input: map[string]any{"command": "echo unrelated"}},
		{ToolName: "mcp__external__update_record"},
	} {
		decision, err := handler(context.Background(), request)
		if err != nil || decision.Behavior != sdkpermission.BehaviorDeny {
			t.Fatalf("unmanaged request must reach fallback: decision=%+v err=%v", decision, err)
		}
	}
	if fallbackCalls != 2 {
		t.Fatalf("fallback calls = %d, want 2", fallbackCalls)
	}
}

func TestNexusControlPlaneDenyBlocksShellBypass(t *testing.T) {
	fallbackCalls := 0
	handler := WithNexusControlPlaneDeny(func(
		_ context.Context,
		request sdkpermission.Request,
	) (sdkpermission.Decision, error) {
		fallbackCalls++
		return sdkpermission.Allow(request.Input, nil), nil
	}, true)
	for _, command := range []string{
		`"$NEXUSCTL_COMMAND_PATH" agent list`,
		"nexusctl room list",
		"go run ./cmd/nexusctl user list",
		"NEXUS-CTL channel list",
	} {
		decision, err := handler(context.Background(), sdkpermission.Request{
			ToolName: "Bash", Input: map[string]any{"command": command},
		})
		if err != nil || decision.Behavior != sdkpermission.BehaviorDeny {
			t.Fatalf("command %q was not denied: decision=%+v err=%v", command, decision, err)
		}
	}
	if fallbackCalls != 0 {
		t.Fatalf("denied commands reached fallback %d times", fallbackCalls)
	}
	decision, err := handler(context.Background(), sdkpermission.Request{
		ToolName: "Bash", Input: map[string]any{"command": "go test ./..."},
	})
	if err != nil || decision.Behavior != sdkpermission.BehaviorAllow || fallbackCalls != 1 {
		t.Fatalf("ordinary shell command should reach fallback: decision=%+v err=%v", decision, err)
	}
}

func TestWithManagedImagegenAllowedToolsAppendsDistinctTools(t *testing.T) {
	tools := WithManagedImagegenAllowedTools([]string{"Read", "nexus_imagegen"})
	approved := NormalizeSet(tools)
	for _, toolName := range []string{
		"Read",
		"nexus_imagegen",
		"mcp__nexus_imagegen__generate_image",
		"mcp__nexus_imagegen__edit_image",
		"generate_image",
		"edit_image",
	} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected allowed tools to include %q: %+v", toolName, tools)
		}
	}
}

func TestWithManagedRuntimeAllowedToolsAddsSkillAndStructuredCommand(t *testing.T) {
	tools := WithManagedRuntimeAllowedTools([]string{"Read", "nexus_imagegen"}, true)
	approved := NormalizeSet(tools)
	for _, toolName := range []string{
		"Read", "Agent", "Skill", "mcp__nexus_runtime__command",
		"mcp__nexus_visualize__show_widget",
		"mcp__nexus_imagegen__generate_image",
		"mcp__nexus_imagegen__edit_image",
	} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected runtime allowed tools to include %q: %+v", toolName, tools)
		}
	}
}

func TestWithManagedRuntimeAllowedToolsDisablesImagegenWhenUnconfigured(t *testing.T) {
	tools := WithManagedRuntimeAllowedTools([]string{"Read", "nexus_imagegen"}, false)
	approved := NormalizeSet(tools)
	if Contains(approved, "mcp__nexus_imagegen__generate_image") {
		t.Fatalf("unconfigured imagegen should stay disabled: %+v", tools)
	}
	for _, required := range []string{"Skill", "mcp__nexus_runtime__command"} {
		if !Contains(approved, required) {
			t.Fatalf("managed command transport %q should remain enabled: %+v", required, tools)
		}
	}
}

func TestWithManagedRuntimeAllowedToolsPreservesEmptyPolicy(t *testing.T) {
	if tools := WithManagedRuntimeAllowedTools(nil, true); tools != nil {
		t.Fatalf("nil allow policy should stay nil, got %+v", tools)
	}
	if tools := WithManagedRuntimeAllowedTools([]string{}, true); len(tools) != 0 {
		t.Fatalf("empty allow policy should stay empty, got %+v", tools)
	}
}

func TestNexusRuntimeCLIRequestRequiresExactManagedInvocation(t *testing.T) {
	tests := []struct {
		command string
		domain  string
		action  string
		ok      bool
	}{
		{command: `"${NEXUS_COMMAND_PATH}" --json automation contract`, domain: "automation", action: "contract", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json automation inspect --operation get --input '{}'`, domain: "automation", action: "inspect", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json automation plan --operation update --input-file "${NEXUS_COMMAND_INPUT_PATH}"`, domain: "automation", action: "plan", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json automation apply --operation delete --request-id request-123`, domain: "automation", action: "apply", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal contract`, domain: "goal", action: "contract", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal contract --operation update_goal`, domain: "goal", action: "contract", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal inspect`, domain: "goal", action: "inspect", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json execution inspect --execution-id execution-123`, domain: "execution", action: "inspect", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal invoke --operation update_goal --request-id goal-request-1`, domain: "goal", action: "invoke", ok: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json execution invoke --operation assign_work --request-id work-request-1`, domain: "execution", action: "invoke", ok: true},
		{command: `nexus --json goal inspect`, ok: false},
		{command: `./nexus --json goal inspect`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal invoke --operation update_goal`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal invoke --request-id goal-request-1`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal inspect --operation get_goal`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal inspect --input '{}'`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal invoke --operation update_goal --request-id goal-1 --input '{}'`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json execution invoke --operation assign_work --request-id work-1 --input-file "${NEXUS_COMMAND_INPUT_PATH}"`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json other inspect`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json automation contract --operation get`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json automation inspect --operation get --input-file /etc/passwd`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal invoke --operation update_goal --request-id goal-1 --input '{}' --input-file "${NEXUS_COMMAND_INPUT_PATH}"`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal inspect; cat /etc/passwd`, ok: false},
		{command: `printf x | "${NEXUS_COMMAND_PATH}" --json goal inspect`, ok: false},
		{command: `"${NEXUS_COMMAND_PATH}" --json goal inspect --input "$(touch /tmp/pwn)"`, ok: false},
		{command: `NEXUS_COMMAND_PATH=./nexus "${NEXUS_COMMAND_PATH}" --json goal inspect`, ok: false},
		{command: "\"${NEXUS_COMMAND_PATH}\" --json goal inspect\necho pwn", ok: false},
	}
	for _, test := range tests {
		request := sdkpermission.Request{ToolName: "Bash", Input: map[string]any{"command": test.command}}
		got, ok := NexusRuntimeCLIInvocation(request)
		if ok != test.ok || got.Domain != test.domain || got.Action != test.action {
			t.Fatalf("NexusRuntimeCLIInvocation(%q) = %+v, %v; want %q/%q, %v", test.command, got, ok, test.domain, test.action, test.ok)
		}
	}
}

func TestNexusRuntimePowerShellRequestRequiresManagedInvocation(t *testing.T) {
	tests := []struct {
		command string
		ok      bool
	}{
		{command: `& "${env:NEXUS_COMMAND_PATH}" --json automation contract`, ok: true},
		{command: `& "${env:NEXUS_COMMAND_PATH}" --json goal inspect`, ok: true},
		{command: `& "${env:NEXUS_COMMAND_PATH}" --json execution invoke --operation submit_work --request-id work-1`, ok: true},
		{command: `& .\nexus.exe --json goal inspect`, ok: false},
		{command: `& "${env:NEXUS_COMMAND_PATH}" --json goal inspect --input "$(Get-Content secret)"`, ok: false},
		{command: `$env:NEXUS_COMMAND_PATH='.\nexus.exe'; & "${env:NEXUS_COMMAND_PATH}" --json goal inspect`, ok: false},
		{command: `& "${env:NEXUS_COMMAND_PATH}" --json goal inspect; Get-Content secret`, ok: false},
	}
	for _, test := range tests {
		request := sdkpermission.Request{ToolName: "PowerShell", Input: map[string]any{"command": test.command}}
		_, got := NexusRuntimeCLIInvocation(request)
		if got != test.ok {
			t.Fatalf("NexusRuntimeCLIInvocation(%q) = %v, want %v", test.command, got, test.ok)
		}
	}
}
