package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	shared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"
)

func TestSessionKeyPathParamDecodesEscapedSessionKey(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "encoded group key",
			raw:  "agent%3Aa1%3Aws%3Agroup%3Ar1",
			want: "agent:a1:ws:group:r1",
		},
		{
			name: "already decoded key",
			raw:  "agent:a1:ws:dm:r1",
			want: "agent:a1:ws:dm:r1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				"GET",
				"/nexus/v1/sessions/value/runtime-settings",
				nil,
			)
			routeContext := chi.NewRouteContext()
			routeContext.URLParams.Add("session_key", test.raw)
			request = request.WithContext(context.WithValue(
				request.Context(),
				chi.RouteCtxKey,
				routeContext,
			))

			if got := sessionKeyPathParam(request); got != test.want {
				t.Fatalf("sessionKeyPathParam() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteSubagentTaskErrorDistinguishesUnsupportedRuntime(t *testing.T) {
	handler := &Handlers{api: shared.NewAPI(nil)}
	recorder := httptest.NewRecorder()

	handler.writeSubagentTaskError(recorder, sessionpkg.ErrSubagentOperationUnsupported)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "当前运行时不支持该操作") || strings.Contains(body, "已结束") {
		t.Fatalf("unsupported response = %s", body)
	}
}
