// INPUT: Agent 配置请求、可信身份与运行版本。
// OUTPUT: Agent 变更、额度预检和运行权限热同步。
// POS: Agent 配置的输入、读取、校验、写入与通知。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func validateAgentsChange(request ChangeRequest) error {
	switch request.Operation {
	case "create":
		var input protocol.CreateRequest
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Options != nil {
			if err := rejectScopedSkillOptionFields(request.Input, "options"); err != nil {
				return err
			}
			if err := validateConfigurationAgentOptions(*input.Options); err != nil {
				return err
			}
		}
		return nil
	case "update":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input agentUpdatePatch
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if len(input.Options) > 0 && string(input.Options) != "null" {
			if err := rejectScopedSkillOptionFields(input.Options, ""); err != nil {
				return err
			}
			var options protocol.Options
			if err := strictDecodeJSON(input.Options, &options); err != nil {
				return err
			}
			if err := validateConfigurationAgentOptions(options); err != nil {
				return err
			}
		}
		optionsPresent := len(input.Options) > 0 && string(input.Options) != "null"
		if input.Name == nil && !optionsPresent && input.Avatar == nil &&
			input.Description == nil && input.VibeTags == nil {
			return errors.New("agents.update 至少要提供一个待修改字段")
		}
		return nil
	case "update_self_profile":
		var input agentSelfProfilePatch
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Name == nil && input.Avatar == nil && input.Description == nil && input.VibeTags == nil {
			return errors.New("agents.update_self_profile 至少要提供一个待修改字段")
		}
		return nil
	case "update_self_runtime":
		var input agentSelfRuntimePatch
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Provider == nil && input.Model == nil && input.MaxTurns == nil && input.MaxThinkingTokens == nil {
			return errors.New("agents.update_self_runtime 至少要提供一个待修改字段")
		}
		if (input.Provider == nil) != (input.Model == nil) {
			return errors.New("普通 Agent 修改模型时必须同时提供 provider 和 model")
		}
		if input.MaxTurns != nil && *input.MaxTurns <= 0 {
			return errors.New("普通 Agent 的 max_turns 必须大于 0；0 会解除限制，只能由主智能体设置")
		}
		if input.MaxThinkingTokens != nil && *input.MaxThinkingTokens <= 0 {
			return errors.New("普通 Agent 的 max_thinking_tokens 必须大于 0；0 会解除限制，只能由主智能体设置")
		}
		return nil
	case "delete":
		return request.requireTarget()
	default:
		return unsupportedChange(request)
	}
}

func (s *Service) executeAgentsChange(ctx context.Context, actor *resolvedActor, request ChangeRequest, stateVersion int64) (any, error) {
	switch request.Operation {
	case "create":
		var input protocol.CreateRequest
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		return s.agents.CreateAgent(ctx, input)
	case "update":
		var input agentUpdatePatch
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		serviceInput, err := s.agentUpdateInput(ctx, request.Target, input)
		if err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Agent 更新缺少 runtime_version；请重新 plan")
		}
		serviceInput.ExpectedRuntimeVersion = &stateVersion
		updated, err := s.agents.UpdateAgent(ctx, request.Target, serviceInput)
		if err != nil {
			return nil, err
		}
		if inputContainsField(input.Options, "permission_mode") {
			if err = s.hotReloadPermissionMode(ctx, updated); err != nil {
				return updated, err
			}
		}
		s.notifyAgentChanged(ctx, updated.AgentID)
		return updated, nil
	case "update_self_profile":
		var input agentSelfProfilePatch
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		serviceInput := protocol.UpdateRequest{
			Name: input.Name, Avatar: input.Avatar, Description: input.Description, VibeTags: input.VibeTags,
		}
		if stateVersion <= 0 {
			return nil, errors.New("Agent 自有资料更新缺少 runtime_version；请重新 plan")
		}
		serviceInput.ExpectedRuntimeVersion = &stateVersion
		updated, err := s.agents.UpdateAgent(ctx, actor.AgentID, serviceInput)
		if err == nil {
			s.notifyAgentChanged(ctx, actor.AgentID)
		}
		return updated, err
	case "update_self_runtime":
		var input agentSelfRuntimePatch
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		serviceInput, err := s.agentSelfRuntimeUpdateInput(ctx, actor.AgentID, input)
		if err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Agent 自有 runtime 更新缺少 runtime_version；请重新 plan")
		}
		serviceInput.ExpectedRuntimeVersion = &stateVersion
		updated, err := s.agents.UpdateAgent(ctx, actor.AgentID, serviceInput)
		if err == nil {
			s.notifyAgentChanged(ctx, actor.AgentID)
		}
		return updated, err
	case "delete":
		if request.Target == actor.AgentID {
			return nil, errors.New("主智能体不能通过配置控制面删除自己")
		}
		if stateVersion <= 0 {
			return nil, errors.New("Agent 删除缺少 runtime_version；请重新 plan")
		}
		err := s.agents.DeleteAgentAtVersion(ctx, request.Target, stateVersion)
		if err == nil || agentsvc.AgentDeletionCommitted(err) {
			s.notifyAgentDeleted(ctx, request.Target)
		}
		return map[string]any{
			"agent_id": request.Target,
			"deleted":  err == nil || agentsvc.AgentDeletionCommitted(err),
		}, err
	default:
		return nil, unsupportedChange(request)
	}
}

