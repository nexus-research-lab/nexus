package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	agent3 "github.com/nexus-research-lab/nexus/internal/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/conversation/titlegen"
	"github.com/nexus-research-lab/nexus/internal/logx"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	permission3 "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

var (
	// ErrRoomChatNotImplemented 表示 Room 实时编排尚未迁入 Go。
	ErrRoomChatNotImplemented = errors.New("room chat is not implemented yet")
)

// Request 表示一次 DM 会话写入请求。
type Request struct {
	SessionKey        string
	AgentID           string
	Content           string
	RoundID           string
	ReqID             string
	PermissionMode    sdkprotocol.PermissionMode
	PermissionHandler agentclient.PermissionHandler
}

// InterruptRequest 表示一次中断请求。
type InterruptRequest struct {
	SessionKey string
	RoundID    string
}

// MCPServerBuilder 由 bootstrap 注入，按当前会话上下文构造一组进程内 MCP server。
// 用 string 形参避免 chat 包反向依赖 automation 子包，防止 import cycle。
type MCPServerBuilder func(agentID, sessionKey, sourceContextType string) map[string]agentclient.SDKMCPServer

// Service 负责编排 DM 实时链路。
type Service struct {
	config     config.Config
	agents     *agent3.Service
	runtime    *runtimectx.Manager
	permission *permission3.Context
	roomStore  roomSessionStore
	providers  runtimectx.RuntimeConfigResolver
	files      *workspacestore.SessionFileStore
	history    *workspacestore.AgentHistoryStore
	logger     *slog.Logger
	mcpServers MCPServerBuilder
	titles     titleScheduler
}

type roomSessionStore interface {
	GetRoomSessionByKey(context.Context, string, protocol.SessionKey) (*session.Session, error)
	UpdateRoomSessionSDKSessionID(context.Context, string, string) error
}

type titleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

type chatRoundMapperAdapter struct {
	mapper *messageMapper
}

func (a chatRoundMapperAdapter) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) (runtimectx.RoundMapResult, error) {
	events, durableMessages, terminalStatus, resultSubtype, err := a.mapper.Map(incoming, interruptReason...)
	if err != nil {
		return runtimectx.RoundMapResult{}, err
	}
	return runtimectx.RoundMapResult{
		Events:          events,
		DurableMessages: durableMessages,
		TerminalStatus:  terminalStatus,
		ResultSubtype:   resultSubtype,
	}, nil
}

func (a chatRoundMapperAdapter) SessionID() string {
	return a.mapper.SessionID()
}

type roundRunner struct {
	service           *Service
	workspacePath     string
	session           session.Session
	agent             *agent3.Agent
	sessionKey        string
	roundID           string
	reqID             string
	content           string
	client            runtimectx.Client
	runtimeProvider   string
	runtimeModel      string
	mapper            *messageMapper
	permissionMode    sdkprotocol.PermissionMode
	permissionHandler agentclient.PermissionHandler
}

// NewService 创建 DM 会话编排服务。
func NewService(
	cfg config.Config,
	agentService *agent3.Service,
	runtimeManager *runtimectx.Manager,
	permission *permission3.Context,
) *Service {
	return &Service{
		config:     cfg,
		agents:     agentService,
		runtime:    runtimeManager,
		permission: permission,
		files:      workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:    workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		logger:     logx.NewDiscardLogger(),
	}
}

// SetLogger 注入业务日志实例。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetMCPServerBuilder 注入按会话上下文构造进程内 MCP server 的工厂。
// 由 bootstrap 在构造定时任务服务后注入，避免 chat 包反向依赖 automation 子包。
func (s *Service) SetMCPServerBuilder(builder MCPServerBuilder) {
	s.mcpServers = builder
}

// SetProviderResolver 注入 Provider 运行时解析器。
func (s *Service) SetProviderResolver(resolver runtimectx.RuntimeConfigResolver) {
	s.providers = resolver
}

// SetRoomSessionStore 注入 room 成员会话索引读写能力。
func (s *Service) SetRoomSessionStore(store roomSessionStore) {
	s.roomStore = store
}

// SetTitleGenerator 注入会话标题生成器。
func (s *Service) SetTitleGenerator(generator titleScheduler) {
	s.titles = generator
}

