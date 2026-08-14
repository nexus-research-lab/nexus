// INPUT: 应用配置、数据库与基础服务依赖。
// OUTPUT: 完整 AppServices 依赖图、跨域 runtime 装配及自有数据库生命周期。
// POS: Nexus server 服务装配根。
package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/workspaceisolation"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	channelauthorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalobjectivesvc "github.com/nexus-research-lab/nexus/internal/service/goalobjective"
	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
	"github.com/nexus-research-lab/nexus/internal/service/launcher"
	loopsvc "github.com/nexus-research-lab/nexus/internal/service/loops"
	memorymaintenancesvc "github.com/nexus-research-lab/nexus/internal/service/memorymaintenance"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	projectpermissionsvc "github.com/nexus-research-lab/nexus/internal/service/projectpermission"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
)

// AppServices 表示完整应用运行所需的核心依赖容器。
type AppServices struct {
	DB                     *sql.DB
	Core                   *CoreServices
	Auth                   *authsvc.Service
	Provider               *providercfg.Service
	Subscription           *subscriptionsvc.Service
	Workspace              *workspacepkg.Service
	ProjectPermission      *projectpermissionsvc.Service
	Skills                 *skillsvc.Service
	Connectors             *connectorsvc.Service
	ConnectorAuthorization *connectorsvc.AuthorizationControl
	Configuration          *configurationsvc.Service
	Launcher               *launcher.Service
	Title                  *titlegen.Service
	Usage                  *usagesvc.Service
	Preferences            *preferencessvc.Service
	Permission             *permissionctx.Context
	Runtime                *runtimectx.Manager
	Channels               *channels.Router
	ChannelControl         *channels.ControlService
	ChannelAuthorization   *channelauthorizationsvc.Service
	Communication          *communicationsvc.Service
	DM                     *dmsvc.Service
	Ingress                *channels.IngressService
	RoomRealtime           *roomrealtime.Service
	Automation             *automationsvc.Service
	Imagegen               *imagegensvc.Service
	Goal                   *goalsvc.Service
	Orchestration          *orchestrationsvc.Service
	Loops                  *loopsvc.Service
	MemoryMaintenance      *memorymaintenancesvc.Coordinator
	SlashCatalog           *slashcommandsvc.Catalog
	SlashRegistry          *slashcommandsvc.Registry
	ownsDB                 bool
}

// Close 等待仍可能写入 workspace 的标题任务结束，并释放容器自行打开的数据库。
func (s *AppServices) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	var closeErrors []error
	if s.ChannelAuthorization != nil {
		closeErrors = append(closeErrors, s.ChannelAuthorization.Close(ctx))
	}
	if s.Title != nil {
		closeErrors = append(closeErrors, s.Title.Close(ctx))
	}
	if s.ownsDB && s.DB != nil {
		closeErrors = append(closeErrors, s.DB.Close())
	}
	return errors.Join(closeErrors...)
}

