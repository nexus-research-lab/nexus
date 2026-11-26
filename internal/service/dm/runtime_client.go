// INPUT: DM session、Request 与 Agent runtime 配置。
// OUTPUT: runtime client 启动用到的判定规则：Goal 续跑/外部来源信任、受限 Session policy、权限与工具策略、SDK session 续接/工具面 fork 复用、sandbox lease 失败回收。
// POS: ensureClient（见 runtime_client_start.go）各阶段共用的纯规则与 Service 辅助；阶段编排不在这里。
package dm

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// dmClientPreparation 收拢 runtime client 启动后的配置快照，避免扩展返回值参数组。
type dmClientPreparation struct {
	client                   runtimectx.Client
	runtimeKind              string
	runtimeProvider          string
	runtimeModel             string
	toolSurfaceFingerprint   string
	forkSourceSessionID      string
	session                  protocol.Session
	emotionEnabled           bool
	goalIDForUsage           string
	goalContext              string
	goalObjectiveRevision    *atomic.Int64
	responsibilityState      *runtimectx.ResponsibilityAuthorityState
	sdkSessionIdentity       *runtimectx.SDKSessionIdentityState
	commandReceipts          *nexusmcp.CommandReceiptState
	subagentCommandAuthority *bool
	permissionMode           sdkpermission.Mode
	connectorContext         string
}

// trustedDMGoalContinuationAuthority verifies the host-only capability before
// it reaches the shared MCP builder. Request fields are checked as well: a
// continuation plan must not be able to bind a different Goal or round after
// the durable claim has succeeded.
func trustedDMGoalContinuationAuthority(
	authority *runtimectx.GoalContinuationAuthority,
	agentValue *protocol.Agent,
	sessionKey string,
	request Request,
) bool {
	if authority == nil || agentValue == nil || !authority.Valid() || !request.Internal ||
		!isDMGoalContinuationPurpose(request.InputOptions.Purpose) {
		return false
	}
	normalized := authority.Normalized()
	parsed := protocol.ParseSessionKey(sessionKey)
	return normalized.OwnerUserID == strings.TrimSpace(agentValue.OwnerUserID) &&
		normalized.AgentID == strings.TrimSpace(agentValue.AgentID) &&
		normalized.ScopeSessionKey == strings.TrimSpace(sessionKey) &&
		normalized.GoalID == strings.TrimSpace(request.GoalID) &&
		normalized.ObjectiveRevision == request.GoalObjectiveRevision &&
		normalized.ExecutionID == request.ExecutionID &&
		normalized.RootRoundID == strings.TrimSpace(request.RoundID) &&
		parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.ChatType == protocol.RoomTypeDM &&
		parsed.Channel == protocol.SessionChannelWebSocketSegment &&
		strings.TrimSpace(parsed.AgentID) == normalized.AgentID
}

func isDMGoalContinuationPurpose(value string) bool {
	switch strings.TrimSpace(value) {
	case "goal_continuation", "goal_objective_transition_planning":
		return true
	default:
		return false
	}
}

func workGraphDistillationRuntimePolicy() protocol.ScopedSessionRuntimePolicy {
	return protocol.ScopedSessionRuntimePolicy{
		SystemPrompt: "当前是隔离的内部 WorkGraph 保存 Session。只按用户确认的 exact preview_id 调用 distill_workgraph，不读取 workspace、不执行草图任务，也不处理其他请求。",
		ToolPolicy: protocol.RuntimeToolPolicy{
			AllowedTools: []string{"Skill", "mcp__nexus__command"},
			DisallowedTools: []string{
				"Agent", "Edit", "Glob", "Grep", "Task", "WebFetch", "WebSearch",
				"mcp__nexus__show_widget", "mcp__nexus__generate_image", "mcp__nexus__edit_image",
			},
		},
		AllowedSkillNames: []string{"execution-orchestrator"},
		DisableConnectors: true,
	}
}