// HandleChat 处理一条 DM chat 写请求。
func (s *Service) HandleChat(ctx context.Context, request Request) error {
	sessionKey, parsed, err := s.validateRequest(request)
	if err != nil {
		return err
	}
	agentID := firstNonEmpty(parsed.AgentID, request.AgentID)
	if agentID == "" {
		defaultAgent, defaultErr := s.agents.GetDefaultAgent(ctx)
		if defaultErr != nil {
			return defaultErr
		}
		agentID = defaultAgent.AgentID
	}

	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}

	sessionItem, err := s.ensureSession(ctx, agentValue, parsed, sessionKey)
	if err != nil {
		return err
	}
	initialMessageCount := sessionItem.MessageCount

	if err = s.interruptSession(ctx, sessionKey, "收到新的用户消息，上一轮已停止"); err != nil {
		return err
	}

	client, runtimeProvider, runtimeModel, err := s.ensureClient(ctx, sessionKey, agentValue, sessionItem, request)
	if err != nil {
		s.loggerFor(ctx).Error("DM runtime client 初始化失败",
			"session_key", sessionKey,
			"agent_id", agentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return err
	}
	if updatedSession, syncErr := s.syncSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem, client.SessionID(), runtimeProvider, runtimeModel); syncErr != nil {
		return syncErr
	} else {
		sessionItem = updatedSession
	}

	roundCtx, cancel := context.WithCancel(context.Background())
	s.runtime.StartRound(sessionKey, request.RoundID, cancel)
	s.permission.BindSessionRoute(sessionKey, permission3.RouteContext{
		DispatchSessionKey: sessionKey,
		AgentID:            agentID,
		CausedBy:           request.RoundID,
	})

	runner := &roundRunner{
		service:           s,
		workspacePath:     agentValue.WorkspacePath,
		session:           sessionItem,
		agent:             agentValue,
		sessionKey:        sessionKey,
		roundID:           request.RoundID,
		reqID:             firstNonEmpty(request.ReqID, request.RoundID),
		content:           strings.TrimSpace(request.Content),
		client:            client,
		runtimeProvider:   runtimeProvider,
		runtimeModel:      runtimeModel,
		mapper:            newMessageMapper(sessionKey, agentID, request.RoundID),
		permissionMode:    request.PermissionMode,
		permissionHandler: request.PermissionHandler,
	}

	s.loggerFor(ctx).Info("受理 DM 会话消息",
		"session_key", sessionKey,
		"agent_id", agentID,
		"round_id", request.RoundID,
		"req_id", runner.reqID,
		"content_chars", utf8.RuneCountInString(runner.content),
	)

	if err = s.recordRoundMarker(runner.workspacePath, runner.session, runner.roundID, runner.content); err != nil {
		s.runtime.MarkRoundFinished(sessionKey, request.RoundID)
		s.permission.CancelRequestsForSession(sessionKey, "轮次标记持久化失败")
		s.loggerFor(ctx).Error("DM 轮次标记持久化失败",
			"session_key", sessionKey,
			"agent_id", agentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return err
	}

	if updatedSession, syncErr := s.refreshSessionMetaAfterRoundMarker(runner.workspacePath, runner.session); syncErr != nil {
		s.runtime.MarkRoundFinished(sessionKey, request.RoundID)
		s.permission.CancelRequestsForSession(sessionKey, "会话元数据持久化失败")
		s.loggerFor(ctx).Error("DM 轮次元数据持久化失败",
			"session_key", sessionKey,
			"agent_id", agentID,
			"round_id", request.RoundID,
			"err", syncErr,
		)
		return syncErr
	} else if updatedSession != nil {
		runner.session = *updatedSession
	}

	s.scheduleTitleGeneration(ctx, parsed, runner.session, runner.content, initialMessageCount)

	s.permission.BroadcastEvent(ctx, sessionKey, protocol.NewChatAckEvent(sessionKey, runner.reqID, request.RoundID, []map[string]any{}))
	s.permission.BroadcastEvent(ctx, sessionKey, protocol.NewRoundStatusEvent(sessionKey, request.RoundID, "running", ""))
	s.broadcastSessionStatus(ctx, sessionKey)

	go runner.run(roundCtx)
	return nil
}

