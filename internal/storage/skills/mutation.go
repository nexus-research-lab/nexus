// INPUT: owner、可选 expected catalog version 与同一 owner 的来源/导入写入。
// OUTPUT: 持久单调版本 CAS 和覆盖整次数据库写阶段的 owner mutation transaction。
// POS: Skill 全局 catalog 跨进程串行与乐观并发控制的 SQL 真相边界。
package skills

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrCatalogVersionConflict 表示计划使用的 owner Skill catalog 版本已经过期。
	ErrCatalogVersionConflict = errors.New("skill catalog version conflict")
)

// CatalogVersionConflictError 携带过期计划与当前持久版本。
type CatalogVersionConflictError struct {
	Expected int64
	Current  int64
}

func (e *CatalogVersionConflictError) Error() string {
	return fmt.Sprintf(
		"%v: expected=%d current=%d",
		ErrCatalogVersionConflict,
		e.Expected,
		e.Current,
	)
}

func (e *CatalogVersionConflictError) Unwrap() error {
	return ErrCatalogVersionConflict
}

// CatalogMutation 把版本 CAS 与同一次 Skill catalog 数据库写入放在一个 transaction 内。
type CatalogMutation struct {
	repository  *Repository
	tx          *sql.Tx
	ownerUserID string
	version     int64
	finished    bool
}

// CatalogVersion 返回 owner 当前持久 catalog 版本；首次读取建立 version=1 的根。
func (r *Repository) CatalogVersion(ctx context.Context, ownerUserID string) (int64, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return 0, errors.New("skill catalog owner 不能为空")
	}
	if err := r.ensureCatalogVersion(ctx, r.db, ownerUserID); err != nil {
		return 0, err
	}
	var version int64
	if err := r.db.QueryRowContext(
		ctx,
		"SELECT version FROM skill_catalog_versions WHERE owner_user_id = "+r.bind(1),
		ownerUserID,
	).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

// BeginCatalogMutation 先锁定 owner version 行；bump=true 时在 transaction 内执行 CAS 并递增版本。
//
// 调用方必须在任何文件发布之前完成远端下载和 staging，只把快速的原子 rename
// 与数据库 catalog 写入放进本 transaction。
func (r *Repository) BeginCatalogMutation(
	ctx context.Context,
	ownerUserID string,
	expectedVersion *int64,
	bump bool,
) (*CatalogMutation, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, errors.New("skill catalog owner 不能为空")
	}
	if expectedVersion != nil && !bump {
		return nil, errors.New("expected catalog version requires a versioned mutation")
	}
	if err := r.ensureCatalogVersion(ctx, r.db, ownerUserID); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := func(cause error) (*CatalogMutation, error) {
		return nil, errors.Join(cause, tx.Rollback())
	}

	var result sql.Result
	switch {
	case bump && expectedVersion != nil:
		result, err = tx.ExecContext(
			ctx,
			"UPDATE skill_catalog_versions SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = "+
				r.bind(1)+" AND version = "+r.bind(2),
			ownerUserID,
			*expectedVersion,
		)
	case bump:
		result, err = tx.ExecContext(
			ctx,
			"UPDATE skill_catalog_versions SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = "+
				r.bind(1),
			ownerUserID,
		)
	default:
		// 即使健康检查等元数据写不改变功能版本，也必须拿到同一 owner 的持久写锁。
		result, err = tx.ExecContext(
			ctx,
			"UPDATE skill_catalog_versions SET updated_at = updated_at WHERE owner_user_id = "+r.bind(1),
			ownerUserID,
		)
	}
	if err != nil {
		return rollback(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return rollback(err)
	}
	if affected != 1 {
		_ = tx.Rollback()
		current, currentErr := r.CatalogVersion(ctx, ownerUserID)
		if currentErr != nil {
			return nil, currentErr
		}
		if expectedVersion != nil {
			return nil, &CatalogVersionConflictError{
				Expected: *expectedVersion,
				Current:  current,
			}
		}
		return nil, errors.New("skill catalog version row disappeared")
	}

	var version int64
	if err = tx.QueryRowContext(
		ctx,
		"SELECT version FROM skill_catalog_versions WHERE owner_user_id = "+r.bind(1),
		ownerUserID,
	).Scan(&version); err != nil {
		return rollback(err)
	}
	return &CatalogMutation{
		repository:  r,
		tx:          tx,
		ownerUserID: ownerUserID,
		version:     version,
	}, nil
}

