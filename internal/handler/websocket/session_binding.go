// INPUT: authenticated WebSocket sender 与 structured bind/unbind_session 命令。
// OUTPUT: permission、command catalog、context usage 及 owner/session Execution invalidation 租约。
// POS: 会话绑定协议入口；只登记 transport scope，不读取或修改 WorkGraph。
package websocket

import (
	"context"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func (h *Handler) handleBindSession(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, parsed, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if parsed.Kind == protocol.SessionKeyKindUnknown {
		return
	}
	catalogEvent, err := h.commandCatalogEvent(ctx, sessionKey, parsed, inbound)
	if err != nil {
		h.sendGatewayError(ctx, sender, sessionKey, "command_catalog_error", err, map[string]any{
			"type": "bind_session",
		})
		return
	}
	h.permission.BindSession(sessionKey, sender)
	if h.executionInvalidations != nil {
		h.executionInvalidations.Bind(authctx.OwnerUserID(ctx), sessionKey, sender)
	}
	if h.channels != nil {
		_ = h.channels.RememberWebSocketRoute(ctx, sessionKey)
	}
	h.broadcastSessionStatus(ctx, sessionKey)
	if err = sender.SendEvent(ctx, catalogEvent); err != nil {
		h.sendGatewayError(ctx, sender, sessionKey, "command_catalog_error", err, map[string]any{
			"type": "bind_session",
		})
		return
	}
	if err = h.replayContextUsageSnapshots(ctx, sender, sessionKey); err != nil {
		h.sendGatewayError(ctx, sender, sessionKey, "context_usage_replay_error", err, map[string]any{
			"type": "bind_session",
		})
		return
	}
	if err = h.replayAutomationPermissionRequests(ctx, sender, sessionKey); err != nil {
		h.sendGatewayError(ctx, sender, sessionKey, "automation_permission_replay_error", err, map[string]any{
			"type": "bind_session",
		})
		return
	}
	// execution_invalidated is ephemeral. Rebinding therefore emits an empty
	// identity fence so a graph created or terminalized while disconnected is
	// re-read without falling back to conversation activity heuristics.
	if err = sendExecutionInvalidationFence(ctx, sender, sessionKey); err != nil {
		return
	}
	if parsed.Kind == protocol.SessionKeyKindAgent && h.dm != nil {
		if err := h.dm.SendInputQueueSnapshot(ctx, sessionKey, handlershared.StringValue(inbound["agent_id"])); err != nil {
			h.sendGatewayError(ctx, sender, sessionKey, "input_queue_error", err, map[string]any{"type": "bind_session"})
		}
	}
}

type sessionEventSender interface {
	SendEvent(context.Context, protocol.EventMessage) error
}

func (h *Handler) replayAutomationPermissionRequests(
	ctx context.Context,
	sender sessionEventSender,
	sessionKey string,
) error {
	if h == nil || h.automationPermissions == nil || sender == nil {
		return nil
	}
	events, err := h.automationPermissions.ListSessionPermissionEvents(ctx, sessionKey)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err = sender.SendEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// replayContextUsageSnapshots 在重新绑定历史 Session 时恢复最后一次权威快照。
func (h *Handler) replayContextUsageSnapshots(
	ctx context.Context,
	sender sessionEventSender,
	sessionKey string,
) error {
	if h == nil || h.runtime == nil || sender == nil {
		return nil
	}
	snapshots := h.runtime.ContextUsageSnapshots(sessionKey)
	if h.contextUsage != nil {
		persisted, err := h.contextUsage.GetPersistedContextUsageSnapshots(
			ctx,
			sessionKey,
		)
		if err != nil {
			return err
		}
		for agentID, usage := range persisted {
			if hasContextUsageSnapshot(snapshots, agentID) {
				continue
			}
			h.runtime.RecordContextUsage(sessionKey, agentID, usage)
		}
		snapshots = h.runtime.ContextUsageSnapshots(sessionKey)
	}
	for _, snapshot := range snapshots {
		event := protocol.NewContextUsageEvent(
			sessionKey,
			snapshot.AgentID,
			snapshot.Usage,
		)
		if err := sender.SendEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func hasContextUsageSnapshot(
	snapshots []runtimectx.ContextUsageSnapshot,
	agentID string,
) bool {
	for _, snapshot := range snapshots {
		if snapshot.AgentID == agentID {
			return true
		}
	}
	return false
}

func (h *Handler) handleUnbindSession(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, _, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	h.permission.UnbindSession(sessionKey, sender)
	if h.executionInvalidations != nil {
		h.executionInvalidations.Unbind(authctx.OwnerUserID(ctx), sessionKey, sender)
	}
	h.broadcastSessionStatus(ctx, sessionKey)
}

func (h *Handler) ensureSessionBinding(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	sessionKey string,
) {
	if h.permission.IsBound(sessionKey, sender) {
		if h.executionInvalidations != nil {
			h.executionInvalidations.Bind(authctx.OwnerUserID(ctx), sessionKey, sender)
		}
		return
	}
	h.permission.BindSession(sessionKey, sender)
	if h.executionInvalidations != nil {
		h.executionInvalidations.Bind(authctx.OwnerUserID(ctx), sessionKey, sender)
	}
	h.broadcastSessionStatus(ctx, sessionKey)
}

func (h *Handler) validateSessionKey(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) (string, protocol.SessionKey, bool) {
	sessionKey := handlershared.StringValue(inbound["session_key"])
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		errorType := "invalid_session_key"
		if err.Error() == "session_key is required" {
			errorType = "validation_error"
		}
		h.sendGatewayError(ctx, sender, sessionKey, errorType, err, map[string]any{"type": handlershared.StringValue(inbound["type"])})
		return "", protocol.SessionKey{}, false
	}
	return normalized, protocol.ParseSessionKey(normalized), true
}

func (h *Handler) broadcastSessionStatus(ctx context.Context, sessionKeys ...string) {
	for _, sessionKey := range sessionKeys {
		if strings.TrimSpace(sessionKey) == "" {
			continue
		}
		_ = h.permission.BroadcastSessionStatus(ctx, sessionKey, h.runtime.GetRunningRoundIDs(sessionKey))
	}
}
