// INPUT: 一次 DM 轮次的 Agent、Session 与 Request（含后台 Connector 预备）。
// OUTPUT: 已连接或已 fork 的 runtime client，以及本轮配置快照 dmClientPreparation。
// POS: ensureClient 的分阶段实现：会话 → 权限 → 提示词 → 权威 → 工具 → 选项 → 连接；阶段之间只经 dmClientStart 传值。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// dmClientStart 是一次 runtime client 启动的工作台：各阶段按顺序填充，后一阶段只读前面阶段的结果。
type dmClientStart struct {
	s          *Service
	ctx        context.Context
	sessionKey string
	agent      *protocol.Agent
	session    protocol.Session
	request    Request
	startup    *runtimectx.ClientStartup

	// 会话：受限 Session policy 与用户发起的 conversation fork 边界。
	scopedPolicy        protocol.ScopedSessionRuntimePolicy
	scopedPolicyActive  bool
	forkSourceSessionID string
	forkMessageID       string

	// 权限与提示词。
	permissionMode     sdkpermission.Mode
	permissionHandler  sdkpermission.Handler
	skillNames         []string
	disabledSkillNames []string
	dynamicPrompt      string

	// 权威：Goal/Execution/责任绑定与 nexus.command 作用域，同轮共享。
	goalContext              string
	goalIDForUsage           string
	goalAuthority            *runtimectx.GoalAuthorityState
	responsibility           *runtimectx.ResponsibilityAuthorityState
	sdkSessionIdentity       *runtimectx.SDKSessionIdentityState
	commandReceipts          *nexusmcp.CommandReceiptState
	subagentCommandAuthority *bool
	sourceContextType        string
	builderContext           context.Context
	commandContext           runtimectx.RuntimeCommandContext

	// 工具：Connector 选择、MCP server 与配置环境。
	configurationEnv    map[string]string
	enabledConnectorIDs []string
	connectorContext    string
	mcpServers          map[string]sdkmcp.ServerConfig

	// 选项：本轮 runtime 选择与工具面指纹。
	selection              runtimeselectionsvc.Selection
	options                agentclient.Options
	runtimeProvider        string
	toolSurfaceFingerprint string
	toolSurfaceComplete    bool
}

func (s *Service) ensureClient(
	ctx context.Context,
	sessionKey string,
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
	request Request,
) (dmClientPreparation, error) {
	if !request.runtimePreparationOnly {
		s.cancelConnectorRuntimePreparation(agentValue.OwnerUserID, sessionKey)
	}
	start := &dmClientStart{
		s:                   s,
		ctx:                 ctx,
		sessionKey:          sessionKey,
		agent:               agentValue,
		session:             sessionItem,
		request:             request,
		forkSourceSessionID: strings.TrimSpace(request.forkSourceSessionID),
		forkMessageID:       strings.TrimSpace(request.forkMessageID),
	}
	if err := start.requireForkPair(); err != nil {
		return dmClientPreparation{}, err
	}
	startup, err := s.Runtime.BeginClientStartup(ctx, sessionKey, agentValue.OwnerUserID)
	if err != nil {
		return dmClientPreparation{}, err
	}
	defer startup.Close()
	start.startup = startup
	for _, phase := range []func() error{
		start.loadSession,
		start.resolvePermission,
		start.resolvePrompts,
		start.resolveAuthority,
		start.resolveTools,
		start.buildOptions,
	} {
		if err = phase(); err != nil {
			return dmClientPreparation{}, err
		}
	}
	return start.connect()
}

func (c *dmClientStart) requireForkPair() error {
	if (c.forkSourceSessionID == "") != (c.forkMessageID == "") {
		return errors.New("fork source session id and message id must be provided together")
	}
	return nil
}

func (c *dmClientStart) forking() bool { return c.forkSourceSessionID != "" }

