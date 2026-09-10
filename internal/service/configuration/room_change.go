// INPUT: Room 配置请求、可信成员身份与资源版本。
// OUTPUT: 成员和会话变更、实时调度及写后核验。
// POS: Room 配置操作的同域实现，沿用统一批准和 CAS。
package configuration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

func validateRoomsChange(request ChangeRequest) error {
	target := strings.TrimSpace(request.Target)
	switch request.Operation {
	case "create":
		if target != "" {
			return errors.New("rooms.create 不能指定现有 room_id")
		}
		var input protocol.CreateRoomRequest
		if err := request.decodeInput(&input); err != nil {
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
	case "update_profile":
		var input roomProfilePatch
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Name == nil && input.Description == nil && input.Avatar == nil {
			return errors.New("rooms.update_profile 至少要提供一个待修改字段")
		}
		return nil
	case "set_collaboration_policy":
		var input roomCollaborationPolicyPatch
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.SkillNames == nil && input.HostAutoReplyEnabled == nil && input.PrivateMessagesEnabled == nil {
			return errors.New("rooms.set_collaboration_policy 至少要提供一个待修改字段")
		}
		return nil
	case "add_member", "remove_member", "transfer_host":
		var input roomAgentTarget
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Normalized().AgentID == "" {
			return errors.New("agent_id 不能为空")
		}
		return nil
	case "set_member_participation":
		var input roomMemberParticipationInput
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Normalized().AgentID == "" {
			return errors.New("agent_id 不能为空")
		}
		if input.Paused == nil {
			return errors.New("paused 必须显式提供")
		}
		return nil
	case "create_conversation":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input protocol.CreateConversationRequest
		return request.decodeInput(&input)
	case "update_conversation":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input roomConversationTarget
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Normalized().ConversationID == "" {
			return errors.New("conversation_id 不能为空")
		}
		if input.Normalized().Title == "" {
			return errors.New("title 不能为空")
		}
		return nil
	case "delete_conversation":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input roomConversationTarget
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.Normalized().ConversationID == "" {
			return errors.New("conversation_id 不能为空")
		}
		if input.Normalized().Title != "" {
			return errors.New("rooms.delete_conversation 不接受 title")
		}
		return nil
	case "delete":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&struct{}{})
	default:
		return unsupportedChange(request)
	}
}