func conversationForkResumeAt(options map[string]any, forkMessageID string) string {
	if atTail, _ := options[protocol.OptionRuntimeForkAtTranscriptTail].(bool); atTail {
		return ""
	}
	return strings.TrimSpace(forkMessageID)
}

func forkSessionStateCommitted(
	sessionItem protocol.Session,
	sessionID string,
	toolSurfaceFingerprint string,
) bool {
	currentSessionID := textutil.PointerValue(sessionItem.SessionID)
	storedToolSurface, _ := sessionItem.Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
	return currentSessionID == strings.TrimSpace(sessionID) &&
		strings.TrimSpace(storedToolSurface) == strings.TrimSpace(toolSurfaceFingerprint)
}

func dmMCPSourceContextType(sessionKey string, agentID string, request Request) string {
	executionOrigin := request.ExecutionOrigin
	if request.trustedQueuedConfigurationContext &&
		(executionOrigin == "queue" || executionOrigin == mainInboxExecutionOrigin) &&
		trustedDMWebSocketSession(sessionKey, agentID) {
		return "agent"
	}
	if request.TrustedExternalInteractiveContext &&
		executionOrigin == "channel" &&
		trustedExternalDMSession(sessionKey, agentID) {
		return "agent_paired"
	}
	switch {
	case executionOrigin != "":
		return "agent_" + strings.ToLower(executionOrigin)
	case request.Internal:
		return "agent_internal"
	case request.ExternalReplyTarget != nil:
		return "agent_external"
	case !request.TrustedConfigurationContext:
		return "agent_untrusted"
	}
	if !trustedDMWebSocketSession(sessionKey, agentID) {
		return "agent_untrusted"
	}
	return "agent"
}

func trustedExternalDMSession(sessionKey string, agentID string) bool {
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		parsed.ChatType != protocol.RoomTypeDM ||
		strings.TrimSpace(parsed.AgentID) != strings.TrimSpace(agentID) {
		return false
	}
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	return channel != "" &&
		channel != protocol.SessionChannelWebSocket &&
		channel != protocol.SessionChannelInternalSegment
}

func trustedDMWebSocketSession(sessionKey string, agentID string) bool {
	parsed := protocol.ParseSessionKey(sessionKey)
	return parsed.IsStructured &&
		parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.Channel == protocol.SessionChannelWebSocketSegment &&
		parsed.ChatType == protocol.RoomTypeDM &&
		strings.TrimSpace(parsed.AgentID) == strings.TrimSpace(agentID)
}

func resolvePermissionMode(
	requestMode sdkpermission.Mode,
	sessionMode string,
	agentMode string,
) sdkpermission.Mode {
	if requestMode != "" {
		return runtimepermission.NormalizeMode(requestMode)
	}
	if sessionMode != "" {
		return runtimepermission.NormalizeMode(sdkpermission.Mode(sessionMode))
	}
	if agentMode != "" {
		return runtimepermission.NormalizeMode(sdkpermission.Mode(agentMode))
	}
	return sdkpermission.ModeDefault
}

func resolveDMRuntimeToolPolicy(
	agentOptions protocol.Options,
	snapshot *protocol.RuntimeToolPolicy,
	imagegenDefaultEnabled bool,
) ([]string, []string) {
	if snapshot != nil {
		return append([]string(nil), snapshot.AllowedTools...), append([]string(nil), snapshot.DisallowedTools...)
	}
	return toolpolicy.WithManagedRuntimeAllowedTools(
		agentOptions.AllowedTools,
		imagegenDefaultEnabled,
	), append([]string(nil), agentOptions.DisallowedTools...)
}

func scopedSessionAdditionalDirectories(session protocol.Session, scoped bool) []string {
	if scoped {
		return nil
	}
	return protocol.SessionAdditionalDirectoriesFromOptions(session.Options)
}

