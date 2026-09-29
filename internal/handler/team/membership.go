package team

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func (h *Handlers) verifyOwnedAgents(writer http.ResponseWriter, request *http.Request, agentIDs []string) error {
	err := h.tokens.VerifyOwnedAgents(request.Context(), authsvc.PrincipalFromContext(request.Context()), agentIDs)
	if err == nil {
		return nil
	}
	if errors.Is(err, authsvc.ErrAgentOwnerInvalid) {
		h.api.WriteError(writer, request, http.StatusForbidden, handlershared.FailureSpec{
			Code: "team.agent_owner_required", Category: protocol.FailureCategoryAuthorization,
			Effect: protocol.FailureEffectNotApplied, Detail: "只能添加自己在当前组织发布的 Agent",
		})
		return err
	}
	h.api.WriteError(writer, request, http.StatusBadGateway, handlershared.FailureSpec{
		Code: "team.agent_check_failed", Category: protocol.FailureCategoryUnavailable,
		Effect: protocol.FailureEffectNotApplied, Detail: "暂时无法确认 Agent 归属", Cause: err,
	})
	return err
}

// HandleAddAgent 将当前真人拥有的在线 Agent 加入 Room。
func (h *Handlers) HandleAddAgent(writer http.ResponseWriter, request *http.Request) {
	var input relaycontract.AddRoomAgentInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	if !ok || h.verifyOwnedAgents(writer, request, []string{input.AgentID}) != nil {
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.AddAgent(request.Context(), teamAccess(request, token), roomID, key, input)
	h.writeMembershipResult(writer, request, result, err)
}

// HandleRemoveAgent 将 Agent 成员移出 Room。
func (h *Handlers) HandleRemoveAgent(writer http.ResponseWriter, request *http.Request) {
	var input relaycontract.RemoveRoomAgentInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	agentID := strings.TrimSpace(chi.URLParam(request, "agent_id"))
	if !ok || !validResourceID(agentID) {
		if ok {
			h.writeRequestError(writer, request, "team.agent_invalid", "Agent 参数无效", true)
		}
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.RemoveAgent(request.Context(), teamAccess(request, token), roomID, agentID, key, input)
	h.writeMembershipResult(writer, request, result, err)
}

// HandleUpdateAgent 暂停或恢复当前真人拥有的在线 Agent。
func (h *Handlers) HandleUpdateAgent(writer http.ResponseWriter, request *http.Request) {
	var input relaycontract.UpdateRoomAgentInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	agentID := strings.TrimSpace(chi.URLParam(request, "agent_id"))
	if !ok || !validResourceID(agentID) {
		if ok {
			h.writeRequestError(writer, request, "team.agent_invalid", "Agent 参数无效", true)
		}
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.UpdateAgent(request.Context(), teamAccess(request, token), roomID, agentID, key, input)
	h.writeMembershipResult(writer, request, result, err)
}

// HandleUpdateRoom 转发群设置和解散命令，字段与角色由 Relay 权威校验。
func (h *Handlers) HandleUpdateRoom(writer http.ResponseWriter, request *http.Request) {
	var input relaycontract.UpdateRoomInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	if !ok || input.ExpectedConfigurationVersion <= 0 {
		if ok {
			h.writeRequestError(writer, request, "team.coordinator_invalid", "主持 Agent 参数无效", true)
		}
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.UpdateRoom(request.Context(), teamAccess(request, token), roomID, key, input)
	if err != nil {
		h.writeRelayError(writer, request, err, true)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleGetRoom 返回当前 active 真人成员可见的 Room 管理快照。
func (h *Handlers) HandleGetRoom(writer http.ResponseWriter, request *http.Request) {
	h.noStore(writer)
	roomID := strings.TrimSpace(chi.URLParam(request, "room_id"))
	if !validResourceID(roomID) {
		h.writeRequestError(writer, request, "team.room_invalid", "群聊参数无效", false)
		return
	}
	token, ok := h.exchangeToken(writer, request, false)
	if !ok {
		return
	}
	result, err := h.team.GetRoom(request.Context(), teamAccess(request, token), roomID)
	if err != nil {
		h.writeRelayError(writer, request, err, false)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleListInvitations 返回当前真人尚未处理的在线 Room 邀请。
func (h *Handlers) HandleListInvitations(writer http.ResponseWriter, request *http.Request) {
	h.noStore(writer)
	token, ok := h.exchangeToken(writer, request, false)
	if !ok {
		return
	}
	result, err := h.team.ListInvitations(request.Context(), teamAccess(request, token))
	if err != nil {
		h.writeRelayError(writer, request, err, false)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleInviteMember 邀请一名当前 Organization 真人加入 Room。
func (h *Handlers) HandleInviteMember(writer http.ResponseWriter, request *http.Request) {
	var input relaycontract.InviteRoomMemberInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	if !ok {
		return
	}
	principal := authsvc.PrincipalFromContext(request.Context())
	if err := h.tokens.VerifyOrganizationMembers(request.Context(), principal, []string{input.UserID}); err != nil {
		if errors.Is(err, authsvc.ErrOrganizationMemberInvalid) {
			h.api.WriteError(writer, request, http.StatusForbidden, handlershared.FailureSpec{
				Code: "team.organization_member_required", Category: protocol.FailureCategoryAuthorization,
				Effect: protocol.FailureEffectNotApplied, Detail: "只能邀请当前组织的成员",
			})
			return
		}
		h.api.WriteError(writer, request, http.StatusBadGateway, handlershared.FailureSpec{
			Code: "team.organization_check_failed", Category: protocol.FailureCategoryUnavailable,
			Effect: protocol.FailureEffectNotApplied, Detail: "暂时无法确认成员的组织归属", Cause: err,
		})
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.InviteUser(request.Context(), teamAccess(request, token), roomID, key, input)
	h.writeMembershipResult(writer, request, result, err)
}

// HandleAcceptInvitation 接受当前真人自己的 Room 邀请。
func (h *Handlers) HandleAcceptInvitation(writer http.ResponseWriter, request *http.Request) {
	h.resolveInvitation(writer, request, true)
}

// HandleRejectInvitation 拒绝当前真人自己的 Room 邀请。
func (h *Handlers) HandleRejectInvitation(writer http.ResponseWriter, request *http.Request) {
	h.resolveInvitation(writer, request, false)
}

func (h *Handlers) resolveInvitation(writer http.ResponseWriter, request *http.Request, accept bool) {
	var input relaycontract.ResolveRoomInvitationInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	if !ok {
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	access := teamAccess(request, token)
	var result relaycontract.RoomMembershipMutation
	var err error
	if accept {
		result, err = h.team.AcceptInvitation(request.Context(), access, roomID, key, input)
	} else {
		result, err = h.team.RejectInvitation(request.Context(), access, roomID, key, input)
	}
	h.writeMembershipResult(writer, request, result, err)
}

// HandleRevokeInvitation 撤销目标真人尚未接受的 Room 邀请。
func (h *Handlers) HandleRevokeInvitation(writer http.ResponseWriter, request *http.Request) {
	var input relaycontract.ResolveRoomInvitationInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	userID := strings.TrimSpace(chi.URLParam(request, "user_id"))
	if !ok || !validResourceID(userID) {
		if ok {
			h.writeRequestError(writer, request, "team.member_invalid", "成员参数无效", true)
		}
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.RevokeInvitation(request.Context(), teamAccess(request, token), roomID, userID, key, input)
	h.writeMembershipResult(writer, request, result, err)
}

// HandleUpdateMember 修改一名 active 真人成员角色或将其移除。
func (h *Handlers) HandleUpdateMember(writer http.ResponseWriter, request *http.Request) {
	var input relaycontract.UpdateRoomMemberInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	userID := strings.TrimSpace(chi.URLParam(request, "user_id"))
	if !ok || !validResourceID(userID) {
		if ok {
			h.writeRequestError(writer, request, "team.member_invalid", "成员参数无效", true)
		}
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.UpdateMember(request.Context(), teamAccess(request, token), roomID, userID, key, input)
	h.writeMembershipResult(writer, request, result, err)
}

// HandleTransferOwnership 将真人群主移交给另一名 active 真人成员。
func (h *Handlers) HandleTransferOwnership(writer http.ResponseWriter, request *http.Request) {
	var input relaycontract.TransferRoomOwnershipInput
	roomID, key, ok := h.membershipMutationInput(writer, request, &input)
	if !ok {
		return
	}
	token, ok := h.exchangeToken(writer, request, true)
	if !ok {
		return
	}
	result, err := h.team.TransferOwnership(request.Context(), teamAccess(request, token), roomID, key, input)
	h.writeMembershipResult(writer, request, result, err)
}

func (h *Handlers) membershipMutationInput(writer http.ResponseWriter, request *http.Request, input any) (string, string, bool) {
	h.noStore(writer)
	if !h.requireMutationOrigin(writer, request) {
		return "", "", false
	}
	roomID := strings.TrimSpace(chi.URLParam(request, "room_id"))
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if !validResourceID(roomID) || !validIdempotencyKey(key) {
		h.writeRequestError(writer, request, "team.member_invalid", "成员请求参数无效", true)
		return "", "", false
	}
	if err := decodeStrictJSON(writer, request, input); err != nil {
		h.writeRequestError(writer, request, "team.member_invalid", "成员请求正文无效", true)
		return "", "", false
	}
	return roomID, key, true
}

func (h *Handlers) writeMembershipResult(writer http.ResponseWriter, request *http.Request, result relaycontract.RoomMembershipMutation, err error) {
	if err != nil {
		h.writeRelayError(writer, request, err, true)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleRoomDeliveryStatuses 限定消息批量大小；成员和组织权限由 Relay 重验。
func (h *Handlers) HandleRoomDeliveryStatuses(w http.ResponseWriter, r *http.Request) {
	h.noStore(w)
	roomID, ids := chi.URLParam(r, "room_id"), r.URL.Query()["message_id"]
	valid := validResourceID(roomID) && len(ids) > 0 && len(ids) <= 100
	for _, id := range ids {
		valid = valid && validResourceID(id)
	}
	if !valid {
		h.writeRequestError(w, r, "team.delivery_query_invalid", "投递查询需提供 1 至 100 条有效消息", false)
		return
	}
	token, ok := h.exchangeToken(w, r, false)
	if !ok {
		return
	}
	result, err := h.team.RoomDeliveryStatuses(r.Context(), teamAccess(r, token), roomID, ids)
	if err != nil {
		h.writeRelayError(w, r, err, false)
		return
	}
	h.api.WriteSuccess(w, result)
}

func (h *Handlers) HandleRoomMembers(w http.ResponseWriter, r *http.Request) {
	h.noStore(w)
	roomID := chi.URLParam(r, "room_id")
	cursor, epoch := r.URL.Query().Get("after"), r.URL.Query().Get("stream_epoch")
	version, err := strconv.ParseInt(r.URL.Query().Get("membership_version"), 10, 64)
	if !validResourceID(roomID) || cursor == "" || len(cursor) > 256 || epoch == "" || len(epoch) > 128 || err != nil || version <= 0 {
		h.writeRequestError(w, r, "team.member_query_invalid", "成员分页参数无效", false)
		return
	}
	token, ok := h.exchangeToken(w, r, false)
	if !ok {
		return
	}
	result, err := h.team.RoomMembers(r.Context(), teamAccess(r, token), roomID, cursor, epoch, version)
	if err != nil {
		h.writeRelayError(w, r, err, false)
		return
	}
	h.api.WriteSuccess(w, result)
}
