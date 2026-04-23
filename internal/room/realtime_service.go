package room

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/conversation/titlegen"
	"github.com/nexus-research-lab/nexus/internal/logx"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	permission3 "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

const (
	interruptForceCancelDelay = 150 * time.Millisecond
	roomBroadcastTimeout      = 5 * time.Second
)

type roomClientFactory interface {
	New(agentclient.Options) runtimectx.Client
}

// RoomBroadcaster 负责把 Room 共享事件扇出到 room 级订阅者。
type RoomBroadcaster interface {
	Broadcast(context.Context, string, protocol.EventMessage) []error
}

type defaultRoomClientFactory struct{}

func (f defaultRoomClientFactory) New(options agentclient.Options) runtimectx.Client {
	return runtimectx.WrapSDKClient(agentclient.New(options))
}

// ChatRequest 表示 Room 共享会话的一次聊天请求。
type ChatRequest struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	Content        string
	RoundID        string
	ReqID          string
}

// InterruptRequest 表示 Room 会话中断请求。
type InterruptRequest struct {
	SessionKey string
	MsgID      string
}

type activeRoomSlot struct {
	RoomSessionID     string
	SDKSessionID      string
	AgentID           string
	AgentRoundID      string
	MsgID             string
	RuntimeSessionKey string
	WorkspacePath     string
	Client            runtimectx.Client
	Cancel            context.CancelFunc
	Status            string
	Index             int
	TimestampMS       int64
	Done              chan struct{}
	doneOnce          sync.Once
}

type activeRoomRound struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	RoomType       string
	RoundID        string
	Cancel         context.CancelFunc
	Slots          map[string]*activeRoomSlot
	Done           chan struct{}
	doneOnce       sync.Once
}

type roomRoundMapperAdapter struct {
	mapper *slotMessageMapper
}

func (a roomRoundMapperAdapter) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) (runtimectx.RoundMapResult, error) {
	events, messages, terminalStatus, err := a.mapper.Map(incoming, interruptReason...)
	if err != nil {
		return runtimectx.RoundMapResult{}, err
	}
	return runtimectx.RoundMapResult{
		Events:          events,
		DurableMessages: messages,
		TerminalStatus:  terminalStatus,
	}, nil
}

func (a roomRoundMapperAdapter) SessionID() string {
	return a.mapper.SessionID()
}

// RealtimeService 负责 Room 的共享流实时编排。
// MCPServerBuilder 由 bootstrap 注入，按当前会话上下文构造一组进程内 MCP server。
// 用 string 形参避免 room 包反向依赖 automation 子包，防止 import cycle。
type MCPServerBuilder func(agentID, sessionKey, sourceContextType string) map[string]agentclient.SDKMCPServer

type RealtimeService struct {
	config      config.Config
	rooms       *Service
	agents      *agent2.Service
	runtime     *runtimectx.Manager
	permission  *permission3.Context
	providers   runtimectx.RuntimeConfigResolver
	history     *workspacestore.AgentHistoryStore
	roomHistory *workspacestore.RoomHistoryStore
	factory     roomClientFactory
	broadcaster RoomBroadcaster
	logger      *slog.Logger
	mcpServers  MCPServerBuilder
	titles      roomTitleScheduler

	mu           sync.Mutex
	activeRounds map[string]*activeRoomRound
}

type roomTitleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

// NewRealtimeService 创建 Room 实时编排服务。
func NewRealtimeService(
	cfg config.Config,
	roomService *Service,
	agentService *agent2.Service,
	runtimeManager *runtimectx.Manager,
	permission *permission3.Context,
) *RealtimeService {
	return NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, defaultRoomClientFactory{})
}

// NewRealtimeServiceWithFactory 使用自定义客户端工厂创建服务。
func NewRealtimeServiceWithFactory(
	cfg config.Config,
	roomService *Service,
	agentService *agent2.Service,
	runtimeManager *runtimectx.Manager,
	permission *permission3.Context,
	factory roomClientFactory,
) *RealtimeService {
	if factory == nil {
		factory = defaultRoomClientFactory{}
	}
	return &RealtimeService{
		config:       cfg,
		rooms:        roomService,
		agents:       agentService,
		runtime:      runtimeManager,
		permission:   permission,
		history:      workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		roomHistory:  workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
		factory:      factory,
		logger:       logx.NewDiscardLogger(),
		activeRounds: make(map[string]*activeRoomRound),
	}
}

// SetRoomBroadcaster 注入 Room 共享事件广播器。
func (s *RealtimeService) SetRoomBroadcaster(broadcaster RoomBroadcaster) {
	s.broadcaster = broadcaster
}

// SetLogger 注入业务日志实例。
func (s *RealtimeService) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetProviderResolver 注入 Provider 运行时解析器。
func (s *RealtimeService) SetProviderResolver(resolver runtimectx.RuntimeConfigResolver) {
	s.providers = resolver
}

// SetMCPServerBuilder 注入按会话上下文构造进程内 MCP server 的工厂。
func (s *RealtimeService) SetMCPServerBuilder(builder MCPServerBuilder) {
	s.mcpServers = builder
}