func (s *Service) scheduleTitleGeneration(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionItem session.Session,
	content string,
	initialMessageCount int,
) {
	if s.titles == nil {
		return
	}
	conversationID := strings.TrimSpace(stringPointerValue(sessionItem.ConversationID))
	if conversationID == "" && parsed.ChatType == "dm" {
		conversationID = strings.TrimSpace(parsed.Ref)
	}
	roomID := strings.TrimSpace(stringPointerValue(sessionItem.RoomID))
	conversationMessageCount := 0
	if conversationID == "" {
		conversationMessageCount = -1
	}
	s.titles.Schedule(ctx, titlegen.Request{
		SessionKey:               sessionItem.SessionKey,
		Provider:                 "",
		Content:                  content,
		SessionTitle:             sessionItem.Title,
		SessionMessageCount:      initialMessageCount,
		ConversationID:           conversationID,
		ConversationRoomID:       roomID,
		ConversationMessageCount: conversationMessageCount,
	})
}

// HandleInterrupt 处理中断请求。
func (s *Service) HandleInterrupt(ctx context.Context, request InterruptRequest) error {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return err
	}
	return s.interruptSession(ctx, sessionKey, "")
}

func (s *Service) interruptSession(ctx context.Context, sessionKey string, resultText string) error {
	roundIDs, err := s.runtime.InterruptSession(ctx, sessionKey, resultText)
	if err != nil {
		return err
	}
	if len(roundIDs) == 0 {
		return nil
	}
	s.loggerFor(ctx).Warn("中断 DM 会话运行轮次",
		"session_key", sessionKey,
		"round_count", len(roundIDs),
		"reason", resultText,
	)
	s.permission.CancelRequestsForSession(sessionKey, resultText)
	s.broadcastSessionStatus(ctx, sessionKey)
	return nil
}

func (s *Service) ensureClient(
	ctx context.Context,
	sessionKey string,
	agentValue *agent3.Agent,
	sessionItem session.Session,
	request Request,
) (runtimectx.Client, string, string, error) {
	permissionMode := request.PermissionMode
	if permissionMode == "" {
		permissionMode = sdkprotocol.PermissionMode(agentValue.Options.PermissionMode)
	}
	if permissionMode == "" {
		permissionMode = sdkprotocol.PermissionModeDefault
	}
	permissionHandler := request.PermissionHandler
	if permissionHandler == nil {
		permissionHandler = func(permissionCtx context.Context, permissionRequest sdkprotocol.PermissionRequest) (sdkprotocol.PermissionDecision, error) {
			return s.permission.RequestPermission(permissionCtx, sessionKey, permissionRequest)
		}
	}
	appendSystemPrompt, err := s.agents.BuildRuntimePrompt(ctx, agentValue)
	if err != nil {
		return nil, "", "", err
	}
	mcpServers := map[string]agentclient.SDKMCPServer(nil)
	if s.mcpServers != nil {
		mcpServers = s.mcpServers(agentValue.AgentID, sessionKey, "agent")
	}
	options, err := runtimectx.BuildAgentClientOptions(ctx, s.providers, runtimectx.AgentClientOptionsInput{
		WorkspacePath:      agentValue.WorkspacePath,
		Provider:           agentValue.Options.Provider,
		PermissionMode:     permissionMode,
		PermissionHandler:  permissionHandler,
		AllowedTools:       agentValue.Options.AllowedTools,
		DisallowedTools:    agentValue.Options.DisallowedTools,
		SettingSources:     agentValue.Options.SettingSources,
		AppendSystemPrompt: appendSystemPrompt,
		ResumeSessionID:    stringPointerValue(sessionItem.SessionID),
		MaxThinkingTokens:  agentValue.Options.MaxThinkingTokens,
		MaxTurns:           agentValue.Options.MaxTurns,
		MCPServers:         mcpServers,
	})
	if err != nil {
		return nil, "", "", err
	}
	options.Resume = s.resolveReusableSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem, agentValue.Options.Provider, options)
	client, err := s.acquireRuntimeClient(ctx, sessionKey, options)
	if err != nil {
		return nil, "", "", err
	}
	return client, strings.TrimSpace(agentValue.Options.Provider), strings.TrimSpace(options.Model), nil
}

