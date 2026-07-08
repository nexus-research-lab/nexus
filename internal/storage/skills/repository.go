package skills

import (
	"database/sql"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 封装 skill 来源和导入状态 SQL 读写。
type Repository struct {
	db         *sql.DB
	isPostgres bool
}

// NewRepository 创建 skill SQL 仓储。
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

func (r *Repository) boolLiteral(value bool) string {
	if value {
		if r.isPostgres {
			return "TRUE"
		}
		return "1"
	}
	if r.isPostgres {
		return "FALSE"
	}
	return "0"
}

type rowScanner interface {
	Scan(dest ...any) error
}
