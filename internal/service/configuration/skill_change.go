// INPUT: Skill 目录请求与来源、目标身份。
// OUTPUT: Skill 安装、来源管理、目录变更及写后核验。
// POS: Skills 配置操作的同域实现，复用独立快照与 CAS 边界。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func validateSkillsChange(request ChangeRequest) error {
	target := strings.TrimSpace(request.Target)
	switch request.Operation {
	case "search_external":
		if target != "" {
			return errors.New("skills.search_external 不接受 target")
		}
		var input skillSearchInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.Query) == "" {
			return errors.New("skills.search_external 的 query 不能为空")
		}
		return nil
	case "preview_external":
		if target != "" {
			return errors.New("skills.preview_external 不接受 target")
		}
		var input skillPreviewInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.DetailURL) == "" {
			return errors.New("skills.preview_external 的 detail_url 不能为空")
		}
		return nil
	case "create_private_source":
		if target != "" {
			return errors.New("skills.create_private_source 不接受 target")
		}
		var input skillPrivateSourceCreateInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.URL) == "" {
			return errors.New("skills.create_private_source 要求 name 和 url")
		}
		return validatePrivateSkillSourceAuth(input.AuthType, input.Token)
	case "import_git":
		if target != "" {
			return errors.New("skills.import_git 不接受 target")
		}
		var input skillGitImportInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.RepositoryURL) == "" {
			return errors.New("skills.import_git 的 repository_url 不能为空")
		}
		return nil
	case "import_url":
		if target != "" {
			return errors.New("skills.import_url 不接受 target")
		}
		var input skillURLImportInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.SourceURL) == "" {
			return errors.New("skills.import_url 的 source_url 不能为空")
		}
		return nil
	case "import_skills_sh":
		if target != "" {
			return errors.New("skills.import_skills_sh 不接受 target")
		}
		var input skillSkillsShImportInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.PackageSpec) == "" ||
			strings.TrimSpace(input.SkillSlug) == "" {
			return errors.New("skills.import_skills_sh 要求 package_spec 和 skill_slug")
		}
		return nil
	case "update_source":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input skillSourceUpdateInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Name == nil && input.Enabled == nil && input.AuthType == nil && !jsonFieldProvided(input.Token) {
			return errors.New("skills.update_source 至少要提供 name、enabled、auth_type 或 token")
		}
		if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
			return errors.New("skills.update_source 的 name 不能为空")
		}
		if input.AuthType != nil {
			return validatePrivateSkillSourceAuth(*input.AuthType, input.Token)
		}
		return nil
	case "delete_private_source":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&struct{}{})
	case "import_private":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input skillPrivateImportInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.SkillID) == "" {
			return errors.New("skills.import_private 要求 skill_id")
		}
		return nil
	case "install", "uninstall":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input skillAgentTarget
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		return validateSkillSelectionInput(input, true)
	case "install_self", "uninstall_self":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input skillAgentTarget
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		return validateSkillSelectionInput(input, false)
	case "delete", "update_single":
		return request.requireTarget()
	case "check_updates", "update_all":
		if target != "" {
			return fmt.Errorf("skills.%s 不接受 target", request.Operation)
		}
		return request.decodeInput(&struct{}{})
	default:
		return unsupportedChange(request)
	}
}

