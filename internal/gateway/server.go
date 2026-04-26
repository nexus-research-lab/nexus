package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/bootstrap"
	"github.com/nexus-research-lab/nexus/internal/config"
	agentgateway "github.com/nexus-research-lab/nexus/internal/gateway/agent"
	authgateway "github.com/nexus-research-lab/nexus/internal/gateway/auth"
	automationgateway "github.com/nexus-research-lab/nexus/internal/gateway/automation"
	capabilitygateway "github.com/nexus-research-lab/nexus/internal/gateway/capability"
	channelgateway "github.com/nexus-research-lab/nexus/internal/gateway/channel"
	connectorgateway "github.com/nexus-research-lab/nexus/internal/gateway/connector"
	coregateway "github.com/nexus-research-lab/nexus/internal/gateway/core"
	launchergateway "github.com/nexus-research-lab/nexus/internal/gateway/launcher"
	roomgateway "github.com/nexus-research-lab/nexus/internal/gateway/room"
	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	skillgateway "github.com/nexus-research-lab/nexus/internal/gateway/skill"
	gatewaywebsocket "github.com/nexus-research-lab/nexus/internal/gateway/websocket"
	workspacegateway "github.com/nexus-research-lab/nexus/internal/gateway/workspace"
	"github.com/nexus-research-lab/nexus/internal/logx"

	"github.com/go-chi/chi/v5"
)

// Server 提供基础 REST 与 WebSocket 网关骨架。
type Server struct {
	config config.Config
	logger *slog.Logger
	api    *gatewayshared.API
	router chi.Router

	auth       *authgateway.Handlers
	core       *coregateway.Handlers
	agent      *agentgateway.Handlers
	room       *roomgateway.Handlers
	capability *capabilitygateway.Handlers
	skill      *skillgateway.Handlers
	connector  *connectorgateway.Handlers
	channel    *channelgateway.Handlers
	automation *automationgateway.Handlers
	launcher   *launchergateway.Handlers
	workspace  *workspacegateway.Handlers
	websocket  *gatewaywebsocket.Handler

	channels   *bootstrap.AppServices
	httpRouter *chi.Mux
}

// NewServer 创建网关服务。
func NewServer(cfg config.Config) (*Server, error) {
	return NewServerWithLogger(cfg, nil)
}

// NewServerWithLogger 创建带显式 logger 的网关服务。
func NewServerWithLogger(cfg config.Config, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = logx.New(logx.Options{
			Service: cfg.ProjectName,
			Level:   cfg.LogLevel,
			Format:  cfg.LogFormat,
			Stdout:  cfg.LogStdout,
			NoColor: cfg.LogNoColor,
			File: logx.FileOptions{
				Enabled:     cfg.LogFileEnabled,
				Path:        cfg.LogPath,
				RotateDaily: cfg.LogRotateDaily,
				MaxSizeMB:   cfg.LogMaxSizeMB,
				MaxAgeDays:  cfg.LogMaxAgeDays,
				MaxBackups:  cfg.LogMaxBackups,
				Compress:    cfg.LogCompress,
			},
		})
	}

	appServices, err := bootstrap.NewAppServices(cfg, logger)
	if err != nil {
		return nil, err
	}

	api := gatewayshared.NewAPI(logger)
	runtimeProvider := func(agentID string) gatewaywebsocket.RuntimeSnapshot {
		runningCount := appServices.Runtime.CountRunningRounds(agentID)
		if appServices.RoomRealtime != nil {
			runningCount += appServices.RoomRealtime.CountRunningTasks(agentID)
		}
		status := "idle"
		if runningCount > 0 {
			status = "running"
		}
		return gatewaywebsocket.RuntimeSnapshot{
			AgentID:          agentID,
			RunningTaskCount: runningCount,
			Status:           status,
		}
	}
	websocketHandler := gatewaywebsocket.NewHandler(
		api,
		appServices.Core.Room,
		appServices.RoomRealtime,
		appServices.Chat,
		appServices.Permission,
		appServices.Runtime,
		appServices.Channels,
		appServices.Workspace,
		runtimeProvider,
	)

	server := &Server{
		config: cfg,
		logger: logger,
		api:    api,
		router: chi.NewRouter(),
		auth:   authgateway.New(api, appServices.Auth, appServices.Usage),
		core:   coregateway.New(api, appServices.Core.Agent, appServices.Provider, appServices.Preferences),
		agent: agentgateway.New(
			api,
			appServices.Core.Agent,
			appServices.Core.Session,
			appServices.Runtime,
			appServices.RoomRealtime,
			appServices.Preferences,
		),
		room: roomgateway.New(
			api,
			appServices.Core.Room,
			appServices.RoomRealtime,
			appServices.Core.Session,
			websocketHandler.BroadcastRoomEvent,
			websocketHandler.BroadcastRoomResyncRequired,
			websocketHandler.RemoveRoom,
		),
		capability: capabilitygateway.New(api, appServices.Skills, appServices.Connectors, appServices.Automation),
		skill:      skillgateway.New(api, appServices.Skills),
		connector:  connectorgateway.New(api, appServices.Connectors),
		channel:    channelgateway.New(api, appServices.Ingress),
		automation: automationgateway.New(api, appServices.Automation),
		launcher:   launchergateway.New(api, appServices.Launcher),
		workspace:  workspacegateway.New(api, appServices.Workspace),
		websocket:  websocketHandler,
		channels:   appServices,
	}

	server.router.Use(gatewayshared.RequestContextMiddleware(logger))
	server.router.Use(gatewayshared.AccessLogMiddleware())
	server.router.Use(gatewayshared.RecoverMiddleware(api))
	server.router.Use(gatewayshared.AuthMiddleware(api, appServices.Auth))
	server.mountRoutes()
	return server, nil
}

// Router 返回已初始化路由。
func (s *Server) Router() http.Handler {
	return s.router
}

// ListenAndServe 启动 HTTP 服务。
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.channels != nil && s.channels.Channels != nil {
		s.api.BaseLogger().Info("启动通道适配器",
			"discord_enabled", s.config.DiscordEnabled,
			"discord_configured", strings.TrimSpace(s.config.DiscordBotToken) != "",
			"telegram_enabled", s.config.TelegramEnabled,
			"telegram_configured", strings.TrimSpace(s.config.TelegramBotToken) != "",
			"registered_channels", s.channels.Channels.RegisteredChannelTypes(),
		)
		if err := s.channels.Channels.Start(ctx); err != nil {
			s.api.BaseLogger().Error("启动通道适配器失败", "err", err)
			return err
		}
		defer s.channels.Channels.Stop(context.Background())
	}
	if s.channels != nil && s.channels.Automation != nil {
		s.api.BaseLogger().Info("启动自动化调度器")
		if err := s.channels.Automation.Start(ctx); err != nil {
			s.api.BaseLogger().Error("启动自动化调度器失败", "err", err)
			return err
		}
		defer s.channels.Automation.Stop()
	}

	httpServer := &http.Server{
		Addr:              s.config.Address(),
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		s.api.BaseLogger().Info("收到停止信号，开始关闭 HTTP 服务")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	s.api.BaseLogger().Info("HTTP 服务开始监听",
		"addr", s.config.Address(),
		"api_prefix", s.config.APIPrefix,
		"websocket_path", s.config.WebSocketPath,
	)
	return httpServer.ListenAndServe()
}
