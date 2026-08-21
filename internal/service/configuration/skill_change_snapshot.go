// INPUT: 已授权的 Skills 变更、显式 target_scope/source_identity 与当前 Agent 目录快照。
// OUTPUT: 绑定来源身份、Agent runtime_version、非破坏性启停状态与完整性 revision 的计划/写后快照。
// POS: Skills 控制面从 owner 目录语义收敛到具体来源及 Agent 配置资源的并发边界。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

type skillCatalogItemView struct {
	Name              string                         `json:"name"`
	Title             string                         `json:"title"`
	Description       string                         `json:"description"`
	Scope             string                         `json:"scope"`
	Tags              []string                       `json:"tags,omitempty"`
	CategoryKey       string                         `json:"category_key,omitempty"`
	CategoryName      string                         `json:"category_name,omitempty"`
	SourceType        string                         `json:"source_type"`
	Version           string                         `json:"version,omitempty"`
	Locked            bool                           `json:"locked"`
	HasUpdate         bool                           `json:"has_update"`
	Deletable         bool                           `json:"deletable"`
	SourceKind        string                         `json:"source_kind,omitempty"`
	SourceName        string                         `json:"source_name,omitempty"`
	SourceTrust       string                         `json:"source_trust,omitempty"`
	ImportMode        string                         `json:"import_mode,omitempty"`
	StorageScope      string                         `json:"storage_scope,omitempty"`
	OriginKind        string                         `json:"origin_kind,omitempty"`
	TargetScope       skillsvc.AgentSkillTargetScope `json:"target_scope,omitempty"`
	SourceIdentity    string                         `json:"source_identity,omitempty"`
	EnabledForAgent   bool                           `json:"enabled_for_agent"`
	EnabledAgentCount int                            `json:"enabled_agent_count,omitempty"`
}

type skillSourceView struct {
	SourceID             string `json:"source_id"`
	Name                 string `json:"name"`
	Kind                 string `json:"kind"`
	URL                  string `json:"url"`
	Trust                string `json:"trust"`
	Enabled              bool   `json:"enabled"`
	SortOrder            int    `json:"sort_order"`
	ManagedBy            string `json:"managed_by"`
	AuthType             string `json:"auth_type"`
	CredentialConfigured bool   `json:"credential_configured"`
	Deletable            bool   `json:"deletable"`
}

func safeSkillCatalogItems(items []skillsvc.Info) []skillCatalogItemView {
	result := make([]skillCatalogItemView, 0, len(items))
	for _, item := range items {
		result = append(result, skillCatalogItemView{
			Name: item.Name, Title: item.Title, Description: item.Description,
			Scope: item.Scope, Tags: item.Tags, CategoryKey: item.CategoryKey,
			CategoryName: item.CategoryName, SourceType: item.SourceType,
			Version: item.Version, Locked: item.Locked, HasUpdate: item.HasUpdate,
			Deletable: item.Deletable, SourceKind: item.SourceKind,
			SourceName: item.SourceName, SourceTrust: item.SourceTrust,
			ImportMode: item.ImportMode, StorageScope: item.StorageScope,
			OriginKind: item.OriginKind, TargetScope: item.TargetScope,
			SourceIdentity: item.SourceIdentity, EnabledForAgent: item.EnabledForAgent,
			EnabledAgentCount: item.EnabledAgentCount,
		})
	}
	return result
}

func safeSkillSources(items []skillsvc.ExternalSkillSourceInfo) []skillSourceView {
	result := make([]skillSourceView, 0, len(items))
	for _, item := range items {
		result = append(result, skillSourceView{
			SourceID: item.SourceID, Name: item.Name, Kind: item.Kind,
			URL: item.URL, Trust: item.Trust, Enabled: item.Enabled,
			SortOrder: item.SortOrder, ManagedBy: item.ManagedBy,
			AuthType: item.AuthType, CredentialConfigured: item.CredentialConfigured,
			Deletable: item.Deletable,
		})
	}
	return result
}