// loadSession 读取最新 Session 并确定受限 policy；后台预备在 Connector 选择或配置版本变化时让位。
func (c *dmClientStart) loadSession() error {
	if c.request.runtimePreparationOnly {
		current, err := c.s.ensureSession(c.ctx, c.agent, protocol.ParseSessionKey(c.sessionKey), c.sessionKey)
		if err != nil {
			return err
		}
		c.session = current
	} else {
		latest, _, err := c.s.Files.ForOwner(c.agent.OwnerUserID).FindSession([]string{c.agent.WorkspacePath}, c.sessionKey)
		if err != nil {
			return err
		}
		if latest != nil {
			c.session = *latest
		}
	}
	if c.s.scopedSessionPolicy != nil {
		var err error
		c.scopedPolicy, c.scopedPolicyActive, err = c.s.scopedSessionPolicy.RuntimeEditorPolicy(c.ctx, c.agent.OwnerUserID, c.sessionKey)
		if err != nil {
			return err
		}
	}
	if !c.scopedPolicyActive && protocol.SessionPurpose(c.session) == protocol.SessionPurposeWorkGraphDistillation {
		c.scopedPolicy = workGraphDistillationRuntimePolicy()
		c.scopedPolicyActive = true
	}
	if err := c.requirePreparationCurrent(c.session, "Connector selection changed during startup"); err != nil {
		return err
	}
	if c.forkSourceSessionID == "" && c.forkMessageID == "" {
		c.forkSourceSessionID, c.forkMessageID = pendingConversationFork(c.session.Options)
	}
	return c.requireForkPair()
}

// requirePreparationCurrent 只约束后台预备：期间配置版本或 Connector 选择变化，说明预备结果已被取代。
func (c *dmClientStart) requirePreparationCurrent(session protocol.Session, selectionChanged string) error {
	if !c.request.runtimePreparationOnly {
		return nil
	}
	if session.ConfigurationVersion != c.request.expectedConfigurationVersion {
		return fmt.Errorf(
			"%w: expected configuration_version=%d actual=%d",
			errConnectorPreparationSuperseded,
			c.request.expectedConfigurationVersion,
			session.ConfigurationVersion,
		)
	}
	if c.request.expectedConnectorSelection != nil &&
		!protocol.SessionConnectorSelectionFromOptions(session.Options).Equal(*c.request.expectedConnectorSelection) {
		return fmt.Errorf("%w: %s", errConnectorPreparationSuperseded, selectionChanged)
	}
	return nil
}

func (c *dmClientStart) resolvePermission() error {
	sessionSettings := protocol.SessionRuntimeSettingsFromOptions(c.session.Options)
	c.permissionMode = resolvePermissionMode(
		c.request.PermissionMode,
		sessionSettings.PermissionMode,
		c.agent.Options.PermissionMode,
	)
	handler := c.request.PermissionHandler
	if handler == nil {
		sessionKey := c.sessionKey
		handler = func(permissionCtx context.Context, permissionRequest sdkpermission.Request) (sdkpermission.Decision, error) {
			return c.s.Permission.RequestPermission(permissionCtx, sessionKey, permissionRequest)
		}
	}
	handler = toolpolicy.WithManagedRuntimeAutoApproval(handler)
	c.permissionHandler = toolpolicy.WithMalformedInputDeny(handler)
	return nil
}

func (c *dmClientStart) resolvePrompts() error {
	var err error
	if !c.scopedPolicyActive || !c.scopedPolicy.DisableSkills {
		c.skillNames, c.disabledSkillNames, err = c.s.AgentRuntimeSkills(*c.agent, c.scopedPolicy.AllowedSkillNames)
		if err != nil {
			return err
		}
	}
	c.dynamicPrompt, err = c.s.Agents.BuildRuntimePrompt(c.ctx, c.agent)
	if err != nil {
		return err
	}
	if c.scopedPolicyActive {
		c.dynamicPrompt = runtimehost.JoinPromptSections(c.dynamicPrompt, c.scopedPolicy.SystemPrompt)
	}
	return nil
}

