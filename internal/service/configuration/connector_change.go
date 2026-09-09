// INPUT: Connector 配置请求与可信 owner。
// OUTPUT: 连接、断开、OAuth 配置与健康检查。
// POS: Connector 配置操作的同域实现。
package configuration

import (
	"context"
	"errors"
	"strings"

	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

func validateConnectorsChange(request ChangeRequest) error {
	switch request.Operation {
	case "connect":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&connectorCredentials{})
	case "disconnect", "delete_oauth_client":
		return request.requireTarget()
	case "save_oauth_client":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&connectorsvc.OAuthClientConfigRequest{})
	default:
		return unsupportedChange(request)
	}
}

func (s *Service) executeConnectorsChange(ctx context.Context, actor *resolvedActor, request ChangeRequest, stateVersion int64) (any, error) {
	switch request.Operation {
	case "connect":
		var input connectorCredentials
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "disconnect":
		if stateVersion <= 0 {
			return nil, errors.New("Connector 断开缺少 configuration_version；请重新 plan")
		}
		return s.connectors.DisconnectAtVersion(ctx, actor.OwnerUserID, request.Target, stateVersion)
	case "save_oauth_client":
		var input connectorsvc.OAuthClientConfigRequest
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "delete_oauth_client":
		if stateVersion <= 0 {
			return nil, errors.New("Connector OAuth 删除缺少 configuration_version；请重新 plan")
		}
		return s.connectors.DeleteOAuthClientConfigAtVersion(
			ctx,
			actor.OwnerUserID,
			request.Target,
			stateVersion,
		)
	default:
		return nil, unsupportedChange(request)
	}
}

func connectorChecks(values connectorDomainValues, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainConnectors, "connectors_readable", err)}
	}
	checks := []Check{okCheck(DomainConnectors, "connectors_readable", "Connector 目录、认证类型、连接状态、额外字段与凭据存在状态可读取")}
	for _, item := range values.Items {
		if item.ConfigError == nil || strings.TrimSpace(*item.ConfigError) == "" {
			continue
		}
		checks = append(checks, Check{
			Code: "connector_configuration_error", Status: "warning", Message: *item.ConfigError,
			Domain: DomainConnectors, Target: item.ConnectorID,
			Remedy: "核对 OAuth client 或部署级 connector 配置", Verified: true,
		})
	}
	return checks
}

type connectorCredentials struct {
	Credentials map[string]string `json:"credentials"`
}

func (s *Service) readConnectorsConfiguration(ctx context.Context, actor *resolvedActor, target string) (any, []Check, int64, ScopeRef, error) {
	scope := actor.Context
	items, err := s.connectors.ListConnectors(ctx, actor.OwnerUserID, "", "", "")
	if err != nil {
		return nil, connectorChecks(connectorDomainValues{}, err), 0, scope, err
	}
	details := make(map[string]*connectorsvc.Detail, len(items))
	for _, item := range items {
		detail, detailErr := s.connectors.GetConnectorDetail(ctx, actor.OwnerUserID, item.ConnectorID)
		if detailErr != nil {
			return nil, connectorChecks(connectorDomainValues{Items: items}, detailErr), 0, scope, detailErr
		}
		details[item.ConnectorID] = detail
	}
	values := connectorDomainValues{Items: items, Details: details}
	return values, connectorChecks(values, nil), 0, scope, nil
}
