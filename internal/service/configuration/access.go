// INPUT: 每次调用重新解析的可信 Actor、配置域与用户声明的目标。
// OUTPUT: 最小权限能力目录、不可伪造的资源 scope 与规范化变更请求。
// POS: configuration 的字段/操作授权真相源；所有 inspect/plan/apply 必须先经过这里。
package configuration

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var (
	selfAgentOperations   = []string{"update_self_profile", "update_self_runtime"}
	selfSkillOperations   = []string{"install_self", "uninstall_self"}
	selfEmotionOperations = []string{"set_base", "set_context", "clear_context"}
	selfSessionOperations = []string{"update_title"}
	roomEmotionOperations = []string{"set_context", "clear_context"}
	roomHostOperations    = []string{
		"update_profile",
		"set_collaboration_policy",
		"add_member",
		"remove_member",
		"transfer_host",
		"create_conversation",
		"update_conversation",
		"delete_conversation",
	}
)

func accessFor(actor *resolvedActor, definition DomainDefinition) Access {
	access := Access{Authority: actor.Authority}
	switch actor.Authority {
	case AuthorityOwnerMain:
		if actor.Context.Kind != ScopeKindOwner {
			access.Reason = "主智能体的 owner 级能力只在自己的私有 DM 中开放"
			return access
		}
		if definition.Name == DomainHost && !actor.canManageHostConfiguration() {
			access.Reason = "宿主运行设置只允许本地单用户或真实 owner/admin 管理"
			return access
		}
		access.CanRead = true
		access.AllowedOperations = operationNames(definition.Operations)
	case AuthorityAgentSelf:
		switch definition.Name {
		case DomainAgents:
			access.CanRead = true
			access.AllowedOperations = slices.Clone(selfAgentOperations)
		case DomainSkills:
			access.CanRead = true
			access.AllowedOperations = slices.Clone(selfSkillOperations)
		case DomainProviders:
			access.CanRead = true
			access.Reason = "只读可用模型目录；凭据永不返回"
		case DomainEmotion:
			access.CanRead = true
			access.AllowedOperations = slices.Clone(selfEmotionOperations)
		case DomainSessions:
			access.CanRead = true
			access.AllowedOperations = slices.Clone(selfSessionOperations)
			access.Reason = "只能检查并重命名当前可信 WebSocket 私有 DM session"
		default:
			access.Reason = "普通 Agent 只能在自己的 DM 管理安全的 profile/runtime/skill 子集"
		}
	case AuthorityRoomHost:
		switch definition.Name {
		case DomainRooms:
			access.CanRead = true
			access.AllowedOperations = slices.Clone(roomHostOperations)
		case DomainEmotion:
			access.CanRead = true
			access.AllowedOperations = slices.Clone(roomEmotionOperations)
			access.Reason = "群主只能修改当前 Agent 自己在当前 conversation 的上下文情绪"
		default:
			access.Reason = "群主只能管理当前 Room，不能借群聊修改 Agent 或 owner 全局配置"
		}
	case AuthorityRoomMember:
		switch definition.Name {
		case DomainRooms:
			access.CanRead = true
			access.Reason = "普通成员可核对当前 Room 设置，但不能持久修改"
		case DomainEmotion:
			access.CanRead = true
			access.AllowedOperations = slices.Clone(roomEmotionOperations)
			access.Reason = "普通成员只能修改当前 Agent 自己在当前 conversation 的上下文情绪"
		default:
			access.Reason = "普通成员在 Room 中没有全局配置能力"
		}
	}
	return access
}

func (r *resolvedActor) canManageHostConfiguration() bool {
	if r == nil {
		return false
	}
	if r.LocalSingleUser &&
		strings.TrimSpace(r.OwnerUserID) == authctx.SystemUserID &&
		strings.TrimSpace(r.AuthMethod) == authctx.AuthMethodLocal {
		return true
	}
	switch strings.TrimSpace(r.PrincipalRole) {
	case authctx.RoleOwner, authctx.RoleAdmin:
		return true
	default:
		return false
	}
}

func operationNames(operations []OperationDefinition) []string {
	result := make([]string, 0, len(operations))
	for _, operation := range operations {
		result = append(result, operation.Name)
	}
	return result
}

func definitionForActor(actor *resolvedActor, domain string) (DomainDefinition, Access, error) {
	definition, err := definitionFor(domain)
	if err != nil {
		return DomainDefinition{}, Access{}, err
	}
	access := accessFor(actor, definition)
	if !access.CanRead {
		return DomainDefinition{}, access, fmt.Errorf(
			"%s 无权读取配置域 %s：%s",
			actor.Authority,
			definition.Name,
			access.Reason,
		)
	}
	filtered := definition
	filtered.Operations = make([]OperationDefinition, 0, len(access.AllowedOperations))
	for _, operation := range definition.Operations {
		if slices.Contains(access.AllowedOperations, operation.Name) {
			filtered.Operations = append(filtered.Operations, operation)
		}
	}
	return filtered, access, nil
}

func readableDomains(actor *resolvedActor) []string {
	result := make([]string, 0, len(domainCatalog))
	for _, definition := range domainCatalog {
		if accessFor(actor, definition).CanRead {
			result = append(result, definition.Name)
		}
	}
	return result
}

