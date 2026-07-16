package memorymaintenance

// 本文件负责用 Agent 当前 owner/provider/background model 启动一次性 nxs。

import (
	"context"
	"errors"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
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
	preferences preferencesService
	providers   clientopts.RuntimeConfigResolver
	selector    *runtimeselectionsvc.Service
}

// NewCoordinator 构建 Nexus 托管 AutoDream 协调器。
func NewCoordinator(
	cfg config.Config,
	agents agentCatalog,
	providers clientopts.RuntimeConfigResolver,
	preferences preferencesService,
) *Coordinator {
	runner := &runtimeDreamRunner{
		preferences: preferences,
		providers:   providers,
		selector:    runtimeselectionsvc.NewService(preferences),
	}
	return newCoordinator(cfg.MemoryMaintenance, agents, runner)
}

func (r *runtimeDreamRunner) tryAutoDream(ctx context.Context, agentValue protocol.Agent) (agentclient.AutoDreamResult, error) {
	ownerContext := contextForAgentOwner(ctx, agentValue)
	selection, err := r.selector.Resolve(ownerContext, runtimeselectionsvc.Request{
		Agent:        &agentValue,
		OwnerUserIDs: []string{agentValue.OwnerUserID},
	})
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	if strings.TrimSpace(selection.RuntimeKind) != "nxs" {
		return agentclient.AutoDreamResult{
			Status: agentclient.AutoDreamStatusSkipped,
			Reason: "runtime_not_nxs",
		}, nil
	}
	provider, model, err := r.backgroundSelection(ownerContext, agentValue.OwnerUserID, selection)
	if err != nil {
		return agentclient.AutoDreamResult{}, err
	}
	options, err := clientopts.BuildAgentClientOptions(ownerContext, r.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:     agentValue.WorkspacePath,
		RuntimeKind:       selection.RuntimeKind,
		Provider:          provider,
		Model:             model,
		PermissionMode:    sdkpermission.ModeAcceptEdits,
		SettingSources:    ensureProjectSettingsSource(agentValue.Options.SettingSources),
		ToolSearchEnabled: selection.ToolSearchEnabled,
		WebSearch:         selection.WebSearch,
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
	defer closeDreamSession(session)
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
) (string, string, error) {
	provider := strings.TrimSpace(selection.Provider)
	model := strings.TrimSpace(selection.Model)
	if r.preferences != nil {
		preferences, err := r.preferences.Get(ctx, strings.TrimSpace(ownerUserID))
		if err != nil {
			return "", "", err
		}
		background := preferences.DefaultBackgroundModelSelection
		if strings.TrimSpace(background.Provider) != "" && strings.TrimSpace(background.Model) != "" {
			provider = strings.TrimSpace(background.Provider)
			model = strings.TrimSpace(background.Model)
		}
	}
	if provider == "" || model == "" {
		return "", "", errors.New("AutoDream 缺少可用的 provider/model")
	}
	return provider, model, nil
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
