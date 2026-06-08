package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestResultUsageLimitReachedDetectsExplicitUsageLimit(t *testing.T) {
	result := &sdkprotocol.ResultMessage{
		IsError:        true,
		TerminalReason: "error",
		Result:         "You've hit your usage limit. Try again later.",
		Additional: map[string]any{
			"error": map[string]any{
				"type": "usage_limit_reached",
			},
		},
	}

	ok, reason := ResultUsageLimitReached(result)
	if !ok {
		t.Fatal("ResultUsageLimitReached() ok = false, want true")
	}
	if reason != result.Result {
		t.Fatalf("reason = %q, want result text", reason)
	}
}

func TestResultUsageLimitReachedIgnoresTokenAndContextLimits(t *testing.T) {
	tests := []*sdkprotocol.ResultMessage{
		{TerminalReason: "max_output_tokens", Result: "Maximum output tokens reached."},
		{TerminalReason: "context_length", Result: "Context length exceeded."},
		{TerminalReason: "rate_limit", Result: "Provider rate limit, retry later."},
	}
	for _, result := range tests {
		if ok, reason := ResultUsageLimitReached(result); ok {
			t.Fatalf("ResultUsageLimitReached(%#v) = true reason %q, want false", result, reason)
		}
	}
}
