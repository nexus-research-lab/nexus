// INPUT: owner 认证上下文、路径 Agent id 与联系人 JSON。
// OUTPUT: 双向联系人列表、创建/改名和删除 HTTP 响应。
// POS: 用户管理 Agent 通讯录的 HTTP 边界。
package agent

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

// HandleListAgentContacts 返回一个 Agent 的通讯录。
func (h *Handlers) HandleListAgentContacts(writer http.ResponseWriter, request *http.Request) {
	items, err := h.agents.ListAgentContacts(request.Context(), chi.URLParam(request, "agent_id"))
	if err != nil {
		h.writeContactFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleAddAgentContact 创建双向联系人或更新发起方别名。
func (h *Handlers) HandleAddAgentContact(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.CreateAgentContactRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.agents.AddAgentContact(
		request.Context(), chi.URLParam(request, "agent_id"), payload,
	)
	if err != nil {
		h.writeContactFailure(writer, err)
		return
	}
	h.broadcastDirectoryChanged(request.Context(), "agent_contact_changed", map[string]any{
		"agent_id":         chi.URLParam(request, "agent_id"),
		"contact_agent_id": item.ContactAgentID,
	})
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteAgentContact 删除双向联系人关系。
func (h *Handlers) HandleDeleteAgentContact(writer http.ResponseWriter, request *http.Request) {
	ownerAgentID := chi.URLParam(request, "agent_id")
	contactAgentID := chi.URLParam(request, "contact_agent_id")
	if err := h.agents.DeleteAgentContact(request.Context(), ownerAgentID, contactAgentID); err != nil {
		h.writeContactFailure(writer, err)
		return
	}
	h.broadcastDirectoryChanged(request.Context(), "agent_contact_changed", map[string]any{
		"agent_id": ownerAgentID, "contact_agent_id": contactAgentID,
	})
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

func (h *Handlers) writeContactFailure(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agentsvc.ErrAgentNotFound), errors.Is(err, agentsvc.ErrAgentContactNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
	case errors.Is(err, agentsvc.ErrAgentContactUnsupported),
		strings.Contains(err.Error(), "不能为空"),
		strings.Contains(err.Error(), "不能"),
		strings.Contains(err.Error(), "超过"):
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
	}
}
