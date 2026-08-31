// INPUT: Agent Handler 已确认的领域错误、更新阶段与提交事实。
// OUTPUT: 保持旧 HTTP 状态的 Agent FailureCore 映射，不改变更新/删除事务或业务身份。
// POS: Agent HTTP 边界的失败证据投影；不得从错误文案猜测提交结果。
package agent

import (
	"errors"
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
)

func agentCreateFailure(err error) (int, handlershared.FailureSpec) {
	status := http.StatusInternalServerError
	spec := handlershared.FailureSpec{
		Code:     "agent.creation_outcome_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "无法确认 Agent 是否已经创建",
		Cause:    err,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "agent.check_creation_request",
		},
	}

	switch {
	case errors.Is(err, agentpkg.ErrAgentCreationRequestInvalid):
		status = http.StatusBadRequest
		spec.Code = "agent.creation_request_invalid"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "创建请求无效，Agent 没有创建"
		spec.Resolution.Action = "agent.review_creation"
	case errors.Is(err, agentpkg.ErrAgentNameInvalid):
		status = http.StatusBadRequest
		spec.Code = "agent.creation_rejected"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = err.Error()
		spec.Resolution.Action = "agent.review_creation"
	case errors.Is(err, agentpkg.ErrAgentCreationRequestConflict):
		status = http.StatusConflict
		spec.Code = "agent.creation_request_conflict"
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "这个创建编号已经用于另一项 Agent 设置"
	case errors.Is(err, agentpkg.ErrAgentCreationPending):
		status = http.StatusConflict
		spec.Code = "agent.creation_in_progress"
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectAccepted
		spec.Detail = "Agent 创建请求已受理，但还没有完成"
	case errors.Is(err, agentpkg.ErrAgentCreationFailed):
		spec.Code = "agent.creation_failed"
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "Agent 没有创建，已有 Agent、会话和文件不受影响"
		spec.Resolution.Action = "agent.start_new_creation"
	case errors.Is(err, agentpkg.ErrAgentCreationResultDeleted):
		status = http.StatusGone
		spec.Code = "agent.creation_result_deleted"
		spec.Category = protocol.FailureCategoryNotFound
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "这个创建请求对应的 Agent 之后已被删除，本次没有重新创建"
		spec.Resolution.Action = "agent.start_new_creation"
	case agentpkg.AgentCreationCommitted(err):
		spec.Code = "agent.creation_projection_incomplete"
		spec.Effect = protocol.FailureEffectCommitted
		spec.Detail = "Agent 已创建，但页面还没有拿到完整结果"
	}
	return status, spec
}

func agentCreationLookupFailure(err error) (int, handlershared.FailureSpec) {
	if errors.Is(err, agentpkg.ErrAgentCreationRequestInvalid) {
		return http.StatusBadRequest, handlershared.FailureSpec{
			Code:     "agent.creation_request_invalid",
			Category: protocol.FailureCategoryValidation,
			Effect:   protocol.FailureEffectNotApplicable,
			Detail:   "创建请求编号无效",
			Cause:    err,
		}
	}
	if agentpkg.AgentCreationCommitted(err) {
		return http.StatusInternalServerError, handlershared.FailureSpec{
			Code:     "agent.creation_projection_incomplete",
			Category: protocol.FailureCategoryInternal,
			Effect:   protocol.FailureEffectCommitted,
			Detail:   "Agent 已创建，但暂时无法读取完整结果",
			Cause:    err,
			Resolution: &protocol.FailureResolution{
				Actor:  protocol.FailureRecoveryActorUser,
				Action: "agent.check_creation_request",
			},
		}
	}
	return http.StatusInternalServerError, handlershared.FailureSpec{
		Code:     "agent.creation_request_unavailable",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   "暂时无法查看 Agent 创建结果",
		Cause:    err,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "agent.check_creation_request",
		},
	}
}

func agentDeleteFailure(err error) (int, handlershared.FailureSpec) {
	status := http.StatusInternalServerError
	spec := handlershared.FailureSpec{
		Code:     "agent.deletion_outcome_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "无法确认成员是否已经删除",
		Cause:    err,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "agent.refresh_directory",
		},
	}

	switch {
	case errors.Is(err, agentpkg.ErrAgentNotFound):
		status = http.StatusNotFound
		spec.Code = "agent.not_found"
		spec.Category = protocol.FailureCategoryNotFound
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "成员不存在或已经删除"
		spec.Resolution.Action = "agent.refresh_directory"
	case agentpkg.AgentDeletionCommitted(err):
		spec.Code = "agent.deletion_cleanup_incomplete"
		spec.Effect = protocol.FailureEffectCommitted
		spec.Detail = "成员已删除，但关联内容没有全部清理完成"
		spec.Resolution.Action = "agent.refresh_directory"
	case errors.Is(err, agentpkg.ErrAgentDeletionNotAllowed):
		status = http.StatusBadRequest
		spec.Code = "agent.deletion_not_allowed"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "主智能体不能删除"
		spec.Resolution = nil
	case errors.Is(err, agentpkg.ErrRuntimeVersionConflict):
		status = http.StatusConflict
		spec.Code = "agent.deletion_conflict"
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "成员设置已被其他操作更新，删除没有执行"
		spec.Resolution.Action = "agent.refresh_directory"
	}
	return status, spec
}

func agentUpdateFailure(err error) (int, handlershared.FailureSpec) {
	status := http.StatusInternalServerError
	spec := handlershared.FailureSpec{
		Code:     "agent.update_outcome_unknown",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "无法确认 Agent 设置是否已经保存",
		Cause:    err,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "agent.refresh_directory",
		},
	}

	switch {
	case errors.Is(err, agentpkg.ErrAgentNotFound):
		status = http.StatusNotFound
		spec.Code = "agent.not_found"
		spec.Category = protocol.FailureCategoryNotFound
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "Agent 不存在，设置没有保存"
	case errors.Is(err, agentpkg.ErrAgentNameInvalid),
		errors.Is(err, agentpkg.ErrMainAgentNameImmutable):
		status = http.StatusBadRequest
		spec.Code = "agent.update_rejected"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		if errors.Is(err, agentpkg.ErrMainAgentNameImmutable) {
			spec.Detail = "主智能体名称不能修改"
		} else {
			spec.Detail = err.Error()
		}
		spec.Resolution = &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "agent.review_settings",
		}
	case errors.Is(err, agentpkg.ErrRuntimeVersionConflict):
		status = http.StatusConflict
		spec.Code = "agent.update_conflict"
		spec.Category = protocol.FailureCategoryConflict
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "Agent 设置已被其他操作更新"
	case agentpkg.AgentUpdateCommitted(err):
		spec.Code = "agent.update_projection_incomplete"
		spec.Effect = protocol.FailureEffectCommitted
		spec.Detail = "Agent 设置已保存，但本地运行配置没有全部同步完成"
	}
	return status, spec
}

func agentPermissionModeSyncFailure(err error) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "agent.permission_mode_sync_incomplete",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectCommitted,
		Detail:   "Agent 设置已保存，但部分运行中的会话没有完成同步",
		Cause:    err,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "agent.refresh_directory",
		},
	}
}
