package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (h *Handler) handleAppServerRPC(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	request, err := decodeAppServerRPCRequest(inbound)
	if err != nil {
		h.sendAppServerRPCError(ctx, sender, protocol.AppServerRequestID{}, protocol.NewAppServerRPCError(
			protocol.AppServerRPCInvalidRequestCode,
			"Invalid request: "+err.Error(),
		))
		return
	}
	if request.ID.IsZero() {
		return
	}
	if h.goals == nil {
		h.sendAppServerRPCError(ctx, sender, request.ID, protocol.NewAppServerRPCError(
			protocol.AppServerRPCInternalErrorCode,
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
		h.sendAppServerRPCError(ctx, sender, request.ID, protocol.NewAppServerRPCError(
			protocol.AppServerRPCMethodNotFoundCode,
			"method not found: "+strings.TrimSpace(request.Method),
		))
	}
}

func (h *Handler) handleThreadGoalSetRPC(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	request protocol.AppServerJSONRPCRequest,
) {
	var params protocol.ThreadGoalSetParams
	if !h.decodeAppServerRPCParams(ctx, sender, request, &params) {
		return
	}
	h.registerAppServerGoalRPCSender(params.ThreadID, sender)
	item, err := h.goals.SetFromThreadGoalParams(ctx, params)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
	}
	goal := protocol.ThreadGoalFromGoal(*item)
	h.sendAppServerRPCResponse(ctx, sender, request.ID, protocol.ThreadGoalSetResponse{Goal: goal})
	h.broadcastAppServerGoalNotification(ctx, sender, item.SessionKey, protocol.AppServerJSONRPCNotification{
		Method: "thread/goal/updated",
		Params: protocol.ThreadGoalUpdatedNotification{
			ThreadID: item.SessionKey,
			TurnID:   nil,
			Goal:     goal,
		},
	})
}

func (h *Handler) handleThreadGoalGetRPC(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	request protocol.AppServerJSONRPCRequest,
) {
	var params protocol.ThreadGoalGetParams
	if !h.decodeAppServerRPCParams(ctx, sender, request, &params) {
		return
	}
	h.registerAppServerGoalRPCSender(params.ThreadID, sender)
	item, err := h.goals.CurrentOptional(ctx, params.ThreadID)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
	}
	h.sendAppServerRPCResponse(ctx, sender, request.ID, protocol.ThreadGoalGetResponse{
		Goal: protocol.ThreadGoalPointerFromGoal(item),
	})
}

func (h *Handler) handleThreadGoalClearRPC(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	request protocol.AppServerJSONRPCRequest,
) {
	var params protocol.ThreadGoalClearParams
	if !h.decodeAppServerRPCParams(ctx, sender, request, &params) {
		return
	}
	h.registerAppServerGoalRPCSender(params.ThreadID, sender)
	cleared, err := h.goals.ClearFromThreadGoalParams(ctx, params)
	if err != nil {
		h.sendGoalRPCError(ctx, sender, request.ID, err)
		return
	}
	h.sendAppServerRPCResponse(ctx, sender, request.ID, protocol.ThreadGoalClearResponse{Cleared: cleared})
	if cleared {
		h.broadcastAppServerGoalNotification(ctx, sender, params.ThreadID, protocol.AppServerJSONRPCNotification{
			Method: "thread/goal/cleared",
			Params: protocol.ThreadGoalClearedNotification{
				ThreadID: params.ThreadID,
			},
		})
	}
}

func decodeAppServerRPCRequest(inbound map[string]any) (protocol.AppServerJSONRPCRequest, error) {
	payload, err := json.Marshal(inbound)
	if err != nil {
		return protocol.AppServerJSONRPCRequest{}, err
	}
	var request protocol.AppServerJSONRPCRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return protocol.AppServerJSONRPCRequest{}, err
	}
	return request, nil
}

func (h *Handler) decodeAppServerRPCParams(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	request protocol.AppServerJSONRPCRequest,
	target any,
) bool {
	params := request.Params
	if len(params) == 0 {
		params = []byte("{}")
	}
	if err := json.Unmarshal(params, target); err != nil {
		h.sendAppServerRPCError(ctx, sender, request.ID, protocol.NewAppServerRPCError(
			protocol.AppServerRPCInvalidRequestCode,
			"Invalid request: "+err.Error(),
		))
		return false
	}
	return true
}

func (h *Handler) sendGoalRPCError(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	id protocol.AppServerRequestID,
	err error,
) {
	code := protocol.AppServerRPCInternalErrorCode
	message := strings.TrimSpace(err.Error())
	if errors.Is(err, goalsvc.ErrGoalDisabled) ||
		errors.Is(err, goalsvc.ErrGoalInvalidInput) ||
		errors.Is(err, goalsvc.ErrGoalInvalidState) ||
		errors.Is(err, goalsvc.ErrGoalNotFound) ||
		errors.Is(err, goalsvc.ErrGoalConflict) ||
		errors.Is(err, goalsvc.ErrGoalVersionStale) {
		code = protocol.AppServerRPCInvalidRequestCode
	}
	h.sendAppServerRPCError(ctx, sender, id, protocol.NewAppServerRPCError(code, message))
}

func (h *Handler) sendAppServerRPCResponse(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	id protocol.AppServerRequestID,
	result any,
) {
	_ = sender.SendJSON(ctx, protocol.AppServerJSONRPCResponse{
		ID:     id,
		Result: result,
	})
}

func (h *Handler) sendAppServerRPCError(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	id protocol.AppServerRequestID,
	rpcError protocol.AppServerRPCErrorBody,
) {
	if id.IsZero() {
		return
	}
	_ = sender.SendJSON(ctx, protocol.AppServerJSONRPCError{
		ID:    id,
		Error: rpcError,
	})
}

func (h *Handler) broadcastAppServerGoalNotification(
	ctx context.Context,
	current *handlershared.WebSocketSender,
	threadID string,
	notification protocol.AppServerJSONRPCNotification,
) {
	if h.goalRPCSubs == nil {
		_ = current.SendJSON(ctx, notification)
		return
	}
	h.goalRPCSubs.Broadcast(ctx, threadID, current, notification)
}

func (h *Handler) registerAppServerGoalRPCSender(threadID string, sender *handlershared.WebSocketSender) {
	if h.goalRPCSubs == nil {
		return
	}
	h.goalRPCSubs.Register(threadID, sender)
}