// SetTitleGenerator 注入会话标题生成器。
func (s *RealtimeService) SetTitleGenerator(generator roomTitleScheduler) {
	s.titles = generator
}

// HandleChat 处理 Room 主对话消息。
func (s *RealtimeService) HandleChat(ctx context.Context, request ChatRequest) error {
	sessionKey, conversationID, err := s.validateChatRequest(request)
	if err != nil {
		return err
	}

	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	roomID := firstNonEmpty(strings.TrimSpace(request.RoomID), contextValue.Room.ID)

	if err = s.interruptRound(ctx, sessionKey, "", "收到新的用户消息，上一轮已停止", true); err != nil {
		return err
	}

	agentNameByID, agentByID, err := s.buildAgentDirectory(ctx, contextValue.Members)
	if err != nil {
		return err
	}
	targetAgentIDs := ResolveMentionAgentIDs(request.Content, reverseAgentNames(agentNameByID))
	if len(targetAgentIDs) == 0 && len(agentNameByID) == 1 {
		// 单成员直聊 Room 再强制 @mention 只会制造额外交互噪音，
		// 这里直接把唯一成员当作默认目标，保持与 DM 直觉一致。
		for agentID := range agentNameByID {
			targetAgentIDs = []string{agentID}
		}
	}
	s.loggerFor(ctx).Info("受理 Room 会话消息",
		"session_key", sessionKey,
		"room_id", roomID,
		"conversation_id", conversationID,
		"round_id", request.RoundID,
		"target_agent_count", len(targetAgentIDs),
		"target_agents", append([]string(nil), targetAgentIDs...),
		"content_chars", utf8.RuneCountInString(strings.TrimSpace(request.Content)),
	)

	history, err := s.roomHistory.ReadMessages(conversationID, nil)
	if err != nil {
		return err
	}

	userMessage := sessionmodel.Message{
		"message_id":      request.RoundID,
		"session_key":     sessionKey,
		"room_id":         roomID,
		"conversation_id": conversationID,
		"agent_id":        "",
		"round_id":        request.RoundID,
		"role":            "user",
		"content":         strings.TrimSpace(request.Content),
		"timestamp":       time.Now().UnixMilli(),
	}
	if err = s.persistSharedInlineMessage(conversationID, userMessage); err != nil {
		return err
	}
	s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomMessageEvent(roomID, conversationID, userMessage, request.RoundID))
	s.scheduleTitleGeneration(ctx, sessionKey, contextValue, strings.TrimSpace(request.Content))

	if len(targetAgentIDs) == 0 {
		s.loggerFor(ctx).Warn("Room 消息未命中任何目标成员",
			"session_key", sessionKey,
			"room_id", roomID,
			"conversation_id", conversationID,
			"round_id", request.RoundID,
		)
		hintMessage := sessionmodel.Message{
			"message_id":      "result_" + request.RoundID,
			"session_key":     sessionKey,
			"room_id":         roomID,
			"conversation_id": conversationID,
			"agent_id":        "",
			"round_id":        request.RoundID,
			"role":            "result",
			"subtype":         "success",
			"duration_ms":     0,
			"duration_api_ms": 0,
			"num_turns":       0,
			"result":          "请使用 @AgentName 指定要对话的成员",
			"is_error":        false,
			"timestamp":       time.Now().UnixMilli(),
		}
		if err = s.persistSharedInlineMessage(conversationID, hintMessage); err != nil {
			return err
		}
		s.broadcastSharedEvent(
			ctx,
			sessionKey,
			roomID,
			wrapRoomMessageEvent(
				roomID,
				conversationID,
				sessionmodel.ProjectResultMessage(nil, hintMessage),
				request.RoundID,
			),
		)
		s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomRoundStatusEvent(sessionKey, roomID, conversationID, request.RoundID, "finished", "success"))
		return nil
	}

	sessionsByAgent := make(map[string]SessionRecord, len(contextValue.Sessions))
	for _, item := range contextValue.Sessions {
		sessionsByAgent[item.AgentID] = item
	}

	activeRound := &activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         roomID,
		ConversationID: conversationID,
		RoomType:       contextValue.Room.RoomType,
		RoundID:        request.RoundID,
		Slots:          make(map[string]*activeRoomSlot),
		Done:           make(chan struct{}),
	}

	pending := make([]map[string]any, 0, len(targetAgentIDs))
	for index, agentID := range targetAgentIDs {
		sessionRecord, ok := sessionsByAgent[agentID]
		if !ok {
			continue
		}
		agentValue := agentByID[agentID]
		if agentValue == nil {
			continue
		}
		msgID := newRealtimeID()
		agentRoundID := request.RoundID
		if len(targetAgentIDs) > 1 {
			agentRoundID = fmt.Sprintf("%s:%s", request.RoundID, agentID)
		}
		activeRound.Slots[msgID] = &activeRoomSlot{
			RoomSessionID:     sessionRecord.ID,
			SDKSessionID:      strings.TrimSpace(sessionRecord.SDKSessionID),
			AgentID:           agentID,
			AgentRoundID:      agentRoundID,
			MsgID:             msgID,
			RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, agentID, contextValue.Room.RoomType),
			WorkspacePath:     agentValue.WorkspacePath,
			Status:            "pending",
			Index:             index,
			TimestampMS:       normalizeInt64(userMessage["timestamp"]),
			Done:              make(chan struct{}),
		}
		_ = sessionRecord
		pending = append(pending, map[string]any{
			"agent_id":  agentID,
			"msg_id":    msgID,
			"round_id":  agentRoundID,
			"status":    "pending",
			"timestamp": userMessage["timestamp"],
			"index":     index,
		})
	}
	if len(activeRound.Slots) == 0 {
		s.loggerFor(ctx).Warn("Room 中没有可用成员会话",
			"session_key", sessionKey,
			"room_id", roomID,
			"conversation_id", conversationID,
			"round_id", request.RoundID,
		)
		s.broadcastSharedEvent(ctx, sessionKey, roomID, s.newRoomErrorEvent(sessionKey, roomID, conversationID, "room_error", "Room 中没有可用成员会话", request.RoundID))
		s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomRoundStatusEvent(sessionKey, roomID, conversationID, request.RoundID, "error", "error"))
		return nil
	}

	roundCtx, cancel := context.WithCancel(context.Background())
	activeRound.Cancel = cancel
	s.registerRound(activeRound)
	s.runtime.StartRound(sessionKey, request.RoundID, cancel)

	s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomRoundStatusEvent(sessionKey, roomID, conversationID, request.RoundID, "running", ""))
	s.broadcastSharedEvent(ctx, sessionKey, roomID, wrapRoomChatAckEvent(sessionKey, roomID, conversationID, firstNonEmpty(request.ReqID, request.RoundID), request.RoundID, pending))
	s.broadcastSessionStatus(ctx, sessionKey)

	go s.runRound(roundCtx, activeRound, history, request.Content, agentNameByID, agentByID)
	return nil
}

