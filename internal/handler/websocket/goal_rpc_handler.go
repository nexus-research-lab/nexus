// INPUT: 已认证 WebSocket owner、app-server JSON-RPC request 与 Goal service。
// OUTPUT: owner-scoped thread/goal set/get/clear response、带稳定 conflict reason_code 的错误和成功后订阅登记。
// POS: WebSocket app-server Goal transport；授权成功前不得注册订阅或产生 Goal 副作用。
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func (h *Handler) handleAppServerRPC(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	request, err := decodeAppServerRPCRequest(inbound)
	if err != nil {
		h.sendAppServerRPCError(ctx, sender, goalappserver.AppServerRequestID{}, goalappserver.NewAppServerRPCError(
			goalappserver.AppServerRPCInvalidRequestCode,
			"Invalid request: "+err.Error(),
		))
		return
	}
	if request.ID.IsZero() {
		return
	}
	if h.goals == nil {
		h.sendAppServerRPCError(ctx, sender, request.ID, goalappserver.NewAppServerRPCError(
			goalappserver.AppServerRPCInternalErrorCode,
			"goals service is unavailable",
		))
		return
	}

	switch strings.TrimSpace(request.Method) {
	case "thread/goal/set":
		h.handleThreadGoalSetRPC(ctx, sender, request)
	case "thread/goal/get":
		h.handleThreadGoalGetRPC(ctx, sender, request)
	case "thread/goal/clear":
		h.handleThreadGoalClearRPC(ctx, sender, request)
	default:
		h.sendAppServerRPCError(ctx, sender, request.ID, goalappserver.NewAppServerRPCError(
			goalappserver.AppServerRPCMethodNotFoundCode,
			"method not found: "+strings.TrimSpace(request.Method),
		))
	}
}

func (h *Handler) handleThreadGoalSetRPC(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	request goalappserver.AppServerJSONRPCRequest,
) {
	var params goalappserver.ThreadGoalSetParams
	if !h.decodeAppServerRPCParams(ctx, sender, request, &params) {
		return
	}
	params.OwnerUserID = authsvc.OwnerUserID(ctx)
	item, err := h.goals.SetFromThreadGoalParams(goalsvc.WithActiveGoalContinuationSuppressed(ctx), params)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
	}
	h.registerAppServerGoalRPCSender(
		protocol.GoalMetadataString(item.Metadata, protocol.GoalMetadataOwnerUserID),
		params.ThreadID,
		sender,
	)
	goal := goalappserver.ThreadGoalFromGoal(*item)
	h.sendAppServerRPCResponse(ctx, sender, request.ID, goalappserver.ThreadGoalSetResponse{Goal: goal})
	h.broadcastThreadGoalSetNotification(ctx, sender, *item, goal)
	h.goals.DispatchActiveGoalContinuation(ctx, *item)
}

func (h *Handler) handleThreadGoalGetRPC(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	request goalappserver.AppServerJSONRPCRequest,
) {
	var params goalappserver.ThreadGoalGetParams
	if !h.decodeAppServerRPCParams(ctx, sender, request, &params) {
		return
	}
	ownerUserID := authsvc.OwnerUserID(ctx)
	item, err := h.goals.CurrentOptionalForOwner(ctx, params.ThreadID, ownerUserID)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
	}
	if item != nil {
		h.registerAppServerGoalRPCSender(ownerUserID, params.ThreadID, sender)
	}
	h.sendAppServerRPCResponse(ctx, sender, request.ID, goalappserver.ThreadGoalGetResponse{
		Goal: goalappserver.ThreadGoalPointerFromGoal(item),
	})
}

func (h *Handler) handleThreadGoalClearRPC(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	request goalappserver.AppServerJSONRPCRequest,
) {
	var params goalappserver.ThreadGoalClearParams
	if !h.decodeAppServerRPCParams(ctx, sender, request, &params) {
		return
	}
	params.OwnerUserID = authsvc.OwnerUserID(ctx)
	cleared, err := h.goals.ClearFromThreadGoalParams(ctx, params)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
	}
	h.sendAppServerRPCResponse(ctx, sender, request.ID, goalappserver.ThreadGoalClearResponse{Cleared: cleared})
	if cleared {
		h.broadcastAppServerGoalNotification(ctx, sender, params.OwnerUserID, params.ThreadID, goalappserver.AppServerJSONRPCNotification{
			Method: "thread/goal/cleared",
			Params: goalappserver.ThreadGoalClearedNotification{
				ThreadID: params.ThreadID,
			},
		})
	}
}

func decodeAppServerRPCRequest(inbound map[string]any) (goalappserver.AppServerJSONRPCRequest, error) {
	payload, err := json.Marshal(inbound)
	if err != nil {
		return goalappserver.AppServerJSONRPCRequest{}, err
	}
	var request goalappserver.AppServerJSONRPCRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return goalappserver.AppServerJSONRPCRequest{}, err
	}
	return request, nil
}