func (s *Service) skillDomainValues(
	ctx context.Context,
	actor *resolvedActor,
) (any, int64, ScopeRef, error) {
	scope := ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}
	for range 4 {
		before, err := s.skills.GetCatalogState(ctx)
		if err != nil {
			return nil, 0, scope, err
		}
		if !actor.isMain() {
			items, listErr := s.skills.ListSkills(ctx, skillsvc.Query{AgentID: actor.AgentID})
			if listErr != nil {
				return nil, 0, ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, listErr
			}
			after, stateErr := s.skills.GetCatalogState(ctx)
			if stateErr != nil {
				return nil, 0, ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, stateErr
			}
			if before.Version != after.Version {
				continue
			}
			return map[string]any{
					"catalog_version": after.Version,
					"agent_id":        actor.AgentID,
					"items":           safeSkillCatalogItems(items),
				},
				after.Version,
				ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID},
				nil
		}
		sources, err := s.skills.ListExternalSkillSources(ctx)
		if err != nil {
			return nil, 0, scope, err
		}
		globalLibraryItems, err := s.skills.ListSkills(ctx, skillsvc.Query{})
		if err != nil {
			return nil, 0, scope, err
		}
		agents, err := s.agents.ListAgentRecords(ctx)
		if err != nil {
			return nil, 0, scope, err
		}
		agentWorkspaceItems := make(map[string]any, len(agents))
		var mainAgentItems []skillCatalogItemView
		for index := range agents {
			agentValue := agents[index]
			items, listErr := s.skills.ListSkills(ctx, skillsvc.Query{AgentID: agentValue.AgentID})
			if listErr != nil {
				return nil, 0, scope, listErr
			}
			workspaceItems := make([]skillsvc.Info, 0)
			for _, item := range items {
				if item.TargetScope == skillsvc.AgentSkillTargetWorkspace {
					workspaceItems = append(workspaceItems, item)
				}
			}
			agentWorkspaceItems[agentValue.AgentID] = map[string]any{
				"agent_id": agentValue.AgentID,
				"name":     agentValue.Name,
				"items":    safeSkillCatalogItems(workspaceItems),
			}
			if agentValue.AgentID == actor.AgentID {
				mainAgentItems = safeSkillCatalogItems(items)
			}
		}
		after, err := s.skills.GetCatalogState(ctx)
		if err != nil {
			return nil, 0, scope, err
		}
		if before.Version != after.Version {
			continue
		}
		return map[string]any{
			"catalog_version":       after.Version,
			"sources":               safeSkillSources(sources),
			"global_library_items":  safeSkillCatalogItems(globalLibraryItems),
			"main_agent_items":      mainAgentItems,
			"agent_workspace_items": agentWorkspaceItems,
		}, after.Version, scope, nil
	}
	return nil, 0, scope, skillsvc.ErrCatalogSnapshotUnstable
}

func skillChangeTarget(actor *resolvedActor, request ChangeRequest) (skillAgentTarget, bool, error) {
	if request.Domain != DomainSkills {
		return skillAgentTarget{}, false, nil
	}
	switch request.Operation {
	case "install", "uninstall":
		var input skillAgentTarget
		if len(request.Input) == 0 {
			return skillAgentTarget{}, true, errors.New("skills install/uninstall 要求 input")
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return skillAgentTarget{}, true, fmt.Errorf("解析 Skills 目标 Agent: %w", err)
		}
		input = input.Normalized()
		if input.AgentID == "" {
			return skillAgentTarget{}, true, errors.New("skills install/uninstall 的 agent_id 不能为空")
		}
		return input, true, nil
	case "install_self", "uninstall_self":
		agentID := strings.TrimSpace(actor.AgentID)
		if agentID == "" {
			return skillAgentTarget{}, true, errors.New("Skills self 操作缺少可信 Agent 身份")
		}
		var input skillAgentTarget
		if len(request.Input) == 0 {
			return skillAgentTarget{}, true, errors.New("Skills self 操作要求 input")
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return skillAgentTarget{}, true, fmt.Errorf("解析 Skills self 目标: %w", err)
		}
		input.AgentID = agentID
		return input.Normalized(), true, nil
	default:
		return skillAgentTarget{}, false, nil
	}
}

