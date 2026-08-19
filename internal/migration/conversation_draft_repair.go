// INPUT: 已完成 v56 schema migration 的桌面 SQLite、Room owner 与 canonical 会话文件。
// OUTPUT: 每个 Room 至多保留一个无用户输入页，并把 keeper 修复为唯一 draft。
// POS: 桌面版本升级期的一次性兼容修复；先领取标记，再复用 Room 维护服务，失败不允许自动重扫新数据。
package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/sessionrepo"
)

const (
	legacyConversationDraftRepairName = "20260727_consolidate_legacy_no_user_input_conversations"
	oneTimeRepairStateStarted         = "started"
	oneTimeRepairStateCompleted       = "completed"
)

type legacyConversationDraftRepairSummary struct {
	OwnersScanned     int
	RoomsScanned      int
	ConversationsSeen int
	ConversationsGone int
	DraftRepairs      int
	UnknownPreserved  int
}

type legacyConversationDraftRepairApply func(
	context.Context,
	config.Config,
	*slog.Logger,
) (legacyConversationDraftRepairSummary, error)

// RunDesktopLegacyConversationDraftRepair 在桌面 SQLite 升级启动期执行一次兼容修复。
//
// started 与 completed 都会阻止后续自动重跑。若首次尝试失败，保留 started，
// 由显式维护命令继续处理，避免下一次启动误删升级后新建但尚未输入的 Session。
func RunDesktopLegacyConversationDraftRepair(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) error {
	return runDesktopLegacyConversationDraftRepairOnce(
		ctx,
		cfg,
		logger,
		legacyConversationDraftRepairMarkerPath(),
		applyDesktopLegacyConversationDraftRepair,
	)
}

func runDesktopLegacyConversationDraftRepairOnce(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
	markerPath string,
	apply legacyConversationDraftRepairApply,
) error {
	if !isDesktopSQLite(cfg) {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if apply == nil {
		return errors.New("legacy conversation draft repair apply function is nil")
	}

	claimed, previousState, err := claimOneTimeRepair(markerPath)
	if err != nil {
		return fmt.Errorf("领取历史空白 Session 修复标记: %w", err)
	}
	if !claimed {
		if previousState != oneTimeRepairStateCompleted {
			logger.Warn(
				"历史空白 Session 自动修复已有未完成尝试，跳过自动重跑；可使用 nexusctl 显式检查",
				"migration", legacyConversationDraftRepairName,
				"state", previousState,
			)
		}
		return nil
	}

	summary, err := apply(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf(
			"历史空白 Session 自动修复未完成，已保留 started 标记且不会阻塞启动: %w",
			err,
		)
	}
	if err = completeOneTimeRepair(markerPath); err != nil {
		return fmt.Errorf("提交历史空白 Session 修复完成标记: %w", err)
	}
	logger.Info(
		"历史空白 Session 一次性兼容修复完成",
		"migration", legacyConversationDraftRepairName,
		"owners_scanned", summary.OwnersScanned,
		"rooms_scanned", summary.RoomsScanned,
		"conversations_scanned", summary.ConversationsSeen,
		"conversations_deleted", summary.ConversationsGone,
		"draft_repairs", summary.DraftRepairs,
		"unknown_preserved", summary.UnknownPreserved,
	)
	return nil
}

func legacyConversationDraftRepairMarkerPath() string {
	return filepath.Join(appfs.AppDir(), ".migrations", legacyConversationDraftRepairName)
}

// claimOneTimeRepair 先以 O_EXCL 写入 started，确保任何中断都不会触发下一次全量重扫。
func claimOneTimeRepair(markerPath string) (bool, string, error) {
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o700); err != nil {
		return false, "", err
	}
	file, err := os.OpenFile(markerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		content, readErr := os.ReadFile(markerPath)
		if readErr != nil {
			// 文件存在本身就是已领取证据；读取失败也不能再次执行破坏性扫描。
			return false, "unreadable", nil
		}
		state := strings.TrimSpace(string(content))
		if state == "" {
			state = "unknown"
		}
		return false, state, nil
	}
	if err != nil {
		return false, "", err
	}
	if _, err = file.WriteString(oneTimeRepairStateStarted + "\n"); err != nil {
		_ = file.Close()
		return false, "", err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return false, "", err
	}
	if err = file.Close(); err != nil {
		return false, "", err
	}
	return true, oneTimeRepairStateStarted, nil
}

