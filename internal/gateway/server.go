// =====================================================
// @File   ：server.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	agent2 "github.com/nexus-research-lab/nexus-core/internal/agent"
	authsvc "github.com/nexus-research-lab/nexus-core/internal/auth"
	automationsvc "github.com/nexus-research-lab/nexus-core/internal/automation"
	channels3 "github.com/nexus-research-lab/nexus-core/internal/channels"
	chatsvc "github.com/nexus-research-lab/nexus-core/internal/chat"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	connectorsvc "github.com/nexus-research-lab/nexus-core/internal/connectors"
	"github.com/nexus-research-lab/nexus-core/internal/launcher"
	"github.com/nexus-research-lab/nexus-core/internal/logx"
	permissionctx "github.com/nexus-research-lab/nexus-core/internal/permission"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus-core/internal/providerconfig"
	room2 "github.com/nexus-research-lab/nexus-core/internal/room"
	runtimectx "github.com/nexus-research-lab/nexus-core/internal/runtime"
	sessionsvc "github.com/nexus-research-lab/nexus-core/internal/session"
	skillsvc "github.com/nexus-research-lab/nexus-core/internal/skills"
	"github.com/nexus-research-lab/nexus-core/internal/storage"
	workspacepkg "github.com/nexus-research-lab/nexus-core/internal/workspace"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
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
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return nil, err
	}
	agentService := agent2.NewServiceWithDB(cfg, db)
	authService := authsvc.NewServiceWithDB(cfg, db)
	providerService := providercfg.NewServiceWithDB(cfg, db)
	roomService := room2.NewServiceWithDB(cfg, db)
	sessionService := sessionsvc.NewServiceWithDB(cfg, db, agentService)
	workspaceService := workspacepkg.NewService(cfg, agentService)
	skillService := skillsvc.NewService(cfg, agentService, workspaceService)
	connectorService := connectorsvc.NewService(cfg, db)
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	channelRouter := channels3.NewRouter(cfg, db, agentService, permission)
	channelRouter.SetLogger(logger.With("component", "channels"))
	chatService := chatsvc.NewService(cfg, agentService, runtimeManager, permission)
	chatService.SetLogger(logger.With("component", "chat"))
	chatService.SetProviderResolver(providerService)
	ingressService := channels3.NewIngressService(cfg, agentService, chatService, channelRouter)
	ingressService.SetLogger(logger.With("component", "channels.ingress"))
	channelRouter.SetIngress(ingressService)
	launcherService := launcher.NewService(cfg, agentService, roomService)
	roomRealtime := room2.NewRealtimeService(cfg, roomService, agentService, runtimeManager, permission)
	roomRealtime.SetLogger(logger.With("component", "room"))
	roomRealtime.SetProviderResolver(providerService)
	roomSubs := newRoomSubscriptionRegistry(128)
	roomRealtime.SetRoomBroadcaster(roomSubs)
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
	automationService := automationsvc.NewService(cfg, db, agentService, chatService, roomRealtime, permission, workspaceService, channelRouter)
	automationService.SetLogger(logger.With("component", "automation"))

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
		s.baseLogger().Info("启动通道适配器")
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

