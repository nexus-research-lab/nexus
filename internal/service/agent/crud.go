// INPUT: 可信 owner 上下文中的 Agent CRUD 请求与可选资源版本。
// OUTPUT: 带宿主受管语义 Skill 读取不变量的 Agent 持久状态、workspace 投影及跨域协调后的删除结果。
// POS: Agent 业务 CRUD 主链，删除通过消费侧协调器进入关联能力域。
package agent

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

type nameInvalidError struct {
	reason string
}

func (e nameInvalidError) Error() string {
	return e.reason
}

func (e nameInvalidError) Is(target error) bool {
	return target == ErrAgentNameInvalid
}

func fmtAgentNameInvalid(reason string) error {
	if strings.TrimSpace(reason) == "" {
		reason = "名称不合法"
	}
	return nameInvalidError{reason: reason}
}

// ListAgents 返回所有活跃 Agent。
func (s *Service) ListAgents(ctx context.Context) ([]protocol.Agent, error) {
	return s.listAgents(ctx, true)
}

// ListAgentRecords 返回所有活跃 Agent 的落库基础记录。
func (s *Service) ListAgentRecords(ctx context.Context) ([]protocol.Agent, error) {
	return s.listAgents(ctx, false)
}

// ListAllAgentRecordsForMaintenance 返回全局活跃 Agent 记录，仅供维护任务跨 owner 迁移使用。
func (s *Service) ListAllAgentRecordsForMaintenance(ctx context.Context) ([]protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	agents, err := s.repository.ListActiveAgents(ctx, "")
	if err != nil {
		return nil, err
	}
	normalizeManagedSemanticSkillBindings(agents)
	normalizeAgentAvatars(agents)
	return agents, nil
}

