// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：server.go
// @Date   ：2026/04/17 10:30:00
// @Author ：leemysw
// 2026/04/17 10:30:00   Create
// =====================================================

package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/bootstrap"
	channels3 "github.com/nexus-research-lab/nexus/internal/channels"
	chatsvc "github.com/nexus-research-lab/nexus/internal/chat"
	"github.com/nexus-research-lab/nexus/internal/config"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/launcher"
	"github.com/nexus-research-lab/nexus/internal/logx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	providercfg "github.com/nexus-research-lab/nexus/internal/provider"
	room2 "github.com/nexus-research-lab/nexus/internal/room"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/session"
	skillsvc "github.com/nexus-research-lab/nexus/internal/skills"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"

	"github.com/go-chi/chi/v5"
)

// Server 提供基础 REST 与 WebSocket 网关骨架。
type Server struct {
	config        config.Config
	logger        *slog.Logger
	router        chi.Router
	agentService  *agent2.Service
	auth          *authsvc.Service
	providers     *providercfg.Service
	roomService   *room2.Service
	roomRealtime  *room2.RealtimeService
	roomSubs      *roomSubscriptionRegistry
	workspaceSubs *workspaceSubscriptionRegistry
	session       *sessionsvc.Service
	chat          *chatsvc.Service
	launcher      *launcher.Service
	workspace     *workspacepkg.Service
	skills        *skillsvc.Service
	connectors    *connectorsvc.Service
	automation    *automationsvc.Service
	channels      *channels3.Router
	ingress       channelIngress
	permission    *permissionctx.Context
	runtime       *runtimectx.Manager
}

type channelIngress interface {
	Accept(context.Context, channels3.IngressRequest) (*channels3.IngressResult, error)
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
	agentService := appServices.Core.Agent
	roomService := appServices.Core.Room
	sessionService := appServices.Core.Session
	authService := appServices.Auth
	providerService := appServices.Provider
	workspaceService := appServices.Workspace
	skillService := appServices.Skills
	connectorService := appServices.Connectors
	permission := appServices.Permission
	runtimeManager := appServices.Runtime
	channelRouter := appServices.Channels
	chatService := appServices.Chat
	ingressService := appServices.Ingress
	launcherService := appServices.Launcher
	roomRealtime := appServices.RoomRealtime
	roomSubs := newRoomSubscriptionRegistry(128)
	roomRealtime.SetRoomBroadcaster(roomSubs)
	sessionService.SetRuntimeManager(runtimeManager)
	workspaceSubs := newWorkspaceSubscriptionRegistry(workspaceService, func(agentID string) runtimeSnapshot {
		runningCount := runtimeManager.CountRunningRounds(agentID)
		if roomRealtime != nil {
			runningCount += roomRealtime.CountRunningTasks(agentID)
		}
		status := "idle"
		if runningCount > 0 {
			status = "running"
		}
		return runtimeSnapshot{
			AgentID:          agentID,
			RunningTaskCount: runningCount,
			Status:           status,
		}
	})
	automationService := appServices.Automation

	server := &Server{
		config:        cfg,
		logger:        logger,
		router:        chi.NewRouter(),
		agentService:  agentService,
		auth:          authService,
		providers:     providerService,
		roomService:   roomService,
		roomRealtime:  roomRealtime,
		roomSubs:      roomSubs,
		workspaceSubs: workspaceSubs,
		session:       sessionService,
		chat:          chatService,
		launcher:      launcherService,
		workspace:     workspaceService,
		skills:        skillService,
		connectors:    connectorService,
		automation:    automationService,
		channels:      channelRouter,
		ingress:       ingressService,
		permission:    permission,
		runtime:       runtimeManager,
	}
	server.router.Use(server.requestContextMiddleware)
	server.router.Use(server.accessLogMiddleware)
	server.router.Use(server.recoverMiddleware)
	server.router.Use(server.authMiddleware)
	server.mountRoutes()
	return server, nil
}

// Router 返回已初始化路由。
func (s *Server) Router() http.Handler {
	return s.router
}

// ListenAndServe 启动 http 服务。
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.channels != nil {
		s.baseLogger().Info("启动通道适配器",
			"discord_enabled", s.config.DiscordEnabled,
			"discord_configured", strings.TrimSpace(s.config.DiscordBotToken) != "",
			"telegram_enabled", s.config.TelegramEnabled,
			"telegram_configured", strings.TrimSpace(s.config.TelegramBotToken) != "",
			"registered_channels", s.channels.RegisteredChannelTypes(),
		)
		if err := s.channels.Start(ctx); err != nil {
			s.baseLogger().Error("启动通道适配器失败", "err", err)
			return err
		}
		defer s.channels.Stop(context.Background())
	}
	if s.automation != nil {
		s.baseLogger().Info("启动自动化调度器")
		if err := s.automation.Start(ctx); err != nil {
			s.baseLogger().Error("启动自动化调度器失败", "err", err)
			return err
		}
		defer s.automation.Stop()
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
		s.baseLogger().Info("收到停止信号，开始关闭 HTTP 服务")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	s.baseLogger().Info("HTTP 服务开始监听",
		"addr", s.config.Address(),
		"api_prefix", s.config.APIPrefix,
		"websocket_path", s.config.WebSocketPath,
	)
	return httpServer.ListenAndServe()
}
