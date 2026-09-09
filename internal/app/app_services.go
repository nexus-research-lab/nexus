// INPUT: 应用配置、数据库与基础服务依赖。
// OUTPUT: 完整 AppServices 依赖图、可选 Team Relay client、跨域 runtime 装配及自有数据库生命周期。
// POS: HTTP 与 CLI 共用的服务装配根；不持有 HTTP server。
package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	appexecution "github.com/nexus-research-lab/nexus/internal/app/execution"
	appgoal "github.com/nexus-research-lab/nexus/internal/app/goal"
	appruntime "github.com/nexus-research-lab/nexus/internal/app/runtime"
	appworkgraph "github.com/nexus-research-lab/nexus/internal/app/workgraph"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	goalcommandcontract "github.com/nexus-research-lab/nexus/internal/mcp/command/goal/contract"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/workspaceisolation"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	browsersvc "github.com/nexus-research-lab/nexus/internal/service/browser"
	channelauthorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	echosvc "github.com/nexus-research-lab/nexus/internal/service/echo"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/goalexecution"
	goalobjectivesvc "github.com/nexus-research-lab/nexus/internal/service/goalobjective"
	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
	"github.com/nexus-research-lab/nexus/internal/service/launcher"
	memorymaintenancesvc "github.com/nexus-research-lab/nexus/internal/service/memorymaintenance"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	projectpermissionsvc "github.com/nexus-research-lab/nexus/internal/service/projectpermission"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	teamrelaystore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
	workgraphworkflowstore "github.com/nexus-research-lab/nexus/internal/storage/workgraphworkflow"
)

// AppServices 表示完整应用运行所需的核心依赖容器。
type AppServices struct {
	DB                     *sql.DB
	Core                   *CoreServices
	Auth                   authsvc.Authority
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
	Echo                   *echosvc.Service
	Ingress                *channels.IngressService
	RoomRealtime           *roomrealtime.Service
	Automation             *automationsvc.Service
	Imagegen               *imagegensvc.Service
	Goal                   *goalsvc.Service
	GoalCommand            goalcommandcontract.Service
	Orchestration          *orchestrationsvc.Service
	WorkGraphWorkflow      *workgraphworkflowsvc.Service
	MemoryMaintenance      *memorymaintenancesvc.Coordinator
	Browser                *browsersvc.Service
	Relay                  *relaysvc.Client
	TeamRelay              *teamrelaystore.Repository
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
	if s.Browser != nil {
		s.Browser.Close()
	}
	if s.ownsDB && s.DB != nil {
		closeErrors = append(closeErrors, s.DB.Close())
	}
	return errors.Join(closeErrors...)
}

// NewAppServices 创建完整应用依赖容器。
func NewAppServices(cfg config.Config, logger *slog.Logger) (*AppServices, error) {
	relayClient, err := newOptionalRelayClient(cfg)
	if err != nil {
		return nil, err
	}
	db, err := OpenDB(cfg)
	if err != nil {
		return nil, err
	}
	services := NewAppServicesWithDB(cfg, db, logger)
	services.Relay = relayClient
	services.ownsDB = true
	return services, nil
}

func newOptionalRelayClient(cfg config.Config) (*relaysvc.Client, error) {
	if strings.TrimSpace(cfg.RelayURL) == "" {
		return nil, nil
	}
	return relaysvc.NewClient(
		cfg.RelayURL,
		time.Duration(cfg.RelayRequestTimeoutSeconds)*time.Second,
	)
}

