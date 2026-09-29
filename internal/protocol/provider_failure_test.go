package protocol

import "testing"

func TestIsProviderTokenLimitError(t *testing.T) {
	for _, signal := range []string{
		"context_length_exceeded",
		"Maximum context length is 128000 tokens",
		"request_too_large",
		"You've hit your usage limit",
		"out of tokens",
		"insufficient_quota",
		"上下文长度超过上限",
	} {
		if !IsProviderTokenLimitError(signal) {
			t.Fatalf("IsProviderTokenLimitError(%q) = false, want true", signal)
		}
	}
}

func TestIsProviderTokenLimitErrorDoesNotClassifyAuthenticationToken(t *testing.T) {
	if IsProviderTokenLimitError("invalid API token") {
		t.Fatal("an authentication token error must not be treated as a token limit")
	}
}
