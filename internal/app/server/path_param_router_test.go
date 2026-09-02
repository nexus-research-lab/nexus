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

func TestPathParamRouterDecodesBrowserSegmentsExactlyOnce(t *testing.T) {
	t.Parallel()

	router := newPathParamRouter()
	router.Get("/resources/{resource_id}", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, chi.URLParam(request, "resource_id"))
	})
	server := httptest.NewServer(router)
	defer server.Close()

	tests := []struct {
		path string
		want string
	}{
		{path: "bfd76ae5976a%40im.bot", want: "bfd76ae5976a@im.bot"},
		{path: "custom-mcp%3Arichmail", want: "custom-mcp:richmail"},
		{path: "scope%2Fskill", want: "scope/skill"},
		{path: "%E5%BE%AE%E4%BF%A1", want: "微信"},
		{path: "literal%252Fvalue", want: "literal%2Fvalue"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.path, func(t *testing.T) {
			response, err := server.Client().Get(server.URL + "/resources/" + test.path)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer response.Body.Close()
			payload, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response failed: %v", err)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
			}
			if got := string(payload); got != test.want {
				t.Fatalf("path param = %q, want %q", got, test.want)
			}
		})
	}
}

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