func (s *Service) listAgents(ctx context.Context, includeSkillsCount bool) ([]protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	agents, err := s.repository.ListActiveAgents(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	normalizeManagedSemanticSkillBindings(agents)
	if err = s.ensureAgentRuntimeStates(agents); err != nil {
		return nil, err
	}
	normalizeAgentAvatars(agents)
	if includeSkillsCount {
		err = s.enrichAgentsWithSkillsCount(agents)
	}
	if err != nil {
		return nil, err
	}
	return agents, nil
}

// GetAgent 获取指定 Agent。
func (s *Service) GetAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	agent, err := s.repository.GetAgent(ctx, agentID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.Status != "active" {
		return nil, ErrAgentNotFound
	}
	normalizeManagedSemanticSkillBinding(agent)
	normalizeAgentAvatar(agent)
	if err = s.ensureAgentRuntimeState(*agent); err != nil {
		return nil, err
	}
	if err = s.enrichAgentWithSkillsCount(agent); err != nil {
		return nil, err
	}
	return agent, nil
}

// GetAgentsByIDs 批量获取指定 ID 列表的活跃 Agent。
func (s *Service) GetAgentsByIDs(ctx context.Context, agentIDs []string) ([]protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	agents, err := s.repository.ListAgentsByIDs(ctx, ownerUserID, agentIDs)
	if err != nil {
		return nil, err
	}
	normalizeManagedSemanticSkillBindings(agents)
	if err = s.ensureAgentRuntimeStates(agents); err != nil {
		return nil, err
	}
	normalizeAgentAvatars(agents)
	return agents, nil
}

// GetDefaultAgent 返回当前作用域下的主智能体。
func (s *Service) GetDefaultAgent(ctx context.Context) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID := effectiveOwnerUserID(ctx)
	agent, err := s.repository.GetMainAgent(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.Status != "active" {
		return nil, ErrAgentNotFound
	}
	normalizeManagedSemanticSkillBinding(agent)
	normalizeAgentAvatar(agent)
	if err = s.ensureAgentRuntimeState(*agent); err != nil {
		return nil, err
	}
	if err = s.enrichAgentWithSkillsCount(agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *Service) ensureAgentRuntimeStates(agents []protocol.Agent) error {
	for _, agentValue := range agents {
		if err := s.ensureAgentRuntimeState(agentValue); err != nil {
			return err
		}
	}
	return nil
}

func normalizeAgentAvatar(agent *protocol.Agent) {
	if agent == nil {
		return
	}
	agent.Avatar = resolveAgentAvatar(agent.Avatar, agent.AgentID, agent.IsMain)
}

func normalizeAgentAvatars(agents []protocol.Agent) {
	for index := range agents {
		normalizeAgentAvatar(&agents[index])
	}
}

// normalizeManagedSemanticSkillBinding projects the host-owned Goal/Execution
// Skills as a read invariant. Persistence migrations repair stored rows, but a
// runtime launch never trusts a stale row enough to disable its control plane.
func normalizeManagedSemanticSkillBinding(agent *protocol.Agent) {
	if agent == nil {
		return
	}
	agent.Options.SkillIDs, agent.Options.DisabledSkillIDs = runtimecommand.BindManagedSemanticSkills(
		agent.Options.SkillIDs,
		agent.Options.DisabledSkillIDs,
	)
}

func normalizeManagedSemanticSkillBindings(agents []protocol.Agent) {
	for index := range agents {
		normalizeManagedSemanticSkillBinding(&agents[index])
	}
}

// ValidateName 校验名称格式。
func (s *Service) ValidateName(ctx context.Context, name string, excludeAgentID string) (protocol.ValidateNameResponse, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return protocol.ValidateNameResponse{}, err
	}
	return validateName(name), nil
}

func validateName(name string) protocol.ValidateNameResponse {
	normalized := NormalizeName(name)
	response := protocol.ValidateNameResponse{
		Name:           name,
		NormalizedName: normalized,
	}

	if reason := ValidateName(name); reason != "" {
		response.Reason = reason
		return response
	}

	response.IsValid = true
	response.IsAvailable = true
	return response
}

// CreateAgent 创建普通 Agent。
func (s *Service) CreateAgent(ctx context.Context, request protocol.CreateRequest) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}

	ownerUserID := effectiveOwnerUserID(ctx)
	validation := validateName(request.Name)
	if !validation.IsValid || !validation.IsAvailable {
		return nil, fmtAgentNameInvalid(validation.Reason)
	}

	agentID, workspacePath, err := s.createAgentWorkspacePath(ownerUserID)
	if err != nil {
		return nil, err
	}
	workspaceAgent := protocol.Agent{
		AgentID:       agentID,
		OwnerUserID:   ownerUserID,
		WorkspacePath: workspacePath,
	}
	root, err := s.openAgentWorkspace(workspaceAgent, false)
	if err != nil {
		_ = s.cleanupAgentWorkspace(ctx, workspaceAgent)
		return nil, err
	}
	if err = ensureRuntimeEmotionStateAt(root); err == nil {
		err = writeProfileTemplateAt(root, request.ProfileTemplate)
	}
	closeErr := root.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = s.cleanupAgentWorkspace(ctx, workspaceAgent)
		return nil, err
	}
	record := BuildCreateRecord(
		s.config,
		request,
		ownerUserID,
		validation.NormalizedName,
		agentID,
		workspacePath,
		"active",
		false,
	)
	if err = s.initializeAgentWorkspace(ctx, protocol.Agent{
		AgentID:       record.AgentID,
		OwnerUserID:   record.OwnerUserID,
		Name:          record.Name,
		WorkspacePath: record.WorkspacePath,
		Status:        record.Status,
		IsMain:        record.IsMain,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		_ = s.cleanupAgentWorkspace(ctx, workspaceAgent)
		return nil, err
	}
	created, err := s.repository.CreateAgent(ctx, record)
	if err != nil {
		_ = s.cleanupAgentWorkspace(ctx, workspaceAgent)
		return nil, err
	}
	if err = s.ensureAgentRuntimeState(*created); err != nil {
		return nil, err
	}
	normalizeAgentAvatar(created)
	return created, nil
}

// UpdateAgent 更新 Agent 配置。
func (s *Service) UpdateAgent(ctx context.Context, agentID string, request protocol.UpdateRequest) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	update := agentUpdate{
		service: s,
		ctx:     ctx,
		agentID: strings.TrimSpace(agentID),
		request: request,
	}
	return update.run()
}

// UpdateAgentSkillSelection 原子更新 Agent 的技能启用与停用集合。
//
// 技能开关不再复用完整 Agent 更新快照，避免编辑器中的旧 options 覆盖刚完成
// 的技能操作。
func (s *Service) UpdateAgentSkillSelection(
	ctx context.Context,
	agentID string,
	skillIDs []string,
	disabledSkillIDs []string,
) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	existing, err := s.loadAgentForSkillSelection(ctx, agentID)
	if err != nil {
		return nil, err
	}
	updated, err := s.repository.UpdateAgentSkillSelection(
		ctx,
		existing.AgentID,
		existing.OwnerUserID,
		mustJSONString(normalizeManagedSkillIDs(skillIDs)),
		mustJSONString(normalizeManagedDisabledSkillIDs(disabledSkillIDs)),
	)
	return s.finalizeAgentSkillSelection(updated, err)
}