func (s *RealtimeService) scheduleTitleGeneration(
	ctx context.Context,
	sessionKey string,
	contextValue *ConversationContextAggregate,
	content string,
) {
	if s.titles == nil || contextValue == nil {
		return
	}
	s.titles.Schedule(ctx, titlegen.Request{
		SessionKey:               sessionKey,
		Provider:                 "",
		Content:                  content,
		ConversationID:           contextValue.Conversation.ID,
		ConversationRoomID:       contextValue.Room.ID,
		ConversationTitle:        contextValue.Conversation.Title,
		ConversationRoomName:     contextValue.Room.Name,
		ConversationMessageCount: contextValue.Conversation.MessageCount,
	})
}

// HandleInterrupt 处理中断请求。
func (s *RealtimeService) HandleInterrupt(ctx context.Context, request InterruptRequest) error {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return err
	}
	return s.interruptRound(ctx, sessionKey, strings.TrimSpace(request.MsgID), "", false)
}

// CountRunningTasks 返回指定 Agent 当前在 Room 中的活跃任务数。
func (s *RealtimeService) CountRunningTasks(agentID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, roundValue := range s.activeRounds {
		for _, slot := range roundValue.Slots {
			if slot.AgentID == agentID && slot.Status != "finished" && slot.Status != "error" && slot.Status != "cancelled" {
				count++
			}
		}
	}
	return count
}

func (s *RealtimeService) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

func (s *RealtimeService) withBroadcastTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, roomBroadcastTimeout)
}

func (s *RealtimeService) broadcastSharedEventWithTimeout(
	ctx context.Context,
	sessionKey string,
	roomID string,
	event protocol.EventMessage,
) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	s.broadcastSharedEvent(broadcastCtx, sessionKey, roomID, event)
}

func (s *RealtimeService) broadcastSessionStatus(ctx context.Context, sessionKey string) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	if errs := s.permission.BroadcastSessionStatus(
		broadcastCtx,
		sessionKey,
		s.runtime.GetRunningRoundIDs(sessionKey),
	); len(errs) > 0 {
		s.loggerFor(broadcastCtx).Warn("广播 room session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

// InterruptConversation 中断指定 conversation 的全部活跃轮次。
func (s *RealtimeService) InterruptConversation(ctx context.Context, conversationID string, message string) error {
	normalizedConversationID := strings.TrimSpace(conversationID)
	if normalizedConversationID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectRoundTargets(func(roundValue *activeRoomRound) bool {
		return roundValue.ConversationID == normalizedConversationID
	}), message)
}

// InterruptRoom 中断指定 Room 下的全部活跃轮次。
func (s *RealtimeService) InterruptRoom(ctx context.Context, roomID string, message string) error {
	normalizedRoomID := strings.TrimSpace(roomID)
	if normalizedRoomID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectRoundTargets(func(roundValue *activeRoomRound) bool {
		return roundValue.RoomID == normalizedRoomID
	}), message)
}

