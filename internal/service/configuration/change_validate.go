// INPUT: 已规范化的 domain/operation/target/input。
// OUTPUT: 严格字段、必填字段、目标与主机路径预检结果。
// POS: configuration 写入前的纯校验阶段。
package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func requireInputFields(input json.RawMessage, required []string) error {
	if len(required) == 0 {
		return nil
	}
	payload := input
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	var values map[string]any
	if err := json.Unmarshal(payload, &values); err != nil {
		return fmt.Errorf("input 无效: %w", err)
	}
	missing := make([]string, 0)
	for _, field := range required {
		if _, ok := values[field]; !ok {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("input 缺少必填字段: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateChangeRequest(request ChangeRequest) error {
	target := strings.TrimSpace(request.Target)
	requireTarget := func() error {
		if target == "" {
			return fmt.Errorf("%s.%s 要求 target", request.Domain, request.Operation)
		}
		return nil
	}
	decode := func(destination any) error {
		if len(request.Input) == 0 {
			return strictDecodeJSON([]byte(`{}`), destination)
		}
		if !json.Valid(request.Input) {
			return errors.New("input 必须是 JSON object")
		}
		if err := strictDecodeJSON(request.Input, destination); err != nil {
			return fmt.Errorf("input 无效: %w", err)
		}
		return nil
	}
	switch request.Domain + "." + request.Operation {
	case DomainPreferences + ".update":
		if err := requireNonEmptyJSONObject(request.Input, "preferences.update"); err != nil {
			return err
		}
		var input preferencessvc.UpdateRequest
		if err := decode(&input); err != nil {
			return err
		}
		if input.DefaultAgentOptions != nil {
			if err := rejectScopedSkillOptionFields(request.Input, "default_agent_options"); err != nil {
				return err
			}
			if err := validateConfigurationAgentOptions(*input.DefaultAgentOptions); err != nil {
				return fmt.Errorf("default_agent_options 无效: %w", err)
			}
		}
		return nil
	case DomainProviders + ".create":
		var input providerCreateRequest
		if err := decode(&input); err != nil {
			return err
		}
		switch strings.TrimSpace(input.Visibility) {
		case "", "private":
			return nil
		default:
			return errors.New("对话配置只能创建当前 owner 的私有 Provider；公共订阅 Provider 必须由人类运营界面管理")
		}
	case DomainProviders + ".update":
		if err := requireTarget(); err != nil {
			return err
		}
		var input providerUpdateRequest
		if err := decode(&input); err != nil {
			return err
		}
		if input.ProviderKind == nil && input.PresetKey == nil && input.APIFormat == nil &&
			input.DisplayName == nil && input.AuthToken == nil && input.BaseURL == nil &&
			input.ModelsPath == nil && input.Enabled == nil {
			return errors.New("providers.update 至少要提供一个待修改字段")
		}
		return nil
	case DomainProviders + ".delete":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&providersvc.DeleteInput{})
	case DomainProviders + ".fetch_models", DomainProviders + ".test_provider":
		return requireTarget()
	case DomainProviders + ".update_model":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&providerModelMutation{})
	case DomainProviders + ".set_default_model", DomainProviders + ".test_model":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&providerModelTarget{})
	case DomainAgents + ".create":
		var input protocol.CreateRequest
		if err := decode(&input); err != nil {
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
	case DomainAgents + ".update":
		if err := requireTarget(); err != nil {
			return err
		}
		var input agentUpdatePatch
		if err := decode(&input); err != nil {
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
	case DomainAgents + ".update_self_profile":
		var input agentSelfProfilePatch
		if err := decode(&input); err != nil {
			return err
		}
		if input.Name == nil && input.Avatar == nil && input.Description == nil && input.VibeTags == nil {
			return errors.New("agents.update_self_profile 至少要提供一个待修改字段")
		}
		return nil
	case DomainAgents + ".update_self_runtime":
		var input agentSelfRuntimePatch
		if err := decode(&input); err != nil {
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
	case DomainAgents + ".delete":
		return requireTarget()
	case DomainEmotion + ".set_base":
		var input emotionBaseInput
		if err := decode(&input); err != nil {
			return err
		}
		if err := validateEmotionText("mood", input.Mood); err != nil {
			return err
		}
		if err := validateEmotionScore("energy", input.Energy); err != nil {
			return err
		}
		if err := validateEmotionScore("valence", input.Valence); err != nil {
			return err
		}
		return validateEmotionText("description", input.Description)
	case DomainEmotion + ".set_context":
		var input emotionContextInput
		if err := decode(&input); err != nil {
			return err
		}
		if err := validateEmotionText("mood", input.Mood); err != nil {
			return err
		}
		if err := validateEmotionScore("valence", input.Valence); err != nil {
			return err
		}
		return validateEmotionText("trigger", input.Trigger)
	case DomainEmotion + ".clear_context":
		return decode(&struct{}{})
	case DomainChannels + ".upsert":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&channels.UpsertChannelConfigRequest{})
	case DomainChannels + ".delete_config":
		return requireTarget()
	case DomainChannels + ".delete_account":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&channelAccountTarget{})
	case DomainChannels + ".create_pairing":
		return decode(&channels.CreatePairingRequest{})
	case DomainChannels + ".update_pairing":
		if err := requireTarget(); err != nil {
			return err
		}
		var input channels.UpdatePairingRequest
		if err := decode(&input); err != nil {
			return err
		}
		if input.AgentID == nil && input.Status == nil && input.ExternalName == nil {
			return errors.New("channels.update_pairing 至少要提供一个待修改字段")
		}
		return nil
	case DomainChannels + ".delete_pairing":
		return requireTarget()
	case DomainConnectors + ".connect":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&connectorCredentials{})
	case DomainConnectors + ".disconnect", DomainConnectors + ".delete_oauth_client":
		return requireTarget()
	case DomainConnectors + ".save_oauth_client":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&connectorsvc.OAuthClientConfigRequest{})
	case DomainSkills + ".search_external":
		if target != "" {
			return errors.New("skills.search_external 不接受 target")
		}
		var input skillSearchInput
		if err := decode(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.Query) == "" {
			return errors.New("skills.search_external 的 query 不能为空")
		}
		return nil
	case DomainSkills + ".preview_external":
		if target != "" {
			return errors.New("skills.preview_external 不接受 target")
		}
		var input skillPreviewInput
		if err := decode(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.DetailURL) == "" {
			return errors.New("skills.preview_external 的 detail_url 不能为空")
		}
		return nil
	case DomainSkills + ".create_private_source":
		if target != "" {
			return errors.New("skills.create_private_source 不接受 target")
		}
		var input skillPrivateSourceCreateInput
		if err := decode(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.URL) == "" {
			return errors.New("skills.create_private_source 要求 name 和 url")
		}
		return validatePrivateSkillSourceAuth(input.AuthType, input.Token)
	case DomainSkills + ".import_git":
		if target != "" {
			return errors.New("skills.import_git 不接受 target")
		}
		var input skillGitImportInput
		if err := decode(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.RepositoryURL) == "" {
			return errors.New("skills.import_git 的 repository_url 不能为空")
		}
		return nil
	case DomainSkills + ".import_url":
		if target != "" {
			return errors.New("skills.import_url 不接受 target")
		}
		var input skillURLImportInput
		if err := decode(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.SourceURL) == "" {
			return errors.New("skills.import_url 的 source_url 不能为空")
		}
		return nil
	case DomainSkills + ".import_skills_sh":
		if target != "" {
			return errors.New("skills.import_skills_sh 不接受 target")
		}
		var input skillSkillsShImportInput
		if err := decode(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.PackageSpec) == "" ||
			strings.TrimSpace(input.SkillSlug) == "" {
			return errors.New("skills.import_skills_sh 要求 package_spec 和 skill_slug")
		}
		return nil
	case DomainSkills + ".update_source":
		if err := requireTarget(); err != nil {
			return err
		}
		var input skillSourceUpdateInput
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".delete_private_source":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&struct{}{})
	case DomainSkills + ".import_private":
		if err := requireTarget(); err != nil {
			return err
		}
		var input skillPrivateImportInput
		if err := decode(&input); err != nil {
			return err
		}
		if strings.TrimSpace(input.SkillID) == "" {
			return errors.New("skills.import_private 要求 skill_id")
		}
		return nil
	case DomainSkills + ".install", DomainSkills + ".uninstall":
		if err := requireTarget(); err != nil {
			return err
		}
		var input skillAgentTarget
		if err := decode(&input); err != nil {
			return err
		}
		return validateSkillSelectionInput(input, true)
	case DomainSkills + ".install_self", DomainSkills + ".uninstall_self":
		if err := requireTarget(); err != nil {
			return err
		}
		var input skillAgentTarget
		if err := decode(&input); err != nil {
			return err
		}
		return validateSkillSelectionInput(input, false)
	case DomainSkills + ".delete", DomainSkills + ".update_single":
		return requireTarget()
	case DomainSkills + ".check_updates", DomainSkills + ".update_all":
		if target != "" {
			return fmt.Errorf("skills.%s 不接受 target", request.Operation)
		}
		return decode(&struct{}{})
	case DomainSessions + ".update_title":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&sessionTitleInput{})
	case DomainSessions + ".delete":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&struct{}{})
	case DomainRooms + ".create":
		if target != "" {
			return errors.New("rooms.create 不能指定现有 room_id")
		}
		var input protocol.CreateRoomRequest
		if err := decode(&input); err != nil {
			return err
		}
		if len(input.AgentIDs) == 0 {
			return errors.New("rooms.create 至少需要一个 agent_id")
		}
		seenAgentIDs := make(map[string]struct{}, len(input.AgentIDs))
		for _, rawAgentID := range input.AgentIDs {
			agentID := strings.TrimSpace(rawAgentID)
			if agentID == "" {
				return errors.New("rooms.create 的 agent_id 不能为空")
			}
			if _, exists := seenAgentIDs[agentID]; exists {
				return fmt.Errorf("rooms.create 的 agent_id %s 重复", agentID)
			}
			seenAgentIDs[agentID] = struct{}{}
		}
		hostAgentID := strings.TrimSpace(input.HostAgentID)
		if input.HostAutoReplyEnabled && hostAgentID == "" {
			return errors.New("启用群主接管时必须设置 host_agent_id")
		}
		if hostAgentID != "" {
			if _, exists := seenAgentIDs[hostAgentID]; !exists {
				return errors.New("host_agent_id 必须属于 agent_ids")
			}
		}
		return nil
	case DomainRooms + ".update_profile":
		var input roomProfilePatch
		if err := decode(&input); err != nil {
			return err
		}
		if input.Name == nil && input.Description == nil && input.Avatar == nil {
			return errors.New("rooms.update_profile 至少要提供一个待修改字段")
		}
		return nil
	case DomainRooms + ".set_collaboration_policy":
		var input roomCollaborationPolicyPatch
		if err := decode(&input); err != nil {
			return err
		}
		if input.SkillNames == nil && input.HostAutoReplyEnabled == nil && input.PrivateMessagesEnabled == nil {
			return errors.New("rooms.set_collaboration_policy 至少要提供一个待修改字段")
		}
		return nil
	case DomainRooms + ".add_member", DomainRooms + ".remove_member", DomainRooms + ".transfer_host":
		var input roomAgentTarget
		if err := decode(&input); err != nil {
			return err
		}
		if input.Normalized().AgentID == "" {
			return errors.New("agent_id 不能为空")
		}
		return nil
	case DomainRooms + ".set_member_participation":
		var input roomMemberParticipationInput
		if err := decode(&input); err != nil {
			return err
		}
		if input.Normalized().AgentID == "" {
			return errors.New("agent_id 不能为空")
		}
		if input.Paused == nil {
			return errors.New("paused 必须显式提供")
		}
		return nil
	case DomainRooms + ".create_conversation":
		if err := requireTarget(); err != nil {
			return err
		}
		var input protocol.CreateConversationRequest
		return decode(&input)
	case DomainRooms + ".update_conversation":
		if err := requireTarget(); err != nil {
			return err
		}
		var input roomConversationTarget
		if err := decode(&input); err != nil {
			return err
		}
		if input.Normalized().ConversationID == "" {
			return errors.New("conversation_id 不能为空")
		}
		if input.Normalized().Title == "" {
			return errors.New("title 不能为空")
		}
		return nil
	case DomainRooms + ".delete_conversation":
		if err := requireTarget(); err != nil {
			return err
		}
		var input roomConversationTarget
		if err := decode(&input); err != nil {
			return err
		}
		if input.Normalized().ConversationID == "" {
			return errors.New("conversation_id 不能为空")
		}
		if input.Normalized().Title != "" {
			return errors.New("rooms.delete_conversation 不接受 title")
		}
		return nil
	case DomainRooms + ".delete":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&struct{}{})
	default:
		return fmt.Errorf("不支持配置操作 %s.%s", request.Domain, request.Operation)
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