func (s *Service) hotReloadPermissionMode(ctx context.Context, agent *protocol.Agent) error {
	if agent == nil {
		return errors.New("Agent 已更新，但无法读取新的 permission_mode")
	}
	mode := sdkpermission.Mode(strings.TrimSpace(agent.Options.PermissionMode))
	errs := make([]error, 0, 2)
	if s.runtime != nil {
		if err := s.runtime.SetPermissionModeForAgent(ctx, agent.AgentID, mode); err != nil {
			errs = append(errs, fmt.Errorf("同步活跃 DM runtime permission_mode: %w", err))
		}
	}
	if s.roomRuntime != nil {
		if err := s.roomRuntime.SetPermissionModeForAgent(ctx, agent.AgentID, mode); err != nil {
			errs = append(errs, fmt.Errorf("同步活跃 Room runtime permission_mode: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) notifyAgentChanged(ctx context.Context, agentID string) {
	if s.notifier != nil {
		s.notifier.AgentChanged(ctx, agentID, "agent_configuration_updated")
	}
}

func (s *Service) notifyAgentDeleted(ctx context.Context, agentID string) {
	if s.notifier != nil {
		s.notifier.AgentChanged(ctx, agentID, "agent_deleted")
	}
}

func validateConfigurationAgentOptions(options protocol.Options) error {
	if options.SkillIDs != nil || options.DisabledSkillIDs != nil {
		return errors.New(
			"Agent options 不能直接修改 skill_ids/disabled_skill_ids；" +
				"必须使用带 target_scope、source_identity 和 runtime_version 的 Skills 操作",
		)
	}
	if _, err := clientopts.MergeAgentMCPServers(nil, options.MCPServers); err != nil {
		return fmt.Errorf("Agent mcp_servers 无效: %w", err)
	}
	return nil
}

func rejectScopedSkillOptionFields(input json.RawMessage, container string) error {
	var values map[string]json.RawMessage
	if len(input) == 0 || json.Unmarshal(input, &values) != nil {
		return nil
	}
	if container != "" {
		nested, ok := values[container]
		if !ok || string(nested) == "null" {
			return nil
		}
		values = nil
		if json.Unmarshal(nested, &values) != nil {
			return nil
		}
	}
	for _, field := range []string{"skill_ids", "disabled_skill_ids"} {
		if _, present := values[field]; present {
			return errors.New(
				"Agent options 不能直接修改 skill_ids/disabled_skill_ids；" +
					"必须使用带 target_scope、source_identity 和 runtime_version 的 Skills 操作",
			)
		}
	}
	return nil
}

func validateSelfRuntimeLimit(name string, requested, current *int) error {
	if requested == nil || current == nil || *current <= 0 {
		return nil
	}
	if *requested > *current {
		return fmt.Errorf(
			"普通 Agent 只能收紧 %s，不能从 %d 提高到 %d；提高或解除上限必须由主智能体设置",
			name,
			*current,
			*requested,
		)
	}
	return nil
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
		if len(item.Options.MCPServers) > 0 {
			names := make([]string, 0, len(item.Options.MCPServers))
			for name := range item.Options.MCPServers {
				names = append(names, name)
			}
			slices.Sort(names)
			if _, validationErr := clientopts.MergeAgentMCPServers(nil, item.Options.MCPServers); validationErr != nil {
				checks = append(checks, Check{
					Code: "agent_mcp_servers_invalid", Status: "error",
					Message: validationErr.Error(), Domain: DomainAgents, Target: item.AgentID,
					Remedy: "由主智能体重新 plan/apply Agent options.mcp_servers", Verified: true,
				})
			} else {
				checks = append(checks, Check{
					Code: "agent_mcp_servers_valid", Status: "ok",
					Message: fmt.Sprintf("已校验 %d 个自定义 MCP server: %s", len(names), strings.Join(names, ", ")),
					Domain:  DomainAgents, Target: item.AgentID, Verified: true,
				})
			}
		}
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

type agentUpdatePatch struct {
	Name        *string         `json:"name,omitempty"`
	Options     json.RawMessage `json:"options,omitempty"`
	Avatar      *string         `json:"avatar,omitempty"`
	Description *string         `json:"description,omitempty"`
	VibeTags    []string        `json:"vibe_tags,omitempty"`
}

type agentSelfProfilePatch struct {
	Name        *string  `json:"name,omitempty"`
	Avatar      *string  `json:"avatar,omitempty"`
	Description *string  `json:"description,omitempty"`
	VibeTags    []string `json:"vibe_tags,omitempty"`
}

type agentSelfRuntimePatch struct {
	Provider          *string `json:"provider,omitempty"`
	Model             *string `json:"model,omitempty"`
	MaxTurns          *int    `json:"max_turns,omitempty"`
	MaxThinkingTokens *int    `json:"max_thinking_tokens,omitempty"`
}

func (s *Service) agentUpdateInput(
	ctx context.Context,
	agentID string,
	patch agentUpdatePatch,
) (protocol.UpdateRequest, error) {
	result := protocol.UpdateRequest{
		Name: patch.Name, Avatar: patch.Avatar, Description: patch.Description, VibeTags: patch.VibeTags,
	}
	if len(patch.Options) == 0 || string(patch.Options) == "null" {
		return result, nil
	}
	current, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return protocol.UpdateRequest{}, err
	}
	merged, err := mergeJSONObject(current.Options, patch.Options)
	if err != nil {
		return protocol.UpdateRequest{}, err
	}
	var options protocol.Options
	if err = strictDecodeJSON(merged, &options); err != nil {
		return protocol.UpdateRequest{}, fmt.Errorf("合并 Agent options patch: %w", err)
	}
	result.Options = &options
	return result, nil
}

func (s *Service) agentSelfRuntimeUpdateInput(
	ctx context.Context,
	agentID string,
	patch agentSelfRuntimePatch,
) (protocol.UpdateRequest, error) {
	current, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return protocol.UpdateRequest{}, err
	}
	options := current.Options
	if patch.Provider != nil {
		options.Provider = *patch.Provider
	}
	if patch.Model != nil {
		options.Model = *patch.Model
	}
	if patch.MaxTurns != nil {
		options.MaxTurns = patch.MaxTurns
	}
	if patch.MaxThinkingTokens != nil {
		options.MaxThinkingTokens = patch.MaxThinkingTokens
	}
	return protocol.UpdateRequest{Options: &options}, nil
}

func (s *Service) readAgentsConfiguration(ctx context.Context, actor *resolvedActor, target string) (any, []Check, int64, ScopeRef, error) {
	scope := actor.Context
	agentID := target
	if actor.isSelfDM() {
		agentID = actor.AgentID
	}
	var values []protocol.Agent
	var err error
	if agentID != "" {
		var item *protocol.Agent
		item, err = s.agents.GetAgent(ctx, agentID)
		if item != nil {
			values = []protocol.Agent{*item}
			scope = ScopeRef{Kind: ScopeKindAgent, ID: item.AgentID}
		}
	} else {
		values, err = s.agents.ListAgentRecords(ctx)
	}
	if err != nil {
		return nil, agentChecks(nil, nil, err), 0, scope, err
	}
	providers, providerErr := s.providers.List(ctx)
	var version int64
	if len(values) == 1 {
		version = values[0].RuntimeVersion
	}
	return values, agentChecks(values, providers, providerErr), version, scope, providerErr
}

func (s *Service) validateScopedAgentsChange(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	if actor.Authority != AuthorityAgentSelf ||
		request.Domain != DomainAgents ||
		request.Operation != "update_self_runtime" {
		return nil
	}
	var input agentSelfRuntimePatch
	if err := strictDecodeJSON(request.Input, &input); err != nil {
		return err
	}
	if actor.Agent == nil {
		return errors.New("无法核对当前 Agent runtime 上限")
	}
	if err := validateSelfRuntimeLimit("max_turns", input.MaxTurns, actor.Agent.Options.MaxTurns); err != nil {
		return err
	}
	if err := validateSelfRuntimeLimit(
		"max_thinking_tokens",
		input.MaxThinkingTokens,
		actor.Agent.Options.MaxThinkingTokens,
	); err != nil {
		return err
	}
	if input.Provider == nil {
		return nil
	}
	providerKey := strings.TrimSpace(*input.Provider)
	modelID := strings.TrimSpace(*input.Model)
	if providerKey == "" && modelID == "" {
		return nil
	}
	if providerKey == "" || modelID == "" {
		return errors.New("provider 和 model 必须同时为空或同时配置")
	}
	record, err := s.providers.Get(ctx, providerKey)
	if err != nil {
		return fmt.Errorf("选择 Provider: %w", err)
	}
	if record == nil || !record.Enabled || !record.AgentRuntimeSupported {
		return fmt.Errorf("Provider %s 未启用或不支持 Agent runtime", providerKey)
	}
	for _, model := range record.Models {
		if model.ModelID == modelID && model.Enabled {
			return nil
		}
	}
	return fmt.Errorf("模型 %s/%s 不存在或未启用", providerKey, modelID)
}

func (s *Service) verifyDeletedAgents(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	items, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if strings.TrimSpace(item.AgentID) == request.Target {
			return fmt.Errorf("Agent %s 删除后仍存在", request.Target)
		}
	}

	return nil
}
