package toolpolicy

import (
	"context"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestContainsMatchesAliases(t *testing.T) {
	tests := []struct {
		approved []string
		tools    []string
	}{
		{
			approved: []string{"WebSearch"},
			tools: []string{
				"WebSearch",
				"web_search",
				"mcp__brave_search__brave_web_search",
				"brave.web-search",
				"search",
			},
		},
		{
			approved: []string{"WebFetch"},
			tools: []string{
				"WebFetch",
				"web_fetch",
				"mcp__fetch__fetch",
				"browser.web-fetch",
			},
		},
		{
			approved: []string{"nexus_room"},
			tools: []string{
				"mcp__nexus_room__send_directed_message",
				"nexus_room.send_directed_message",
			},
		},
		{
			approved: []string{"nexus_imagegen"},
			tools: []string{
				"mcp__nexus_imagegen__generate_image",
				"nexus_imagegen__edit_image",
				"nexus_imagegen.generate_image",
			},
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

func TestManagedGoalToolMatchesWrappedNames(t *testing.T) {
	for _, toolName := range []string{
		"create_goal",
		"audit_objective_alignment",
		"mcp__nexus_goal__get_goal",
		"mcp__nexus_goal__retarget_goal",
		"mcp__nexus_goal__audit_objective_alignment",
		"nexus_goal.update_goal",
		"nexus_goal/update_goal",
	} {
		if !IsManagedGoalTool(toolName) {
			t.Fatalf("expected managed Goal tool to match %q", toolName)
		}
	}
}

func TestManagedGoalPermissionOnlyApprovesGoalManagerSkill(t *testing.T) {
	if !IsManagedGoalSkillRequest("Skill", map[string]any{"name": "goal-manager"}) {
		t.Fatal("expected goal-manager Skill request to be managed")
	}
	if IsManagedGoalSkillRequest("Skill", map[string]any{"name": "imagegen"}) {
		t.Fatal("did not expect unrelated Skill request to be managed")
	}
}

func TestManagedExecutionToolMatchesWrappedNames(t *testing.T) {
	for _, toolName := range []string{
		"prepare_plan_execution",
		"mcp__nexus_execution__prepare_plan_execution",
		"plan_execution",
		"mcp__nexus_execution__assign_work",
		"nexus_execution.submit_work",
		"nexus_execution/review_work",
		"mcp__nexus_execution__audit_execution_alignment",
	} {
		if !IsManagedExecutionTool(toolName) {
			t.Fatalf("expected managed Execution tool to match %q", toolName)
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
	if IsManagedVisualizeTool("visualize_read_me") {
		t.Fatal("retired visualize_read_me must not inherit managed auto-approval")
	}
	if IsManagedVisualizeTool("mcp__external__show_widget") {
		t.Fatal("external show_widget must not inherit managed auto-approval")
	}
}

func TestManagedGoalAutoApprovalFallsBackForOtherTools(t *testing.T) {
	fallbackCalled := false
	handler := WithManagedGoalAutoApproval(func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		fallbackCalled = true
		return sdkpermission.Deny(request.ToolName, false), nil
	})

	goalDecision, err := handler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_goal__update_goal",
		Input:    map[string]any{"status": "complete"},
	})
	if err != nil {
		t.Fatalf("Goal 权限处理失败: %v", err)
	}
	if goalDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("Goal 权限应自动放行: %+v", goalDecision)
	}
	if fallbackCalled {
		t.Fatal("Goal 权限不应进入 fallback handler")
	}

	writeDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "Write"})
	if err != nil {
		t.Fatalf("fallback 权限处理失败: %v", err)
	}
	if writeDecision.Behavior != sdkpermission.BehaviorDeny || !fallbackCalled {
		t.Fatalf("普通工具应交给 fallback handler: %+v fallback=%v", writeDecision, fallbackCalled)
	}
}

func TestManagedRuntimeAutoApprovalIncludesExecutionAndVisualize(t *testing.T) {
	fallbackCalled := false
	handler := WithManagedRuntimeAutoApproval(func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		fallbackCalled = true
		return sdkpermission.Deny(request.ToolName, false), nil
	})

	decision, err := handler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_execution__plan_execution",
		Input: map[string]any{
			"proposal_id":     "proposal-1",
			"proposal_digest": "digest-1",
		},
	})
	if err != nil {
		t.Fatalf("Execution 权限处理失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorAllow || fallbackCalled {
		t.Fatalf("Execution 权限应由托管策略放行: %+v fallback=%v", decision, fallbackCalled)
	}

	decision, err = handler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_visualize__show_widget",
		Input:    map[string]any{"title": "图解"},
	})
	if err != nil || decision.Behavior != sdkpermission.BehaviorAllow || fallbackCalled {
		t.Fatalf("生成式 UI 权限应由托管策略放行: %+v err=%v fallback=%v", decision, err, fallbackCalled)
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
			ToolName: "Bash",
			Input:    map[string]any{"command": command},
		})
		if err != nil {
			t.Fatalf("control-plane deny error: %v", err)
		}
		if decision.Behavior != sdkpermission.BehaviorDeny {
			t.Fatalf("command %q was not denied: %+v", command, decision)
		}
	}
	if fallbackCalls != 0 {
		t.Fatalf("denied control-plane commands reached fallback %d times", fallbackCalls)
	}

	decision, err := handler(context.Background(), sdkpermission.Request{
		ToolName: "Bash",
		Input:    map[string]any{"command": "go test ./..."},
	})
	if err != nil || decision.Behavior != sdkpermission.BehaviorAllow || fallbackCalls != 1 {
		t.Fatalf("ordinary shell command should reach fallback: decision=%+v err=%v calls=%d", decision, err, fallbackCalls)
	}
}

