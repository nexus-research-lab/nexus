package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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
	h.registerAppServerGoalRPCSender(params.ThreadID, sender)
	item, err := h.goals.SetFromThreadGoalParams(goalsvc.WithActiveGoalContinuationSuppressed(ctx), params)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
	}
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
	h.registerAppServerGoalRPCSender(params.ThreadID, sender)
	item, err := h.goals.CurrentOptional(ctx, params.ThreadID)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
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
	h.registerAppServerGoalRPCSender(params.ThreadID, sender)
	cleared, err := h.goals.ClearFromThreadGoalParams(ctx, params)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
	}
	h.sendAppServerRPCResponse(ctx, sender, request.ID, goalappserver.ThreadGoalClearResponse{Cleared: cleared})
	if cleared {
		h.broadcastAppServerGoalNotification(ctx, sender, params.ThreadID, goalappserver.AppServerJSONRPCNotification{
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
	code := goalappserver.AppServerRPCInternalErrorCode
	message := strings.TrimSpace(err.Error())
	if errors.Is(err, goalsvc.ErrGoalDisabled) ||
		errors.Is(err, goalsvc.ErrGoalInvalidInput) ||
		errors.Is(err, goalsvc.ErrGoalInvalidState) ||
		errors.Is(err, goalsvc.ErrGoalNotFound) ||
		errors.Is(err, goalsvc.ErrGoalConflict) ||
		errors.Is(err, goalsvc.ErrGoalVersionStale) {
		code = goalappserver.AppServerRPCInvalidRequestCode
	}
	h.sendAppServerRPCError(ctx, sender, id, goalappserver.NewAppServerRPCError(code, message))
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
	threadID string,
	notification goalappserver.AppServerJSONRPCNotification,
) {
	if h.goalRPCSubs == nil {
		_ = current.SendJSON(ctx, notification)
		return
	}
	h.goalRPCSubs.Broadcast(ctx, threadID, current, notification)
}

func (h *Handler) broadcastThreadGoalSetNotification(
	ctx context.Context,
	current *handlershared.WebSocketSender,
	item protocol.Goal,
	goal goalappserver.ThreadGoal,
) {
	threadID := strings.TrimSpace(item.SessionKey)
	if protocol.NormalizeGoalStatus(item.Status) == protocol.GoalStatusComplete {
		h.broadcastAppServerGoalNotification(ctx, current, threadID, goalappserver.AppServerJSONRPCNotification{
			Method: "thread/goal/cleared",
			Params: goalappserver.ThreadGoalClearedNotification{
				ThreadID: threadID,
			},
		})
		return
	}
	h.broadcastAppServerGoalNotification(ctx, current, threadID, goalappserver.AppServerJSONRPCNotification{
		Method: "thread/goal/updated",
		Params: goalappserver.ThreadGoalUpdatedNotification{
			ThreadID: threadID,
			TurnID:   nil,
			Goal:     goal,
		},
	})
}

func (h *Handler) registerAppServerGoalRPCSender(threadID string, sender *handlershared.WebSocketSender) {
	if h.goalRPCSubs == nil {
		return
	}
	h.goalRPCSubs.Register(threadID, sender)
}