func (s *Service) resolveReusableSDKSessionID(
	ctx context.Context,
	workspacePath string,
	sessionItem session.Session,
	provider string,
	options agentclient.Options,
) string {
	resumeID := strings.TrimSpace(options.Resume)
	if resumeID == "" {
		return ""
	}
	expectedProvider := strings.TrimSpace(provider)
	expectedModel := strings.TrimSpace(options.Model)
	actualProvider, _ := sessionItem.Options[sessionmodel.OptionRuntimeProvider].(string)
	actualModel, _ := sessionItem.Options[sessionmodel.OptionRuntimeModel].(string)
	if strings.TrimSpace(actualProvider) == expectedProvider && strings.TrimSpace(actualModel) == expectedModel {
		return resumeID
	}
	s.loggerFor(ctx).Warn("DM session runtime 配置已变更，跳过旧 SDK session resume",
		"session_key", sessionItem.SessionKey,
		"old_provider", strings.TrimSpace(actualProvider),
		"new_provider", expectedProvider,
		"old_model", strings.TrimSpace(actualModel),
		"new_model", expectedModel,
	)
	sessionItem.SessionID = nil
	if sessionItem.Options == nil {
		sessionItem.Options = map[string]any{}
	}
	sessionItem.Options[sessionmodel.OptionRuntimeProvider] = expectedProvider
	sessionItem.Options[sessionmodel.OptionRuntimeModel] = expectedModel
	if _, err := s.files.UpsertSession(workspacePath, sessionItem); err != nil {
		s.loggerFor(ctx).Error("DM session runtime 配置指纹更新失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
	}
	return ""
}

func (s *Service) acquireRuntimeClient(
	ctx context.Context,
	sessionKey string,
	options agentclient.Options,
) (runtimectx.Client, error) {
	client, err := s.runtime.GetOrCreate(ctx, sessionKey, options)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

func (s *Service) ensureSession(
	ctx context.Context,
	agentValue *agent3.Agent,
	parsed protocol.SessionKey,
	sessionKey string,
) (session.Session, error) {
	item, _, err := s.files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil {
		return session.Session{}, err
	}
	roomSession, err := s.lookupRoomSession(ctx, parsed)
	if err != nil {
		return session.Session{}, err
	}

	if item != nil {
		if roomSession != nil {
			merged := mergeRoomBackedSession(*item, *roomSession)
			if !sessionItemsEqual(*item, merged) {
				updated, updateErr := s.files.UpsertSession(agentValue.WorkspacePath, merged)
				if updateErr != nil {
					return session.Session{}, updateErr
				}
				if updated != nil {
					item = updated
				} else {
					item = &merged
				}
			}
		}
		if err := sessionmodel.EnsureTranscriptHistory(item.Options, sessionKey); err != nil {
			return session.Session{}, err
		}
		return *item, nil
	}

	if roomSession != nil {
		updated, updateErr := s.files.UpsertSession(agentValue.WorkspacePath, *roomSession)
		if updateErr != nil {
			return session.Session{}, updateErr
		}
		if updated == nil {
			return session.Session{}, fmt.Errorf("创建 room 成员会话失败: %s", sessionKey)
		}
		if err := sessionmodel.EnsureTranscriptHistory(updated.Options, sessionKey); err != nil {
			return session.Session{}, err
		}
		return *updated, nil
	}

	now := time.Now().UTC()
	created, err := s.files.UpsertSession(agentValue.WorkspacePath, session.Session{
		SessionKey:   sessionKey,
		AgentID:      agentValue.AgentID,
		ChannelType:  protocol.NormalizeStoredChannelType(parsed.Channel),
		ChatType:     protocol.NormalizeSessionChatType(parsed.ChatType),
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "New Chat",
		Options: map[string]any{
			sessionmodel.OptionHistorySource: sessionmodel.HistorySourceTranscript,
		},
		IsActive: true,
	})
	if err != nil {
		return session.Session{}, err
	}
	if created == nil {
		return session.Session{}, fmt.Errorf("创建 session 失败: %s", sessionKey)
	}
	return *created, nil
}

func (s *Service) lookupRoomSession(
	ctx context.Context,
	parsed protocol.SessionKey,
) (*session.Session, error) {
	if s.roomStore == nil {
		return nil, nil
	}
	return s.roomStore.GetRoomSessionByKey(ctx, ownerUserIDFromContext(ctx), parsed)
}

func ownerUserIDFromContext(ctx context.Context) string {
	if userID, ok := authsvc.CurrentUserID(ctx); ok {
		return userID
	}
	return authsvc.SystemUserID
}

func (s *Service) validateRequest(request Request) (string, protocol.SessionKey, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", protocol.SessionKey{}, err
	}
	if strings.TrimSpace(request.Content) == "" {
		return "", protocol.SessionKey{}, errors.New("content is required")
	}
	if strings.TrimSpace(request.RoundID) == "" {
		return "", protocol.SessionKey{}, errors.New("round_id is required")
	}

	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return "", protocol.SessionKey{}, ErrRoomChatNotImplemented
	}
	return sessionKey, parsed, nil
}

