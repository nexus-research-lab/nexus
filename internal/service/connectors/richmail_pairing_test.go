package connectors

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRichMailPairingConnectsAndRetriesExactCompletedAttempt(t *testing.T) {
	var pollCount atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/mcp/auth/request":
			if request.Method != http.MethodPost {
				t.Fatalf("start method=%s", request.Method)
			}
			if request.Header.Get("Authorization") != "" {
				t.Fatal("首次 RichMail 配对请求不应携带 Authorization")
			}
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"data":{"requestId":"request-1","expiresIn":300,"pollInterval":1}}`))
		case "/mcp/auth/poll":
			pollCount.Add(1)
			if request.Method != http.MethodGet || request.URL.Query().Get("requestId") != "request-1" {
				t.Fatalf("poll request=%s %s", request.Method, request.URL.String())
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"status":"approved","header":"Authorization: Bearer rich-secret"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer provider.Close()

	service, db := newRichMailPairingTestService(t)
	defer func() { _ = db.Close() }()
	service.richMailBaseURL = provider.URL

	ctx := context.Background()
	started, err := service.StartLocalPairing(ctx, "owner-1", richMailConnectorID)
	if err != nil {
		t.Fatalf("启动 RichMail 配对失败: %v", err)
	}
	if started.Endpoint != richMailDefaultMCPURL || started.Interval != 1 ||
		!strings.HasPrefix(started.AttemptToken, richMailPairingAttemptPrefix) {
		t.Fatalf("RichMail 配对启动结果不正确: %+v", started)
	}

	connected, err := service.PollLocalPairing(
		ctx, "owner-1", richMailConnectorID, started.AttemptToken,
	)
	if err != nil {
		t.Fatalf("完成 RichMail 配对失败: %v", err)
	}
	if connected.Status != localPairingStatusConnected ||
		connected.Connector == nil ||
		connected.Connector.ConnectionState != "connected" {
		t.Fatalf("RichMail 连接结果不正确: %+v", connected)
	}
	snapshot, err := service.LoadActiveConnection(ctx, "owner-1", richMailConnectorID)
	if err != nil {
		t.Fatalf("读取 RichMail 连接失败: %v", err)
	}
	if snapshot == nil || snapshot.AccessToken != "rich-secret" ||
		strings.TrimSpace(snapshot.Extra["pairing_attempt_id"]) == "" {
		t.Fatalf("RichMail 凭据快照不正确: %+v", snapshot)
	}

	retried, err := service.PollLocalPairing(
		ctx, "owner-1", richMailConnectorID, started.AttemptToken,
	)
	if err != nil || retried.Status != localPairingStatusConnected {
		t.Fatalf("ACK 丢失后的相同 attempt 应可对账: result=%+v err=%v", retried, err)
	}
	if pollCount.Load() != 1 {
		t.Fatalf("完成态对账不应重复请求 provider: polls=%d", pollCount.Load())
	}

	var plain string
	var encrypted sql.NullString
	if err = db.QueryRowContext(ctx, `