func (s *Server) mountRoutes() {
	s.router.Get("/agent/v1/health", s.handleHealth)
	s.router.Get("/agent/v1/auth/status", s.handleAuthStatus)
	s.router.Post("/agent/v1/auth/login", s.handleAuthLogin)
	s.router.Post("/agent/v1/auth/logout", s.handleAuthLogout)
	s.router.Get("/agent/v1/runtime/options", s.handleRuntimeOptions)
	s.router.Get("/agent/v1/settings/providers", s.handleListProviderConfigs)
	s.router.Get("/agent/v1/settings/providers/options", s.handleListProviderOptions)
	s.router.Post("/agent/v1/settings/providers", s.handleCreateProviderConfig)
	s.router.Put("/agent/v1/settings/providers/{provider}", s.handleUpdateProviderConfig)
	s.router.Delete("/agent/v1/settings/providers/{provider}", s.handleDeleteProviderConfig)
	s.router.Get("/agent/v1/chat/ws", s.handleWebSocket)
	s.router.Get("/agent/v1/agents", s.handleListAgents)
	s.router.Get("/agent/v1/agents/runtime/statuses", s.handleAgentRuntimeStatuses)
	s.router.Post("/agent/v1/agents", s.handleCreateAgent)
	s.router.Get("/agent/v1/agents/validate/name", s.handleValidateAgentName)
	s.router.Get("/agent/v1/agents/{agent_id}", s.handleGetAgent)
	s.router.Patch("/agent/v1/agents/{agent_id}", s.handleUpdateAgent)
	s.router.Delete("/agent/v1/agents/{agent_id}", s.handleDeleteAgent)
	s.router.Get("/agent/v1/agents/{agent_id}/sessions", s.handleListAgentSessions)
	s.router.Get("/agent/v1/agents/{agent_id}/cost/summary", s.handleAgentCostSummary)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/files", s.handleWorkspaceFiles)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/file", s.handleWorkspaceFile)
	s.router.Put("/agent/v1/agents/{agent_id}/workspace/file", s.handleUpdateWorkspaceFile)
	s.router.Post("/agent/v1/agents/{agent_id}/workspace/upload", s.handleUploadWorkspaceFile)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/download", s.handleDownloadWorkspaceFile)
	s.router.Post("/agent/v1/agents/{agent_id}/workspace/entry", s.handleCreateWorkspaceEntry)
	s.router.Patch("/agent/v1/agents/{agent_id}/workspace/entry", s.handleRenameWorkspaceEntry)
	s.router.Delete("/agent/v1/agents/{agent_id}/workspace/entry", s.handleDeleteWorkspaceEntry)
	s.router.Get("/agent/v1/agents/{agent_id}/skills", s.handleAgentSkills)
	s.router.Post("/agent/v1/agents/{agent_id}/skills", s.handleInstallAgentSkill)
	s.router.Delete("/agent/v1/agents/{agent_id}/skills/{skill_name}", s.handleUninstallAgentSkill)
	s.router.Get("/agent/v1/sessions", s.handleListSessions)
	s.router.Post("/agent/v1/sessions", s.handleCreateSession)
	s.router.Patch("/agent/v1/sessions/{session_key}", s.handleUpdateSession)
	s.router.Get("/agent/v1/sessions/{session_key}/messages", s.handleSessionMessages)
	s.router.Get("/agent/v1/sessions/{session_key}/cost/summary", s.handleSessionCostSummary)
	s.router.Delete("/agent/v1/sessions/{session_key}", s.handleDeleteSession)
	s.router.Get("/agent/v1/rooms/dm/{agent_id}", s.handleEnsureDirectRoom)
	s.router.Get("/agent/v1/rooms", s.handleListRooms)
	s.router.Post("/agent/v1/rooms", s.handleCreateRoom)
	s.router.Get("/agent/v1/rooms/{room_id}", s.handleGetRoom)
	s.router.Patch("/agent/v1/rooms/{room_id}", s.handleUpdateRoom)
	s.router.Delete("/agent/v1/rooms/{room_id}", s.handleDeleteRoom)
	s.router.Get("/agent/v1/rooms/{room_id}/contexts", s.handleGetRoomContexts)
	s.router.Post("/agent/v1/rooms/{room_id}/members", s.handleAddRoomMember)
	s.router.Delete("/agent/v1/rooms/{room_id}/members/{agent_id}", s.handleRemoveRoomMember)
	s.router.Post("/agent/v1/rooms/{room_id}/conversations", s.handleCreateConversation)
	s.router.Patch("/agent/v1/rooms/{room_id}/conversations/{conversation_id}", s.handleUpdateConversation)
	s.router.Delete("/agent/v1/rooms/{room_id}/conversations/{conversation_id}", s.handleDeleteConversation)
	s.router.Post("/agent/v1/launcher/query", s.handleLauncherQuery)
	s.router.Get("/agent/v1/launcher/suggestions", s.handleLauncherSuggestions)
	s.router.Get("/agent/v1/skills", s.handleListSkills)
	s.router.Get("/agent/v1/skills/{skill_name}", s.handleGetSkillDetail)
	s.router.Post("/agent/v1/skills/import/local", s.handleImportLocalSkill)
	s.router.Post("/agent/v1/skills/import/git", s.handleImportGitSkill)
	s.router.Get("/agent/v1/skills/search/external", s.handleSearchExternalSkills)
	s.router.Get("/agent/v1/skills/external/preview", s.handlePreviewExternalSkill)
	s.router.Post("/agent/v1/skills/import/skills-sh", s.handleImportSkillsShSkill)
	s.router.Post("/agent/v1/skills/update-imported", s.handleUpdateImportedSkills)
	s.router.Post("/agent/v1/skills/{skill_name}/update", s.handleUpdateSingleSkill)
	s.router.Delete("/agent/v1/skills/{skill_name}", s.handleDeleteSkill)
	s.router.Get("/agent/v1/connectors", s.handleListConnectors)
	s.router.Get("/agent/v1/connectors/categories", s.handleConnectorCategories)
	s.router.Get("/agent/v1/connectors/count", s.handleConnectorCount)
	s.router.Get("/agent/v1/connectors/{connector_id}", s.handleConnectorDetail)
	s.router.Get("/agent/v1/connectors/{connector_id}/auth-url", s.handleConnectorAuthURL)
	s.router.Post("/agent/v1/connectors/oauth/callback", s.handleConnectorOAuthCallback)
	s.router.Post("/agent/v1/connectors/{connector_id}/connect", s.handleConnectConnector)
	s.router.Post("/agent/v1/connectors/{connector_id}/disconnect", s.handleDisconnectConnector)
	s.router.Post("/agent/v1/channels/messages", s.handleChannelIngress)
	s.router.Post("/agent/v1/channels/internal/messages", s.handleInternalChannelIngress)
	s.router.Post("/agent/v1/channels/discord/messages", s.handleDiscordChannelIngress)
	s.router.Post("/agent/v1/channels/telegram/messages", s.handleTelegramChannelIngress)
	s.router.Get("/agent/v1/capability/scheduled/tasks", s.handleListScheduledTasks)
	s.router.Post("/agent/v1/capability/scheduled/tasks", s.handleCreateScheduledTask)
	s.router.Patch("/agent/v1/capability/scheduled/tasks/{job_id}", s.handleUpdateScheduledTask)
	s.router.Delete("/agent/v1/capability/scheduled/tasks/{job_id}", s.handleDeleteScheduledTask)
	s.router.Post("/agent/v1/capability/scheduled/tasks/{job_id}/run", s.handleRunScheduledTask)
	s.router.Patch("/agent/v1/capability/scheduled/tasks/{job_id}/status", s.handleUpdateScheduledTaskStatus)
	s.router.Get("/agent/v1/capability/scheduled/tasks/{job_id}/runs", s.handleListScheduledTaskRuns)
	s.router.Get("/agent/v1/scheduled/tasks", s.handleListScheduledTasks)
	s.router.Post("/agent/v1/scheduled/tasks", s.handleCreateScheduledTask)
	s.router.Patch("/agent/v1/scheduled/tasks/{job_id}", s.handleUpdateScheduledTask)
	s.router.Delete("/agent/v1/scheduled/tasks/{job_id}", s.handleDeleteScheduledTask)
	s.router.Post("/agent/v1/scheduled/tasks/{job_id}/run", s.handleRunScheduledTask)
	s.router.Patch("/agent/v1/scheduled/tasks/{job_id}/status", s.handleUpdateScheduledTaskStatus)
	s.router.Get("/agent/v1/scheduled/tasks/{job_id}/runs", s.handleListScheduledTaskRuns)
	s.router.Get("/agent/v1/automation/heartbeat/{agent_id}", s.handleGetHeartbeat)
	s.router.Put("/agent/v1/automation/heartbeat/{agent_id}", s.handleUpdateHeartbeat)
	s.router.Post("/agent/v1/automation/heartbeat/{agent_id}/wake", s.handleWakeHeartbeat)

	for _, group := range []string{} {
		s.mountPlaceholderGroup(group)
	}
}

