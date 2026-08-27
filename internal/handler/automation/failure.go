// INPUT: Automation HTTP 操作及 service 返回的领域错误。
// OUTPUT: 保持既有状态码与安全 detail，同时补充有证据的 FailureCore 事实。
// POS: Automation handler 的失败映射边界；不得从错误文本推断写入是否发生。
package automation

import (
	"errors"
	"io"
	"net/http"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

func (h *Handlers) bindAutomationJSON(
	writer http.ResponseWriter,
	request *http.Request,
	target any,
	allowEmpty bool,
) bool {
	err := handlershared.DecodeJSONBody(request.Body, target, allowEmpty)
	if allowEmpty && errors.Is(err, io.EOF) {
		return true
	}
	if err == nil {
		return true
	}
	h.api.WriteError(writer, request, http.StatusBadRequest, handlershared.FailureSpec{
		Code:     "automation.request_body_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "请求参数错误",
		Cause:    err,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "automation.review_request",
		},
	})
	return false
}

type automationFailureOperation string

const (
	automationFailureListTasks              automationFailureOperation = "list_tasks"
	automationFailureListPermissions        automationFailureOperation = "list_permissions"
	automationFailureGetStatus              automationFailureOperation = "get_status"
	automationFailureListEvents             automationFailureOperation = "list_events"
	automationFailureDailyReport            automationFailureOperation = "daily_report"
	automationFailureGetCreateRequest       automationFailureOperation = "get_create_request"
	automationFailureCreateTask             automationFailureOperation = "create_task"
	automationFailureReplayCreateTask       automationFailureOperation = "replay_create_task"
	automationFailureUpdateTask             automationFailureOperation = "update_task"
	automationFailureUpdateStatus           automationFailureOperation = "update_status"
	automationFailureResolvePermission      automationFailureOperation = "resolve_permission"
	automationFailureResumePermission       automationFailureOperation = "resume_permission"
	automationFailureRunTask                automationFailureOperation = "run_task"
	automationFailureRecoverTask            automationFailureOperation = "recover_task"
	automationFailureRetryDelivery          automationFailureOperation = "retry_delivery"
	automationFailureDeleteTask             automationFailureOperation = "delete_task"
	automationFailureConfirmDeletionStopped automationFailureOperation = "confirm_deletion_stopped"
)

var (
	errPageScriptCreateUnsupported          = errors.New("页面暂不支持创建脚本任务")
	errPageScriptUpdateUnsupported          = errors.New("页面暂不支持创建或修改脚本任务")
	errTaskStatusEnabledRequired            = errors.New("enabled is required")
	errExpectedConfigurationVersionRequired = errors.New("expected_configuration_version is required")
)

type automationHTTPFailure struct {
	status int
	spec   handlershared.FailureSpec
}

func (h *Handlers) writeAutomationFailure(
	writer http.ResponseWriter,
	request *http.Request,
	operation automationFailureOperation,
	err error,
) {
	failure := mapAutomationFailure(operation, err)
	h.api.WriteError(writer, request, failure.status, failure.spec)
}

