package shared

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareWritesRequestIDAndAccessLog(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := RequestContextMiddleware(logger)(
		AccessLogMiddleware()(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte("ok"))
		})),
	)

	request := httptest.NewRequest(http.MethodPost, "/nexus/v1/agents?limit=10", nil)
	request.RemoteAddr = "127.0.0.1:9000"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if requestID := recorder.Result().Header.Get("X-Request-ID"); requestID == "" {
		t.Fatal("响应头未写入 X-Request-ID")
	}

	output := buffer.String()
	if !strings.Contains(output, "\"msg\":\"HTTP 请求完成\"") {
		t.Fatalf("未写入 access log: %s", output)
	}
	if !strings.Contains(output, "\"status\":201") {
		t.Fatalf("access log 状态码不正确: %s", output)
	}
	if !strings.Contains(output, "\"request_id\"") {
		t.Fatalf("access log 缺少 request_id: %s", output)
	}
	if strings.Contains(output, "\"ok\"") {
		t.Fatalf("access log 不应包含响应体: %s", output)
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

func TestAccessLogMiddlewareDemotesSuccessfulGetAtInfoLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := RequestContextMiddleware(logger)(
		AccessLogMiddleware()(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})),
	)

	request := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if output := buffer.String(); strings.Contains(output, "HTTP 请求完成") {
		t.Fatalf("成功 GET 不应写入 info access log: %s", output)
	}
}

func TestAccessLogMiddlewareDemotesSuccessfulWebSocketAtInfoLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := RequestContextMiddleware(logger)(
		AccessLogMiddleware()(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusSwitchingProtocols)
		})),
	)

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/chat/ws", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if output := buffer.String(); strings.Contains(output, "HTTP 请求完成") {
		t.Fatalf("WebSocket 101 不应写入 info access log: %s", output)
	}
}

func TestAccessLogMiddlewareKeepsFailureAtInfoLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := RequestContextMiddleware(logger)(
		AccessLogMiddleware()(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNotFound)
		})),
	)

	request := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	output := buffer.String()
	if !strings.Contains(output, "\"msg\":\"HTTP 请求完成\"") || !strings.Contains(output, "\"status\":404") {
		t.Fatalf("失败请求应继续写入 access log: %s", output)
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

func TestDesktopSessionTokenMiddlewareProtectsAPI(t *testing.T) {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := DesktopSessionTokenMiddleware(api, "desktop-token", "/nexus/v1")(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("ok"))
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/runtime/options", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("缺少 token 应返回 401，实际: %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/nexus/v1/runtime/options", nil)
	request.Header.Set(DesktopSessionTokenHeader, "desktop-token")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "ok" {
		t.Fatalf("合法 token 未通过: status=%d body=%s", recorder.Code, recorder.Body.String())
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

func TestDesktopSessionTokenMiddlewareAcceptsWebSocketProtocolToken(t *testing.T) {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := DesktopSessionTokenMiddleware(api, "desktop-token", "/nexus/v1")(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("ok"))
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/chat/ws", nil)
	request.Header.Set("Sec-WebSocket-Protocol", "nexus.desktop.v1, nexus.desktop.token.desktop-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("WebSocket protocol token 未通过: %d", recorder.Code)
	}
}

func TestDesktopSessionTokenMiddlewareAcceptsCookieToken(t *testing.T) {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := DesktopSessionTokenMiddleware(api, "desktop-token", "/nexus/v1")(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("ok"))
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/chat/ws", nil)
	request.AddCookie(&http.Cookie{Name: DesktopSessionTokenCookie, Value: "desktop-token"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("桌面 cookie token 未通过: %d", recorder.Code)
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
