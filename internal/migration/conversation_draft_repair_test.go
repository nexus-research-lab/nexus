package migration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"github.com/pressly/goose/v3"
)

func TestDesktopLegacyConversationDraftRepairRunsOnlyOnce(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "repair.marker")
	cfg := config.Config{AppMode: "desktop", DatabaseDriver: "sqlite"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	calls := 0
	apply := func(
		context.Context,
		config.Config,
		*slog.Logger,
	) (legacyConversationDraftRepairSummary, error) {
		calls++
		return legacyConversationDraftRepairSummary{OwnersScanned: 2}, nil
	}

	if err := runDesktopLegacyConversationDraftRepairOnce(
		t.Context(),
		cfg,
		logger,
		markerPath,
		apply,
	); err != nil {
		t.Fatalf("首次修复失败: %v", err)
	}
	if calls != 1 {
		t.Fatalf("首次 apply 次数 = %d, want 1", calls)
	}
	assertOneTimeRepairMarker(t, markerPath, oneTimeRepairStateCompleted)

	if err := runDesktopLegacyConversationDraftRepairOnce(
		t.Context(),
		cfg,
		logger,
		markerPath,
		apply,
	); err != nil {
		t.Fatalf("完成后重复检查失败: %v", err)
	}
	if calls != 1 {
		t.Fatalf("completed marker 后不应重跑，apply 次数 = %d", calls)
	}
}

func TestDesktopLegacyConversationDraftRepairStartedMarkerSuppressesRetry(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "repair.marker")
	cfg := config.Config{AppMode: "desktop", DatabaseDriver: "sqlite"}
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))
	calls := 0
	firstFailure := errors.New("simulated repair failure")
	apply := func(
		context.Context,
		config.Config,
		*slog.Logger,
	) (legacyConversationDraftRepairSummary, error) {
		calls++
		return legacyConversationDraftRepairSummary{}, firstFailure
	}

	err := runDesktopLegacyConversationDraftRepairOnce(
		t.Context(),
		cfg,
		logger,
		markerPath,
		apply,
	)
	if !errors.Is(err, firstFailure) {
		t.Fatalf("首次失败 = %v, want wrapped simulated failure", err)
	}
	assertOneTimeRepairMarker(t, markerPath, oneTimeRepairStateStarted)

	if err = runDesktopLegacyConversationDraftRepairOnce(
		t.Context(),
		cfg,
		logger,
		markerPath,
		apply,
	); err != nil {
		t.Fatalf("started marker 应安全跳过自动重跑: %v", err)
	}
	if calls != 1 {
		t.Fatalf("started marker 后不应重跑，apply 次数 = %d", calls)
	}
	if !strings.Contains(logOutput.String(), "跳过自动重跑") {
		t.Fatalf("started marker 应记录维护提示: %s", logOutput.String())
	}
}

func TestDesktopLegacyConversationDraftRepairSkipsUnsafeDeployments(t *testing.T) {
	for name, cfg := range map[string]config.Config{
		"sqlite server": {
			AppMode:        "server",
			DatabaseDriver: "sqlite",
		},
		"postgres desktop": {
			AppMode:        "desktop",
			DatabaseDriver: "postgres",
		},
	} {
		t.Run(name, func(t *testing.T) {
			markerPath := filepath.Join(t.TempDir(), "repair.marker")
			called := false
			err := runDesktopLegacyConversationDraftRepairOnce(
				t.Context(),
				cfg,
				discardMigrationLogger(),
				markerPath,
				func(
					context.Context,
					config.Config,
					*slog.Logger,
				) (legacyConversationDraftRepairSummary, error) {
					called = true
					return legacyConversationDraftRepairSummary{}, nil
				},
			)
			if err != nil {
				t.Fatalf("不安全部署应直接跳过: %v", err)
			}
			if called {
				t.Fatal("不安全部署不应执行 apply")
			}
			if _, statErr := os.Stat(markerPath); !os.IsNotExist(statErr) {
				t.Fatalf("跳过时不应创建 marker: %v", statErr)
			}
		})
	}
}

