package toolpolicy

import (
	"context"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

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
	if IsManagedGoalSkillRequest("Skill", map[string]any{"name": "memory-manager"}) {
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
