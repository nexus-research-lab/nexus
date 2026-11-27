// shared_test.go 覆盖公共 Handler API、失败分类、中间件与 WebSocket 发送器测试。
package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func TestGatewayClientErrorDetailPreservesDefaultModelHint(t *testing.T) {
	detail := "默认模型仍使用 Provider kimi-code；请先在设置中切换默认模型"
	got := GatewayClientErrorDetail(http.StatusBadRequest, detail)
	if got != detail {
		t.Fatalf("BadRequest 下的默认模型保护提示应直接返回给客户端，got=%q want=%q", got, detail)
	}
}

func TestPublicAuthRouteAllowsTeamCapabilitiesOnly(t *testing.T) {
	for path, want := range map[string]bool{
		"/nexus/v1/team/capabilities": true,
		"/nexus/v1/team/rooms":        false,
		"/nexus/v1/team/invitations":  false,
	} {
		if got := PublicAuthRoute(httptest.NewRequest(http.MethodGet, path, nil)); got != want {
			t.Fatalf("PublicAuthRoute(%s)=%v want=%v", path, got, want)
		}
	}
}

func TestWriteFailureKeepsLegacyCancellationAndSanitization(t *testing.T) {
	api := newFailureTestAPI()

	canceled := httptest.NewRecorder()
	api.WriteFailure(canceled, http.StatusInternalServerError, "context canceled")
	if canceled.Code != 499 || !strings.Contains(canceled.Body.String(), "请求已取消") {
		t.Fatalf("旧取消请求投影被改变: status=%d body=%s", canceled.Code, canceled.Body.String())
	}

	internal := httptest.NewRecorder()
	api.WriteFailure(internal, http.StatusInternalServerError, "sqlite secret")
	if strings.Contains(internal.Body.String(), "sqlite secret") ||
		!strings.Contains(internal.Body.String(), "服务内部错误") {
		t.Fatalf("旧内部错误脱敏被改变: %s", internal.Body.String())
	}

	legacyGatewayTimeout := httptest.NewRecorder()
	api.WriteFailure(legacyGatewayTimeout, http.StatusGatewayTimeout, "upstream timeout")
	if !strings.Contains(legacyGatewayTimeout.Body.String(), "服务内部错误") {
		t.Fatalf("旧 WriteFailure 的 504 文案不应被新协议改写: %s", legacyGatewayTimeout.Body.String())
	}
}

func TestWriteErrorDoesNotExposeCauseOrInventRequestID(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodPost, "/workgraphs/one", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusInternalServerError, FailureSpec{
		Cause: errors.New("sqlite secret path"),
	})

	body := recorder.Body.String()
	if strings.Contains(body, "sqlite secret path") {
		t.Fatalf("内部 Cause 泄露到响应: %s", body)
	}
	if strings.Contains(body, "transport_request_id") || strings.Contains(body, `"request_id"`) {
		t.Fatalf("缺少 middleware 时不应另造 request ID: %s", body)
	}
	if !strings.Contains(body, `"code":"common.request_failed"`) ||
		!strings.Contains(body, `"effect":"unknown"`) {
		t.Fatalf("空结构化事实没有安全回退: %s", body)
	}
}

func TestNormalizeFailureSemanticKeyRequiresDomainReasonShape(t *testing.T) {
	tests := []struct {
		value string
		want  string
		valid bool
	}{
		{value: " workgraph.refresh_editor ", want: "workgraph.refresh_editor", valid: true},
		{value: "automation.delivery_retry_2", want: "automation.delivery_retry_2", valid: true},
		{value: "", want: "", valid: true},
		{value: "request_failed", valid: false},
		{value: "WorkGraph.refresh", valid: false},
		{value: "workgraph..refresh", valid: false},
		{value: "workgraph.-refresh", valid: false},
		{value: "workgraph.refresh/editor", valid: false},
		{value: strings.Repeat("a", maxFailureSemanticKeyLength+1) + ".reason", valid: false},
	}
	for _, test := range tests {
		got, valid := normalizeFailureSemanticKey(test.value)
		if got != test.want || valid != test.valid {
			t.Fatalf("normalizeFailureSemanticKey(%q)=(%q,%t) want=(%q,%t)", test.value, got, valid, test.want, test.valid)
		}
	}
}