// resolveAuthority 签发本轮 Goal/Execution/责任权威与 command 作用域；调度绑定的轮次先向 WorkGraph 取回 Goal。
func (c *dmClientStart) resolveAuthority() error {
	request := &c.request
	if request.queuedWorkBinding != nil || request.queuedReviewBinding != nil {
		provider, ok := c.s.ExecutionContext.(interface {
			PrepareScheduledDMRound(context.Context, orchestration.ActorContext) (*protocol.Execution, error)
		})
		if !ok {
			return errors.New("scheduled DM authority unavailable")
		}
		boundExecution, err := provider.PrepareScheduledDMRound(c.ctx, orchestration.ActorContext{OwnerUserID: c.agent.OwnerUserID, SessionKey: c.sessionKey, AgentID: c.agent.AgentID, ExecutionID: request.ExecutionID, ScopeKind: protocol.ExecutionScopeDM, Role: orchestration.ExecutionActorMember, WorkBinding: request.queuedWorkBinding, ReviewBinding: request.queuedReviewBinding, RootRoundID: request.RoundID, RuntimeRoundID: request.RoundID, AgentRoundID: request.AgentRoundID})
		if err != nil {
			return err
		}
		request.GoalID, request.GoalObjectiveRevision = boundExecution.GoalID, boundExecution.GoalObjectiveRevision
	}
	objectiveRevision := int64(0)
	explicitGoalID := strings.TrimSpace(request.GoalID)
	executionID := request.ExecutionID
	explicitGoalRevision := request.GoalObjectiveRevision
	goalBoundRequest := request.Internal && explicitGoalID != "" && explicitGoalRevision > 0
	if !goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(c.permissionMode)) && goalBoundRequest {
		c.goalContext, c.goalIDForUsage, objectiveRevision = c.s.goalRuntimeContext(c.ctx, c.sessionKey)
		if c.goalIDForUsage != explicitGoalID || objectiveRevision != explicitGoalRevision {
			return goalsvc.ErrGoalRevisionStale
		}
		c.goalIDForUsage = explicitGoalID
		objectiveRevision = explicitGoalRevision
	}
	c.goalAuthority = runtimectx.NewGoalAuthorityState(c.goalIDForUsage, objectiveRevision, executionID)
	c.responsibility = runtimectx.NewResponsibilityAuthorityState(
		c.goalAuthority,
		executionID,
		request.queuedWorkBinding,
		request.queuedReviewBinding,
	)
	c.sdkSessionIdentity = runtimectx.NewSDKSessionIdentityState(textutil.PointerValue(c.session.SessionID))
	c.commandReceipts = nexusmcp.NewCommandReceiptState()
	if err := c.resolveSourceContext(); err != nil {
		return err
	}
	commandScopeSessionKey := c.sessionKey
	workGraphPreviewID := ""
	if c.sourceContextType == protocol.SessionPurposeWorkGraphDistillation {
		commandScopeSessionKey = strings.TrimSpace(request.WorkGraphSaveSourceSessionKey)
		workGraphPreviewID = strings.TrimSpace(request.InputOptions.Metadata["preview_id"])
		if commandScopeSessionKey == "" || workGraphPreviewID == "" {
			return errors.New("WorkGraph distillation round is missing its exact source or preview binding")
		}
	}
	c.builderContext = runtimectx.WithRuntimeRoundLease(c.ctx, c.sessionKey, request.RoundID)
	c.subagentCommandAuthority = new(bool)
	c.commandContext = runtimectx.RuntimeCommandContext{
		Agent: c.agent, ScopeSessionKey: commandScopeSessionKey, RuntimeSessionKey: c.sessionKey,
		ExecutionID:        executionID,
		CoordinatorAgentID: c.agent.AgentID,
		RootRoundID:        request.RoundID, AgentRoundID: request.AgentRoundID,
		SourceContextType: c.sourceContextType, SourceContextID: c.agent.AgentID,
		SourceContextLabel: c.agent.Name, PermissionMode: c.permissionMode,
		GoalAuthority: c.goalAuthority, ResponsibilityAuthority: c.responsibility,
		GoalContinuationAuthority: request.goalContinuationAuthority,
		SDKSessionIdentity:        c.sdkSessionIdentity,
		AutomationRun:             request.AutomationRun.NormalizedCopy(),
		WorkGraphPreviewID:        workGraphPreviewID,
		SubagentCommandAuthority:  c.subagentCommandAuthority,
	}
	return nil
}

