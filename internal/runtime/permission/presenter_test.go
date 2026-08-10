package permission

import (
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestResolveInteractionModeKeepsEveryToolActionable(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		want     string
	}{
		{
			name:     "structured question",
			toolName: "AskUserQuestion",
			want:     interactionModeQuestion,
		},
		{
			name:     "unknown future tool",
			toolName: "RequestHumanReview",
			want:     interactionModeApproval,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveInteractionMode(test.toolName); got != test.want {
				t.Fatalf(
					"resolveInteractionMode(%q) = %q, want %q",
					test.toolName,
					got,
					test.want,
				)
			}
		})
	}
}

func TestExecutionPermissionUsesSemanticRiskAndSummary(t *testing.T) {
	t.Parallel()

	risk, label := resolveRisk("mcp__nexus_execution__plan_execution")
	if risk != "medium" || label != "编排" {
		t.Fatalf("plan_execution risk = %q/%q, want medium/编排", risk, label)
	}
	risk, label = resolveRisk("mcp__nexus_execution__get_execution")
	if risk != "low" || label != "只读" {
		t.Fatalf("get_execution risk = %q/%q, want low/只读", risk, label)
	}
	risk, label = resolveRisk("mcp__nexus_execution__prepare_plan_execution")
	if risk != "low" || label != "封存提案" {
		t.Fatalf("prepare_plan_execution risk = %q/%q, want low/封存提案", risk, label)
	}
	summary := summarizeInput(
		"mcp__nexus_execution__plan_execution",
		map[string]any{"proposal_id": "proposal-1", "proposal_digest": "digest-1"},
	)
	if summary != "mcp__nexus_execution__plan_execution" {
		t.Fatalf("Execution summary = %q", summary)
	}
}

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		name string
		in   sdkpermission.Mode
		want sdkpermission.Mode
	}{
		{name: "empty", in: "", want: sdkpermission.ModeDefault},
		{name: "trimmed", in: " dontAsk ", want: sdkpermission.ModeDontAsk},
		{name: "unknown", in: "unsafe-mode", want: sdkpermission.ModeDefault},
		{name: "auto", in: sdkpermission.ModeAuto, want: sdkpermission.ModeAuto},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeMode(test.in); got != test.want {
				t.Fatalf("NormalizeMode(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}