func TestWriteErrorDegradesUnknownCategoryFromHTTPStatus(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodPost, "/scheduled-tasks", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusConflict, FailureSpec{
		Code:     "automation.task_update_failed",
		Category: protocol.FailureCategory("provider_secret_category"),
		Effect:   protocol.FailureEffectNotApplied,
	})

	body := recorder.Body.String()
	if !strings.Contains(body, `"category":"conflict"`) || strings.Contains(body, "provider_secret_category") {
		t.Fatalf("未知 category 没有按 HTTP 语义安全降级: %s", body)
	}
}

func TestWriteErrorKeepsCancellationStatusAndCategoryConsistent(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodGet, "/runs", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusInternalServerError, FailureSpec{
		Code:     "automation.run_history_unavailable",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Cause:    context.Canceled,
	})

	if recorder.Code != 499 ||
		!strings.Contains(recorder.Body.String(), `"category":"canceled"`) ||
		!strings.Contains(recorder.Body.String(), `"effect":"not_applicable"`) {
		t.Fatalf("取消投影不一致: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestFailureEffectBeforeHandlerUsesOnlyRequestSemantics(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		request := httptest.NewRequest(method, "/resource", nil)
		if got := failureEffectBeforeHandler(request); got != protocol.FailureEffectNotApplicable {
			t.Fatalf("%s pre-handler effect=%q", method, got)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodDelete} {
		request := httptest.NewRequest(method, "/resource", nil)
		if got := failureEffectBeforeHandler(request); got != protocol.FailureEffectNotApplied {
			t.Fatalf("%s pre-handler effect=%q", method, got)
		}
	}
}

func TestDiagnosticRequestIDRejectsUnsafeOrOversizedValuesWithoutRejectingRequest(t *testing.T) {
	for _, raw := range []string{
		"request id with spaces",
		"request/id",
		strings.Repeat("a", maxDiagnosticRequestIDLength+1),
	} {
		if got := normalizeDiagnosticRequestID(raw); got != "" {
			t.Fatalf("不安全 request ID %q 被接受为 %q", raw, got)
		}
	}
	if got := normalizeDiagnosticRequestID(" trace_01:attempt-2 "); got != "trace_01:attempt-2" {
		t.Fatalf("合法 request ID 被改变: %q", got)
	}

	api := newFailureTestAPI()
	called := false
	handler := RequestContextMiddleware(api.BaseLogger())(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			called = true
			writer.WriteHeader(http.StatusNoContent)
		},
	))
	request := httptest.NewRequest(http.MethodPost, "/scheduled-tasks", nil)
	request.Header.Set("X-Request-ID", strings.Repeat("a", maxDiagnosticRequestIDLength+1))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("无效诊断 ID 不得拒绝请求: called=%v status=%d", called, recorder.Code)
	}
	generated := recorder.Header().Get("X-Request-ID")
	if generated == "" || generated == request.Header.Get("X-Request-ID") {
		t.Fatalf("无效诊断 ID 应静默替换: %q", generated)
	}
}