// InterruptAgentTasks 中断指定成员在 Room 中的全部活跃子任务。
func (s *RealtimeService) InterruptAgentTasks(ctx context.Context, roomID string, agentID string, message string) error {
	normalizedRoomID := strings.TrimSpace(roomID)
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedRoomID == "" || normalizedAgentID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectSlotTargets(func(roundValue *activeRoomRound, slot *activeRoomSlot) bool {
		return roundValue.RoomID == normalizedRoomID && slot.AgentID == normalizedAgentID
	}), message)
}

// ActiveRoundSnapshot 表示 Room 当前仍在执行的主轮次快照。
type ActiveRoundSnapshot struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	RoundID        string
	Pending        []map[string]any
}

// GetActiveRoundSnapshot 返回指定 conversation 的活跃 slot 快照。
func (s *RealtimeService) GetActiveRoundSnapshot(conversationID string) *ActiveRoundSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, roundValue := range s.activeRounds {
		if roundValue == nil || roundValue.ConversationID != conversationID {
			continue
		}
		pending := make([]map[string]any, 0, len(roundValue.Slots))
		for _, slot := range roundValue.Slots {
			if slot == nil || slot.Status == "finished" || slot.Status == "error" || slot.Status == "cancelled" {
				continue
			}
			status := slot.Status
			if status == "running" {
				status = "streaming"
			}
			pending = append(pending, map[string]any{
				"agent_id":  slot.AgentID,
				"msg_id":    slot.MsgID,
				"round_id":  slot.AgentRoundID,
				"status":    status,
				"timestamp": slot.TimestampMS,
				"index":     slot.Index,
			})
		}
		if len(pending) == 0 {
			return nil
		}
		sort.Slice(pending, func(i int, j int) bool {
			return intValue(pending[i]["index"]) < intValue(pending[j]["index"])
		})
		for _, item := range pending {
			delete(item, "index")
		}
		return &ActiveRoundSnapshot{
			SessionKey:     roundValue.SessionKey,
			RoomID:         roundValue.RoomID,
			ConversationID: roundValue.ConversationID,
			RoundID:        roundValue.RoundID,
			Pending:        pending,
		}
	}
	return nil
}

func (s *RealtimeService) runRound(
	ctx context.Context,
	roundValue *activeRoomRound,
	history []sessionmodel.Message,
	latestUserMessage string,
	agentNameByID map[string]string,
	agentByID map[string]*agent2.Agent,
) {
	logger := s.loggerFor(ctx).With(
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"round_id", roundValue.RoundID,
	)
	logger.Info("开始执行 Room round", "slot_count", len(roundValue.Slots))
	var waitGroup sync.WaitGroup
	for _, slot := range roundValue.Slots {
		waitGroup.Add(1)
		go func(currentSlot *activeRoomSlot) {
			defer waitGroup.Done()
			s.runSlot(ctx, roundValue, currentSlot, history, latestUserMessage, agentNameByID, agentByID[currentSlot.AgentID])
		}(slot)
	}
	waitGroup.Wait()

	s.finishRound(roundValue.SessionKey)

	finalStatus := "finished"
	if roundValue.allSlotsCancelled() {
		finalStatus = "interrupted"
	}
	logger.Info("Room round 结束", "status", finalStatus)
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, wrapRoomRoundStatusEvent(
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		roundValue.RoundID,
		finalStatus,
		mapTerminalSubtype(finalStatus),
	))
	s.broadcastSessionStatus(ctx, roundValue.SessionKey)
}

