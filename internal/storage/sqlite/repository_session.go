package sqlite

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/storage/sessionrepo"
)

// SessionRepository 提供 SQLite 的 Room Session 视图查询。
type SessionRepository = sessionrepo.SQLRepository

// NewSessionRepository 创建 SessionRepository。
func NewSessionRepository(db *sql.DB) *SessionRepository {
	return sessionrepo.NewSQLRepository("sqlite", db)
}
