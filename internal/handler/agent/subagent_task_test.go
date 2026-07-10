package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"
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

func TestWriteSubagentTaskErrorDistinguishesUnsupportedRuntime(t *testing.T) {
	handler := &Handlers{api: handlershared.NewAPI(nil)}
	recorder := httptest.NewRecorder()

	handler.writeSubagentTaskError(recorder, sessionpkg.ErrSubagentOperationUnsupported)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "当前运行时不支持该操作") || strings.Contains(body, "已结束") {
		t.Fatalf("unsupported response = %s", body)
	}
}