func newFailureTestAPI() *API {
	return NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type webAccessAuthority struct {
	authsvc.Authority
	local    bool
	disabled bool
}

func (a webAccessAuthority) InspectRequest(context.Context, *http.Request) (*authsvc.Principal, authsvc.State, error) {
	return &authsvc.Principal{UserID: "user", WebAccessDisabled: a.disabled}, authsvc.State{AuthRequired: !a.local}, nil
}

func TestWebAccessDoesNotGrantServerResourcesOrBlockDesktopRelay(t *testing.T) {
	for _, path := range []string{"/nexus/v1/agents", "/nexus/v1/rooms", "/nexus/v1/ws", "/nexus/v1/team-node/room", "/nexus/v1/team/rooms", "/nexus/v1/team/rooms/r/events", "/nexus/v1/auth/status"} {
		for _, local := range []bool{false, true} {
			for _, disabled := range []bool{false, true} {
				handler := AuthMiddleware(NewAPI(nil), webAccessAuthority{local: local, disabled: disabled})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
				r := httptest.NewRequest(http.MethodGet, path, nil)
				r.Header.Set("User-Agent", "Nexus Desktop")
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				want := http.StatusNoContent
				if disabled && !local && !strings.HasPrefix(path, "/nexus/v1/team/") && path != "/nexus/v1/auth/status" {
					want = http.StatusForbidden
				}
				if w.Code != want {
					t.Fatalf("path=%s local=%v disabled=%v: status=%d want=%d", path, local, disabled, w.Code, want)
				}
			}
		}
	}
}

func TestAccessLogMiddlewareRedactsSensitiveQueryValues(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := RequestContextMiddleware(logger)(
		AccessLogMiddleware()(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/agents?access_token=super-secret&token=another-secret&api_key=third-secret&limit=10",
		nil,
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	output := buffer.String()
	for _, secret := range []string{"super-secret", "another-secret", "third-secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("access log 不应泄露敏感 query 参数: %s", output)
		}
	}
	for _, key := range []string{"access_token", "token", "api_key"} {
		if !strings.Contains(output, key+"=%5Bredacted%5D") {
			t.Fatalf("access log 未脱敏 %s: %s", key, output)
		}
	}
	if !strings.Contains(output, "limit=10") {
		t.Fatalf("access log 不应移除非敏感 query 参数: %s", output)
	}
}

func TestRecoverMiddlewareReturnsInternalError(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	api := NewAPI(logger)

	handler := RequestContextMiddleware(logger)(
		AccessLogMiddleware()(
			RecoverMiddleware(api)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				panic("boom")
			})),
		),
	)

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/panic", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("panic 后状态码不正确: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "服务内部错误") {
		t.Fatalf("panic 后返回体不正确: %s", recorder.Body.String())
	}

	output := buffer.String()
	if !strings.Contains(output, "\"msg\":\"HTTP 请求 panic\"") {
		t.Fatalf("未记录 panic 日志: %s", output)
	}
}

func TestDesktopSessionTokenMiddlewareAllowsHealthAndStatic(t *testing.T) {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := DesktopSessionTokenMiddleware(api, "desktop-token", "/nexus/v1")(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("ok"))
		}),
	)

	for _, path := range []string{"/nexus/v1/health", "/nexus/v1/system/version", "/", "/assets/index.js", "/nexus/v1/internal/actions"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s 应绕过桌面 token，实际状态码: %d", path, recorder.Code)
		}
	}
}

func TestDesktopSessionTokenMiddlewareAllowsOAuthCallbackPost(t *testing.T) {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := DesktopSessionTokenMiddleware(api, "desktop-token", "/nexus/v1")(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("ok"))
		}),
	)

	request := httptest.NewRequest(http.MethodPost, "/nexus/v1/connectors/oauth/callback", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("OAuth callback POST 应绕过桌面 token，实际状态码: %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/nexus/v1/connectors/oauth/callback", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("OAuth callback GET 不应绕过桌面 token，实际状态码: %d", recorder.Code)
	}
}

func TestPublicAuthRouteAllowsOAuthCallbackPost(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/nexus/v1/connectors/oauth/callback", nil)
	if !PublicAuthRoute(request) {
		t.Fatal("OAuth callback POST 应为公开路由")
	}

	request = httptest.NewRequest(http.MethodGet, "/nexus/v1/connectors/oauth/callback", nil)
	if PublicAuthRoute(request) {
		t.Fatal("OAuth callback GET 不应为公开路由")
	}
}

func TestPublicAuthRouteAllowsRuntimeConfigurationBrokerWithCustomPrefix(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/custom/internal/runtime/configuration", nil)
	if !PublicAuthRoute(request) {
		t.Fatal("nexuscfg runtime broker 应自行通过 capability 鉴权")
	}
}

