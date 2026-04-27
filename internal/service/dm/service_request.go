package dm

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// HandleChat 处理一条 DM 写请求。
func (s *Service) HandleChat(ctx context.Context, request Request) error {
	sessionKey, parsed, err := s.validateRequest(request)
	if err != nil {
		return err
	}
	agentID := dmdomain.FirstNonEmpty(parsed.AgentID, request.AgentID)
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
	deliveryPolicy := protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy))

	if protocol.ShouldGuideRunningRound(deliveryPolicy) {
		delivered, guideErr := s.guideRunningInput(ctx, sessionKey, agentValue, sessionItem, request)
		if guideErr != nil && !errors.Is(guideErr, runtimectx.ErrNoRunningRound) {
			return guideErr
		}
		if delivered {
			return nil
		}
		// 引导只对已运行的 round 有意义；空闲时退化为普通新一轮，避免历史里出现假“已引导”用户消息。
		deliveryPolicy = protocol.ChatDeliveryPolicyQueue
	}

	if protocol.ShouldQueueRunningRound(deliveryPolicy) {
		delivered, queueErr := s.queueRunningInput(ctx, sessionKey, agentValue, sessionItem, request, initialMessageCount)
		if queueErr != nil && !errors.Is(queueErr, runtimectx.ErrNoRunningRound) {
			return queueErr
		}
		if delivered {
			return nil
		}
	}

	if deliveryPolicy == protocol.ChatDeliveryPolicyInterrupt {
		if err = s.interruptSession(ctx, sessionKey, "收到新的用户消息，上一轮已停止"); err != nil {
			return err
		}
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
	s.permission.BindSessionRoute(sessionKey, permissionctx.RouteContext{
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
		reqID:             dmdomain.FirstNonEmpty(request.ReqID, request.RoundID),
		content:           strings.TrimSpace(request.Content),
		client:            client,
		runtimeProvider:   runtimeProvider,
		runtimeModel:      runtimeModel,
		ownerUserID:       authctx.OwnerUserID(ctx),
		mapper:            dmdomain.NewMessageMapper(sessionKey, agentID, request.RoundID),
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

	if err = s.recordRoundMarker(runner.workspacePath, runner.session, runner.roundID, runner.content, deliveryPolicy); err != nil {
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
	if request.BroadcastUserMessage {
		s.broadcastUserRoundMarker(ctx, runner.session, runner.roundID, runner.content, deliveryPolicy)
	}
	s.permission.BroadcastEvent(ctx, sessionKey, protocol.NewRoundStatusEvent(sessionKey, request.RoundID, "running", ""))
	s.broadcastSessionStatus(ctx, sessionKey)

	go runner.run(roundCtx)
	return nil
}

func (s *Service) queueRunningInput(
	ctx context.Context,
	sessionKey string,
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
	request Request,
	initialMessageCount int,
) (bool, error) {
	content := strings.TrimSpace(request.Content)
	runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey)
	if len(runningRoundIDs) == 0 {
		return false, runtimectx.ErrNoRunningRound
	}
	if _, err := s.runtime.SendContentToRunningRound(ctx, sessionKey, content); err != nil {
		return false, err
	}
	if err := s.recordRoundMarker(agentValue.WorkspacePath, sessionItem, request.RoundID, content, protocol.ChatDeliveryPolicyQueue); err != nil {
		s.loggerFor(ctx).Error("DM 排队消息持久化失败",
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return false, err
	}
	if _, err := s.refreshSessionMetaAfterRoundMarker(agentValue.WorkspacePath, sessionItem); err != nil {
		s.loggerFor(ctx).Error("DM 排队消息刷新 session meta 失败",
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return false, err
	}
	s.scheduleTitleGeneration(ctx, protocol.ParseSessionKey(sessionKey), sessionItem, content, initialMessageCount)
	s.permission.BroadcastEvent(ctx, sessionKey, protocol.NewChatAckEvent(sessionKey, dmdomain.FirstNonEmpty(request.ReqID, request.RoundID), request.RoundID, []map[string]any{}))
	if request.BroadcastUserMessage {
		s.broadcastUserRoundMarker(ctx, sessionItem, request.RoundID, content, protocol.ChatDeliveryPolicyQueue)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	s.loggerFor(ctx).Info("排队 DM 消息到运行中 round",
		"session_key", sessionKey,
		"agent_id", agentValue.AgentID,
		"round_id", request.RoundID,
		"running_round_ids", runningRoundIDs,
	)
	return true, nil
}

func (s *Service) guideRunningInput(
	ctx context.Context,
	sessionKey string,
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
	request Request,
) (bool, error) {
	content := strings.TrimSpace(request.Content)
	runningRoundIDs, err := s.runtime.QueueGuidanceInput(ctx, sessionKey, request.RoundID, content)
	if err != nil {
		return false, err
	}
	s.permission.BroadcastEvent(ctx, sessionKey, protocol.NewChatAckEvent(sessionKey, dmdomain.FirstNonEmpty(request.ReqID, request.RoundID), request.RoundID, []map[string]any{}))
	if request.BroadcastUserMessage {
		for _, targetRoundID := range runningRoundIDs {
			s.broadcastGuidanceMessage(ctx, sessionItem, targetRoundID, request.RoundID, content)
		}
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	s.loggerFor(ctx).Info("登记 DM 引导消息等待 PostToolUse 注入",
		"session_key", sessionKey,
		"agent_id", agentValue.AgentID,
		"round_id", request.RoundID,
		"running_round_ids", runningRoundIDs,
	)
	return true, nil
}

func (s *Service) scheduleTitleGeneration(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionItem protocol.Session,
	content string,
	initialMessageCount int,
) {
	if s.titles == nil {
		return
	}
	conversationID := strings.TrimSpace(dmdomain.StringPointerValue(sessionItem.ConversationID))
	if conversationID == "" && parsed.ChatType == "dm" {
		conversationID = strings.TrimSpace(parsed.Ref)
	}
	roomID := strings.TrimSpace(dmdomain.StringPointerValue(sessionItem.RoomID))
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
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
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
	options, err := clientopts.BuildAgentClientOptions(ctx, s.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:      agentValue.WorkspacePath,
		Provider:           agentValue.Options.Provider,
		PermissionMode:     permissionMode,
		PermissionHandler:  permissionHandler,
		AllowedTools:       agentValue.Options.AllowedTools,
		DisallowedTools:    agentValue.Options.DisallowedTools,
		SettingSources:     agentValue.Options.SettingSources,
		AppendSystemPrompt: appendSystemPrompt,
		ResumeSessionID:    dmdomain.StringPointerValue(sessionItem.SessionID),
		MaxThinkingTokens:  agentValue.Options.MaxThinkingTokens,
		MaxTurns:           agentValue.Options.MaxTurns,
		MCPServers:         mcpServers,
	})
	if err != nil {
		return nil, "", "", err
	}
	options = s.runtime.WithGuidanceHook(options, sessionKey)
	options = s.withInputQueueGuidanceHook(options, sessionKey, workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: agentValue.WorkspacePath,
		SessionKey:    sessionKey,
	}, sessionItem)
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
	sessionItem protocol.Session,
	provider string,
	options agentclient.Options,
) string {
	resumeID := strings.TrimSpace(options.Resume)
	if resumeID == "" {
		return ""
	}
	expectedProvider := strings.TrimSpace(provider)
	expectedModel := strings.TrimSpace(options.Model)
	actualProvider, _ := sessionItem.Options[protocol.OptionRuntimeProvider].(string)
	actualModel, _ := sessionItem.Options[protocol.OptionRuntimeModel].(string)
	if strings.TrimSpace(actualProvider) == expectedProvider && strings.TrimSpace(actualModel) == expectedModel {
		return resumeID
	}
	s.loggerFor(ctx).Warn("DM session runtime 配置已变更，跳过过期 SDK session resume",
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
	sessionItem.Options[protocol.OptionRuntimeProvider] = expectedProvider
	sessionItem.Options[protocol.OptionRuntimeModel] = expectedModel
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
		return "", protocol.SessionKey{}, ErrRoomSessionNotImplemented
	}
	return sessionKey, parsed, nil
}
