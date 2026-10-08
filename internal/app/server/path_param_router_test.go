package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"

	"github.com/go-chi/chi/v5"
)

func TestPathParamRouterPreservesProviderModelIDForLegacyServiceDecoder(t *testing.T) {
	t.Parallel()

	router := newPathParamRouter()
	router.Get("/models/{model_id}", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(
			writer,
			chi.URLParam(request, "model_id")+"|"+handlershared.PathParam(request, "model_id"),
		)
	})
	server := httptest.NewServer(router)
	defer server.Close()

	response, err := server.Client().Get(server.URL + "/models/namespace%2Fmodel")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if got := strings.TrimSpace(string(payload)); got != "namespace%2Fmodel|namespace%2Fmodel" {
		t.Fatalf("model_id = %q, want service-compatible escaped value", got)
	}
}