func (s *Service) snapshotForChange(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
	verify bool,
) (DomainSnapshot, error) {
	return s.snapshotForChangeState(ctx, actor, request, verify, verify)
}

func (s *Service) snapshotForChangeState(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
	verify bool,
	verifyOutcome bool,
) (DomainSnapshot, error) {
	snapshot, err := s.domainSnapshot(ctx, actor, request.Domain, request.Target, verify)
	if err != nil {
		return DomainSnapshot{}, err
	}
	snapshot, err = s.augmentConnectorChangeSnapshot(ctx, actor, request, snapshot, verifyOutcome)
	if err != nil {
		return DomainSnapshot{}, err
	}
	selection, scopedToAgent, err := skillChangeTarget(actor, request)
	if err != nil {
		return DomainSnapshot{}, err
	}
	if !scopedToAgent {
		return s.augmentSkillCatalogChangeSnapshot(ctx, request, snapshot, verifyOutcome)
	}
	targetState, err := s.skills.GetAgentSkillStateInScope(
		ctx,
		selection.AgentID,
		request.Target,
		selection.TargetScope,
	)
	if err != nil {
		return DomainSnapshot{}, fmt.Errorf("读取 Skills 目标状态: %w", err)
	}
	if targetState.SourceIdentity != selection.SourceIdentity {
		return DomainSnapshot{}, errors.New("Skills source_identity 与当前来源不匹配；请重新 inspect 后选择明确来源")
	}
	if !verify && !targetState.Available {
		return DomainSnapshot{}, fmt.Errorf(
			"Agent %s 的可见 Skill 目录中不存在 %s",
			targetState.AgentID,
			request.Target,
		)
	}
	if !verify && targetState.Locked {
		return DomainSnapshot{}, fmt.Errorf(
			"Skill %s 由系统托管，不能通过 Agent 配置开关修改",
			targetState.SkillName,
		)
	}
	if !verify {
		switch request.Operation {
		case "install", "install_self":
			if targetState.Installed {
				return DomainSnapshot{}, fmt.Errorf(
					"Agent %s 的 Skill %s 在 %s 已启用，无需重复变更",
					targetState.AgentID,
					targetState.SkillName,
					targetState.TargetScope,
				)
			}
		case "uninstall", "uninstall_self":
			if !targetState.Installed {
				return DomainSnapshot{}, fmt.Errorf(
					"Agent %s 的 Skill %s 在 %s 已停用，无需重复变更",
					targetState.AgentID,
					targetState.SkillName,
					targetState.TargetScope,
				)
			}
		}
	}
	if verifyOutcome {
		if err = verifySkillTargetOutcome(request, targetState); err != nil {
			return DomainSnapshot{}, err
		}
	}
	key, err := s.integrityKeyBytes()
	if err != nil {
		return DomainSnapshot{}, fmt.Errorf("初始化 Skills revision 密钥: %w", err)
	}
	snapshot.Revision, err = integrityRevisionFor(map[string]any{
		"skill_catalog_revision": snapshot.Revision,
		"agent_id":               targetState.AgentID,
		"agent_runtime_version":  targetState.RuntimeVersion,
		"target_scope":           targetState.TargetScope,
		"source_identity":        targetState.SourceIdentity,
		"target_skill":           targetState,
	}, key)
	if err != nil {
		return DomainSnapshot{}, err
	}
	snapshot.Scope = ScopeRef{Kind: ScopeKindAgent, ID: targetState.AgentID}
	snapshot.StateVersion = targetState.RuntimeVersion
	snapshot.Values = map[string]any{
		"catalog": snapshot.Values,
		"target_agent": map[string]any{
			"agent_id": targetState.AgentID, "runtime_version": targetState.RuntimeVersion,
		},
		"selection": map[string]any{
			"target_scope": targetState.TargetScope, "source_identity": targetState.SourceIdentity,
		},
		"target_skill": targetState,
	}
	snapshot.Checks = append(snapshot.Checks, Check{
		Code:   "skill_target_state_readable",
		Status: "ok",
		Message: fmt.Sprintf(
			"已绑定 Agent %s runtime_version=%d，Skill %s target_scope=%s source_identity=%s available=%t installed=%t",
			targetState.AgentID,
			targetState.RuntimeVersion,
			targetState.SkillName,
			targetState.TargetScope,
			targetState.SourceIdentity,
			targetState.Available,
			targetState.Installed,
		),
		Domain:   DomainSkills,
		Target:   targetState.SkillName,
		Verified: true,
	})
	if verifyOutcome {
		snapshot.Checks = append(snapshot.Checks, Check{
			Code:   "skill_target_installation_state_verified",
			Status: "ok",
			Message: fmt.Sprintf(
				"已核对 Agent %s 上 Skill %s target_scope=%s source_identity=%s installed=%t",
				targetState.AgentID,
				targetState.SkillName,
				targetState.TargetScope,
				targetState.SourceIdentity,
				targetState.Installed,
			),
			Domain:   DomainSkills,
			Target:   targetState.SkillName,
			Verified: true,
		})
	}
	return snapshot, nil
}

