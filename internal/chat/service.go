// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：service.go
// @Date   ：2026/04/11 02:35:00
// @Author ：leemysw
// 2026/04/11 02:35:00   Create
// =====================================================

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
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/logx"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	permission3 "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/provider"
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

// Service 负责编排 DM 实时链路。
type Service struct {
	config     config.Config
	agents     *agent3.Service
	runtime    *runtimectx.Manager
	permission *permission3.Context
	roomStore  roomSessionStore
	providers  providerRuntimeResolver
	files      *workspacestore.SessionFileStore
	history    *workspacestore.AgentHistoryStore
	logger     *slog.Logger
}

type providerRuntimeResolver interface {
	ResolveRuntimeConfig(context.Context, string) (*providercfg.RuntimeConfig, error)
}

type roomSessionStore interface {
	GetRoomSessionByKey(context.Context, protocol.SessionKey) (*session.Session, error)
	UpdateRoomSessionSDKSessionID(context.Context, string, string) error
}

type roundRunner struct {
	service       *Service
	workspacePath string
	session       session.Session
	agent         *agent3.Agent
	sessionKey    string
	roundID       string
	reqID         string
	content       string
	client        runtimectx.Client
	mapper        *messageMapper
	terminalSeen  bool
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

// SetProviderResolver 注入 Provider 运行时解析器。
func (s *Service) SetProviderResolver(resolver providerRuntimeResolver) {
	s.providers = resolver
}

// SetRoomSessionStore 注入 room 成员会话索引读写能力。
func (s *Service) SetRoomSessionStore(store roomSessionStore) {
	s.roomStore = store
}

// HandleChat 处理一条 DM chat 写请求。
func (s *Service) HandleChat(ctx context.Context, request Request) error {
	sessionKey, parsed, err := s.validateRequest(request)
	if err != nil {
		return err
	}
	agentID := firstNonEmpty(parsed.AgentID, request.AgentID, s.config.DefaultAgentID)

	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}

	sessionItem, err := s.ensureSession(ctx, agentValue, parsed, sessionKey)
	if err != nil {
		return err
	}

	if err = s.interruptSession(ctx, sessionKey, "收到新的用户消息，上一轮已停止"); err != nil {
		return err
	}

	client, err := s.ensureClient(ctx, sessionKey, agentValue, sessionItem, request)
	if err != nil {
		return err
	}
	if updatedSession, syncErr := s.syncSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem, client.SessionID()); syncErr != nil {
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
		service:       s,
		workspacePath: agentValue.WorkspacePath,
		session:       sessionItem,
		agent:         agentValue,
		sessionKey:    sessionKey,
		roundID:       request.RoundID,
		reqID:         firstNonEmpty(request.ReqID, request.RoundID),
		content:       strings.TrimSpace(request.Content),
		client:        client,
		mapper:        newMessageMapper(sessionKey, agentID, request.RoundID),
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

	s.permission.BroadcastEvent(ctx, sessionKey, protocol.NewChatAckEvent(sessionKey, runner.reqID, request.RoundID, []map[string]any{}))
	s.permission.BroadcastEvent(ctx, sessionKey, protocol.NewRoundStatusEvent(sessionKey, request.RoundID, "running", ""))
	s.broadcastSessionStatus(ctx, sessionKey)

	go runner.run(roundCtx)
	return nil
}

// HandleInterrupt 处理中断请求。
func (s *Service) HandleInterrupt(ctx context.Context, request InterruptRequest) error {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return err
	}
	return s.interruptSession(ctx, sessionKey, "任务已中断")
}

func (s *Service) interruptSession(ctx context.Context, sessionKey string, resultText string) error {
	roundIDs, err := s.runtime.InterruptSession(ctx, sessionKey)
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
	for _, roundID := range roundIDs {
		s.emitInterruptedRound(ctx, sessionKey, roundID, resultText)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	return nil
}

func (s *Service) emitInterruptedRound(ctx context.Context, sessionKey string, roundID string, resultText string) {
	parsed := protocol.ParseSessionKey(sessionKey)
	resultMessage := session.Message{
		"message_id":      "result_" + roundID,
		"session_key":     sessionKey,
		"agent_id":        parsed.AgentID,
		"round_id":        roundID,
		"role":            "result",
		"timestamp":       time.Now().UnixMilli(),
		"subtype":         "interrupted",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       1,
		"result":          resultText,
		"is_error":        false,
	}

	// 对齐 Python 行为，先修复本轮未完成的 assistant 片段，再写入 result。
	if err := s.repairInterruptedRound(ctx, sessionKey, roundID, parsed.AgentID); err != nil {
		s.loggerFor(ctx).Warn("DM interrupted round 修复失败",
			"session_key", sessionKey,
			"round_id", roundID,
			"err", err,
		)
	}

	if err := s.persistInterruptedRound(ctx, sessionKey, parsed, resultMessage); err != nil {
		s.loggerFor(ctx).Error("DM interrupted 结果持久化失败",
			"session_key", sessionKey,
			"agent_id", parsed.AgentID,
			"round_id", roundID,
			"err", err,
		)
	}
	s.permission.BroadcastEvent(ctx, sessionKey, protocol.EventMessage{
		ProtocolVersion: 2,
		DeliveryMode:    "durable",
		EventType:       protocol.EventTypeMessage,
		SessionKey:      sessionKey,
		Data:            resultMessage,
		Timestamp:       time.Now().UnixMilli(),
	})
	s.permission.BroadcastEvent(ctx, sessionKey, protocol.NewRoundStatusEvent(sessionKey, roundID, "interrupted", "interrupted"))
}

func (s *Service) repairInterruptedRound(ctx context.Context, sessionKey string, roundID string, agentID string) error {
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}
	_, err = s.ensureSession(ctx, agentValue, protocol.ParseSessionKey(sessionKey), sessionKey)
	if err != nil {
		return err
	}
	// transcript 已经成为唯一正文真相源，这里不再扫描旧 messages.jsonl 修补半截 assistant。
	_ = roundID
	return nil
}

