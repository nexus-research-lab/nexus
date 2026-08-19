// INPUT: Agent owner/provider/background model、workspace settings 与 runtime admission。
// OUTPUT: 可被认证转场强制取消并关闭的一次性 nxs AutoDream 结果。
// POS: Nexus 托管 AutoDream 的 runtime 启动与生命周期边界。
package memorymaintenance

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

const (
	autoDreamWakeModeEnv        = "NEXUS_AUTO_DREAM_WAKE_MODE"
	providerManagedByHostEnv    = "NEXUS_PROVIDER_MANAGED_BY_HOST"
	backgroundModelEnv          = "NEXUS_BACKGROUND_MODEL"
	autoDreamWakeModeHost       = "host"
	internalRuntimeCloseTimeout = 10 * time.Second
)

type preferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

type runtimeDreamRunner struct {
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
) *Coordinator {
	runner := &runtimeDreamRunner{
		config:      cfg,
		preferences: preferences,
		providers:   providers,
		selector:    runtimeselectionsvc.NewService(preferences),
		admission:   admission,
	}
	return newCoordinator(cfg.MemoryMaintenance, agents, runner)
}

func (r *runtimeDreamRunner) tryAutoDream(ctx context.Context, agentValue protocol.Agent) (agentclient.AutoDreamResult, error) {
	ownerContext := contextForAgentOwner(ctx, agentValue)
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
	selection, err := r.selector.Resolve(ownerContext, runtimeselectionsvc.Request{
		Agent:        &agentValue,
		OwnerUserIDs: []string{agentValue.OwnerUserID},
	})
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
	options, err := clientopts.BuildAgentClientOptions(ownerContext, r.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:        agentValue.WorkspacePath,
		OwnerUserID:          agentValue.OwnerUserID,
		IsMainAgent:          agentValue.IsMain,
		RuntimeKind:          "nxs",
		Provider:             provider,
		Model:                model,
		PermissionMode:       sdkpermission.ModeAcceptEdits,
		SkillIDs:             runtimeSkillNames,
		DisabledSkillIDs:     runtimeDisabledSkillNames,
		SkillDirectories:     workspacepkg.SkillLibraryRoots(r.config, agentValue.OwnerUserID),
		SettingSources:       ensureProjectSettingsSource(agentValue.Options.SettingSources),
		ToolSearchEnabled:    selection.ToolSearchEnabled,
		WebSearch:            selection.WebSearch,
		RuntimeIsolationMode: r.config.RuntimeIsolationMode,
		RuntimeLauncherPath:  r.config.RuntimeLauncherPath,
		ExtraEnv: map[string]string{
			autoDreamWakeModeEnv:     autoDreamWakeModeHost,
			providerManagedByHostEnv: "1",
			backgroundModelEnv:       model,
		},
	})
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	session, err := agentclient.NewSession(ownerContext, options)
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	var closeOnce sync.Once
	closeSession := func() {
		closeOnce.Do(func() {
			closeDreamSession(session)
		})
	}
	defer closeSession()
	stopForcedClose := closeDreamSessionOnCancellation(ownerContext, closeSession)
	defer stopForcedClose()
	stopDrain := drainDreamSession(ownerContext, session)
	defer stopDrain()
	if !session.Supports(agentclient.CapabilityAutoDream) {
		return agentclient.AutoDreamResult{}, errors.New("当前 runtime 不支持 AutoDream")
	}
	return session.Control().TryAutoDream(ownerContext)
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

func closeDreamSession(session *agentclient.Session) {
	if session == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), internalRuntimeCloseTimeout)
	defer cancel()
	_ = session.Close(ctx)
}

// closeDreamSessionOnCancellation 确保 admission 撤销不只依赖 control RPC
// 主动观察 context；即使 RPC 未及时返回，也会并发关闭 bridge session。
func closeDreamSessionOnCancellation(ctx context.Context, closeSession func()) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	var stopOnce sync.Once
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			if closeSession != nil {
				closeSession()
			}
		case <-stop:
		}
	}()
	return func() {
		stopOnce.Do(func() {
			close(stop)
		})
		<-done
	}
}

// drainDreamSession 消费维护过程事件，避免无人读取的 bridge 消息队列反压 control response。
func drainDreamSession(ctx context.Context, session *agentclient.Session) func() {
	drainContext, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := session.Recv(drainContext); err != nil {
				return
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
