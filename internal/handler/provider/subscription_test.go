package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func TestSubscriptionProviderHandlersRejectMemberBeforeServiceAccess(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil)
	for _, test := range []struct {
		name   string
		method string
		path   string
		handle func(http.ResponseWriter, *http.Request)
	}{
		{name: "list", method: http.MethodGet, path: "/admin/subscription/providers", handle: handler.HandleListSubscriptionProviderConfigs},
		{name: "set default", method: http.MethodPost, path: "/admin/subscription/providers/shared/models/model/default", handle: handler.HandleSetSubscriptionDefaultProviderModel},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			request = request.WithContext(authctx.WithPrincipal(request.Context(), &authctx.Principal{
				UserID: "member-user",
				Role:   authctx.RoleMember,
			}))
			response := httptest.NewRecorder()

			test.handle(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("member status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
			}
		})
	}
}

func TestSubscriptionProviderAdminBoundary(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil)
	for _, test := range []struct {
		name      string
		principal *authctx.Principal
		allowed   bool
	}{
		{name: "local single user", allowed: true},
		{name: "owner", principal: &authctx.Principal{UserID: "owner", Role: authctx.RoleOwner}, allowed: true},
		{name: "admin", principal: &authctx.Principal{UserID: "admin", Role: authctx.RoleAdmin}, allowed: true},
		{name: "member", principal: &authctx.Principal{UserID: "member", Role: authctx.RoleMember}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.principal != nil {
				request = request.WithContext(authctx.WithPrincipal(request.Context(), test.principal))
			}
			response := httptest.NewRecorder()
			if got := handler.requireSubscriptionProviderAdmin(response, request); got != test.allowed {
				t.Fatalf("allowed = %v, want %v", got, test.allowed)
			}
			if !test.allowed && response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
			}
		})
	}
}