func (s *Server) mountPlaceholderGroup(group string) {
	base := strings.TrimPrefix(group, "/")
	s.router.HandleFunc("/agent/v1/"+base, s.handleNotImplemented(group))
	s.router.HandleFunc("/agent/v1/"+base+"/*", s.handleNotImplemented(group))
}

func (s *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	s.writeJSON(writer, http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "ok",
		"data": map[string]any{
			"status": "ok",
		},
	})
}

func (s *Server) handleRuntimeOptions(writer http.ResponseWriter, request *http.Request) {
	if err := s.agentService.EnsureReady(request.Context()); err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(writer, http.StatusOK, map[string]any{
		"code":    "0000",
		"message": "success",
		"success": true,
		"data": map[string]any{
			"default_agent_id": s.config.DefaultAgentID,
		},
	})
}

func (s *Server) handleListAgents(writer http.ResponseWriter, request *http.Request) {
	agents, err := s.agentService.ListAgents(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, agents)
}

func (s *Server) handleAgentRuntimeStatuses(writer http.ResponseWriter, request *http.Request) {
	agents, err := s.agentService.ListAgents(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	statuses := make([]map[string]any, 0, len(agents))
	for _, item := range agents {
		runningCount := s.runtime.CountRunningRounds(item.AgentID)
		if s.roomRealtime != nil {
			runningCount += s.roomRealtime.CountRunningTasks(item.AgentID)
		}
		status := "idle"
		if runningCount > 0 {
			status = "running"
		}
		statuses = append(statuses, map[string]any{
			"agent_id":           item.AgentID,
			"running_task_count": runningCount,
			"status":             status,
		})
	}
	s.writeSuccess(writer, statuses)
}

func (s *Server) handleGetAgent(writer http.ResponseWriter, request *http.Request) {
	agentID := chi.URLParam(request, "agent_id")
	agentValue, err := s.agentService.GetAgent(request.Context(), agentID)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, agentValue)
}

func (s *Server) handleValidateAgentName(writer http.ResponseWriter, request *http.Request) {
	name := request.URL.Query().Get("name")
	excludeAgentID := request.URL.Query().Get("exclude_agent_id")
	result, err := s.agentService.ValidateName(request.Context(), name, excludeAgentID)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, result)
}

func (s *Server) handleCreateAgent(writer http.ResponseWriter, request *http.Request) {
	var payload agent2.CreateRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}

	created, err := s.agentService.CreateAgent(request.Context(), payload)
	if err != nil {
		if strings.Contains(err.Error(), "名称") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, created)
}

