package connectors

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/service/auth"

	_ "modernc.org/sqlite"
)

func TestServiceEncryptsConnectionCredentials(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := NewService(cfg, db)
	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"access_token":"secret-token"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}

	var credentialText string
	var encrypted sql.NullString
	//goland:noinspection SqlResolve
	if err = db.QueryRowContext(ctx, "SELECT credentials, credentials_encrypted FROM connector_connections WHERE connector_id = ?", "github").Scan(&credentialText, &encrypted); err != nil {
		t.Fatalf("读取连接凭证失败: %v", err)
	}
	if credentialText != "__encrypted__" {
		t.Fatalf("明文字段不应保存 token payload: %q", credentialText)
	}
	if !encrypted.Valid || strings.Contains(encrypted.String, "secret-token") {
		t.Fatalf("加密字段未正确写入: %q", encrypted.String)
	}
	key, err := credentials.DecodeKey(cfg.ConnectorCredentialsKey)
	if err != nil {
		t.Fatalf("解析测试密钥失败: %v", err)
	}
	plain, err := credentials.DecryptPayload(key, encrypted.String)
	if err != nil {
		t.Fatalf("解密连接凭证失败: %v", err)
	}
	if string(plain) != `{"access_token":"secret-token"}` {
		t.Fatalf("解密后的凭证不正确: %s", plain)
	}
}

func TestServiceLoadActiveConnectionDecryptsAccessToken(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	cfg.ConnectorCredentialsKey = testConnectorCredentialKey()
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"access_token":"token","scope":"repo"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}

	item, err := service.LoadActiveConnection(ctx, auth.SystemUserID, "github")
	if err != nil {
		t.Fatalf("读取连接快照失败: %v", err)
	}
	if item == nil || item.AccessToken != "token" || item.APIBaseURL != "https://api.github.com" {
		t.Fatalf("连接快照不正确: %+v", item)
	}
	if item.Extra["scope"] != "repo" {
		t.Fatalf("extra 字段未保留: %+v", item.Extra)
	}

	items, err := service.ListActiveConnections(ctx, auth.SystemUserID)
	if err != nil {
		t.Fatalf("列出连接快照失败: %v", err)
	}
	if len(items) != 1 || items[0].ConnectorID != "github" || items[0].AccessToken != "token" {
		t.Fatalf("连接快照列表不正确: %+v", items)
	}
}

func TestServiceConnectsAPIKeyConnectors(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()

	tests := []struct {
		connectorID string
		query       string
		key         string
		apiBaseURL  string
		mcpURL      string
	}{
		{
			connectorID: "amap",
			query:       "高德",
			key:         "amap-key",
			apiBaseURL:  "https://restapi.amap.com",
			mcpURL:      "https://mcp.amap.com/mcp",
		},
		{
			connectorID: "didi",
			query:       "滴滴",
			key:         "didi-key",
			apiBaseURL:  "https://mcp.didichuxing.com",
			mcpURL:      "https://mcp.didichuxing.com/mcp-servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.connectorID, func(t *testing.T) {
			items, err := service.ListConnectors(ctx, auth.SystemUserID, tt.query, "", "")
			if err != nil {
				t.Fatalf("列出连接器失败: %v", err)
			}
			if len(items) != 1 || items[0].ConnectorID != tt.connectorID || items[0].AuthType != "api_key" || !items[0].IsConfigured {
				t.Fatalf("连接器目录不正确: %+v", items)
			}

			if _, err = service.Connect(ctx, auth.SystemUserID, tt.connectorID, map[string]string{}); err == nil || !strings.Contains(err.Error(), "API Key") {
				t.Fatalf("缺少 API Key 应报错，实际: %v", err)
			}

			info, err := service.Connect(ctx, auth.SystemUserID, tt.connectorID, map[string]string{"api_key": tt.key})
			if err != nil {
				t.Fatalf("连接失败: %v", err)
			}
			if info.ConnectionState != "connected" {
				t.Fatalf("连接状态不正确: %+v", info)
			}

			snapshot, err := service.LoadActiveConnection(ctx, auth.SystemUserID, tt.connectorID)
			if err != nil {
				t.Fatalf("读取连接快照失败: %v", err)
			}
			if snapshot == nil || snapshot.AccessToken != tt.key || snapshot.APIBaseURL != tt.apiBaseURL {
				t.Fatalf("连接快照不正确: %+v", snapshot)
			}

			detail, err := service.GetConnectorDetail(ctx, auth.SystemUserID, tt.connectorID)
			if err != nil {
				t.Fatalf("读取详情失败: %v", err)
			}
			if detail.MCPServerURL != tt.mcpURL {
				t.Fatalf("MCP server 地址不正确: got=%q want=%q", detail.MCPServerURL, tt.mcpURL)
			}
			if len(detail.FeatureDetails) != len(detail.Features) {
				t.Fatalf("能力说明不完整: features=%v details=%v", detail.Features, detail.FeatureDetails)
			}
		})
	}
}