func (s *Service) augmentSkillCatalogChangeSnapshot(
	ctx context.Context,
	request ChangeRequest,
	snapshot DomainSnapshot,
	verifyOutcome bool,
) (DomainSnapshot, error) {
	if request.Domain != DomainSkills {
		return snapshot, nil
	}
	var (
		targetKind string
		target     any
		version    int64
	)
	switch request.Operation {
	case "update_source", "delete_private_source", "import_private":
		state, err := s.skills.GetCatalogSourceState(ctx, request.Target)
		if err != nil {
			return DomainSnapshot{}, fmt.Errorf("读取 Skill 来源状态: %w", err)
		}
		if !state.Exists && !(verifyOutcome && request.Operation == "delete_private_source") {
			return DomainSnapshot{}, errors.New("Skill 来源不存在")
		}
		switch request.Operation {
		case "update_source":
			var input skillSourceUpdateInput
			if err = strictDecodeJSON(request.Input, &input); err != nil {
				return DomainSnapshot{}, err
			}
			if !state.Deletable &&
				(input.Name != nil || input.AuthType != nil || jsonFieldProvided(input.Token)) {
				return DomainSnapshot{}, errors.New("系统 Skill 来源只允许修改 enabled")
			}
			if jsonFieldProvided(input.Token) && input.AuthType == nil &&
				!strings.EqualFold(state.AuthType, "bearer") {
				return DomainSnapshot{}, errors.New("只有 bearer 私有来源可以轮换 token")
			}
			if verifyOutcome {
				if input.Name != nil && state.Name != strings.TrimSpace(*input.Name) {
					return DomainSnapshot{}, errors.New("Skill 来源写后 name 与计划不一致")
				}
				if input.Enabled != nil && state.Enabled != *input.Enabled {
					return DomainSnapshot{}, errors.New("Skill 来源写后 enabled 与计划不一致")
				}
				if input.AuthType != nil && !strings.EqualFold(state.AuthType, strings.TrimSpace(*input.AuthType)) {
					return DomainSnapshot{}, errors.New("Skill 来源写后 auth_type 与计划不一致")
				}
				if jsonFieldProvided(input.Token) && !state.CredentialConfigured {
					return DomainSnapshot{}, errors.New("Skill 来源写后未记录已配置凭据")
				}
				if input.AuthType != nil && strings.EqualFold(strings.TrimSpace(*input.AuthType), "none") &&
					state.CredentialConfigured {
					return DomainSnapshot{}, errors.New("Skill 来源切换为无认证后仍残留凭据")
				}
			}
		case "delete_private_source":
			if verifyOutcome {
				if state.Exists {
					return DomainSnapshot{}, errors.New("私有 Skill 来源删除后仍存在")
				}
			} else if !state.Deletable {
				return DomainSnapshot{}, errors.New("该 Skill 来源不是可删除的 owner 私有来源")
			}
		case "import_private":
			if !state.Deletable {
				return DomainSnapshot{}, errors.New("skills.import_private 要求 owner 私有来源")
			}
			if !state.Enabled {
				return DomainSnapshot{}, errors.New("私有 Skill 来源已停用")
			}
		}
		targetKind, target, version = "source", state, state.CatalogVersion
	case "delete", "update_single":
		state, err := s.skills.GetCatalogSkillState(ctx, request.Target)
		if err != nil {
			return DomainSnapshot{}, fmt.Errorf("读取 Skill catalog 目标: %w", err)
		}
		if verifyOutcome && request.Operation == "delete" {
			if state.Exists {
				return DomainSnapshot{}, fmt.Errorf("Skill %s 删除后仍存在", request.Target)
			}
		} else {
			if !state.Exists {
				return DomainSnapshot{}, fmt.Errorf("Skill %s 不存在", request.Target)
			}
			if !verifyOutcome && request.Operation == "delete" && !state.Deletable {
				return DomainSnapshot{}, fmt.Errorf("Skill %s 不允许删除", request.Target)
			}
		}
		targetKind, target, version = "skill", state, state.CatalogVersion
	default:
		return snapshot, nil
	}
	key, err := s.integrityKeyBytes()
	if err != nil {
		return DomainSnapshot{}, fmt.Errorf("初始化 Skills catalog revision 密钥: %w", err)
	}
	snapshot.Revision, err = integrityRevisionFor(map[string]any{
		"skill_catalog_revision": snapshot.Revision,
		"target_kind":            targetKind,
		"target":                 target,
	}, key)
	if err != nil {
		return DomainSnapshot{}, err
	}
	snapshot.StateVersion = version
	snapshot.Values = map[string]any{
		"catalog":     snapshot.Values,
		"target_kind": targetKind,
		"target":      target,
	}
	snapshot.Checks = append(snapshot.Checks, Check{
		Code: "skill_catalog_target_readable", Status: "ok",
		Message: fmt.Sprintf(
			"已绑定 Skill catalog_version=%d 与 %s 目标 %s",
			version,
			targetKind,
			request.Target,
		),
		Domain: DomainSkills, Target: request.Target, Verified: true,
	})
	if verifyOutcome {
		snapshot.Checks = append(snapshot.Checks, Check{
			Code: "skill_catalog_target_verified", Status: "ok",
			Message: "已从版本化 Skill catalog 重新核对目标结果",
			Domain:  DomainSkills, Target: request.Target, Verified: true,
		})
	}
	return snapshot, nil
}

func verifySkillTargetOutcome(request ChangeRequest, state skillsvc.AgentSkillState) error {
	switch request.Operation {
	case "install", "install_self":
		if !state.Available || !state.Installed {
			return fmt.Errorf(
				"Skills 写后核对失败：Agent %s 的 Skill %s expected installed=true, available=%t installed=%t",
				state.AgentID,
				state.SkillName,
				state.Available,
				state.Installed,
			)
		}
	case "uninstall", "uninstall_self":
		if state.Installed {
			return fmt.Errorf(
				"Skills 写后核对失败：Agent %s 的 Skill %s expected installed=false, actual=true",
				state.AgentID,
				state.SkillName,
			)
		}
	}
	return nil
}
