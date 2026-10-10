// INPUT: 配置的数据库驱动与连接。
// OUTPUT: 本机节点授权与任务 outbox 的共享仓储句柄。
// POS: teamrelay 存储包的唯一构造入口。
package teamrelay

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 保存本机节点授权、执行任务与输出 outbox。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

// NewRepository 创建本机节点仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}
