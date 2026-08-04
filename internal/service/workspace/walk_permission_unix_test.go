//go:build !windows

// 本文件在真实文件权限下回归不可读子树不会拖垮 workspace 列表与订阅。
package workspace

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestServiceSkipsUnreadableWorkspaceSubtree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 不受目录权限位限制")
	}
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := NewService(cfg, agentService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "权限容错助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(agentValue.WorkspacePath, "visible.md"), []byte("visible"), 0o644); err != nil {
		t.Fatalf("写入可见文件失败: %v", err)
	}
	privateDirectory := filepath.Join(agentValue.WorkspacePath, "memory")
	if err = os.Mkdir(privateDirectory, 0o700); err != nil {
		t.Fatalf("创建私有目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(privateDirectory, "secret.md"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("写入私有文件失败: %v", err)
	}
	if err = os.Chmod(privateDirectory, 0); err != nil {
		t.Fatalf("收紧私有目录失败: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(privateDirectory, 0o700) })

	files, err := workspaceService.ListFiles(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("不可读子树不应让文件列表失败: %v", err)
	}
	if !containsWorkspacePath(files, "visible.md") {
		t.Fatalf("文件列表丢失可读文件: %+v", files)
	}
	if containsWorkspacePath(files, "memory/secret.md") {
		t.Fatalf("文件列表不应穿过不可读子树: %+v", files)
	}

	token, err := workspaceService.SubscribeLive(ctx, agentValue.AgentID, func(LiveEvent) {})
	if err != nil {
		t.Fatalf("不可读子树不应让实时订阅失败: %v", err)
	}
	workspaceService.UnsubscribeLive(token)
}
