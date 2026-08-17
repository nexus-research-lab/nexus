package connectors

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
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
	if !IsCustomMCPConnectorID(created.ConnectorID) || created.Env["TOKEN"] != nil {
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
	if updated.Env["TOKEN"] != nil || updated.Args[1] != "new-local-mcp" {
		t.Fatalf("更新结果不正确: %+v", updated)
	}

	name, runtimeConfig, err := service.LoadActiveCustomMCPServer(
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
