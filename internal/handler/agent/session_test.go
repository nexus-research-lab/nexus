package agent

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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
