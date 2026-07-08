package subscription

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 负责订阅运营数据的读写。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{
		db:      db,
		dialect: storage.NewSQLDialect(cfg.DatabaseDriver),
	}
}
