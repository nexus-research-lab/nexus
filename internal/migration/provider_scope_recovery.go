// INPUT: 桌面 App 的本地 SQLite 数据库、Agent runtime 归属与用户 preferences.json。
// OUTPUT: 恢复 00018 scope migration 前遗留 Provider 的用户私有副本，并保留无法判定归属的公共 fallback。
// POS: 仅在桌面 App 启动时执行的一次性数据补偿；不参与 Web/服务器部署，也不改写 preferences 内容。
package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

const providerRecoveryTableSQL = `
CREATE TABLE IF NOT EXISTS provider_scope_recovery (
    source_provider_id VARCHAR(64) NOT NULL,
    owner_user_id VARCHAR(64) NOT NULL,
    recovered_provider_id VARCHAR(64) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (source_provider_id, owner_user_id),
    UNIQUE (recovered_provider_id)
)`

type providerPreferenceSnapshot struct {
	DefaultAgentOptions struct {
		Provider string `json:"provider"`
	} `json:"default_agent_options"`
	// 兼容模型偏好改名之前的本地 preferences 文件。
	LegacyDefaultModelSelection                preferenceModelSelection `json:"default_model_selection"`
	LegacyDefaultImageGenerationModelSelection preferenceModelSelection `json:"default_image_generation_model_selection"`
	DefaultImageModelSelection                 preferenceModelSelection `json:"default_image_model_selection"`
	DefaultVisionModelSelection                preferenceModelSelection `json:"default_vision_model_selection"`
	DefaultBackgroundModelSelection            preferenceModelSelection `json:"default_background_model_selection"`
}

type preferenceModelSelection struct {
	Provider string `json:"provider"`
}

type providerRecoveryCandidate struct {
	sourceID string
	ownerID  string
}

// RepairDesktopProviderScope 修复桌面 App 本地库中被 00018 迁移误标为公共的 Provider。
func RepairDesktopProviderScope(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if !isDesktopSQLite(cfg) {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("打开桌面 Provider scope 修复数据库: %w", err)
	}
	defer db.Close()

	if _, err = db.ExecContext(ctx, providerRecoveryTableSQL); err != nil {
		return fmt.Errorf("创建桌面 Provider scope 修复账本: %w", err)
	}

	repaired := 0
	candidates, err := runtimeRecoveryCandidates(ctx, db)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		changed, repairErr := recoverProviderForOwner(ctx, db, candidate.sourceID, candidate.ownerID)
		if repairErr != nil {
			return fmt.Errorf("恢复 runtime %s 的 Provider %s: %w", candidate.ownerID, candidate.sourceID, repairErr)
		}
		if changed {
			repaired++
		}
	}

	owners, err := providerPreferenceOwners(ctx, db)
	if err != nil {
		return err
	}
	for _, ownerID := range owners {
		providers, readErr := readPreferenceProviders(cfg, ownerID)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			logger.Warn("读取桌面 Provider 偏好失败，跳过该用户", "owner_user_id", ownerID, "err", readErr)
			continue
		}
		for _, provider := range providers {
			sourceID, findErr := findLegacyPublicProvider(ctx, db, provider)
			if findErr != nil {
				return findErr
			}
			if sourceID == "" {
				continue
			}
			changed, repairErr := recoverProviderForOwner(ctx, db, sourceID, ownerID)
			if repairErr != nil {
				return fmt.Errorf("恢复偏好 %s 的 Provider %s: %w", ownerID, provider, repairErr)
			}
			if changed {
				repaired++
			}
		}
	}

	if repaired > 0 {
		logger.Info("已恢复桌面 App 中被误标为公共的 Provider", "recovered_private_providers", repaired)
	}
	if unresolved, countErr := countUnresolvedLegacyProviders(ctx, db); countErr != nil {
		logger.Warn("无法统计桌面中尚未归属的旧公共 Provider", "err", countErr)
	} else if unresolved > 0 {
		logger.Warn(
			"仍有旧公共 Provider 无法从 runtime 或 preferences 推断 owner，暂保留为 fallback",
			"unresolved_providers", unresolved,
		)
	}
	return nil
}

func isDesktopSQLite(cfg config.Config) bool {
	return strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") &&
		storage.IsSQLiteSQLDriver(cfg.DatabaseDriver)
}

