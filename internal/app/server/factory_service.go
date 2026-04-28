package server

import (
	"database/sql"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/service/session"
	"github.com/nexus-research-lab/nexus/internal/storage"
	postgresrepo "github.com/nexus-research-lab/nexus/internal/storage/postgres"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
)

// CoreServices 表示核心领域服务与共享 DB 的统一装配结果。
type CoreServices struct {
	DB      *sql.DB
	Agent   *agent.Service
	Room    *room.Service
	Session *session.Service
}

// OpenDB 打开数据库连接。
func OpenDB(cfg config.Config) (*sql.DB, error) {
	return storage.OpenDB(cfg)
}

// NewCoreServicesWithDB 使用共享 DB 创建核心领域服务。
func NewCoreServicesWithDB(cfg config.Config, db *sql.DB) *CoreServices {
	agentService := NewAgentServiceWithDB(cfg, db)
	return &CoreServices{
		DB:      db,
		Agent:   agentService,
		Room:    NewRoomServiceWithDB(cfg, db, agentService),
		Session: NewSessionServiceWithDB(cfg, db, agentService),
	}
}

// NewAgentService 创建 Agent 服务。
func NewAgentService(cfg config.Config) (*agent.Service, *sql.DB, error) {
	db, err := OpenDB(cfg)
	if err != nil {
		return nil, nil, err
	}
	return NewAgentServiceWithDB(cfg, db), db, nil
}

// NewAgentServiceWithDB 使用共享 DB 创建 Agent 服务。
func NewAgentServiceWithDB(cfg config.Config, db *sql.DB) *agent.Service {
	return agent.NewService(cfg, newAgentRepository(cfg, db))
}

// NewRoomServiceWithDB 使用共享 DB 创建 Room 服务。
func NewRoomServiceWithDB(cfg config.Config, db *sql.DB, agentService *agent.Service) *room.Service {
	return room.NewService(cfg, agentService, newRoomRepository(cfg, db))
}

// NewSessionServiceWithDB 使用共享 DB 创建 Session 服务。
func NewSessionServiceWithDB(cfg config.Config, db *sql.DB, agentService *agent.Service) *session.Service {
	return session.NewService(cfg, agentService, newSessionRepository(cfg, db))
}

func newAgentRepository(cfg config.Config, db *sql.DB) agent.Repository {
	switch strings.ToLower(cfg.DatabaseDriver) {
	case "postgres", "postgresql", "pg":
		return postgresrepo.NewAgentRepository(db)
	default:
		return sqliterepo.NewAgentRepository(db)
	}
}

func newRoomRepository(cfg config.Config, db *sql.DB) room.Repository {
	switch strings.ToLower(cfg.DatabaseDriver) {
	case "postgres", "postgresql", "pg":
		return postgresrepo.NewRoomRepository(db)
	default:
		return sqliterepo.NewRoomRepository(db)
	}
}

func newSessionRepository(cfg config.Config, db *sql.DB) session.SQLRepository {
	switch strings.ToLower(cfg.DatabaseDriver) {
	case "postgres", "postgresql", "pg":
		return postgresrepo.NewSessionRepository(db)
	default:
		return sqliterepo.NewSessionRepository(db)
	}
}