func (s *Service) persistInterruptedRound(
	ctx context.Context,
	sessionKey string,
	parsed protocol.SessionKey,
	resultMessage session.Message,
) error {
	agentValue, err := s.agents.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return err
	}
	sessionValue, err := s.ensureSession(ctx, agentValue, parsed, sessionKey)
	if err != nil {
		return err
	}
	if sessionValue.SessionID != nil && strings.TrimSpace(*sessionValue.SessionID) != "" {
		resultMessage["session_id"] = strings.TrimSpace(*sessionValue.SessionID)
	}
	if err := s.appendSyntheticHistoryMessage(agentValue.WorkspacePath, sessionValue, resultMessage); err != nil {
		return err
	}
	_, err = s.refreshSessionMetaAfterMessage(agentValue.WorkspacePath, sessionValue, resultMessage)
	return err
}

func (s *Service) ensureClient(
	ctx context.Context,
	sessionKey string,
	agentValue *agent3.Agent,
	sessionItem session.Session,
	request Request,
) (runtimectx.Client, error) {
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
	// Agent 级 runtime 已收口为 provider-only，
	// 这里不再透传旧的 Agent model，而是统一从 Provider 解析运行时环境。
	runtimeEnv, err := s.buildRuntimeEnv(ctx, agentValue)
	if err != nil {
		return nil, err
	}
	options := agentclient.Options{
		CWD:                    agentValue.WorkspacePath,
		PermissionMode:         permissionMode,
		AllowedTools:           append([]string(nil), agentValue.Options.AllowedTools...),
		DisallowedTools:        append([]string(nil), agentValue.Options.DisallowedTools...),
		SettingSources:         append([]string(nil), agentValue.Options.SettingSources...),
		IncludePartialMessages: true,
		Env:                    runtimeEnv,
		PermissionHandler:      permissionHandler,
	}
	if sessionItem.SessionID != nil && strings.TrimSpace(*sessionItem.SessionID) != "" {
		options.Resume = strings.TrimSpace(*sessionItem.SessionID)
	}
	if agentValue.Options.MaxThinkingTokens != nil && *agentValue.Options.MaxThinkingTokens > 0 {
		options.MaxThinkingTokens = *agentValue.Options.MaxThinkingTokens
	}
	if agentValue.Options.MaxTurns != nil && *agentValue.Options.MaxTurns > 0 {
		options.MaxTurns = *agentValue.Options.MaxTurns
	}
	client, err := s.runtime.GetOrCreate(ctx, sessionKey, options)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	if permissionMode != "" {
		if err := client.SetPermissionMode(ctx, permissionMode); err != nil && !errors.Is(err, agentclient.ErrNotConnected) {
			return nil, err
		}
	}
	return client, nil
}

func (s *Service) buildRuntimeEnv(ctx context.Context, agentValue *agent3.Agent) (map[string]string, error) {
	if s.providers == nil {
		return nil, nil
	}
	runtimeConfig, err := s.providers.ResolveRuntimeConfig(ctx, agentValue.Options.Provider)
	if err != nil {
		return nil, err
	}
	if runtimeConfig == nil {
		return nil, nil
	}
	env := map[string]string{
		"ANTHROPIC_AUTH_TOKEN":           runtimeConfig.AuthToken,
		"ANTHROPIC_BASE_URL":             runtimeConfig.BaseURL,
		"ANTHROPIC_MODEL":                runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  runtimeConfig.Model,
		"CLAUDE_CODE_SUBAGENT_MODEL":     runtimeConfig.Model,
	}
	if strings.Contains(strings.ToLower(runtimeConfig.Model), "kimi") {
		env["ENABLE_TOOL_SEARCH"] = "false"
	}
	return env, nil
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
	return s.roomStore.GetRoomSessionByKey(ctx, parsed)
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
	_ = s.permission.BroadcastSessionStatus(ctx, sessionKey, s.runtime.GetRunningRoundIDs(sessionKey))
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
	if err := r.client.Query(ctx, r.content); err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			logger.Warn("DM round 在发起查询前被取消")
			return
		}
		r.failRound(err)
		return
	}

	messageCh := r.client.ReceiveMessages(ctx)
	for {
		select {
		case <-ctx.Done():
			logger.Warn("DM round 上下文已取消")
			return
		case incoming, ok := <-messageCh:
			if !ok {
				if r.terminalSeen {
					logger.Info("DM round 消息流关闭")
					return
				}
				r.failRound(errors.New("DM 子任务在收到终态前提前结束"))
				return
			}
			logger.Debug("Agent ", runtimectx.BuildSDKMessageLogFields(incoming)...)
			if r.handleIncomingMessage(incoming) {
				return
			}
		}
	}
}

