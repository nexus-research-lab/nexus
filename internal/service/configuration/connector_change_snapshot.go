// INPUT: 已授权的 Connector 变更、owner scope 与 Connector 配置状态。
// OUTPUT: 绑定目标 Connector 单调版本的 revision、Values、Checks 与写后结果证明。
// POS: Connector 对话配置避免秘密轮换不可见和并发连接状态复活的 CAS 读取层。
package configuration

import (
	"context"
	"fmt"

	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

func (s *Service) augmentConnectorChangeSnapshot(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
	snapshot DomainSnapshot,
	verifyOutcome bool,
) (DomainSnapshot, error) {
	if request.Domain != DomainConnectors {
		return snapshot, nil
	}
	state, err := s.connectors.GetConfigurationState(ctx, actor.OwnerUserID, request.Target)
	if err != nil {
		return DomainSnapshot{}, fmt.Errorf("读取 Connector 目标状态: %w", err)
	}
	detail, err := s.connectors.GetConnectorDetail(ctx, actor.OwnerUserID, request.Target)
	if err != nil {
		return DomainSnapshot{}, fmt.Errorf("读取 Connector 目标定义: %w", err)
	}
	if verifyOutcome {
		if err = verifyConnectorTargetOutcome(request, *state); err != nil {
			return snapshot, err
		}
	}
	key, err := s.integrityKeyBytes()
	if err != nil {
		return DomainSnapshot{}, fmt.Errorf("初始化 Connector revision 密钥: %w", err)
	}
	snapshot.Revision, err = integrityRevisionFor(map[string]any{
		"target_definition": detail,
		"target_connector":  state,
	}, key)
	if err != nil {
		return DomainSnapshot{}, err
	}
	snapshot.Scope = ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}
	snapshot.StateVersion = state.ConfigurationVersion
	snapshot.Values = map[string]any{
		"catalog":          snapshot.Values,
		"target_connector": state,
	}
	snapshot.Checks = append(snapshot.Checks, Check{
		Code:   "connector_target_state_readable",
		Status: "ok",
		Message: fmt.Sprintf(
			"已绑定 Connector %s configuration_version=%d，connection=%s，oauth_client_configured=%t",
			state.ConnectorID,
			state.ConfigurationVersion,
			state.ConnectionState,
			state.OAuthClientConfigured,
		),
		Domain:   DomainConnectors,
		Target:   state.ConnectorID,
		Verified: true,
	})
	if verifyOutcome {
		snapshot.Checks = append(snapshot.Checks, Check{
			Code:     "connector_target_outcome_verified",
			Status:   "ok",
			Message:  fmt.Sprintf("已从持久化真相源核对 Connector %s 的目标状态", state.ConnectorID),
			Domain:   DomainConnectors,
			Target:   state.ConnectorID,
			Verified: true,
		})
	}
	return snapshot, nil
}

func verifyConnectorTargetOutcome(request ChangeRequest, state connectorsvc.ConfigurationState) error {
	switch request.Operation {
	case "connect":
		if state.ConnectionState != "connected" {
			return fmt.Errorf(
				"Connector 写后核对失败：%s expected connection_state=connected, actual=%q",
				state.ConnectorID,
				state.ConnectionState,
			)
		}
	case "disconnect":
		if state.ConnectionState != "disconnected" || state.ConnectionConfigured {
			return fmt.Errorf(
				"Connector 写后核对失败：%s expected disconnected and credentials cleared, state=%q credentials=%t",
				state.ConnectorID,
				state.ConnectionState,
				state.ConnectionConfigured,
			)
		}
	case "save_oauth_client":
		if !state.OAuthClientExists || !state.OAuthClientConfigured {
			return fmt.Errorf(
				"Connector 写后核对失败：%s expected OAuth client configured",
				state.ConnectorID,
			)
		}
	case "delete_oauth_client":
		if state.OAuthClientExists || state.OAuthClientConfigured ||
			state.ConnectionState != "disconnected" || state.ConnectionConfigured {
			return fmt.Errorf(
				"Connector 写后核对失败：%s expected OAuth client removed and connection disconnected",
				state.ConnectorID,
			)
		}
	}
	return nil
}
