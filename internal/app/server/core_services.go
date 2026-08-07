package server

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/service/agent"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
	"github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/service/session"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/sessionrepo"
)

// CoreServices 表示核心领域服务与共享 DB 的统一装配结果。
type CoreServices struct {
	DB       *sql.DB
	Agent    *agent.Service
	Room     *room.Service
	Session  *session.Service
	Deletion *deletionsvc.Coordinator
}

// OpenDB 打开数据库连接。
func OpenDB(cfg config.Config) (*sql.DB, error) {
	return storage.OpenDB(cfg)
}

// NewCoreServicesWithDB 使用共享 DB 创建核心领域服务。
func NewCoreServicesWithDB(cfg config.Config, db *sql.DB) *CoreServices {
	deletionCoordinator := deletionsvc.NewCoordinator(cfg, db)
	agentService := agent.NewService(cfg, newAgentRepository(cfg, db))
	roomService := room.NewService(cfg, agentService, newRoomRepository(cfg, db))
	roomService.SetDeletionCoordinator(deletionCoordinator)
	sessionService := session.NewService(cfg, agentService, newSessionRepository(cfg, db))
	sessionService.SetDeletionCoordinator(deletionCoordinator)
	agentService.SetDeletionLifecycle(sessionService, nil)
	return &CoreServices{
		DB:       db,
		Agent:    agentService,
		Room:     roomService,
		Session:  sessionService,
		Deletion: deletionCoordinator,
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
	service := room.NewService(cfg, agentService, newRoomRepository(cfg, db))
	service.SetDeletionCoordinator(deletionsvc.NewCoordinator(cfg, db))
	return service
}

// NewSessionServiceWithDB 使用共享 DB 创建 Session 服务。
func NewSessionServiceWithDB(cfg config.Config, db *sql.DB, agentService *agent.Service) *session.Service {
	service := session.NewService(cfg, agentService, newSessionRepository(cfg, db))
	service.SetDeletionCoordinator(deletionsvc.NewCoordinator(cfg, db))
	return service
}

func newAgentRepository(cfg config.Config, db *sql.DB) *agentrepo.SQLRepository {
	return agentrepo.NewSQLRepository(cfg.DatabaseDriver, db)
}

func newRoomRepository(cfg config.Config, db *sql.DB) *roomrepo.SQLRepository {
	return roomrepo.NewSQLRepository(cfg.DatabaseDriver, db)
}

func newSessionRepository(cfg config.Config, db *sql.DB) *sessionrepo.SQLRepository {
	return sessionrepo.NewSQLRepository(cfg.DatabaseDriver, db)
}