func (h *Handler) decodeAppServerRPCParams(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	request goalappserver.AppServerJSONRPCRequest,
	target any,
) bool {
	params := request.Params
	if len(params) == 0 {
		params = []byte("{}")
	}
	if err := json.Unmarshal(params, target); err != nil {
		h.sendAppServerRPCError(ctx, sender, request.ID, goalappserver.NewAppServerRPCError(
			goalappserver.AppServerRPCInvalidRequestCode,
			"Invalid request: "+err.Error(),
		))
		return false
	}
	return true
}

func (h *Handler) sendGoalRPCError(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	id goalappserver.AppServerRequestID,
	err error,
) {
	h.sendAppServerRPCError(ctx, sender, id, appServerGoalRPCError(err))
}

func appServerGoalRPCError(err error) goalappserver.AppServerRPCErrorBody {
	message := strings.TrimSpace(err.Error())
	switch {
	case errors.Is(err, goalsvc.ErrGoalRevisionStale):
		return goalappserver.NewAppServerRPCConflictError(
			message,
			goalappserver.AppServerRPCReasonRevisionStale,
		)
	case errors.Is(err, goalsvc.ErrGoalExecutionBindingConflict):
		return goalappserver.NewAppServerRPCConflictError(
			message,
			goalappserver.AppServerRPCReasonExecutionBindingConflict,
		)
	case errors.Is(err, goalsvc.ErrGoalVersionStale):
		return goalappserver.NewAppServerRPCConflictError(
			message,
			goalappserver.AppServerRPCReasonVersionStale,
		)
	case errors.Is(err, goalsvc.ErrGoalConflict):
		return goalappserver.NewAppServerRPCConflictError(
			message,
			goalappserver.AppServerRPCReasonConflict,
		)
	}
	code := goalappserver.AppServerRPCInternalErrorCode
	if errors.Is(err, goalsvc.ErrGoalDisabled) ||
		errors.Is(err, goalsvc.ErrGoalInvalidInput) ||
		errors.Is(err, goalsvc.ErrGoalInvalidState) ||
		errors.Is(err, goalsvc.ErrGoalForbidden) ||
		errors.Is(err, goalsvc.ErrGoalNotFound) {
		code = goalappserver.AppServerRPCInvalidRequestCode
	}
	return goalappserver.NewAppServerRPCError(code, message)
}

func (h *Handler) sendAppServerRPCResponse(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	id goalappserver.AppServerRequestID,
	result any,
) {
	_ = sender.SendJSON(ctx, goalappserver.AppServerJSONRPCResponse{
		ID:     id,
		Result: result,
	})
}

func (h *Handler) sendAppServerRPCError(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	id goalappserver.AppServerRequestID,
	rpcError goalappserver.AppServerRPCErrorBody,
) {
	if id.IsZero() {
		return
	}
	_ = sender.SendJSON(ctx, goalappserver.AppServerJSONRPCError{
		ID:    id,
		Error: rpcError,
	})
}

func (h *Handler) broadcastAppServerGoalNotification(
	ctx context.Context,
	current *handlershared.WebSocketSender,
	ownerUserID string,
	threadID string,
	notification goalappserver.AppServerJSONRPCNotification,
) {
	if h.goalRPCSubs == nil {
		_ = current.SendJSON(ctx, notification)
		return
	}
	h.goalRPCSubs.Broadcast(ctx, ownerUserID, threadID, current, notification)
}

func (h *Handler) broadcastThreadGoalSetNotification(
	ctx context.Context,
	current *handlershared.WebSocketSender,
	item protocol.Goal,
	goal goalappserver.ThreadGoal,
) {
	threadID := strings.TrimSpace(item.SessionKey)
	ownerUserID := protocol.GoalMetadataString(item.Metadata, protocol.GoalMetadataOwnerUserID)
	if protocol.NormalizeGoalStatus(item.Status) == protocol.GoalStatusComplete {
		h.broadcastAppServerGoalNotification(ctx, current, ownerUserID, threadID, goalappserver.AppServerJSONRPCNotification{
			Method: "thread/goal/cleared",
			Params: goalappserver.ThreadGoalClearedNotification{
				ThreadID: threadID,
			},
		})
		return
	}
	h.broadcastAppServerGoalNotification(ctx, current, ownerUserID, threadID, goalappserver.AppServerJSONRPCNotification{
		Method: "thread/goal/updated",
		Params: goalappserver.ThreadGoalUpdatedNotification{
			ThreadID: threadID,
			TurnID:   nil,
			Goal:     goal,
		},
	})
}

func (h *Handler) registerAppServerGoalRPCSender(
	ownerUserID string,
	threadID string,
	sender *handlershared.WebSocketSender,
) {
	if h.goalRPCSubs == nil {
		return
	}
	h.goalRPCSubs.Register(ownerUserID, threadID, sender)
}
