package connectors

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/service/auth"

	_ "modernc.org/sqlite"
)

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