func (s *Service) executeRoomsChange(ctx context.Context, actor *resolvedActor, request ChangeRequest, stateVersion int64) (any, error) {
	switch request.Operation {
	case "create":
		var input protocol.CreateRoomRequest
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		value, err := s.rooms.CreateRoom(ctx, input)
		if err == nil {
			s.notifyRoomChanged(ctx, value, "room_created")
		}
		return value, err
	case "create_conversation":
		var input protocol.CreateConversationRequest
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "update_conversation":
		var input roomConversationTarget
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "delete_conversation":
		var input roomConversationTarget
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "update_profile":
		var input roomProfilePatch
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "set_collaboration_policy":
		var input roomCollaborationPolicyPatch
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "add_member":
		var input roomAgentTarget
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "remove_member":
		var input roomAgentTarget
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "set_member_participation":
		var input roomMemberParticipationInput
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "transfer_host":
		var input roomAgentTarget
		if err := request.decodeAppliedInput(&input); err != nil {
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
	case "delete":
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
		return nil, unsupportedChange(request)
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

func (s *Service) verifyRoomLifecycleChange(
	ctx context.Context,
	request ChangeRequest,
	plan ChangePlan,
	resultValue any,
	after DomainSnapshot,
) (*Check, error) {
	switch request.Operation {
	case "create":
		created, ok := resultValue.(*protocol.ConversationContextAggregate)
		if !ok || created == nil {
			return nil, errors.New("Room 已创建但执行结果缺少可核验的 Room/conversation 身份")
		}
		roomID := strings.TrimSpace(created.Room.ID)
		conversationID := strings.TrimSpace(created.Conversation.ID)
		if roomID == "" || conversationID == "" {
			return nil, errors.New("Room 已创建但返回的 Room/conversation 身份为空")
		}
		roomValue, err := s.rooms.GetRoom(ctx, roomID)
		if err != nil {
			return nil, fmt.Errorf("重新读取新建 Room: %w", err)
		}
		conversationValue, err := s.rooms.GetConversationContext(ctx, conversationID)
		if err != nil {
			return nil, fmt.Errorf("重新读取新建 Room 的初始 conversation: %w", err)
		}
		if roomValue == nil || conversationValue == nil ||
			strings.TrimSpace(conversationValue.Room.ID) != roomID {
			return nil, errors.New("新建 Room 与初始 conversation 的写后归属不一致")
		}
		if after.Revision == plan.CurrentRevision {
			return nil, errors.New("Room 创建后 owner Room 目录 revision 未变化")
		}
		return &Check{
			Code: "room_creation_verified", Status: "ok",
			Message: "已重新读取新建 Room 及初始 conversation，并核对 owner 目录 revision 已变化",
			Domain:  DomainRooms, Target: roomID, Verified: true,
		}, nil
	case "create_conversation":
		created, ok := resultValue.(*protocol.ConversationContextAggregate)
		if !ok || created == nil {
			return nil, errors.New("Conversation 已创建但执行结果缺少可核验身份")
		}
		conversationID := strings.TrimSpace(created.Conversation.ID)
		value, err := s.rooms.GetConversationContext(ctx, conversationID)
		if err != nil {
			return nil, fmt.Errorf("重新读取新建 conversation: %w", err)
		}
		if value == nil || strings.TrimSpace(value.Room.ID) != request.Target {
			return nil, errors.New("新建 conversation 不属于目标 Room")
		}
		return &Check{
			Code: "room_conversation_creation_verified", Status: "ok",
			Message: "已重新读取新建 conversation 并核对其 Room 归属",
			Domain:  DomainRooms, Target: conversationID, Verified: true,
		}, nil
	case "update_conversation":
		var input roomConversationTarget
		if err := strictDecodeJSON(request.Input, &input); err != nil {
			return nil, err
		}
		value, err := s.rooms.GetConversationContext(ctx, input.ConversationID)
		if err != nil {
			return nil, fmt.Errorf("重新读取已更新 conversation: %w", err)
		}
		if value == nil || strings.TrimSpace(value.Room.ID) != request.Target ||
			strings.TrimSpace(value.Conversation.Title) != strings.TrimSpace(input.Title) {
			return nil, errors.New("Conversation 写后标题或 Room 归属与计划不一致")
		}
		return &Check{
			Code: "room_conversation_update_verified", Status: "ok",
			Message: "已重新读取 conversation 并核对标题与 Room 归属",
			Domain:  DomainRooms, Target: input.ConversationID, Verified: true,
		}, nil
	case "delete_conversation":
		var input roomConversationTarget
		if err := strictDecodeJSON(request.Input, &input); err != nil {
			return nil, err
		}
		_, err := s.rooms.GetConversationContext(ctx, input.ConversationID)
		if err == nil {
			return nil, fmt.Errorf("Conversation %s 删除后仍存在", input.ConversationID)
		}
		if !errors.Is(err, roomsvc.ErrConversationNotFound) {
			return nil, fmt.Errorf("核对已删除 conversation: %w", err)
		}
		return &Check{
			Code: "room_conversation_deletion_verified", Status: "ok",
			Message: "已从当前真相源核对 conversation 不再存在",
			Domain:  DomainRooms, Target: input.ConversationID, Verified: true,
		}, nil
	case "set_member_participation":
		var input roomMemberParticipationInput
		if err := strictDecodeJSON(request.Input, &input); err != nil {
			return nil, err
		}
		input = input.Normalized()
		if input.Paused == nil {
			return nil, errors.New("核对 Room 成员参与状态缺少 paused")
		}
		roomValue, err := s.rooms.GetRoom(ctx, request.Target)
		if err != nil {
			return nil, fmt.Errorf("重新读取 Room 成员参与状态: %w", err)
		}
		if roomValue == nil {
			return nil, errors.New("重新读取 Room 成员参与状态得到空结果")
		}
		memberID := input.AgentID
		for _, member := range roomValue.Members {
			if member.MemberType != protocol.MemberTypeAgent ||
				strings.TrimSpace(member.MemberAgentID) != memberID {
				continue
			}
			if member.ParticipationPaused != *input.Paused {
				return nil, fmt.Errorf(
					"Room 成员参与状态写后不一致：expected=%t actual=%t",
					*input.Paused,
					member.ParticipationPaused,
				)
			}
			return &Check{
				Code: "room_member_participation_verified", Status: "ok",
				Message: "已从 Room 真相源重新读取并核对成员参与状态",
				Domain:  DomainRooms, Target: memberID, Verified: true,
			}, nil
		}
		return nil, fmt.Errorf("Room 成员 %s 在写后核对时不存在", memberID)
	default:
		return nil, nil
	}
}

type roomProfilePatch struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Avatar      *string `json:"avatar,omitempty"`
}

type roomCollaborationPolicyPatch struct {
	SkillNames             *[]string `json:"skill_names,omitempty"`
	HostAutoReplyEnabled   *bool     `json:"host_auto_reply_enabled,omitempty"`
	PrivateMessagesEnabled *bool     `json:"private_messages_enabled,omitempty"`
}

type roomAgentTarget struct {
	AgentID string `json:"agent_id"`
}

func (i roomAgentTarget) Normalized() roomAgentTarget {
	i.AgentID = strings.TrimSpace(i.AgentID)
	return i
}

type roomMemberParticipationInput struct {
	AgentID string `json:"agent_id"`
	Paused  *bool  `json:"paused"`
}

func (i roomMemberParticipationInput) Normalized() roomMemberParticipationInput {
	i.AgentID = strings.TrimSpace(i.AgentID)
	return i
}

type roomConversationTarget struct {
	ConversationID string `json:"conversation_id"`
	Title          string `json:"title,omitempty"`
}

func (i roomConversationTarget) Normalized() roomConversationTarget {
	i.ConversationID = strings.TrimSpace(i.ConversationID)
	i.Title = strings.TrimSpace(i.Title)
	return i
}

func roomProfileUpdateRequest(patch roomProfilePatch) protocol.UpdateRoomRequest {
	return protocol.UpdateRoomRequest{
		Name: patch.Name, Description: patch.Description, Avatar: patch.Avatar,
	}
}

func roomPolicyUpdateRequest(patch roomCollaborationPolicyPatch) protocol.UpdateRoomRequest {
	return protocol.UpdateRoomRequest{
		SkillNames:             patch.SkillNames,
		HostAutoReplyEnabled:   patch.HostAutoReplyEnabled,
		PrivateMessagesEnabled: patch.PrivateMessagesEnabled,
	}
}

func (s *Service) readRoomsConfiguration(ctx context.Context, actor *resolvedActor, target string) (any, []Check, int64, ScopeRef, error) {
	scope := actor.Context
	roomID := target
	if actor.Context.Kind == ScopeKindRoom {
		roomID = actor.RoomID
	}
	if roomID != "" {
		value, err := s.rooms.GetRoom(ctx, roomID)
		if err != nil {
			return nil, nil, 0, ScopeRef{Kind: ScopeKindRoom, ID: roomID}, err
		}
		return value,
			[]Check{okCheck(DomainRooms, "room_configuration_readable", "Room 共享配置、成员与当前群主已重新核对")},
			value.Room.ConfigurationVersion,
			ScopeRef{Kind: ScopeKindRoom, ID: roomID},
			nil
	}
	values, err := s.rooms.ListRooms(ctx, 100)
	return values,
		[]Check{okCheck(DomainRooms, "rooms_readable", fmt.Sprintf("已核对 %d 个 Room", len(values)))},
		0,
		scope,
		err
}

func (s *Service) validateScopedRoomsChange(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
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
	return nil

}

func (s *Service) verifyDeletedRooms(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	items, err := s.rooms.ListRooms(ctx, int(^uint(0)>>1))
	if err != nil {
		return err
	}
	for _, item := range items {
		if strings.TrimSpace(item.Room.ID) == request.Target {
			return fmt.Errorf("Room %s 删除后仍存在", request.Target)
		}
	}

	return nil
}
