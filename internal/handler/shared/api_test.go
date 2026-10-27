package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayClientErrorDetailPreservesDefaultModelHint(t *testing.T) {
	detail := "默认模型仍使用 Provider kimi-code；请先在设置中切换默认模型"
	got := GatewayClientErrorDetail(http.StatusBadRequest, detail)
	if got != detail {
		t.Fatalf("BadRequest 下的默认模型保护提示应直接返回给客户端，got=%q want=%q", got, detail)
	}
}

func TestPublicAuthRouteAllowsTeamCapabilitiesOnly(t *testing.T) {
	for path, want := range map[string]bool{
		"/nexus/v1/team/capabilities": true,
		"/nexus/v1/team/rooms":        false,
		"/nexus/v1/team/invitations":  false,
	} {
		if got := PublicAuthRoute(httptest.NewRequest(http.MethodGet, path, nil)); got != want {
			t.Fatalf("PublicAuthRoute(%s)=%v want=%v", path, got, want)
		}
	}
}