func (s *Server) handleListAgentSessions(writer http.ResponseWriter, request *http.Request) {
	items, err := s.session.ListAgentSessions(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleAgentCostSummary(writer http.ResponseWriter, request *http.Request) {
	item, err := s.session.GetAgentCostSummary(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleListSessions(writer http.ResponseWriter, request *http.Request) {
	items, err := s.session.ListSessions(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleCreateSession(writer http.ResponseWriter, request *http.Request) {
	var payload sessionsvc.CreateRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.session.CreateSession(request.Context(), payload)
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionMutationUnsupported) || isClientMessageError(err) {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateSession(writer http.ResponseWriter, request *http.Request) {
	var payload sessionsvc.UpdateRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.session.UpdateSession(request.Context(), chi.URLParam(request, "session_key"), payload)
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionMutationUnsupported) || isClientMessageError(err) {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleSessionMessages(writer http.ResponseWriter, request *http.Request) {
	items, err := s.session.GetSessionMessages(request.Context(), chi.URLParam(request, "session_key"))
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleSessionCostSummary(writer http.ResponseWriter, request *http.Request) {
	item, err := s.session.GetSessionCostSummary(request.Context(), chi.URLParam(request, "session_key"))
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteSession(writer http.ResponseWriter, request *http.Request) {
	err := s.session.DeleteSession(request.Context(), chi.URLParam(request, "session_key"))
	if isStructuredSessionKeyError(err) {
		s.writeFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, sessionsvc.ErrSessionMutationUnsupported) || isClientMessageError(err) {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}

func (s *Server) handleListRooms(writer http.ResponseWriter, request *http.Request) {
	limit := 20
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := s.roomService.ListRooms(request.Context(), limit)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleGetRoom(writer http.ResponseWriter, request *http.Request) {
	item, err := s.roomService.GetRoom(request.Context(), chi.URLParam(request, "room_id"))
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleGetRoomContexts(writer http.ResponseWriter, request *http.Request) {
	items, err := s.roomService.GetRoomContexts(request.Context(), chi.URLParam(request, "room_id"))
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleCreateRoom(writer http.ResponseWriter, request *http.Request) {
	var payload room2.CreateRoomRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.roomService.CreateRoom(request.Context(), payload)
	if errors.Is(err, room2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateRoom(writer http.ResponseWriter, request *http.Request) {
	var payload room2.UpdateRoomRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.roomService.UpdateRoom(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomResyncRequired(request.Context(), item.Room.ID, item.Conversation.ID, "room_updated")
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteRoom(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	if s.roomRealtime != nil {
		_ = s.roomRealtime.InterruptRoom(request.Context(), roomID, "room 已删除")
	}
	err := s.roomService.DeleteRoom(request.Context(), roomID)
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomEvent(request.Context(), roomID, protocol.EventTypeRoomDeleted, map[string]any{
		"room_id": roomID,
	})
	if s.roomSubs != nil {
		s.roomSubs.RemoveRoom(roomID)
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}

func (s *Server) handleEnsureDirectRoom(writer http.ResponseWriter, request *http.Request) {
	item, err := s.roomService.EnsureDirectRoom(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, room2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleAddRoomMember(writer http.ResponseWriter, request *http.Request) {
	var payload room2.AddRoomMemberRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.roomService.AddRoomMember(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, room2.ErrRoomNotFound) || errors.Is(err, room2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomEvent(request.Context(), item.Room.ID, protocol.EventTypeRoomMemberAdded, map[string]any{
		"room_id":  item.Room.ID,
		"agent_id": payload.AgentID,
	})
	s.writeSuccess(writer, item)
}

func (s *Server) handleRemoveRoomMember(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	agentID := chi.URLParam(request, "agent_id")
	if s.roomRealtime != nil {
		_ = s.roomRealtime.InterruptAgentTasks(request.Context(), roomID, agentID, "成员已移出 room")
	}
	item, err := s.roomService.RemoveRoomMember(request.Context(), roomID, agentID)
	if errors.Is(err, room2.ErrRoomNotFound) || errors.Is(err, room2.ErrRoomMemberNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomEvent(request.Context(), item.Room.ID, protocol.EventTypeRoomMemberRemoved, map[string]any{
		"room_id":  item.Room.ID,
		"agent_id": agentID,
	})
	s.writeSuccess(writer, item)
}

func (s *Server) handleCreateConversation(writer http.ResponseWriter, request *http.Request) {
	var payload room2.CreateConversationRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.roomService.CreateConversation(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, room2.ErrRoomNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomResyncRequired(request.Context(), item.Room.ID, item.Conversation.ID, "conversation_created")
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateConversation(writer http.ResponseWriter, request *http.Request) {
	var payload room2.UpdateConversationRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.roomService.UpdateConversation(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
		payload,
	)
	if errors.Is(err, room2.ErrRoomNotFound) || errors.Is(err, room2.ErrConversationNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomResyncRequired(request.Context(), item.Room.ID, item.Conversation.ID, "conversation_updated")
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteConversation(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	conversationID := chi.URLParam(request, "conversation_id")
	if s.roomRealtime != nil {
		_ = s.roomRealtime.InterruptConversation(request.Context(), conversationID, "对话已删除")
	}
	item, err := s.roomService.DeleteConversation(
		request.Context(),
		roomID,
		conversationID,
	)
	if errors.Is(err, room2.ErrRoomNotFound) || errors.Is(err, room2.ErrConversationNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastRoomResyncRequired(request.Context(), roomID, conversationID, "conversation_deleted")
	s.writeSuccess(writer, item)
}

func (s *Server) handleLauncherQuery(writer http.ResponseWriter, request *http.Request) {
	var payload launcher.QueryRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.launcher.Query(request.Context(), payload.Query)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleLauncherSuggestions(writer http.ResponseWriter, request *http.Request) {
	item, err := s.launcher.Suggestions(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleWorkspaceFiles(writer http.ResponseWriter, request *http.Request) {
	items, err := s.workspace.ListFiles(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	item, err := s.workspace.GetFile(request.Context(), chi.URLParam(request, "agent_id"), request.URL.Query().Get("path"))
	if errors.Is(err, agent2.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if isClientMessageError(err) || strings.Contains(err.Error(), "文件路径") || strings.Contains(err.Error(), "目录") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.workspace.UpdateFile(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.Content)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "文件路径") || strings.Contains(err.Error(), "目录") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleCreateWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path      string `json:"path"`
		EntryType string `json:"entry_type"`
		Content   string `json:"content"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.workspace.CreateEntry(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.EntryType, payload.Content)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "存在") || strings.Contains(err.Error(), "仅支持") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleRenameWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.workspace.RenameEntry(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.NewPath)
	if errors.Is(err, agent2.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "相同") || strings.Contains(err.Error(), "存在") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	item, err := s.workspace.DeleteEntry(request.Context(), chi.URLParam(request, "agent_id"), request.URL.Query().Get("path"))
	if errors.Is(err, agent2.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleListSkills(writer http.ResponseWriter, request *http.Request) {
	items, err := s.skills.ListSkills(request.Context(), skillsvc.Query{
		AgentID:     request.URL.Query().Get("agent_id"),
		CategoryKey: request.URL.Query().Get("category_key"),
		SourceType:  request.URL.Query().Get("source_type"),
		Q:           request.URL.Query().Get("q"),
	})
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleGetSkillDetail(writer http.ResponseWriter, request *http.Request) {
	item, err := s.skills.GetSkillDetail(request.Context(), chi.URLParam(request, "skill_name"), request.URL.Query().Get("agent_id"))
	if errors.Is(err, agent2.ErrAgentNotFound) || strings.Contains(strings.ToLower(errString(err)), "not found") {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleAgentSkills(writer http.ResponseWriter, request *http.Request) {
	items, err := s.skills.GetAgentSkills(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleInstallAgentSkill(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		SkillName string `json:"skill_name"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.skills.InstallSkill(request.Context(), chi.URLParam(request, "agent_id"), payload.SkillName)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "不能") || strings.Contains(err.Error(), "仅允许") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUninstallAgentSkill(writer http.ResponseWriter, request *http.Request) {
	err := s.skills.UninstallSkill(request.Context(), chi.URLParam(request, "agent_id"), chi.URLParam(request, "skill_name"))
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "不能") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}

func (s *Server) handleImportLocalSkill(writer http.ResponseWriter, request *http.Request) {
	filePayload, filename, localPath, err := s.parseLocalSkillImportRequest(request)
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	var item *skillsvc.Detail
	if len(filePayload) > 0 {
		item, err = s.skills.ImportUploadedArchive(filename, filePayload)
	} else {
		item, err = s.skills.ImportLocalPath(localPath)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "SKILL.md") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDeleteSkill(writer http.ResponseWriter, request *http.Request) {
	err := s.skills.DeleteSkill(request.Context(), chi.URLParam(request, "skill_name"))
	if err != nil {
		if strings.Contains(err.Error(), "不允许") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"success": true})
}

func (s *Server) handleListConnectors(writer http.ResponseWriter, request *http.Request) {
	items, err := s.connectors.ListConnectors(
		request.Context(),
		request.URL.Query().Get("q"),
		request.URL.Query().Get("category"),
		request.URL.Query().Get("status"),
	)
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, items)
}

func (s *Server) handleConnectorCategories(writer http.ResponseWriter, request *http.Request) {
	s.writeSuccess(writer, s.connectors.GetCategories())
}

func (s *Server) handleConnectorCount(writer http.ResponseWriter, request *http.Request) {
	count, err := s.connectors.GetConnectedCount(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, map[string]any{"count": count})
}

func (s *Server) handleConnectorDetail(writer http.ResponseWriter, request *http.Request) {
	item, err := s.connectors.GetConnectorDetail(request.Context(), chi.URLParam(request, "connector_id"))
	if strings.Contains(strings.ToLower(errString(err)), "not found") {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleConnectorAuthURL(writer http.ResponseWriter, request *http.Request) {
	item, err := s.connectors.GetAuthURL(request.Context(), chi.URLParam(request, "connector_id"), request.URL.Query().Get("redirect_uri"))
	if strings.Contains(strings.ToLower(errString(err)), "未知连接器") {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleConnectorOAuthCallback(writer http.ResponseWriter, request *http.Request) {
	var payload connectorsvc.OAuthCallbackRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.connectors.CompleteOAuthCallback(request.Context(), payload)
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleConnectConnector(writer http.ResponseWriter, request *http.Request) {
	var payload map[string]string
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.connectors.Connect(request.Context(), chi.URLParam(request, "connector_id"), payload)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "未知连接器") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDisconnectConnector(writer http.ResponseWriter, request *http.Request) {
	item, err := s.connectors.Disconnect(request.Context(), chi.URLParam(request, "connector_id"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "未知连接器") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleWebSocket(writer http.ResponseWriter, request *http.Request) {
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	sender := newWebSocketSender(connection)
	defer func() {
		sender.MarkClosed()
		if s.workspaceSubs != nil {
			s.workspaceSubs.UnregisterSender(sender)
		}
		if s.roomSubs != nil {
			s.roomSubs.UnregisterSender(sender)
		}
		_ = connection.Close(websocket.StatusNormalClosure, "closed")
		s.broadcastSessionStatus(request.Context(), s.permission.UnregisterSender(sender)...)
	}()

	ctx := request.Context()
	for {
		var inbound map[string]any
		if err := wsjson.Read(ctx, connection, &inbound); err != nil {
			return
		}
		s.dispatchWebSocketMessage(ctx, sender, inbound)
	}
}

func (s *Server) dispatchWebSocketMessage(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	msgType := stringValue(inbound["type"])
	switch msgType {
	case "ping":
		_ = sender.SendEvent(ctx, protocol.NewPongEvent(stringValue(inbound["session_key"])))
		return
	case "subscribe_workspace":
		s.handleSubscribeWorkspace(ctx, sender, inbound)
		return
	case "unsubscribe_workspace":
		s.handleUnsubscribeWorkspace(sender, inbound)
		return
	case "subscribe_room":
		s.handleSubscribeRoom(ctx, sender, inbound)
		return
	case "unsubscribe_room":
		s.handleUnsubscribeRoom(sender, inbound)
		return
	case "bind_session":
		s.handleBindSession(ctx, sender, inbound)
		return
	case "unbind_session":
		s.handleUnbindSession(ctx, sender, inbound)
		return
	case "chat", "interrupt", "permission_response":
		s.handleControlMessage(ctx, sender, inbound)
		return
	default:
		_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
			stringValue(inbound["session_key"]),
			"unknown_message_type",
			"Go 网关已接管入口，但该消息类型尚未实现",
			map[string]any{"type": msgType},
		))
		return
	}
}

func (s *Server) handleSubscribeRoom(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	roomID := stringValue(inbound["room_id"])
	conversationID := stringValue(inbound["conversation_id"])
	if err := s.validateRoomSubscription(ctx, roomID, conversationID); err != nil {
		_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
			"",
			"invalid_room_subscription",
			err.Error(),
			map[string]any{
				"type":            stringValue(inbound["type"]),
				"room_id":         roomID,
				"conversation_id": conversationID,
			},
		))
		return
	}
	var latestRoomSeq int64
	if s.roomSubs != nil {
		latestRoomSeq = s.roomSubs.CurrentRoomSeq(roomID)
	}
	hasPending := s.restoreRoomPendingSlots(ctx, sender, roomID, conversationID)
	if s.roomSubs != nil {
		lastSeenRoomSeq := int64Value(inbound["last_seen_room_seq"])
		var lastSeenPtr *int64
		if lastSeenRoomSeq > 0 {
			lastSeenPtr = &lastSeenRoomSeq
		} else if hasPending && latestRoomSeq > 0 {
			lastSeenPtr = &latestRoomSeq
		}
		if err := s.roomSubs.SubscribeRoom(ctx, sender, roomID, conversationID, lastSeenPtr); err != nil {
			_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
				"",
				"room_subscription_error",
				err.Error(),
				map[string]any{
					"type":            stringValue(inbound["type"]),
					"room_id":         roomID,
					"conversation_id": conversationID,
				},
			))
			return
		}
	}
}

func (s *Server) handleUnsubscribeRoom(sender *websocketSender, inbound map[string]any) {
	if s.roomSubs == nil {
		return
	}
	s.roomSubs.UnsubscribeRoom(sender, stringValue(inbound["room_id"]))
}

func (s *Server) handleBindSession(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	sessionKey, parsed, ok := s.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if parsed.Kind == protocol.SessionKeyKindUnknown {
		return
	}
	requestControl, requestControlExists := boolValue(inbound["request_control"])
	if !requestControlExists {
		requestControl = true
	}
	s.permission.BindSession(
		sessionKey,
		sender,
		stringValue(inbound["client_id"]),
		requestControl,
	)
	if s.channels != nil {
		_ = s.channels.RememberWebSocketRoute(ctx, sessionKey)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
}

func (s *Server) validateRoomSubscription(ctx context.Context, roomID string, conversationID string) error {
	if strings.TrimSpace(roomID) == "" {
		return errors.New("room_id is required")
	}
	if strings.TrimSpace(conversationID) == "" {
		_, err := s.roomService.GetRoom(ctx, roomID)
		return err
	}

	contextValue, err := s.roomService.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	if contextValue.Room.ID != roomID {
		return errors.New("conversation_id does not belong to room_id")
	}
	return nil
}

func (s *Server) restoreRoomPendingSlots(ctx context.Context, sender *websocketSender, roomID string, conversationID string) bool {
	if s.roomRealtime == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}

	snapshot := s.roomRealtime.GetActiveRoundSnapshot(conversationID)
	if snapshot == nil || len(snapshot.Pending) == 0 {
		return false
	}

	event := protocol.NewChatAckEvent(snapshot.SessionKey, snapshot.RoundID, snapshot.RoundID, snapshot.Pending)
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.CausedBy = snapshot.RoundID
	_ = sender.SendEvent(ctx, event)
	return true
}

func (s *Server) handleUnbindSession(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	sessionKey, _, ok := s.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	s.permission.UnbindSession(sessionKey, sender)
	s.broadcastSessionStatus(ctx, sessionKey)
}

func (s *Server) handleControlMessage(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	sessionKey, parsed, ok := s.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if s.ensureSessionBinding(ctx, sender, inbound, sessionKey) {
		return
	}
	if s.rejectControlMessageFromObserver(ctx, sender, inbound, sessionKey) {
		return
	}

	msgType := stringValue(inbound["type"])
	switch msgType {
	case "chat":
		var err error
		if parsed.Kind == protocol.SessionKeyKindRoom && s.roomRealtime != nil {
			err = s.roomRealtime.HandleChat(ctx, room2.ChatRequest{
				SessionKey:     sessionKey,
				RoomID:         stringValue(inbound["room_id"]),
				ConversationID: stringValue(inbound["conversation_id"]),
				Content:        stringValue(inbound["content"]),
				RoundID:        stringValue(inbound["round_id"]),
				ReqID:          stringValue(inbound["req_id"]),
			})
		} else {
			err = s.chat.HandleChat(ctx, chatsvc.Request{
				SessionKey: sessionKey,
				AgentID:    stringValue(inbound["agent_id"]),
				Content:    stringValue(inbound["content"]),
				RoundID:    stringValue(inbound["round_id"]),
				ReqID:      stringValue(inbound["req_id"]),
			})
		}
		if err != nil {
			errorType := "chat_error"
			if errors.Is(err, chatsvc.ErrRoomChatNotImplemented) {
				errorType = "not_implemented"
			}
			_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
				sessionKey,
				errorType,
				err.Error(),
				map[string]any{"type": msgType},
			))
		}
	case "interrupt":
		var err error
		if parsed.Kind == protocol.SessionKeyKindRoom && s.roomRealtime != nil {
			err = s.roomRealtime.HandleInterrupt(ctx, room2.InterruptRequest{
				SessionKey: sessionKey,
				MsgID:      stringValue(inbound["msg_id"]),
			})
		} else {
			err = s.chat.HandleInterrupt(ctx, chatsvc.InterruptRequest{
				SessionKey: sessionKey,
				RoundID:    stringValue(inbound["round_id"]),
			})
		}
		if err != nil {
			_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
				sessionKey,
				"interrupt_error",
				err.Error(),
				map[string]any{"type": msgType},
			))
		}
	case "permission_response":
		if !s.permission.HandlePermissionResponse(inbound) {
			_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
				sessionKey,
				"permission_request_not_found",
				"未找到待确认的权限请求",
				map[string]any{"type": msgType},
			))
		}
	default:
		_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
			sessionKey,
			"not_implemented",
			"Go 运行时已接管控制面，但该写操作尚未实现",
			map[string]any{"type": msgType},
		))
	}
}

func (s *Server) ensureSessionBinding(ctx context.Context, sender *websocketSender, inbound map[string]any, sessionKey string) bool {
	if s.permission.IsBound(sessionKey, sender) {
		return false
	}
	if s.permission.HasBindings(sessionKey) {
		return false
	}
	s.permission.BindSession(
		sessionKey,
		sender,
		stringValue(inbound["client_id"]),
		true,
	)
	s.broadcastSessionStatus(ctx, sessionKey)
	return false
}

func (s *Server) rejectControlMessageFromObserver(ctx context.Context, sender *websocketSender, inbound map[string]any, sessionKey string) bool {
	if s.permission.IsSessionController(sessionKey, sender) {
		return false
	}
	actionLabel := map[string]string{
		"chat":                "发送消息",
		"interrupt":           "停止生成",
		"permission_response": "确认权限",
	}[stringValue(inbound["type"])]
	if actionLabel == "" {
		actionLabel = "执行操作"
	}
	_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
		sessionKey,
		"session_control_denied",
		"当前窗口不是该会话的控制端，无法"+actionLabel,
		map[string]any{"type": stringValue(inbound["type"])},
	))
	return true
}

func (s *Server) validateSessionKey(ctx context.Context, sender *websocketSender, inbound map[string]any) (string, protocol.SessionKey, bool) {
	sessionKey := stringValue(inbound["session_key"])
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		errorType := "invalid_session_key"
		if err.Error() == "session_key is required" {
			errorType = "validation_error"
		}
		_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
			sessionKey,
			errorType,
			err.Error(),
			map[string]any{"type": stringValue(inbound["type"])},
		))
		return "", protocol.SessionKey{}, false
	}
	return normalized, protocol.ParseSessionKey(normalized), true
}

func (s *Server) newGatewayErrorEvent(sessionKey string, errorType string, message string, details map[string]any) protocol.EventMessage {
	data := map[string]any{
		"message":    message,
		"error_type": errorType,
	}
	for key, value := range details {
		data[key] = value
	}
	event := protocol.NewEvent(protocol.EventTypeError, data)
	event.SessionKey = sessionKey
	return event
}

func (s *Server) broadcastRoomEvent(
	ctx context.Context,
	roomID string,
	eventType protocol.EventType,
	data map[string]any,
) {
	if s.roomSubs == nil || strings.TrimSpace(roomID) == "" {
		return
	}
	event := protocol.NewEvent(eventType, data)
	event.RoomID = strings.TrimSpace(roomID)
	s.roomSubs.Broadcast(ctx, event.RoomID, event)
}

func (s *Server) broadcastRoomResyncRequired(
	ctx context.Context,
	roomID string,
	conversationID string,
	reason string,
) {
	if s.roomSubs == nil || strings.TrimSpace(roomID) == "" {
		return
	}
	data := map[string]any{
		"room_id":         strings.TrimSpace(roomID),
		"conversation_id": strings.TrimSpace(conversationID),
		"reason":          strings.TrimSpace(reason),
	}
	event := protocol.NewEvent(protocol.EventTypeRoomResyncRequired, data)
	event.RoomID = data["room_id"].(string)
	s.roomSubs.Broadcast(ctx, event.RoomID, event)
}

func (s *Server) broadcastSessionStatus(ctx context.Context, sessionKeys ...string) {
	for _, sessionKey := range sessionKeys {
		if strings.TrimSpace(sessionKey) == "" {
			continue
		}
		_ = s.permission.BroadcastSessionStatus(ctx, sessionKey, s.runtime.GetRunningRoundIDs(sessionKey))
	}
}

func (s *Server) handleNotImplemented(group string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		s.writeJSON(writer, http.StatusNotImplemented, map[string]any{
			"code": 1,
			"msg":  "not_implemented",
			"data": map[string]any{
				"group": group,
				"path":  request.URL.Path,
			},
		})
	}
}

func (s *Server) writeJSON(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func (s *Server) writeSuccess(writer http.ResponseWriter, data any) {
	s.writeJSON(writer, http.StatusOK, map[string]any{
		"code":    "0000",
		"message": "success",
		"success": true,
		"data":    data,
	})
}

func (s *Server) writeFailure(writer http.ResponseWriter, status int, detail string) {
	s.writeJSON(writer, status, map[string]any{
		"code":    fmtStatusCode(status),
		"message": "failed",
		"success": false,
		"data": map[string]any{
			"detail": detail,
		},
	})
}

func fmtStatusCode(status int) string {
	return strings.TrimSpace(strconv.Itoa(status))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func isClientMessageError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "不能为空") ||
		strings.Contains(message, "不一致") ||
		strings.Contains(message, "已存在") ||
		strings.Contains(message, "至少") ||
		strings.Contains(message, "不支持") ||
		strings.Contains(message, "不能作为") ||
		strings.Contains(message, " is required") ||
		strings.Contains(message, " must be ") ||
		strings.Contains(message, "正在运行中")
}

func isStructuredSessionKeyError(err error) bool {
	if err == nil {
		return false
	}
	var target protocol.StructuredSessionKeyError
	return errors.As(err, &target)
}

func stringValue(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func boolValue(value any) (bool, bool) {
	typed, ok := value.(bool)
	if ok {
		return typed, true
	}
	return false, false
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