// resolveSourceContext 决定 MCP 来源上下文：Goal 续跑与受限 Session 各自覆盖普通来源，二者互斥。
func (c *dmClientStart) resolveSourceContext() error {
	c.sourceContextType = dmMCPSourceContextType(c.sessionKey, c.agent.AgentID, c.request)
	if c.request.goalContinuationAuthority != nil {
		if !trustedDMGoalContinuationAuthority(c.request.goalContinuationAuthority, c.agent, c.sessionKey, c.request) {
			return errors.New("DM Goal continuation authority does not match the exact owner, Agent, session, Goal, revision, Execution, or round")
		}
		c.sourceContextType = runtimectx.SourceContextGoalContinuation
	}
	if c.scopedPolicyActive {
		if c.request.goalContinuationAuthority != nil {
			return errors.New("DM Goal continuation cannot use a scoped WorkGraph policy session")
		}
		c.sourceContextType = protocol.SessionPurpose(c.session)
	}
	return nil
}

// resolveTools 装配配置环境、Connector 与 MCP server；受限 Session 只保留 WorkGraph 需要的 nexus 工具。
func (c *dmClientStart) resolveTools() error {
	var err error
	if !c.request.runtimePreparationOnly && !c.scopedPolicyActive && c.s.ConfigurationRuntimeEnv != nil {
		c.configurationEnv, err = c.s.ConfigurationRuntimeEnv(
			c.builderContext,
			c.agent,
			c.sessionKey,
			c.request.RoundID,
			c.sourceContextType,
			c.agent.AgentID,
		)
		if err != nil {
			return err
		}
	}
	c.permissionHandler = toolpolicy.WithNexusControlPlaneDeny(c.permissionHandler, !c.agent.IsMain)
	c.enabledConnectorIDs = protocol.EffectiveSessionConnectorIDs(c.agent.Options.ConnectorIDs, c.session.Options)
	if c.scopedPolicyActive && c.scopedPolicy.DisableConnectors {
		c.enabledConnectorIDs = []string{}
	}
	// Connector 状态随配置与选择变化，不进 system prompt；由本轮上下文在变化时追加（见 connectorContextualInputs）。
	c.connectorContext = c.s.connectorRuntimeStatePrompt(c.ctx, c.agent.OwnerUserID, c.enabledConnectorIDs)
	mcpContext := runtimectx.WithEnabledConnectorIDs(c.builderContext, c.enabledConnectorIDs)
	mcpContext = runtimectx.WithGoalAuthorityState(mcpContext, c.goalAuthority)
	mcpContext = runtimectx.WithResponsibilityAuthorityState(mcpContext, c.responsibility)
	if c.s.MCPServers != nil && !c.scopedPolicyActive {
		c.mcpServers = c.s.MCPServers(
			mcpContext,
			c.agent,
			c.sessionKey,
			c.request.RoundID,
			c.sourceContextType,
			c.agent.AgentID,
			c.agent.Name,
			c.goalAuthority.ObjectiveRevisionState(),
			c.permissionMode,
		)
	}
	if (!c.scopedPolicyActive || c.sourceContextType == protocol.SessionPurposeWorkGraphEditor ||
		c.sourceContextType == protocol.SessionPurposeWorkGraphDistillation) &&
		c.s.NexusMCP != nil {
		c.mcpServers, err = c.s.MergeNexusMCPServers(mcpContext, c.mcpServers, nexusmcp.RoundContext{
			SessionKey: c.sessionKey, RoundID: c.request.RoundID, InputContent: c.request.Content,
			SubagentControl:   c.s.Runtime.BindSubagentControl(c.sessionKey, c.request.RoundID),
			SourceContextType: c.sourceContextType, SourceContextID: c.agent.AgentID,
			SourceContextLabel: c.agent.Name,
			CommandContext:     c.commandContext, CommandReceipts: c.commandReceipts,
		})
		if err != nil {
			return err
		}
	}
	c.connectorContext = runtimehost.JoinPromptSections(c.connectorContext, connectorRuntimeToolPrompt(c.enabledConnectorIDs, c.mcpServers))
	return nil
}

