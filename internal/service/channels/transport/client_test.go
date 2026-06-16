package transport

import (
	"net/http"
	"testing"
)

func TestWebsocketProxyFromEnvironmentMapsWebSocketSchemes(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://http-proxy.example:8080")
	t.Setenv("HTTPS_PROXY", "http://https-proxy.example:8443")
	t.Setenv("NO_PROXY", "internal.example")

	tests := []struct {
		name      string
		target    string
		wantProxy string
	}{
		{
			name:      "ws uses http proxy",
			target:    "ws://chat.example/socket",
			wantProxy: "http://http-proxy.example:8080",
		},
		{
			name:      "wss uses https proxy",
			target:    "wss://chat.example/socket",
			wantProxy: "http://https-proxy.example:8443",
		},
		{
			name:      "no proxy still bypasses websocket target",
			target:    "wss://internal.example/socket",
			wantProxy: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.target, nil)
			if err != nil {
				t.Fatal(err)
			}

			proxyURL, err := websocketProxyFromEnvironment(req)
			if err != nil {
				t.Fatal(err)
			}
			if proxyURL == nil {
				if tt.wantProxy != "" {
					t.Fatalf("proxy = nil, want %s", tt.wantProxy)
				}
				return
			}
			if got := proxyURL.String(); got != tt.wantProxy {
				t.Fatalf("proxy = %s, want %s", got, tt.wantProxy)
			}
		})
	}
}
