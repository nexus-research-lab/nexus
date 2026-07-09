package automation

import (
	"net/http"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"

	"github.com/go-chi/chi/v5"
)

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
	item, err := h.automation.UpdateHeartbeat(request.Context(), chi.URLParam(request, "agent_id"), automationdomain.HeartbeatUpdateInput{
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
	item, err := h.automation.WakeHeartbeat(request.Context(), chi.URLParam(request, "agent_id"), automationdomain.HeartbeatWakeInput{
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
