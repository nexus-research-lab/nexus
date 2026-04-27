package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/logx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	chatsvc "github.com/nexus-research-lab/nexus/internal/service/chat"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	"github.com/nexus-research-lab/nexus/internal/service/launcher"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
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
	Title        *titlegen.Service
	Usage        *usagesvc.Service
	Preferences  *preferencessvc.Service
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
	usageService := usagesvc.NewServiceWithDB(cfg, db)
	providerService := providercfg.NewServiceWithDB(cfg, db)
	preferencesService := preferencessvc.NewService(cfg)
	workspaceService := workspacepkg.NewService(cfg, core.Agent)
	skillService := skillsvc.NewService(cfg, core.Agent, workspaceService)
	connectorService := connectorsvc.NewService(cfg, db)
	launcherService := launcher.NewService(cfg, core.Agent, core.Room, core.Session)
	permission := permissionctx.NewContext()
	titleService := titlegen.NewService(providerService, core.Session, core.Room, permission)
	titleService.SetLogger(logger.With("component", "title"))
	runtimeManager := runtimectx.NewManager()
	channelRouter := channels.NewRouter(cfg, db, core.Agent, permission)
	channelRouter.SetLogger(logger.With("component", "channels"))
	chatService := chatsvc.NewService(cfg, core.Agent, runtimeManager, permission)
	chatService.SetLogger(logger.With("component", "chat"))
	chatService.SetProviderResolver(providerService)
	chatService.SetUsageRecorder(usageService)
	chatService.SetRoomSessionStore(newSessionRepository(cfg, db))
	chatService.SetTitleGenerator(titleService)
	ingressService := channels.NewIngressService(cfg, core.Agent, chatService, channelRouter)
	ingressService.SetLogger(logger.With("component", "channels.ingress"))
	channelRouter.SetIngress(ingressService)
	roomRealtime := roomsvc.NewRealtimeService(cfg, core.Room, core.Agent, runtimeManager, permission)
	roomRealtime.SetLogger(logger.With("component", "room"))
	roomRealtime.SetProviderResolver(providerService)
	roomRealtime.SetUsageRecorder(usageService)
	roomRealtime.SetTitleGenerator(titleService)
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

	// 把内置 MCP server 注入聊天/Room runtime。
	automationBuilder := newAutomationMCPBuilder(automationService, core.Agent, cfg.DefaultTimezone)
	connectorBuilder := newConnectorMCPBuilder(connectorService, core.Agent)
	mcpBuilder := combinedMCPBuilder(automationBuilder, connectorBuilder)
	chatService.SetMCPServerBuilder(mcpBuilder)
	roomRealtime.SetMCPServerBuilder(mcpBuilder)

	warnIfProviderMissing(providerService, logger)

	return &AppServices{
		DB:           db,
		Core:         core,
		Auth:         authService,
		Provider:     providerService,
		Preferences:  preferencesService,
		Workspace:    workspaceService,
		Skills:       skillService,
		Connectors:   connectorService,
		Launcher:     launcherService,
		Title:        titleService,
		Usage:        usageService,
		Permission:   permission,
		Runtime:      runtimeManager,
		Channels:     channelRouter,
		Chat:         chatService,
		Ingress:      ingressService,
		RoomRealtime: roomRealtime,
		Automation:   automationService,
	}
}

// warnIfProviderMissing 在启动期上报 Provider 配置缺口；不阻塞启动，避免空数据库下无法跑迁移/初始化。
func warnIfProviderMissing(svc *providercfg.Service, logger *slog.Logger) {
	state, err := svc.Availability(context.Background())
	if err != nil {
		logger.Warn("无法读取 Provider 配置，跳过启动检查", "err", err)
		return
	}
	switch {
	case state.Total == 0:
		logger.Warn("尚未配置任何 LLM Provider，请前往 Web Settings 或使用 nexusctl 添加；未配置前 Agent 调用会失败")
	case len(state.EnabledList) == 0:
		logger.Warn("已有 Provider 配置但全部处于禁用状态，请到 Settings 启用至少一个 Provider", "total", state.Total)
	case !state.HasDefault:
		logger.Warn("已启用 Provider 但未指定默认项，未显式声明 provider 的 Agent 将报错", "enabled", state.EnabledList)
	default:
		logger.Info("Provider 配置就绪", "enabled", state.EnabledList)
	}
}
