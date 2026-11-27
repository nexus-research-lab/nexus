package browser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	browsersvc "github.com/nexus-research-lab/nexus/internal/service/browser"
)

func TestTrustedRequestRequiresNexusExtensionAndSubprotocol(t *testing.T) {
	valid := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/internal/browser/ws", nil)
	valid.RemoteAddr = "127.0.0.1:54321"
	valid.Header.Set("Origin", browsersvc.BrowserExtensionOrigin)
	valid.Header.Set("Sec-WebSocket-Protocol", browsersvc.WebSocketSubprotocol)
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
				t.Fatal("不可信 Browser 请求被接受")
			}
		})
	}
}

func TestWriteLockWaitHonorsCancellationWithoutLateSend(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	send := boundedSender(func(context.Context, any) error {
		entered <- struct{}{}
		<-release
		return nil
	})
	first := make(chan error, 1)
	go func() { first <- send(context.Background(), "first") }()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := send(ctx, "cancelled"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
		t.Fatal("cancelled write was sent")
	default:
	}
}