func TestDesktopLegacyConversationDraftRepairCoversEveryRoomOwner(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	cfg := config.Config{
		AppMode:        "desktop",
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(stateRoot, "app", "data", "nexus.db"),
		WorkspacePath:  filepath.Join(stateRoot, "users"),
		GoalEnabled:    true,
	}
	migrateConversationDraftRepairDatabase(t, cfg)

	db, err := storage.OpenMigrationDB(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库: %v", err)
	}
	agentService := agentsvc.NewService(
		cfg,
		agentrepo.NewSQLRepository(cfg.DatabaseDriver, db),
	)
	roomService := roomsvc.NewService(
		cfg,
		agentService,
		roomrepo.NewSQLRepository(cfg.DatabaseDriver, db),
	)
	history := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	type fixture struct {
		ownerID      string
		roomID       string
		occupiedID   string
		olderEmptyID string
		keeperID     string
	}
	fixtures := make([]fixture, 0, 2)
	for index, ownerID := range []string{"owner-a", "owner-b"} {
		ownerContext := authctx.WithPrincipal(t.Context(), &authctx.Principal{
			UserID:     ownerID,
			Username:   ownerID,
			Role:       authctx.RoleOwner,
			AuthMethod: "test",
		})
		agentValue, createErr := agentService.CreateAgent(ownerContext, protocol.CreateRequest{
			Name: "迁移测试助手 " + ownerID,
		})
		if createErr != nil {
			t.Fatalf("owner %s 创建 Agent: %v", ownerID, createErr)
		}
		mainContext, createErr := roomService.CreateRoom(ownerContext, protocol.CreateRoomRequest{
			AgentIDs: []string{agentValue.AgentID},
			Name:     "迁移测试 Room " + ownerID,
		})
		if createErr != nil {
			t.Fatalf("owner %s 创建 Room: %v", ownerID, createErr)
		}
		if _, createErr = db.ExecContext(
			t.Context(),
			`UPDATE conversations SET is_draft = 0 WHERE room_id = ?`,
			mainContext.Room.ID,
		); createErr != nil {
			t.Fatalf("owner %s 准备旧版本主会话: %v", ownerID, createErr)
		}
		olderEmpty, createErr := roomService.CreateConversation(
			ownerContext,
			mainContext.Room.ID,
			protocol.CreateConversationRequest{Title: "可能已生成过的标题"},
		)
		if createErr != nil {
			t.Fatalf("owner %s 创建旧空白 Session: %v", ownerID, createErr)
		}
		if _, createErr = db.ExecContext(
			t.Context(),
			`UPDATE conversations SET is_draft = 0 WHERE room_id = ?`,
			mainContext.Room.ID,
		); createErr != nil {
			t.Fatalf("owner %s 准备旧版本空白会话: %v", ownerID, createErr)
		}
		keeper, createErr := roomService.CreateConversation(
			ownerContext,
			mainContext.Room.ID,
			protocol.CreateConversationRequest{Title: "标题不能代替用户输入证据"},
		)
		if createErr != nil {
			t.Fatalf("owner %s 创建新空白 Session: %v", ownerID, createErr)
		}
		if err = history.AppendInlineMessage(ownerID, mainContext.Conversation.ID, protocol.Message{
			"message_id": "user-" + ownerID,
			"role":       "user",
			"content":    "已经输入过",
			"timestamp":  int64(index + 1),
		}); err != nil {
			t.Fatalf("owner %s 写入 canonical 用户输入: %v", ownerID, err)
		}
		if _, err = db.ExecContext(t.Context(), `
UPDATE conversations
SET is_draft = 0,
    created_at = CASE id
        WHEN ? THEN '2026-01-01 09:00:00'
        WHEN ? THEN '2026-01-01 10:00:00'
        ELSE '2026-01-01 11:00:00'
    END,
    updated_at = created_at,
    last_activity_at = created_at
WHERE room_id = ?`,
			mainContext.Conversation.ID,
			olderEmpty.Conversation.ID,
			mainContext.Room.ID,
		); err != nil {
			t.Fatalf("owner %s 准备旧版本 fixture: %v", ownerID, err)
		}
		fixtures = append(fixtures, fixture{
			ownerID:      ownerID,
			roomID:       mainContext.Room.ID,
			occupiedID:   mainContext.Conversation.ID,
			olderEmptyID: olderEmpty.Conversation.ID,
			keeperID:     keeper.Conversation.ID,
		})
	}
	if err = db.Close(); err != nil {
		t.Fatalf("关闭 fixture 数据库: %v", err)
	}

	if err = RunDesktopLegacyConversationDraftRepair(
		t.Context(),
		cfg,
		discardMigrationLogger(),
	); err != nil {
		t.Fatalf("执行桌面升级修复: %v", err)
	}
	assertOneTimeRepairMarker(
		t,
		legacyConversationDraftRepairMarkerPath(),
		oneTimeRepairStateCompleted,
	)

	db, err = storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("重新打开测试数据库: %v", err)
	}
	defer db.Close()
	for _, item := range fixtures {
		var (
			conversationCount int
			draftCount        int
			keeperDraft       bool
			occupiedCount     int
			olderEmptyCount   int
		)
		if err = db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*), SUM(CASE WHEN is_draft THEN 1 ELSE 0 END)
