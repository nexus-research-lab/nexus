// INPUT: Provider SQL database, dialect and stable aggregate mutation outcomes.
// OUTPUT: Repository plus exact not-found, CAS, rollback and model-miss sentinels.
// POS: Provider persistence evidence boundary; only a confirmed rollback may claim not-applied.
package provider

import (
	"database/sql"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

var (
	// ErrProviderNotFound 表示条件写入的 Provider 已不存在。
	ErrProviderNotFound = errors.New("provider not found")
	// ErrConfigurationVersionConflict 表示 Provider 聚合已被其他写入推进。
	ErrConfigurationVersionConflict = errors.New("provider configuration version conflict")
	// ErrModelNotFound 表示条件写入的模型卡不存在。
	ErrModelNotFound = errors.New("provider model not found")
	// ErrMutationNotApplied 表示事务未提交且回滚已确认。
	ErrMutationNotApplied = errors.New("provider mutation not applied")
)

// Repository 封装 provider 配置的 SQL 读写。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

const (
	// VisibilityPublic 表示平台公共 Provider。
	VisibilityPublic = "public"
	// VisibilityPrivate 表示用户私有 Provider。
	VisibilityPrivate = "private"
	// apiFormatDashScopeImageGeneration 表示无模型列表端点的 DashScope 生图分支协议。
	apiFormatDashScopeImageGeneration = "dashscope_image_generation"
	// apiFormatModelScopeImageGeneration 表示无模型列表端点的 ModelScope 生图分支协议。
	apiFormatModelScopeImageGeneration = "modelscope_image_generation"
)

// NewRepository 创建 provider SQL 仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{
		db:      db,
		dialect: storage.NewSQLDialect(cfg.DatabaseDriver),
	}
}

func (r *Repository) bind(index int) string {
	return r.dialect.Bind(index)
}

func (r *Repository) trueValue() string {
	return r.dialect.TrueValue()
}

func (r *Repository) falseValue() string {
	return r.dialect.FalseValue()
}

func (r *Repository) currentTimestamp() string {
	return r.dialect.CurrentTimestamp()
}