SELECT credentials, credentials_encrypted
FROM connector_connections
WHERE owner_user_id = ? AND connector_id = ?`,
		"owner-1", richMailConnectorID,
	).Scan(&plain, &encrypted); err != nil {
		t.Fatalf("读取 RichMail 持久凭据失败: %v", err)
	}
	if plain != "__encrypted__" || !encrypted.Valid ||
		strings.Contains(encrypted.String, "rich-secret") {
		t.Fatalf("RichMail Token 不应明文持久化: plain=%q encrypted=%q", plain, encrypted.String)
	}
}

func TestRichMailPairingRejectsStaleConfigurationAndCrossOwnerAttempt(t *testing.T) {
	var pollCount atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/mcp/auth/request" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"requestId":"request-stale"}`))
			return
		}
		pollCount.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"approved","token":"late-token"}`))
	}))
	defer provider.Close()

	service, db := newRichMailPairingTestService(t)
	defer func() { _ = db.Close() }()
	service.richMailBaseURL = provider.URL
	ctx := context.Background()
	started, err := service.StartLocalPairing(ctx, "owner-a", richMailConnectorID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.PollLocalPairing(
		ctx, "owner-b", richMailConnectorID, started.AttemptToken,
	); err == nil || !strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("跨 owner attempt 未被拒绝: %v", err)
	}
	if _, err = service.Disconnect(ctx, "owner-a", richMailConnectorID); err != nil {
		t.Fatalf("推进 RichMail 配置版本失败: %v", err)
	}
	if _, err = service.PollLocalPairing(
		ctx, "owner-a", richMailConnectorID, started.AttemptToken,
	); !errors.Is(err, ErrConfigurationConflict) {
		t.Fatalf("旧配对必须拒绝覆盖新配置: %v", err)
	}
	if pollCount.Load() != 0 {
		t.Fatalf("配置冲突应在调用 provider 前拒绝: polls=%d", pollCount.Load())
	}
}

func TestRichMailBearerTokenAcceptsKnownShapesAndRejectsUnsafeHeaders(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		header  string
		want    string
	}{
		{
			name: "nested authorization header",
			payload: map[string]any{
				"headers": map[string]any{"authorization": "Bearer nested-token"},
			},
			want: "nested-token",
		},
		{
			name:   "response authorization header",
			header: "Bearer response-token",
			want:   "response-token",
		},
		{
			name:    "non bearer header",
			payload: map[string]any{"authorization": "Basic unsafe"},
		},
		{
			name:    "header injection",
			payload: map[string]any{"token": "token\r\nX-Unsafe: value"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			headers := http.Header{}
			headers.Set("Authorization", test.header)
			if got := richMailBearerToken(test.payload, headers); got != test.want {
				t.Fatalf("token=%q want=%q", got, test.want)
			}
		})
	}
}

func TestRichMailIsFixedAvailableConnector(t *testing.T) {
	service, db := newRichMailPairingTestService(t)
	defer func() { _ = db.Close() }()
	items, err := service.ListConnectors(
		context.Background(), "owner-1", "richmail", "", "available",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ConnectorID != richMailConnectorID ||
		items[0].AuthType != "local_pairing" {
		t.Fatalf("RichMail 目录投影不正确: %+v", items)
	}
	detail, err := service.GetConnectorDetail(
		context.Background(), "owner-1", richMailConnectorID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if detail.MCPServerURL != richMailDefaultMCPURL || detail.ConnectionState != "disconnected" {
		t.Fatalf("RichMail 详情不正确: %+v", detail)
	}
}

func TestDiscoverRichMailCapabilitiesUsesSavedBearerAndPreservesDescription(t *testing.T) {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "richmail", Title: "RichMail", Version: "1.0.0"},
		nil,
	)
	server.AddTool(&mcp.Tool{
		Name:        "getMailDetail",
		Description: "获取单封邮件的完整正文与附件列表。[Phase A]",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mid": map[string]any{"type": "string"},
			},
			"required": []string{"mid"},
		},
	}, nil)
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	)
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer rich-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(writer, request)
	}))
	defer remote.Close()

	service, db := newRichMailPairingTestService(t)
	defer func() { _ = db.Close() }()
	service.richMailMCPURL = remote.URL
	if err := service.upsertConnection(context.Background(), connectionRecord{
		OwnerUserID: "owner-1",
		ConnectorID: richMailConnectorID,
		State:       "connected",
		Credentials: `{"token":"rich-token"}`,
		AuthType:    "local_pairing",
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := service.DiscoverConnectorMCPCapabilities(
		context.Background(), "owner-1", richMailConnectorID,
	)
	if err != nil {
		t.Fatalf("读取 RichMail 工具失败: %v", err)
	}
	if len(catalog.Tools) != 1 || catalog.Tools[0].Name != "getMailDetail" ||
		catalog.Tools[0].Description != "获取单封邮件的完整正文与附件列表。[Phase A]" ||
		len(catalog.Tools[0].Arguments) != 1 || !catalog.Tools[0].Arguments[0].Required {
		t.Fatalf("RichMail 工具投影不正确: %+v", catalog)
	}
}

func newRichMailPairingTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	return NewService(cfg, db), db
}
