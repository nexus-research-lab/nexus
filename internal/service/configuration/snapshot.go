// INPUT: 主智能体 Actor、配置域筛选与本地校验开关。
// OUTPUT: 来自各真相源的脱敏配置值、域版本和健康检查。
// POS: configuration 控制面的统一读取与变更后核对阶段。
package configuration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

// Inspect 读取指定配置域；空列表表示全部域。
func (s *Service) Inspect(ctx context.Context, actor Actor, domains []string, verify bool) (*Inspection, error) {
	if err := requireMainActor(actor); err != nil {
		return nil, err
	}
	requested, err := normalizeDomains(domains)
	if err != nil {
		return nil, err
	}
	result := &Inspection{GeneratedAt: time.Now().UTC(), Domains: make(map[string]DomainSnapshot, len(requested))}
	for _, domain := range requested {
		snapshot, snapshotErr := s.domainSnapshot(scopedContext(ctx, actor), actor, domain, verify)
		if snapshotErr != nil {
			return nil, fmt.Errorf("读取配置域 %s: %w", domain, snapshotErr)
		}
		result.Domains[domain] = snapshot
	}
	return result, nil
}

func normalizeDomains(domains []string) ([]string, error) {
	if len(domains) == 0 {
		return domainNames(), nil
	}
	result := make([]string, 0, len(domains))
	seen := map[string]struct{}{}
	for _, domain := range domains {
		definition, err := definitionFor(domain)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[definition.Name]; ok {
			continue
		}
		seen[definition.Name] = struct{}{}
		result = append(result, definition.Name)
	}
	slices.Sort(result)
	return result, nil
}

func (s *Service) domainSnapshot(ctx context.Context, actor Actor, domain string, verify bool) (DomainSnapshot, error) {
	definition, err := definitionFor(domain)
	if err != nil {
		return DomainSnapshot{}, err
	}
	values, checks, err := s.domainValues(ctx, actor, definition.Name, verify)
	if err != nil {
		return DomainSnapshot{}, err
	}
	safeValues := sanitizeValue(values)
	revision, err := revisionFor(safeValues)
	if err != nil {
		return DomainSnapshot{}, err
	}
	return DomainSnapshot{Definition: definition, Revision: revision, Values: safeValues, Checks: checks}, nil
}

func (s *Service) domainValues(ctx context.Context, actor Actor, domain string, verify bool) (any, []Check, error) {
	switch domain {
	case DomainPreferences:
		value, err := s.prefs.Get(ctx, actor.OwnerUserID)
		return value, preferencesChecks(value, err), err
	case DomainProviders:
		items, err := s.providers.List(ctx)
		if err != nil {
			return nil, providerChecks(providerDomainValues{}, err), err
		}
		preferences, err := s.prefs.Get(ctx, actor.OwnerUserID)
		if err != nil {
			return nil, providerChecks(providerDomainValues{}, err), err
		}
		options, err := s.providers.ListOptionsForRuntime(ctx, preferences.AgentRuntimeKind)
		values := providerDomainValues{
			Items: items, Presets: s.providers.ListPresets(), RuntimeOptions: options,
		}
		return values, providerChecks(values, err), err
	case DomainAgents:
		values, err := s.agents.ListAgentRecords(ctx)
		if err != nil {
			return nil, agentChecks(nil, nil, err), err
		}
		providers, providerErr := s.providers.List(ctx)
		return values, agentChecks(values, providers, providerErr), providerErr
	case DomainChannels:
		values, err := s.channels.ListChannels(ctx, actor.OwnerUserID)
		if err != nil {
			return nil, nil, err
		}
		pairings, err := s.channels.ListPairings(ctx, actor.OwnerUserID, channels.PairingQuery{})
		return map[string]any{"items": values, "pairings": pairings}, channelChecks(values, err), err
	case DomainConnectors:
		items, err := s.connectors.ListConnectors(ctx, actor.OwnerUserID, "", "", "")
		if err != nil {
			return nil, connectorChecks(connectorDomainValues{}, err), err
		}
		details := make(map[string]*connectorsvc.Detail, len(items))
		for _, item := range items {
			detail, detailErr := s.connectors.GetConnectorDetail(ctx, actor.OwnerUserID, item.ConnectorID)
			if detailErr != nil {
				return nil, connectorChecks(connectorDomainValues{Items: items}, detailErr), detailErr
			}
			details[item.ConnectorID] = detail
		}
		values := connectorDomainValues{Items: items, Details: details}
		return values, connectorChecks(values, nil), nil
	case DomainSkills:
		sources, err := s.skills.ListExternalSkillSources(ctx)
		if err != nil {
			return nil, nil, err
		}
		items, err := s.skills.ListSkills(ctx, skillsvc.Query{AgentID: actor.AgentID})
		return map[string]any{"sources": sources, "main_agent_items": items}, skillChecks(sources, err), err
	case DomainHost:
		values := map[string]any{
			"current_workspace_path": s.cfg.WorkspacePath,
			"startup_configuration":  s.cfg,
			"mutability": map[string]any{
				"workspace_path":        "read_only; change deployment environment or use the native desktop state-root migration",
				"startup_configuration": "read_only; change deployment environment and restart",
			},
		}
		return values, s.hostChecks(verify), nil
	case DomainAutomation, DomainRooms, DomainWorkspaces, DomainGoals:
		definition, _ := definitionFor(domain)
		return map[string]any{
			"delegated": true, "managed_by": definition.ManagedBy,
			"reason": "该域已有更强的专用对话工具；统一目录负责发现与边界说明，写入仍走专用领域服务",
		}, []Check{okCheck(domain, "delegated_control", "专用配置控制入口可用")}, nil
	default:
		return nil, nil, fmt.Errorf("未实现配置域 %s", domain)
	}
}

