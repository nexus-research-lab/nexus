package agent

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSessionTaskSessionKeyParamDecodesEscapedSessionKey(t *testing.T) {
	request := httptest.NewRequest("GET", "/nexus/v1/sessions/agent%3Aa1%3Aws%3Adm%3Ar1/tasks", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("session_key", "agent%3Aa1%3Aws%3Adm%3Ar1")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))

	got := sessionTaskSessionKeyParam(request)
	if got != "agent:a1:ws:dm:r1" {
		t.Fatalf("sessionTaskSessionKeyParam() = %q, want decoded session key", got)
	}
}
