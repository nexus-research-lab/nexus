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