func (s *RealtimeService) runSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	history []sessionmodel.Message,
	latestUserMessage string,
	agentNameByID map[string]string,
	agentValue *agent2.Agent,
) {
	if agentValue == nil {
		slot.Status = "error"
		s.loggerFor(ctx).Error("Room slot 缺少 agent 配置",
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"round_id", slot.AgentRoundID,
			"agent_id", slot.AgentID,
			"msg_id", slot.MsgID,
		)
		return
	}

	slotCtx, cancel := context.WithCancel(ctx)
	slot.Cancel = cancel
	logger := s.loggerFor(slotCtx).With(
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
	)
	mapper := newSlotMessageMapper(roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID, slot.AgentID, slot.MsgID, slot.AgentRoundID)
	slot.Status = "running"
	logger.Info("开始执行 Room slot")
	defer s.finishSlot(slot)

	if err := s.recordPrivateRoundMarker(slot, latestUserMessage); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}

	s.permission.BindSessionRoute(slot.RuntimeSessionKey, permission3.RouteContext{
		DispatchSessionKey: roundValue.SessionKey,
		RoomID:             roundValue.RoomID,
		ConversationID:     roundValue.ConversationID,
		AgentID:            slot.AgentID,
		MessageID:          slot.MsgID,
		CausedBy:           slot.AgentRoundID,
	})

	appendSystemPrompt, err := s.agents.BuildRuntimePrompt(slotCtx, agentValue)
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	mcpServers := map[string]agentclient.SDKMCPServer(nil)
	if s.mcpServers != nil {
		mcpServers = s.mcpServers(agentValue.AgentID, slot.RuntimeSessionKey, "room")
	}
	permissionMode := sdkprotocol.PermissionMode(agentValue.Options.PermissionMode)
	permissionHandler := func(permissionCtx context.Context, request sdkprotocol.PermissionRequest) (sdkprotocol.PermissionDecision, error) {
		return s.permission.RequestPermission(permissionCtx, slot.RuntimeSessionKey, request)
	}
	options, err := runtimectx.BuildAgentClientOptions(slotCtx, s.providers, runtimectx.AgentClientOptionsInput{
		WorkspacePath:      agentValue.WorkspacePath,
		Provider:           agentValue.Options.Provider,
		PermissionMode:     permissionMode,
		PermissionHandler:  permissionHandler,
		AllowedTools:       agentValue.Options.AllowedTools,
		DisallowedTools:    agentValue.Options.DisallowedTools,
		SettingSources:     agentValue.Options.SettingSources,
		AppendSystemPrompt: appendSystemPrompt,
		ResumeSessionID:    slot.SDKSessionID,
		MaxThinkingTokens:  agentValue.Options.MaxThinkingTokens,
		MaxTurns:           agentValue.Options.MaxTurns,
		MCPServers:         mcpServers,
	})
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	client := s.factory.New(options)
	slot.Client = client
	defer s.permission.UnbindSessionRoute(slot.RuntimeSessionKey)

	if err := client.Connect(slotCtx); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	defer client.Disconnect(context.Background())
	if err := s.syncSlotSDKSessionID(slotCtx, slot, client.SessionID()); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}

	s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, wrapRoomLifecycleEvent(
		protocol.EventTypeStreamStart,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))

	dispatchPrompt := BuildDispatchPrompt(history, latestUserMessage, agentNameByID, slot.AgentID)
	result, err := runtimectx.ExecuteRound(slotCtx, runtimectx.RoundExecutionRequest{
		Query:  dispatchPrompt,
		Client: client,
		Mapper: roomRoundMapperAdapter{mapper: mapper},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			if logger.Enabled(slotCtx, slog.LevelDebug) {
				logger.Debug("Agent ", runtimectx.BuildSDKMessageLogFields(incoming)...)
			}
		},
		SyncSessionID: func(sessionID string) error {
			return s.syncSlotSDKSessionID(slotCtx, slot, sessionID)
		},
		HandleDurableMessage: func(messageValue sessionmodel.Message) error {
			if err := s.persistSharedDurableMessage(roundValue.ConversationID, slot, messageValue); err != nil {
				return err
			}
			if !sessionmodel.IsTranscriptNativeMessage(sessionmodel.Message(messageValue)) {
				if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(messageValue, slot.RuntimeSessionKey)); err != nil {
					return err
				}
			}
			if sessionmodel.MessageRole(messageValue) == "result" {
				slot.Status = resultStatus(messageValue["subtype"])
			}
			return nil
		},
		EmitEvent: func(event protocol.EventMessage) error {
			s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, event)
			return nil
		},
	})
	if err != nil {
		if errors.Is(err, runtimectx.ErrRoundInterrupted) {
			s.handleSlotCancelled(slotCtx, roundValue, slot, mapper)
			return
		}
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}

	if slot.Status == "running" {
		slot.Status = resultStatus(result.ResultSubtype)
	}
	s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, wrapRoomLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
	logger.Info("Room slot 结束", "status", slot.Status)
}

func (s *RealtimeService) syncSlotSDKSessionID(ctx context.Context, slot *activeRoomSlot, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || sessionID == strings.TrimSpace(slot.SDKSessionID) {
		return nil
	}
	slot.SDKSessionID = sessionID
	if s.rooms == nil {
		return nil
	}
	return s.rooms.UpdateSessionSDKSessionID(ctx, slot.RoomSessionID, sessionID)
}

func (s *RealtimeService) handleSlotFailure(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, mapper *slotMessageMapper, err error) {
	s.loggerFor(ctx).Error("Room slot 执行失败",
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
		"err", err,
	)
	slot.Status = "error"
	resultMessage := sessionmodel.Message{
		"message_id":      "result_" + slot.AgentRoundID,
		"session_key":     roundValue.SessionKey,
		"room_id":         roundValue.RoomID,
		"conversation_id": roundValue.ConversationID,
		"agent_id":        slot.AgentID,
		"round_id":        slot.AgentRoundID,
		"parent_id":       slot.MsgID,
		"role":            "result",
		"subtype":         "error",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"result":          err.Error(),
		"is_error":        true,
		"timestamp":       time.Now().UnixMilli(),
	}
	_ = s.persistSharedInlineMessage(roundValue.ConversationID, resultMessage)
	_ = s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(resultMessage, slot.RuntimeSessionKey))
	projectedMessage := sessionmodel.ProjectResultMessage(nil, resultMessage)
	if mapper != nil {
		projectedMessage = mapper.ProjectResultMessage(resultMessage)
	}
	s.broadcastSharedEventWithTimeout(
		ctx,
		roundValue.SessionKey,
		roundValue.RoomID,
		wrapRoomMessageEvent(
			roundValue.RoomID,
			roundValue.ConversationID,
			projectedMessage,
			slot.AgentRoundID,
		),
	)
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, s.newRoomErrorEvent(roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID, "room_error", err.Error(), slot.AgentRoundID))
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, wrapRoomLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
}

