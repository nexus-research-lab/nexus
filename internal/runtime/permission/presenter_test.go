package permission

import "testing"

func TestResolveInteractionModeKeepsEveryToolActionable(t *testing.T) {
	t.Parallel()

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
			name:     "plan confirmation",
			toolName: "ExitPlanMode",
			want:     interactionModeApproval,
		},
		{
			name:     "unknown future tool",
			toolName: "RequestHumanReview",
			want:     interactionModeApproval,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
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
	summary := summarizeInput(
		"mcp__nexus_execution__plan_execution",
		map[string]any{"objective": "Ship the WorkGraph"},
	)
	if summary != "Ship the WorkGraph" {
		t.Fatalf("Execution summary = %q", summary)
	}
}