// UpdateAgentSkillIDsAtVersion 仅更新全局 Skill 绑定并拒绝过期 runtime_version。
func (s *Service) UpdateAgentSkillIDsAtVersion(
	ctx context.Context,
	agentID string,
	skillIDs []string,
	expectedRuntimeVersion int64,
) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	existing, err := s.loadAgentForSkillSelection(ctx, agentID)
	if err != nil {
		return nil, err
	}
	updated, err := s.repository.UpdateAgentSkillIDsAtVersion(
		ctx,
		existing.AgentID,
		existing.OwnerUserID,
		mustJSONString(normalizeManagedSkillIDs(skillIDs)),
		expectedRuntimeVersion,
	)
	return s.finalizeAgentSkillSelection(updated, err)
}

// UpdateAgentDisabledSkillIDsAtVersion 仅更新 workspace Skill 停用集合并拒绝过期版本。
func (s *Service) UpdateAgentDisabledSkillIDsAtVersion(
	ctx context.Context,
	agentID string,
	disabledSkillIDs []string,
	expectedRuntimeVersion int64,
) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	existing, err := s.loadAgentForSkillSelection(ctx, agentID)
	if err != nil {
		return nil, err
	}
	updated, err := s.repository.UpdateAgentDisabledSkillIDsAtVersion(
		ctx,
		existing.AgentID,
		existing.OwnerUserID,
		mustJSONString(normalizeManagedDisabledSkillIDs(disabledSkillIDs)),
		expectedRuntimeVersion,
	)
	return s.finalizeAgentSkillSelection(updated, err)
}

func (s *Service) loadAgentForSkillSelection(
	ctx context.Context,
	agentID string,
) (*protocol.Agent, error) {
	scopedOwnerID, _ := scopedOwnerUserID(ctx)
	existing, err := s.repository.GetAgent(ctx, strings.TrimSpace(agentID), scopedOwnerID)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.Status != "active" {
		return nil, ErrAgentNotFound
	}
	normalizeManagedSemanticSkillBinding(existing)
	return existing, nil
}