// completeOneTimeRepair 只更新诊断状态；即使更新中断，marker 仍存在并保持防重跑语义。
func completeOneTimeRepair(markerPath string) error {
	file, err := os.OpenFile(markerPath, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.WriteString(oneTimeRepairStateCompleted + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func applyDesktopLegacyConversationDraftRepair(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) (legacyConversationDraftRepairSummary, error) {
	summary := legacyConversationDraftRepairSummary{}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return summary, fmt.Errorf("打开修复数据库: %w", err)
	}
	defer db.Close()

	ownerUserIDs, err := listRoomOwnerUserIDs(ctx, db)
	if err != nil {
		return summary, err
	}
	summary.OwnersScanned = len(ownerUserIDs)

	agentService := agentsvc.NewService(
		cfg,
		agentrepo.NewSQLRepository(cfg.DatabaseDriver, db),
	)
	roomService := roomsvc.NewService(
		cfg,
		agentService,
		roomrepo.NewSQLRepository(cfg.DatabaseDriver, db),
	)
	runtimeManager := runtimectx.NewManager()
	sessionService := sessionsvc.NewService(
		cfg,
		agentService,
		sessionrepo.NewSQLRepository(cfg.DatabaseDriver, db),
	)
	sessionService.SetRuntimeManager(runtimeManager)
	roomService.SetSessionArtifactDeletionCoordinator(sessionService)
	goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	roomService.SetGoalCleaner(goalService)

	var repairErrors []error
	for _, ownerUserID := range ownerUserIDs {
		ownerContext := authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID:     ownerUserID,
			Username:   ownerUserID,
			Role:       authctx.RoleOwner,
			AuthMethod: "migration",
		})
		report, repairErr := roomService.PruneEmptyConversations(
			ownerContext,
			roomsvc.PruneEmptyConversationsOptions{Apply: true},
		)
		if repairErr != nil {
			repairErrors = append(
				repairErrors,
				fmt.Errorf("owner %s: %w", ownerUserID, repairErr),
			)
			continue
		}
		summary.RoomsScanned += report.RoomsScanned
		summary.ConversationsSeen += report.ConversationsScanned
		summary.ConversationsGone += report.Deleted
		successfulDraftRepairs := len(report.DraftRepairs) - report.DraftRepairFailed
		if successfulDraftRepairs > 0 {
			summary.DraftRepairs += successfulDraftRepairs
		}
		summary.UnknownPreserved += report.Unknown

		if report.DeleteFailed > 0 || report.DraftRepairFailed > 0 || len(report.Warnings) > 0 {
			repairErrors = append(
				repairErrors,
				fmt.Errorf(
					"owner %s: delete_failed=%d draft_repair_failed=%d warnings=%s",
					ownerUserID,
					report.DeleteFailed,
					report.DraftRepairFailed,
					strings.Join(report.Warnings, "; "),
				),
			)
		}
		logger.Info(
			"历史空白 Session owner scope 修复完成",
			"owner_user_id", ownerUserID,
			"rooms_scanned", report.RoomsScanned,
			"conversations_deleted", report.Deleted,
			"unknown_preserved", report.Unknown,
		)
	}
	return summary, errors.Join(repairErrors...)
}

func listRoomOwnerUserIDs(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
SELECT DISTINCT COALESCE(owner_user_id, '')
FROM rooms
ORDER BY COALESCE(owner_user_id, '') ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取 Room owner 列表: %w", err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var ownerUserID string
		if err = rows.Scan(&ownerUserID); err != nil {
			return nil, fmt.Errorf("扫描 Room owner: %w", err)
		}
		ownerUserID = strings.TrimSpace(ownerUserID)
		if ownerUserID == "" {
			return nil, errors.New("存在缺少 owner_user_id 的 Room，自动修复已停止")
		}
		result = append(result, ownerUserID)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 Room owner: %w", err)
	}
	return result, nil
}
