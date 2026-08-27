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
	Enabled bool `json:"enabled"`
}

type scheduledTaskRecoverPayload struct {
	RunID string `json:"run_id,omitempty"`
}

type scheduledTaskRetryDeliveryPayload struct{}

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
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleListPermissionRequests 返回当前 owner 的持久自动化交互请求。
func (h *Handlers) HandleListPermissionRequests(writer http.ResponseWriter, request *http.Request) {
	items, err := h.automation.ListPermissionRequests(
		request.Context(),
		strings.TrimSpace(request.URL.Query().Get("status")),
		strings.TrimSpace(request.URL.Query().Get("job_id")),
	)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleResolvePermissionRequest 原子提交 allow-once / allow-task / deny / retry。
func (h *Handlers) HandleResolvePermissionRequest(writer http.ResponseWriter, request *http.Request) {
	var payload automationPermissionDecisionPayload
	if !h.api.BindJSON(writer, request, &payload) {
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
		switch {
		case errors.Is(err, automationdomain.ErrPermissionRequestNotFound):
			h.api.WriteFailure(writer, http.StatusNotFound, "审批请求不存在")
		case errors.Is(err, automationdomain.ErrPermissionRequestResolved),
			errors.Is(err, automationdomain.ErrPermissionRequestStale):
			h.api.WriteFailure(writer, http.StatusConflict, err.Error())
		case errors.Is(err, automationdomain.ErrPermissionDecisionInvalid):
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		case errors.Is(err, automationdomain.ErrPermissionConnectorNotReady):
			h.api.WriteFailure(writer, http.StatusConflict, err.Error())
		case handlershared.IsClientMessageError(err):
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		default:
			h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		}
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleResumePermissionRun 明确确认重放 ready_to_retry 的 logical run。
func (h *Handlers) HandleResumePermissionRun(writer http.ResponseWriter, request *http.Request) {
	var payload automationPermissionResumePayload
	if !h.api.BindJSON(writer, request, &payload) {
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
		switch {
		case errors.Is(err, automationdomain.ErrJobNotFound), errors.Is(err, automationdomain.ErrRunNotFound):
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		case errors.Is(err, automationdomain.ErrPermissionRunNotResumable),
			errors.Is(err, automationdomain.ErrPermissionRequestStale):
			h.api.WriteFailure(writer, http.StatusConflict, err.Error())
		case errors.Is(err, automationdomain.ErrPermissionDecisionInvalid):
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		case handlershared.IsClientMessageError(err):
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		default:
			h.api.WriteFailure(writer, http.StatusConflict, err.Error())
		}
		return
	}
	h.api.WriteSuccess(writer, result)
}

func (h *Handlers) HandleCreateScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskCreatePayload
	if !h.api.BindJSON(writer, request, &payload) {
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
		h.api.WriteFailure(writer, http.StatusBadRequest, "页面暂不支持创建脚本任务")
		return
	}
	source := automationdomain.Source{Kind: automationdomain.SourceKindUserPage}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	item, err := h.automation.CreateTask(request.Context(), automationdomain.CreateJobInput{
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
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleUpdateScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskUpdatePayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	if payload.ExecutionKind != nil &&
		automationdomain.NormalizeExecutionKind(*payload.ExecutionKind) == automationdomain.ExecutionKindScript {
		h.api.WriteFailure(writer, http.StatusBadRequest, "页面暂不支持创建或修改脚本任务")
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
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if errors.Is(err, automationdomain.ErrTaskSessionRebindRequired) {
			h.api.WriteFailure(writer, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
			h.api.WriteFailure(writer, http.StatusConflict, "任务配置已被其他操作修改，请重新打开后再保存")
			return
		}
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleDeleteScheduledTask(writer http.ResponseWriter, request *http.Request) {
	result, err := h.automation.DeleteTask(request.Context(), chi.URLParam(request, "job_id"))
	if err != nil {
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, result)
}

func (h *Handlers) HandleRunScheduledTask(writer http.ResponseWriter, request *http.Request) {
	item, err := h.automation.RunTaskNow(request.Context(), chi.URLParam(request, "job_id"))
	if err != nil {
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if errors.Is(err, automationdomain.ErrTaskSessionRebindRequired) {
			h.api.WriteFailure(writer, http.StatusConflict, err.Error())
			return
		}
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleRecoverScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskRecoverPayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.automation.RecoverTaskRunningRun(request.Context(), chi.URLParam(request, "job_id"), payload.RunID)
	if err != nil {
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleUpdateScheduledTaskStatus(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskStatusPayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.automation.UpdateTaskStatus(request.Context(), chi.URLParam(request, "job_id"), payload.Enabled)
	if err != nil {
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if errors.Is(err, automationdomain.ErrTaskSessionRebindRequired) {
			h.api.WriteFailure(writer, http.StatusConflict, err.Error())
			return
		}
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
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
				Resolution: &protocol.FailureResolution{
					Actor:  protocol.FailureRecoveryActorUser,
					Action: "automation.return_to_tasks",
				},
			})
			return
		}
		h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
			Code:     "automation.run_history_unavailable",
			Category: protocol.FailureCategoryInternal,
			Effect:   protocol.FailureEffectNotApplicable,
			Detail:   "运行历史读取失败",
			Cause:    err,
			Resolution: &protocol.FailureResolution{
				Actor:  protocol.FailureRecoveryActorUser,
				Action: "automation.reload_run_history",
			},
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
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
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
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
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
		if errors.Is(err, automationdomain.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		message := strings.ToLower(err.Error())
		if handlershared.IsClientMessageError(err) || strings.Contains(message, "date must be") || strings.Contains(message, "invalid timezone") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleRetryScheduledTaskRunDelivery(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskRetryDeliveryPayload
	if !h.api.BindJSONAllowEmpty(writer, request, &payload) {
		return
	}
	item, err := h.automation.RetryRunDelivery(
		request.Context(),
		chi.URLParam(request, "job_id"),
		chi.URLParam(request, "run_id"),
	)
	if err != nil {
		if errors.Is(err, automationdomain.ErrJobNotFound) || errors.Is(err, automationdomain.ErrRunNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if errors.Is(err, automationdomain.ErrTaskSessionRebindRequired) {
			h.api.WriteFailure(writer, http.StatusConflict, err.Error())
			return
		}
		if handlershared.IsClientMessageError(err) || handlershared.IsStructuredSessionKeyError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
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
