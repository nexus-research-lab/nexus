// INPUT: 已通过动态授权、plan digest、revision、人工批准与审计门闩的规范化 ChangeRequest。
// OUTPUT: 领域服务写入、即时权限同步、下一轮重配信号与实时失效通知。
// POS: configuration 控制面到各配置真相源及热生效机制的唯一分派层。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
)

func (s *Service) executeChange(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
	stateVersion int64,
) (any, error) {
	if request.Domain == DomainMembers {
		if s.members == nil {
			return nil, errors.New("Control 成员管理未装配")
		}
		return s.members.ManageMembers(ctx, actor.OwnerUserID, actor.AuthSessionID, request.Operation, request.Target, request.Input, stateVersion)
	}
	switch request.Domain {
	case DomainPreferences:
		return s.executePreferencesChange(ctx, actor, request, stateVersion)
	case DomainProviders:
		return s.executeProvidersChange(ctx, actor, request, stateVersion)
	case DomainAgents:
		return s.executeAgentsChange(ctx, actor, request, stateVersion)
	case DomainEmotion:
		return s.executeEmotionChange(ctx, actor, request, stateVersion)
	case DomainChannels:
		return s.executeChannelsChange(ctx, actor, request, stateVersion)
	case DomainConnectors:
		return s.executeConnectorsChange(ctx, actor, request, stateVersion)
	case DomainSkills:
		return s.executeSkillsChange(ctx, actor, request, stateVersion)
	case DomainSessions:
		return s.executeSessionsChange(ctx, actor, request, stateVersion)
	case DomainRooms:
		return s.executeRoomsChange(ctx, actor, request, stateVersion)
	default:
		return nil, unsupportedChange(request)
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