func (s *Service) broadcastSessionStatus(ctx context.Context, sessionKey string) {
	if errs := s.permission.BroadcastSessionStatus(ctx, sessionKey, s.runtime.GetRunningRoundIDs(sessionKey)); len(errs) > 0 {
		s.loggerFor(ctx).Warn("广播 session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

func (r *roundRunner) run(ctx context.Context) {
	logger := r.service.loggerFor(ctx).With(
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
	)
	logger.Info("开始执行 DM round")
	result, err := r.executeRound(ctx, logger)
	if err != nil {
		if errors.Is(err, runtimectx.ErrRoundInterrupted) {
			r.finishInterrupted(r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID))
			return
		}
		r.failRound(err)
		return
	}

	r.service.loggerFor(context.Background()).Info("DM round 结束",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"status", result.TerminalStatus,
		"result_subtype", result.ResultSubtype,
	)
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	r.service.permission.BroadcastEvent(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, result.TerminalStatus, result.ResultSubtype),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
}

func (r *roundRunner) executeRound(
	ctx context.Context,
	logger *slog.Logger,
) (runtimectx.RoundExecutionResult, error) {
	return runtimectx.ExecuteRound(ctx, runtimectx.RoundExecutionRequest{
		Query:  r.content,
		Client: r.client,
		Mapper: chatRoundMapperAdapter{mapper: r.mapper},
		InterruptReason: func() string {
			return r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID)
		},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			logger.Debug("Agent ", runtimectx.BuildSDKMessageLogFields(incoming)...)
		},
		SyncSessionID: func(sessionID string) error {
			updatedSession, syncErr := r.service.syncSDKSessionID(
				context.Background(),
				r.workspacePath,
				r.session,
				sessionID,
				r.runtimeProvider,
				r.runtimeModel,
			)
			if syncErr != nil {
				return syncErr
			}
			r.session = updatedSession
			return nil
		},
		HandleDurableMessage: func(message sessionmodel.Message) error {
			if err := r.persistMessage(message); err != nil {
				return err
			}
			if message["role"] == "assistant" {
				r.service.permission.BindSessionRoute(r.sessionKey, permission3.RouteContext{
					DispatchSessionKey: r.sessionKey,
					AgentID:            r.agent.AgentID,
					MessageID:          normalizeString(message["message_id"]),
					CausedBy:           r.roundID,
				})
			}
			return nil
		},
		EmitEvent: func(event protocol.EventMessage) error {
			r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, event)
			return nil
		},
	})
}

func (r *roundRunner) failRound(err error) {
	if interruptReason := r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID); interruptReason != "" {
		r.finishInterrupted(interruptReason)
		return
	}
	r.service.loggerFor(context.Background()).Error("DM round 执行失败",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"err", err,
	)
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	persistedSessionID := ""
	if r.session.SessionID != nil {
		persistedSessionID = strings.TrimSpace(*r.session.SessionID)
	}
	resultMessage := sessionmodel.Message{
		"message_id":      "result_" + r.roundID,
		"session_key":     r.sessionKey,
		"agent_id":        r.agent.AgentID,
		"round_id":        r.roundID,
		"session_id":      firstNonEmpty(r.client.SessionID(), persistedSessionID),
		"role":            "result",
		"timestamp":       time.Now().UnixMilli(),
		"subtype":         "error",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"usage":           map[string]any{},
		"result":          err.Error(),
		"is_error":        true,
	}
	if persistErr := r.service.appendSyntheticHistoryMessage(r.workspacePath, r.session, resultMessage); persistErr != nil {
		r.service.loggerFor(context.Background()).Error("DM 错误结果持久化失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", persistErr,
		)
	} else {
		if updated, updateErr := r.service.refreshSessionMetaAfterMessage(r.workspacePath, r.session, resultMessage); updateErr != nil {
			r.service.loggerFor(context.Background()).Error("DM 错误结果刷新 session meta 失败",
				"session_key", r.sessionKey,
				"agent_id", r.agent.AgentID,
				"round_id", r.roundID,
				"err", updateErr,
			)
		} else if updated != nil {
			r.session = *updated
		}
		event := protocol.NewEvent(protocol.EventTypeMessage, r.mapper.ProjectResultMessage(resultMessage))
		event.SessionKey = r.sessionKey
		event.AgentID = r.agent.AgentID
		event.MessageID = normalizeString(event.Data["message_id"])
		event.DeliveryMode = "durable"
		r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, event)
	}
	errorEvent := protocol.NewErrorEvent(r.sessionKey, err.Error())
	errorEvent.AgentID = r.agent.AgentID
	errorEvent.CausedBy = r.roundID
	if messageID := strings.TrimSpace(r.mapper.CurrentMessageID()); messageID != "" {
		errorEvent.MessageID = messageID
	}
	r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, errorEvent)
	r.service.permission.BroadcastEvent(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, "error", "error"),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
}

