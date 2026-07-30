package connectors

import (
	"net/http"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"

	_ "modernc.org/sqlite"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newConnectorsTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	return config.Config{
		Host:                         "127.0.0.1",
		Port:                         18013,
		ProjectName:                  "nexus-connectors-test",
		APIPrefix:                    "/nexus/v1",
		WebSocketPath:                "/nexus/v1/chat/ws",
		DefaultAgentID:               "nexus",
		WorkspacePath:                filepath.Join(root, "workspace"),
		CacheFileDir:                 filepath.Join(root, "cache"),
		DatabaseDriver:               "sqlite",
		DatabaseURL:                  filepath.Join(root, "nexus.db"),
		ConnectorOAuthRedirectURI:    "http://localhost:3000/capability/connectors/oauth/callback",
		ConnectorOAuthAllowedOrigins: []string{"http://localhost:3000"},
		ConnectorCredentialsKey:      "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		ConnectorGitHubClientID:      "github-client-id",
		ConnectorGitHubClientSecret:  "github-client-secret",
		ConnectorTwitterClientID:     "twitter-client-id",
		ConnectorTwitterClientSecret: "twitter-client-secret",
		ConnectorShopifyClientID:     "shopify-client-id",
		ConnectorShopifyClientSecret: "shopify-client-secret",
	}
}

func testConnectorCredentialKey() string {
	return "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
}

func migrateConnectorsSQLite(t *testing.T, databaseURL string) {
	t.Helper()
	handlertest.MigrateSQLiteFromDir(t, databaseURL, connectorsTestMigrationDir(t))
}

func connectorsTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
