// INPUT: Agent owner/provider/background model、workspace settings、runtime admission 与共享 Manager。
// OUTPUT: 经 Manager 进程/策略回执、scratch 交接和统一退出管理的一次性 nxs AutoDream 结果。
// POS: Nexus 托管 AutoDream 的 runtime 启动与生命周期边界。
package memorymaintenance

import (
	"context"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

const (
	autoDreamWakeModeEnv     = "NEXUS_AUTO_DREAM_WAKE_MODE"
	providerManagedByHostEnv = "NEXUS_PROVIDER_MANAGED_BY_HOST"
	backgroundModelEnv       = "NEXUS_BACKGROUND_MODEL"
	autoDreamWakeModeHost    = "host"
)

type runtimeDreamRunner struct {
	runtime     *runtimectx.Manager
	config      config.Config
	preferences preferencesService
	providers   clientopts.RuntimeConfigResolver
	selector    *runtimeselectionsvc.Service
	admission   clientopts.AgentRuntimeAdmissionResolver
}

// NewCoordinator 构建 Nexus 托管 AutoDream 协调器。
func NewCoordinator(
	cfg config.Config,
	agents agentCatalog,
	providers clientopts.RuntimeConfigResolver,
	preferences preferencesService,
	admission clientopts.AgentRuntimeAdmissionResolver,
	runtime *runtimectx.Manager,
) *Coordinator {
	runner := &runtimeDreamRunner{
		runtime:     runtime,
		config:      cfg,
		preferences: preferences,
		providers:   providers,
		selector:    runtimeselectionsvc.NewService(preferences),
		admission:   admission,
	}
	return newCoordinator(cfg.MemoryMaintenance, agents, preferences, runner)
}

func sandboxResourcesFromLease(lease *runtimectx.SandboxResourceLease) *agentclient.SandboxResourcePolicy {
	if lease == nil {
		return nil
	}
	return lease.Resources()
}

func (r *runtimeDreamRunner) tryAutoDream(ctx context.Context, agentValue protocol.Agent) (result agentclient.AutoDreamResult, err error) {
	ownerContext := contextForAgentOwner(ctx, agentValue)
	selection, err := r.selector.Resolve(ownerContext, runtimeselectionsvc.Request{
		Agent:        &agentValue,
		OwnerUserIDs: []string{agentValue.OwnerUserID},
	})
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	if selection.AutoDreamDisabled {
		return agentclient.AutoDreamResult{
			Status: agentclient.AutoDreamStatusSkipped,
			Reason: "gate_closed",
		}, nil
	}
	if err := workspacepkg.EnsureUserSkillLibrary(r.config, agentValue.OwnerUserID); err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	runtimeSkillNames, err := workspacepkg.RuntimeSkillNamesForAgent(r.config, agentValue)
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	runtimeDisabledSkillNames, err := workspacepkg.RuntimeDisabledSkillNamesForAgent(
		r.config,
		agentValue,
	)
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	admission, err := clientopts.BeginAgentRuntimeAdmission(
		ownerContext,
		r.admission,
	)
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	defer admission.Release()
	ownerContext = admission.Context()
	provider, model, available, err := r.backgroundSelection(ownerContext, agentValue.OwnerUserID, selection)
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	if !available {
		return agentclient.AutoDreamResult{
			Status: agentclient.AutoDreamStatusSkipped,
			Reason: autoDreamProviderUnavailableReason,
		}, nil
	}
	var scratchLease *runtimectx.SandboxResourceLease
	scratchLeaseOwned := false
	if strings.EqualFold(strings.TrimSpace(r.config.AppMode), "desktop") {
		scratchLease, err = runtimectx.AcquireSandboxResource(ownerContext, runtimectx.SandboxResourceInput{
			OwnerUserID: agentValue.OwnerUserID,
			SessionKey:  "memory-maintenance:" + strings.TrimSpace(agentValue.AgentID),
			RoundID:     "auto-dream",
		})
		if err != nil {
			return agentclient.AutoDreamResult{}, err
		}
		defer func() {
			if !scratchLeaseOwned {
				_ = scratchLease.Release()
			}
		}()
	}
	options, err := clientopts.BuildAgentClientOptions(ownerContext, r.providers, clientopts.AgentClientOptionsInput{
		AppMode:               r.config.AppMode,
		DesktopSandboxEnabled: r.config.DesktopSandboxEnabled,
		WindowsSandboxPreview: r.config.WindowsSandboxPreview,
		WorkspacePath:         agentValue.WorkspacePath,
		OwnerUserID:           agentValue.OwnerUserID,
		IsMainAgent:           agentValue.IsMain,
		RuntimeKind:           "nxs",
		Provider:              provider,
		Model:                 model,
		PermissionMode:        sdkpermission.ModeAcceptEdits,
		SkillIDs:              runtimeSkillNames,
		DisabledSkillIDs:      runtimeDisabledSkillNames,
		SkillDirectories:      workspacepkg.SkillLibraryRoots(r.config, agentValue.OwnerUserID),
		SettingSources:        ensureProjectSettingsSource(agentValue.Options.SettingSources),
		AutoDreamDisabled:     selection.AutoDreamDisabled,
		ToolSearchEnabled:     selection.ToolSearchEnabled,
		WebSearch:             selection.WebSearch,
		RuntimeIsolationMode:  r.config.RuntimeIsolationMode,
		RuntimeLauncherPath:   r.config.RuntimeLauncherPath,
		SandboxResources:      sandboxResourcesFromLease(scratchLease),
		ExtraEnv: map[string]string{
			autoDreamWakeModeEnv:     autoDreamWakeModeHost,
			providerManagedByHostEnv: "1",
			backgroundModelEnv:       model,
		},
	})
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	result, scratchLeaseOwned, err = r.runtime.TryAutoDream(ownerContext, "memory-maintenance:"+strings.TrimSpace(agentValue.AgentID), options, scratchLease)
	return result, err
}

func (r *runtimeDreamRunner) backgroundSelection(
	ctx context.Context,
	ownerUserID string,
	selection runtimeselectionsvc.Selection,
) (string, string, bool, error) {
	provider := strings.TrimSpace(selection.Provider)
	model := strings.TrimSpace(selection.Model)
	if r.preferences != nil {
		preferences, err := r.preferences.Get(ctx, strings.TrimSpace(ownerUserID))
		if err != nil {
			return "", "", false, err
		}
		background := preferences.DefaultBackgroundModelSelection
		if strings.TrimSpace(background.Provider) != "" && strings.TrimSpace(background.Model) != "" {
			provider = strings.TrimSpace(background.Provider)
			model = strings.TrimSpace(background.Model)
		}
	}
	if provider == "" || model == "" {
		return "", "", false, nil
	}
	return provider, model, true, nil
}

func contextForAgentOwner(ctx context.Context, agentValue protocol.Agent) context.Context {
	ownerUserID := strings.TrimSpace(agentValue.OwnerUserID)
	if ownerUserID == "" {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "memory_maintenance",
	})
}

func ensureProjectSettingsSource(sources []string) []string {
	result := make([]string, 0, len(sources)+1)
	found := false
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if source == "project" || source == "projectSettings" {
			found = true
		}
		result = append(result, source)
	}
	if !found {
		result = append(result, "project")
	}
	return result
}
