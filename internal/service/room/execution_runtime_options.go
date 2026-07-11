package room

import (
	"context"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	"github.com/nexus-research-lab/nexus/internal/service/room/runtimepolicy"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	nexusRoomIDEnvName             = "NEXUS_ROOM_ID"
	nexusRoomConversationIDEnvName = "NEXUS_ROOM_CONVERSATION_ID"
	nexusRoomAgentIDEnvName        = "NEXUS_ROOM_AGENT_ID"
	nexusctlUserIDEnvName          = "NEXUSCTL_USER_ID"
)

type preparedSlotRuntime struct {
	options   agentclient.Options
	selection runtimeselectionsvc.Selection
	provider  string
}

func (e *slotExecution) prepareRuntimeClient() (runtimectx.Client, error) {
	if err := workspacepkg.EnsureInitialized(
		e.agent.AgentID,
		e.agent.Name,
		e.agent.WorkspacePath,
		e.agent.IsMain,
		e.agent.CreatedAt,
	); err != nil {
		return nil, err
	}

	runtimeValue, err := e.prepareRuntime()
	if err != nil {
		return nil, err
	}
	client, err := e.connectRuntime(&runtimeValue)
	if err != nil {
		return nil, err
	}
	e.logger.Info("Room runtime 启动成功",
		append(roomRuntimeStartupLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot),
			"sdk_session_id", strings.TrimSpace(client.SessionID()),
		)...,
	)
	return client, nil
}

func (e *slotExecution) prepareRuntime() (preparedSlotRuntime, error) {
	prompt, permissionMode, err := e.buildRuntimePrompt()
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	beginGoalUsageForSlot(e.slot)

	selection, err := e.service.resolveAgentRuntimeSelection(e.ctx, e.round, e.agent)
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	options, err := clientopts.BuildAgentClientOptions(e.ctx, e.service.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:              e.agent.WorkspacePath,
		RuntimeKind:                selection.RuntimeKind,
		Provider:                   selection.Provider,
		Model:                      selection.Model,
		PermissionMode:             permissionMode,
		PermissionHandler:          e.runtimePermissionHandler(),
		AllowedTools:               toolpolicy.WithManagedRuntimeAllowedTools(runtimepolicy.AllowedTools(e.agent.Options.AllowedTools, e.round.Context.Room.PrivateMessagesEnabled), e.service.runtimeImagegenDefaultEnabled(e.ctx)),
		DisallowedTools:            runtimepolicy.DisallowedTools(e.agent.Options.DisallowedTools, e.round.Context.Room.PrivateMessagesEnabled),
		SettingSources:             e.agent.Options.SettingSources,
		AppendSystemPrompt:         prompt,
		ResumeSessionID:            e.slot.getSDKSessionID(),
		MaxThinkingTokens:          e.agent.Options.MaxThinkingTokens,
		MaxTurns:                   e.agent.Options.MaxTurns,
		MCPServers:                 e.runtimeMCPServers(),
		ExtraEnv:                   e.service.roomRuntimeEnv(e.round, e.slot),
		AgentSDKDiagnosticsEnabled: selection.AgentSDKDiagnosticsEnabled,
	})
	if err != nil {
		return preparedSlotRuntime{}, err
	}

	e.slot.setRuntimeKind(string(options.Runtime.Kind))
	options = e.applyRuntimeHooks(options)
	runtimeProvider := clientopts.ResolvedRuntimeProvider(selection.Provider, options)
	resumeID, err := e.service.resolveReusableRoomSDKSessionID(
		e.ctx,
		e.logger,
		e.agent.WorkspacePath,
		e.slot,
		options.Session.ResumeID,
	)
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	options.Session.ResumeID = resumeID
	return preparedSlotRuntime{options: options, selection: selection, provider: runtimeProvider}, nil
}

func (e *slotExecution) buildRuntimePrompt() (string, sdkpermission.Mode, error) {
	prompt, err := e.service.agents.BuildRuntimePrompt(e.ctx, e.agent)
	if err != nil {
		return "", "", err
	}
	prompt = appendPromptSection(prompt, roomdomain.BuildSystemPrompt(e.round.Context.Room.PrivateMessagesEnabled))
	roomSkillPrompt, err := e.service.rooms.BuildRoomSkillPrompt(e.ctx, e.round.Context.Room.SkillNames)
	if err != nil {
		return "", "", err
	}
	prompt = appendPromptSection(prompt, roomSkillPrompt)
	prompt = appendPromptSection(prompt, roomdomain.BuildMemberDirectoryPrompt(e.agentNameByID))

	permissionMode := sdkpermission.Mode(e.agent.Options.PermissionMode)
	if e.round.PermissionMode != "" {
		permissionMode = e.round.PermissionMode
	}
	e.slot.GoalRuntimeIgnored = goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(permissionMode))
	if !e.slot.GoalRuntimeIgnored {
		prompt, e.slot.GoalContext, e.slot.GoalIDForUsage, e.slot.GoalSessionKey = e.service.resolveGoalRuntimeContextForSlot(e.ctx, e.round, e.slot, prompt)
	}
	if override := strings.TrimSpace(e.round.GoalContext); e.round.Internal && override != "" {
		e.slot.GoalContext = override
	}
	return prompt, permissionMode, nil
}

