// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：app_services.go
// @Date   ：2026/04/16 23:35:27
// @Author ：leemysw
// 2026/04/16 23:35:27   Create
// =====================================================

package bootstrap

import (
	"database/sql"
	"log/slog"

	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/channels"
	chatsvc "github.com/nexus-research-lab/nexus/internal/chat"
	"github.com/nexus-research-lab/nexus/internal/config"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/launcher"
	"github.com/nexus-research-lab/nexus/internal/logx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	providercfg "github.com/nexus-research-lab/nexus/internal/provider"
	roomsvc "github.com/nexus-research-lab/nexus/internal/room"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	skillsvc "github.com/nexus-research-lab/nexus/internal/skills"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"
)

// AppServices 表示完整应用运行所需的核心依赖容器。
type AppServices struct {
	DB           *sql.DB
	Core         *CoreServices
	Auth         *authsvc.Service
	Provider     *providercfg.Service
	Workspace    *workspacepkg.Service
	Skills       *skillsvc.Service
	Connectors   *connectorsvc.Service
	Launcher     *launcher.Service
	Permission   *permissionctx.Context
	Runtime      *runtimectx.Manager
	Channels     *channels.Router
	Chat         *chatsvc.Service
	Ingress      *channels.IngressService
	RoomRealtime *roomsvc.RealtimeService
	Automation   *automationsvc.Service
}

// NewAppServices 创建完整应用依赖容器。
func NewAppServices(cfg config.Config, logger *slog.Logger) (*AppServices, error) {
	db, err := OpenDB(cfg)
	if err != nil {
		return nil, err
	}
	return NewAppServicesWithDB(cfg, db, logger), nil
}

// NewAppServicesWithDB 使用共享 DB 创建完整应用依赖容器。
func NewAppServicesWithDB(cfg config.Config, db *sql.DB, logger *slog.Logger) *AppServices {
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	core := NewCoreServicesWithDB(cfg, db)
	authService := authsvc.NewServiceWithDB(cfg, db)
	providerService := providercfg.NewServiceWithDB(cfg, db)
	workspaceService := workspacepkg.NewService(cfg, core.Agent)
	skillService := skillsvc.NewService(cfg, core.Agent, workspaceService)
	connectorService := connectorsvc.NewService(cfg, db)
	launcherService := launcher.NewService(cfg, core.Agent, core.Room)
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	channelRouter := channels.NewRouter(cfg, db, core.Agent, permission)
	channelRouter.SetLogger(logger.With("component", "channels"))
	chatService := chatsvc.NewService(cfg, core.Agent, runtimeManager, permission)
	chatService.SetLogger(logger.With("component", "chat"))
	chatService.SetProviderResolver(providerService)
	chatService.SetRoomSessionStore(newSessionRepository(cfg, db))
	ingressService := channels.NewIngressService(cfg, core.Agent, chatService, channelRouter)
	ingressService.SetLogger(logger.With("component", "channels.ingress"))
	channelRouter.SetIngress(ingressService)
	roomRealtime := roomsvc.NewRealtimeService(cfg, core.Room, core.Agent, runtimeManager, permission)
	roomRealtime.SetLogger(logger.With("component", "room"))
	roomRealtime.SetProviderResolver(providerService)
	automationService := automationsvc.NewService(
		cfg,
		db,
		core.Agent,
		chatService,
		roomRealtime,
		permission,
		workspaceService,
		channelRouter,
	)
	automationService.SetRuntimeSessionCloser(runtimeManager)
	automationService.SetLogger(logger.With("component", "automation"))

	// 把 nexus_automation MCP server 注入聊天/Room runtime，主智能体可通过工具自助管理定时任务。
	mcpBuilder := newAutomationMCPBuilder(automationService, core.Agent)
	chatService.SetMCPServerBuilder(mcpBuilder)
	roomRealtime.SetMCPServerBuilder(mcpBuilder)

	return &AppServices{
		DB:           db,
		Core:         core,
		Auth:         authService,
		Provider:     providerService,
		Workspace:    workspaceService,
		Skills:       skillService,
		Connectors:   connectorService,
		Launcher:     launcherService,
		Permission:   permission,
		Runtime:      runtimeManager,
		Channels:     channelRouter,
		Chat:         chatService,
		Ingress:      ingressService,
		RoomRealtime: roomRealtime,
		Automation:   automationService,
	}
}