func (s *RealtimeService) handleSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, mapper *slotMessageMapper) {
	if !s.markSlotCancelled(slot) {
		return
	}
	s.loggerFor(ctx).Warn("Room slot 已取消",
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
	)
	s.emitInterruptedSlotResult(roundValue, slot, mapper, "")
	s.broadcastSlotCancelled(ctx, roundValue, slot)
}

func (s *RealtimeService) markSlotCancelled(slot *activeRoomSlot) bool {
	if slot == nil || slot.Status == "cancelled" {
		return false
	}
	slot.Status = "cancelled"
	return true
}

func (s *RealtimeService) broadcastSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot) {
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, wrapRoomLifecycleEvent(
		protocol.EventTypeStreamCancelled,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
}

func (s *RealtimeService) emitInterruptedSlotResult(roundValue *activeRoomRound, slot *activeRoomSlot, mapper *slotMessageMapper, resultText string) {
	if roundValue == nil || slot == nil {
		return
	}
	resultMessage := sessionmodel.Message{
		"message_id":      "result_" + slot.AgentRoundID,
		"session_key":     roundValue.SessionKey,
		"room_id":         roundValue.RoomID,
		"conversation_id": roundValue.ConversationID,
		"agent_id":        slot.AgentID,
		"round_id":        slot.AgentRoundID,
		"parent_id":       slot.MsgID,
		"role":            "result",
		"subtype":         "interrupted",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"is_error":        false,
		"timestamp":       time.Now().UnixMilli(),
	}
	if trimmedResult := strings.TrimSpace(resultText); trimmedResult != "" {
		resultMessage["result"] = trimmedResult
	}
	if slot.Client != nil {
		if sessionID := strings.TrimSpace(slot.Client.SessionID()); sessionID != "" {
			resultMessage["session_id"] = sessionID
		}
	}
	if err := s.persistSharedInlineMessage(roundValue.ConversationID, resultMessage); err != nil {
		s.loggerFor(context.Background()).Error("Room interrupted 共享结果持久化失败",
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"agent_id", slot.AgentID,
			"round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"err", err,
		)
	} else {
		projectedMessage := sessionmodel.ProjectResultMessage(nil, resultMessage)
		if mapper != nil {
			projectedMessage = mapper.ProjectResultMessage(resultMessage)
		}
		s.broadcastSharedEvent(
			context.Background(),
			roundValue.SessionKey,
			roundValue.RoomID,
			wrapRoomMessageEvent(
				roundValue.RoomID,
				roundValue.ConversationID,
				projectedMessage,
				slot.AgentRoundID,
			),
		)
	}
	if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(resultMessage, slot.RuntimeSessionKey)); err != nil {
		s.loggerFor(context.Background()).Error("Room interrupted 私有结果持久化失败",
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"agent_id", slot.AgentID,
			"round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"err", err,
		)
	}
}

func (s *RealtimeService) recordPrivateRoundMarker(slot *activeRoomSlot, latestUserMessage string) error {
	if s.history == nil {
		return nil
	}
	return s.history.AppendRoundMarker(
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		slot.AgentRoundID,
		strings.TrimSpace(latestUserMessage),
		time.Now().UnixMilli(),
	)
}

func (s *RealtimeService) persistPrivateOverlayMessage(slot *activeRoomSlot, message sessionmodel.Message) error {
	if s.history == nil {
		return nil
	}
	privateMessage := normalizePrivateOverlayMessage(cloneMessageWithSessionKey(message, slot.RuntimeSessionKey))
	privateMessage["session_key"] = slot.RuntimeSessionKey
	if sessionID := firstNonEmpty(strings.TrimSpace(anyString(privateMessage["session_id"])), strings.TrimSpace(slot.SDKSessionID)); sessionID != "" {
		privateMessage["session_id"] = sessionID
	}
	if strings.TrimSpace(anyString(privateMessage["message_id"])) == "" {
		privateMessage["message_id"] = "overlay_" + slot.AgentRoundID
	}
	privateMessage["metadata"] = mergePrivateOverlayMetadata(privateMessage["metadata"], map[string]any{
		"overlay_source":  "room_runtime",
		"room_session_id": slot.RoomSessionID,
	})
	return s.history.AppendOverlayMessage(slot.WorkspacePath, slot.RuntimeSessionKey, privateMessage)
}

func (s *RealtimeService) validateChatRequest(request ChatRequest) (string, string, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", "", err
	}
	if !protocol.IsRoomSharedSessionKey(sessionKey) {
		return "", "", errors.New("session_key must be room shared key")
	}
	if strings.TrimSpace(request.RoundID) == "" {
		return "", "", errors.New("round_id is required")
	}
	if strings.TrimSpace(request.Content) == "" {
		return "", "", errors.New("content is required")
	}
	conversationID := firstNonEmpty(strings.TrimSpace(request.ConversationID), protocol.ParseRoomConversationID(sessionKey))
	if conversationID == "" {
		return "", "", errors.New("conversation_id is required")
	}
	return sessionKey, conversationID, nil
}

