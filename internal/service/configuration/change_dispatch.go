// INPUT: 已通过 plan、revision、confirm 与审计门闩的规范化 ChangeRequest。
// OUTPUT: 现有领域服务的原生写入结果。
// POS: configuration 控制面到领域服务的唯一分派层。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func (s *Service) executeChange(ctx context.Context, actor Actor, request ChangeRequest) (any, error) {
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
		return s.updatePreferences(ctx, actor, input, request.Input)
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
		current, err := s.providers.Get(ctx, request.Target)
		if err != nil {
			return nil, err
		}
		return s.providers.Update(ctx, request.Target, input.serviceInput(*current))
	case DomainProviders + ".delete":
		var input providersvc.DeleteInput
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.providers.Delete(ctx, request.Target, input)
	case DomainProviders + ".fetch_models":
		return s.providers.FetchModels(ctx, request.Target)
	case DomainProviders + ".update_model":
		var input providerModelMutation
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.providers.UpdateModel(ctx, request.Target, input.ModelID, input.Input)
	case DomainProviders + ".set_default_model":
		var input providerModelTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.providers.SetDefaultModel(ctx, request.Target, input.ModelID)
	case DomainProviders + ".test_provider":
		return s.providers.TestProvider(ctx, request.Target)
	case DomainProviders + ".test_model":
		var input providerModelTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.providers.TestModel(ctx, request.Target, input.ModelID)
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
		return s.agents.UpdateAgent(ctx, request.Target, serviceInput)
	case DomainAgents + ".delete":
		if request.Target == actor.AgentID {
			return nil, errors.New("主智能体不能通过配置控制面删除自己")
		}
		return map[string]any{"agent_id": request.Target, "deleted": true}, s.agents.DeleteAgent(ctx, request.Target)
	case DomainChannels + ".upsert":
		var input channels.UpsertChannelConfigRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.channels.UpsertChannelConfig(ctx, actor.OwnerUserID, request.Target, input)
	case DomainChannels + ".delete_config":
		return map[string]any{"channel_type": request.Target, "deleted": true},
			s.channels.DeleteChannelConfig(ctx, actor.OwnerUserID, request.Target)
	case DomainChannels + ".delete_account":
		var input channelAccountTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.channels.DeleteChannelAccount(ctx, actor.OwnerUserID, request.Target, input.AccountID)
	case DomainChannels + ".create_pairing":
		var input channels.CreatePairingRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.channels.CreatePairing(ctx, actor.OwnerUserID, input)
	case DomainChannels + ".update_pairing":
		var input channels.UpdatePairingRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.channels.UpdatePairing(ctx, actor.OwnerUserID, request.Target, input)
	case DomainChannels + ".delete_pairing":
		return map[string]any{"pairing_id": request.Target, "deleted": true},
			s.channels.DeletePairing(ctx, actor.OwnerUserID, request.Target)
	case DomainConnectors + ".connect":
		var input connectorCredentials
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.connectors.Connect(ctx, actor.OwnerUserID, request.Target, input.Credentials)
	case DomainConnectors + ".disconnect":
		return s.connectors.Disconnect(ctx, actor.OwnerUserID, request.Target)
	case DomainConnectors + ".save_oauth_client":
		var input connectorsvc.OAuthClientConfigRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.connectors.SaveOAuthClientConfig(ctx, actor.OwnerUserID, request.Target, input)
	case DomainConnectors + ".delete_oauth_client":
		return s.connectors.DeleteOAuthClientConfig(ctx, actor.OwnerUserID, request.Target)
	case DomainSkills + ".update_source":
		var input skillsvc.ExternalSkillSourceRequest
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.skills.UpdateExternalSkillSource(ctx, request.Target, input)
	case DomainSkills + ".install":
		var input skillAgentTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		return s.skills.InstallSkill(ctx, input.AgentID, request.Target)
	case DomainSkills + ".uninstall":
		var input skillAgentTarget
		if err := decode(&input); err != nil {
			return nil, err
		}
		return map[string]any{"agent_id": input.AgentID, "skill_name": request.Target, "uninstalled": true},
			s.skills.UninstallSkill(ctx, input.AgentID, request.Target)
	case DomainSkills + ".delete":
		return map[string]any{"skill_name": request.Target, "deleted": true}, s.skills.DeleteSkill(ctx, request.Target)
	case DomainSkills + ".check_updates":
		return s.skills.CheckImportedSkillUpdates(ctx)
	case DomainSkills + ".update_imported":
		return s.skills.UpdateImportedSkills(ctx)
	default:
		return nil, fmt.Errorf("不支持配置操作 %s.%s", request.Domain, request.Operation)
	}
}