func TestValidateDesktopSessionTokenReportsSource(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*http.Request)
		wantValid  bool
		wantSource string
		wantReason string
	}{
		{
			name: "header_valid",
			configure: func(request *http.Request) {
				request.Header.Set(DesktopSessionTokenHeader, "desktop-token")
				request.AddCookie(&http.Cookie{Name: DesktopSessionTokenCookie, Value: "old-token"})
			},
			wantValid:  true,
			wantSource: "header",
			wantReason: "ok",
		},
		{
			name: "header_mismatch_takes_precedence",
			configure: func(request *http.Request) {
				request.Header.Set(DesktopSessionTokenHeader, "old-token")
				request.AddCookie(&http.Cookie{Name: DesktopSessionTokenCookie, Value: "desktop-token"})
			},
			wantValid:  false,
			wantSource: "header",
			wantReason: "header_mismatch",
		},
		{
			name: "protocol_mismatch",
			configure: func(request *http.Request) {
				request.Header.Set("Sec-WebSocket-Protocol", "nexus.desktop.v1, nexus.desktop.token.old-token")
			},
			wantValid:  false,
			wantSource: "protocol",
			wantReason: "protocol_mismatch",
		},
		{
			name: "cookie_mismatch",
			configure: func(request *http.Request) {
				request.AddCookie(&http.Cookie{Name: DesktopSessionTokenCookie, Value: "old-token"})
			},
			wantValid:  false,
			wantSource: "cookie",
			wantReason: "cookie_mismatch",
		},
		{
			name:       "missing",
			configure:  func(request *http.Request) {},
			wantValid:  false,
			wantSource: "none",
			wantReason: "missing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/nexus/v1/runtime/options", nil)
			test.configure(request)
			result := validateDesktopSessionToken(request, "desktop-token")
			if result.valid != test.wantValid || result.source != test.wantSource || result.reason != test.wantReason {
				t.Fatalf(
					"token 校验结果不正确: valid=%v source=%s reason=%s",
					result.valid,
					result.source,
					result.reason,
				)
			}
		})
	}
}

// A retired connection's request context can still reach a newly bound sender
// through a session broadcast. It must not retire that healthy connection.
func TestWebSocketSenderCanceledBroadcastKeepsConnection(t *testing.T) {
	sender, peer := websocketSenderPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sender.SendJSON(ctx, map[string]string{"event": "old"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled send = %v", err)
	}
	if sender.IsClosed() {
		t.Fatal("canceled broadcast retired the healthy replacement connection")
	}
	assertWebSocketSenderDelivery(t, sender, peer, map[string]string{"event": "new-round"})
}

// Once an event enters the connection writer, caller cancellation must not
// interrupt its frame and poison subsequent events on the shared connection.
func TestWebSocketSenderAdmittedWriteSurvivesCallerCancellation(t *testing.T) {
	sender, peer := websocketSenderPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	payload := cancelDuringJSON{cancel: cancel}
	if err := sender.SendJSON(ctx, payload); err != nil {
		t.Fatalf("admitted write = %v", err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("fixture did not cancel the originating request")
	}
	assertWebSocketPeerEvent(t, peer, "admitted")
	assertWebSocketSenderDelivery(t, sender, peer, map[string]string{"event": "next-round"})
}

func TestWebSocketSenderTransportFailureRetiresConnection(t *testing.T) {
	sender, _ := websocketSenderPair(t)
	_ = sender.conn.CloseNow()
	if err := sender.SendJSON(context.Background(), map[string]string{"event": "closed"}); err == nil {
		t.Fatal("send on closed transport succeeded")
	}
	if !sender.IsClosed() {
		t.Fatal("failed transport remained available")
	}
}

type cancelDuringJSON struct{ cancel context.CancelFunc }

func (p cancelDuringJSON) MarshalJSON() ([]byte, error) {
	p.cancel()
	return json.Marshal(map[string]string{"event": "admitted"})
}

func websocketSenderPair(t *testing.T) (*WebSocketSender, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err == nil {
			accepted <- conn
		}
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	peer, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.CloseNow() })
	select {
	case conn := <-accepted:
		t.Cleanup(func() { _ = conn.CloseNow() })
		return NewWebSocketSender(conn), peer
	case <-ctx.Done():
		t.Fatal("server did not accept the WebSocket")
		return nil, nil
	}
}

func assertWebSocketSenderDelivery(t *testing.T, sender *WebSocketSender, peer *websocket.Conn, payload map[string]string) {
	t.Helper()
	if err := sender.SendJSON(context.Background(), payload); err != nil {
		t.Fatalf("next event failed: %v", err)
	}
	assertWebSocketPeerEvent(t, peer, payload["event"])
}

func assertWebSocketPeerEvent(t *testing.T, peer *websocket.Conn, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var event map[string]string
	if err := wsjson.Read(ctx, peer, &event); err != nil {
		t.Fatal(err)
	}
	if event["event"] != want {
		t.Fatalf("event = %v, want %q", event, want)
	}
}
