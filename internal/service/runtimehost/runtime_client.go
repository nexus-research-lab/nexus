// INPUT: Agent、runtime 选择、权限、工具、Skill、提示词、MCP 与 desktop sandbox 资源。
// OUTPUT: DM 与 Room 共用的 Agent runtime 选项装配，以及带失效 resume 回退的启动状态机。
// POS: 宿主决定 session key、权限来源与提示词内容；这里只实现两个宿主相同的启动步骤。
package runtimehost

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

// RuntimePreferences 读取用户级 runtime 偏好。
type RuntimePreferences interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

// SetPreferences 注入用户级 runtime 偏好读取。
func (h *Host) SetPreferences(preferences RuntimePreferences) {
	h.Preferences = preferences
}

// ResolveAgentRuntimeSelection 按 Agent、会话选项与 owner 偏好解析 runtime 与模型。
func (h *Host) ResolveAgentRuntimeSelection(
	ctx context.Context,
	agent *protocol.Agent,
	sessionOptions map[string]any,
	ownerUserIDs ...string,
) (runtimeselectionsvc.Selection, error) {
	return runtimeselectionsvc.NewServiceWithRuntimeConfigResolver(h.Preferences, h.Providers).Resolve(ctx, runtimeselectionsvc.Request{
		Agent:          agent,
		OwnerUserIDs:   ownerUserIDs,
		SessionOptions: sessionOptions,
	})
}

// AgentRuntimeSkills 初始化 Agent 的 Skill 库并返回启用与禁用的 Skill；allowed 非空时只启用其中的 Skill。
func (h *Host) AgentRuntimeSkills(agent protocol.Agent, allowed []string) ([]string, []string, error) {
	if err := workspacepkg.EnsureUserSkillLibrary(h.Config, agent.OwnerUserID); err != nil {
		return nil, nil, err
	}
	if err := workspacepkg.EnsureInitializedForAgent(h.Config, agent); err != nil {
		return nil, nil, err
	}
	enabled, err := workspacepkg.RuntimeSkillNamesForAgent(h.Config, agent)
	if err != nil {
		return nil, nil, err
	}
	disabled, err := workspacepkg.RuntimeDisabledSkillNamesForAgent(h.Config, agent)
	if err != nil {
		return nil, nil, err
	}
	if len(allowed) == 0 {
		return enabled, disabled, nil
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		if name = strings.TrimSpace(name); name != "" {
			allowedSet[name] = struct{}{}
		}
	}
	filtered := make([]string, 0, len(disabled)+len(enabled))
	for _, name := range append(disabled, enabled...) {
		if _, ok := allowedSet[strings.TrimSpace(name)]; !ok {
			filtered = append(filtered, name)
		}
	}
	return append([]string(nil), allowed...), filtered, nil
}

// MergeNexusMCPServers 把宿主自有的 nexus MCP server 合入已装配的 MCP server 集合。
func (h *Host) MergeNexusMCPServers(
	ctx context.Context,
	servers map[string]sdkmcp.ServerConfig,
	round nexusmcp.RoundContext,
) (map[string]sdkmcp.ServerConfig, error) {
	runtimeServers, err := h.NexusMCP(ctx, round)
	if err != nil {
		return nil, err
	}
	if len(runtimeServers) > 0 && servers == nil {
		servers = make(map[string]sdkmcp.ServerConfig, len(runtimeServers))
	}
	for name, server := range runtimeServers {
		servers[name] = server
	}
	return servers, nil
}

// AcquireRuntimeScratch 在桌面 nxs 非完全放行时申请 sandbox scratch；不需要时返回 nil。
func (h *Host) AcquireRuntimeScratch(
	ctx context.Context,
	selection runtimeselectionsvc.Selection,
	permissionMode sdkpermission.Mode,
	input runtimectx.SandboxResourceInput,
) (*runtimectx.SandboxResourceLease, error) {
	if !strings.EqualFold(h.Config.AppMode, "desktop") ||
		(selection.RuntimeKind != "" && !strings.EqualFold(selection.RuntimeKind, "nxs")) ||
		permissionMode == sdkpermission.ModeBypassPermissions {
		return nil, nil
	}
	lease, err := runtimectx.AcquireSandboxResource(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("准备 desktop sandbox scratch: %w", err)
	}
	return lease, nil
}