func TestServiceConnectsOfficeMCPTokenConnectors(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()

	tests := []struct {
		connectorID string
		query       string
		token       string
		apiBaseURL  string
		mcpURL      string
	}{
		{
			connectorID: "dingtalk-ai-table",
			query:       "钉钉 AI 表格",
			token:       "https://mcp.dingtalk.com/sse?token=dingtalk-secret",
			apiBaseURL:  "https://mcp.dingtalk.com",
			mcpURL:      "https://mcp.dingtalk.com/#/detail?mcpId=9555&detailType=marketMcpDetail",
		},
		{
			connectorID: "tencent-docs",
			query:       "腾讯文档",
			token:       "tencent-docs-token",
			apiBaseURL:  "https://docs.qq.com",
			mcpURL:      "https://docs.qq.com/openapi/mcp",
		},
		{
			connectorID: "yuque",
			query:       "语雀",
			token:       "yuque-token",
			apiBaseURL:  "https://www.yuque.com/api/v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.connectorID, func(t *testing.T) {
			items, err := service.ListConnectors(ctx, auth.SystemUserID, tt.query, "", "")
			if err != nil {
				t.Fatalf("列出连接器失败: %v", err)
			}
			if len(items) != 1 || items[0].ConnectorID != tt.connectorID || items[0].AuthType != "token" || !items[0].IsConfigured {
				t.Fatalf("连接器目录不正确: %+v", items)
			}

			if _, err = service.Connect(ctx, auth.SystemUserID, tt.connectorID, map[string]string{}); err == nil || !strings.Contains(err.Error(), "Token") {
				t.Fatalf("缺少 Token 应报错，实际: %v", err)
			}

			info, err := service.Connect(ctx, auth.SystemUserID, tt.connectorID, map[string]string{"token": tt.token})
			if err != nil {
				t.Fatalf("连接失败: %v", err)
			}
			if info.ConnectionState != "connected" {
				t.Fatalf("连接状态不正确: %+v", info)
			}

			snapshot, err := service.LoadActiveConnection(ctx, auth.SystemUserID, tt.connectorID)
			if err != nil {
				t.Fatalf("读取连接快照失败: %v", err)
			}
			if snapshot == nil || snapshot.AccessToken != tt.token || snapshot.APIBaseURL != tt.apiBaseURL {
				t.Fatalf("连接快照不正确: %+v", snapshot)
			}

			detail, err := service.GetConnectorDetail(ctx, auth.SystemUserID, tt.connectorID)
			if err != nil {
				t.Fatalf("读取详情失败: %v", err)
			}
			if detail.MCPServerURL != tt.mcpURL {
				t.Fatalf("MCP server 地址不正确: got=%q want=%q", detail.MCPServerURL, tt.mcpURL)
			}
			if len(detail.FeatureDetails) != len(detail.Features) {
				t.Fatalf("能力说明不完整: features=%v details=%v", detail.Features, detail.FeatureDetails)
			}
		})
	}
}

