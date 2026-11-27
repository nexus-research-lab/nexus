// connectors_test.go 覆盖连接器夹具、目录、配置版本、凭据、自定义 MCP 与 Richmail 配对测试。
package connectors

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/service/auth"
	_ "modernc.org/sqlite"
)

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

func hasHan(text string) bool {
	for _, char := range text {
		if unicode.Is(unicode.Han, char) {
			return true
		}
	}
	return false
}

func catalogDetail(entry CatalogEntry) Detail {
	return Detail{
		Info:           Info{ConnectorID: entry.ConnectorID, Kind: ConnectorKindCatalog, Title: entry.Title, Description: entry.Description},
		Scopes:         entry.Scopes,
		Features:       entry.Features,
		FeatureDetails: connectorFeatureDetailsFor(entry),
	}
}

func TestLocalizeDetailCoversEveryCatalogText(t *testing.T) {
	for _, entry := range connectorCatalog {
		source := catalogDetail(entry)
		detail := LocalizeDetail(source, "en")
		texts := map[string]string{"title": detail.Title, "description": detail.Description}
		for _, feature := range detail.Features {
			texts["feature "+feature] = feature
		}
		for _, feature := range detail.FeatureDetails {
			texts[feature.Name+" description"] = feature.Description
			for _, item := range feature.Items {
				texts[feature.Name+" item "+item] = item
			}
		}
		for name, text := range texts {
			if text == "" || hasHan(text) {
				t.Errorf("%s %s 缺少英文文案: %q", entry.ConnectorID, name, text)
			}
		}
		if len(detail.FeatureDetails) != len(source.FeatureDetails) {
			t.Fatalf("%s 能力数量变化: %d != %d", entry.ConnectorID, len(detail.FeatureDetails), len(source.FeatureDetails))
		}
		for index, feature := range detail.FeatureDetails {
			if feature.Name != detail.Features[index] {
				t.Errorf("%s 能力名与详情不一致: %q != %q", entry.ConnectorID, detail.Features[index], feature.Name)
			}
			if len(feature.Items) != len(source.FeatureDetails[index].Items) {
				t.Errorf("%s 能力 %s 的要点数量变化", entry.ConnectorID, feature.Name)
			}
			if !reflect.DeepEqual(feature.Scopes, source.FeatureDetails[index].Scopes) {
				t.Errorf("%s 能力 %s 的授权范围被改写", entry.ConnectorID, feature.Name)
			}
		}
		if info := LocalizeInfo(source.Info, "en"); !reflect.DeepEqual(info, detail.Info) {
			t.Errorf("%s 目录项与详情文案不一致", entry.ConnectorID)
		}
	}
}

func TestLocalizeKeepsChineseCatalogAndCustomMCP(t *testing.T) {
	for _, locale := range []string{"", "zh", "zh-CN", "enigma"} {
		for _, entry := range connectorCatalog {
			source := catalogDetail(entry)
			if got := LocalizeDetail(source, locale); !reflect.DeepEqual(got, source) {
				t.Errorf("locale %q 改写了 %s 的中文目录", locale, entry.ConnectorID)
			}
		}
	}
	custom := Info{ConnectorID: "github", Kind: ConnectorKindCustomMCP, Title: "我的工具", Description: "自定义说明"}
	if got := LocalizeInfo(custom, "en"); !reflect.DeepEqual(got, custom) {
		t.Errorf("自定义 MCP 文案被改写: %+v", got)
	}
}