func runtimeRecoveryCandidates(ctx context.Context, db *sql.DB) ([]providerRecoveryCandidate, error) {
	// 00018 曾把当时已有的所有 Provider 统一改成 public；以 goose 的实际执行时间区分后续有意创建的公共 Provider。
	rows, err := db.QueryContext(ctx, `
SELECT DISTINCT p.id, TRIM(a.owner_user_id)
FROM provider p
JOIN runtimes r ON LOWER(TRIM(r.provider)) = LOWER(TRIM(p.provider))
JOIN agents a ON a.id = r.agent_id
WHERE p.visibility = 'public'
  AND p.owner_user_id IS NULL
  AND TRIM(COALESCE(r.provider, '')) <> ''
  AND TRIM(COALESCE(a.owner_user_id, '')) <> ''
  AND EXISTS (
      SELECT 1
      FROM goose_db_version migration
      WHERE migration.version_id = 18 AND migration.is_applied = 1
  )
  AND substr(replace(p.created_at, 'T', ' '), 1, 19) <
      substr(replace((
          SELECT migration.tstamp
          FROM goose_db_version migration
          WHERE migration.version_id = 18 AND migration.is_applied = 1
          ORDER BY migration.id DESC
          LIMIT 1
      ), 'T', ' '), 1, 19)
ORDER BY p.id ASC, a.owner_user_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取桌面 Provider runtime 归属: %w", err)
	}
	defer rows.Close()

	result := make([]providerRecoveryCandidate, 0)
	for rows.Next() {
		var candidate providerRecoveryCandidate
		if err = rows.Scan(&candidate.sourceID, &candidate.ownerID); err != nil {
			return nil, fmt.Errorf("扫描桌面 Provider runtime 归属: %w", err)
		}
		candidate.sourceID = strings.TrimSpace(candidate.sourceID)
		candidate.ownerID = strings.TrimSpace(candidate.ownerID)
		if candidate.sourceID != "" && candidate.ownerID != "" {
			result = append(result, candidate)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历桌面 Provider runtime 归属: %w", err)
	}
	return result, nil
}

func providerPreferenceOwners(ctx context.Context, db *sql.DB) ([]string, error) {
	owners := map[string]struct{}{authctx.SystemUserID: {}}
	rows, err := db.QueryContext(ctx, `SELECT user_id FROM users`)
	if err != nil {
		return nil, fmt.Errorf("读取桌面 Provider 偏好用户列表: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ownerID string
		if err = rows.Scan(&ownerID); err != nil {
			return nil, fmt.Errorf("扫描桌面 Provider 偏好用户: %w", err)
		}
		ownerID = strings.TrimSpace(ownerID)
		if ownerID != "" {
			owners[ownerID] = struct{}{}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历桌面 Provider 偏好用户: %w", err)
	}
	result := make([]string, 0, len(owners))
	for ownerID := range owners {
		result = append(result, ownerID)
	}
	sort.Strings(result)
	return result, nil
}

func readPreferenceProviders(cfg config.Config, ownerID string) ([]string, error) {
	path := filepath.Join(agentpkg.UserWorkspaceBasePath(cfg, ownerID), ".settings", "preferences.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot providerPreferenceSnapshot
	if err = json.Unmarshal(content, &snapshot); err != nil {
		return nil, fmt.Errorf("解析 %s: %w", path, err)
	}
	providers := map[string]struct{}{}
	for _, provider := range []string{
		snapshot.DefaultAgentOptions.Provider,
		snapshot.LegacyDefaultModelSelection.Provider,
		snapshot.LegacyDefaultImageGenerationModelSelection.Provider,
		snapshot.DefaultImageModelSelection.Provider,
		snapshot.DefaultVisionModelSelection.Provider,
		snapshot.DefaultBackgroundModelSelection.Provider,
	} {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider != "" {
			providers[provider] = struct{}{}
		}
	}
	result := make([]string, 0, len(providers))
	for provider := range providers {
		result = append(result, provider)
	}
	sort.Strings(result)
	return result, nil
}

func findLegacyPublicProvider(ctx context.Context, db *sql.DB, provider string) (string, error) {
	row := db.QueryRowContext(ctx, `
SELECT id
FROM provider
WHERE visibility = 'public'
  AND owner_user_id IS NULL
  AND LOWER(TRIM(provider)) = ?
  AND EXISTS (
      SELECT 1
      FROM goose_db_version migration
      WHERE migration.version_id = 18 AND migration.is_applied = 1
  )
  AND substr(replace(created_at, 'T', ' '), 1, 19) <
      substr(replace((
          SELECT migration.tstamp
          FROM goose_db_version migration
          WHERE migration.version_id = 18 AND migration.is_applied = 1
          ORDER BY migration.id DESC
          LIMIT 1
      ), 'T', ' '), 1, 19)
ORDER BY created_at ASC, id ASC
LIMIT 1`, strings.ToLower(strings.TrimSpace(provider)))
	var sourceID string
	if err := row.Scan(&sourceID); errors.Is(err, sql.ErrNoRows) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("读取旧公共 Provider: %w", err)
	}
	return strings.TrimSpace(sourceID), nil
}

func recoverProviderForOwner(ctx context.Context, db *sql.DB, sourceID string, ownerID string) (bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	ownerID = strings.TrimSpace(ownerID)
	if sourceID == "" || ownerID == "" {
		return false, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var providerName string
	if err = tx.QueryRowContext(ctx, `
SELECT provider
FROM provider
WHERE id = ? AND visibility = 'public' AND owner_user_id IS NULL`, sourceID).Scan(&providerName); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("读取待恢复 Provider: %w", err)
	}
	privateID, err := findPrivateProviderTx(ctx, tx, ownerID, providerName)
	if err != nil {
		return false, err
	}
	if privateID != "" {
		return false, nil
	}

	targetID := recoveryProviderID(sourceID, ownerID)
	var mappingCreated bool
	var existingTarget string
	if err = tx.QueryRowContext(ctx, `
SELECT recovered_provider_id
FROM provider_scope_recovery
WHERE source_provider_id = ? AND owner_user_id = ?`, sourceID, ownerID).Scan(&existingTarget); errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO provider_scope_recovery (
    source_provider_id, owner_user_id, recovered_provider_id
) VALUES (?, ?, ?)`, sourceID, ownerID, targetID)
		if insertErr != nil {
			return false, fmt.Errorf("写入 Provider scope 修复账本: %w", insertErr)
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return false, fmt.Errorf("读取 Provider scope 修复账本写入结果: %w", affectedErr)
		}
		mappingCreated = affected > 0
		if err = tx.QueryRowContext(ctx, `
SELECT recovered_provider_id
FROM provider_scope_recovery
WHERE source_provider_id = ? AND owner_user_id = ?`, sourceID, ownerID).Scan(&existingTarget); err != nil {
			return false, fmt.Errorf("读取 Provider scope 修复账本: %w", err)
		}
	} else if err != nil {
		return false, fmt.Errorf("读取 Provider scope 修复账本: %w", err)
	}
	targetID = strings.TrimSpace(existingTarget)
	if targetID == "" {
		return false, fmt.Errorf("Provider scope 修复账本缺少目标 ID: source=%s owner=%s", sourceID, ownerID)
	}

	providerRows, err := copyProviderTx(ctx, tx, sourceID, targetID, ownerID)
	if err != nil {
		return false, err
	}
	modelRows, err := copyProviderModelsTx(ctx, tx, sourceID, targetID)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return mappingCreated || providerRows > 0 || modelRows > 0, nil
}

