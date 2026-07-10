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
				"nexus_room__publish_public_message",
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
		"mcp__nexus_goal__get_goal",
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

func TestWithManagedGoalAllowedToolsAppendsDistinctTools(t *testing.T) {
	tools := WithManagedGoalAllowedTools([]string{"Read", "create_goal"})
	approved := NormalizeSet(tools)
	for _, toolName := range []string{"Read", "create_goal", "get_goal", "update_goal", "mcp__nexus_goal__get_goal", "mcp__nexus_goal__create_goal", "mcp__nexus_goal__update_goal", "Skill"} {
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
		"mcp__nexus_imagegen__generate_image",
		"mcp__nexus_imagegen__edit_image",
	} {
		if !Contains(approved, toolName) {
			t.Fatalf("expected runtime allowed tools to include %q: %+v", toolName, tools)
		}
	}
}

func TestWithManagedRuntimeAllowedToolsKeepsMainThreadAgent(t *testing.T) {
	tools := WithManagedRuntimeAllowedTools([]string{"Read"}, false)
	approved := NormalizeSet(tools)
	if !Contains(approved, "Agent") {
		t.Fatalf("显式白名单应保留主线程 Agent 工具: %+v", tools)
	}
}

func TestWithManagedRuntimeAllowedToolsEnablesDefaultImagegen(t *testing.T) {
	tools := WithManagedRuntimeAllowedTools([]string{"Read"}, true)
	approved := NormalizeSet(tools)
	if !Contains(approved, "mcp__nexus_imagegen__generate_image") {
		t.Fatalf("configured imagegen should be enabled by default: %+v", tools)
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
