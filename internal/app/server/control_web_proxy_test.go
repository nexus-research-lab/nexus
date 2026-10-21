package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
)

func TestDesktopTeamCapabilityIsolation(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(strconv.FormatBool(disabled), func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"enabled":false},"success":true,"code":"0000"}`))
			}))
			defer upstream.Close()
			s := &Server{config: config.Config{AppMode: "desktop", APIPrefix: "/nexus/v1", RemoteURL: upstream.URL, MultiplayerDisabled: disabled}, api: handlershared.NewAPI(nil), router: newPathParamRouter()}
			s.mountRemoteGateway()
			s.mountTeamRoutes()
			response := httptest.NewRecorder()
			s.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/nexus/v1/team/capabilities", nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":false`) {
				t.Fatalf("capability status=%d body=%s", response.Code, response.Body.String())
			}
			if (calls == 0) != disabled {
				t.Fatalf("upstream calls=%d disabled=%v", calls, disabled)
			}
		})
	}
}

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
