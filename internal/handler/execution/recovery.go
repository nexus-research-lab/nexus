// INPUT: Authenticated user and exact WorkGraph recovery request.
// OUTPUT: Durable intake acknowledgement or a rejected/stale target response.
// POS: HTTP boundary; never grants agent mutation capabilities or replays tools.
package execution

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type workRecoveryRequester interface {
	RequestWorkRecovery(context.Context, string, string, protocol.RequestExecutionRecovery) (*protocol.ExecutionRecoveryReceipt, error)
	GetWorkRecoveryReceipt(context.Context, string, string, string, string) (*protocol.ExecutionRecoveryReceipt, error)
}

func (h *Handlers) HandleGetWorkRecoveryReceipt(w http.ResponseWriter, r *http.Request) {
	reader, ok := h.executions.(workRecoveryRequester)
	if !ok {
		h.api.WriteFailure(w, http.StatusServiceUnavailable, "WorkGraph recovery is unavailable")
		return
	}
	receipt, err := reader.GetWorkRecoveryReceipt(r.Context(), authsvc.OwnerUserID(r.Context()), strings.TrimSpace(chi.URLParam(r, "execution_id")), strings.TrimSpace(r.URL.Query().Get("session_key")), strings.TrimSpace(r.URL.Query().Get("request_id")))
	if err != nil {
		h.writeExecutionError(w, err, "恢复请求核对失败")
		return
	}
	h.api.WriteSuccess(w, receipt)
}

func (h *Handlers) HandleRequestWorkRecovery(w http.ResponseWriter, r *http.Request) {
	requester, ok := h.executions.(workRecoveryRequester)
	if !ok {
		h.api.WriteFailure(w, http.StatusServiceUnavailable, "WorkGraph recovery is unavailable")
		return
	}
	var input protocol.RequestExecutionRecovery
	if !h.api.BindJSON(w, r, &input) {
		return
	}
	input.SessionKey = strings.TrimSpace(input.SessionKey)
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.WorkItemID = strings.TrimSpace(input.WorkItemID)
	input.AttemptID = strings.TrimSpace(input.AttemptID)
	receipt, err := requester.RequestWorkRecovery(r.Context(), authsvc.OwnerUserID(r.Context()), strings.TrimSpace(chi.URLParam(r, "execution_id")), input)
	if err != nil {
		var domainErr *orchestrationsvc.DomainError
		if errors.As(err, &domainErr) {
			h.api.WriteFailure(w, http.StatusConflict, "恢复目标已变化或不可恢复，请刷新工作图")
			return
		}
		h.api.WriteFailure(w, http.StatusServiceUnavailable, "恢复请求未确认，请刷新工作图后重试")
		return
	}
	h.api.WriteSuccess(w, receipt)
}
