package webbridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	webbridgesvc "github.com/nexus-research-lab/nexus/internal/service/webbridge"
)

func TestHandleStatusReturnsDisconnectedState(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), webbridgesvc.NewService())
	response := httptest.NewRecorder()
	handler.HandleStatus(
		response,
		httptest.NewRequest(http.MethodGet, "/internal/webbridge/status", nil),
	)

	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if response.Code != http.StatusOK || payload.Data["connected"] != false {
		t.Fatalf("status = %d, payload = %+v", response.Code, payload.Data)
	}
}

func TestTrustedRequestRequiresNexusExtensionAndSubprotocol(t *testing.T) {
	valid := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/internal/webbridge/ws", nil)
	valid.RemoteAddr = "127.0.0.1:54321"
	valid.Header.Set("Origin", webbridgesvc.BrowserExtensionOrigin)
	valid.Header.Set("Sec-WebSocket-Protocol", webbridgesvc.WebSocketSubprotocol)
	if !trustedRequest(valid) {
		t.Fatal("合法 Nexus 扩展请求被拒绝")
	}
	remote := valid.Clone(valid.Context())
	remote.Header = valid.Header.Clone()
	remote.RemoteAddr = "192.0.2.1:54321"
	if !trustedRequest(remote) {
		t.Fatal("远端 Nexus 扩展请求被拒绝")
	}

	for name, mutate := range map[string]func(*http.Request){
		"origin": func(request *http.Request) { request.Header.Set("Origin", "https://example.com") },
		"extension": func(request *http.Request) {
			request.Header.Set("Origin", "chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		},
		"subprotocol": func(request *http.Request) { request.Header.Del("Sec-WebSocket-Protocol") },
	} {
		t.Run(name, func(t *testing.T) {
			request := valid.Clone(valid.Context())
			request.Header = valid.Header.Clone()
			mutate(request)
			if trustedRequest(request) {
				t.Fatal("不可信 WebBridge 请求被接受")
			}
		})
	}
}
