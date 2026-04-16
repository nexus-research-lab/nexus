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
	agent3 "github.com/nexus-research-lab/nexus-core/internal/agent"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	"github.com/nexus-research-lab/nexus-core/internal/logx"
	permission3 "github.com/nexus-research-lab/nexus-core/internal/permission"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus-core/internal/providerconfig"
	runtimectx "github.com/nexus-research-lab/nexus-core/internal/runtime"
	"github.com/nexus-research-lab/nexus-core/internal/sessiondomain"
	workspacestore "github.com/nexus-research-lab/nexus-core/internal/storage/workspace"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

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
	session       sessiondomain.Session
	agent         *agent3.Agent
	sessionKey    string
	roundID       string
	reqID         string
	content       string
	client        runtimectx.Client
	mapper        *messageMapper
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

	client, err := s.ensureClient(ctx, sessionKey, agentValue, request)
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
	resultMessage := sessiondomain.Message{
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

func (s *Service) ensureClient(
	ctx context.Context,
	sessionKey string,
	agentValue *agent3.Agent,
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
	// 中文注释：当前 Go SDK 链路先与 Python 主线对齐，暂不透传 model，
	// 避免向底层 CLI 传入尚未稳定支持的选项。
	runtimeEnv, err := s.buildRuntimeEnv(ctx, agentValue)
	if err != nil {
		return nil, err
	}
	options := agentclient.Options{
		CWD:               agentValue.WorkspacePath,
		PermissionMode:    permissionMode,
		AllowedTools:      append([]string(nil), agentValue.Options.AllowedTools...),
		DisallowedTools:   append([]string(nil), agentValue.Options.DisallowedTools...),
		SettingSources:    append([]string(nil), agentValue.Options.SettingSources...),
		Env:               runtimeEnv,
		PermissionHandler: permissionHandler,
	}
	if model := strings.TrimSpace(agentValue.Options.Model); model != "" {
		options.Model = model
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
		"ANTHROPIC_AUTH_TOKEN": runtimeConfig.AuthToken,
		"ANTHROPIC_BASE_URL":   runtimeConfig.BaseURL,
		"ANTHROPIC_MODEL":      runtimeConfig.Model,
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
) (sessiondomain.Session, error) {
	item, _, err := s.files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil {
		return sessiondomain.Session{}, err
	}
	if item != nil {
		return *item, nil
	}

	now := time.Now().UTC()
	created, err := s.files.UpsertSession(agentValue.WorkspacePath, sessiondomain.Session{
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
		return sessiondomain.Session{}, err
	}
	if created == nil {
		return sessiondomain.Session{}, fmt.Errorf("创建 session 失败: %s", sessionKey)
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
				r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
				r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
				logger.Info("DM round 消息流关闭")
				return
			}
			r.handleIncomingMessage(incoming)
		}
	}
}

func (r *roundRunner) handleIncomingMessage(message sdkprotocol.ReceivedMessage) {
	events, terminalStatus, resultSubtype := r.mapper.Map(message)
	if sid := strings.TrimSpace(firstNonEmpty(message.SessionID, r.client.SessionID())); sid != "" {
		r.session.SessionID = &sid
	}

	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage {
			payload := sessiondomain.Message(event.Data)
			if payload != nil {
				if err := r.persistMessage(payload); err != nil {
					r.failRound(err)
					return
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
	}
}

func (r *roundRunner) failRound(err error) {
	r.service.loggerFor(context.Background()).Error("DM round 执行失败",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"err", err,
	)
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, protocol.NewErrorEvent(r.sessionKey, err.Error()))
	r.service.permission.BroadcastEvent(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, "error", "error"),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
}

func (r *roundRunner) buildUserMessage() sessiondomain.Message {
	message := sessiondomain.Message{
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

func (r *roundRunner) persistMessage(message sessiondomain.Message) error {
	if err := r.service.files.AppendSessionMessage(r.workspacePath, r.sessionKey, message); err != nil {
		return err
	}
	if message["role"] == "result" {
		if err := r.service.files.AppendSessionCost(r.workspacePath, r.sessionKey, buildCostRow(message)); err != nil {
			return err
		}
	}
	r.session.MessageCount++
	r.session.LastActivity = time.Now().UTC()
	r.session.SessionID = preferSessionID(r.session.SessionID, normalizeString(message["session_id"]))
	if message["role"] == "result" {
		r.session.Status = "active"
	}
	updated, err := r.service.files.UpsertSession(r.workspacePath, r.session)
	if err != nil {
		return err
	}
	if updated != nil {
		r.session = *updated
	}
	return nil
}

func buildCostRow(message sessiondomain.Message) map[string]any {
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
