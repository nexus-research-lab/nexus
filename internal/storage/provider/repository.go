package provider

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
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
