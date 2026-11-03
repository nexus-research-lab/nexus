package message

import "testing"

func TestInterruptReasonClassification(t *testing.T) {
	for _, tc := range []struct {
		reason   string
		display  string
		explicit bool
	}{
		{InterruptWithoutMessage, "", true},
		{InterruptHostControlWithoutMessage, "", true},
		{"收到新的用户消息，上一轮已停止", "收到新的用户消息，上一轮已停止", false},
		{"host_shutdown", "host_shutdown", false},
		{"", "", false},
	} {
		if got := NormalizeInterruptDisplayText(tc.reason); got != tc.display {
			t.Fatalf("display(%q) = %q, want %q", tc.reason, got, tc.display)
		}
		if got := IsExplicitStopInterrupt(tc.reason); got != tc.explicit {
			t.Fatalf("explicit(%q) = %v, want %v", tc.reason, got, tc.explicit)
		}
	}
}