// NewAppServicesWithDB 使用共享 DB 创建完整应用依赖容器。
func NewAppServicesWithDB(cfg config.Config, db *sql.DB, logger *slog.Logger) *AppServices {
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	core := NewCoreServicesWithDB(cfg, db)
	var authService authsvc.Authority
	usageService := usagesvc.NewServiceWithDB(cfg, db)
	providerService := providercfg.NewServiceWithDB(cfg, db)
	providerService.SetLogger(logger.With("component", "provider"))
	subscriptionService := subscriptionsvc.NewServiceWithDB(cfg, db)
	goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	goalService.SetLogger(logger.With("component", "goal"))
	goalService.SetSessionOwnershipVerifier(appgoal.NewSessionOwnershipVerifier(
		core.Agent,
		core.Room,
	))
	orchestrationService := orchestrationsvc.NewService(orchestrationstore.NewRepository(cfg, db))
	workGraphWorkflowService := workgraphworkflowsvc.NewService(
		workgraphworkflowstore.NewRepository(cfg, db),
		orchestrationService,
	)
	orchestrationService.SetRuntimeGraphSubagentToolHistoryProvider(
		appexecution.NewSubagentToolHistory(core.Session),
	)
	explicitGoalCoordinator := goalexecution.NewExplicitExecutionCoordinator(goalService, orchestrationService)
	goalService.SetObjectiveRetargetCoordinator(explicitGoalCoordinator)
	orchestrationService.SetExplicitGoalBindingGateway(explicitGoalCoordinator)
	orchestrationService.SetGoalPromotionGateway(goalexecution.NewExecutionPromotionGateway(cfg, goalService))
	goalService.SetExecutionGoalCompletionReadiness(goalexecution.NewExecutionCompletionReadiness(orchestrationService))
	preferencesService := preferencessvc.NewService(cfg)
	workGraphWorkflowService.SetAbstractor(
		workgraphworkflowsvc.NewLLMAbstractor(providerService, preferencesService),
	)
	workGraphWorkflowService.SetMainAgentResolver(core.Agent)
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
	var browserService *browsersvc.Service
	if cfg.BrowserEnabled {
		browserService = browsersvc.NewService()
	}
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
	if browserService != nil {
		runtimeManager.SetRoundFinishedObserver(func(sessionKey string, roundID string) {
			go func() {
				if err := browserService.FinalizeRound(context.Background(), sessionKey, roundID); err != nil {
					logger.Warn("Browser round 收尾失败", "session_key", sessionKey, "round_id", roundID, "err", err)
				}
			}()
		})
	}
	if strings.TrimSpace(cfg.ControlURL) != "" &&
		!strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") {
		authService = authsvc.NewControlAuthority(cfg, db, nil)
	} else {
		authService = authsvc.NewLocalAuthority(cfg.DatabaseDriver, db, nil)
	}
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
	dmService.SetConnectorRuntimeStateLoader(func(
		ctx context.Context,
		ownerUserID string,
	) ([]dmsvc.ConnectorRuntimeState, error) {
		items, err := connectorService.ListConnectors(ctx, ownerUserID, "", "", "available")
		if err != nil {
			return nil, err
		}
		states := make([]dmsvc.ConnectorRuntimeState, 0, len(items))
		for _, item := range items {
			if item.Kind != connectorsvc.ConnectorKindCatalog {
				continue
			}
			states = append(states, dmsvc.ConnectorRuntimeState{
				ConnectorID: item.ConnectorID,
				Configured:  item.ConnectionState == "connected",
			})
		}
		return states, nil
	})
	dmService.SetUsageRecorder(usageService)
	dmService.SetQuotaChecker(subscriptionService)
	dmService.SetGoalContextProvider(goalService)
	dmService.SetExecutionContextProvider(orchestrationService)
	dmService.SetRuntimeSlashExpander(workGraphWorkflowService)
	dmService.SetScopedSessionRuntimePolicyProvider(workGraphWorkflowService)
	dmService.SetSubagentAdmissionProvider(orchestrationService)
	dmService.SetRuntimeAdmissionResolver(authService)
	dmService.SetQueueAdmissionStore(queueAdmissionRepository)
	dmService.SetRoomSessionStore(newSessionRepository(cfg, db))
	dmService.SetRoomConversationActivityStore(core.Room)
	core.Room.SetConversationSessionForker(dmService)
	workGraphWorkflowService.SetEditorSessionManager(
		appworkgraph.NewEditorSessionManager(dmService, core.Session),
	)
	dmService.SetTitleGenerator(titleService)
	dmService.SetExternalReplyDispatcher(dmExternalReplyDispatcher{router: channelRouter})
	echoService := echosvc.NewService(
		cfg,
		db,
		core.Agent,
		core.Session,
		dmService,
		runtimeManager,
		providerService,
		preferencesService,
	)
	echoService.SetLogger(logger.With("component", "echo"))
	dmService.SetEchoLifecycleHooks(dmsvc.EchoLifecycleHooks{
		OnUserActivity: echoService.OnUserActivity,
		OnTerminal:     echoService.OnTerminal,
	})
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
	roomRealtime.SetRuntimeSlashExpander(workGraphWorkflowService)
	roomRealtime.SetSubagentAdmissionProvider(orchestrationService)
	roomRealtime.SetRuntimeAdmissionResolver(authService)
	roomRealtime.SetQueueAdmissionStore(queueAdmissionRepository)
	roomRealtime.SetTitleGenerator(titleService)
	orchestrationService.SetAssignmentTargetAuthorizer(roomRealtime)
	orchestrationService.SetExecutionDispatchConsumer(roomRealtime)
	orchestrationService.SetExecutionReviewDispatchConsumer(roomRealtime)
	orchestrationService.SetExecutionCancellationConsumer(
		appexecution.NewCancellationConsumer(roomRealtime, runtimeManager),
	)
	goalService.SetRoomGoalCompletionReadiness(roomRealtime)
	goalService.SetGuidanceDispatcher(appgoal.NewGuidanceDispatcher(runtimeManager, roomRealtime))
	goalService.SetRuntimeInterrupter(appgoal.NewInterruptDispatcher(dmService, roomRealtime))
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
	configurationRuntimeEnvironmentBuilder := appruntime.NewConfigurationEnvironmentBuilder(
		cfg,
		configurationService,
		core.Agent,
	)
	dmService.SetConfigurationRuntimeEnvironmentBuilder(configurationRuntimeEnvironmentBuilder)
	roomRealtime.SetConfigurationRuntimeEnvironmentBuilder(configurationRuntimeEnvironmentBuilder)
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
	permission.SetHumanToolApprovalRecorder(appruntime.NewHumanToolApprovalRouter(
		configurationService,
		connectorAuthorization,
	))
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
		slashcommandsvc.GoalCommandDependencies{
			Executor: appgoal.NewCommandRouter(dmService, roomRealtime, goalService),
		},
	); err != nil {
		// 内置命令依赖由组合根静态装配；失败属于启动期编程错误。
		panic(err)
	}

	// 宿主自有工具按 physical round 合并到唯一 nexus MCP；Connector MCP 保持独立生命周期。
	communicationService := communicationsvc.NewService(
		core.Agent, core.Room, roomRealtime, runtimeManager, channelControl,
	)
	connectorBuilder := appruntime.NewConnectorBuilder(connectorService)
	builtInTools := appruntime.CombineToolBuilders(
		appruntime.NewCommunicationToolBuilder(
			communicationService,
			roomRealtime,
			core.Agent,
			core.Room.GetRoom,
		),
		appruntime.NewConnectorAuthorizationToolBuilder(connectorAuthorization, core.Agent),
		appruntime.NewChannelAuthorizationToolBuilder(channelAuthorization, core.Agent),
		appruntime.NewVisualizeToolBuilder(),
		appruntime.NewImagegenToolBuilder(imagegenService, providerService),
		appruntime.NewBrowserToolBuilder(browserService, preferencesService),
	)
	nexusMCPBuilder := appruntime.NewServerBuilder(
		cfg,
		core.Agent,
		automationService,
		explicitGoalCoordinator,
		orchestrationService,
		permission,
		builtInTools,
		workGraphWorkflowService,
	)
	dmService.SetMCPServerBuilder(dmsvc.MCPServerBuilder(connectorBuilder))
	roomRealtime.SetMCPServerBuilder(roomrealtime.MCPServerBuilder(connectorBuilder))
	dmService.SetNexusMCPServerBuilder(nexusMCPBuilder)
	roomRealtime.SetNexusMCPServerBuilder(nexusMCPBuilder)
	core.Session.SetRuntimeSettingsPreparationScheduler(dmService)

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
		Echo:                   echoService,
		Ingress:                ingressService,
		RoomRealtime:           roomRealtime,
		Automation:             automationService,
		Imagegen:               imagegenService,
		Goal:                   goalService,
		GoalCommand:            explicitGoalCoordinator,
		Orchestration:          orchestrationService,
		WorkGraphWorkflow:      workGraphWorkflowService,
		MemoryMaintenance:      memoryMaintenance,
		Browser:                browserService,
		TeamRelay:              teamrelaystore.NewRepository(cfg, db),
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