func (s *RealtimeService) buildAgentDirectory(
	ctx context.Context,
	members []MemberRecord,
) (map[string]string, map[string]*agent2.Agent, error) {
	agentNameByID := make(map[string]string)
	agentByID := make(map[string]*agent2.Agent)
	for _, member := range members {
		if member.MemberType != "agent" || strings.TrimSpace(member.MemberAgentID) == "" {
			continue
		}
		agentValue, err := s.agents.GetAgent(ctx, member.MemberAgentID)
		if err != nil {
			return nil, nil, err
		}
		agentNameByID[agentValue.AgentID] = agentValue.Name
		agentByID[agentValue.AgentID] = agentValue
	}
	return agentNameByID, agentByID, nil
}

func (s *RealtimeService) persistSharedInlineMessage(conversationID string, message sessionmodel.Message) error {
	return s.roomHistory.AppendInlineMessage(conversationID, message)
}

func (s *RealtimeService) persistSharedDurableMessage(
	conversationID string,
	slot *activeRoomSlot,
	message sessionmodel.Message,
) error {
	if slot == nil || !sessionmodel.IsTranscriptNativeMessage(sessionmodel.Message(message)) {
		return s.persistSharedInlineMessage(conversationID, message)
	}
	return s.roomHistory.AppendTranscriptReference(
		conversationID,
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		message,
	)
}

func normalizePrivateOverlayMessage(message sessionmodel.Message) sessionmodel.Message {
	normalized := cloneMessageWithSessionKey(message, anyString(message["session_key"]))
	delete(normalized, "stream_status")
	delete(normalized, "is_complete")
	return normalized
}

func mergePrivateOverlayMetadata(current any, extra map[string]any) map[string]any {
	result := map[string]any{}
	if payload, ok := current.(map[string]any); ok {
		for key, value := range payload {
			result[key] = value
		}
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
}

func (s *RealtimeService) registerRound(roundValue *activeRoomRound) {
	s.mu.Lock()
	s.activeRounds[roundValue.SessionKey] = roundValue
	s.mu.Unlock()
}

func (s *RealtimeService) broadcastSharedEvent(ctx context.Context, sessionKey string, roomID string, event protocol.EventMessage) {
	if s.broadcaster != nil && strings.TrimSpace(roomID) != "" {
		s.broadcaster.Broadcast(ctx, roomID, event)
		return
	}
	s.permission.BroadcastEvent(ctx, sessionKey, event)
}

func (s *RealtimeService) finishRound(sessionKey string) {
	var roundValue *activeRoomRound
	s.runtime.MarkRoundFinished(sessionKey, s.currentRoundID(sessionKey))
	s.mu.Lock()
	roundValue = s.activeRounds[sessionKey]
	delete(s.activeRounds, sessionKey)
	s.mu.Unlock()
	if roundValue != nil {
		roundValue.doneOnce.Do(func() {
			close(roundValue.Done)
		})
	}
}

func (s *RealtimeService) currentRoundID(sessionKey string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if roundValue := s.activeRounds[sessionKey]; roundValue != nil {
		return roundValue.RoundID
	}
	return ""
}

func (s *RealtimeService) finishSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.doneOnce.Do(func() {
		close(slot.Done)
	})
}

type interruptTarget struct {
	SessionKey string
	MsgID      string
}

func (s *RealtimeService) collectRoundTargets(
	matcher func(*activeRoomRound) bool,
) []interruptTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	targets := make([]interruptTarget, 0)
	for _, roundValue := range s.activeRounds {
		if roundValue == nil || !matcher(roundValue) {
			continue
		}
		targets = append(targets, interruptTarget{SessionKey: roundValue.SessionKey})
	}
	return targets
}

func (s *RealtimeService) collectSlotTargets(
	matcher func(*activeRoomRound, *activeRoomSlot) bool,
) []interruptTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	targets := make([]interruptTarget, 0)
	seen := make(map[string]struct{})
	for _, roundValue := range s.activeRounds {
		if roundValue == nil {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || !matcher(roundValue, slot) {
				continue
			}
			targetKey := roundValue.SessionKey + "::" + slot.MsgID
			if _, exists := seen[targetKey]; exists {
				continue
			}
			seen[targetKey] = struct{}{}
			targets = append(targets, interruptTarget{
				SessionKey: roundValue.SessionKey,
				MsgID:      slot.MsgID,
			})
		}
	}
	return targets
}

