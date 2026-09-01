// INPUT: owner 认证上下文、当前 Agent 路径身份、联系人或群目标与正文。
// OUTPUT: 好友直聊 Room 上下文，以及以当前 Agent 身份发送的消息回执。
// POS: Agent 通讯客户端的 owner 控制面 HTTP 边界。
package agent

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

// HandleOpenAgentContactChannel 打开或恢复好友已有的隐藏直聊 Room。
func (h *Handlers) HandleOpenAgentContactChannel(writer http.ResponseWriter, request *http.Request) {
	if h.communication == nil {
		h.api.WriteError(writer, request, http.StatusServiceUnavailable, handlershared.FailureSpec{
			Code:     "communication.channel_unavailable",
			Category: protocol.FailureCategoryUnavailable,
			Effect:   protocol.FailureEffectNotApplied,
			Detail:   "联络会话暂时无法打开",
		})
		return
	}
	item, err := h.communication.OpenContactChannel(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		chi.URLParam(request, "contact_agent_id"),
	)
	if err != nil {
		h.writeCommunicationFailure(
			writer, request, "communication.channel_open_failed", "联络会话没有打开", err,
		)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleSendAgentCommunicationMessage 以 owner 选中的普通 Agent 身份发送消息。
func (h *Handlers) HandleSendAgentCommunicationMessage(writer http.ResponseWriter, request *http.Request) {
	if h.communication == nil {
		h.api.WriteError(writer, request, http.StatusServiceUnavailable, handlershared.FailureSpec{
			Code:     "communication.send_unavailable",
			Category: protocol.FailureCategoryUnavailable,
			Effect:   protocol.FailureEffectNotApplied,
			Detail:   "消息暂时无法发送",
		})
		return
	}
	var payload communicationsvc.SendRequest
	if !h.api.BindJSONError(writer, request, &payload, handlershared.FailureSpec{
		Code:     "communication.request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "消息内容或接收方格式不正确",
	}) {
		return
	}
	item, err := h.communication.SendMessageAsAgent(
		request.Context(), chi.URLParam(request, "agent_id"), payload,
	)
	if err != nil {
		h.writeCommunicationFailure(
			writer, request, "communication.send_failed", "消息没有发送完成", err,
		)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) writeCommunicationFailure(
	writer http.ResponseWriter,
	request *http.Request,
	code string,
	detail string,
	err error,
) {
	status := http.StatusInternalServerError
	spec := handlershared.FailureSpec{
		Code:     code,
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   detail,
		Cause:    err,
	}
	var inputError *communicationsvc.InputError
	switch {
	case errors.Is(err, agentsvc.ErrAgentNotFound),
		errors.Is(err, agentsvc.ErrAgentContactNotFound),
		errors.Is(err, roomsvc.ErrRoomNotFound),
		errors.Is(err, roomsvc.ErrConversationNotFound),
		errors.Is(err, roomsvc.ErrRoomMemberNotFound):
		status = http.StatusNotFound
		spec.Code = "communication.target_not_found"
		spec.Category = protocol.FailureCategoryNotFound
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "联系人或会话不存在，本次操作没有执行"
	case errors.Is(err, channels.ErrExternalSessionGrantUnavailable):
		status = http.StatusForbidden
		spec.Code = "communication.external_session_unavailable"
		spec.Category = protocol.FailureCategoryAuthorization
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = "外部私聊已解绑或不属于当前 Agent，本次操作没有执行"
	case errors.As(err, &inputError):
		status = http.StatusBadRequest
		spec.Code = "communication.request_invalid"
		spec.Category = protocol.FailureCategoryValidation
		spec.Effect = protocol.FailureEffectNotApplied
		spec.Detail = inputError.Error()
	}
	h.api.WriteError(writer, request, status, spec)
}