func jsonFieldProvided(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func requireNonEmptyJSONObject(input json.RawMessage, operation string) error {
	var fields map[string]json.RawMessage
	payload := input
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(payload, &fields); err != nil {
		return fmt.Errorf("input 无效: %w", err)
	}
	if len(fields) == 0 {
		return fmt.Errorf("%s 至少要提供一个待修改字段", operation)
	}
	return nil
}

func (s *Service) validateScopedChange(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
) error {
	if request.Domain == DomainProviders {
		if request.Operation == "create" {
			return nil
		}
		if _, err := s.providers.GetPrivate(ctx, request.Target); err != nil {
			return fmt.Errorf(
				"对话配置只能管理当前 owner 的私有 Provider；公共订阅 Provider 必须由人类运营界面管理: %w",
				err,
			)
		}
		return nil
	}
	if request.Domain == DomainRooms {
		switch request.Operation {
		case "create_conversation":
			contexts, err := s.rooms.GetRoomContexts(ctx, request.Target)
			if err != nil {
				return fmt.Errorf("核对 Room conversation: %w", err)
			}
			for _, contextValue := range contexts {
				if contextValue.Conversation.IsDraft {
					return errors.New("当前 Room 已有尚未开始的 draft conversation；请复用或先发送消息，不能重复创建")
				}
			}
		case "update_conversation", "delete_conversation":
			var input roomConversationTarget
			if err := strictDecodeJSON(request.Input, &input); err != nil {
				return err
			}
			contextValue, err := s.rooms.GetConversationContext(ctx, input.ConversationID)
			if err != nil {
				return fmt.Errorf("核对 conversation: %w", err)
			}
			if contextValue == nil || strings.TrimSpace(contextValue.Room.ID) != request.Target {
				return errors.New("conversation_id 不属于目标 Room")
			}
		case "set_member_participation":
			var input roomMemberParticipationInput
			if err := strictDecodeJSON(request.Input, &input); err != nil {
				return err
			}
			input = input.Normalized()
			roomValue, err := s.rooms.GetRoom(ctx, request.Target)
			if err != nil {
				return fmt.Errorf("核对 Room 成员参与状态: %w", err)
			}
			memberID := input.AgentID
			memberFound := false
			for _, member := range roomValue.Members {
				if member.MemberType == protocol.MemberTypeAgent &&
					strings.TrimSpace(member.MemberAgentID) == memberID {
					memberFound = true
					break
				}
			}
			if !memberFound {
				return errors.New("agent_id 不是当前 Room 成员")
			}
			if actor.Authority == AuthorityRoomHost && memberID == actor.AgentID &&
				input.Paused != nil && *input.Paused {
				return errors.New("群主不能在 Room 对话中暂停自己；请先转让群主或由主智能体管理")
			}
		}
	}
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

func strictDecodeJSON(payload []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("input 只能包含一个 JSON object")
		}
		return err
	}
	return nil
}