func (s *RealtimeService) interruptTargets(
	ctx context.Context,
	targets []interruptTarget,
	message string,
) error {
	errs := make([]error, 0)
	for _, target := range targets {
		if err := s.interruptRound(ctx, target.SessionKey, target.MsgID, message, true); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *RealtimeService) interruptRound(
	ctx context.Context,
	sessionKey string,
	msgID string,
	message string,
	suppressError bool,
) error {
	s.mu.Lock()
	roundValue := s.activeRounds[sessionKey]
	s.mu.Unlock()
	if roundValue == nil {
		return nil
	}

	if strings.TrimSpace(msgID) != "" {
		slot := roundValue.Slots[msgID]
		if slot == nil {
			if suppressError {
				return nil
			}
			return errors.New("target room slot not found")
		}
		shouldBroadcast := slot.Status != "finished" && slot.Status != "error" && slot.Status != "cancelled"
		if slot.Client != nil {
			if err := slot.Client.Interrupt(ctx); err != nil && !suppressError {
				return err
			}
		}
		s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, message)
		if shouldBroadcast {
			s.loggerFor(ctx).Warn("请求中断 Room slot",
				"session_key", sessionKey,
				"room_id", roundValue.RoomID,
				"conversation_id", roundValue.ConversationID,
				"agent_id", slot.AgentID,
				"round_id", slot.AgentRoundID,
				"msg_id", slot.MsgID,
				"reason", message,
			)
		}
		select {
		case <-slot.Done:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interruptForceCancelDelay):
			if slot.Cancel != nil {
				slot.Cancel()
			}
			select {
			case <-slot.Done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		s.broadcastSessionStatus(ctx, sessionKey)
		return nil
	}

	s.loggerFor(ctx).Warn("请求中断 Room round",
		"session_key", sessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"round_id", roundValue.RoundID,
		"reason", message,
	)
	for _, slot := range roundValue.Slots {
		if slot.Client != nil {
			if err := slot.Client.Interrupt(ctx); err != nil && !suppressError {
				return err
			}
		}
		s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, message)
	}
	select {
	case <-roundValue.Done:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(interruptForceCancelDelay):
		if roundValue.Cancel != nil {
			roundValue.Cancel()
		}
		select {
		case <-roundValue.Done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	return nil
}

func (r *activeRoomRound) allSlotsCancelled() bool {
	if len(r.Slots) == 0 {
		return false
	}
	for _, slot := range r.Slots {
		if slot.Status != "cancelled" {
			return false
		}
	}
	return true
}

func wrapRoomMessageEvent(roomID string, conversationID string, message sessionmodel.Message, causedBy string) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeMessage, message)
	event.DeliveryMode = "durable"
	event.SessionKey = anyString(message["session_key"])
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.AgentID = anyString(message["agent_id"])
	event.MessageID = anyString(message["message_id"])
	event.CausedBy = causedBy
	return event
}

func wrapRoomRoundStatusEvent(sessionKey string, roomID string, conversationID string, roundID string, status string, resultSubtype string) protocol.EventMessage {
	event := protocol.NewRoundStatusEvent(sessionKey, roundID, status, resultSubtype)
	event.DeliveryMode = "durable"
	event.RoomID = roomID
	event.ConversationID = conversationID
	return event
}

func wrapRoomChatAckEvent(sessionKey string, roomID string, conversationID string, reqID string, roundID string, pending []map[string]any) protocol.EventMessage {
	event := protocol.NewChatAckEvent(sessionKey, reqID, roundID, pending)
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.CausedBy = roundID
	return event
}

func wrapRoomLifecycleEvent(eventType protocol.EventType, sessionKey string, roomID string, conversationID string, agentID string, msgID string, roundID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, map[string]any{
		"msg_id":   msgID,
		"agent_id": agentID,
		"round_id": roundID,
	})
	event.SessionKey = sessionKey
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.AgentID = agentID
	event.MessageID = msgID
	event.CausedBy = roundID
	return event
}

func (s *RealtimeService) newRoomErrorEvent(sessionKey string, roomID string, conversationID string, errorType string, message string, causedBy string) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeError, map[string]any{
		"error_type": errorType,
		"message":    message,
	})
	event.SessionKey = sessionKey
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.CausedBy = causedBy
	return event
}

func reverseAgentNames(agentNameByID map[string]string) map[string]string {
	result := make(map[string]string, len(agentNameByID))
	for agentID, name := range agentNameByID {
		result[name] = agentID
	}
	return result
}

func mapTerminalSubtype(status string) string {
	switch status {
	case "finished":
		return "success"
	case "interrupted":
		return "interrupted"
	case "error":
		return "error"
	default:
		return ""
	}
}

func resultStatus(subtype any) string {
	switch strings.TrimSpace(anyString(subtype)) {
	case "interrupted":
		return "cancelled"
	case "error":
		return "error"
	default:
		return "finished"
	}
}

func cloneMessageWithSessionKey(message sessionmodel.Message, sessionKey string) sessionmodel.Message {
	result := make(sessionmodel.Message, len(message))
	for key, value := range message {
		result[key] = value
	}
	result["session_key"] = sessionKey
	return result
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	normalized := strings.TrimSpace(value)
	return &normalized
}

func normalizeInt64(value any) int64 {
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

func newRealtimeID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("room_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}
