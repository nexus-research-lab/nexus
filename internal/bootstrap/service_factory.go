// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：service_factory.go
// @Date   ：2026/04/16 22:03:49
// @Author ：leemysw
// 2026/04/16 22:03:49   Create
// =====================================================

package bootstrap

import (
	"database/sql"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/room"
	"github.com/nexus-research-lab/nexus/internal/session"
	"github.com/nexus-research-lab/nexus/internal/storage"
	postgresrepo "github.com/nexus-research-lab/nexus/internal/storage/postgres"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
)

// OpenDB 打开数据库连接。
func OpenDB(cfg config.Config) (*sql.DB, error) {
	return storage.OpenDB(cfg)
}

// NewAgentService 创建 Agent 服务。
func NewAgentService(cfg config.Config) (*agent.Service, *sql.DB, error) {
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return nil, nil, err
	}
	return NewAgentServiceWithDB(cfg, db), db, nil
}

// NewAgentServiceWithDB 使用共享 DB 创建 Agent 服务。
func NewAgentServiceWithDB(cfg config.Config, db *sql.DB) *agent.Service {
	return agent.NewService(cfg, newAgentRepository(cfg, db))
}

// NewRoomService 创建 Room 服务。
func NewRoomService(cfg config.Config, agentService *agent.Service) (*room.Service, *sql.DB, error) {
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return nil, nil, err
	}
	return NewRoomServiceWithDB(cfg, db, agentService), db, nil
}

// NewRoomServiceWithDB 使用共享 DB 创建 Room 服务。
func NewRoomServiceWithDB(cfg config.Config, db *sql.DB, agentService *agent.Service) *room.Service {
	return room.NewService(cfg, agentService, newRoomRepository(cfg, db))
}

// NewSessionService 创建 Session 服务。
func NewSessionService(cfg config.Config, agentService *agent.Service) (*session.Service, *sql.DB, error) {
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return nil, nil, err
	}
	return NewSessionServiceWithDB(cfg, db, agentService), db, nil
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
