package runtime

import "testing"

// Explicit stops must not reach the provider as a submit interrupt, which
// providers treat as "new user input arrived" rather than a stop.
func TestClientInterruptWireReasonKeepsExplicitStopsSilent(t *testing.T) {
	for _, reason := range []string{"", clientInterruptWithoutMessage, clientInterruptHostControl} {
		if got := clientInterruptWireReason(reason); got != "" {
			t.Fatalf("wire reason for %q = %q, want empty", reason, got)
		}
	}
	if got := clientInterruptWireReason("收到新的用户消息，上一轮已停止"); got != clientInterruptReasonSubmit {
		t.Fatalf("submit interrupt wire reason = %q", got)
	}
}
