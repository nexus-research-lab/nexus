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