func TestWithManagedGoalAllowedToolsAppendsDistinctTools(t *testing.T) {
	tools := WithManagedGoalAllowedTools([]string{"Read", "create_goal"})
	approved := NormalizeSet(tools)
	for _, toolName := range []string{"Read", "create_goal", "get_goal", "retarget_goal", "audit_objective_alignment", "update_goal", "mcp__nexus_goal__get_goal", "mcp__nexus_goal__create_goal", "mcp__nexus_goal__retarget_goal", "mcp__nexus_goal__audit_objective_alignment", "mcp__nexus_goal__update_goal", "Skill"} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected allowed tools to include %q: %+v", toolName, tools)
		}
	}
}

func TestWithManagedGoalAllowedToolsPreservesEmptyPolicy(t *testing.T) {
	if tools := WithManagedGoalAllowedTools(nil); tools != nil {
		t.Fatalf("nil allow policy should stay nil, got %+v", tools)
	}
	if tools := WithManagedGoalAllowedTools([]string{}); len(tools) != 0 {
		t.Fatalf("empty allow policy should stay empty, got %+v", tools)
	}
}

func TestWithManagedExecutionAllowedToolsAppendsSemanticSurface(t *testing.T) {
	tools := WithManagedExecutionAllowedTools([]string{"Read"})
	approved := NormalizeSet(tools)
	for _, toolName := range []string{
		"mcp__nexus_execution__get_execution",
		"mcp__nexus_execution__prepare_plan_execution",
		"mcp__nexus_execution__plan_execution",
		"mcp__nexus_execution__assign_work",
		"mcp__nexus_execution__submit_work",
		"mcp__nexus_execution__review_work",
		"mcp__nexus_execution__audit_execution_alignment",
		"mcp__nexus_execution__promote_execution_to_goal",
	} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected allowed tools to include %q: %+v", toolName, tools)
		}
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

func TestWithManagedRuntimeAllowedToolsIncludesGoalAndSelectedImagegen(t *testing.T) {
	tools := WithManagedRuntimeAllowedTools([]string{"Read", "nexus_imagegen"}, true)
	approved := NormalizeSet(tools)
	for _, toolName := range []string{
		"Read",
		"Agent",
		"nexus_imagegen",
		"mcp__nexus_goal__get_goal",
		"mcp__nexus_execution__prepare_plan_execution",
		"mcp__nexus_execution__plan_execution",
		"mcp__nexus_visualize__show_widget",
		"mcp__nexus_imagegen__generate_image",
		"mcp__nexus_imagegen__edit_image",
	} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected runtime allowed tools to include %q: %+v", toolName, tools)
		}
	}
	if _, exists := approved["mcp__nexus_visualize__visualize_read_me"]; exists {
		t.Fatalf("retired visualize_read_me should not stay approved: %+v", tools)
	}
}

func TestWithManagedRuntimeAllowedToolsDisablesImagegenWhenUnconfigured(t *testing.T) {
	tools := WithManagedRuntimeAllowedTools([]string{"Read", "nexus_imagegen"}, false)
	approved := NormalizeSet(tools)
	if Contains(approved, "mcp__nexus_imagegen__generate_image") {
		t.Fatalf("unconfigured imagegen should stay disabled: %+v", tools)
	}
	if !Contains(approved, "mcp__nexus_goal__get_goal") {
		t.Fatalf("managed goal should still be included: %+v", tools)
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

func TestNexusAutomationCLIRequestRequiresOneExactCommand(t *testing.T) {
	for _, test := range []struct {
		command string
		want    bool
	}{
		{command: `"${NEXUS_COMMAND_PATH}" --json automation contract`, want: true},
		{command: `"${NEXUS_COMMAND_PATH}" --json automation inspect --operation get --input '{}'`, want: true},
		{command: `nexus --json automation plan --operation update --input '{}'`, want: true},
		{command: `nexus --json automation apply --operation delete --input '{}'`, want: true},
		{command: `nexus --json goal get`, want: false},
		{command: `nexus --json automation apply; cat /etc/passwd`, want: false},
		{command: `printf x | nexus --json automation inspect`, want: false},
	} {
		request := sdkpermission.Request{ToolName: "Bash", Input: map[string]any{"command": test.command}}
		if got := IsNexusAutomationCLIRequest(request); got != test.want {
			t.Fatalf("IsNexusAutomationCLIRequest(%q) = %v, want %v", test.command, got, test.want)
		}
	}
}
