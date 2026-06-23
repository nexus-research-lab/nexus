package connectors

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/service/auth"

	_ "modernc.org/sqlite"
)

func TestServiceDesktopGitHubDeviceFlowUsesPublicClientID(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	cfg.AppMode = "desktop"
	cfg.ConnectorGitHubClientSecret = ""
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	tokenPollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Fatalf("解析 GitHub device 请求失败: %v", err)
		}
		if request.Form.Get("client_id") != cfg.ConnectorGitHubClientID {
			t.Fatalf("device flow 未使用公开 client_id: %v", request.Form)
		}
		if request.Form.Get("client_secret") != "" {
			t.Fatalf("device flow 不应发送 client_secret: %v", request.Form)
		}
		switch request.URL.Path {
		case "/device":
			_, _ = writer.Write([]byte(`{"device_code":"device-code","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":1}`))
		case "/token":
			tokenPollCount++
			if request.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
				t.Fatalf("grant_type 不正确: %v", request.Form)
			}
			if request.Form.Get("device_code") != "device-code" {
				t.Fatalf("device_code 不正确: %v", request.Form)
			}
			if tokenPollCount == 1 {
				_, _ = writer.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"github-device-token","scope":"repo","token_type":"bearer"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL", server.URL+"/device")
	t.Setenv("NEXUS_CONNECTOR_GITHUB_TOKEN_URL", server.URL+"/token")

	service := NewService(cfg, db)
	service.httpClient = server.Client()
	ctx := context.Background()

	items, err := service.ListConnectors(ctx, auth.SystemUserID, "github", "", "")
	if err != nil {
		t.Fatalf("列出连接器失败: %v", err)
	}
	if len(items) != 1 || !items[0].IsConfigured {
		t.Fatalf("桌面 GitHub 只配置 client_id 时应可用: %+v", items)
	}

	start, err := service.StartDeviceAuth(ctx, auth.SystemUserID, "github")
	if err != nil {
		t.Fatalf("启动 GitHub device flow 失败: %v", err)
	}
	if start.UserCode != "ABCD-1234" || start.DeviceCode != "device-code" {
		t.Fatalf("device flow 启动结果不正确: %+v", start)
	}

	pending, err := service.PollDeviceAuth(ctx, auth.SystemUserID, "github", start.DeviceCode)
	if err != nil {
		t.Fatalf("轮询 GitHub device flow 失败: %v", err)
	}
	if pending.Status != deviceAuthStatusPending {
		t.Fatalf("首次轮询应为 pending: %+v", pending)
	}

	connected, err := service.PollDeviceAuth(ctx, auth.SystemUserID, "github", start.DeviceCode)
	if err != nil {
		t.Fatalf("完成 GitHub device flow 失败: %v", err)
	}
	if connected.Status != deviceAuthStatusConnected || connected.Connector == nil || connected.Connector.ConnectionState != "connected" {
		t.Fatalf("device flow 未完成连接: %+v", connected)
	}
	snapshot, err := service.LoadActiveConnection(ctx, auth.SystemUserID, "github")
	if err != nil {
		t.Fatalf("读取 GitHub 连接失败: %v", err)
	}
	if snapshot == nil || snapshot.AccessToken != "github-device-token" {
		t.Fatalf("GitHub token 未保存: %+v", snapshot)
	}
}

func TestServiceDesktopGitHubDeviceFlowDisabledMessage(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	cfg.AppMode = "desktop"
	cfg.ConnectorGitHubClientSecret = ""
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, `{"error":"device_flow_disabled","error_description":"Device Flow must be explicitly enabled for this App"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL", server.URL)

	service := NewService(cfg, db)
	service.httpClient = server.Client()
	_, err = service.StartDeviceAuth(context.Background(), auth.SystemUserID, "github")
	if err == nil || !strings.Contains(err.Error(), "未启用 Device Flow") {
		t.Fatalf("device_flow_disabled 应转成可读错误，实际: %v", err)
	}
}