func okCheck(domain, code, message string) Check {
	return Check{Code: code, Status: "ok", Message: message, Domain: domain, Verified: true}
}

func errorCheck(domain, code string, err error) Check {
	return Check{Code: code, Status: "error", Message: err.Error(), Domain: domain, Verified: true}
}

func preferencesChecks(_ any, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainPreferences, "preferences_readable", err)}
	}
	return []Check{okCheck(DomainPreferences, "preferences_readable", "偏好文件可读取并通过结构校验")}
}

type providerDomainValues struct {
	Items          []providersvc.Record         `json:"items"`
	Presets        []providersvc.Preset         `json:"presets"`
	RuntimeOptions *providersvc.OptionsResponse `json:"runtime_options"`
}

func providerChecks(values providerDomainValues, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainProviders, "providers_readable", err)}
	}
	checks := []Check{okCheck(DomainProviders, "providers_readable", "Provider 配置、preset、模型卡与 runtime 默认选择可读取；远端连通性仅在显式 test 操作时验证")}
	enabled := 0
	for _, item := range values.Items {
		if item.Enabled {
			enabled++
		}
		if item.LastTestStatus == providersvc.TestStatusFailed {
			checks = append(checks, Check{
				Code: "provider_last_test_failed", Status: "warning",
				Message: item.LastTestError, Domain: DomainProviders, Target: item.Provider,
				Remedy: "核对 endpoint/token/model 后显式执行 test_provider", Verified: true,
			})
		}
	}
	switch {
	case len(values.Items) == 0:
		checks = append(checks, Check{
			Code: "provider_missing", Status: "error", Message: "尚未配置 Provider",
			Domain: DomainProviders, Remedy: "先 plan/apply providers.create", Verified: true,
		})
	case enabled == 0:
		checks = append(checks, Check{
			Code: "provider_enabled_missing", Status: "error", Message: "Provider 全部处于禁用状态",
			Domain: DomainProviders, Remedy: "启用至少一个有效 Provider", Verified: true,
		})
	case values.RuntimeOptions == nil || values.RuntimeOptions.DefaultSelection == nil:
		checks = append(checks, Check{
			Code: "provider_default_missing", Status: "warning", Message: "当前 runtime 没有默认模型选择",
			Domain: DomainProviders, Remedy: "设置一个已启用模型为默认", Verified: true,
		})
	}
	return checks
}