func mapAutomationFailure(
	operation automationFailureOperation,
	err error,
) automationHTTPFailure {
	switch operation {
	case automationFailureListTasks:
		return automationFailure(
			http.StatusInternalServerError,
			"automation.task_list_unavailable",
			protocol.FailureCategoryInternal,
			protocol.FailureEffectNotApplicable,
			"任务列表读取失败",
			err,
			"automation.reload_tasks",
		)
	case automationFailureListPermissions:
		return automationFailure(
			http.StatusInternalServerError,
			"automation.permission_list_unavailable",
			protocol.FailureCategoryInternal,
			protocol.FailureEffectNotApplicable,
			"权限请求读取失败",
			err,
			"automation.reload_permissions",
		)
	case automationFailureGetStatus:
		return mapAutomationReadFailure(err, "task_status", "任务状态读取失败", "automation.reload_task")
	case automationFailureListEvents:
		return mapAutomationReadFailure(err, "task_events", "任务事件读取失败", "automation.reload_task_events")
	case automationFailureDailyReport:
		return mapAutomationDailyReportFailure(err)
	case automationFailureGetCreateRequest:
		if handlershared.IsClientMessageError(err) {
			return automationFailure(http.StatusBadRequest, "automation.task_create_request_invalid", protocol.FailureCategoryValidation, protocol.FailureEffectNotApplicable, "创建记录编号无效", err, "automation.reload_tasks")
		}
		return automationFailure(http.StatusInternalServerError, "automation.task_create_request_unavailable", protocol.FailureCategoryInternal, protocol.FailureEffectNotApplicable, "创建结果读取失败", err, "automation.reload_tasks")
	case automationFailureCreateTask:
		return mapCreateTaskFailure(err, false)
	case automationFailureReplayCreateTask:
		return mapCreateTaskFailure(err, true)
	case automationFailureUpdateTask:
		return mapTaskDefinitionFailure("update", err)
	case automationFailureUpdateStatus:
		return mapTaskDefinitionFailure("status", err)
	case automationFailureResolvePermission:
		return mapResolvePermissionFailure(err)
	case automationFailureResumePermission:
		return mapResumePermissionFailure(err)
	case automationFailureRunTask:
		return mapRunTaskFailure(err)
	case automationFailureRecoverTask:
		return mapRecoverTaskFailure(err)
	case automationFailureRetryDelivery:
		return mapRetryDeliveryFailure(err)
	case automationFailureDeleteTask:
		return mapDeleteTaskFailure(err)
	case automationFailureConfirmDeletionStopped:
		return mapConfirmDeletionStoppedFailure(err)
	default:
		return automationFailure(
			http.StatusInternalServerError,
			"automation.request_failed",
			protocol.FailureCategoryInternal,
			protocol.FailureEffectUnknown,
			"自动化操作失败",
			err,
			"automation.reload_tasks",
		)
	}
}

func mapAutomationReadFailure(
	err error,
	kind string,
	detail string,
	action string,
) automationHTTPFailure {
	if errors.Is(err, automationdomain.ErrJobNotFound) {
		return automationFailure(
			http.StatusNotFound,
			"automation."+kind+"_not_found",
			protocol.FailureCategoryNotFound,
			protocol.FailureEffectNotApplicable,
			"资源不存在",
			err,
			"automation.return_to_tasks",
		)
	}
	return automationFailure(
		http.StatusInternalServerError,
		"automation."+kind+"_unavailable",
		protocol.FailureCategoryInternal,
		protocol.FailureEffectNotApplicable,
		detail,
		err,
		action,
	)
}

func mapAutomationDailyReportFailure(err error) automationHTTPFailure {
	if errors.Is(err, automationdomain.ErrJobNotFound) {
		return mapAutomationReadFailure(err, "daily_report", "日报读取失败", "automation.reload_daily_report")
	}
	message := strings.ToLower(err.Error())
	if handlershared.IsClientMessageError(err) ||
		strings.Contains(message, "date must be") ||
		strings.Contains(message, "invalid timezone") {
		return automationFailure(
			http.StatusBadRequest,
			"automation.daily_report_invalid",
			protocol.FailureCategoryValidation,
			protocol.FailureEffectNotApplicable,
			err.Error(),
			err,
			"automation.review_report_filters",
		)
	}
	return mapAutomationReadFailure(err, "daily_report", "日报读取失败", "automation.reload_daily_report")
}

func mapCreateTaskFailure(err error, replayable bool) automationHTTPFailure {
	if errors.Is(err, automationdomain.ErrCreateRequestResultGone) {
		return automationFailure(
			http.StatusGone,
			"automation.task_create_result_gone",
			protocol.FailureCategoryNotFound,
			protocol.FailureEffectCommitted,
			"该创建请求曾成功，但任务后来已被删除",
			err,
			"automation.start_new_task",
		)
	}
	if errors.Is(err, automationdomain.ErrCreateRequestConflict) {
		return automationFailure(
			http.StatusConflict,
			"automation.task_create_conflict",
			protocol.FailureCategoryConflict,
			protocol.FailureEffectNotApplied,
			"这次创建请求与之前的内容不一致，请重新提交",
			err,
			"automation.review_task",
		)
	}
	if errors.Is(err, errPageScriptCreateUnsupported) {
		return automationFailure(
			http.StatusBadRequest,
			"automation.script_create_unsupported",
			protocol.FailureCategoryValidation,
			protocol.FailureEffectNotApplied,
			errPageScriptCreateUnsupported.Error(),
			err,
			"",
		)
	}
	if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
		effect := protocol.FailureEffectNotApplied
		if replayable {
			// A ledger entry may precede mutable Session/Room/delivery preflight;
			// readable validation text alone cannot disprove that earlier commit.
			effect = protocol.FailureEffectUnknown
		}
		return automationFailure(
			http.StatusBadRequest,
			"automation.task_create_invalid",
			protocol.FailureCategoryValidation,
			effect,
			err.Error(),
			err,
			"automation.edit_task",
		)
	}
	return automationFailure(
		http.StatusInternalServerError,
		"automation.task_create_failed",
		protocol.FailureCategoryInternal,
		protocol.FailureEffectUnknown,
		"定时任务创建失败",
		err,
		"automation.reload_tasks",
	)
}