// buildOptions 解析 runtime 选择并生成本轮 Options；fork 沿用源 Session 的 runtime 指纹。
func (c *dmClientStart) buildOptions() error {
	var err error
	c.selection, err = c.s.ResolveAgentRuntimeSelection(c.ctx, c.agent, c.session.Options)
	if err != nil {
		return err
	}
	if c.forking() {
		fingerprint := runtimeFingerprintFromSession(c.session)
		c.selection.RuntimeKind = textutil.FirstNonEmpty(fingerprint.kind, c.selection.RuntimeKind)
		c.selection.Provider = textutil.FirstNonEmpty(fingerprint.provider, c.selection.Provider)
		c.selection.Model = textutil.FirstNonEmpty(fingerprint.model, c.selection.Model)
	}
	if err = c.s.Agents.EnsureRuntimeVisionSettingsProjection(*c.agent, c.selection.VisionProvider, c.selection.VisionModel); err != nil {
		return err
	}
	toolPolicy := c.request.RuntimeToolPolicy
	if c.scopedPolicyActive {
		toolPolicy = &c.scopedPolicy.ToolPolicy
	}
	allowedTools, disallowedTools := resolveDMRuntimeToolPolicy(c.agent.Options, toolPolicy, c.s.RuntimeImagegenDefaultEnabled(c.ctx))
	c.options, _, err = c.s.BuildAgentRuntimeOptions(c.ctx, runtimehost.AgentRuntimeOptionsInput{
		Agent:                 c.agent,
		Selection:             c.selection,
		PermissionMode:        c.permissionMode,
		PermissionHandler:     c.permissionHandler,
		AllowedTools:          allowedTools,
		DisallowedTools:       disallowedTools,
		SkillIDs:              c.skillNames,
		DisabledSkillIDs:      c.disabledSkillNames,
		AdditionalDirectories: scopedSessionAdditionalDirectories(c.session, c.scopedPolicyActive),
		StablePrompt:          orchestration.StablePrompt(),
		DynamicPrompt:         c.dynamicPrompt,
		ResumeSessionID:       textutil.FirstNonEmpty(c.forkSourceSessionID, textutil.PointerValue(c.session.SessionID)),
		MCPServers:            c.mcpServers,
		ConfigurationEnv:      c.configurationEnv,
	})
	if err != nil {
		return err
	}
	c.options = c.s.WithAgentRuntimeHooks(c.options, c.sessionKey,
		c.s.LoggerFor(context.Background()).With("session_key", c.sessionKey, "agent_id", c.agent.AgentID),
		c.withGuidanceHook,
	)
	c.runtimeProvider = clientopts.ResolvedRuntimeProvider(c.selection.Provider, c.options)
	c.toolSurfaceFingerprint, c.toolSurfaceComplete, err = runtimectx.ModelToolSurfaceFingerprint(c.ctx, c.options)
	if err != nil {
		return fmt.Errorf("计算 DM runtime 工具面指纹: %w", err)
	}
	if strings.TrimSpace(c.options.Session.ResumeID) != "" && !c.toolSurfaceComplete && c.requiresToolSurfaceFork() {
		return errors.New("runtime 工具面检查不完整，无法安全 fork 旧 SDK session")
	}
	return nil
}

