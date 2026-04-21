package sqlite

import "database/sql"

// Repository 提供 SQLite 仓储骨架。
type Repository struct {
	DB *sql.DB
}

// New 创建 SQLite 仓储。
func New(db *sql.DB) *Repository {
	return &Repository{DB: db}
}