func (e *slotExecution) runtimeMCPServers() map[string]sdkmcp.ServerConfig {
	if e.service.mcpServers == nil {
		return nil
	}
	return e.service.mcpServers(
		e.agent.AgentID,
		e.round.SessionKey,
		e.round.RootRoundID,
		"room",
		e.round.RoomID,
		roomSourceContextLabel(e.round),
	)
}

func (e *slotExecution) runtimePermissionHandler() sdkpermission.Handler {
	handler := e.round.PermissionHandler
	if handler == nil {
		handler = func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
			return e.service.permission.RequestPermission(ctx, e.slot.RuntimeSessionKey, request)
		}
	}
	handler = runtimepolicy.PermissionHandler(handler, e.round.Context.Room.PrivateMessagesEnabled)
	handler = toolpolicy.WithManagedGoalAutoApproval(handler)
	return toolpolicy.WithMalformedInputDeny(handler)
}

func (e *slotExecution) applyRuntimeHooks(options agentclient.Options) agentclient.Options {
	options = e.service.runtime.WithGuidanceHook(options, e.slot.RuntimeSessionKey)
	if goalSessionKey := goalSessionKeyForSlot(e.slot); goalSessionKey != "" && goalSessionKey != e.slot.RuntimeSessionKey {
		options = e.service.runtime.WithGuidanceHook(options, goalSessionKey)
	}
	options = runtimectx.WithPostToolUseGuidanceHook(options, e.service.roomSlotGuidanceHook(e.round, e.slot, workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  e.agent.WorkspacePath,
		SessionKey:     e.slot.RuntimeSessionKey,
		RoomID:         e.round.RoomID,
		ConversationID: e.round.ConversationID,
	}))
	return withRoomRuntimeDiagnosticsLogger(options, e.logger.With("agent_id", e.slot.AgentID, "agent_round_id", e.slot.AgentRoundID))
}

func (e *slotExecution) connectRuntime(runtimeValue *preparedSlotRuntime) (runtimectx.Client, error) {
	client, err := e.connectRuntimeOnce(*runtimeValue)
	if err != nil && strings.TrimSpace(runtimeValue.options.Session.ResumeID) != "" && runtimectx.IsRuntimeTransportClosedError(err) {
		e.logger.Warn("Room SDK session resume 失效，清除后重试",
			append(roomRuntimeConnectFailureLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot, err),
				"sdk_session_id", strings.TrimSpace(runtimeValue.options.Session.ResumeID),
			)...,
		)
		if closeErr := e.service.runtime.CloseSession(context.Background(), e.slot.RuntimeSessionKey); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
			return nil, closeErr
		}
		if clearErr := e.service.clearSlotSDKSessionID(e.ctx, e.slot); clearErr != nil {
			return nil, clearErr
		}
		runtimeValue.options.Session.ResumeID = ""
		client, err = e.connectRuntimeOnce(*runtimeValue)
	}
	if err == nil {
		return client, nil
	}
	if closeErr := e.service.runtime.CloseSession(context.Background(), e.slot.RuntimeSessionKey); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
		e.logger.Warn("清理启动失败的 Room runtime 返回错误", "err", closeErr)
	}
	e.logger.Error("Room runtime 启动失败", roomRuntimeConnectFailureLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot, err)...)
	return nil, err
}

func (e *slotExecution) connectRuntimeOnce(runtimeValue preparedSlotRuntime) (runtimectx.Client, error) {
	e.logger.Info("准备启动 Room runtime",
		roomRuntimeStartupLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot)...,
	)
	client, err := e.service.runtime.GetOrCreateWithFactory(
		e.ctx,
		e.slot.RuntimeSessionKey,
		runtimeValue.options,
		e.service.factory,
	)
	if err != nil {
		return nil, err
	}
	e.slot.setRuntimeKind(string(e.service.runtime.RuntimeKind(e.slot.RuntimeSessionKey)))
	e.slot.setClient(client)
	if err = client.Connect(e.ctx); err != nil {
		return nil, err
	}
	return client, nil
}

func (s *RealtimeService) roomRuntimeEnv(roundValue *activeRoomRound, slot *activeRoomSlot) map[string]string {
	if roundValue == nil || slot == nil {
		return nil
	}
	env := map[string]string{
		nexusRoomIDEnvName:             strings.TrimSpace(roundValue.RoomID),
		nexusRoomConversationIDEnvName: strings.TrimSpace(roundValue.ConversationID),
		nexusRoomAgentIDEnvName:        strings.TrimSpace(slot.AgentID),
		nexusctlUserIDEnvName:          strings.TrimSpace(roundValue.OwnerUserID),
	}
	return env
}

type imagegenDefaultResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

func (s *RealtimeService) runtimeImagegenDefaultEnabled(ctx context.Context) bool {
	resolver, ok := s.providers.(imagegenDefaultResolver)
	if !ok || resolver == nil {
		return false
	}
	_, err := resolver.ResolveImageConfig(ctx, "")
	return err == nil
}

func (s *RealtimeService) resolveAgentRuntimeSelection(
	ctx context.Context,
	roundValue *activeRoomRound,
	agentValue *protocol.Agent,
) (runtimeselectionsvc.Selection, error) {
	ownerUserIDs := []string(nil)
	if roundValue != nil {
		ownerUserIDs = append(ownerUserIDs, roundValue.OwnerUserID)
	}
	return runtimeselectionsvc.NewService(s.prefs).Resolve(ctx, runtimeselectionsvc.Request{
		Agent:        agentValue,
		OwnerUserIDs: ownerUserIDs,
	})
}