func findPrivateProviderTx(ctx context.Context, tx *sql.Tx, ownerID string, providerName string) (string, error) {
	row := tx.QueryRowContext(ctx, `
SELECT id
FROM provider
WHERE visibility = 'private'
  AND owner_user_id = ?
  AND LOWER(TRIM(provider)) = LOWER(TRIM(?))
ORDER BY created_at ASC, id ASC
LIMIT 1`, ownerID, providerName)
	var providerID string
	if err := row.Scan(&providerID); errors.Is(err, sql.ErrNoRows) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("读取用户私有 Provider: %w", err)
	}
	return strings.TrimSpace(providerID), nil
}

func copyProviderTx(ctx context.Context, tx *sql.Tx, sourceID string, targetID string, ownerID string) (int64, error) {
	result, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO provider (
    id, provider, display_name, auth_token, base_url, enabled,
    created_at, updated_at, provider_kind, preset_key, api_format, models_path,
    last_test_status, last_test_error, last_test_at, owner_user_id, visibility
)
SELECT
    ?, source.provider, source.display_name, source.auth_token, source.base_url, source.enabled,
    source.created_at, source.updated_at, source.provider_kind, source.preset_key, source.api_format, source.models_path,
    source.last_test_status, source.last_test_error, source.last_test_at, ?, 'private'
FROM provider source
WHERE source.id = ?
  AND NOT EXISTS (
      SELECT 1
      FROM provider existing
      WHERE existing.visibility = 'private'
        AND existing.owner_user_id = ?
        AND LOWER(TRIM(existing.provider)) = LOWER(TRIM(source.provider))
  )`, targetID, ownerID, sourceID, ownerID)
	if err != nil {
		return 0, fmt.Errorf("复制 Provider 配置: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("读取 Provider 写入结果: %w", err)
	}
	return rows, nil
}

func copyProviderModelsTx(ctx context.Context, tx *sql.Tx, sourceID string, targetID string) (int64, error) {
	result, err := tx.ExecContext(ctx, `
WITH recovery_params AS (
    SELECT ? AS target_id, ? AS source_id
), ranked_models AS (
    SELECT
        models.model_id,
        models.display_name,
        models.category,
        models.enabled,
        models.is_default,
        models.capabilities_auto_json,
        models.capabilities_override_json,
        models.context_window,
        models.max_output_tokens,
        models.provider_options_json,
        models.last_seen_at,
        models.created_at,
        models.updated_at,
        ROW_NUMBER() OVER (ORDER BY models.model_id, models.id) AS model_no
    FROM provider_models models
    JOIN recovery_params ON models.provider_id = recovery_params.source_id
)
INSERT OR IGNORE INTO provider_models (
    id, provider_id, model_id, display_name, category, enabled, is_default,
    capabilities_auto_json, capabilities_override_json, context_window,
    max_output_tokens, provider_options_json, last_seen_at, created_at, updated_at
)
SELECT
    substr('provider_model_recovered_' || substr(replace(recovery_params.target_id, '-', ''), -30) || '_' || CAST(ranked_models.model_no AS VARCHAR(8)), 1, 64),
    recovery_params.target_id,
    ranked_models.model_id,
    ranked_models.display_name,
    ranked_models.category,
    ranked_models.enabled,
    ranked_models.is_default,
    ranked_models.capabilities_auto_json,
    ranked_models.capabilities_override_json,
    ranked_models.context_window,
    ranked_models.max_output_tokens,
    ranked_models.provider_options_json,
    ranked_models.last_seen_at,
    ranked_models.created_at,
    ranked_models.updated_at
FROM ranked_models
JOIN recovery_params ON 1 = 1
WHERE NOT EXISTS (
    SELECT 1
    FROM provider_models existing
    WHERE existing.provider_id = recovery_params.target_id
      AND existing.model_id = ranked_models.model_id
)`, targetID, sourceID)
	if err != nil {
		return 0, fmt.Errorf("复制 Provider 模型: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("读取 Provider 模型写入结果: %w", err)
	}
	return rows, nil
}

func countUnresolvedLegacyProviders(ctx context.Context, db *sql.DB) (int, error) {
	row := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM provider p
WHERE p.visibility = 'public'
  AND p.owner_user_id IS NULL
  AND EXISTS (
      SELECT 1
      FROM goose_db_version migration
      WHERE migration.version_id = 18 AND migration.is_applied = 1
  )
  AND substr(replace(p.created_at, 'T', ' '), 1, 19) <
      substr(replace((
          SELECT migration.tstamp
          FROM goose_db_version migration
          WHERE migration.version_id = 18 AND migration.is_applied = 1
          ORDER BY migration.id DESC
          LIMIT 1
      ), 'T', ' '), 1, 19)
  AND NOT EXISTS (
      SELECT 1
      FROM provider_scope_recovery recovery
      WHERE recovery.source_provider_id = p.id
  )`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("统计未归属旧公共 Provider: %w", err)
	}
	return count, nil
}

func recoveryProviderID(sourceID string, ownerID string) string {
	digest := sha256.Sum256([]byte(sourceID + "\x00" + ownerID))
	return "provider_app_recovered_" + hex.EncodeToString(digest[:16])
}