// withGuidanceHook 给运行中输入挂 PostToolUse 引导：主窗口取 inbox，其余 DM 取 JSONL 待发送队列。
func (c *dmClientStart) withGuidanceHook(options agentclient.Options) agentclient.Options {
	if target := mainInboxTargetOf(c.agent, c.sessionKey); target != nil {
		return runtimectx.WithPostToolUseGuidanceHook(options, c.s.mainInboxGuidanceHook(*target))
	}
	return c.s.withInputQueueGuidanceHook(options, c.sessionKey, workspacestore.InputQueueLocation{
		OwnerUserID:   c.agent.OwnerUserID,
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: c.agent.WorkspacePath,
		SessionKey:    c.sessionKey,
	})
}

func (c *dmClientStart) requiresToolSurfaceFork() bool {
	storedToolSurface, _ := c.session.Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
	return sessionresumesvc.RequiresToolSurfaceFork(storedToolSurface, c.toolSurfaceFingerprint, len(c.enabledConnectorIDs) > 0)
}

// connect 决定续接/fork 并连接 runtime。后台预备只在工具面需要 fork 时才真正启动，否则直接返回空快照。
func (c *dmClientStart) connect() (dmClientPreparation, error) {
	if c.request.runtimePreparationOnly && !c.requiresToolSurfaceFork() {
		return dmClientPreparation{}, nil
	}
	resumeID, toolSurfaceFork := c.s.resolveReusableSDKSessionID(
		c.ctx,
		c.agent.WorkspacePath,
		c.session,
		c.runtimeProvider,
		c.options,
		c.toolSurfaceFingerprint,
		len(c.enabledConnectorIDs) > 0,
	)
	conversationFork := c.forking()
	if conversationFork && resumeID == "" {
		return dmClientPreparation{}, errors.New("fork source SDK session is unavailable")
	}
	if c.request.runtimePreparationOnly {
		if !toolSurfaceFork {
			return dmClientPreparation{}, nil
		}
		latest, err := c.s.ensureSession(c.ctx, c.agent, protocol.ParseSessionKey(c.sessionKey), c.sessionKey)
		if err != nil {
			return dmClientPreparation{}, err
		}
		if err = c.requirePreparationCurrent(latest, "Connector selection or configuration version changed before fork"); err != nil {
			return dmClientPreparation{}, err
		}
		c.session = latest
	}
	c.options.Session.ResumeID = resumeID
	c.options.Session.ResumeAt = conversationForkResumeAt(c.session.Options, c.forkMessageID)
	c.options.Session.Fork = conversationFork || toolSurfaceFork
	if toolSurfaceFork {
		retired, retireErr := runtimehost.RetireExistingRuntimeClient(c.ctx, c.startup)
		if retireErr != nil && !runtimectx.IsRuntimeTransportClosedError(retireErr) {
			return dmClientPreparation{}, fmt.Errorf("换代 runtime 工具面: %w", retireErr)
		}
		c.s.LoggerFor(c.ctx).Info("Session 工具面变化，从旧 transcript fork 新 SDK session",
			"session_key", c.sessionKey,
			"retired_warm_client", retired,
		)
	}
	c.s.LoggerFor(c.ctx).Info("准备启动 DM runtime",
		append(clientopts.RuntimeStartupLogFields(c.options),
			"session_key", c.sessionKey,
			"agent_id", c.agent.AgentID,
			"requested_runtime_kind", c.selection.RuntimeKind,
			"requested_provider", strings.TrimSpace(c.selection.Provider),
			"requested_model", strings.TrimSpace(c.selection.Model),
			"runtime_provider", c.runtimeProvider,
		)...,
	)
	clearResume := func() error {
		if _, err := c.s.clearReusableSDKSessionID(c.ctx, c.agent.WorkspacePath, c.session); err != nil {
			return err
		}
		c.sdkSessionIdentity.Set("")
		return nil
	}
	if conversationFork {
		// 用户发起的 fork 不能静默退化为全新 SDK session。
		clearResume = nil
	}
	client, err := runtimehost.RuntimeConnect{
		Startup: c.startup,
		Options: &c.options,
		Logger:  c.s.LoggerFor(c.ctx).With("session_key", c.sessionKey, "agent_id", c.agent.AgentID),
		Attempt: func(startup *runtimectx.ClientStartup, options agentclient.Options) (runtimectx.Client, error) {
			return c.s.acquireRuntimeClient(c.ctx, startup, options)
		},
		ClearResume: clearResume,
	}.Run(c.ctx)
	if err != nil {
		return dmClientPreparation{}, err
	}
	forkSourceSessionID := ""
	if c.options.Session.Fork {
		var settled bool
		forkSourceSessionID, settled, err = c.commitFork(client, resumeID)
		if err != nil || !settled {
			return dmClientPreparation{}, err
		}
	}
	if currentSessionID := strings.TrimSpace(client.SessionID()); currentSessionID != "" {
		c.sdkSessionIdentity.Set(currentSessionID)
	} else if forkSourceSessionID != "" {
		c.sdkSessionIdentity.Set("")
	} else {
		c.sdkSessionIdentity.Set(textutil.PointerValue(c.session.SessionID))
	}
	return dmClientPreparation{
		client:                   client,
		runtimeKind:              strings.TrimSpace(string(c.options.Runtime.Kind)),
		runtimeProvider:          c.runtimeProvider,
		runtimeModel:             strings.TrimSpace(c.options.Model),
		toolSurfaceFingerprint:   c.toolSurfaceFingerprint,
		forkSourceSessionID:      forkSourceSessionID,
		session:                  c.session,
		emotionEnabled:           c.selection.EmotionEnabled,
		goalIDForUsage:           c.goalIDForUsage,
		goalContext:              c.goalContext,
		goalObjectiveRevision:    c.goalAuthority.ObjectiveRevisionState(),
		responsibilityState:      c.responsibility,
		sdkSessionIdentity:       c.sdkSessionIdentity,
		commandReceipts:          c.commandReceipts,
		subagentCommandAuthority: c.subagentCommandAuthority,
		permissionMode:           c.permissionMode,
		connectorContext:         c.connectorContext,
	}, nil
}