func (r *roundRunner) finishInterrupted(resultText string) {
	r.service.loggerFor(context.Background()).Warn("DM round 以中断状态结束",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"reason", resultText,
	)
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	persistedSessionID := ""
	if r.session.SessionID != nil {
		persistedSessionID = strings.TrimSpace(*r.session.SessionID)
	}
	resultMessage := sessionmodel.Message{
		"message_id":      "result_" + r.roundID,
		"session_key":     r.sessionKey,
		"agent_id":        r.agent.AgentID,
		"round_id":        r.roundID,
		"session_id":      firstNonEmpty(r.client.SessionID(), persistedSessionID),
		"role":            "result",
		"timestamp":       time.Now().UnixMilli(),
		"subtype":         "interrupted",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"usage":           map[string]any{},
		"is_error":        false,
	}
	if trimmedResult := strings.TrimSpace(resultText); trimmedResult != "" {
		resultMessage["result"] = trimmedResult
	}
	if persistErr := r.service.appendSyntheticHistoryMessage(r.workspacePath, r.session, resultMessage); persistErr != nil {
		r.service.loggerFor(context.Background()).Error("DM interrupted 结果持久化失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", persistErr,
		)
	} else {
		if updated, updateErr := r.service.refreshSessionMetaAfterMessage(r.workspacePath, r.session, resultMessage); updateErr != nil {
			r.service.loggerFor(context.Background()).Error("DM interrupted 刷新 session meta 失败",
				"session_key", r.sessionKey,
				"agent_id", r.agent.AgentID,
				"round_id", r.roundID,
				"err", updateErr,
			)
		} else if updated != nil {
			r.session = *updated
		}
		event := protocol.NewEvent(protocol.EventTypeMessage, r.mapper.ProjectResultMessage(resultMessage))
		event.SessionKey = r.sessionKey
		event.AgentID = r.agent.AgentID
		event.MessageID = normalizeString(event.Data["message_id"])
		event.DeliveryMode = "durable"
		r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, event)
	}
	r.service.permission.BroadcastEvent(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, "interrupted", "interrupted"),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
}

func (r *roundRunner) persistMessage(message sessionmodel.Message) error {
	if err := r.service.appendRuntimeHistoryMessage(r.workspacePath, r.session, message); err != nil {
		return err
	}
	updated, err := r.service.refreshSessionMetaAfterMessage(r.workspacePath, r.session, message)
	if err != nil {
		return err
	}
	if updated != nil {
		r.session = *updated
	}
	return nil
}

func (s *Service) appendRuntimeHistoryMessage(
	workspacePath string,
	sessionValue session.Session,
	message sessionmodel.Message,
) error {
	if err := sessionmodel.EnsureTranscriptHistory(sessionValue.Options, sessionValue.SessionKey); err != nil {
		return err
	}
	if sessionmodel.IsTranscriptNativeMessage(sessionmodel.Message(message)) {
		return nil
	}
	return s.history.AppendOverlayMessage(workspacePath, sessionValue.SessionKey, message)
}

func (s *Service) appendSyntheticHistoryMessage(
	workspacePath string,
	sessionValue session.Session,
	message sessionmodel.Message,
) error {
	if err := sessionmodel.EnsureTranscriptHistory(sessionValue.Options, sessionValue.SessionKey); err != nil {
		return err
	}
	return s.history.AppendOverlayMessage(workspacePath, sessionValue.SessionKey, message)
}

