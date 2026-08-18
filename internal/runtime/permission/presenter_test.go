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