// AgentRuntimeOptionsInput 是宿主为一次 Agent runtime 启动准备好的输入。
type AgentRuntimeOptionsInput struct {
	Agent                 *protocol.Agent
	Selection             runtimeselectionsvc.Selection
	PermissionMode        sdkpermission.Mode
	PermissionHandler     sdkpermission.Handler
	AllowedTools          []string
	DisallowedTools       []string
	SkillIDs              []string
	DisabledSkillIDs      []string
	AdditionalDirectories []string
	StablePrompt          string
	DynamicPrompt         string
	ResumeSessionID       string
	MCPServers            map[string]sdkmcp.ServerConfig
	ExtraEnv              map[string]string
	ConfigurationEnv      map[string]string
	Scratch               *runtimectx.SandboxResourceLease
}

// BuildAgentRuntimeOptions 用宿主配置与 Agent 设置补齐 runtime client 选项。
func (h *Host) BuildAgentRuntimeOptions(
	ctx context.Context,
	input AgentRuntimeOptionsInput,
) (agentclient.Options, *clientopts.RuntimeConfig, error) {
	agent := input.Agent
	selection := input.Selection
	return clientopts.BuildAgentClientOptionsWithConfig(ctx, h.Providers, clientopts.AgentClientOptionsInput{
		AppMode:                    h.Config.AppMode,
		DesktopSandboxEnabled:      h.Config.DesktopSandboxEnabled,
		WorkspacePath:              agent.WorkspacePath,
		OwnerUserID:                agent.OwnerUserID,
		IsMainAgent:                agent.IsMain,
		RuntimeKind:                selection.RuntimeKind,
		Provider:                   selection.Provider,
		Model:                      selection.Model,
		BackgroundProvider:         selection.BackgroundProvider,
		BackgroundModel:            selection.BackgroundModel,
		VisionProvider:             selection.VisionProvider,
		VisionModel:                selection.VisionModel,
		PermissionMode:             input.PermissionMode,
		PermissionHandler:          input.PermissionHandler,
		AllowedTools:               input.AllowedTools,
		DisallowedTools:            input.DisallowedTools,
		SkillIDs:                   input.SkillIDs,
		DisabledSkillIDs:           input.DisabledSkillIDs,
		SkillDirectories:           workspacepkg.SkillLibraryRoots(h.Config, agent.OwnerUserID),
		AdditionalDirectories:      input.AdditionalDirectories,
		SettingSources:             agent.Options.SettingSources,
		AppendSystemPrompt:         JoinPromptSections(input.StablePrompt, input.DynamicPrompt),
		AppendSystemPromptStatic:   input.StablePrompt,
		AppendSystemPromptDynamic:  input.DynamicPrompt,
		ResumeSessionID:            input.ResumeSessionID,
		MaxThinkingTokens:          agent.Options.MaxThinkingTokens,
		MaxTurns:                   agent.Options.MaxTurns,
		MCPServers:                 input.MCPServers,
		AgentMCPServers:            agent.Options.MCPServers,
		ExtraEnv:                   input.ExtraEnv,
		ConfigurationEnv:           input.ConfigurationEnv,
		AgentSDKDiagnosticsEnabled: selection.AgentSDKDiagnosticsEnabled,
		AutoMemoryDisabled:         selection.AutoMemoryDisabled,
		AutoDreamDisabled:          selection.AutoDreamDisabled,
		ToolSearchEnabled:          selection.ToolSearchEnabled,
		WebSearch:                  selection.WebSearch,
		RuntimeIsolationMode:       h.Config.RuntimeIsolationMode,
		RuntimeLauncherPath:        h.Config.RuntimeLauncherPath,
		SandboxResources:           input.Scratch.Resources(),
	})
}

// JoinPromptSections 用分隔线连接非空提示词段落。
func JoinPromptSections(sections ...string) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		if section = strings.TrimSpace(section); section != "" {
			parts = append(parts, section)
		}
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// WithAgentRuntimeHooks 为 runtime 选项挂上 guidance、子智能体准入、宿主额外 hook 与诊断日志。
func (h *Host) WithAgentRuntimeHooks(
	options agentclient.Options,
	sessionKey string,
	logger *slog.Logger,
	extra ...func(agentclient.Options) agentclient.Options,
) agentclient.Options {
	options = h.Runtime.WithGuidanceHook(options, sessionKey)
	options = h.Runtime.WithSubagentAdmissionHooks(options, sessionKey)
	for _, hook := range extra {
		options = hook(options)
	}
	return WithRuntimeDiagnosticsLogger(options, logger)
}

