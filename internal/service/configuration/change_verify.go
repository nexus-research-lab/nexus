// INPUT: 已执行的配置计划、规范化请求与写后真相源。
// OUTPUT: 删除存在性（含 Agent/Provider/Room/channel 子资源）、精确资源版本推进与并发覆盖检查。
// POS: configuration apply 在宣告 success 前的资源级写后证明。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func (s *Service) snapshotAfterChange(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
	plan ChangePlan,
	resultValue any,
) (DomainSnapshot, error) {
	target := plan.Target
	if isTargetDeletion(request) {
		target = ""
	}
	snapshotRequest := request
	snapshotRequest.Target = target
	after, err := s.snapshotForChange(ctx, actor, snapshotRequest, true)
	if err != nil {
		return DomainSnapshot{}, err
	}

	if request.Domain == DomainRooms {
		check, verifyErr := s.verifyRoomLifecycleChange(ctx, request, plan, resultValue, after)
		if verifyErr != nil {
			return after, verifyErr
		}
		if check != nil {
			after.Checks = append(after.Checks, *check)
		}
	}
	if request.Domain == DomainSkills {
		check, verifyErr := s.verifySkillCatalogResult(
			ctx,
			request,
			resultValue,
			plan,
			after,
		)
		if verifyErr != nil {
			return after, verifyErr
		}
		if check != nil {
			after.Checks = append(after.Checks, *check)
		}
	}
	if request.Domain == DomainSessions && request.Operation == "update_title" {
		check, verifyErr := s.verifySessionTitleChange(ctx, request)
		if verifyErr != nil {
			return after, verifyErr
		}
		after.Checks = append(after.Checks, check)
	}

	if isTargetDeletion(request) {
		if err = s.verifyDeletedTarget(ctx, actor, request); err != nil {
			return after, err
		}
		after.Checks = append(after.Checks, Check{
			Code: "configuration_target_deleted", Status: "ok",
			Message: "已从当前真相源核对目标不存在",
			Domain:  request.Domain, Target: request.Target, Verified: true,
		})
		return after, nil
	}

	if plan.StateVersion > 0 && operationAdvancesStateVersion(request) {
		expectedVersion := plan.StateVersion + 1
		if after.StateVersion != expectedVersion {
			return after, fmt.Errorf(
				"%s/%s 写后版本异常：expected=%d actual=%d；可能发生并发覆盖，请重新 inspect 并 reconcile",
				plan.Scope.Kind,
				plan.Scope.ID,
				expectedVersion,
				after.StateVersion,
			)
		}
		after.Checks = append(after.Checks, Check{
			Code: "configuration_resource_version_advanced", Status: "ok",
			Message: fmt.Sprintf("资源版本已按 CAS 从 %d 推进到 %d", plan.StateVersion, after.StateVersion),
			Domain:  request.Domain, Target: request.Target, Verified: true,
		})
	}
	return after, nil
}

func operationAdvancesStateVersion(request ChangeRequest) bool {
	if request.Domain == DomainSkills {
		switch request.Operation {
		case "search_external", "preview_external", "check_updates", "update_all":
			return false
		}
	}
	return true
}

func (s *Service) verifySessionTitleChange(
	ctx context.Context,
	request ChangeRequest,
) (Check, error) {
	var input sessionTitleInput
	if err := strictDecodeJSON(request.Input, &input); err != nil {
		return Check{}, err
	}
	expectedTitle := strings.TrimSpace(input.Title)
	if expectedTitle == "" {
		expectedTitle = "New Chat"
	}
	item, err := s.sessions.GetMutableSession(ctx, request.Target)
	if err != nil {
		return Check{}, fmt.Errorf("重新读取 Session: %w", err)
	}
	actualTitle := ""
	if item != nil {
		actualTitle = strings.TrimSpace(item.Title)
	}
	if item == nil || actualTitle != expectedTitle {
		return Check{}, fmt.Errorf(
			"Session 标题写后不一致：expected=%q actual=%q",
			expectedTitle,
			actualTitle,
		)
	}
	return Check{
		Code: "session_title_verified", Status: "ok",
		Message: "已从 owner workspace 重新读取 Session，并核对标题与计划一致",
		Domain:  DomainSessions, Target: request.Target, Verified: true,
	}, nil
}