func (s *Service) finalizeAgentSkillSelection(
	updated *protocol.Agent,
	err error,
) (*protocol.Agent, error) {
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrAgentNotFound
	}
	normalizeManagedSemanticSkillBinding(updated)
	if err = s.ensureAgentRuntimeState(*updated); err != nil {
		return nil, err
	}
	normalizeAgentAvatar(updated)
	if err = s.enrichAgentWithSkillsCount(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

type agentUpdate struct {
	service     *Service
	ctx         context.Context
	agentID     string
	request     protocol.UpdateRequest
	existing    *protocol.Agent
	ownerUserID string
}

func (u *agentUpdate) run() (*protocol.Agent, error) {
	if err := u.load(); err != nil {
		return nil, err
	}
	record, err := u.record()
	if err != nil {
		return nil, err
	}
	updated, err := u.service.repository.UpdateAgent(u.ctx, record)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrAgentNotFound
	}
	if err = u.finalize(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *agentUpdate) load() error {
	scopedOwnerID, _ := scopedOwnerUserID(u.ctx)
	existing, err := u.service.repository.GetAgent(u.ctx, u.agentID, scopedOwnerID)
	if err != nil {
		return err
	}
	if existing == nil || existing.Status != "active" {
		return ErrAgentNotFound
	}
	normalizeManagedSemanticSkillBinding(existing)
	u.existing = existing
	u.ownerUserID = existing.OwnerUserID
	if scopedOwnerID != "" {
		u.ownerUserID = scopedOwnerID
	}
	return nil
}

func (u *agentUpdate) record() (agentrepo.UpdateRecord, error) {
	name, err := u.normalizedName()
	if err != nil {
		return agentrepo.UpdateRecord{}, err
	}
	options := u.updatedOptions()
	options.SkillIDs, options.DisabledSkillIDs = runtimecommand.BindManagedSemanticSkills(
		options.SkillIDs,
		options.DisabledSkillIDs,
	)
	return agentrepo.UpdateRecord{
		AgentID:                u.existing.AgentID,
		OwnerUserID:            u.ownerUserID,
		Name:                   name,
		WorkspacePath:          u.existing.WorkspacePath,
		Avatar:                 updatedAgentText(u.existing.Avatar, u.request.Avatar),
		Description:            updatedAgentText(u.existing.Description, u.request.Description),
		VibeTagsJSON:           mustJSONString(u.updatedVibeTags()),
		Provider:               options.Provider,
		Model:                  options.Model,
		PermissionMode:         options.PermissionMode,
		AllowedToolsJSON:       mustJSONString(options.AllowedTools),
		DisallowedToolsJSON:    mustJSONString(options.DisallowedTools),
		MCPServersJSON:         mustJSONString(options.MCPServers),
		ConnectorIDsJSON:       mustJSONString(options.ConnectorIDs),
		SkillIDsJSON:           mustJSONString(options.SkillIDs),
		DisabledSkillIDsJSON:   mustJSONString(options.DisabledSkillIDs),
		MaxTurns:               options.MaxTurns,
		MaxThinkingTokens:      options.MaxThinkingTokens,
		SettingSourcesJSON:     mustJSONString(options.SettingSources),
		ExpectedRuntimeVersion: u.request.ExpectedRuntimeVersion,
	}, nil
}

func normalizeManagedSkillIDs(skillIDs []string) []string {
	bound, _ := runtimecommand.BindManagedSemanticSkills(normalizeStringList(skillIDs), nil)
	return bound
}

func normalizeManagedDisabledSkillIDs(disabledSkillIDs []string) []string {
	_, disabled := runtimecommand.BindManagedSemanticSkills(nil, normalizeStringList(disabledSkillIDs))
	return disabled
}

func (u *agentUpdate) normalizedName() (string, error) {
	if u.request.Name == nil {
		return u.existing.Name, nil
	}
	candidate := NormalizeName(*u.request.Name)
	if candidate == u.existing.Name {
		return u.existing.Name, nil
	}
	if u.existing.IsMain {
		return "", errors.New("主智能体名称不可修改")
	}
	validation := validateName(candidate)
	if !validation.IsValid || !validation.IsAvailable {
		return "", fmtAgentNameInvalid(validation.Reason)
	}
	return validation.NormalizedName, nil
}

func (u *agentUpdate) updatedOptions() protocol.Options {
	options := u.existing.Options
	if u.request.Options != nil {
		options = mergeOptions(u.existing.Options, *u.request.Options)
	}
	// Nexus 主智能体是全局默认模型的执行主体，不能留存单独的 Provider/Model 覆盖。
	if u.existing.IsMain {
		options.Provider = ""
		options.Model = ""
	}
	return options
}

func (u *agentUpdate) updatedVibeTags() []string {
	if u.request.VibeTags == nil {
		return u.existing.VibeTags
	}
	return slices.Clone(u.request.VibeTags)
}

func updatedAgentText(current string, requested *string) string {
	if requested == nil {
		return current
	}
	return strings.TrimSpace(*requested)
}

func (u *agentUpdate) finalize(updated *protocol.Agent) error {
	normalizeAgentAvatar(updated)
	if err := u.service.ensureAgentRuntimeState(*updated); err != nil {
		return err
	}
	return u.service.enrichAgentWithSkillsCount(updated)
}

// DeleteAgent 删除 Agent，并清理 workspace 目录与数据库记录。
func (s *Service) DeleteAgent(ctx context.Context, agentID string) error {
	return s.deleteAgent(ctx, agentID, nil)
}

// DeleteAgentAtVersion 仅在 runtime_version 仍等于计划版本时删除 Agent。
func (s *Service) DeleteAgentAtVersion(
	ctx context.Context,
	agentID string,
	expectedRuntimeVersion int64,
) error {
	return s.deleteAgent(ctx, agentID, &expectedRuntimeVersion)
}

func (s *Service) deleteAgent(
	ctx context.Context,
	agentID string,
	expectedRuntimeVersion *int64,
) error {
	if err := s.EnsureReady(ctx); err != nil {
		return err
	}
	agentID = strings.TrimSpace(agentID)
	ownerUserID, _ := scopedOwnerUserID(ctx)
	existing, err := s.repository.GetAgent(ctx, agentID, ownerUserID)
	if err != nil {
		return err
	}
	if existing == nil || existing.Status != "active" {
		return ErrAgentNotFound
	}
	if existing.IsMain {
		return errors.New("主智能体不可删除")
	}
	if expectedRuntimeVersion != nil && existing.RuntimeVersion != *expectedRuntimeVersion {
		return ErrRuntimeVersionConflict
	}
	sessions := []protocol.Session{}
	if s.sessions != nil {
		sessions, err = s.sessions.ListAgentSessions(ctx, existing.AgentID)
		if err != nil {
			return err
		}
	}
	return s.applyAgentDeletion(ctx, *existing, sessions, expectedRuntimeVersion)
}
