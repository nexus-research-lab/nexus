package shared

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

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
