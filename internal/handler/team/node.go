package team

import (
	"errors"
	"net/http"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	teamsvc "github.com/nexus-research-lab/nexus/internal/service/team"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

// NodeHandlers 是本机授权边界，与转发到远端的 Team 聊天 gateway 分离。
type NodeHandlers struct {
	api        *handlershared.API
	service    *teamsvc.NodeService
	cookieName string
}

func NewNodeHandlers(api *handlershared.API, service *teamsvc.NodeService, cookieName string) *NodeHandlers {
	return &NodeHandlers{api: api, service: service, cookieName: cookieName}
}

func (h *NodeHandlers) Handle(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet && !sameOrigin(request) {
		h.api.WriteFailure(writer, http.StatusForbidden, "请求来源无效")
		return
	}
	cookie, err := request.Cookie(h.cookieName)
	if err != nil || cookie.Value == "" {
		h.writeError(writer, teamsvc.ErrNodeLogin)
		return
	}
	switch request.Method {
	case http.MethodPost:
		var input teamsvc.NodeConnectInput
		if err = decodeStrictJSON(writer, request, &input); err == nil {
			err = h.service.Connect(request.Context(), cookie.Value, input)
		} else {
			err = teamsvc.ErrNodeInput
		}
	case http.MethodDelete:
		err = h.service.Revoke(request.Context(), cookie.Value)
	case http.MethodGet:
		var view teamsvc.NodeView
		if request.URL.Query().Has("room_id") {
			view, err = h.service.View(request.Context(), cookie.Value, teamsvc.NodeJobQuery{RoomID: request.URL.Query().Get("room_id"), MessageIDs: request.URL.Query()["message_id"], JobID: request.URL.Query().Get("job_id")})
		} else {
			view, err = h.service.View(request.Context(), cookie.Value)
		}
		if err == nil {
			h.api.WriteSuccess(writer, view)
			return
		}
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		h.writeError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]bool{"ok": true})
}

func (h *NodeHandlers) writeError(writer http.ResponseWriter, err error) {
	status, message := http.StatusServiceUnavailable, "节点授权未能确认，请重试原操作"
	switch {
	case errors.Is(err, teamsvc.ErrNodeLogin):
		status, message = http.StatusUnauthorized, "请先登录远程账户"
	case errors.Is(err, teamsvc.ErrNodeInput):
		status, message = http.StatusBadRequest, "只能授权本机已发布的 Agent"
	case errors.Is(err, teamstore.ErrNodeConflict):
		status, message = http.StatusConflict, "授权状态已变化，请刷新并对账原操作"
	case errors.Is(err, credentials.ErrKeyUnavailable):
		message = "宿主凭据密钥不可用，未向远端发送授权"
	}
	h.api.WriteFailure(writer, status, message)
}

// HandleRoom 校验本人入群 Agent，物化会话并自动登记执行；工具审批仍由本机 Room 持有。
func (h *NodeHandlers) HandleRoom(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if !sameOrigin(request) {
		h.api.WriteFailure(writer, http.StatusForbidden, "请求来源无效")
		return
	}
	cookie, err := request.Cookie(h.cookieName)
	if err != nil || cookie.Value == "" {
		h.writeError(writer, teamsvc.ErrNodeLogin)
		return
	}
	var input struct {
		RoomID  string   `json:"room_id"`
		RoomIDs []string `json:"room_ids"`
	}
	if err := decodeStrictJSON(writer, request, &input); err != nil {
		h.writeError(writer, teamsvc.ErrNodeInput)
		return
	}
	if input.RoomID != "" {
		input.RoomIDs = append(input.RoomIDs, input.RoomID)
	}
	bindings, err := h.service.PrepareRooms(request.Context(), cookie.Value, input.RoomIDs)
	if err != nil {
		h.writeError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, bindings)
}

// HandleRecover 核验原执行后结算，不暴露绕过核验的状态改写入口。
func (h *NodeHandlers) HandleRecover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !sameOrigin(r) {
		h.api.WriteFailure(w, http.StatusForbidden, "请求来源无效")
		return
	}
	cookie, err := r.Cookie(h.cookieName)
	if err != nil || cookie.Value == "" {
		h.writeError(w, teamsvc.ErrNodeLogin)
		return
	}
	var input struct {
		JobID string `json:"job_id"`
	}
	if decodeStrictJSON(w, r, &input) != nil || input.JobID == "" || len(input.JobID) > 128 {
		h.writeError(w, teamsvc.ErrNodeInput)
		return
	}
	if err = h.service.RecoverJob(r.Context(), cookie.Value, input.JobID); err != nil {
		h.writeError(w, err)
		return
	}
	h.api.WriteSuccess(w, map[string]bool{"ok": true})
}
