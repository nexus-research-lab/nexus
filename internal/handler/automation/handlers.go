// INPUT: owner-scoped Automation HTTP 参数与领域服务结果。
// OUTPUT: 任务/运行/权限资源响应；删除按配置版本、立即运行按领域 request identity、人工重投按配置版本+已见 attempts 精确防重。
// POS: Automation HTTP transport；不暴露私有删除或投递 token。
package automation

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"

	"github.com/go-chi/chi/v5"
)

type scheduledTaskCreatePayload struct {
	RequestID      string                           `json:"request_id,omitempty"`
	Name           string                           `json:"name"`
	AgentID        string                           `json:"agent_id"`
	Schedule       automationdomain.Schedule        `json:"schedule"`
	Instruction    string                           `json:"instruction"`
	ExecutionKind  string                           `json:"execution_kind,omitempty"`
	PermissionMode string                           `json:"permission_mode,omitempty"`
	SessionTarget  *automationdomain.SessionTarget  `json:"session_target,omitempty"`
	Delivery       *automationdomain.DeliveryTarget `json:"delivery,omitempty"`
	Source         *automationdomain.Source         `json:"source,omitempty"`
	OverlapPolicy  string                           `json:"overlap_policy,omitempty"`
	ExpiresAt      *time.Time                       `json:"expires_at,omitempty"`
	Enabled        *bool                            `json:"enabled,omitempty"`
}

type scheduledTaskUpdatePayload struct {
	ExpectedConfigurationVersion *int64                           `json:"expected_configuration_version,omitempty"`
	Name                         *string                          `json:"name,omitempty"`
	AgentID                      *string                          `json:"agent_id,omitempty"`
	Schedule                     *automationdomain.Schedule       `json:"schedule,omitempty"`
	Instruction                  *string                          `json:"instruction,omitempty"`
	ExecutionKind                *string                          `json:"execution_kind,omitempty"`
	PermissionMode               *string                          `json:"permission_mode,omitempty"`
	SessionTarget                *automationdomain.SessionTarget  `json:"session_target,omitempty"`
	Delivery                     *automationdomain.DeliveryTarget `json:"delivery,omitempty"`
	OverlapPolicy                *string                          `json:"overlap_policy,omitempty"`
	ExpiresAt                    *time.Time                       `json:"expires_at,omitempty"`
	ClearExpiresAt               bool                             `json:"clear_expires_at,omitempty"`
	Enabled                      *bool                            `json:"enabled,omitempty"`
}

type scheduledTaskStatusPayload struct {
	Enabled                      *bool  `json:"enabled"`
	ExpectedConfigurationVersion *int64 `json:"expected_configuration_version,omitempty"`
}

type scheduledTaskRecoverPayload struct {
	RunID string `json:"run_id,omitempty"`
}

type scheduledTaskRunPayload struct {
	ExpectedConfigurationVersion *int64 `json:"expected_configuration_version,omitempty"`
	RequestID                    string `json:"request_id,omitempty"`
}

type scheduledTaskDeletePayload struct {
	ExpectedConfigurationVersion *int64 `json:"expected_configuration_version,omitempty"`
}

type scheduledTaskDeletionConfirmStoppedPayload struct {
	ExpectedConfigurationVersion *int64 `json:"expected_configuration_version"`
}

type scheduledTaskRetryDeliveryPayload struct {
	ExpectedConfigurationVersion *int64 `json:"expected_configuration_version,omitempty"`
	ExpectedDeliveryAttempts     *int   `json:"expected_delivery_attempts,omitempty"`
	ConfirmUnverifiedAttempt     bool   `json:"confirm_unverified_attempt,omitempty"`
}

type automationPermissionDecisionPayload struct {
	Decision       string `json:"decision"`
	JobID          string `json:"job_id"`
	RunID          string `json:"run_id"`
	PolicyRevision int    `json:"policy_revision"`
}

type automationPermissionResumePayload struct {
	RequestID      string `json:"request_id"`
	PolicyRevision int    `json:"policy_revision"`
}

// Handlers 封装自动化域 HTTP handlers。
type Handlers struct {
	api        *handlershared.API
	automation *automationsvc.Service
}

// New 创建自动化 handlers。
func New(api *handlershared.API, automation *automationsvc.Service) *Handlers {
	return &Handlers{
		api:        api,
		automation: automation,
	}
}

