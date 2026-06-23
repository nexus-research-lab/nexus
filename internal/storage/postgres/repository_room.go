package postgres

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// RoomRepository 提供 PostgreSQL 的 Room 仓储实现。
type RoomRepository = roomrepo.SQLRepository

// NewRoomRepository 创建 Room 仓储。
func NewRoomRepository(db *sql.DB) *RoomRepository {
	return roomrepo.NewSQLRepository("postgres", db)
}
