// INPUT: 已完成 schema/bootstrap 的用户表与 Linux launcher 配置。
// OUTPUT: 每个产品 owner 的稳定 OS UID/GID、私有组和既有用户目录 ACL。
// POS: 启动期可重复执行的 runtime identity 同步；容器重建后会恢复 /etc 账号。
package migration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/runtime/workspaceisolation"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// RunRuntimeIdentitySync 让数据库用户、root-owned registry 与 OS 账号保持一致。
func RunRuntimeIdentitySync(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) error {
	mode, err := workspaceisolation.NormalizeMode(cfg.RuntimeIsolationMode)
	if err != nil {
		return err
	}
	if mode != workspaceisolation.ModeEnforce {
		return nil
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("runtime isolation enforce 只支持 Linux")
	}
	launcherPath := filepath.Clean(strings.TrimSpace(cfg.RuntimeLauncherPath))
	if !filepath.IsAbs(launcherPath) {
		return fmt.Errorf("runtime launcher 必须是绝对路径: %q", cfg.RuntimeLauncherPath)
	}
	hostLayoutOutput, err := runRuntimeIsolationLauncher(ctx, launcherPath, "ensure-host")
	if err != nil {
		return fmt.Errorf("确认 Nexus host 文件权限: %w", err)
	}
	if err = verifyRuntimeLauncherStateRoot(hostLayoutOutput, appfs.StateRoot()); err != nil {
		return err
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("打开 runtime identity 同步数据库: %w", err)
	}
	defer db.Close()
	owners, err := loadRuntimeIdentityOwners(ctx, db)
	if err != nil {
		return err
	}
	for _, ownerUserID := range owners {
		output, commandErr := runRuntimeIsolationLauncher(
			ctx,
			launcherPath,
			"ensure-user",
			"--owner",
			ownerUserID,
		)
		if commandErr != nil {
			return fmt.Errorf("同步 owner %s runtime identity: %w", ownerUserID, commandErr)
		}
		if len(bytes.TrimSpace(output)) == 0 {
			return fmt.Errorf("同步 owner %s runtime identity: launcher 未返回身份", ownerUserID)
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("runtime OS identity 同步完成",
		"owners", len(owners),
		"isolation_mode", string(mode),
	)
	return nil
}

func verifyRuntimeLauncherStateRoot(output []byte, expected string) error {
	var result struct {
		Ready     bool   `json:"ready"`
		StateRoot string `json:"state_root"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("解析 runtime launcher host layout: %w", err)
	}
	if !result.Ready {
		return errors.New("runtime launcher host layout 未就绪")
	}
	launcherRoot := filepath.Clean(strings.TrimSpace(result.StateRoot))
	expectedRoot := filepath.Clean(strings.TrimSpace(expected))
	if resolved, err := filepath.EvalSymlinks(launcherRoot); err == nil {
		launcherRoot = filepath.Clean(resolved)
	}
	if resolved, err := filepath.EvalSymlinks(expectedRoot); err == nil {
		expectedRoot = filepath.Clean(resolved)
	}
	if launcherRoot != expectedRoot {
		return fmt.Errorf("runtime launcher state root 不匹配: launcher=%q server=%q", launcherRoot, expectedRoot)
	}
	return nil
}

func runRuntimeIsolationLauncher(
	ctx context.Context,
	launcherPath string,
	arguments ...string,
) ([]byte, error) {
	command := exec.CommandContext(ctx, launcherPath, arguments...)
	command.Env = []string{
		"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
		"LANG=C.UTF-8",
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		detail = err.Error()
	}
	return nil, errors.New(detail)
}

func loadRuntimeIdentityOwners(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT owner_user_id FROM owner_profiles ORDER BY owner_user_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取 runtime identity 用户: %w", err)
	}
	defer rows.Close()
	seen := map[string]struct{}{authctx.SystemUserID: {}}
	for rows.Next() {
		var ownerUserID string
		if err = rows.Scan(&ownerUserID); err != nil {
			return nil, fmt.Errorf("扫描 runtime identity 用户: %w", err)
		}
		if ownerUserID = strings.TrimSpace(ownerUserID); ownerUserID != "" {
			seen[ownerUserID] = struct{}{}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 runtime identity 用户: %w", err)
	}
	owners := make([]string, 0, len(seen))
	for ownerUserID := range seen {
		owners = append(owners, ownerUserID)
	}
	sort.Strings(owners)
	return owners, nil
}