// NewAppServices 创建完整应用依赖容器。
func NewAppServices(cfg config.Config, logger *slog.Logger) (*AppServices, error) {
	db, err := OpenDB(cfg)
	if err != nil {
		return nil, err
	}
	services := NewAppServicesWithDB(cfg, db, logger)
	services.ownsDB = true
	return services, nil
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
	providerService.SetLogger(logger.With("component", "provider"))
	subscriptionService := subscriptionsvc.NewServiceWithDB(cfg, db)
	goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	goalService.SetLogger(logger.With("component", "goal"))
	goalService.SetSessionOwnershipVerifier(newGoalSessionOwnershipVerifier(
		core.Agent,
		core.Room,
	))
	orchestrationService := orchestrationsvc.NewService(orchestrationstore.NewRepository(cfg, db))
	orchestrationService.SetRuntimeGraphSubagentToolHistoryProvider(
		executionSubagentToolHistory{sessions: core.Session},
	)
	explicitGoalCoordinator := newExplicitGoalExecutionCoordinator(goalService, orchestrationService)
	goalService.SetObjectiveRetargetCoordinator(explicitGoalCoordinator)
	orchestrationService.SetExplicitGoalBindingGateway(explicitGoalCoordinator)
	orchestrationService.SetGoalPromotionGateway(newExecutionGoalPromotionGateway(cfg, goalService))
	goalService.SetExecutionGoalCompletionReadiness(executionGoalCompletionReadiness{
		orchestration: orchestrationService,
	})
	preferencesService := preferencessvc.NewService(cfg)
	providerService.SetDefaultAgentSelectionResolver(func(ctx context.Context, ownerUserID string) (providercfg.DefaultAgentSelection, error) {
		prefs, err := preferencesService.Get(ctx, ownerUserID)
		if err != nil {
			return providercfg.DefaultAgentSelection{}, err
		}
		return providercfg.DefaultAgentSelection{
			Provider:    prefs.DefaultAgentOptions.Provider,
			Model:       prefs.DefaultAgentOptions.Model,
			RuntimeKind: prefs.AgentRuntimeKind,
		}, nil
	})
	imagegenService := imagegensvc.NewService(providerService, cfg.WorkspacePath)
	loopService := loopsvc.NewService()
	imagegenService.SetPreferences(preferencesService)
	workspaceService := workspacepkg.NewService(cfg, core.Agent)
	projectPermissionService := projectpermissionsvc.NewService(cfg)
	skillService := skillsvc.NewServiceWithDB(cfg, db, core.Agent, workspaceService)
	core.Room.SetSkillCatalog(skillService)
	connectorService := connectorsvc.NewService(cfg, db)
	launcherService := launcher.NewService(cfg, core.Agent, core.Room, core.Session)
	permission := permissionctx.NewContext()
	goalService.SetEventBroadcaster(permission)
	core.Session.SetGoalCompletionUsageProvider(goalService)
	titleService := titlegen.NewService(providerService, core.Session, core.Room, permission, preferencesService)
	titleService.SetLogger(logger.With("component", "title"))
	runtimeManager := runtimectx.NewManager()
	runtimeManager.SetOwnerProcessReaper(workspaceisolation.OwnerProcessReaper{
		Mode:         workspaceisolation.Mode(cfg.RuntimeIsolationMode),
		LauncherPath: cfg.RuntimeLauncherPath,
	})
	runtimeTransition := newRuntimeAuthTransition(runtimeManager)
	authService.SetRuntimeTransitionCoordinator(runtimeTransition)
	projectPermissionService.SetRuntimeSessionCloser(runtimeManager)
	goalService.SetPreviewFiller(titleService)
	goalObjectiveService := goalobjectivesvc.NewService(providerService, preferencesService)
	goalObjectiveService.SetConversationResolvers(core.Agent, core.Room)
	goalService.SetObjectiveRewriter(goalObjectiveService)
	goalService.SetExternalMutationAccountant(runtimeManager)
	core.Agent.SetGoalCleaner(goalService)
	core.Room.SetGoalCleaner(goalService)
	core.Deletion.SetGoalCleaner(goalService)
	core.Room.SetRuntimeManager(runtimeManager)
	core.Session.SetRuntimeManager(runtimeManager)
	channelRouter := channels.NewRouter(cfg, db, core.Agent, permission)
	channelRouter.SetSessionProjectionResolver(core.Session)
	channelRouter.SetLogger(logger.With("component", "channels"))
	channelControl := channels.NewControlService(cfg, db, core.Agent, channelRouter)
	core.Session.SetExternalSessionIdentityResolver(channelControl)
	core.Room.SetSessionArtifactDeletionCoordinator(core.Session)
	core.Agent.SetDeletionCoordinator(newAgentDeletionCoordinator(channelControl, runtimeManager))
	queueAdmissionRepository := queueadmissionstore.NewRepository(cfg, db)
	dmService := dmsvc.NewService(cfg, core.Agent, runtimeManager, permission)
	dmService.SetLogger(logger.With("component", "dm"))
	dmService.SetProviderResolver(providerService)
	dmService.SetPreferences(preferencesService)
	dmService.SetUsageRecorder(usageService)
	dmService.SetQuotaChecker(subscriptionService)
	dmService.SetGoalContextProvider(goalService)
	dmService.SetExecutionContextProvider(orchestrationService)
	dmService.SetSubagentAdmissionProvider(orchestrationService)
	dmService.SetRuntimeAdmissionResolver(authService)
	dmService.SetQueueAdmissionStore(queueAdmissionRepository)
	dmService.SetRoomSessionStore(newSessionRepository(cfg, db))
	dmService.SetRoomConversationActivityStore(core.Room)
	dmService.SetTitleGenerator(titleService)
	dmService.SetExternalReplyDispatcher(dmExternalReplyDispatcher{router: channelRouter})
	ingressService := channels.NewIngressService(cfg, core.Agent, dmService, channelRouter)
	ingressService.SetLogger(logger.With("component", "channels.ingress"))
	ingressService.SetControlService(channelControl)
	ingressService.SetRuntimePermissionContext(permission)
	channelRouter.SetIngress(ingressService)
	roomRealtime := roomrealtime.NewService(cfg, core.Room, core.Agent, runtimeManager, permission)
	roomRealtime.SetLogger(logger.With("component", "room"))
	roomRealtime.SetProviderResolver(providerService)
	roomRealtime.SetPreferences(preferencesService)
	roomRealtime.SetUsageRecorder(usageService)
	roomRealtime.SetQuotaChecker(subscriptionService)
	roomRealtime.SetGoalContextProvider(goalService)
	roomRealtime.SetExecutionContextProvider(orchestrationService)
	roomRealtime.SetSubagentAdmissionProvider(orchestrationService)
	roomRealtime.SetRuntimeAdmissionResolver(authService)
	roomRealtime.SetQueueAdmissionStore(queueAdmissionRepository)
	roomRealtime.SetTitleGenerator(titleService)
	orchestrationService.SetAssignmentTargetAuthorizer(roomRealtime)
	orchestrationService.SetExecutionDispatchConsumer(roomRealtime)
	orchestrationService.SetExecutionReviewDispatchConsumer(roomRealtime)
	orchestrationService.SetExecutionCancellationConsumer(executionCancellationConsumer{
		room:    roomRealtime,
		runtime: runtimeManager,
	})
	goalService.SetRoomGoalCompletionReadiness(roomRealtime)
	goalService.SetGuidanceDispatcher(goalGuidanceDispatcher{runtime: runtimeManager, room: roomRealtime})
	goalService.SetRuntimeInterrupter(newGoalInterruptDispatcher(dmService, roomRealtime))
	automationService := automationsvc.NewService(
		cfg,
		db,
		core.Agent,
		dmService,
		roomRealtime,
		permission,
		workspaceService,
		channelRouter,
	)
	automationService.SetSessionArtifactDeletionCoordinator(core.Session)
	automationService.SetDeliverySessionResolver(core.Session)
	core.Deletion.SetTaskCleaner(automationService)
	core.Agent.SetDeletionLifecycle(core.Session, automationService)
	automationService.SetProviderResolver(providerService)
	automationService.SetConnectorResolver(connectorService)
	automationService.SetDeliveryGrantResolver(channelControl)
	core.Session.SetTaskReferenceResolver(automationService)
	ingressService.SetCommandHandler(automationService)
	automationService.SetLogger(logger.With("component", "automation"))
	memoryMaintenance := memorymaintenancesvc.NewCoordinator(cfg, core.Agent, providerService, preferencesService, authService)
	memoryMaintenance.SetLogger(logger.With("component", "memory.maintenance"))
	configurationService := configurationsvc.NewService(
		cfg,
		db,
		core.Agent,
		providerService,
		preferencesService,
		channelControl,
		connectorService,
		skillService,
		runtimeManager,
	)
	configurationService.SetSessionControl(core.Session)
	configurationService.SetRoomControl(core.Room, roomRealtime)
	configurationService.SetPrincipalVerifiers(authService, authService)
	connectorAuthorization, err := connectorsvc.NewAuthorizationControl(
		connectorService,
		core.Agent,
		runtimeManager,
		authService,
		authService,
	)
	if err != nil {
		logger.Warn("Connector 对话授权未启用", "err", err)
		connectorAuthorization = nil
	}
	channelAuthorization := channelauthorizationsvc.NewService(
		cfg,
		db,
		configurationService,
		authService,
		channelControl,
		nil,
	)
	if err := channelAuthorization.Initialize(context.Background()); err != nil {
		logger.Warn("Channel 对话授权未启用", "err", err)
		_ = channelAuthorization.Close(context.Background())
		channelAuthorization = nil
	}
	if channelAuthorization != nil {
		channelControl.SetChannelLoginAuthorizationCommitGuard(channelAuthorization)
	}
	permission.SetHumanToolApprovalRecorder(humanToolApprovalRouter{
		configuration: configurationService,
		connector:     connectorAuthorization,
	})
	slashCommandCatalog := slashcommandsvc.NewCatalog()
	slashCommandRegistry := slashcommandsvc.NewRegistry()
	if err := slashcommandsvc.RegisterModelCommand(
		slashCommandRegistry,
		slashcommandsvc.ModelCommandDependencies{
			Agents:      core.Agent,
			Sessions:    core.Session,
			Preferences: preferencesService,
			Providers:   providerService,
		},
	); err != nil {
		// 内置命令依赖由组合根静态装配；失败属于启动期编程错误。
		panic(err)
	}
	if err := slashcommandsvc.RegisterGoalCommand(
		slashCommandRegistry,
		slashcommandsvc.GoalCommandDependencies{Executor: goalCommandRouter{
			dm:    dmService,
			room:  roomRealtime,
			goals: goalService,
		}},
	); err != nil {
		// 内置命令依赖由组合根静态装配；失败属于启动期编程错误。
		panic(err)
	}

	// 把内置配置、平台通讯、自动化、授权、生成式 UI、图片生成和 Room 通讯 MCP server 注入 DM/Room runtime。
	configurationBuilder := newConfigurationMCPBuilder(configurationService, core.Agent)
	communicationService := communicationsvc.NewService(core.Agent, core.Room, roomRealtime, runtimeManager)
	communicationBuilder := newCommunicationMCPBuilder(communicationService, core.Agent)
	automationBuilder := newAutomationMCPBuilder(automationService, core.Agent, cfg.DefaultTimezone)
	connectorBuilder := newConnectorMCPBuilder(connectorService)
	connectorAuthorizationBuilder := newConnectorAuthorizationMCPBuilder(connectorAuthorization, core.Agent)
	channelAuthorizationBuilder := newChannelAuthorizationMCPBuilder(channelAuthorization, core.Agent)
	goalBuilder := newGoalMCPBuilder(cfg, explicitGoalCoordinator)
	visualizeBuilder := newVisualizeMCPBuilder()
	imagegenBuilder := newImagegenMCPBuilder(imagegenService)
	roomBuilder := newRoomMCPBuilder(roomRealtime, core.Room.GetRoom)
	executionBuilder := combinedExecutionMCPBuilder(
		newExecutionMCPBuilder(orchestrationService),
		newAutomationExecutionMCPBuilder(automationService, cfg.DefaultTimezone),
	)
	mcpBuilder := combinedMCPBuilder(
		configurationBuilder,
		communicationBuilder,
		automationBuilder,
		connectorBuilder,
		connectorAuthorizationBuilder,
		channelAuthorizationBuilder,
		goalBuilder,
		contextOnlyMCPBuilder(visualizeBuilder),
		contextOnlyMCPBuilder(imagegenBuilder),
		roundContextMCPBuilder(roomBuilder),
	)
	dmService.SetMCPServerBuilder(mcpBuilder)
	roomRealtime.SetMCPServerBuilder(mcpBuilder)
	dmService.SetExecutionMCPServerBuilder(executionBuilder)
	roomRealtime.SetExecutionMCPServerBuilder(executionBuilder)

	warnIfProviderMissing(providerService, logger)

	return &AppServices{
		DB:                     db,
		Core:                   core,
		Auth:                   authService,
		Provider:               providerService,
		Subscription:           subscriptionService,
		Preferences:            preferencesService,
		Workspace:              workspaceService,
		ProjectPermission:      projectPermissionService,
		Skills:                 skillService,
		Connectors:             connectorService,
		ConnectorAuthorization: connectorAuthorization,
		Configuration:          configurationService,
		Launcher:               launcherService,
		Title:                  titleService,
		Usage:                  usageService,
		Permission:             permission,
		Runtime:                runtimeManager,
		Channels:               channelRouter,
		ChannelControl:         channelControl,
		ChannelAuthorization:   channelAuthorization,
		Communication:          communicationService,
		DM:                     dmService,
		Ingress:                ingressService,
		RoomRealtime:           roomRealtime,
		Automation:             automationService,
		Imagegen:               imagegenService,
		Goal:                   goalService,
		Orchestration:          orchestrationService,
		Loops:                  loopService,
		MemoryMaintenance:      memoryMaintenance,
		SlashCatalog:           slashCommandCatalog,
		SlashRegistry:          slashCommandRegistry,
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
