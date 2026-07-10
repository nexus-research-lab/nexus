package memory_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
)

func TestMemoryRoutesAreNotRegistered(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	for _, path := range []string{
		"/nexus/v1/agents/nexus/memory/items",
		"/nexus/v1/memory/items",
	} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		recorder := httptest.NewRecorder()
		server.Router().ServeHTTP(recorder, request)
		if recorder.Code == http.StatusOK {
			t.Fatalf("memory API %s unexpectedly registered", path)
		}
	}
}
