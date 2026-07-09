package automation

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"

	"github.com/go-chi/chi/v5"
)

type scheduledTaskCreatePayload struct {
	Name          string                           `json:"name"`
	AgentID       string                           `json:"agent_id"`
	Schedule      automationdomain.Schedule        `json:"schedule"`
	Instruction   string                           `json:"instruction"`
	ExecutionKind string                           `json:"execution_kind,omitempty"`
	SessionTarget *automationdomain.SessionTarget  `json:"session_target,omitempty"`
	Delivery      *automationdomain.DeliveryTarget `json:"delivery,omitempty"`
	Source        *automationdomain.Source         `json:"source,omitempty"`
	OverlapPolicy string                           `json:"overlap_policy,omitempty"`
	Enabled       *bool                            `json:"enabled,omitempty"`
}

type scheduledTaskUpdatePayload struct {
	Name          *string                          `json:"name,omitempty"`
	Schedule      *automationdomain.Schedule       `json:"schedule,omitempty"`
	Instruction   *string                          `json:"instruction,omitempty"`
	ExecutionKind *string                          `json:"execution_kind,omitempty"`
	SessionTarget *automationdomain.SessionTarget  `json:"session_target,omitempty"`
	Delivery      *automationdomain.DeliveryTarget `json:"delivery,omitempty"`
	Source        *automationdomain.Source         `json:"source,omitempty"`
	OverlapPolicy *string                          `json:"overlap_policy,omitempty"`
	Enabled       *bool                            `json:"enabled,omitempty"`
}

type scheduledTaskStatusPayload struct {
	Enabled bool `json:"enabled"`
}

type scheduledTaskRecoverPayload struct {
	RunID string `json:"run_id,omitempty"`
}

type scheduledTaskRetryDeliveryPayload struct{}

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
	source := automationdomain.Source{}
	if payload.Source != nil {
		source = *payload.Source
	}
	source.Kind = automationdomain.SourceKindUserPage
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	item, err := h.automation.CreateTask(request.Context(), automationdomain.CreateJobInput{
		Name:          payload.Name,
		AgentID:       payload.AgentID,
		Schedule:      payload.Schedule,
		Instruction:   payload.Instruction,
		ExecutionKind: payload.ExecutionKind,
		SessionTarget: sessionTarget,
		Delivery:      delivery,
		Source:        source,
		OverlapPolicy: payload.OverlapPolicy,
		Enabled:       enabled,
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
	item, err := h.automation.UpdateTask(request.Context(), chi.URLParam(request, "job_id"), automationdomain.UpdateJobInput{
		Name:          payload.Name,
		Schedule:      payload.Schedule,
		Instruction:   payload.Instruction,
		ExecutionKind: payload.ExecutionKind,
		SessionTarget: payload.SessionTarget,
		Delivery:      payload.Delivery,
		Source:        payload.Source,
		OverlapPolicy: payload.OverlapPolicy,
		Enabled:       payload.Enabled,
	})
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
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
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
	item, err := h.automation.GetDailyReport(request.Context(), automationdomain.CronDailyReportInput{
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