// RetireRuntimeClient 退休启动中的 runtime client，不受调用方取消影响。
func RetireRuntimeClient(startup *runtimectx.ClientStartup) (bool, error) {
	closeCtx, cancel := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
	defer cancel()
	return startup.RetireCurrent(closeCtx)
}

// RuntimeConnectAttempt 用当前选项与 scratch 启动一次 client；返回 scratch 所有权是否已交给 runtime。
type RuntimeConnectAttempt func(
	startup *runtimectx.ClientStartup,
	options agentclient.Options,
	scratch *runtimectx.SandboxResourceLease,
) (runtimectx.Client, bool, error)

// RuntimeConnect 描述一次带失效 resume 回退的 runtime 启动。
type RuntimeConnect struct {
	Startup      *runtimectx.ClientStartup
	Options      *agentclient.Options
	Scratch      *runtimectx.SandboxResourceLease
	ScratchInput runtimectx.SandboxResourceInput
	Logger       *slog.Logger
	Attempt      RuntimeConnectAttempt
	// ClearResume 在旧 SDK session 失效后清除宿主持久化的 resume 身份；为 nil 时不回退重试。
	ClearResume func() error
}

// Run 启动 runtime client。resume 的 transcript 已失效时，退休失败的 client、清除 resume、
// 换一份 scratch 后以全新 SDK session 重试一次；fork 仍返回 source session 时视为失败。
// 返回时 Scratch 已交给 runtime 或已释放。
func (c RuntimeConnect) Run(ctx context.Context) (runtimectx.Client, error) {
	scratch := c.Scratch
	client, transferred, err := c.Attempt(c.Startup, *c.Options, scratch)
	if err != nil && c.ClearResume != nil &&
		strings.TrimSpace(c.Options.Session.ResumeID) != "" &&
		runtimectx.IsRuntimeTransportClosedError(err) {
		client, scratch, transferred, err = c.retryWithoutResume(ctx, scratch, transferred, err)
	}
	if err == nil && c.Options.Session.Fork &&
		strings.TrimSpace(client.SessionID()) == strings.TrimSpace(c.Options.Session.ResumeID) {
		err = errors.New("runtime fork 仍返回 source SDK session")
	}
	if err != nil {
		if _, closeErr := RetireRuntimeClient(c.Startup); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
			c.Logger.Warn("清理启动失败的 runtime 返回错误", "startup_err", err, "cleanup_err", closeErr)
		}
	}
	if scratch != nil && !transferred {
		_ = scratch.Release()
	}
	return client, err
}

func (c RuntimeConnect) retryWithoutResume(
	ctx context.Context,
	scratch *runtimectx.SandboxResourceLease,
	transferred bool,
	cause error,
) (runtimectx.Client, *runtimectx.SandboxResourceLease, bool, error) {
	c.Logger.Warn("SDK session resume 失效，清除后重试",
		"sdk_session_id", strings.TrimSpace(c.Options.Session.ResumeID),
		"err", cause,
	)
	retired, closeErr := RetireRuntimeClient(c.Startup)
	if closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
		c.Logger.Warn("清理失效 resume 的 runtime 返回错误", "startup_err", cause, "cleanup_err", closeErr)
	}
	if !retired || errors.Is(closeErr, context.Canceled) || errors.Is(closeErr, context.DeadlineExceeded) {
		return nil, scratch, transferred, cause
	}
	if err := c.ClearResume(); err != nil {
		return nil, scratch, transferred, err
	}
	c.Options.Session.ResumeID = ""
	c.Options.Session.ResumeAt = ""
	c.Options.Session.Fork = false
	if scratch != nil {
		// 创建或绑定失败时 lease 尚未交给 runtime manager；先释放这份句柄再换新，
		// 否则旧 scratch 目录会永久登记而无人可达。
		if !transferred {
			if err := scratch.Release(); err != nil {
				return nil, nil, false, fmt.Errorf("清理失效 resume 的 desktop sandbox scratch: %w", err)
			}
		}
		next, err := runtimectx.AcquireSandboxResource(ctx, c.ScratchInput)
		if err != nil {
			return nil, nil, false, fmt.Errorf("重新准备 desktop sandbox scratch: %w", err)
		}
		scratch, transferred = next, false
	}
	client, transferred, err := c.Attempt(c.Startup, *c.Options, scratch)
	return client, scratch, transferred, err
}
