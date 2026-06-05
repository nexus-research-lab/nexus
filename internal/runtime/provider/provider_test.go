package provider

import "testing"

func TestSupportsAPIFormat(t *testing.T) {
	tests := []struct {
		name        string
		runtimeKind string
		apiFormat   string
		want        bool
	}{
		{name: "claude anthropic", runtimeKind: RuntimeKindClaude, apiFormat: APIFormatAnthropicMessages, want: true},
		{name: "nxs anthropic", runtimeKind: RuntimeKindNXS, apiFormat: APIFormatAnthropicMessages, want: true},
		{name: "claude empty defaults anthropic", runtimeKind: RuntimeKindClaude, apiFormat: "", want: true},
		{name: "nxs chat completions", runtimeKind: RuntimeKindNXS, apiFormat: APIFormatChatCompletions, want: true},
		{name: "claude rejects chat completions", runtimeKind: RuntimeKindClaude, apiFormat: APIFormatChatCompletions, want: false},
		{name: "nxs rejects responses", runtimeKind: RuntimeKindNXS, apiFormat: "responses", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SupportsAPIFormat(tt.runtimeKind, tt.apiFormat)
			if got != tt.want {
				t.Fatalf("SupportsAPIFormat(%q, %q)=%t, want %t", tt.runtimeKind, tt.apiFormat, got, tt.want)
			}
		})
	}
}

func TestSupportsAnyRuntime(t *testing.T) {
	if !SupportsAnyRuntime(APIFormatAnthropicMessages) {
		t.Fatalf("Anthropic Messages 应至少被 Claude runtime 支持")
	}
	if !SupportsAnyRuntime(APIFormatChatCompletions) {
		t.Fatalf("Chat Completions 应至少被 nxs runtime 支持")
	}
	if SupportsAnyRuntime("responses") {
		t.Fatalf("Responses 当前不应被 Agent runtime 支持")
	}
}

func TestNormalizeRuntimeKind(t *testing.T) {
	tests := []struct {
		name        string
		runtimeKind string
		want        string
	}{
		{name: "empty defaults claude", runtimeKind: "", want: RuntimeKindClaude},
		{name: "nxs", runtimeKind: "nxs", want: RuntimeKindNXS},
		{name: "nxs uppercase", runtimeKind: "NXS", want: RuntimeKindNXS},
		{name: "go native alias", runtimeKind: "go-native", want: RuntimeKindNXS},
		{name: "claude code alias", runtimeKind: "claude-code", want: RuntimeKindClaude},
		{name: "unknown defaults claude", runtimeKind: "custom", want: RuntimeKindClaude},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRuntimeKind(tt.runtimeKind)
			if got != tt.want {
				t.Fatalf("NormalizeRuntimeKind(%q)=%q, want %q", tt.runtimeKind, got, tt.want)
			}
		})
	}
}
