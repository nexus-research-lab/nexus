// INPUT: 已执行的配置计划、规范化请求与写后真相源。
// OUTPUT: 删除存在性（含 Agent/Provider/Room/channel 子资源）、Room participation 精确值、资源版本推进与并发覆盖检查。
// POS: configuration apply 在宣告 success 前的资源级写后证明。
package configuration

import (
	"context"
	"fmt"
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

	if request.Domain == DomainMembers {
		if err = verifyMemberResult(resultValue, after, request.Operation); err != nil {
			return after, err
		}
		after.Checks = append(after.Checks, okCheck(DomainMembers, "member_verified", "已从 Control 核对成员写后状态"))
		return after, nil
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
		return s.verifyDeletedAgents(ctx, actor, request)
	case DomainProviders:
		return s.verifyDeletedProviders(ctx, actor, request)
	case DomainRooms:
		return s.verifyDeletedRooms(ctx, actor, request)
	case DomainSessions:
		return s.verifyDeletedSessions(ctx, actor, request)
	case DomainChannels:
		return s.verifyDeletedChannels(ctx, actor, request)
	default:
		return nil
	}
}