func (s *Service) executeSkillsChange(ctx context.Context, actor *resolvedActor, request ChangeRequest, stateVersion int64) (any, error) {
	switch request.Operation {
	case "search_external":
		var input skillSearchInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		return s.skills.SearchExternalSkills(ctx, input.Query, input.IncludeReadme)
	case "preview_external":
		var input skillPreviewInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		return s.skills.GetExternalSkillPreview(ctx, input.DetailURL)
	case "create_private_source":
		var input skillsvc.CreateExternalSkillSourceRequest
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("私有 Skill 来源创建缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.CreateExternalSkillSourceAtVersion(
			ctx,
			input,
			stateVersion,
		)
		if err == nil || skillsvc.SkillMutationApplied(err) {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case "import_git":
		var input skillGitImportInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Skill Git 导入缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.ImportGitPathAtVersion(
			ctx,
			input.RepositoryURL,
			input.Branch,
			input.SkillPath,
			stateVersion,
		)
		if err == nil {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case "import_url":
		var input skillURLImportInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Skill URL 导入缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.ImportSkillURLAtVersion(ctx, input.SourceURL, stateVersion)
		if err == nil {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case "import_skills_sh":
		var input skillSkillsShImportInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("skills.sh 导入缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.ImportSkillsShAtVersion(
			ctx,
			input.PackageSpec,
			input.SkillSlug,
			stateVersion,
		)
		if err == nil {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case "update_source":
		var input skillsvc.ExternalSkillSourceRequest
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Skill 来源更新缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.UpdateExternalSkillSourceAtVersion(
			ctx,
			request.Target,
			input,
			stateVersion,
		)
		if err == nil {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case "delete_private_source":
		if stateVersion <= 0 {
			return nil, errors.New("私有 Skill 来源删除缺少 catalog_version；请重新 plan")
		}
		err := s.skills.DeleteExternalSkillSourceAtVersion(
			ctx,
			request.Target,
			stateVersion,
		)
		applied := err == nil || skillsvc.SkillMutationApplied(err)
		if applied {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return map[string]any{
			"source_id": request.Target,
			"deleted":   applied,
		}, err
	case "import_private":
		var input skillPrivateImportInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("私有 Skill 导入缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.ImportPrivateSkillFromSourceAtVersion(
			ctx,
			skillsvc.ImportPrivateSkillRequest{
				SourceID: request.Target,
				SkillID:  input.SkillID,
			},
			stateVersion,
		)
		if err == nil || skillsvc.SkillMutationApplied(err) {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case "install":
		var input skillAgentTarget
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		input = input.Normalized()
		if stateVersion <= 0 {
			return nil, errors.New("Skills 安装缺少目标 Agent runtime_version；请重新 plan")
		}
		result, err := s.skills.SetAgentSkillEnabledInScopeAtVersion(
			ctx,
			input.AgentID,
			request.Target,
			true,
			input.TargetScope,
			input.SourceIdentity,
			stateVersion,
		)
		if err == nil {
			s.notifyAgentChanged(ctx, input.AgentID)
		}
		return result, err
	case "uninstall":
		var input skillAgentTarget
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		input = input.Normalized()
		if stateVersion <= 0 {
			return nil, errors.New("Skills 停用缺少目标 Agent runtime_version；请重新 plan")
		}
		result, err := s.skills.SetAgentSkillEnabledInScopeAtVersion(
			ctx,
			input.AgentID,
			request.Target,
			false,
			input.TargetScope,
			input.SourceIdentity,
			stateVersion,
		)
		if err == nil {
			s.notifyAgentChanged(ctx, input.AgentID)
		}
		return result, err
	case "install_self":
		var input skillAgentTarget
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Skills 安装缺少当前 Agent runtime_version；请重新 plan")
		}
		result, err := s.skills.SetAgentSkillEnabledInScopeAtVersion(
			ctx,
			actor.AgentID,
			request.Target,
			true,
			input.TargetScope,
			input.SourceIdentity,
			stateVersion,
		)
		if err == nil {
			s.notifyAgentChanged(ctx, actor.AgentID)
		}
		return result, err
	case "uninstall_self":
		var input skillAgentTarget
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Skills 停用缺少当前 Agent runtime_version；请重新 plan")
		}
		result, err := s.skills.SetAgentSkillEnabledInScopeAtVersion(
			ctx,
			actor.AgentID,
			request.Target,
			false,
			input.TargetScope,
			input.SourceIdentity,
			stateVersion,
		)
		if err == nil {
			s.notifyAgentChanged(ctx, actor.AgentID)
		}
		return result, err
	case "delete":
		if stateVersion <= 0 {
			return nil, errors.New("Skill 删除缺少 catalog_version；请重新 plan")
		}
		err := s.skills.DeleteSkillAtVersion(ctx, request.Target, stateVersion)
		if err == nil || skillsvc.SkillMutationApplied(err) {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return map[string]any{"skill_name": request.Target, "deleted": err == nil || skillsvc.SkillMutationApplied(err)}, err
	case "check_updates":
		return s.skills.CheckImportedSkillUpdates(ctx)
	case "update_single":
		if stateVersion <= 0 {
			return nil, errors.New("Skill 更新缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.UpdateSingleSkillAtVersion(ctx, request.Target, stateVersion)
		if err == nil || skillsvc.SkillMutationApplied(err) {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case "update_all":
		if stateVersion <= 0 {
			return nil, errors.New("Skill 批量更新缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.UpdateImportedSkillsAtVersion(ctx, stateVersion)
		if err == nil || skillsvc.SkillMutationApplied(err) {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	default:
		return nil, unsupportedChange(request)
	}
}

func (s *Service) notifySkillCatalogChanged(ctx context.Context, agentID string) {
	if s.notifier != nil {
		s.notifier.AgentChanged(ctx, agentID, "skill_catalog_updated")
	}
}

func validateSkillSelectionInput(input skillAgentTarget, requireAgentID bool) error {
	input = input.Normalized()
	if requireAgentID && input.AgentID == "" {
		return errors.New("Skills 变更要求 input.agent_id")
	}
	if !requireAgentID && input.AgentID != "" {
		return errors.New("Skills self 变更不能指定 agent_id")
	}
	switch input.TargetScope {
	case skillsvc.AgentSkillTargetGlobalLibrary, skillsvc.AgentSkillTargetWorkspace:
	default:
		return errors.New("Skills 变更的 target_scope 必须是 global_library 或 agent_workspace")
	}
	if input.SourceIdentity == "" {
		return errors.New("Skills 变更要求 inspect 返回的 source_identity")
	}
	return nil
}

func validatePrivateSkillSourceAuth(authType string, token json.RawMessage) error {
	switch strings.ToLower(strings.TrimSpace(authType)) {
	case "none":
		if jsonFieldProvided(token) {
			return errors.New("auth_type=none 时不能提供 token")
		}
	case "bearer":
		if !jsonFieldProvided(token) {
			return errors.New("auth_type=bearer 时必须通过 secret slot 提供 token")
		}
	default:
		return errors.New("auth_type 必须是 none 或 bearer")
	}
	return nil
}

func (s *Service) verifySkillCatalogResult(
	ctx context.Context,
	request ChangeRequest,
	resultValue any,
	plan ChangePlan,
	after DomainSnapshot,
) (*Check, error) {
	switch request.Operation {
	case "create_private_source":
		created, ok := resultValue.(*skillsvc.ExternalSkillSourceInfo)
		if !ok || created == nil || strings.TrimSpace(created.SourceID) == "" {
			return nil, errors.New("私有 Skill 来源创建结果缺少可核验身份")
		}
		state, err := s.skills.GetCatalogSourceState(ctx, created.SourceID)
		if err != nil {
			return nil, fmt.Errorf("重新读取私有 Skill 来源: %w", err)
		}
		var input skillPrivateSourceCreateInput
		if err = strictDecodeJSON(request.Input, &input); err != nil {
			return nil, err
		}
		if !state.Exists || !state.Deletable ||
			state.Name != strings.TrimSpace(input.Name) ||
			!strings.EqualFold(state.AuthType, strings.TrimSpace(input.AuthType)) {
			return nil, errors.New("私有 Skill 来源写后状态与创建计划不一致")
		}
		if strings.EqualFold(state.AuthType, "bearer") && !state.CredentialConfigured {
			return nil, errors.New("私有 Skill 来源写后未记录 Bearer 凭据")
		}
		return &Check{
			Code: "skill_private_source_creation_verified", Status: "ok",
			Message: "已重新读取私有 Skill 来源并核对归属、认证元数据和加密凭据存在性",
			Domain:  DomainSkills, Target: state.SourceID, Verified: true,
		}, nil
	case "update_source":
		updated, ok := resultValue.(*skillsvc.ExternalSkillSourceInfo)
		if !ok || updated == nil || strings.TrimSpace(updated.SourceID) != request.Target {
			return nil, errors.New("Skill 来源更新结果缺少可核验身份")
		}
		state, err := s.skills.GetCatalogSourceState(ctx, request.Target)
		if err != nil {
			return nil, fmt.Errorf("重新读取更新后的 Skill 来源: %w", err)
		}
		if !state.Exists || state.SourceID != request.Target {
			return nil, errors.New("Skill 来源更新后未出现在 catalog")
		}
		return &Check{
			Code: "skill_source_configuration_verified", Status: "ok",
			Message: "已重新读取 Skill 来源并核对功能配置与凭据存在性标记",
			Domain:  DomainSkills, Target: state.SourceID, Verified: true,
		}, nil
	case "delete_private_source":
		state, err := s.skills.GetCatalogSourceState(ctx, request.Target)
		if err != nil {
			return nil, fmt.Errorf("重新读取已删除私有 Skill 来源: %w", err)
		}
		if state.Exists {
			return nil, errors.New("私有 Skill 来源删除后仍存在")
		}
		return &Check{
			Code: "skill_private_source_deletion_verified", Status: "ok",
			Message: "已重新读取 owner Skill 来源目录并核对目标不存在；既有导入 Skill 保留",
			Domain:  DomainSkills, Target: request.Target, Verified: true,
		}, nil
	case "import_git", "import_url", "import_skills_sh", "import_private", "update_single":
		detail, ok := resultValue.(*skillsvc.Detail)
		if !ok || detail == nil || strings.TrimSpace(detail.Name) == "" {
			return nil, errors.New("Skill catalog 变更结果缺少可核验的 Skill 身份")
		}
		state, err := s.skills.GetCatalogSkillState(ctx, detail.Name)
		if err != nil {
			return nil, fmt.Errorf("重新读取 Skill catalog 结果: %w", err)
		}
		if !state.Exists {
			return nil, fmt.Errorf("Skill %s 写入后未出现在 catalog", detail.Name)
		}
		return &Check{
			Code: "skill_catalog_publication_verified", Status: "ok",
			Message: "已重新读取 Skill catalog 并核对原子发布结果",
			Domain:  DomainSkills, Target: detail.Name, Verified: true,
		}, nil
	case "update_all":
		result, ok := resultValue.(*skillsvc.UpdateInstalledSkillsResponse)
		if !ok || result == nil {
			return nil, errors.New("Skill 批量更新结果缺少可核验的逐项结果")
		}
		expectedVersion := plan.StateVersion + int64(len(result.UpdatedSkills))
		if after.StateVersion != expectedVersion {
			return nil, fmt.Errorf(
				"Skill 批量更新版本异常：expected=%d actual=%d；请 reconcile",
				expectedVersion,
				after.StateVersion,
			)
		}
		seen := make(map[string]struct{}, len(result.UpdatedSkills))
		for _, rawName := range result.UpdatedSkills {
			name := strings.TrimSpace(rawName)
			if name == "" {
				return nil, errors.New("Skill 批量更新结果包含空名称")
			}
			if _, duplicate := seen[name]; duplicate {
				return nil, fmt.Errorf("Skill 批量更新结果重复返回 %s", name)
			}
			seen[name] = struct{}{}
			state, err := s.skills.GetCatalogSkillState(ctx, name)
			if err != nil {
				return nil, fmt.Errorf("重新读取批量更新 Skill %s: %w", name, err)
			}
			if !state.Exists || state.CatalogVersion != after.StateVersion {
				return nil, fmt.Errorf(
					"Skill %s 批量更新后状态不一致：exists=%t catalog_version=%d",
					name,
					state.Exists,
					state.CatalogVersion,
				)
			}
		}
		return &Check{
			Code: "skill_catalog_bulk_update_verified", Status: "ok",
			Message: fmt.Sprintf(
				"已核对批量结果：更新 %d、跳过 %d、失败 %d，catalog version 从 %d 推进到 %d",
				len(result.UpdatedSkills),
				len(result.SkippedSkills),
				len(result.Failures),
				plan.StateVersion,
				after.StateVersion,
			),
			Domain: DomainSkills, Verified: true,
		}, nil
	default:
		return nil, nil
	}
}

func skillChecks(values any, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainSkills, "skills_readable", err)}
	}
	return []Check{okCheck(DomainSkills, "skills_readable", "Skill 来源与主智能体安装状态可读取")}
}

type skillAgentTarget struct {
	AgentID        string                         `json:"agent_id,omitempty"`
	TargetScope    skillsvc.AgentSkillTargetScope `json:"target_scope"`
	SourceIdentity string                         `json:"source_identity"`
}

func (i skillAgentTarget) Normalized() skillAgentTarget {
	i.AgentID = strings.TrimSpace(i.AgentID)
	i.SourceIdentity = strings.TrimSpace(i.SourceIdentity)
	return i
}

type skillSearchInput struct {
	Query         string `json:"query"`
	IncludeReadme bool   `json:"include_readme,omitempty"`
}

type skillPreviewInput struct {
	DetailURL string `json:"detail_url"`
}

type skillGitImportInput struct {
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch,omitempty"`
	SkillPath     string `json:"skill_path,omitempty"`
}

type skillURLImportInput struct {
	SourceURL string `json:"source_url"`
}

type skillSkillsShImportInput struct {
	PackageSpec string `json:"package_spec"`
	SkillSlug   string `json:"skill_slug"`
}

type skillPrivateSourceCreateInput struct {
	Name     string          `json:"name"`
	URL      string          `json:"url"`
	AuthType string          `json:"auth_type"`
	Token    json.RawMessage `json:"token,omitempty"`
}

type skillSourceUpdateInput struct {
	Name     *string         `json:"name,omitempty"`
	Enabled  *bool           `json:"enabled,omitempty"`
	AuthType *string         `json:"auth_type,omitempty"`
	Token    json.RawMessage `json:"token,omitempty"`
}

type skillPrivateImportInput struct {
	SkillID string `json:"skill_id"`
}