func agentChecks(values []protocol.Agent, providers []providersvc.Record, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainAgents, "agent_runtime_references_readable", err)}
	}
	checks := []Check{okCheck(DomainAgents, "agents_readable", fmt.Sprintf("已核对 %d 个 Agent 配置", len(values)))}
	providerByKey := make(map[string]providersvc.Record, len(providers))
	hasDefaultModel := false
	for _, item := range providers {
		providerByKey[item.Provider] = item
		if !item.Enabled {
			continue
		}
		for _, model := range item.Models {
			if model.Enabled && model.IsDefault {
				hasDefaultModel = true
				break
			}
		}
	}
	for _, item := range values {
		if strings.TrimSpace(item.Options.Provider) == "" != (strings.TrimSpace(item.Options.Model) == "") {
			checks = append(checks, Check{
				Code: "agent_model_selection_incomplete", Status: "warning",
				Message: "Agent 的 provider/model 必须同时为空或同时配置", Domain: DomainAgents,
				Target: item.AgentID, Remedy: "更新 Agent options 中的 provider 与 model", Verified: true,
			})
			continue
		}
		if strings.TrimSpace(item.Options.Provider) == "" {
			if !hasDefaultModel {
				checks = append(checks, Check{
					Code: "agent_default_model_unavailable", Status: "error",
					Message: "Agent 跟随系统默认模型，但当前没有可用默认模型", Domain: DomainAgents,
					Target: item.AgentID, Remedy: "设置默认模型，或为 Agent 指定 provider/model", Verified: true,
				})
			}
			continue
		}
		provider, ok := providerByKey[item.Options.Provider]
		if !ok || !provider.Enabled {
			checks = append(checks, Check{
				Code: "agent_provider_unavailable", Status: "error",
				Message: "Agent 引用的 Provider 不存在或未启用", Domain: DomainAgents,
				Target: item.AgentID, Remedy: "启用对应 Provider，或更新 Agent provider/model", Verified: true,
			})
			continue
		}
		modelReady := false
		for _, model := range provider.Models {
			if model.ModelID == item.Options.Model && model.Enabled {
				modelReady = true
				break
			}
		}
		if !modelReady {
			checks = append(checks, Check{
				Code: "agent_model_unavailable", Status: "error",
				Message: "Agent 引用的模型不存在或未启用", Domain: DomainAgents,
				Target: item.AgentID, Remedy: "刷新/启用模型，或更新 Agent provider/model", Verified: true,
			})
		}
	}
	return checks
}

func channelChecks(values []channels.ChannelConfigView, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainChannels, "channels_readable", err)}
	}
	checks := []Check{okCheck(DomainChannels, "channels_readable", "Channel 公共配置、凭据存在状态、runtime 状态与配对可读取")}
	for _, item := range values {
		if !item.Configured || (item.Status != channels.ChannelConfigStatusError && strings.TrimSpace(item.LastError) == "") {
			continue
		}
		checks = append(checks, Check{
			Code: "channel_runtime_error", Status: "warning", Message: item.LastError,
			Domain: DomainChannels, Target: item.ChannelType,
			Remedy: "核对公开配置、凭据和路由 Agent 后重新 upsert", Verified: true,
		})
	}
	return checks
}

type connectorDomainValues struct {
	Items   []connectorsvc.Info             `json:"items"`
	Details map[string]*connectorsvc.Detail `json:"details"`
}

func connectorChecks(values connectorDomainValues, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainConnectors, "connectors_readable", err)}
	}
	checks := []Check{okCheck(DomainConnectors, "connectors_readable", "Connector 目录、认证类型、连接状态、额外字段与凭据存在状态可读取")}
	for _, item := range values.Items {
		if item.ConfigError == nil || strings.TrimSpace(*item.ConfigError) == "" {
			continue
		}
		checks = append(checks, Check{
			Code: "connector_configuration_error", Status: "warning", Message: *item.ConfigError,
			Domain: DomainConnectors, Target: item.ConnectorID,
			Remedy: "核对 OAuth client 或部署级 connector 配置", Verified: true,
		})
	}
	return checks
}

func skillChecks(values any, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainSkills, "skills_readable", err)}
	}
	return []Check{okCheck(DomainSkills, "skills_readable", "Skill 来源与主智能体安装状态可读取")}
}

func (s *Service) hostChecks(verify bool) []Check {
	checks := []Check{okCheck(DomainHost, "startup_config_loaded", "启动配置已读取；敏感项仅显示 configured 状态")}
	if !verify {
		return checks
	}
	path := strings.TrimSpace(s.cfg.WorkspacePath)
	if path == "" {
		return append(checks, Check{
			Code: "workspace_path_default", Status: "warning", Message: "未显式配置 workspace_path",
			Domain: DomainHost, Remedy: "通过部署环境或原生桌面状态根迁移设置后重启", Verified: true,
		})
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return append(checks, errorCheck(DomainHost, "workspace_path_resolvable", err))
	}
	info, err := os.Stat(resolved)
	switch {
	case errors.Is(err, os.ErrNotExist):
		checks = append(checks, Check{
			Code: "workspace_path_exists", Status: "warning", Message: "workspace 路径尚不存在，重启时将尝试创建",
			Domain: DomainHost, Target: resolved, Verified: true,
		})
	case err != nil:
		checks = append(checks, errorCheck(DomainHost, "workspace_path_accessible", err))
	case !info.IsDir():
		checks = append(checks, Check{
			Code: "workspace_path_directory", Status: "error", Message: "workspace_path 不是目录",
			Domain: DomainHost, Target: resolved, Verified: true,
		})
	default:
		checks = append(checks, okCheck(DomainHost, "workspace_path_exists", "workspace_path 存在且为目录"))
	}
	return checks
}
