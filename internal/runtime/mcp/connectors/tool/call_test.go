package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
)

type stubConnectorService struct {
	item *connectordomain.ConnectionSnapshot
}

func (s stubConnectorService) ListActiveConnections(ctx context.Context, ownerUserID string) ([]connectordomain.ConnectionSnapshot, error) {
	if s.item == nil {
		return nil, nil
	}
	return []connectordomain.ConnectionSnapshot{*s.item}, nil
}

func (s stubConnectorService) LoadActiveConnection(ctx context.Context, ownerUserID, connectorID string) (*connectordomain.ConnectionSnapshot, error) {
	if s.item == nil || s.item.ConnectorID != connectorID {
		return nil, nil
	}
	return s.item, nil
}

func TestConnectorCallSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("Authorization header not set: %q", request.Header.Get("Authorization"))
		}
		if request.URL.Path != "/user" || request.URL.Query().Get("visibility") != "private" {
			t.Fatalf("unexpected request URL: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"login":"octo"}`))
	}))
	defer server.Close()

	output := callTool(t, stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "github",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{
		"connector_id": "github",
		"method":       "GET",
		"path":         "/user",
		"query":        map[string]any{"visibility": "private"},
		"headers":      map[string]any{"Authorization": "Bearer evil", "X-Test": "ok"},
	})
	if output["status"].(float64) != 200 {
		t.Fatalf("unexpected status: %+v", output)
	}
	if !strings.Contains(output["body"].(string), "octo") {
		t.Fatalf("unexpected body: %+v", output)
	}
}

func TestConnectorCallReturnsNon2xxBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	output := callTool(t, stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "github",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{"connector_id": "github", "method": "GET", "path": "/missing"})
	if output["status"].(float64) != 404 {
		t.Fatalf("unexpected non-2xx output: %+v", output)
	}
	if !strings.Contains(output["body"].(string), "not found") {
		t.Fatalf("non-2xx body should be preserved: %+v", output)
	}
}

func TestConnectorCallTruncatesLargeBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", maxResponseBytes+10)))
	}))
	defer server.Close()

	output := callTool(t, stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "github",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{"connector_id": "github", "method": "GET", "path": "/big"})
	if output["_truncated"] != true {
		t.Fatalf("large response should be truncated: %+v", output)
	}
	if len(output["body"].(string)) != maxResponseBytes {
		t.Fatalf("truncated body size mismatch: %d", len(output["body"].(string)))
	}
}

func TestConnectorCallUnknownConnector(t *testing.T) {
	result := callToolResult(t, stubConnectorService{}, map[string]any{"connector_id": "missing", "method": "GET", "path": "/user"})
	if !result.IsError {
		t.Fatalf("unknown connector should return MCP error: %+v", result)
	}
}

func callTool(t *testing.T, svc contract.Service, args map[string]any) map[string]any {
	t.Helper()
	result := callToolResult(t, svc, args)
	if result.IsError {
		t.Fatalf("tool returned error: %+v", result)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Content[0]["text"].(string)), &output); err != nil {
		t.Fatalf("decode tool output: %v", err)
	}
	return output
}

func callToolResult(t *testing.T, svc contract.Service, args map[string]any) agentclient.MCPToolResult {
	t.Helper()
	for _, item := range BuildAll(svc, contract.ServerContext{OwnerUserID: "user-1"}) {
		if item.Name == "connector_call" {
			result, err := item.Handler(context.Background(), args)
			if err != nil {
				t.Fatalf("tool handler returned transport error: %v", err)
			}
			return result
		}
	}
	t.Fatal("connector_call tool not found")
	return agentclient.MCPToolResult{}
}
