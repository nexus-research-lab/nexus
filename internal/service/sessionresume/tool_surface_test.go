package sessionresume

import "testing"

func TestRequiresK3ToolSurfaceReset(t *testing.T) {
	for _, test := range []struct {
		name     string
		provider string
		model    string
		baseURL  string
		stored   string
		current  string
		want     bool
	}{
		{name: "legacy K3 session", provider: "kimi-code", model: "k3", current: "new", want: true},
		{name: "changed K3 surface", provider: "kimi-code", model: "k3", stored: "old", current: "new", want: true},
		{name: "stable K3 surface", provider: "kimi-code", model: "k3", stored: "same", current: "same"},
		{name: "Moonshot K3 alias", provider: "moonshot", model: "kimi-k3", stored: "old", current: "new", want: true},
		{name: "custom Kimi endpoint", provider: "corp-coding", model: "k3", baseURL: "https://api.kimi.com/coding/", stored: "old", current: "new", want: true},
		{name: "other Kimi model", provider: "kimi-code", model: "k2.6", stored: "old", current: "new"},
		{name: "other provider", provider: "glm", model: "glm-5.1", stored: "old", current: "new"},
		{name: "unknown current surface", provider: "kimi-code", model: "k3", stored: "old"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := RequiresK3ToolSurfaceReset(test.provider, test.model, test.baseURL, test.stored, test.current); got != test.want {
				t.Fatalf("RequiresK3ToolSurfaceReset() = %v, want %v", got, test.want)
			}
		})
	}
}
