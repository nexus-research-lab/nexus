// INPUT: 已通过动态授权、plan digest、revision、人工批准与审计门闩的规范化 ChangeRequest。
// OUTPUT: 领域服务写入、即时权限同步、下一轮重配信号与实时失效通知。
// POS: configuration 控制面到各配置真相源及热生效机制的唯一分派层。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func (s *Service) executeChange(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
	stateVersion int64,
) (any, error) {
	if request.Domain == DomainProviders {
		ctx = privateProviderMutationContext(ctx, actor.Actor)
	}
	decode := func(destination any) error {
		payload := request.Input
		if len(payload) == 0 {
			payload = json.RawMessage(`{}`)
		}
		return json.Unmarshal(payload, destination)
	}
	switch request.Domain + "." + request.Operation {
	case DomainPreferences + ".update":
		var input preferencessvc.UpdateRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Preferences 配置缺少可比较的 state_version")
		}
		return s.updatePreferences(ctx, actor.Actor, input, request.Input, stateVersion)
	case DomainProviders + ".create":
		var input providerCreateRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.providers.Create(ctx, input.serviceInput())
	case DomainProviders + ".update":
		var input providerUpdateRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 更新缺少 configuration_version；请重新 plan")
		}
		return s.providers.PatchAtVersion(ctx, request.Target, input.patchInput(), stateVersion)
	case DomainProviders + ".delete":
		var input providersvc.DeleteInput
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 删除缺少 configuration_version；请重新 plan")
		}
		return s.providers.DeleteAtVersion(ctx, request.Target, input, stateVersion)
	case DomainProviders + ".fetch_models":
		if stateVersion <= 0 {
			return nil, errors.New("Provider 模型同步缺少 configuration_version；请重新 plan")
		}
		return s.providers.FetchModelsAtVersion(ctx, request.Target, stateVersion)
	case DomainProviders + ".update_model":
		var input providerModelMutation
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 模型更新缺少 configuration_version；请重新 plan")
		}
		return s.providers.UpdateModelAtVersion(ctx, request.Target, input.ModelID, input.Input, stateVersion)
	case DomainProviders + ".set_default_model":
		var input providerModelTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 默认模型更新缺少 configuration_version；请重新 plan")
		}
		return s.providers.SetDefaultModelAtVersion(ctx, request.Target, input.ModelID, stateVersion)
	case DomainProviders + ".test_provider":
		if stateVersion <= 0 {
			return nil, errors.New("Provider 测试缺少 configuration_version；请重新 plan")
		}
		return s.providers.TestProviderAtVersion(ctx, request.Target, stateVersion)
	case DomainProviders + ".test_model":
		var input providerModelTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 模型测试缺少 configuration_version；请重新 plan")
		}
		return s.providers.TestModelAtVersion(ctx, request.Target, input.ModelID, stateVersion)
	case DomainAgents + ".create":
		var input protocol.CreateRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.agents.CreateAgent(ctx, input)
	case DomainAgents + ".update":
		var input agentUpdatePatch
		if err := decode(&input); err != nil {
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
	case DomainAgents + ".update_self_profile":
		var input agentSelfProfilePatch
		if err := decode(&input); err != nil {
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
	case DomainAgents + ".update_self_runtime":
		var input agentSelfRuntimePatch
		if err := decode(&input); err != nil {
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
	case DomainAgents + ".delete":
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
	case DomainEmotion + ".set_base":
		if stateVersion <= 0 {
			return nil, errors.New("Emotion 更新缺少 state_version；请重新 plan")
		}
		var input emotionBaseInput
		if err := decode(&input); err != nil {
			return nil, err
		}
		view, err := s.agents.SetAgentRuntimeEmotionBaseAtVersion(
			ctx,
			actor.AgentID,
			agentsvc.RuntimeEmotionBaseUpdate{
				Mood: input.Mood, Energy: input.Energy,
				Valence: input.Valence, Description: input.Description,
			},
			stateVersion,
		)
		return safeEmotionView(view), err
	case DomainEmotion + ".set_context":
		if stateVersion <= 0 {
			return nil, errors.New("Emotion 更新缺少 state_version；请重新 plan")
		}
		contextID, err := trustedEmotionContextID(actor)
		if err != nil {
			return nil, err
		}
		var input emotionContextInput
		if err = decode(&input); err != nil {
			return nil, err
		}
		view, err := s.agents.SetAgentRuntimeEmotionContextAtVersion(
			ctx,
			actor.AgentID,
			agentsvc.RuntimeEmotionContextUpdate{
				ContextID: contextID, Mood: input.Mood,
				Valence: input.Valence, Trigger: input.Trigger,
			},
			stateVersion,
		)
		return safeEmotionView(view), err
	case DomainEmotion + ".clear_context":
		if stateVersion <= 0 {
			return nil, errors.New("Emotion 更新缺少 state_version；请重新 plan")
		}
		contextID, err := trustedEmotionContextID(actor)
		if err != nil {
			return nil, err
		}
		view, err := s.agents.ClearAgentRuntimeEmotionContextAtVersion(
			ctx,
			actor.AgentID,
			contextID,
			stateVersion,
		)
		return safeEmotionView(view), err
	case DomainChannels + ".upsert":
		var input channels.UpsertChannelConfigRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Channel 更新缺少 configuration_version；请重新 plan")
		}
		return s.channels.UpsertChannelConfigAtVersion(
			ctx,
			actor.OwnerUserID,
			request.Target,
			input,
			stateVersion,
		)
	case DomainChannels + ".delete_config":
		if stateVersion <= 0 {
			return nil, errors.New("Channel 删除缺少 configuration_version；请重新 plan")
		}
		return map[string]any{"channel_type": request.Target, "deleted": true},
			s.channels.DeleteChannelConfigAtVersion(ctx, actor.OwnerUserID, request.Target, stateVersion)
	case DomainChannels + ".delete_account":
		var input channelAccountTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Channel account 删除缺少 configuration_version；请重新 plan")
		}
		return s.channels.DeleteChannelAccountAtVersion(
			ctx,
			actor.OwnerUserID,
			request.Target,
			input.AccountID,
			stateVersion,
		)
	case DomainChannels + ".create_pairing":
		var input channels.CreatePairingRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Channel pairing 创建缺少 configuration_version；请重新 plan")
		}
		return s.channels.CreatePairingAtVersion(ctx, actor.OwnerUserID, input, stateVersion)
	case DomainChannels + ".update_pairing":
		var input channels.UpdatePairingRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Channel pairing 更新缺少 configuration_version；请重新 plan")
		}
		return s.channels.UpdatePairingAtVersion(
			ctx,
			actor.OwnerUserID,
			request.Target,
			input,
			stateVersion,
		)
	case DomainChannels + ".delete_pairing":
		if stateVersion <= 0 {
			return nil, errors.New("Channel pairing 删除缺少 configuration_version；请重新 plan")
		}
		return map[string]any{"pairing_id": request.Target, "deleted": true},
			s.channels.DeletePairingAtVersion(ctx, actor.OwnerUserID, request.Target, stateVersion)
	case DomainConnectors + ".connect":
		var input connectorCredentials
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Connector 连接缺少 configuration_version；请重新 plan")
		}
		return s.connectors.ConnectAtVersion(
			ctx,
			actor.OwnerUserID,
			request.Target,
			input.Credentials,
			stateVersion,
		)
	case DomainConnectors + ".disconnect":
		if stateVersion <= 0 {
			return nil, errors.New("Connector 断开缺少 configuration_version；请重新 plan")
		}
		return s.connectors.DisconnectAtVersion(ctx, actor.OwnerUserID, request.Target, stateVersion)
	case DomainConnectors + ".save_oauth_client":
		var input connectorsvc.OAuthClientConfigRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Connector OAuth 配置缺少 configuration_version；请重新 plan")
		}
		return s.connectors.SaveOAuthClientConfigAtVersion(
			ctx,
			actor.OwnerUserID,
			request.Target,
			input,
			stateVersion,
		)
	case DomainConnectors + ".delete_oauth_client":
		if stateVersion <= 0 {
			return nil, errors.New("Connector OAuth 删除缺少 configuration_version；请重新 plan")
		}
		return s.connectors.DeleteOAuthClientConfigAtVersion(
			ctx,
			actor.OwnerUserID,
			request.Target,
			stateVersion,
		)
	case DomainSkills + ".search_external":
		var input skillSearchInput
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.skills.SearchExternalSkills(ctx, input.Query, input.IncludeReadme)
	case DomainSkills + ".preview_external":
		var input skillPreviewInput
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.skills.GetExternalSkillPreview(ctx, input.DetailURL)
	case DomainSkills + ".create_private_source":
		var input skillsvc.CreateExternalSkillSourceRequest
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".import_git":
		var input skillGitImportInput
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".import_url":
		var input skillURLImportInput
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".import_skills_sh":
		var input skillSkillsShImportInput
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".update_source":
		var input skillsvc.ExternalSkillSourceRequest
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".delete_private_source":
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
	case DomainSkills + ".import_private":
		var input skillPrivateImportInput
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".install":
		var input skillAgentTarget
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".uninstall":
		var input skillAgentTarget
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".install_self":
		var input skillAgentTarget
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".uninstall_self":
		var input skillAgentTarget
		if err := decode(&input); err != nil {
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
	case DomainSkills + ".delete":
		if stateVersion <= 0 {
			return nil, errors.New("Skill 删除缺少 catalog_version；请重新 plan")
		}
		err := s.skills.DeleteSkillAtVersion(ctx, request.Target, stateVersion)
		if err == nil || skillsvc.SkillMutationApplied(err) {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return map[string]any{"skill_name": request.Target, "deleted": err == nil || skillsvc.SkillMutationApplied(err)}, err
	case DomainSkills + ".check_updates":
		return s.skills.CheckImportedSkillUpdates(ctx)
	case DomainSkills + ".update_single":
		if stateVersion <= 0 {
			return nil, errors.New("Skill 更新缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.UpdateSingleSkillAtVersion(ctx, request.Target, stateVersion)
		if err == nil || skillsvc.SkillMutationApplied(err) {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case DomainSkills + ".update_all":
		if stateVersion <= 0 {
			return nil, errors.New("Skill 批量更新缺少 catalog_version；请重新 plan")
		}
		result, err := s.skills.UpdateImportedSkillsAtVersion(ctx, stateVersion)
		if err == nil || skillsvc.SkillMutationApplied(err) {
			s.notifySkillCatalogChanged(ctx, actor.AgentID)
		}
		return result, err
	case DomainSessions + ".update_title":
		if s.sessions == nil {
			return nil, errors.New("Sessions 配置服务未装配")
		}
		if stateVersion <= 0 {
			return nil, errors.New("Session 标题更新缺少 configuration_version；请重新 plan")
		}
		var input sessionTitleInput
		if err := decode(&input); err != nil {
			return nil, err
		}
		updated, err := s.sessions.UpdateSessionTitleAtVersion(
			ctx,
			request.Target,
			input.Title,
			stateVersion,
		)
		if err != nil {
			return nil, err
		}
		if updated == nil {
			return nil, errors.New("Session 标题更新未返回可核验结果")
		}
		return safeSessionTitleChangeResult(*updated), nil
	case DomainSessions + ".delete":
		if s.sessions == nil {
			return nil, errors.New("Sessions 配置服务未装配")
		}
		if stateVersion <= 0 {
			return nil, errors.New("Session 删除缺少 configuration_version；请重新 plan")
		}
		err := s.sessions.DeleteSessionAtVersion(ctx, request.Target, stateVersion)
		return map[string]any{
			"session_key": request.Target,
			"deleted":     err == nil || sessionsvc.SessionDeletionCommitted(err),
		}, err
	case DomainRooms + ".create":
		var input protocol.CreateRoomRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		value, err := s.rooms.CreateRoom(ctx, input)
		if err == nil {
			s.notifyRoomChanged(ctx, value, "room_created")
		}
		return value, err
	case DomainRooms + ".create_conversation":
		var input protocol.CreateConversationRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Conversation 创建缺少 Room configuration_version；请重新 plan")
		}
		value, err := s.rooms.CreateConversationAtVersion(ctx, request.Target, input, stateVersion)
		if err == nil {
			s.notifyRoomChanged(ctx, value, "room_conversation_created")
		}
		return value, err
	case DomainRooms + ".update_conversation":
		var input roomConversationTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Conversation 更新缺少 Room configuration_version；请重新 plan")
		}
		value, err := s.rooms.UpdateConversationAtVersion(
			ctx,
			request.Target,
			input.ConversationID,
			protocol.UpdateConversationRequest{Title: input.Title},
			stateVersion,
		)
		if err == nil {
			s.notifyRoomChanged(ctx, value, "room_conversation_updated")
		}
		return value, err
	case DomainRooms + ".delete_conversation":
		var input roomConversationTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Conversation 删除缺少 Room configuration_version；请重新 plan")
		}
		value, err := s.rooms.DeleteConversationAtVersion(
			ctx,
			request.Target,
			input.ConversationID,
			stateVersion,
		)
		if err == nil || roomsvc.ConversationDeletionCommitted(err) {
			s.notifyConversationChanged(
				ctx,
				request.Target,
				input.ConversationID,
				"room_conversation_deleted",
			)
		}
		return value, err
	case DomainRooms + ".update_profile":
		var input roomProfilePatch
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Room profile 更新缺少 configuration_version；请重新 plan")
		}
		update := roomProfileUpdateRequest(input)
		update.ExpectedConfigurationVersion = &stateVersion
		value, err := s.rooms.UpdateRoom(ctx, request.Target, update)
		if err == nil {
			s.notifyRoomChanged(ctx, value, "room_configuration_profile_updated")
		}
		return value, err
	case DomainRooms + ".set_collaboration_policy":
		var input roomCollaborationPolicyPatch
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Room 协作策略更新缺少 configuration_version；请重新 plan")
		}
		update := roomPolicyUpdateRequest(input)
		update.ExpectedConfigurationVersion = &stateVersion
		value, err := s.rooms.UpdateRoom(ctx, request.Target, update)
		if err == nil {
			s.notifyRoomChanged(ctx, value, "room_collaboration_policy_updated")
		}
		return value, err
	case DomainRooms + ".add_member":
		var input roomAgentTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Room 添加成员缺少 configuration_version；请重新 plan")
		}
		value, err := s.rooms.AddRoomMemberAtVersion(
			ctx, request.Target, protocol.AddRoomMemberRequest{AgentID: input.AgentID}, stateVersion,
		)
		if err == nil {
			s.notifyRoomMemberChanged(ctx, request.Target, input.AgentID, true)
		}
		return value, err
	case DomainRooms + ".remove_member":
		var input roomAgentTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		input = input.Normalized()
		if stateVersion <= 0 {
			return nil, errors.New("Room 移除成员缺少 configuration_version；请重新 plan")
		}
		current, err := s.rooms.GetRoom(ctx, request.Target)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(current.Room.HostAgentID) == input.AgentID {
			return nil, errors.New("不能直接移除当前群主；请先用 transfer_host 指定继任者")
		}
		value, err := s.rooms.RemoveRoomMemberAtVersion(ctx, request.Target, input.AgentID, stateVersion)
		if value != nil {
			s.notifyRoomMemberChanged(ctx, request.Target, input.AgentID, false)
			if s.roomRuntime != nil {
				interruptErr := s.roomRuntime.InterruptAgentTasks(
					ctx,
					request.Target,
					input.AgentID,
					"成员配置权限已撤销",
				)
				err = errors.Join(err, interruptErr)
			}
		}
		return value, err
	case DomainRooms + ".set_member_participation":
		var input roomMemberParticipationInput
		if err := decode(&input); err != nil {
			return nil, err
		}
		input = input.Normalized()
		if input.Paused == nil {
			return nil, errors.New("Room 成员参与状态缺少 paused")
		}
		if stateVersion <= 0 {
			return nil, errors.New("Room 成员参与状态更新缺少 configuration_version；请重新 plan")
		}
		controller, ok := s.roomRuntime.(roomParticipationController)
		if !ok {
			return nil, errors.New("Room 成员参与状态实时控制未装配")
		}
		value, err := controller.SetRoomMemberParticipationAtVersion(
			ctx,
			request.Target,
			input.AgentID,
			*input.Paused,
			stateVersion,
		)
		if err == nil {
			s.notifyRoomChanged(ctx, value, "room_member_participation_updated")
		}
		return value, err
	case DomainRooms + ".transfer_host":
		var input roomAgentTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		input = input.Normalized()
		if stateVersion <= 0 {
			return nil, errors.New("Room 群主转让缺少 configuration_version；请重新 plan")
		}
		update := protocol.UpdateRoomRequest{HostAgentID: &input.AgentID}
		update.ExpectedConfigurationVersion = &stateVersion
		value, err := s.rooms.UpdateRoom(ctx, request.Target, update)
		if err == nil {
			s.notifyRoomChanged(ctx, value, "room_host_transferred")
		}
		return value, err
	case DomainRooms + ".delete":
		if stateVersion <= 0 {
			return nil, errors.New("Room 删除缺少 configuration_version；请重新 plan")
		}
		err := s.rooms.DeleteRoomAtVersion(ctx, request.Target, stateVersion)
		committed := err == nil || roomsvc.RoomDeletionCommitted(err)
		if committed {
			s.notifyRoomDeleted(ctx, request.Target)
		}
		return map[string]any{
			"room_id": request.Target,
			"deleted": committed,
		}, err
	default:
		return nil, fmt.Errorf("不支持配置操作 %s.%s", request.Domain, request.Operation)
	}
}

