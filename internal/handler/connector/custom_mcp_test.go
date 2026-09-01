package connector

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCustomMCPConnectorIDDecodesEscapedSeparator(t *testing.T) {
	request := httptest.NewRequest("GET", "/custom-mcp-servers/custom-mcp%3Aabc", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("connector_id", "custom-mcp%3Aabc")
	request = request.WithContext(context.WithValue(
		request.Context(),
		chi.RouteCtxKey,
		routeContext,
	))

	if got := customMCPConnectorID(request); got != "custom-mcp:abc" {
		t.Fatalf("customMCPConnectorID() = %q, want %q", got, "custom-mcp:abc")
	}
}