func mapTaskDefinitionFailure(kind string, err error) automationHTTPFailure {
	prefix := "automation.task_" + kind
	if errors.Is(err, automationdomain.ErrTaskDeleting) {
		return taskDeletingFailure(err)
	}
	if errors.Is(err, errPageScriptUpdateUnsupported) {
		return automationFailure(
			http.StatusBadRequest,
			"automation.script_update_unsupported",
			protocol.FailureCategoryValidation,
			protocol.FailureEffectNotApplied,
			errPageScriptUpdateUnsupported.Error(),
			err,
			"",
		)
	}
	if errors.Is(err, automationdomain.ErrJobNotFound) {
		return automationFailure(
			http.StatusNotFound,
			"automation.task_not_found",
			protocol.FailureCategoryNotFound,
			protocol.FailureEffectNotApplied,
			"资源不存在",
			err,
			"automation.return_to_tasks",
		)
	}
	if errors.Is(err, automationdomain.ErrTaskSessionRebindRequired) {
		return automationFailure(
			http.StatusConflict,
			"automation.session_rebind_required",
			protocol.FailureCategoryConflict,
			protocol.FailureEffectNotApplied,
			err.Error(),
			err,
			"automation.rebind_session",
		)
	}
	if errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		return automationFailure(
			http.StatusConflict,
			"automation.configuration_conflict",
			protocol.FailureCategoryConflict,
			protocol.FailureEffectNotApplied,
			"任务配置已被其他操作修改，请重新打开后再保存",
			err,
			"automation.reload_task",
		)
	}
	if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
		return automationFailure(
			http.StatusBadRequest,
			prefix+"_invalid",
			protocol.FailureCategoryValidation,
			protocol.FailureEffectNotApplied,
			err.Error(),
			err,
			"automation.edit_task",
		)
	}
	return automationFailure(
		http.StatusInternalServerError,
		prefix+"_failed",
		protocol.FailureCategoryInternal,
		protocol.FailureEffectUnknown,
		"定时任务更新失败",
		err,
		"automation.reload_task",
	)
}