func (s *Service) refreshSessionMetaAfterRoundMarker(
	workspacePath string,
	current session.Session,
) (*session.Session, error) {
	current.LastActivity = time.Now().UTC()
	current.MessageCount++
	if err := sessionmodel.EnsureTranscriptHistory(current.Options, current.SessionKey); err != nil {
		return nil, err
	}
	return s.files.UpsertSession(workspacePath, current)
}

func (s *Service) refreshSessionMetaAfterMessage(
	workspacePath string,
	current session.Session,
	message sessionmodel.Message,
) (*session.Session, error) {
	current.SessionID = preferSessionID(current.SessionID, normalizeString(message["session_id"]))
	current.Status = "active"
	current.LastActivity = time.Now().UTC()
	current.MessageCount++
	if err := sessionmodel.EnsureTranscriptHistory(current.Options, current.SessionKey); err != nil {
		return nil, err
	}
	return s.files.UpsertSession(workspacePath, current)
}

func (s *Service) recordRoundMarker(
	workspacePath string,
	sessionValue session.Session,
	roundID string,
	content string,
) error {
	if err := sessionmodel.EnsureTranscriptHistory(sessionValue.Options, sessionValue.SessionKey); err != nil {
		return err
	}
	return s.history.AppendRoundMarker(
		workspacePath,
		sessionValue.SessionKey,
		roundID,
		content,
		time.Now().UnixMilli(),
	)
}

func (s *Service) syncSDKSessionID(
	ctx context.Context,
	workspacePath string,
	current session.Session,
	sessionID string,
	runtimeProvider string,
	runtimeModel string,
) (session.Session, error) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	currentSessionID := strings.TrimSpace(stringPointerValue(current.SessionID))
	if trimmedSessionID == "" {
		return current, nil
	}
	nextProvider := strings.TrimSpace(runtimeProvider)
	nextModel := strings.TrimSpace(runtimeModel)
	currentProvider, _ := current.Options[sessionmodel.OptionRuntimeProvider].(string)
	currentModel, _ := current.Options[sessionmodel.OptionRuntimeModel].(string)
	sessionIDChanged := currentSessionID != trimmedSessionID
	fingerprintChanged := strings.TrimSpace(currentProvider) != nextProvider ||
		strings.TrimSpace(currentModel) != nextModel
	if !sessionIDChanged && !fingerprintChanged {
		return current, nil
	}
	current.SessionID = &trimmedSessionID
	if current.Options == nil {
		current.Options = map[string]any{}
	}
	current.Options[sessionmodel.OptionRuntimeProvider] = nextProvider
	current.Options[sessionmodel.OptionRuntimeModel] = nextModel
	updated, err := s.files.UpsertSession(workspacePath, current)
	if err != nil {
		return session.Session{}, err
	}
	if updated == nil {
		return current, nil
	}
	if sessionIDChanged && s.roomStore != nil && updated.RoomSessionID != nil && strings.TrimSpace(*updated.RoomSessionID) != "" {
		if err := s.roomStore.UpdateRoomSessionSDKSessionID(ctx, strings.TrimSpace(*updated.RoomSessionID), trimmedSessionID); err != nil {
			return session.Session{}, err
		}
	}
	return *updated, nil
}

func mergeRoomBackedSession(current session.Session, roomSession session.Session) session.Session {
	merged := roomSession
	if strings.TrimSpace(stringPointerValue(merged.SessionID)) == "" && current.SessionID != nil {
		merged.SessionID = current.SessionID
	}
	return merged
}

func sessionItemsEqual(left session.Session, right session.Session) bool {
	return left.SessionKey == right.SessionKey &&
		left.AgentID == right.AgentID &&
		stringPointerValue(left.SessionID) == stringPointerValue(right.SessionID) &&
		stringPointerValue(left.RoomSessionID) == stringPointerValue(right.RoomSessionID) &&
		stringPointerValue(left.RoomID) == stringPointerValue(right.RoomID) &&
		stringPointerValue(left.ConversationID) == stringPointerValue(right.ConversationID) &&
		left.ChannelType == right.ChannelType &&
		left.ChatType == right.ChatType &&
		left.Status == right.Status &&
		left.Title == right.Title
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func preferSessionID(current *string, next string) *string {
	if strings.TrimSpace(next) != "" {
		return &next
	}
	return current
}

func normalizeString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