func (h *Handlers) HandleListScheduledTasks(writer http.ResponseWriter, request *http.Request) {
	items, err := h.automation.ListTasks(request.Context(), strings.TrimSpace(request.URL.Query().Get("agent_id")))
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureListTasks, err)
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleGetScheduledTaskCreateRequest 供页面刷新后核对创建是否已提交。
func (h *Handlers) HandleGetScheduledTaskCreateRequest(writer http.ResponseWriter, request *http.Request) {
	result, err := h.automation.GetTaskCreateRequestStatus(
		request.Context(),
		chi.URLParam(request, "request_id"),
	)
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureGetCreateRequest, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleListPermissionRequests 返回当前 owner 的持久自动化交互请求。
func (h *Handlers) HandleListPermissionRequests(writer http.ResponseWriter, request *http.Request) {
	items, err := h.automation.ListPermissionRequests(
		request.Context(),
		strings.TrimSpace(request.URL.Query().Get("status")),
		strings.TrimSpace(request.URL.Query().Get("job_id")),
	)
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureListPermissions, err)
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleResolvePermissionRequest 原子提交 allow-once / allow-task / deny / retry。
func (h *Handlers) HandleResolvePermissionRequest(writer http.ResponseWriter, request *http.Request) {
	var payload automationPermissionDecisionPayload
	if !h.bindAutomationJSON(writer, request, &payload, false) {
		return
	}
	result, err := h.automation.ResolvePermissionRequest(
		request.Context(),
		chi.URLParam(request, "request_id"),
		automationdomain.PermissionDecisionInput{
			Decision:       payload.Decision,
			JobID:          payload.JobID,
			RunID:          payload.RunID,
			PolicyRevision: payload.PolicyRevision,
		},
	)
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureResolvePermission, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleResumePermissionRun 明确确认重放 ready_to_retry 的 logical run。
func (h *Handlers) HandleResumePermissionRun(writer http.ResponseWriter, request *http.Request) {
	var payload automationPermissionResumePayload
	if !h.bindAutomationJSON(writer, request, &payload, false) {
		return
	}
	result, err := h.automation.ResumePermissionRun(
		request.Context(),
		chi.URLParam(request, "job_id"),
		chi.URLParam(request, "run_id"),
		automationdomain.PermissionResumeInput{
			RequestID:      payload.RequestID,
			PolicyRevision: payload.PolicyRevision,
		},
	)
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureResumePermission, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

func (h *Handlers) HandleCreateScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskCreatePayload
	if !h.bindAutomationJSON(writer, request, &payload, false) {
		return
	}
	sessionTarget := automationdomain.SessionTarget{}
	if payload.SessionTarget != nil {
		sessionTarget = *payload.SessionTarget
	}
	delivery := automationdomain.DeliveryTarget{}
	if payload.Delivery != nil {
		delivery = *payload.Delivery
	}
	if automationdomain.NormalizeExecutionKind(payload.ExecutionKind) == automationdomain.ExecutionKindScript {
		h.writeAutomationFailure(writer, request, automationFailureCreateTask, errPageScriptCreateUnsupported)
		return
	}
	source := automationdomain.Source{Kind: automationdomain.SourceKindUserPage}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	item, err := h.automation.CreateTask(request.Context(), automationdomain.CreateJobInput{
		RequestID:      payload.RequestID,
		Name:           payload.Name,
		AgentID:        payload.AgentID,
		Schedule:       payload.Schedule,
		Instruction:    payload.Instruction,
		ExecutionKind:  payload.ExecutionKind,
		PermissionMode: payload.PermissionMode,
		SessionTarget:  sessionTarget,
		Delivery:       delivery,
		Source:         source,
		OverlapPolicy:  payload.OverlapPolicy,
		ExpiresAt:      payload.ExpiresAt,
		Enabled:        enabled,
	})
	if err != nil {
		operation := automationFailureCreateTask
		if strings.TrimSpace(payload.RequestID) != "" {
			operation = automationFailureReplayCreateTask
		}
		h.writeAutomationFailure(writer, request, operation, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleUpdateScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskUpdatePayload
	if !h.bindAutomationJSON(writer, request, &payload, false) {
		return
	}
	if payload.ExecutionKind != nil &&
		automationdomain.NormalizeExecutionKind(*payload.ExecutionKind) == automationdomain.ExecutionKindScript {
		h.writeAutomationFailure(writer, request, automationFailureUpdateTask, errPageScriptUpdateUnsupported)
		return
	}
	var deliveryGrant *automationdomain.Source
	if payload.Delivery != nil {
		source := automationdomain.Source{Kind: automationdomain.SourceKindUserPage}
		deliveryGrant = &source
	}
	input := automationdomain.UpdateJobInput{
		Name:           payload.Name,
		AgentID:        payload.AgentID,
		Schedule:       payload.Schedule,
		Instruction:    payload.Instruction,
		ExecutionKind:  payload.ExecutionKind,
		PermissionMode: payload.PermissionMode,
		SessionTarget:  payload.SessionTarget,
		Delivery:       payload.Delivery,
		Source:         deliveryGrant,
		OverlapPolicy:  payload.OverlapPolicy,
		ExpiresAt:      payload.ExpiresAt,
		ClearExpiresAt: payload.ClearExpiresAt,
		Enabled:        payload.Enabled,
	}
	var item *automationdomain.ScheduledTask
	var err error
	if payload.ExpectedConfigurationVersion != nil {
		item, err = h.automation.UpdateTaskAtVersion(
			request.Context(),
			chi.URLParam(request, "job_id"),
			*payload.ExpectedConfigurationVersion,
			input,
		)
	} else {
		item, err = h.automation.UpdateTask(
			request.Context(),
			chi.URLParam(request, "job_id"),
			input,
		)
	}
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureUpdateTask, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleDeleteScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskDeletePayload
	if !h.bindAutomationJSON(writer, request, &payload, true) {
		return
	}
	var result *automationdomain.DeleteJobResult
	var err error
	if payload.ExpectedConfigurationVersion != nil {
		result, err = h.automation.DeleteTaskAtVersion(
			request.Context(),
			chi.URLParam(request, "job_id"),
			*payload.ExpectedConfigurationVersion,
		)
	} else {
		result, err = h.automation.DeleteTask(request.Context(), chi.URLParam(request, "job_id"))
	}
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureDeleteTask, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleConfirmScheduledTaskDeletionStopped 只收尾 owner 已确认原执行实例停止的
// review_required 删除；私有 deletion token 永不进入 HTTP contract。
func (h *Handlers) HandleConfirmScheduledTaskDeletionStopped(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskDeletionConfirmStoppedPayload
	if !h.bindAutomationJSON(writer, request, &payload, false) {
		return
	}
	if payload.ExpectedConfigurationVersion == nil {
		h.writeAutomationFailure(
			writer,
			request,
			automationFailureConfirmDeletionStopped,
			errExpectedConfigurationVersionRequired,
		)
		return
	}
	result, err := h.automation.ConfirmTaskDeletionStoppedAtVersion(
		request.Context(),
		chi.URLParam(request, "job_id"),
		*payload.ExpectedConfigurationVersion,
	)
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureConfirmDeletionStopped, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

func (h *Handlers) HandleRunScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskRunPayload
	if !h.bindAutomationJSON(writer, request, &payload, true) {
		return
	}
	var item *automationdomain.ExecutionResult
	var err error
	if strings.TrimSpace(payload.RequestID) != "" && payload.ExpectedConfigurationVersion != nil {
		item, err = h.automation.RunTaskNowAtVersionWithRequest(
			request.Context(),
			chi.URLParam(request, "job_id"),
			*payload.ExpectedConfigurationVersion,
			payload.RequestID,
		)
	} else if strings.TrimSpace(payload.RequestID) != "" {
		item, err = h.automation.RunTaskNowWithRequest(
			request.Context(), chi.URLParam(request, "job_id"), payload.RequestID,
		)
	} else if payload.ExpectedConfigurationVersion != nil {
		item, err = h.automation.RunTaskNowAtVersion(
			request.Context(), chi.URLParam(request, "job_id"), *payload.ExpectedConfigurationVersion,
		)
	} else {
		item, err = h.automation.RunTaskNow(request.Context(), chi.URLParam(request, "job_id"))
	}
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureRunTask, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleRecoverScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskRecoverPayload
	if !h.bindAutomationJSON(writer, request, &payload, false) {
		return
	}
	item, err := h.automation.RecoverTaskRunningRun(request.Context(), chi.URLParam(request, "job_id"), payload.RunID)
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureRecoverTask, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleUpdateScheduledTaskStatus(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskStatusPayload
	if !h.bindAutomationJSON(writer, request, &payload, false) {
		return
	}
	if payload.Enabled == nil {
		h.writeAutomationFailure(writer, request, automationFailureUpdateStatus, errTaskStatusEnabledRequired)
		return
	}
	jobID := chi.URLParam(request, "job_id")
	enabled := *payload.Enabled
	var item *automationdomain.ScheduledTask
	var err error
	if payload.ExpectedConfigurationVersion != nil {
		item, err = h.automation.UpdateTaskAtVersion(
			request.Context(),
			jobID,
			*payload.ExpectedConfigurationVersion,
			automationdomain.UpdateJobInput{Enabled: &enabled},
		)
	} else {
		item, err = h.automation.UpdateTaskStatus(request.Context(), jobID, enabled)
	}
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureUpdateStatus, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleListScheduledTaskRuns(writer http.ResponseWriter, request *http.Request) {
	items, err := h.automation.ListTaskRuns(request.Context(), chi.URLParam(request, "job_id"))
	if err != nil {
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteError(writer, request, http.StatusNotFound, handlershared.FailureSpec{
				Code:     "automation.run_history_not_found",
				Category: protocol.FailureCategoryNotFound,
				Effect:   protocol.FailureEffectNotApplicable,
				Detail:   "资源不存在",
				Cause:    err,
			})
			return
		}
		h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
			Code:     "automation.run_history_unavailable",
			Category: protocol.FailureCategoryInternal,
			Effect:   protocol.FailureEffectNotApplicable,
			Detail:   "运行历史读取失败",
			Cause:    err,
		})
		return
	}
	h.api.WriteSuccess(writer, items)
}

func (h *Handlers) HandleGetScheduledTaskStatus(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	item, err := h.automation.GetTaskStatus(
		request.Context(),
		chi.URLParam(request, "job_id"),
		queryInt(query.Get("run_limit")),
		queryInt(query.Get("event_limit")),
	)
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureGetStatus, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleListScheduledTaskEvents(writer http.ResponseWriter, request *http.Request) {
	items, err := h.automation.ListTaskEvents(
		request.Context(),
		chi.URLParam(request, "job_id"),
		queryInt(request.URL.Query().Get("limit")),
	)
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureListEvents, err)
		return
	}
	h.api.WriteSuccess(writer, items)
}

func (h *Handlers) HandleGetScheduledTaskDailyReport(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	item, err := h.automation.GetDailyReport(request.Context(), automationdomain.ScheduledTaskDailyReportInput{
		Date:     strings.TrimSpace(query.Get("date")),
		Timezone: strings.TrimSpace(query.Get("timezone")),
		AgentID:  strings.TrimSpace(query.Get("agent_id")),
		JobID:    strings.TrimSpace(query.Get("job_id")),
	})
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureDailyReport, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleRetryScheduledTaskRunDelivery(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskRetryDeliveryPayload
	if !h.bindAutomationJSON(writer, request, &payload, true) {
		return
	}
	var item *automationdomain.ScheduledTaskRun
	var err error
	if payload.ConfirmUnverifiedAttempt {
		expectedVersion := int64(0)
		if payload.ExpectedConfigurationVersion != nil {
			expectedVersion = *payload.ExpectedConfigurationVersion
		}
		expectedAttempts := -1
		if payload.ExpectedDeliveryAttempts != nil {
			expectedAttempts = *payload.ExpectedDeliveryAttempts
		}
		item, err = h.automation.RetryUnverifiedRunDeliveryAtVersion(
			request.Context(),
			chi.URLParam(request, "job_id"),
			chi.URLParam(request, "run_id"),
			expectedVersion,
			expectedAttempts,
		)
	} else if payload.ExpectedConfigurationVersion != nil && payload.ExpectedDeliveryAttempts != nil {
		item, err = h.automation.RetryRunDeliveryAtVersionAndAttempts(
			request.Context(),
			chi.URLParam(request, "job_id"),
			chi.URLParam(request, "run_id"),
			*payload.ExpectedConfigurationVersion,
			*payload.ExpectedDeliveryAttempts,
		)
	} else if payload.ExpectedConfigurationVersion != nil {
		item, err = h.automation.RetryRunDeliveryAtVersion(
			request.Context(),
			chi.URLParam(request, "job_id"),
			chi.URLParam(request, "run_id"),
			*payload.ExpectedConfigurationVersion,
		)
	} else {
		item, err = h.automation.RetryRunDelivery(
			request.Context(),
			chi.URLParam(request, "job_id"),
			chi.URLParam(request, "run_id"),
		)
	}
	if err != nil {
		h.writeAutomationFailure(writer, request, automationFailureRetryDelivery, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func queryInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}