FROM conversations WHERE room_id = ?`,
			item.roomID,
		).Scan(&conversationCount, &draftCount); err != nil {
			t.Fatalf("owner %s 读取收口结果: %v", item.ownerID, err)
		}
		if conversationCount != 2 || draftCount != 1 {
			t.Fatalf(
				"owner %s 收口结果 = conversations:%d drafts:%d, want 2/1",
				item.ownerID,
				conversationCount,
				draftCount,
			)
		}
		if err = db.QueryRowContext(
			t.Context(),
			`SELECT is_draft FROM conversations WHERE id = ?`,
			item.keeperID,
		).Scan(&keeperDraft); err != nil || !keeperDraft {
			t.Fatalf("owner %s keeper 必须成为 draft: draft=%v err=%v", item.ownerID, keeperDraft, err)
		}
		if err = db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM conversations WHERE id = ?`,
			item.occupiedID,
		).Scan(&occupiedCount); err != nil || occupiedCount != 1 {
			t.Fatalf("owner %s 有用户输入的 Session 必须保留: count=%d err=%v", item.ownerID, occupiedCount, err)
		}
		if err = db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM conversations WHERE id = ?`,
			item.olderEmptyID,
		).Scan(&olderEmptyCount); err != nil || olderEmptyCount != 0 {
			t.Fatalf("owner %s 旧空白 Session 必须删除: count=%d err=%v", item.ownerID, olderEmptyCount, err)
		}
	}
}

func migrateConversationDraftRepairDatabase(t *testing.T, cfg config.Config) {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开 migration 数据库: %v", err)
	}
	defer db.Close()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言: %v", err)
	}
	if err = goose.Up(db, providerRecoveryMigrationDir(t)); err != nil {
		t.Fatalf("执行基础 migration: %v", err)
	}
}

func assertOneTimeRepairMarker(t *testing.T, markerPath string, expected string) {
	t.Helper()
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("读取 repair marker %q: %v", markerPath, err)
	}
	if string(content) != expected+"\n" {
		t.Fatalf("repair marker %q = %q, want %q", markerPath, content, expected+"\n")
	}
	info, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("读取 repair marker 权限: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("repair marker 权限 = %o, want 600", info.Mode().Perm())
	}
}

func TestListRoomOwnerUserIDsRejectsUnownedRoom(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "owners.db"),
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开 owners 数据库: %v", err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE rooms (owner_user_id TEXT)`); err != nil {
		t.Fatalf("创建 rooms fixture: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO rooms (owner_user_id) VALUES ('owner-b'), ('owner-a'), ('owner-a')`); err != nil {
		t.Fatalf("插入 owner fixture: %v", err)
	}
	owners, err := listRoomOwnerUserIDs(t.Context(), db)
	if err != nil {
		t.Fatalf("读取 owner: %v", err)
	}
	if strings.Join(owners, ",") != "owner-a,owner-b" {
		t.Fatalf("owner 列表 = %v", owners)
	}
	if _, err = db.Exec(`INSERT INTO rooms (owner_user_id) VALUES ('')`); err != nil {
		t.Fatalf("插入无 owner fixture: %v", err)
	}
	if _, err = listRoomOwnerUserIDs(t.Context(), db); err == nil {
		t.Fatal("缺少 owner_user_id 时必须停止自动修复")
	}
}