// commitFork 提交 fork 出的 SDK session，返回仍需沿用的源 session（新 identity 尚未提交时）。
// settled=false 表示后台预备遇到尚未公布 fork identity 的 runtime：已退役 client，留给下一轮同步 fork。
func (c *dmClientStart) commitFork(client runtimectx.Client, resumeID string) (string, bool, error) {
	forkedSessionID := strings.TrimSpace(client.SessionID())
	if forkedSessionID == "" {
		if c.request.runtimePreparationOnly {
			_, _ = runtimehost.RetireRuntimeClient(c.startup)
			// Claude Code 等 runtime 直到真实 query 才公布 fork identity；后台预备
			// 不发送隐藏消息，保持下一轮同步 fork 作为正确性边界。
			return "", false, nil
		}
		// Claude Code 只在首条 query 后通过 init 事件公布 fork identity；
		// 在 round 收到该事件前保持旧 identity/工具面基线不变。
		return resumeID, true, nil
	}
	syncArguments := []sdkSessionSyncConstraint(nil)
	if c.request.runtimePreparationOnly {
		syncArguments = append(syncArguments, sdkSessionSyncConstraint{
			configurationVersion: c.request.expectedConfigurationVersion,
			connectorSelection:   c.request.expectedConnectorSelection,
		})
	}
	updatedSession, err := c.s.syncSDKSessionIDForOwner(
		c.ctx,
		c.agent.OwnerUserID,
		c.agent.WorkspacePath,
		c.session,
		forkedSessionID,
		strings.TrimSpace(string(c.options.Runtime.Kind)),
		c.runtimeProvider,
		strings.TrimSpace(c.options.Model),
		c.toolSurfaceFingerprint,
		syncArguments...,
	)
	if err != nil {
		_, _ = runtimehost.RetireRuntimeClient(c.startup)
		return "", false, fmt.Errorf("提交 fork SDK session: %w", err)
	}
	c.session = updatedSession
	if !forkSessionStateCommitted(c.session, forkedSessionID, c.toolSurfaceFingerprint) {
		return resumeID, true, nil
	}
	return "", true, nil
}
