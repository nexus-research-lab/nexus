package automation

import (
	"errors"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"

	"github.com/go-chi/chi/v5"
)

type scheduledTaskCreatePayload struct {
	Name          string                   `json:"name"`
	AgentID       string                   `json:"agent_id"`
	Schedule      protocol.Schedule        `json:"schedule"`
	Instruction   string                   `json:"instruction"`
	SessionTarget *protocol.SessionTarget  `json:"session_target,omitempty"`
	Delivery      *protocol.DeliveryTarget `json:"delivery,omitempty"`
	Source        *protocol.Source         `json:"source,omitempty"`
	Enabled       *bool                    `json:"enabled,omitempty"`
}

type scheduledTaskUpdatePayload struct {
	Name          *string                  `json:"name,omitempty"`
	Schedule      *protocol.Schedule       `json:"schedule,omitempty"`
	Instruction   *string                  `json:"instruction,omitempty"`
	SessionTarget *protocol.SessionTarget  `json:"session_target,omitempty"`
	Delivery      *protocol.DeliveryTarget `json:"delivery,omitempty"`
	Source        *protocol.Source         `json:"source,omitempty"`
	Enabled       *bool                    `json:"enabled,omitempty"`
}

type scheduledTaskStatusPayload struct {
	Enabled bool `json:"enabled"`
}

type heartbeatUpdatePayload struct {
	Enabled      bool   `json:"enabled"`
	EverySeconds int    `json:"every_seconds"`
	TargetMode   string `json:"target_mode"`
	AckMaxChars  int    `json:"ack_max_chars"`
}

type heartbeatWakePayload struct {
	Mode string  `json:"mode"`
	Text *string `json:"text,omitempty"`
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

func (h *Handlers) HandleCreateScheduledTask(writer http.ResponseWriter, request *http.Request) {
	var payload scheduledTaskCreatePayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	sessionTarget := protocol.SessionTarget{}
	if payload.SessionTarget != nil {
		sessionTarget = *payload.SessionTarget
	}
	delivery := protocol.DeliveryTarget{}
	if payload.Delivery != nil {
		delivery = *payload.Delivery
	}
	source := protocol.Source{}
	if payload.Source != nil {
		source = *payload.Source
	}
	source.Kind = protocol.SourceKindUserPage
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	item, err := h.automation.CreateTask(request.Context(), protocol.CreateJobInput{
		Name:          payload.Name,
		AgentID:       payload.AgentID,
		Schedule:      payload.Schedule,
		Instruction:   payload.Instruction,
		SessionTarget: sessionTarget,
		Delivery:      delivery,
		Source:        source,
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
	item, err := h.automation.UpdateTask(request.Context(), chi.URLParam(request, "job_id"), protocol.UpdateJobInput{
		Name:          payload.Name,
		Schedule:      payload.Schedule,
		Instruction:   payload.Instruction,
		SessionTarget: payload.SessionTarget,
		Delivery:      payload.Delivery,
		Source:        payload.Source,
		Enabled:       payload.Enabled,
	})
	if err != nil {
		if errors.Is(err, protocol.ErrJobNotFound) {
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
	if err := h.automation.DeleteTask(request.Context(), chi.URLParam(request, "job_id")); err != nil {
		if errors.Is(err, protocol.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"job_id": chi.URLParam(request, "job_id")})
}

func (h *Handlers) HandleRunScheduledTask(writer http.ResponseWriter, request *http.Request) {
	item, err := h.automation.RunTaskNow(request.Context(), chi.URLParam(request, "job_id"))
	if err != nil {
		if errors.Is(err, protocol.ErrJobNotFound) {
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
		if errors.Is(err, protocol.ErrJobNotFound) {
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
		if errors.Is(err, protocol.ErrJobNotFound) {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

func (h *Handlers) HandleGetHeartbeat(writer http.ResponseWriter, request *http.Request) {
	item, err := h.automation.GetHeartbeatStatus(request.Context(), chi.URLParam(request, "agent_id"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "agent not found") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleUpdateHeartbeat(writer http.ResponseWriter, request *http.Request) {
	var payload heartbeatUpdatePayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.automation.UpdateHeartbeat(request.Context(), chi.URLParam(request, "agent_id"), protocol.HeartbeatUpdateInput{
		Enabled:      payload.Enabled,
		EverySeconds: payload.EverySeconds,
		TargetMode:   payload.TargetMode,
		AckMaxChars:  payload.AckMaxChars,
	})
	if err != nil {
		if handlershared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "agent not found") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleWakeHeartbeat(writer http.ResponseWriter, request *http.Request) {
	var payload heartbeatWakePayload
	if !h.api.BindJSONAllowEmpty(writer, request, &payload) {
		return
	}
	item, err := h.automation.WakeHeartbeat(request.Context(), chi.URLParam(request, "agent_id"), protocol.HeartbeatWakeRequest{
		Mode: payload.Mode,
		Text: payload.Text,
	})
	if err != nil {
		if handlershared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "agent not found") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}