func mapResolvePermissionFailure(err error) automationHTTPFailure {
	switch {
	case errors.Is(err, automationdomain.ErrTaskDeleting):
		return taskDeletingFailure(err)
	case automationsvc.PermissionDecisionCommitted(err):
		// ResolvePermissionRequest 的审批事务已经提交。根因仍可 unwrap 为 stale
		// 等既有错误，但不能再向用户误报为“没有应用”。
		return automationFailure(http.StatusConflict, "automation.permission_decision_committed", protocol.FailureCategoryConflict, protocol.FailureEffectCommitted, "权限决定已保存，但任务后续状态需要刷新确认", err, "automation.reload_permissions")
	case errors.Is(err, automationdomain.ErrPermissionRequestNotFound):
		return automationFailure(http.StatusNotFound, "automation.permission_not_found", protocol.FailureCategoryNotFound, protocol.FailureEffectNotApplied, "审批请求不存在", err, "automation.reload_permissions")
	case errors.Is(err, automationdomain.ErrPermissionRequestResolved),
		errors.Is(err, automationdomain.ErrPermissionRequestStale):
		return automationFailure(http.StatusConflict, "automation.permission_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, err.Error(), err, "automation.reload_permissions")
	case errors.Is(err, automationdomain.ErrPermissionDecisionInvalid):
		return automationFailure(http.StatusBadRequest, "automation.permission_decision_invalid", protocol.FailureCategoryValidation, protocol.FailureEffectNotApplied, err.Error(), err, "automation.review_permission")
	case errors.Is(err, automationdomain.ErrPermissionConnectorNotReady):
		return automationFailure(http.StatusConflict, "automation.permission_connector_unavailable", protocol.FailureCategoryUnavailable, protocol.FailureEffectNotApplied, err.Error(), err, "automation.reconnect_connector")
	case handlershared.IsClientMessageError(err):
		return automationFailure(http.StatusBadRequest, "automation.permission_decision_invalid", protocol.FailureCategoryValidation, protocol.FailureEffectNotApplied, err.Error(), err, "automation.review_permission")
	default:
		return automationFailure(http.StatusInternalServerError, "automation.permission_decision_failed", protocol.FailureCategoryInternal, protocol.FailureEffectUnknown, "权限操作失败", err, "automation.reload_permissions")
	}
}

func mapResumePermissionFailure(err error) automationHTTPFailure {
	switch {
	case errors.Is(err, automationdomain.ErrTaskDeleting):
		return taskDeletingFailure(err)
	case errors.Is(err, automationdomain.ErrJobNotFound), errors.Is(err, automationdomain.ErrRunNotFound):
		return automationFailure(http.StatusNotFound, "automation.permission_resume_not_found", protocol.FailureCategoryNotFound, protocol.FailureEffectNotApplied, "资源不存在", err, "automation.reload_task")
	case errors.Is(err, automationdomain.ErrPermissionRunNotResumable):
		return automationFailure(http.StatusConflict, "automation.permission_resume_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, err.Error(), err, "automation.reload_task")
	case errors.Is(err, automationdomain.ErrPermissionRequestStale):
		// stale 也可能由 ready 状态写入后的 run 复核返回，不能只凭 sentinel 声明未应用。
		return automationFailure(http.StatusConflict, "automation.permission_resume_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectUnknown, err.Error(), err, "automation.reload_task")
	case errors.Is(err, automationdomain.ErrPermissionDecisionInvalid):
		return automationFailure(http.StatusBadRequest, "automation.permission_resume_invalid", protocol.FailureCategoryValidation, protocol.FailureEffectNotApplied, err.Error(), err, "automation.reload_task")
	case handlershared.IsClientMessageError(err):
		// 保留既有 400，但文本可读不能证明 resume 处在写入前。
		return automationFailure(http.StatusBadRequest, "automation.permission_resume_invalid", protocol.FailureCategoryValidation, protocol.FailureEffectUnknown, err.Error(), err, "automation.reload_task")
	default:
		// 保留既有 409；未知错误可能发生在 ready 状态写入或 run claim 之后。
		return automationFailure(http.StatusConflict, "automation.permission_resume_failed", protocol.FailureCategoryConflict, protocol.FailureEffectUnknown, err.Error(), err, "automation.reload_task")
	}
}

func mapRunTaskFailure(err error) automationHTTPFailure {
	switch {
	case errors.Is(err, automationdomain.ErrTaskDeleting):
		return taskDeletingFailure(err)
	case errors.Is(err, automationdomain.ErrConfigurationVersionConflict):
		return automationFailure(http.StatusConflict, "automation.run_configuration_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, "任务配置已变化，请刷新后再启动", err, "automation.reload_task")
	case errors.Is(err, automationdomain.ErrPermissionRequestStale):
		return automationFailure(http.StatusConflict, "automation.run_permission_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, "任务权限状态已变化，请刷新后再启动", err, "automation.reload_task")
	case errors.Is(err, automationdomain.ErrRuntimeCommandConflict):
		return automationFailure(http.StatusConflict, "automation.run_request_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, "这次启动与之前记录的操作不一致，请刷新运行记录后重试", err, "automation.reload_runs")
	case errors.Is(err, automationdomain.ErrJobNotFound):
		return automationFailure(http.StatusNotFound, "automation.run_task_not_found", protocol.FailureCategoryNotFound, protocol.FailureEffectNotApplied, "资源不存在", err, "automation.return_to_tasks")
	case errors.Is(err, automationdomain.ErrTaskSessionRebindRequired):
		return automationFailure(http.StatusConflict, "automation.run_session_rebind_required", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, err.Error(), err, "automation.rebind_session")
	case handlershared.IsClientMessageError(err), handlershared.IsStructuredSessionKeyError(err):
		return automationFailure(http.StatusBadRequest, "automation.run_task_invalid", protocol.FailureCategoryValidation, protocol.FailureEffectUnknown, err.Error(), err, "automation.reload_runs")
	default:
		return automationFailure(http.StatusInternalServerError, "automation.run_task_failed", protocol.FailureCategoryInternal, protocol.FailureEffectUnknown, "定时任务启动失败", err, "automation.reload_runs")
	}
}

func mapRecoverTaskFailure(err error) automationHTTPFailure {
	if errors.Is(err, automationdomain.ErrJobNotFound) {
		return automationFailure(http.StatusNotFound, "automation.recover_task_not_found", protocol.FailureCategoryNotFound, protocol.FailureEffectNotApplied, "资源不存在", err, "automation.return_to_tasks")
	}
	if errors.Is(err, automationdomain.ErrRunRecoveryConflict) {
		// exact 持久事务虽然已回滚，但此前对真实 DM/Room 的 interrupt 可能
		// 已生效；不能把整个恢复动作描述为未执行。
		return automationFailure(http.StatusConflict, "automation.recover_task_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectUnknown, "运行状态已变化，请刷新任务", err, "automation.reload_runs")
	}
	if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
		return automationFailure(http.StatusBadRequest, "automation.recover_task_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectUnknown, err.Error(), err, "automation.reload_runs")
	}
	return automationFailure(http.StatusInternalServerError, "automation.recover_task_failed", protocol.FailureCategoryInternal, protocol.FailureEffectUnknown, "运行恢复失败", err, "automation.reload_runs")
}

func mapRetryDeliveryFailure(err error) automationHTTPFailure {
	switch {
	case errors.Is(err, automationdomain.ErrTaskDeleting):
		return taskDeletingFailure(err)
	case errors.Is(err, automationdomain.ErrDeliveryRetryUnverified):
		return automationFailure(http.StatusConflict, "automation.delivery_retry_unverified", protocol.FailureCategoryConflict, protocol.FailureEffectUnknown, "上一次投递结果尚未确认，请先核对接收端和运行记录", err, "automation.review_delivery")
	case errors.Is(err, automationdomain.ErrDeliveryRetryConflict):
		return automationFailure(http.StatusConflict, "automation.delivery_retry_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, "投递状态已变化，请刷新运行记录后再操作", err, "automation.reload_runs")
	case errors.Is(err, automationdomain.ErrDeliveryRetryCompletionUnconfirmed):
		return automationFailure(http.StatusConflict, "automation.delivery_retry_completion_unconfirmed", protocol.FailureCategoryConflict, protocol.FailureEffectUnknown, "投递可能已经完成，但结果尚未确认，请先核对接收端和运行记录", err, "automation.review_delivery")
	case errors.Is(err, automationdomain.ErrConfigurationVersionConflict):
		return automationFailure(http.StatusConflict, "automation.delivery_configuration_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, "任务投递配置已变化，请刷新后再重试", err, "automation.reload_task")
	case errors.Is(err, automationdomain.ErrJobNotFound):
		return automationFailure(http.StatusNotFound, "automation.delivery_retry_not_found", protocol.FailureCategoryNotFound, protocol.FailureEffectNotApplied, "资源不存在", err, "automation.reload_runs")
	case errors.Is(err, automationdomain.ErrRunNotFound):
		// 同一 sentinel 既可能来自投递前 lookup，也可能来自外部投递与
		// MarkRunDelivery 之后的 reload；没有阶段化错误前必须保守处理。
		return automationFailure(http.StatusNotFound, "automation.delivery_retry_not_found", protocol.FailureCategoryNotFound, protocol.FailureEffectUnknown, "资源不存在", err, "automation.reload_runs")
	case errors.Is(err, automationdomain.ErrTaskSessionRebindRequired):
		return automationFailure(http.StatusConflict, "automation.delivery_session_rebind_required", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, err.Error(), err, "automation.rebind_session")
	case handlershared.IsClientMessageError(err), handlershared.IsStructuredSessionKeyError(err):
		return automationFailure(http.StatusBadRequest, "automation.delivery_retry_invalid", protocol.FailureCategoryValidation, protocol.FailureEffectUnknown, err.Error(), err, "automation.reload_runs")
	default:
		return automationFailure(http.StatusInternalServerError, "automation.delivery_retry_failed", protocol.FailureCategoryInternal, protocol.FailureEffectUnknown, "结果投递重试失败", err, "automation.reload_runs")
	}
}

func taskDeletingFailure(err error) automationHTTPFailure {
	return automationFailure(
		http.StatusConflict,
		"automation.task_deletion_in_progress",
		protocol.FailureCategoryConflict,
		protocol.FailureEffectNotApplied,
		"任务正在删除，不能执行此操作；请刷新任务列表查看结果",
		err,
		"automation.reload_tasks",
	)
}

func mapDeleteTaskFailure(err error) automationHTTPFailure {
	if automationsvc.TaskDeletionPrepared(err) && errors.Is(err, automationsvc.ErrExecutionAttemptOwnershipUnconfirmed) {
		return automationFailure(
			http.StatusConflict,
			"automation.delete_task_review_required",
			protocol.FailureCategoryConflict,
			protocol.FailureEffectAccepted,
			"任务已停止接受新运行，但原运行实例是否仍在执行暂时无法确认，任务数据尚未删除。请确认原运行实例已经停止，再由管理员处理；系统不会自动重放已经发生的操作",
			err,
			"automation.review_task_deletion",
		)
	}
	if automationsvc.TaskDeletionPrepared(err) {
		return automationFailure(
			http.StatusConflict,
			"automation.delete_task_in_progress",
			protocol.FailureCategoryConflict,
			protocol.FailureEffectAccepted,
			"任务已停止接受新运行，系统正在继续清理。已经发生的外部操作不会撤回，也不会自动重放；请稍后刷新查看结果",
			err,
			"automation.reload_tasks",
		)
	}
	if errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		return automationFailure(http.StatusConflict, "automation.delete_configuration_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, "任务配置已变化，请刷新后再删除", err, "automation.reload_task")
	}
	if errors.Is(err, automationdomain.ErrJobNotFound) {
		return automationFailure(http.StatusNotFound, "automation.delete_task_not_found", protocol.FailureCategoryNotFound, protocol.FailureEffectNotApplied, "资源不存在", err, "automation.return_to_tasks")
	}
	// 清理阶段与最终删除不能被推断为未应用。
	return automationFailure(http.StatusInternalServerError, "automation.delete_task_failed", protocol.FailureCategoryInternal, protocol.FailureEffectUnknown, "定时任务删除失败", err, "automation.reload_tasks")
}

func mapConfirmDeletionStoppedFailure(err error) automationHTTPFailure {
	switch {
	case errors.Is(err, errExpectedConfigurationVersionRequired):
		return automationFailure(http.StatusBadRequest, "automation.deletion_confirmation_invalid", protocol.FailureCategoryValidation, protocol.FailureEffectNotApplied, "请先刷新任务，再确认原执行实例已经停止", err, "automation.reload_task")
	case automationsvc.TaskDeletionPrepared(err):
		return automationFailure(http.StatusConflict, "automation.deletion_confirmation_in_progress", protocol.FailureCategoryConflict, protocol.FailureEffectAccepted, "停止确认已受理，但删除收尾尚未完成；不会重新运行或投递，请刷新后再处理", err, "automation.reload_tasks")
	case errors.Is(err, automationdomain.ErrConfigurationVersionConflict):
		return automationFailure(http.StatusConflict, "automation.deletion_confirmation_conflict", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, "任务状态已变化，请刷新后重新确认", err, "automation.reload_task")
	case errors.Is(err, automationdomain.ErrTaskDeletionReviewConflict):
		return automationFailure(http.StatusConflict, "automation.deletion_confirmation_not_required", protocol.FailureCategoryConflict, protocol.FailureEffectNotApplied, "任务当前不在等待原执行实例停止确认，请刷新查看最新状态", err, "automation.reload_tasks")
	case errors.Is(err, automationdomain.ErrJobNotFound):
		return automationFailure(http.StatusNotFound, "automation.deletion_confirmation_not_found", protocol.FailureCategoryNotFound, protocol.FailureEffectNotApplied, "任务已经不存在", err, "automation.return_to_tasks")
	default:
		return automationFailure(http.StatusInternalServerError, "automation.deletion_confirmation_failed", protocol.FailureCategoryInternal, protocol.FailureEffectUnknown, "停止确认未能完成，请刷新任务状态后再处理", err, "automation.reload_tasks")
	}
}

func automationFailure(
	status int,
	code string,
	category protocol.FailureCategory,
	effect protocol.FailureEffect,
	detail string,
	cause error,
	action string,
) automationHTTPFailure {
	var resolution *protocol.FailureResolution
	if action != "" {
		resolution = &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: action,
		}
	}
	return automationHTTPFailure{
		status: status,
		spec: handlershared.FailureSpec{
			Code:       code,
			Category:   category,
			Effect:     effect,
			Detail:     detail,
			Cause:      cause,
			Resolution: resolution,
		},
	}
}