func TestConnectorConfigurationVersionRejectsStaleConversationWrite(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := NewService(cfg, db)
	state, err := service.GetConfigurationState(ctx, auth.SystemUserID, "amap")
	if err != nil {
		t.Fatalf("读取初始配置版本失败: %v", err)
	}
	if state.ConfigurationVersion != 1 || state.ConnectionExists {
		t.Fatalf("初始配置状态不正确: %+v", state)
	}

	if _, err = service.ConnectAtVersion(
		ctx,
		auth.SystemUserID,
		"amap",
		map[string]string{"api_key": "first-key"},
		state.ConfigurationVersion,
	); err != nil {
		t.Fatalf("按版本连接失败: %v", err)
	}
	connected, err := service.GetConfigurationState(ctx, auth.SystemUserID, "amap")
	if err != nil {
		t.Fatalf("读取连接后状态失败: %v", err)
	}
	if connected.ConfigurationVersion != 2 || connected.ConnectionState != "connected" ||
		!connected.ConnectionConfigured {
		t.Fatalf("连接后状态未推进: %+v", connected)
	}

	_, err = service.DisconnectAtVersion(
		ctx,
		auth.SystemUserID,
		"amap",
		state.ConfigurationVersion,
	)
	if !errors.Is(err, ErrConfigurationConflict) {
		t.Fatalf("旧版本必须被拒绝，实际: %v", err)
	}
	stillConnected, err := service.GetConfigurationState(ctx, auth.SystemUserID, "amap")
	if err != nil {
		t.Fatalf("读取冲突后状态失败: %v", err)
	}
	if stillConnected.ConfigurationVersion != 2 || stillConnected.ConnectionState != "connected" {
		t.Fatalf("冲突写不应改变状态: %+v", stillConnected)
	}

	if _, err = service.DisconnectAtVersion(
		ctx,
		auth.SystemUserID,
		"amap",
		connected.ConfigurationVersion,
	); err != nil {
		t.Fatalf("按当前版本断开失败: %v", err)
	}
	disconnected, err := service.GetConfigurationState(ctx, auth.SystemUserID, "amap")
	if err != nil {
		t.Fatalf("读取断开后状态失败: %v", err)
	}
	if disconnected.ConfigurationVersion != 3 || disconnected.ConnectionState != "disconnected" ||
		disconnected.ConnectionConfigured {
		t.Fatalf("断开后凭据或版本不正确: %+v", disconnected)
	}
}

func TestDeleteOAuthClientIsAtomicWithDisconnectAndVersionAdvance(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := NewService(cfg, db)
	initial, err := service.GetConfigurationState(ctx, auth.SystemUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取初始状态失败: %v", err)
	}
	if _, err = service.SaveOAuthClientConfigAtVersion(
		ctx,
		auth.SystemUserID,
		"feishu-docx",
		OAuthClientConfigRequest{ClientID: "client", ClientSecret: "secret"},
		initial.ConfigurationVersion,
	); err != nil {
		t.Fatalf("保存 OAuth Client 失败: %v", err)
	}
	withClient, err := service.GetConfigurationState(ctx, auth.SystemUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取 OAuth Client 状态失败: %v", err)
	}
	if !withClient.OAuthClientConfigured || withClient.ConfigurationVersion != 2 {
		t.Fatalf("OAuth Client 状态不正确: %+v", withClient)
	}
	if _, err = service.upsertConnectionAtVersion(ctx, connectionRecord{
		OwnerUserID: auth.SystemUserID,
		ConnectorID: "feishu-docx",
		State:       "connected",
		Credentials: `{"access_token":"token"}`,
		AuthType:    "oauth2",
	}, &withClient.ConfigurationVersion); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}
	connected, err := service.GetConfigurationState(ctx, auth.SystemUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取连接状态失败: %v", err)
	}

	if _, err = service.DeleteOAuthClientConfigAtVersion(
		ctx,
		auth.SystemUserID,
		"feishu-docx",
		connected.ConfigurationVersion,
	); err != nil {
		t.Fatalf("删除 OAuth Client 失败: %v", err)
	}
	after, err := service.GetConfigurationState(ctx, auth.SystemUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取删除后状态失败: %v", err)
	}
	if after.ConfigurationVersion != connected.ConfigurationVersion+1 ||
		after.OAuthClientExists || after.OAuthClientConfigured ||
		after.ConnectionState != "disconnected" || after.ConnectionConfigured {
		t.Fatalf("OAuth 删除与断开不是原子结果: %+v", after)
	}
}