func inputContainsField(input json.RawMessage, field string) bool {
	if len(input) == 0 {
		return false
	}
	var fields map[string]any
	if json.Unmarshal(input, &fields) != nil {
		return false
	}
	_, ok := fields[field]
	return ok
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

func (s *Service) notifySkillCatalogChanged(ctx context.Context, agentID string) {
	if s.notifier != nil {
		s.notifier.AgentChanged(ctx, agentID, "skill_catalog_updated")
	}
}

func (s *Service) notifyAgentDeleted(ctx context.Context, agentID string) {
	if s.notifier != nil {
		s.notifier.AgentChanged(ctx, agentID, "agent_deleted")
	}
}

func (s *Service) notifyRoomChanged(
	ctx context.Context,
	value *protocol.ConversationContextAggregate,
	reason string,
) {
	if s.notifier == nil || value == nil {
		return
	}
	s.notifier.RoomChanged(ctx, value.Room.ID, value.Conversation.ID, reason)
}

func (s *Service) notifyConversationChanged(ctx context.Context, roomID, conversationID, reason string) {
	if s.notifier != nil {
		s.notifier.RoomChanged(ctx, roomID, conversationID, reason)
	}
}

func (s *Service) notifyRoomMemberChanged(ctx context.Context, roomID, agentID string, added bool) {
	if s.notifier != nil {
		s.notifier.RoomMemberChanged(ctx, roomID, agentID, added)
	}
}

func (s *Service) notifyRoomDeleted(ctx context.Context, roomID string) {
	if s.notifier != nil {
		s.notifier.RoomChanged(ctx, roomID, "", "room_deleted")
	}
}
