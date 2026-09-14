// INPUT: Channel 配置请求与可信 owner。
// OUTPUT: 通道配置、账号和配对变更及删除核验。
// POS: Channel 控制面输入、读取、校验与写入。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

func validateChannelsChange(request ChangeRequest) error {
	switch request.Operation {
	case "upsert":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&channels.UpsertChannelConfigRequest{})
	case "delete_config":
		return request.requireTarget()
	case "delete_account":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&channelAccountTarget{})
	case "create_pairing":
		return request.decodeInput(&channels.CreatePairingRequest{})
	case "update_pairing":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input channels.UpdatePairingRequest
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.AgentID == nil && input.Status == nil && input.ExternalName == nil {
			return errors.New("channels.update_pairing 至少要提供一个待修改字段")
		}
		return nil
	case "delete_pairing":
		return request.requireTarget()
	default:
		return unsupportedChange(request)
	}
}

func (s *Service) executeChannelsChange(ctx context.Context, actor *resolvedActor, request ChangeRequest, stateVersion int64) (any, error) {
	switch request.Operation {
	case "upsert":
		var input channels.UpsertChannelConfigRequest
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "delete_config":
		if stateVersion <= 0 {
			return nil, errors.New("Channel 删除缺少 configuration_version；请重新 plan")
		}
		return map[string]any{"channel_type": request.Target, "deleted": true},
			s.channels.DeleteChannelConfigAtVersion(ctx, actor.OwnerUserID, request.Target, stateVersion)
	case "delete_account":
		var input channelAccountTarget
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "create_pairing":
		var input channels.CreatePairingRequest
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Channel pairing 创建缺少 configuration_version；请重新 plan")
		}
		return s.channels.CreatePairingAtVersion(ctx, actor.OwnerUserID, input, stateVersion)
	case "update_pairing":
		var input channels.UpdatePairingRequest
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "delete_pairing":
		if stateVersion <= 0 {
			return nil, errors.New("Channel pairing 删除缺少 configuration_version；请重新 plan")
		}
		return map[string]any{"pairing_id": request.Target, "deleted": true},
			s.channels.DeletePairingAtVersion(ctx, actor.OwnerUserID, request.Target, stateVersion)
	default:
		return nil, unsupportedChange(request)
	}
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

type channelAccountTarget struct {
	AccountID string `json:"account_id"`
}

func (s *Service) readChannelsConfiguration(ctx context.Context, actor *resolvedActor, target string) (any, []Check, int64, ScopeRef, error) {
	scope := actor.Context
	version, err := s.channels.GetChannelControlVersion(ctx, actor.OwnerUserID)
	if err != nil {
		return nil, nil, 0, scope, err
	}
	values, err := s.channels.ListChannels(ctx, actor.OwnerUserID)
	if err != nil {
		return nil, nil, 0, scope, err
	}
	pairings, err := s.channels.ListPairings(ctx, actor.OwnerUserID, channels.PairingQuery{})
	return map[string]any{
		"configuration_version": version,
		"items":                 values,
		"pairings":              pairings,
	}, channelChecks(values, err), version, scope, err
}

func (s *Service) verifyDeletedChannels(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	if actor == nil {
		return fmt.Errorf("Channels 删除核验缺少可信 actor")
	}
	switch request.Operation {
	case "delete_config":
		exists, err := s.channels.HasChannelConfig(ctx, actor.OwnerUserID, request.Target)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("Channel %s 配置删除后仍存在", request.Target)
		}
		hasAccounts, err := s.channels.HasAnyChannelAccount(ctx, actor.OwnerUserID, request.Target)
		if err != nil {
			return err
		}
		if hasAccounts {
			return fmt.Errorf("Channel %s 配置删除后仍残留 account", request.Target)
		}
	case "delete_account":
		var target channelAccountTarget
		if err := json.Unmarshal(request.Input, &target); err != nil {
			return fmt.Errorf("解析 Channel account 删除目标: %w", err)
		}
		exists, err := s.channels.HasChannelAccount(ctx, actor.OwnerUserID, request.Target, target.AccountID)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("Channel %s account %s 删除后仍存在", request.Target, target.AccountID)
		}
	case "delete_pairing":
		exists, err := s.channels.HasPairing(ctx, actor.OwnerUserID, request.Target)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("Pairing %s 删除后仍存在", request.Target)
		}
	}
	return nil
}