func (r *Repository) ensureCatalogVersion(
	ctx context.Context,
	executor sqlExecutor,
	ownerUserID string,
) error {
	query := `
INSERT INTO skill_catalog_versions (owner_user_id, version, updated_at)
VALUES (` + r.bind(1) + `, 1, CURRENT_TIMESTAMP)
ON CONFLICT (owner_user_id) DO NOTHING`
	_, err := executor.ExecContext(ctx, query, strings.TrimSpace(ownerUserID))
	return err
}

// Version 返回本次 transaction 中的 catalog 版本。
func (m *CatalogMutation) Version() int64 {
	if m == nil {
		return 0
	}
	return m.version
}

// OwnerUserID 返回被锁定的 owner。
func (m *CatalogMutation) OwnerUserID() string {
	if m == nil {
		return ""
	}
	return m.ownerUserID
}

// Commit 提交版本与 catalog 数据写入。
func (m *CatalogMutation) Commit() error {
	if m == nil || m.tx == nil {
		return errors.New("skill catalog mutation 未初始化")
	}
	if m.finished {
		return errors.New("skill catalog mutation 已结束")
	}
	m.finished = true
	return m.tx.Commit()
}

// Rollback 回滚版本与 catalog 数据写入。
func (m *CatalogMutation) Rollback() error {
	if m == nil || m.tx == nil || m.finished {
		return nil
	}
	m.finished = true
	return m.tx.Rollback()
}

func (m *CatalogMutation) validateEntityOwner(ownerUserID string) error {
	if m == nil || m.tx == nil || m.finished {
		return errors.New("skill catalog mutation 不可用")
	}
	if strings.TrimSpace(ownerUserID) != m.ownerUserID {
		return errors.New("skill catalog mutation owner 不匹配")
	}
	return nil
}

// EnsureSource 在 mutation 内建立默认来源记录。
func (m *CatalogMutation) EnsureSource(ctx context.Context, item SourceEntity) error {
	if err := m.validateEntityOwner(item.OwnerUserID); err != nil {
		return err
	}
	return ensureSource(ctx, m.tx, m.repository.isPostgres, m.repository.bind, item)
}

// UpsertSource 在 mutation 内更新来源。
func (m *CatalogMutation) UpsertSource(ctx context.Context, item SourceEntity) error {
	if err := m.validateEntityOwner(item.OwnerUserID); err != nil {
		return err
	}
	return upsertSource(ctx, m.tx, m.repository.isPostgres, item)
}

// GetSource 在 mutation 内读取来源。
func (m *CatalogMutation) GetSource(
	ctx context.Context,
	sourceID string,
) (*SourceEntity, error) {
	row := m.tx.QueryRowContext(ctx, `
SELECT owner_user_id, source_id, name, kind, url, trust, enabled, sort_order,
       last_checked_at, last_error, created_at, updated_at
FROM skill_sources
WHERE owner_user_id = `+m.repository.bind(1)+` AND source_id = `+m.repository.bind(2),
		m.ownerUserID,
		strings.TrimSpace(sourceID),
	)
	item, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// RecordSourceCheck 在 mutation 内更新来源健康元数据。
func (m *CatalogMutation) RecordSourceCheck(
	ctx context.Context,
	sourceID string,
	checkedAt time.Time,
	lastError string,
) error {
	return recordSourceCheck(
		ctx,
		m.tx,
		m.repository.isPostgres,
		m.ownerUserID,
		sourceID,
		checkedAt,
		lastError,
	)
}

// UpsertImportedSkill 在 mutation 内写入导入记录。
func (m *CatalogMutation) UpsertImportedSkill(
	ctx context.Context,
	item ImportedSkillEntity,
) error {
	if err := m.validateEntityOwner(item.OwnerUserID); err != nil {
		return err
	}
	return upsertImportedSkill(ctx, m.tx, m.repository.isPostgres, item)
}

// DeleteImportedSkill 在 mutation 内删除导入记录。
func (m *CatalogMutation) DeleteImportedSkill(ctx context.Context, skillName string) error {
	_, err := m.tx.ExecContext(
		ctx,
		"DELETE FROM imported_skills WHERE owner_user_id = "+m.repository.bind(1)+
			" AND skill_name = "+m.repository.bind(2),
		m.ownerUserID,
		strings.TrimSpace(skillName),
	)
	return err
}

// RecordImportedSkillCheck 在 mutation 内更新远端检查元数据。
func (m *CatalogMutation) RecordImportedSkillCheck(
	ctx context.Context,
	skillName string,
	updateAvailable bool,
	checkedAt time.Time,
	lastError string,
) error {
	return recordImportedSkillCheck(
		ctx,
		m.tx,
		m.repository.bind,
		m.ownerUserID,
		skillName,
		updateAvailable,
		checkedAt,
		lastError,
	)
}