func TestExpiredTokenRefreshCannotReviveConcurrentDisconnect(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		close(refreshStarted)
		<-releaseRefresh
		_, _ = writer.Write([]byte(
			`{"code":0,"data":{"access_token":"new-token","refresh_token":"new-refresh","expires_in":7200}}`,
		))
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_FEISHU_DOCX_TOKEN_URL", server.URL)

	ctx := context.Background()
	service := NewService(cfg, db)
	service.httpClient = server.Client()
	if _, err = service.SaveOAuthClientConfig(
		ctx,
		auth.SystemUserID,
		"feishu-docx",
		OAuthClientConfigRequest{ClientID: "client", ClientSecret: "secret"},
	); err != nil {
		t.Fatalf("保存 OAuth Client 失败: %v", err)
	}
	if err = service.upsertConnection(ctx, connectionRecord{
		OwnerUserID: auth.SystemUserID,
		ConnectorID: "feishu-docx",
		State:       "connected",
		Credentials: `{"access_token":"old-token","refresh_token":"old-refresh","expires_at":"1"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入过期连接失败: %v", err)
	}

	refreshResult := make(chan error, 1)
	go func() {
		_, loadErr := service.LoadActiveConnection(ctx, auth.SystemUserID, "feishu-docx")
		refreshResult <- loadErr
	}()
	select {
	case <-refreshStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh 未开始")
	}
	if _, err = service.Disconnect(ctx, auth.SystemUserID, "feishu-docx"); err != nil {
		t.Fatalf("并发断开失败: %v", err)
	}
	close(releaseRefresh)
	select {
	case err = <-refreshResult:
		if !errors.Is(err, ErrConfigurationConflict) {
			t.Fatalf("过期 refresh 应被版本 CAS 拒绝，实际: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("refresh 未结束")
	}
	after, err := service.GetConfigurationState(ctx, auth.SystemUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取并发断开后状态失败: %v", err)
	}
	if after.ConnectionState != "disconnected" || after.ConnectionConfigured {
		t.Fatalf("refresh 不得复活已断开的连接: %+v", after)
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

func TestCustomMCPServerLifecycleKeepsSecretsEncrypted(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	secret := "secret-token"
	created, err := service.CreateCustomMCPServer(ctx, "owner-1", CustomMCPServerInput{
		Name:    "local_tools",
		Type:    "stdio",
		Command: "npx",
		Args:    []string{"-y", "local-mcp"},
		Env:     map[string]*string{"TOKEN": &secret},
	})
	if err != nil {
		t.Fatalf("创建自定义 MCP 失败: %v", err)
	}
	if !created.Enabled || created.ConfigurationState != customMCPConfigReady ||
		!IsCustomMCPConnectorID(created.ConnectorID) || created.Env["TOKEN"] != nil {
		t.Fatalf("创建结果未脱敏: %+v", created)
	}

	var plain string
	var encrypted sql.NullString
	if err = db.QueryRowContext(
		ctx,
		"SELECT credentials, credentials_encrypted FROM connector_connections WHERE owner_user_id = ? AND connector_id = ?",
		"owner-1",
		created.ConnectorID,
	).Scan(&plain, &encrypted); err != nil {
		t.Fatalf("读取加密配置失败: %v", err)
	}
	if plain != "__encrypted__" || !encrypted.Valid || strings.Contains(encrypted.String, secret) {
		t.Fatalf("自定义 MCP 秘密未加密: plain=%q encrypted=%q", plain, encrypted.String)
	}

	items, err := service.ListConnectors(ctx, "owner-1", "local_tools", "", "available")
	if err != nil {
		t.Fatalf("列出 Connector 失败: %v", err)
	}
	if len(items) != 1 || items[0].Kind != ConnectorKindCustomMCP || items[0].ConnectorID != created.ConnectorID {
		t.Fatalf("自定义 MCP 未进入 Connector 选择面: %+v", items)
	}

	disabled, err := service.SetCustomMCPServerEnabled(ctx, "owner-1", created.ConnectorID, false)
	if err != nil || disabled.Enabled {
		t.Fatalf("关闭自定义 MCP 失败: item=%+v err=%v", disabled, err)
	}
	items, err = service.ListConnectors(ctx, "owner-1", "local_tools", "", "available")
	if err != nil || len(items) != 0 {
		t.Fatalf("关闭后自定义 MCP 仍进入 Connector 选择面: items=%+v err=%v", items, err)
	}
	listed, err := service.ListCustomMCPServers(ctx, "owner-1")
	if err != nil || len(listed) != 1 || listed[0].Enabled {
		t.Fatalf("关闭后自定义 MCP 不应从管理目录消失: items=%+v err=%v", listed, err)
	}
	name, runtimeConfig, err := service.LoadActiveCustomMCPServer(ctx, "owner-1", created.ConnectorID)
	if err != nil || name != "" || runtimeConfig != nil {
		t.Fatalf("关闭后 runtime 仍加载自定义 MCP: name=%q config=%+v err=%v", name, runtimeConfig, err)
	}
	count, err := service.GetConnectedCount(ctx, "owner-1")
	if err != nil || count != 0 {
		t.Fatalf("关闭后连接器计数错误: count=%d err=%v", count, err)
	}
	updated, err := service.UpdateCustomMCPServer(ctx, "owner-1", created.ConnectorID, CustomMCPServerInput{
		Name:    "local_tools",
		Type:    "stdio",
		Command: "npx",
		Args:    []string{"-y", "new-local-mcp"},
		Env:     map[string]*string{"TOKEN": nil},
	})
	if err != nil {
		t.Fatalf("更新自定义 MCP 失败: %v", err)
	}
	if updated.Enabled || updated.Env["TOKEN"] != nil || updated.Args[1] != "new-local-mcp" {
		t.Fatalf("更新结果不正确: %+v", updated)
	}
	name, runtimeConfig, err = service.LoadActiveCustomMCPServer(
		ctx,
		"owner-1",
		created.ConnectorID,
	)
	if err != nil || name != "" || runtimeConfig != nil {
		t.Fatalf("编辑配置不应隐式重新开启 MCP: name=%q config=%+v err=%v", name, runtimeConfig, err)
	}
	if _, err = service.SetCustomMCPServerEnabled(ctx, "owner-1", created.ConnectorID, true); err != nil {
		t.Fatalf("重新开启自定义 MCP 失败: %v", err)
	}

	name, runtimeConfig, err = service.LoadActiveCustomMCPServer(
		ctx,
		"owner-1",
		created.ConnectorID,
	)
	if err != nil {
		t.Fatalf("读取 runtime 配置失败: %v", err)
	}
	env, _ := runtimeConfig["env"].(map[string]string)
	if name != "local_tools" || env["TOKEN"] != secret {
		t.Fatalf("runtime 配置未保留秘密: name=%q config=%+v", name, runtimeConfig)
	}

	otherSecret := "another"
	_, err = service.CreateCustomMCPServer(ctx, "owner-1", CustomMCPServerInput{
		Name:    "LOCAL_TOOLS",
		Type:    "stdio",
		Command: "node",
		Env:     map[string]*string{"TOKEN": &otherSecret},
	})
	if !errors.Is(err, ErrCustomMCPServerNameConflict) {
		t.Fatalf("同名 MCP 未被拒绝: %v", err)
	}

	if err = service.DeleteCustomMCPServer(ctx, "owner-1", created.ConnectorID); err != nil {
		t.Fatalf("删除自定义 MCP 失败: %v", err)
	}
	name, runtimeConfig, err = service.LoadActiveCustomMCPServer(
		ctx,
		"owner-1",
		created.ConnectorID,
	)
	if err != nil || name != "" || runtimeConfig != nil {
		t.Fatalf("删除后仍可读取 runtime 配置: name=%q config=%+v err=%v", name, runtimeConfig, err)
	}
}

func TestListConnectorsKeepsCatalogWhenLegacyCustomMCPPayloadCannotDecrypt(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err = db.Exec(`
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, enabled, credentials,
    credentials_encrypted, auth_type
) VALUES (?, ?, 'connected', TRUE, '__encrypted__', ?, 'custom_mcp')`,
		"owner-1",
		"custom-mcp:legacy-key",
		"v1:payload-from-an-unavailable-legacy-key",
	); err != nil {
		t.Fatal(err)
	}

	service := NewService(cfg, db)
	items, err := service.ListConnectors(context.Background(), "owner-1", "github", "", "")
	if err != nil {
		t.Fatalf("内置 Connector 目录不应被旧 MCP 密文阻断: %v", err)
	}
	if len(items) != 1 || items[0].ConnectorID != "github" {
		t.Fatalf("内置 Connector 目录投影错误: %+v", items)
	}
	legacy, err := service.ListCustomMCPServers(context.Background(), "owner-1")
	if err != nil || len(legacy) != 1 || legacy[0].ConfigurationState != customMCPConfigRecovery {
		t.Fatalf("旧密文应投影为可恢复记录: items=%+v err=%v", legacy, err)
	}
	if _, err = service.SetCustomMCPServerEnabled(
		context.Background(),
		"owner-1",
		legacy[0].ConnectorID,
		false,
	); !errors.Is(err, ErrCustomMCPServerRecoveryRequired) {
		t.Fatalf("恢复前不应允许切换可用性: %v", err)
	}
	recovered, err := service.UpdateCustomMCPServer(
		context.Background(),
		"owner-1",
		legacy[0].ConnectorID,
		CustomMCPServerInput{
			Name:    "recovered_tools",
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "recovered-mcp"},
		},
	)
	if err != nil || recovered.ConfigurationState != customMCPConfigReady || !recovered.Enabled {
		t.Fatalf("完整配置应覆盖无法解密的旧密文: item=%+v err=%v", recovered, err)
	}
	items, err = service.ListConnectors(context.Background(), "owner-1", "recovered_tools", "", "")
	if err != nil || len(items) != 1 || items[0].ConnectorID != recovered.ConnectorID {
		t.Fatalf("恢复后的 MCP 应重新进入选择目录: items=%+v err=%v", items, err)
	}
}

func TestDiscoverRemoteCustomMCPTools(t *testing.T) {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "catalog-server", Title: "Catalog Server", Version: "2.1.0"},
		&mcp.ServerOptions{Instructions: "Use search for indexed records."},
	)
	server.AddTool(&mcp.Tool{
		Name:        "search_records",
		Title:       "Search records",
		Description: "Find records by query. [Phase A]",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{"type": "integer", "description": "Maximum results"},
				"query": map[string]any{"type": "string", "description": "Search query"},
			},
			"required": []string{"query"},
		},
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, nil)

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	)
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer inspection-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(writer, request)
	}))
	defer httpServer.Close()

	catalog, err := discoverRemoteCustomMCPTools(context.Background(), storedCustomMCPServer{
		Enabled:     true,
		Name:        "catalog",
		Type:        "http",
		URL:         httpServer.URL,
		AuthType:    customMCPAuthBearer,
		BearerToken: "inspection-token",
	})
	if err != nil {
		t.Fatalf("探测远程 MCP 工具失败: %v", err)
	}
	if catalog.InspectionState != "connected" || !catalog.SupportsTools ||
		catalog.ServerName != "catalog-server" || catalog.ServerTitle != "Catalog Server" ||
		catalog.ServerVersion != "2.1.0" || len(catalog.Tools) != 1 {
		t.Fatalf("远程 MCP 基础信息或工具目录错误: %+v", catalog)
	}
	tool := catalog.Tools[0]
	if tool.Name != "search_records" || tool.Title != "Search records" || !tool.ReadOnly ||
		tool.Description != "Find records by query. [Phase A]" ||
		len(tool.Arguments) != 2 || tool.Arguments[1].Name != "query" || !tool.Arguments[1].Required {
		t.Fatalf("工具投影错误: %+v", tool)
	}
}

func TestDiscoverRemoteCustomMCPToolsDoesNotFollowRedirects(t *testing.T) {
	var destinationReached atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		destinationReached.Store(true)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		http.Redirect(writer, request, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	_, err := discoverRemoteCustomMCPTools(context.Background(), storedCustomMCPServer{
		Enabled:     true,
		Name:        "redirecting",
		Type:        "http",
		URL:         redirect.URL,
		AuthType:    customMCPAuthBearer,
		BearerToken: "must-not-leak",
	})
	if err == nil {
		t.Fatal("重定向 MCP 探测应失败")
	}
	if destinationReached.Load() {
		t.Fatal("MCP 探测不应把认证请求转发到重定向目标")
	}
}

func TestCreateCustomMCPServerRejectsInvalidRuntimeConfig(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = NewService(cfg, db).CreateCustomMCPServer(
		context.Background(),
		"owner-1",
		CustomMCPServerInput{Name: "broken", Type: "stdio"},
	)
	if !errors.Is(err, ErrCustomMCPServerInvalid) {
		t.Fatalf("无效配置错误 = %v, want ErrCustomMCPServerInvalid", err)
	}
}

func TestCustomMCPBearerAuthenticationIsEncryptedAndRedacted(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	token := "remote-secret"
	created, err := service.CreateCustomMCPServer(ctx, "owner-1", CustomMCPServerInput{
		Name:        "remote_tools",
		Type:        "http",
		URL:         "https://mcp.example.com/mcp",
		AuthType:    customMCPAuthBearer,
		BearerToken: &token,
	})
	if err != nil {
		t.Fatalf("创建 Bearer MCP 失败: %v", err)
	}
	if created.AuthType != customMCPAuthBearer || created.BearerToken != nil {
		t.Fatalf("Bearer 配置未脱敏: %+v", created)
	}

	var encrypted sql.NullString
	if err = db.QueryRowContext(
		ctx,
		"SELECT credentials_encrypted FROM connector_connections WHERE owner_user_id = ? AND connector_id = ?",
		"owner-1",
		created.ConnectorID,
	).Scan(&encrypted); err != nil {
		t.Fatalf("读取加密配置失败: %v", err)
	}
	if !encrypted.Valid || strings.Contains(encrypted.String, token) {
		t.Fatalf("Bearer Token 未加密: %q", encrypted.String)
	}

	_, runtimeConfig, err := service.LoadActiveCustomMCPServer(ctx, "owner-1", created.ConnectorID)
	if err != nil {
		t.Fatalf("读取 runtime 配置失败: %v", err)
	}
	headers, _ := runtimeConfig["headers"].(map[string]string)
	if headers["Authorization"] != "Bearer "+token {
		t.Fatalf("Bearer Token 未进入 Authorization header: %+v", runtimeConfig)
	}

	updated, err := service.UpdateCustomMCPServer(ctx, "owner-1", created.ConnectorID, CustomMCPServerInput{
		Name:     "remote_tools",
		Type:     "http",
		URL:      "https://mcp.example.com/mcp",
		AuthType: customMCPAuthBearer,
	})
	if err != nil || updated.BearerToken != nil {
		t.Fatalf("更新时未保留脱敏 Bearer Token: item=%+v err=%v", updated, err)
	}
	_, runtimeConfig, err = service.LoadActiveCustomMCPServer(ctx, "owner-1", created.ConnectorID)
	if err != nil {
		t.Fatalf("再次读取 runtime 配置失败: %v", err)
	}
	headers, _ = runtimeConfig["headers"].(map[string]string)
	if headers["Authorization"] != "Bearer "+token {
		t.Fatalf("更新后 Bearer Token 未保留: %+v", runtimeConfig)
	}
}

func TestCustomMCPUpdateAddsBearerAuthentication(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	created, err := service.CreateCustomMCPServer(ctx, "owner-1", CustomMCPServerInput{
		Name:     "remote_without_auth",
		Type:     "http",
		URL:      "https://mcp.example.com/mcp",
		AuthType: customMCPAuthNone,
	})
	if err != nil {
		t.Fatalf("创建无认证 MCP 失败: %v", err)
	}

	token := "new-secret"
	updated, err := service.UpdateCustomMCPServer(ctx, "owner-1", created.ConnectorID, CustomMCPServerInput{
		Name:        "remote_without_auth",
		Type:        "http",
		URL:         "https://mcp.example.com/mcp",
		AuthType:    customMCPAuthBearer,
		BearerToken: &token,
	})
	if err != nil {
		t.Fatalf("增加 Bearer Token 失败: %v", err)
	}
	if updated.AuthType != customMCPAuthBearer || updated.BearerToken != nil {
		t.Fatalf("更新响应未正确脱敏: %+v", updated)
	}

	_, runtimeConfig, err := service.LoadActiveCustomMCPServer(
		ctx,
		"owner-1",
		created.ConnectorID,
	)
	if err != nil {
		t.Fatalf("读取更新后的 runtime 配置失败: %v", err)
	}
	headers, _ := runtimeConfig["headers"].(map[string]string)
	if headers["Authorization"] != "Bearer "+token {
		t.Fatalf("新增 Bearer Token 未进入 runtime header: %+v", runtimeConfig)
	}

	var encrypted sql.NullString
	if err = db.QueryRowContext(
		ctx,
		"SELECT credentials_encrypted FROM connector_connections WHERE owner_user_id = ? AND connector_id = ?",
		"owner-1",
		created.ConnectorID,
	).Scan(&encrypted); err != nil {
		t.Fatalf("读取更新后的加密配置失败: %v", err)
	}
	if !encrypted.Valid || strings.Contains(encrypted.String, token) {
		t.Fatalf("新增 Bearer Token 未加密: %q", encrypted.String)
	}
}

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
