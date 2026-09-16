package team

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
)

func TestNodeHandlerRequiresRemoteCookieAndSameOrigin(t *testing.T) {
	handler := NewNodeHandlers(handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil))), nil, "nexus_session")
	for _, test := range []struct {
		method, origin, cookie string
		status                 int
	}{
		{http.MethodGet, "", "", 401},
		{http.MethodPost, "http://nexus.test", "", 401},
		{http.MethodPost, "https://evil.test", "session", 403},
		{http.MethodDelete, "", "session", 403},
	} {
		r := httptest.NewRequest(test.method, "http://nexus.test/nexus/v1/team-node", nil)
		r.Header.Set("Origin", test.origin)
		if test.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "nexus_session", Value: test.cookie})
		}
		w := httptest.NewRecorder()
		handler.Handle(w, r)
		if w.Code != test.status || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s/%s: %d", test.method, test.origin, w.Code)
		}
	}
}
