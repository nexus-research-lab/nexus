// INPUT: 动态解析后的 owner-main / agent-self / room-host / room-member、配置域筛选与校验开关。
// OUTPUT: 当前权限可读的脱敏配置、资源 scope、状态版本、revision、能力目录与健康检查。
// POS: configuration 控制面的作用域读取与变更后核对阶段。
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

	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

// Inspect 读取当前可信 Actor 可见的配置域；空列表表示该 Actor 的全部可读域。
func (s *Service) Inspect(ctx context.Context, actor Actor, domains []string, verify bool) (*Inspection, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	requested, err := normalizeDomainsForActor(resolved, domains)
	if err != nil {
		return nil, err
	}
	result := &Inspection{
		GeneratedAt: time.Now().UTC(),
		Authority:   resolved.Authority,
		Context:     resolved.Context,
		Domains:     make(map[string]DomainSnapshot, len(requested)),
	}
	scoped := scopedContext(ctx, resolved.Actor)
	for _, domain := range requested {
		snapshot, snapshotErr := s.domainSnapshot(scoped, resolved, domain, "", verify)
		if snapshotErr != nil {
			return nil, fmt.Errorf("读取配置域 %s: %w", domain, snapshotErr)
		}
		result.Domains[domain] = snapshot
	}
	return result, nil
}

func normalizeDomainsForActor(actor *resolvedActor, domains []string) ([]string, error) {
	if len(domains) == 0 {
		return readableDomains(actor), nil
	}
	result := make([]string, 0, len(domains))
	seen := map[string]struct{}{}
	for _, domain := range domains {
		definition, _, err := definitionForActor(actor, domain)
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

func (s *Service) domainSnapshot(
	ctx context.Context,
	actor *resolvedActor,
	domain string,
	target string,
	verify bool,
) (DomainSnapshot, error) {
	definition, access, err := definitionForActor(actor, domain)
	if err != nil {
		return DomainSnapshot{}, err
	}
	values, checks, stateVersion, scope, err := s.domainValues(ctx, actor, definition.Name, target, verify)
	if err != nil {
		return DomainSnapshot{}, err
	}
	safeValues := sanitizeValue(values)
	key, err := s.integrityKeyBytes()
	if err != nil {
		return DomainSnapshot{}, fmt.Errorf("初始化配置 revision 密钥: %w", err)
	}
	revision, err := integrityRevisionFor(values, key)
	if err != nil {
		return DomainSnapshot{}, err
	}
	return DomainSnapshot{
		Definition:   definition,
		Scope:        scope,
		Access:       access,
		Revision:     revision,
		StateVersion: stateVersion,
		Values:       safeValues,
		Checks:       checks,
	}, nil
}

func (s *Service) domainValues(
	ctx context.Context,
	actor *resolvedActor,
	domain string,
	target string,
	verify bool,
) (any, []Check, int64, ScopeRef, error) {
	target = strings.TrimSpace(target)
	scope := actor.Context
	if actor.isMain() {
		scope = ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}
	}
	switch domain {
	case DomainMembers:
		value, version, err := s.memberSnapshot(ctx, actor, target)
		return value, nil, version, scope, err
	case DomainPreferences:
		return s.readPreferencesConfiguration(ctx, actor, target)
	case DomainProviders:
		return s.readProvidersConfiguration(ctx, actor, target)
	case DomainAgents:
		return s.readAgentsConfiguration(ctx, actor, target)
	case DomainEmotion:
		return s.readEmotionConfiguration(ctx, actor, target)
	case DomainChannels:
		return s.readChannelsConfiguration(ctx, actor, target)
	case DomainConnectors:
		return s.readConnectorsConfiguration(ctx, actor, target)
	case DomainSkills:
		values, catalogVersion, skillScope, err := s.skillDomainValues(ctx, actor)
		return values, skillChecks(values, err), catalogVersion, skillScope, err
	case DomainHost:
		values := map[string]any{
			"current_workspace_path": s.cfg.WorkspacePath,
			"startup_configuration":  hostStartupConfigurationSnapshot(s.cfg),
			"mutability": map[string]any{
				"workspace_path":        "read_only; change deployment environment or use the native desktop state-root migration",
				"startup_configuration": "read_only; change deployment environment and restart",
			},
		}
		return values, s.hostChecks(verify), 0, scope, nil
	case DomainSessions:
		return s.sessionDomainValues(ctx, actor, target)
	case DomainRooms:
		return s.readRoomsConfiguration(ctx, actor, target)
	case DomainAutomation, DomainWorkspaces, DomainGoals, DomainExecutions:
		definition, _ := definitionFor(domain)
		return map[string]any{
			"delegated": true, "managed_by": definition.ManagedBy,
			"reason": "该域已有更强的专用对话工具；统一目录负责发现与边界说明，写入仍走专用领域服务",
		}, []Check{okCheck(domain, "delegated_control", "专用配置控制入口可用")}, 0, scope, nil
	default:
		return nil, nil, 0, scope, fmt.Errorf("未实现配置域 %s", domain)
	}
}

func okCheck(domain, code, message string) Check {
	return Check{Code: code, Status: "ok", Message: message, Domain: domain, Verified: true}
}

func errorCheck(domain, code string, err error) Check {
	return Check{Code: code, Status: "error", Message: err.Error(), Domain: domain, Verified: true}
}

type providerDomainValues struct {
	Items          []providersvc.Record         `json:"items"`
	Presets        []providersvc.Preset         `json:"presets"`
	RuntimeOptions *providersvc.OptionsResponse `json:"runtime_options"`
}

type connectorDomainValues struct {
	Items   []connectorsvc.Info             `json:"items"`
	Details map[string]*connectorsvc.Detail `json:"details"`
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