func TestServiceScopesAmapAPIKeyByOwner(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	if _, err = service.Connect(ctx, "owner-a", "amap", map[string]string{"api_key": "amap-owner-a"}); err != nil {
		t.Fatalf("连接 owner-a 高德失败: %v", err)
	}
	if _, err = service.Connect(ctx, "owner-b", "amap", map[string]string{"api_key": "amap-owner-b"}); err != nil {
		t.Fatalf("连接 owner-b 高德失败: %v", err)
	}

	snapshotA, err := service.LoadActiveConnection(ctx, "owner-a", "amap")
	if err != nil {
		t.Fatalf("读取 owner-a 高德连接失败: %v", err)
	}
	snapshotB, err := service.LoadActiveConnection(ctx, "owner-b", "amap")
	if err != nil {
		t.Fatalf("读取 owner-b 高德连接失败: %v", err)
	}
	if snapshotA == nil || snapshotA.AccessToken != "amap-owner-a" {
		t.Fatalf("owner-a 不应读到其他用户高德 Key: %+v", snapshotA)
	}
	if snapshotB == nil || snapshotB.AccessToken != "amap-owner-b" {
		t.Fatalf("owner-b 不应读到其他用户高德 Key: %+v", snapshotB)
	}

	if _, err = service.Disconnect(ctx, "owner-b", "amap"); err != nil {
		t.Fatalf("断开 owner-b 高德失败: %v", err)
	}
	snapshotA, err = service.LoadActiveConnection(ctx, "owner-a", "amap")
	if err != nil {
		t.Fatalf("再次读取 owner-a 高德连接失败: %v", err)
	}
	snapshotB, err = service.LoadActiveConnection(ctx, "owner-b", "amap")
	if err != nil {
		t.Fatalf("再次读取 owner-b 高德连接失败: %v", err)
	}
	if snapshotA == nil || snapshotA.AccessToken != "amap-owner-a" || snapshotB != nil {
		t.Fatalf("断开 owner-b 不应影响 owner-a: owner-a=%+v owner-b=%+v", snapshotA, snapshotB)
	}
}

func TestServiceLoadActiveConnectionRequiresAccessToken(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"scope":"repo"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}

	_, err = service.LoadActiveConnection(ctx, auth.SystemUserID, "github")
	if err == nil || !strings.Contains(err.Error(), "access token") {
		t.Fatalf("缺少 access token 应报错，实际: %v", err)
	}
}

func TestServiceLoadActiveConnectionRefreshesExpiredFeishuDocxToken(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.Header.Get("Content-Type"), "application/json") {
			t.Fatalf("飞书 refresh 应使用 JSON: %s", request.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("读取 refresh 请求失败: %v", err)
		}
		if !strings.Contains(string(body), `"refresh_token":"old-refresh"`) {
			t.Fatalf("refresh 请求未带旧 refresh_token: %s", body)
		}
		if !strings.Contains(string(body), `"client_id":"refresh-feishu-client"`) || !strings.Contains(string(body), `"client_secret":"refresh-feishu-secret"`) {
			t.Fatalf("refresh 请求未使用用户自有 OAuth Client: %s", body)
		}
		_, _ = writer.Write([]byte(`{"code":0,"data":{"access_token":"new-feishu-docx-token","refresh_token":"new-refresh","expires_in":7200}}`))
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_FEISHU_DOCX_TOKEN_URL", server.URL)

	service := NewService(cfg, db)
	service.httpClient = server.Client()
	ctx := context.Background()
	if _, err = service.SaveOAuthClientConfig(ctx, auth.SystemUserID, "feishu-docx", OAuthClientConfigRequest{
		ClientID:     "refresh-feishu-client",
		ClientSecret: "refresh-feishu-secret",
	}); err != nil {
		t.Fatalf("保存飞书 OAuth Client 失败: %v", err)
	}
	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "feishu-docx",
		State:       "connected",
		Credentials: `{"access_token":"old-feishu-docx-token","refresh_token":"old-refresh","expires_at":"1","scope":"docx:document"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入飞书连接状态失败: %v", err)
	}

	item, err := service.LoadActiveConnection(ctx, auth.SystemUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取飞书连接快照失败: %v", err)
	}
	if item == nil || item.AccessToken != "new-feishu-docx-token" {
		t.Fatalf("飞书 token 未刷新: %+v", item)
	}
	if item.Extra["refresh_token"] != "new-refresh" || item.Extra["scope"] != "docx:document" {
		t.Fatalf("飞书 refresh_token 或旧 extra 未保留: %+v", item.Extra)
	}
}
