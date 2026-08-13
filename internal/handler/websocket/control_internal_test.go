package websocket

import "testing"

func TestIsGoalHostCommandContentMatchesOnlyGoalCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "goal with objective", content: "/goal ship the release", want: true},
		{name: "case insensitive", content: "  /GOAL ship the release  ", want: true},
		{name: "empty goal command", content: "/goal", want: true},
		{name: "goal prefix is another command", content: "/goalkeeper report", want: false},
		{name: "another slash command", content: "/help", want: false},
		{name: "ordinary chat", content: "please update the goal", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := isGoalHostCommandContent(test.content); got != test.want {
				t.Fatalf("isGoalHostCommandContent(%q) = %v, want %v", test.content, got, test.want)
			}
		})
	}
}
