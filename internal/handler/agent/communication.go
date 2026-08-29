// INPUT: owner 认证上下文、当前 Agent 路径身份、联系人或群目标与正文。
// OUTPUT: 好友直聊 Room 上下文，以及以当前 Agent 身份发送的消息回执。
// POS: Agent 通讯客户端的 owner 控制面 HTTP 边界。
package agent

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

// HandleOpenAgentContactChannel 打开或恢复好友已有的隐藏直聊 Room。
func (h *Handlers) HandleOpenAgentContactChannel(writer http.ResponseWriter, request *http.Request) {
	if h.communication == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "平台通讯服务不可用")
		return
	}
	item, err := h.communication.OpenContactChannel(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		chi.URLParam(request, "contact_agent_id"),
	)
	if err != nil {
		h.writeCommunicationFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleSendAgentCommunicationMessage 以 owner 选中的普通 Agent 身份发送消息。
func (h *Handlers) HandleSendAgentCommunicationMessage(writer http.ResponseWriter, request *http.Request) {
	if h.communication == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "平台通讯服务不可用")
		return
	}
	var payload communicationsvc.SendRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.communication.SendMessageAsAgent(
		request.Context(), chi.URLParam(request, "agent_id"), payload,
	)
	if err != nil {
		h.writeCommunicationFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) writeCommunicationFailure(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agentsvc.ErrAgentNotFound),
		errors.Is(err, agentsvc.ErrAgentContactNotFound),
		errors.Is(err, roomsvc.ErrRoomNotFound),
		errors.Is(err, roomsvc.ErrConversationNotFound),
		errors.Is(err, roomsvc.ErrRoomMemberNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
	case errors.Is(err, channels.ErrExternalSessionGrantUnavailable):
		h.api.WriteFailure(writer, http.StatusForbidden, "外部私聊已解绑或不属于当前 Agent")
	case strings.Contains(err.Error(), "不能"),
		strings.Contains(err.Error(), "不可"),
		strings.Contains(err.Error(), "不属于"),
		strings.Contains(err.Error(), "不能为空"),
		strings.Contains(err.Error(), "只支持"):
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
	}
}