func (s *Service) goalRuntimeContext(ctx context.Context, sessionKey string) (string, string, int64) {
	if s.goals == nil {
		return "", "", 0
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if goalsvc.IsAbsent(err) {
			return "", "", 0
		}
		s.LoggerFor(ctx).Warn("读取 Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return "", "", 0
	}
	goalID := ""
	objectiveRevision := int64(0)
	if goal != nil {
		goalID = strings.TrimSpace(goal.ID)
		objectiveRevision = goal.ObjectiveRevision()
	}
	if strings.TrimSpace(goalContext) == "" {
		return "", goalID, objectiveRevision
	}
	return strings.TrimSpace(goalContext), goalID, objectiveRevision
}

func (s *Service) resolveReusableSDKSessionID(
	ctx context.Context,
	workspacePath string,
	sessionItem protocol.Session,
	provider string,
	options agentclient.Options,
	toolSurfaceFingerprint string,
	forkLegacyToolSurface bool,
) (string, bool) {
	resumeID := strings.TrimSpace(options.Session.ResumeID)
	if resumeID == "" {
		return "", false
	}
	expectedKind := strings.TrimSpace(string(options.Runtime.Kind))
	expectedProvider := strings.TrimSpace(provider)
	expectedModel := strings.TrimSpace(options.Model)
	actualKind, hasKindFingerprint := sessionItem.Options[protocol.OptionRuntimeKind].(string)
	actualProvider, hasProviderFingerprint := sessionItem.Options[protocol.OptionRuntimeProvider].(string)
	actualModel, hasModelFingerprint := sessionItem.Options[protocol.OptionRuntimeModel].(string)
	actualToolSurface, _ := sessionItem.Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
	actualKind = strings.TrimSpace(actualKind)
	actualProvider = strings.TrimSpace(actualProvider)
	actualModel = strings.TrimSpace(actualModel)
	actualToolSurface = strings.TrimSpace(actualToolSurface)
	toolSurfaceFingerprint = strings.TrimSpace(toolSurfaceFingerprint)
	hasFingerprint := hasKindFingerprint || hasProviderFingerprint || hasModelFingerprint
	fingerprintMatches := hasFingerprint &&
		(!hasKindFingerprint || actualKind == expectedKind) &&
		(!hasProviderFingerprint || actualProvider == expectedProvider) &&
		(!hasModelFingerprint || actualModel == expectedModel)
	runtimeChanged := hasKindFingerprint && actualKind != expectedKind
	decision := sessionresumesvc.NewPolicy(
		s.History.ForOwner(authctx.OwnerUserID(ctx)),
	).CanResume(workspacePath, resumeID)
	if decision.Allowed {
		if !runtimeChanged && sessionresumesvc.RequiresToolSurfaceFork(
			actualToolSurface,
			toolSurfaceFingerprint,
			forkLegacyToolSurface,
		) {
			s.LoggerFor(ctx).Info("SDK session 工具面与当前选择不兼容，准备 fork",
				"session_key", sessionItem.SessionKey,
				"sdk_session_id", resumeID,
				"stored_tool_surface_present", actualToolSurface != "",
				"reason", string(decision.Reason),
			)
			return resumeID, true
		}
		if !fingerprintMatches {
			s.LoggerFor(ctx).Info("DM session runtime 配置已变更但 transcript 可恢复，继续 resume",
				"session_key", sessionItem.SessionKey,
				"sdk_session_id", resumeID,
				"old_runtime_kind", actualKind,
				"new_runtime_kind", expectedKind,
				"old_provider", actualProvider,
				"new_provider", expectedProvider,
				"old_model", actualModel,
				"new_model", expectedModel,
				"reason", string(decision.Reason),
			)
		}
		s.persistSDKSessionFingerprint(
			ctx,
			workspacePath,
			sessionItem,
			false,
			expectedKind,
			expectedProvider,
			expectedModel,
			toolSurfaceFingerprint,
		)
		return resumeID, false
	}
	if decision.Err != nil {
		s.LoggerFor(ctx).Warn("检查 SDK session transcript 失败，跳过过期 resume",
			"session_key", sessionItem.SessionKey,
			"workspace_path", workspacePath,
			"sdk_session_id", decision.SessionID,
			"reason", string(decision.Reason),
			"err", decision.Err,
		)
		s.persistSDKSessionFingerprint(
			ctx,
			workspacePath,
			sessionItem,
			true,
			expectedKind,
			expectedProvider,
			expectedModel,
			toolSurfaceFingerprint,
		)
		return "", false
	}

	s.LoggerFor(ctx).Warn("DM SDK session transcript 不存在，跳过过期 resume",
		"session_key", sessionItem.SessionKey,
		"sdk_session_id", decision.SessionID,
		"old_runtime_kind", actualKind,
		"new_runtime_kind", expectedKind,
		"old_provider", actualProvider,
		"new_provider", expectedProvider,
		"old_model", actualModel,
		"new_model", expectedModel,
		"reason", string(decision.Reason),
	)
	s.persistSDKSessionFingerprint(
		ctx,
		workspacePath,
		sessionItem,
		true,
		expectedKind,
		expectedProvider,
		expectedModel,
		toolSurfaceFingerprint,
	)
	return "", false
}

func (s *Service) persistSDKSessionFingerprint(
	ctx context.Context,
	workspacePath string,
	sessionItem protocol.Session,
	clearSessionID bool,
	runtimeKind string,
	provider string,
	model string,
	toolSurfaceFingerprint string,
) {
	if clearSessionID {
		sessionItem.TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
			sessionItem.TranscriptSessionIDs,
			protocol.SessionTranscriptIDs(sessionItem),
		)
		sessionItem.SessionID = nil
	}
	if sessionItem.Options == nil {
		sessionItem.Options = map[string]any{}
	}
	sessionItem.Options[protocol.OptionRuntimeKind] = strings.TrimSpace(runtimeKind)
	sessionItem.Options[protocol.OptionRuntimeProvider] = strings.TrimSpace(provider)
	sessionItem.Options[protocol.OptionRuntimeModel] = strings.TrimSpace(model)
	sessionItem.Options[protocol.OptionRuntimeToolSurfaceFingerprint] = strings.TrimSpace(toolSurfaceFingerprint)
	var err error
	sessionItem, err = s.preservePersistedSessionTitleForOwner(
		authctx.OwnerUserID(ctx),
		workspacePath,
		sessionItem,
	)
	if err != nil {
		s.LoggerFor(ctx).Error("DM session runtime 配置指纹保留标题失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
		return
	}
	if _, err := s.Files.ForOwner(authctx.OwnerUserID(ctx)).PatchSessionRuntime(
		workspacePath,
		sessionItem,
	); err != nil {
		s.LoggerFor(ctx).Error("DM session runtime 配置指纹更新失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
	}
}

func (s *Service) acquireRuntimeClient(
	ctx context.Context,
	startup *runtimectx.ClientStartup,
	options agentclient.Options,
) (runtimectx.Client, error) {
	client, err := startup.GetOrCreateWithFactory(ctx, options, nil)
	if err != nil {
		s.logRuntimeStartupFailure(ctx, startup.SessionKey(), "get_or_create", options, err)
		return client, err
	}
	if err := startup.Connect(ctx); err != nil {
		s.logRuntimeStartupFailure(ctx, startup.SessionKey(), "connect", options, err)
		return client, err
	}
	s.LoggerFor(ctx).Info("runtime client connected",
		"session_key", startup.SessionKey(),
		"sdk_session_id", strings.TrimSpace(client.SessionID()),
	)
	return client, nil
}

func (s *Service) logRuntimeStartupFailure(
	ctx context.Context,
	sessionKey string,
	stage string,
	options agentclient.Options,
	err error,
) {
	s.LoggerFor(ctx).Error("DM runtime 启动失败",
		append(clientopts.RuntimeStartupLogFields(options),
			"session_key", sessionKey,
			"stage", strings.TrimSpace(stage),
			"err", err,
			"error_type", fmt.Sprintf("%T", err),
			"transport_closed", runtimectx.IsRuntimeTransportClosedError(err),
		)...,
	)
}
