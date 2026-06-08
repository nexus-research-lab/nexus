package runtimeselection

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

// PreferencesService 提供用户级 runtime 默认值读取能力。
type PreferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

// Service 收口 Agent runtime 的最终选择逻辑。
type Service struct {
	prefs PreferencesService
}

// Selection 表示启动 runtime 前已经合并完成的选择。
type Selection struct {
	RuntimeKind                string
	Provider                   string
	Model                      string
	AgentSDKDiagnosticsEnabled bool
}

// Request 表示一次 Agent runtime 选择请求。
type Request struct {
	Agent        *protocol.Agent
	OwnerUserIDs []string
}

// NewService 创建 runtime 选择服务。
func NewService(prefs PreferencesService) *Service {
	return &Service{prefs: prefs}
}

// Resolve 以 Agent 显式模型优先，否则回退到用户偏好中的默认 runtime/provider/model。
func (s *Service) Resolve(ctx context.Context, request Request) (Selection, error) {
	selection := Selection{}
	agentProvider, agentModel := explicitAgentModel(request.Agent)
	if agentProvider != "" && agentModel != "" {
		selection.Provider = agentProvider
		selection.Model = agentModel
	}

	prefs, ok, err := s.preferences(ctx, request)
	if err != nil {
		return Selection{}, err
	}
	if ok {
		selection.RuntimeKind = runtimeprovider.NormalizeRuntimeKind(prefs.AgentRuntimeKind)
		selection.AgentSDKDiagnosticsEnabled = prefs.AgentSDKDiagnosticsEnabled
		if selection.Provider == "" || selection.Model == "" {
			defaultProvider := strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
			defaultModel := strings.TrimSpace(prefs.DefaultAgentOptions.Model)
			if defaultProvider != "" && defaultModel != "" {
				selection.Provider = defaultProvider
				selection.Model = defaultModel
			}
		}
	}
	if selection.Provider == "" || selection.Model == "" {
		selection.Provider = firstNonEmpty(selection.Provider, agentProvider)
		selection.Model = firstNonEmpty(selection.Model, agentModel)
	}
	return selection, nil
}

func (s *Service) preferences(
	ctx context.Context,
	request Request,
) (preferencessvc.Preferences, bool, error) {
	if s == nil || s.prefs == nil {
		return preferencessvc.Preferences{}, false, nil
	}
	ownerUserID := ownerUserIDFromRequest(ctx, request)
	if ownerUserID == "" {
		return preferencessvc.Preferences{}, false, nil
	}
	prefs, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return preferencessvc.Preferences{}, false, err
	}
	return prefs, true, nil
}

func ownerUserIDFromRequest(ctx context.Context, request Request) string {
	if currentUserID, ok := authctx.CurrentUserID(ctx); ok {
		if ownerUserID := strings.TrimSpace(currentUserID); ownerUserID != "" {
			return ownerUserID
		}
	}
	for _, candidate := range request.OwnerUserIDs {
		if ownerUserID := strings.TrimSpace(candidate); ownerUserID != "" {
			return ownerUserID
		}
	}
	if request.Agent != nil {
		return strings.TrimSpace(request.Agent.OwnerUserID)
	}
	return ""
}

func explicitAgentModel(agent *protocol.Agent) (string, string) {
	if agent == nil {
		return "", ""
	}
	return strings.TrimSpace(agent.Options.Provider), strings.TrimSpace(agent.Options.Model)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}
