package connectors

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/service/auth"

	_ "modernc.org/sqlite"
)

func TestServiceListsConnectorsAndBuildsAuthURL(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()

	items, err := service.ListConnectors(ctx, auth.SystemUserID, "github", "", "")
	if err != nil {
		t.Fatalf("列出连接器失败: %v", err)
	}
	if len(items) != 1 || items[0].ConnectorID != "github" {
		t.Fatalf("连接器过滤结果不正确: %+v", items)
	}
	if !items[0].IsConfigured {
		t.Fatalf("GitHub 连接器应视为已配置: %+v", items[0])
	}

	authURL, err := service.GetAuthURL(ctx, auth.SystemUserID, "github", "", nil)
	if err != nil {
		t.Fatalf("生成授权地址失败: %v", err)
	}
	parsedURL, err := url.Parse(authURL.AuthURL)
	if err != nil {
		t.Fatalf("解析授权地址失败: %v", err)
	}
	if parsedURL.Query().Get("client_id") != cfg.ConnectorGitHubClientID {
		t.Fatalf("client_id 未写入授权地址: %s", authURL.AuthURL)
	}
	if strings.TrimSpace(authURL.State) == "" {
		t.Fatalf("state 不能为空: %+v", authURL)
	}

	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"access_token":"token"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}
	count, err := service.GetConnectedCount(ctx, auth.SystemUserID)
	if err != nil {
		t.Fatalf("读取已连接数量失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("已连接数量不正确: got=%d want=1", count)
	}
}

func TestServiceScopesConnectionStateByOwner(t *testing.T) {
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
		OwnerUserID: "owner-a",
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"access_token":"owner-a-token"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入 owner-a 连接状态失败: %v", err)
	}
	if err = service.upsertConnection(ctx, connectionRecord{
		OwnerUserID: "owner-b",
		ConnectorID: "github",
		State:       "disconnected",
		Credentials: "",
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入 owner-b 连接状态失败: %v", err)
	}

	countA, err := service.GetConnectedCount(ctx, "owner-a")
	if err != nil {
		t.Fatalf("读取 owner-a 已连接数量失败: %v", err)
	}
	countB, err := service.GetConnectedCount(ctx, "owner-b")
	if err != nil {
		t.Fatalf("读取 owner-b 已连接数量失败: %v", err)
	}
	if countA != 1 || countB != 0 {
		t.Fatalf("连接数量应按 owner 隔离: owner-a=%d owner-b=%d", countA, countB)
	}

	itemsA, err := service.ListConnectors(ctx, "owner-a", "github", "", "")
	if err != nil {
		t.Fatalf("列出 owner-a connector 失败: %v", err)
	}
	itemsB, err := service.ListConnectors(ctx, "owner-b", "github", "", "")
	if err != nil {
		t.Fatalf("列出 owner-b connector 失败: %v", err)
	}
	if len(itemsA) != 1 || itemsA[0].ConnectionState != "connected" {
		t.Fatalf("owner-a 应看到 connected: %+v", itemsA)
	}
	if len(itemsB) != 1 || itemsB[0].ConnectionState != "disconnected" {
		t.Fatalf("owner-b 应看到 disconnected: %+v", itemsB)
	}

	snapshotA, err := service.LoadActiveConnection(ctx, "owner-a", "github")
	if err != nil {
		t.Fatalf("读取 owner-a active connector 失败: %v", err)
	}
	snapshotB, err := service.LoadActiveConnection(ctx, "owner-b", "github")
	if err != nil {
		t.Fatalf("读取 owner-b active connector 失败: %v", err)
	}
	if snapshotA == nil || snapshotA.AccessToken != "owner-a-token" {
		t.Fatalf("owner-a active connector 不正确: %+v", snapshotA)
	}
	if snapshotB != nil {
		t.Fatalf("owner-b 不应读到 owner-a token: %+v", snapshotB)
	}

	activeA, err := service.ListActiveConnections(ctx, "owner-a")
	if err != nil {
		t.Fatalf("列出 owner-a active connectors 失败: %v", err)
	}
	activeB, err := service.ListActiveConnections(ctx, "owner-b")
	if err != nil {
		t.Fatalf("列出 owner-b active connectors 失败: %v", err)
	}
	if len(activeA) != 1 || activeA[0].ConnectorID != "github" || len(activeB) != 0 {
		t.Fatalf("active connector 列表未按 owner 隔离: owner-a=%+v owner-b=%+v", activeA, activeB)
	}
}
