package auth

import (
	"database/sql"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 封装认证相关表的 SQL 读写。
type Repository struct {
	db         *sql.DB
	isPostgres bool
}

// NewRepository 创建认证 SQL 仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
}

func (r *Repository) bind(index int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}