func (s *Service) verifySkillCatalogResult(
	ctx context.Context,
	request ChangeRequest,
	resultValue any,
	plan ChangePlan,
	after DomainSnapshot,
) (*Check, error) {
	switch request.Operation {
	case "import_git", "import_url", "import_skills_sh", "update_single":
		detail, ok := resultValue.(*skillsvc.Detail)
		if !ok || detail == nil || strings.TrimSpace(detail.Name) == "" {
			return nil, errors.New("Skill catalog 变更结果缺少可核验的 Skill 身份")
		}
		state, err := s.skills.GetCatalogSkillState(ctx, detail.Name)
		if err != nil {
			return nil, fmt.Errorf("重新读取 Skill catalog 结果: %w", err)
		}
		if !state.Exists {
			return nil, fmt.Errorf("Skill %s 写入后未出现在 catalog", detail.Name)
		}
		return &Check{
			Code: "skill_catalog_publication_verified", Status: "ok",
			Message: "已重新读取 Skill catalog 并核对原子发布结果",
			Domain:  DomainSkills, Target: detail.Name, Verified: true,
		}, nil
	case "update_all":
		result, ok := resultValue.(*skillsvc.UpdateInstalledSkillsResponse)
		if !ok || result == nil {
			return nil, errors.New("Skill 批量更新结果缺少可核验的逐项结果")
		}
		expectedVersion := plan.StateVersion + int64(len(result.UpdatedSkills))
		if after.StateVersion != expectedVersion {
			return nil, fmt.Errorf(
				"Skill 批量更新版本异常：expected=%d actual=%d；请 reconcile",
				expectedVersion,
				after.StateVersion,
			)
		}
		seen := make(map[string]struct{}, len(result.UpdatedSkills))
		for _, rawName := range result.UpdatedSkills {
			name := strings.TrimSpace(rawName)
			if name == "" {
				return nil, errors.New("Skill 批量更新结果包含空名称")
			}
			if _, duplicate := seen[name]; duplicate {
				return nil, fmt.Errorf("Skill 批量更新结果重复返回 %s", name)
			}
			seen[name] = struct{}{}
			state, err := s.skills.GetCatalogSkillState(ctx, name)
			if err != nil {
				return nil, fmt.Errorf("重新读取批量更新 Skill %s: %w", name, err)
			}
			if !state.Exists || state.CatalogVersion != after.StateVersion {
				return nil, fmt.Errorf(
					"Skill %s 批量更新后状态不一致：exists=%t catalog_version=%d",
					name,
					state.Exists,
					state.CatalogVersion,
				)
			}
		}
		return &Check{
			Code: "skill_catalog_bulk_update_verified", Status: "ok",
			Message: fmt.Sprintf(
				"已核对批量结果：更新 %d、跳过 %d、失败 %d，catalog version 从 %d 推进到 %d",
				len(result.UpdatedSkills),
				len(result.SkippedSkills),
				len(result.Failures),
				plan.StateVersion,
				after.StateVersion,
			),
			Domain: DomainSkills, Verified: true,
		}, nil
	default:
		return nil, nil
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
	default:
		return nil, nil
	}
}

func isTargetDeletion(request ChangeRequest) bool {
	if (request.Domain == DomainAgents ||
		request.Domain == DomainProviders ||
		request.Domain == DomainRooms ||
		request.Domain == DomainSessions) &&
		request.Operation == "delete" {
		return true
	}
	if request.Domain != DomainChannels {
		return false
	}
	switch request.Operation {
	case "delete_config", "delete_account", "delete_pairing":
		return true
	default:
		return false
	}
}

func (s *Service) verifyDeletedTarget(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	switch request.Domain {
	case DomainAgents:
		items, err := s.agents.ListAgentRecords(ctx)
		if err != nil {
			return err
		}
		for _, item := range items {
			if strings.TrimSpace(item.AgentID) == request.Target {
				return fmt.Errorf("Agent %s 删除后仍存在", request.Target)
			}
		}
	case DomainProviders:
		items, err := s.providers.List(ctx)
		if err != nil {
			return err
		}
		for _, item := range items {
			if strings.TrimSpace(item.Provider) == request.Target && item.CanManage {
				return fmt.Errorf("Provider %s 的可管理配置删除后仍存在", request.Target)
			}
		}
	case DomainRooms:
		items, err := s.rooms.ListRooms(ctx, int(^uint(0)>>1))
		if err != nil {
			return err
		}
		for _, item := range items {
			if strings.TrimSpace(item.Room.ID) == request.Target {
				return fmt.Errorf("Room %s 删除后仍存在", request.Target)
			}
		}
	case DomainSessions:
		if s.sessions == nil {
			return errors.New("Sessions 配置服务未装配")
		}
		if _, err := s.sessions.GetMutableSession(ctx, request.Target); err == nil {
			return fmt.Errorf("Session %s 删除后仍存在", request.Target)
		} else if !errors.Is(err, sessionsvc.ErrSessionNotFound) {
			return fmt.Errorf("核对已删除 Session: %w", err)
		}
	case DomainChannels:
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
	}
	return nil
}