func (r *roundRunner) handleIncomingMessage(message sdkprotocol.ReceivedMessage) bool {
	events, terminalStatus, resultSubtype := r.mapper.Map(message)
	if sid := strings.TrimSpace(firstNonEmpty(r.mapper.SessionID(), message.SessionID, r.client.SessionID())); sid != "" {
		r.session.SessionID = &sid
	}

	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage {
			payload := session.Message(event.Data)
			if payload != nil {
				if err := r.persistMessage(payload); err != nil {
					r.failRound(err)
					return true
				}
				if payload["role"] == "assistant" {
					r.service.permission.BindSessionRoute(r.sessionKey, permission3.RouteContext{
						DispatchSessionKey: r.sessionKey,
						AgentID:            r.agent.AgentID,
						MessageID:          normalizeString(payload["message_id"]),
						CausedBy:           r.roundID,
					})
				}
			}
		}
		r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, event)
	}

	if terminalStatus != "" {
		r.terminalSeen = true
		r.service.loggerFor(context.Background()).Info("DM round 结束",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"status", terminalStatus,
			"result_subtype", resultSubtype,
		)
		r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
		r.service.permission.BroadcastEvent(
			context.Background(),
			r.sessionKey,
			protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, terminalStatus, resultSubtype),
		)
		r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
		return true
	}
	return false
}

func (r *roundRunner) failRound(err error) {
	r.service.loggerFor(context.Background()).Error("DM round 执行失败",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"err", err,
	)
	r.terminalSeen = true
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	persistedSessionID := ""
	if r.session.SessionID != nil {
		persistedSessionID = strings.TrimSpace(*r.session.SessionID)
	}
	resultMessage := session.Message{
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
		event := protocol.NewEvent(protocol.EventTypeMessage, resultMessage)
		event.SessionKey = r.sessionKey
		event.AgentID = r.agent.AgentID
		event.MessageID = normalizeString(resultMessage["message_id"])
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

func (r *roundRunner) persistMessage(message session.Message) error {
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
	message session.Message,
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
	message session.Message,
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
	message session.Message,
) (*session.Session, error) {
	current.SessionID = preferSessionID(current.SessionID, normalizeString(message["session_id"]))
	if normalizeString(message["role"]) == "result" {
		current.Status = "active"
	}
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
) (session.Session, error) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" || stringPointerValue(current.SessionID) == trimmedSessionID {
		return current, nil
	}
	current.SessionID = &trimmedSessionID
	updated, err := s.files.UpsertSession(workspacePath, current)
	if err != nil {
		return session.Session{}, err
	}
	if updated == nil {
		return current, nil
	}
	if s.roomStore != nil && updated.RoomSessionID != nil && strings.TrimSpace(*updated.RoomSessionID) != "" {
		if err := s.roomStore.UpdateRoomSessionSDKSessionID(ctx, strings.TrimSpace(*updated.RoomSessionID), trimmedSessionID); err != nil {
			return session.Session{}, err
		}
	}
	return *updated, nil
}

func mergeRoomBackedSession(current session.Session, roomSession session.Session) session.Session {
	merged := current
	merged.SessionKey = firstNonEmpty(merged.SessionKey, roomSession.SessionKey)
	merged.AgentID = firstNonEmpty(merged.AgentID, roomSession.AgentID)
	merged.ChannelType = firstNonEmpty(merged.ChannelType, roomSession.ChannelType)
	merged.ChatType = firstNonEmpty(merged.ChatType, roomSession.ChatType)
	merged.Status = firstNonEmpty(merged.Status, roomSession.Status)
	merged.Title = firstNonEmpty(merged.Title, roomSession.Title)
	if merged.RoomSessionID == nil && roomSession.RoomSessionID != nil {
		merged.RoomSessionID = roomSession.RoomSessionID
	}
	if merged.RoomID == nil && roomSession.RoomID != nil {
		merged.RoomID = roomSession.RoomID
	}
	if merged.ConversationID == nil && roomSession.ConversationID != nil {
		merged.ConversationID = roomSession.ConversationID
	}
	if merged.SessionID == nil && roomSession.SessionID != nil {
		merged.SessionID = roomSession.SessionID
	}
	if merged.Options == nil {
		merged.Options = map[string]any{}
	}
	if roomSession.Options != nil {
		for key, value := range roomSession.Options {
			if _, exists := merged.Options[key]; !exists {
				merged.Options[key] = value
			}
		}
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