func authorizeChange(actor *resolvedActor, request ChangeRequest) (ChangeRequest, ScopeRef, error) {
	definition, access, err := definitionForActor(actor, request.Domain)
	if err != nil {
		return ChangeRequest{}, ScopeRef{}, err
	}
	operation, err := operationFor(definition, request.Operation)
	if err != nil {
		return ChangeRequest{}, ScopeRef{}, err
	}
	if !slices.Contains(access.AllowedOperations, operation.Name) {
		return ChangeRequest{}, ScopeRef{}, fmt.Errorf(
			"%s 无权执行 %s.%s",
			actor.Authority,
			definition.Name,
			operation.Name,
		)
	}
	request.Domain = definition.Name
	request.Operation = operation.Name
	request.Target = strings.TrimSpace(request.Target)

	switch actor.Authority {
	case AuthorityOwnerMain:
		return authorizeOwnerChange(actor, request)
	case AuthorityAgentSelf:
		switch request.Domain + "." + request.Operation {
		case DomainAgents + ".update_self_profile", DomainAgents + ".update_self_runtime":
			if request.Target != "" && request.Target != actor.AgentID {
				return ChangeRequest{}, ScopeRef{}, errors.New("普通 Agent 不能修改其他 Agent")
			}
			request.Target = actor.AgentID
			return request, ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, nil
		case DomainSkills + ".install_self", DomainSkills + ".uninstall_self":
			return request, ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, nil
		case DomainEmotion + ".set_base",
			DomainEmotion + ".set_context",
			DomainEmotion + ".clear_context":
			return authorizeSelfEmotionChange(actor, request)
		case DomainSessions + ".update_title":
			if request.Target != "" && request.Target != actor.SessionKey {
				return ChangeRequest{}, ScopeRef{}, errors.New("普通 Agent 只能修改当前可信 DM session")
			}
			request.Target = actor.SessionKey
			return request, ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, nil
		}
	case AuthorityRoomHost:
		if request.Domain == DomainEmotion {
			return authorizeRoomEmotionChange(actor, request)
		}
		if request.Target != "" && request.Target != actor.RoomID {
			return ChangeRequest{}, ScopeRef{}, errors.New("群主只能修改当前 Room，不能通过 target 切换作用域")
		}
		request.Target = actor.RoomID
		return request, ScopeRef{Kind: ScopeKindRoom, ID: actor.RoomID}, nil
	case AuthorityRoomMember:
		if request.Domain == DomainEmotion {
			return authorizeRoomEmotionChange(actor, request)
		}
	}
	return ChangeRequest{}, ScopeRef{}, fmt.Errorf("%s 无权执行持久配置变更", actor.Authority)
}

func authorizeOwnerChange(actor *resolvedActor, request ChangeRequest) (ChangeRequest, ScopeRef, error) {
	switch request.Domain {
	case DomainAgents:
		if request.Operation == "create" {
			return request, ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}, nil
		}
		if request.Target == "" {
			return ChangeRequest{}, ScopeRef{}, fmt.Errorf("%s.%s 要求 target agent_id", request.Domain, request.Operation)
		}
		return request, ScopeRef{Kind: ScopeKindAgent, ID: request.Target}, nil
	case DomainRooms:
		if request.Operation == "create" {
			if request.Target != "" {
				return ChangeRequest{}, ScopeRef{}, errors.New("rooms.create 不能指定现有 room_id")
			}
			return request, ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}, nil
		}
		if request.Target == "" {
			return ChangeRequest{}, ScopeRef{}, fmt.Errorf("%s.%s 要求 target room_id", request.Domain, request.Operation)
		}
		return request, ScopeRef{Kind: ScopeKindRoom, ID: request.Target}, nil
	case DomainSessions:
		if request.Target == "" {
			return ChangeRequest{}, ScopeRef{}, fmt.Errorf(
				"%s.%s 要求 target session_key",
				request.Domain,
				request.Operation,
			)
		}
		parsed := protocol.ParseSessionKey(request.Target)
		if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent ||
			strings.TrimSpace(parsed.AgentID) == "" {
			return ChangeRequest{}, ScopeRef{}, errors.New("sessions target 必须是结构化 Agent session_key")
		}
		if request.Operation == "delete" &&
			request.Target == strings.TrimSpace(actor.SessionKey) {
			return ChangeRequest{}, ScopeRef{}, errors.New("当前正在执行配置工具的 session 不能删除自己")
		}
		return request, ScopeRef{Kind: ScopeKindAgent, ID: parsed.AgentID}, nil
	case DomainEmotion:
		return authorizeSelfEmotionChange(actor, request)
	case DomainSkills:
		target, scopedToAgent, err := skillChangeTarget(actor, request)
		if err != nil {
			return ChangeRequest{}, ScopeRef{}, err
		}
		if scopedToAgent {
			return request, ScopeRef{Kind: ScopeKindAgent, ID: target.AgentID}, nil
		}
		return request, ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}, nil
	default:
		return request, ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}, nil
	}
}

func authorizeSelfEmotionChange(
	actor *resolvedActor,
	request ChangeRequest,
) (ChangeRequest, ScopeRef, error) {
	if request.Target != "" && request.Target != actor.AgentID {
		return ChangeRequest{}, ScopeRef{}, errors.New("Agent 情绪状态只能绑定当前 Agent，不能切换 target")
	}
	request.Target = actor.AgentID
	return request, ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, nil
}

func authorizeRoomEmotionChange(
	actor *resolvedActor,
	request ChangeRequest,
) (ChangeRequest, ScopeRef, error) {
	if request.Operation == "set_base" {
		return ChangeRequest{}, ScopeRef{}, errors.New("Agent 基础情绪只能在自己的私有 DM 中修改")
	}
	return authorizeSelfEmotionChange(actor, request)
}

func operationForActor(actor *resolvedActor, request ChangeRequest) (OperationDefinition, error) {
	definition, _, err := definitionForActor(actor, request.Domain)
	if err != nil {
		return OperationDefinition{}, err
	}
	return operationFor(definition, request.Operation)
}
