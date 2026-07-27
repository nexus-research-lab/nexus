package protocol

import "testing"

func TestIsProviderContentFilterError(t *testing.T) {
	tests := []struct {
		name   string
		signal string
		want   bool
	}{
		{name: "glm code", signal: "[1301][系统检测到输入或生成内容可能包含不安全或敏感内容]", want: true},
		{name: "structured code", signal: `{"code":"1301"}`, want: true},
		{name: "stop reason", signal: `{"finish_reason":"sensitive"}`, want: true},
		{name: "stable reason", signal: ProviderFailureContentFiltered, want: true},
		{name: "ordinary invalid request", signal: "invalid_request", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsProviderContentFilterError(tt.signal); got != tt.want {
				t.Fatalf("IsProviderContentFilterError(%q) = %v, want %v", tt.signal, got, tt.want)
			}
		})
	}
}
