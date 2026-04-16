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
	providers  providerRuntimeResolver
	files      *workspacestore.SessionFileStore
	logger     *slog.Logger
}

type providerRuntimeResolver interface {
	ResolveRuntimeConfig(context.Context, string) (*providercfg.RuntimeConfig, error)
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

	sessionItem, err := s.ensureSession(agentValue, parsed, sessionKey)
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

	if err = runner.persistMessage(runner.buildUserMessage()); err != nil {
		s.runtime.MarkRoundFinished(sessionKey, request.RoundID)
		s.permission.CancelRequestsForSession(sessionKey, "消息持久化失败")
		s.loggerFor(ctx).Error("DM 用户消息持久化失败",
			"session_key", sessionKey,
			"agent_id", agentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return err
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
	messages, err := s.files.ReadSessionMessages([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if normalizeString(message["role"]) != "assistant" {
			continue
		}
		if normalizeString(message["round_id"]) != roundID {
			continue
		}
		if isComplete, ok := message["is_complete"].(bool); ok && isComplete {
			continue
		}
		repaired := session.Message{}
		for key, value := range message {
			repaired[key] = value
		}
		repaired["is_complete"] = true
		repaired["stream_status"] = "cancelled"
		if err := s.files.AppendSessionMessage(agentValue.WorkspacePath, sessionKey, repaired); err != nil {
			return err
		}
	}
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
	sessionValue, err := s.ensureSession(agentValue, parsed, sessionKey)
	if err != nil {
		return err
	}
	if sessionValue.SessionID != nil && strings.TrimSpace(*sessionValue.SessionID) != "" {
		resultMessage["session_id"] = strings.TrimSpace(*sessionValue.SessionID)
	}
	if err := s.files.AppendSessionMessage(agentValue.WorkspacePath, sessionKey, resultMessage); err != nil {
		return err
	}
	if err := s.files.AppendSessionCost(agentValue.WorkspacePath, sessionKey, buildCostRow(resultMessage)); err != nil {
		return err
	}
	_, err = s.files.RefreshSessionMeta(agentValue.WorkspacePath, sessionKey, sessionValue)
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
	agentValue *agent3.Agent,
	parsed protocol.SessionKey,
	sessionKey string,
) (session.Session, error) {
	item, _, err := s.files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil {
		return session.Session{}, err
	}
	if item != nil {
		return *item, nil
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
		Options:      map[string]any{},
		IsActive:     true,
	})
	if err != nil {
		return session.Session{}, err
	}
	if created == nil {
		return session.Session{}, fmt.Errorf("创建 session 失败: %s", sessionKey)
	}
	return *created, nil
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
	if persistErr := r.persistMessage(resultMessage); persistErr != nil {
		r.service.loggerFor(context.Background()).Error("DM 错误结果持久化失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", persistErr,
		)
	} else {
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

func (r *roundRunner) buildUserMessage() session.Message {
	message := session.Message{
		"message_id":  r.roundID,
		"session_key": r.sessionKey,
		"agent_id":    r.agent.AgentID,
		"round_id":    r.roundID,
		"role":        "user",
		"content":     r.content,
		"timestamp":   time.Now().UnixMilli(),
	}
	if r.session.SessionID != nil {
		message["session_id"] = *r.session.SessionID
	}
	return message
}

func (r *roundRunner) persistMessage(message session.Message) error {
	if err := r.service.files.AppendSessionMessage(r.workspacePath, r.sessionKey, message); err != nil {
		return err
	}
	if message["role"] == "result" {
		if err := r.service.files.AppendSessionCost(r.workspacePath, r.sessionKey, buildCostRow(message)); err != nil {
			return err
		}
	}
	r.session.SessionID = preferSessionID(r.session.SessionID, normalizeString(message["session_id"]))
	if message["role"] == "result" {
		r.session.Status = "active"
	}
	r.session.LastActivity = time.Now().UTC()
	updated, err := r.service.files.RefreshSessionMeta(r.workspacePath, r.sessionKey, r.session)
	if err != nil {
		return err
	}
	if updated != nil {
		r.session = *updated
	}
	return nil
}

func buildCostRow(message session.Message) map[string]any {
	usage, _ := message["usage"].(map[string]any)
	return map[string]any{
		"entry_id":                    "cost_" + normalizeString(message["message_id"]),
		"agent_id":                    normalizeString(message["agent_id"]),
		"session_key":                 normalizeString(message["session_key"]),
		"session_id":                  normalizeString(message["session_id"]),
		"round_id":                    normalizeString(message["round_id"]),
		"message_id":                  normalizeString(message["message_id"]),
		"subtype":                     normalizeString(message["subtype"]),
		"input_tokens":                normalizeIntFromMap(usage, "input_tokens"),
		"output_tokens":               normalizeIntFromMap(usage, "output_tokens"),
		"cache_creation_input_tokens": normalizeIntFromMap(usage, "cache_creation_input_tokens"),
		"cache_read_input_tokens":     normalizeIntFromMap(usage, "cache_read_input_tokens"),
		"total_cost_usd":              normalizeFloat(message["total_cost_usd"]),
		"duration_ms":                 normalizeIntValue(message["duration_ms"]),
		"duration_api_ms":             normalizeIntValue(message["duration_api_ms"]),
		"num_turns":                   normalizeIntValue(message["num_turns"]),
		"created_at":                  time.Now().UTC().Format(time.RFC3339),
	}
}

func preferSessionID(current *string, next string) *string {
	if strings.TrimSpace(next) != "" {
		return &next
	}
	return current
}

func normalizeIntFromMap(raw map[string]any, key string) int {
	if raw == nil {
		return 0
	}
	return normalizeIntValue(raw[key])
}

func normalizeIntValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func normalizeFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
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
