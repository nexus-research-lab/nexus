package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
)

func TestDesktopRemoteGatewayKeepsSessionOnLocalOrigin(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Origin") != "http://"+request.Host ||
			request.Header.Get(handlershared.DesktopSessionTokenHeader) != "" {
			http.Error(writer, "bad forwarded request", http.StatusBadRequest)
			return
		}
		if _, err := request.Cookie(handlershared.DesktopSessionTokenCookie); err == nil {
			http.Error(writer, "desktop cookie leaked", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/auth/v1/login":
			http.SetCookie(writer, &http.Cookie{
				Name: "nexus_session", Value: "opaque", Path: "/", Secure: true, HttpOnly: true,
			})
			writer.WriteHeader(http.StatusOK)
		case "/nexus/v1/team/rooms":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(upstream.Close)
	server := &Server{
		config: config.Config{
			AppMode: "desktop", APIPrefix: "/nexus/v1", RemoteURL: upstream.URL,
			AuthSessionCookieName: "nexus_session",
			DesktopSessionToken:   "desktop-token",
		},
		api:    handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil))),
		router: newPathParamRouter(),
	}
	server.mountRemoteGateway()

	request := httptest.NewRequest(http.MethodPost, "http://app.local/auth/v1/login", nil)
	request.Header.Set("Origin", "http://app.local")
	request.AddCookie(&http.Cookie{Name: handlershared.DesktopSessionTokenCookie, Value: "desktop-token"})
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	cookies := response.Result().Cookies()
	if response.Code != http.StatusOK || len(cookies) != 1 || cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("status = %d, cookies = %+v", response.Code, cookies)
	}

	request = httptest.NewRequest(http.MethodPost, "http://app.local/auth/v1/login", nil)
	request.Header.Set("Origin", "https://attacker.example")
	request.AddCookie(&http.Cookie{Name: handlershared.DesktopSessionTokenCookie, Value: "desktop-token"})
	response = httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "http://app.local/nexus/v1/team/rooms", nil)
	request.Header.Set("Origin", "http://app.local")
	request.Header.Set(handlershared.DesktopSessionTokenHeader, "desktop-token")
	response = httptest.NewRecorder()
	server.router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("Team proxy status = %d", response.Code)
	}
}
