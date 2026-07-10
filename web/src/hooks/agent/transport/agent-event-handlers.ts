import type {
  AgentRoundStatusEventPayload,
  ChatAckData,
  EventMessage,
  RoundStatusEventPayload,
  SessionStatusEventPayload,
} from "@/types";
import type {
  InputQueueEventPayload,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { WorkspaceEventPayload } from "@/types/app/workspace-live";
import type { AssistantMessageStatus } from "@/types/conversation/message";

import {
  buildRoomSubscriptionMessage,
  buildSessionBindMessage,
} from "../actions/conversation-command-builders";
import type {
  AgentEventContext,
  AgentEventHandler,
  AgentEventHandlerMap,
} from "./agent-event-context";

function getEventRoundId(event: EventMessage): string | null {
  const dataRoundId = typeof event.data?.round_id === "string"
    ? event.data.round_id.trim()
    : "";
  const envelopeRoundId = typeof event.round_id === "string"
    ? event.round_id.trim()
    : "";
  return dataRoundId || envelopeRoundId || null;
}

function handleRoomResync(
  event: EventMessage,
  context: AgentEventContext,
): void {
  const { roomId, conversationId } = context.scope;
  if (event.room_id !== roomId) {
    return;
  }

  const latestRoomSeq = event.data?.latest_room_seq;
  if (typeof latestRoomSeq === "number") {
    context.transport.roomSeqCursorRef.current = Math.max(
      context.transport.roomSeqCursorRef.current,
      latestRoomSeq,
    );
  }
  context.callbacks.onRoomEvent(event.event_type, event.data ?? {});

  void context.transport.reloadCurrentSession().finally(() => {
    if (!roomId || context.transport.wsStateRef.current !== "connected") {
      return;
    }
    context.transport.wsSendRef.current(buildRoomSubscriptionMessage({
      type: "subscribe_room",
      room_id: roomId,
      conversation_id: conversationId,
      last_seen_room_seq: context.transport.roomSeqCursorRef.current,
    }));
  });
}

function handleSessionResync(
  event: EventMessage,
  context: AgentEventContext,
): void {
  const incomingSessionKey = event.session_key;
  if (
    !incomingSessionKey ||
    !context.scope.isCurrentSessionEvent(incomingSessionKey)
  ) {
    return;
  }

  const latestSessionSeq = event.data?.latest_session_seq;
  if (typeof latestSessionSeq === "number") {
    context.transport.sessionSeqCursorRef.current = Math.max(
      context.transport.sessionSeqCursorRef.current,
      latestSessionSeq,
    );
  }
  const reason = typeof event.data?.reason === "string"
    ? event.data.reason
    : "";
  const targetRoundId = typeof event.data?.target_round_id === "string"
    ? event.data.target_round_id.trim()
    : "";
  if (reason === "history_rewrite" && targetRoundId) {
    context.runtime.removeRewrittenRound(targetRoundId);
  }
  context.callbacks.onRoomEvent(event.event_type, event.data ?? {});

  void context.transport.reloadCurrentSession().finally(() => {
    const { agentId, conversationId, roomId, sessionKey } = context.scope;
    if (!sessionKey || context.transport.wsStateRef.current !== "connected") {
      return;
    }
    context.transport.wsSendRef.current(buildSessionBindMessage({
      session_key: sessionKey,
      last_seen_session_seq:
        context.transport.sessionSeqCursorRef.current,
      agent_id: agentId,
      room_id: roomId,
      conversation_id: conversationId,
    }));
  });
}

const handleAgentRuntimeEvent: AgentEventHandler = (event, context) => {
  const payload = event.data as
    | { agent_id?: string; running_task_count?: number; status?: string }
    | undefined;
  if (
    payload?.agent_id === context.scope.agentId &&
    payload.running_task_count === 0 &&
    payload.status !== "running"
  ) {
    context.callbacks.settleAgentWorkspaceWrites(payload.agent_id);
  }
};

const handleErrorEvent: AgentEventHandler = (event, context) => {
  const incomingSessionKey = event.session_key || null;
  if (
    incomingSessionKey &&
    !context.scope.isCurrentSessionEvent(incomingSessionKey)
  ) {
    return;
  }

  const roundId = getEventRoundId(event);
  if (roundId) {
    context.runtime.applyRoundStatus(roundId, "error");
  }
  if (event.message_id) {
    context.runtime.updateMessageStatus(event.message_id, "error", roundId);
  }
  const message = event.data?.message || "Unknown error";
  const clientRequestId = typeof event.data?.client_request_id === "string"
    ? event.data.client_request_id
    : "";
  if (clientRequestId) {
    context.runtime.rejectChatAck(clientRequestId, message);
  }
  context.state.setError(message);
};

const handlePermissionRequest: AgentEventHandler = (event, context) => {
  const incomingSessionKey = event.session_key || null;
  if (!context.scope.isCurrentSessionEvent(incomingSessionKey)) {
    return;
  }

  const data = event.data || {};
  context.state.setPendingPermissions((currentPermissions) => {
    const nextPermission = {
      request_id: data.request_id,
      tool_name: data.tool_name,
      tool_input: data.tool_input || {},
      session_key: incomingSessionKey,
      agent_id: data.agent_id ?? event.agent_id ?? null,
      message_id: data.message_id ?? event.message_id ?? null,
      round_id: data.round_id ?? event.round_id ?? null,
      agent_round_id:
        data.agent_round_id ?? event.agent_round_id ?? null,
      tool_use_id: data.tool_use_id ?? null,
      interaction_mode: data.interaction_mode ?? (
        data.tool_name === "AskUserQuestion" ? "question" : "permission"
      ),
      risk_level: data.risk_level,
      risk_label: data.risk_label,
      summary: data.summary,
      suggestions: data.suggestions || [],
      expires_at: data.expires_at,
    };
    return [
      ...currentPermissions.filter(
        (permission) => permission.request_id !== data.request_id,
      ),
      nextPermission,
    ];
  });
};

const handlePermissionResolved: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  const requestId = typeof event.data?.request_id === "string"
    ? event.data.request_id
    : "";
  if (!requestId) {
    return;
  }
  context.state.setPendingPermissions((currentPermissions) => {
    const nextPermissions = currentPermissions.filter(
      (permission) => permission.request_id !== requestId,
    );
    return nextPermissions.length === currentPermissions.length
      ? currentPermissions
      : nextPermissions;
  });
};

const handleWorkspaceEvent: AgentEventHandler = (event, context) => {
  const payload = event.data as WorkspaceEventPayload;
  if (payload?.agent_id && payload.path) {
    context.callbacks.applyWorkspaceEvent(payload);
  }
};

const handleRoomEvent: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentRoomEvent(event.room_id)) {
    return;
  }
  context.callbacks.onRoomEvent(
    event.event_type,
    (event.data ?? {}) as RoomEventPayload,
  );
};

const handleSessionStatus: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  context.runtime.syncSessionStatus(
    (event.data ?? {}) as SessionStatusEventPayload,
  );
};

const handleInputQueue: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  const payload = (event.data ?? {}) as InputQueueEventPayload;
  context.state.setInputQueueItems(
    Array.isArray(payload.items) ? payload.items : [],
  );
};

const handleGoalEvent: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  context.callbacks.onRoomEvent(
    event.event_type,
    (event.data ?? {}) as RoomEventPayload,
  );
};

const handleRoundStatus: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  const payload = (event.data ?? {}) as RoundStatusEventPayload;
  if (payload.round_id && payload.status) {
    context.runtime.applyRoundStatus(payload.round_id, payload.status);
  }
};

const handleAgentRoundStatus: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  const payload = (event.data ?? {}) as AgentRoundStatusEventPayload;
  if (payload.agent_round_id && payload.status) {
    context.runtime.applyAgentRoundStatus(payload);
  }
};

const handleChatAck: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  const ack = event.data as ChatAckData;
  if (ack?.round_id) {
    context.runtime.trackChatAck(ack);
  }
};

function createMessageStatusHandler(
  status: AssistantMessageStatus,
): AgentEventHandler {
  return (event, context) => {
    if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
      return;
    }
    const messageId = event.message_id || event.data?.msg_id;
    if (typeof messageId === "string" && messageId) {
      context.runtime.updateMessageStatus(
        messageId,
        status,
        event.data?.round_id,
      );
    }
  };
}

export const AGENT_STATE_EVENT_HANDLERS: AgentEventHandlerMap = {
  agent_round_status: handleAgentRoundStatus,
  agent_runtime_event: handleAgentRuntimeEvent,
  chat_ack: handleChatAck,
  error: handleErrorEvent,
  goal_cleared: handleGoalEvent,
  goal_continuation: handleGoalEvent,
  goal_created: handleGoalEvent,
  goal_progress: handleGoalEvent,
  goal_status_changed: handleGoalEvent,
  goal_updated: handleGoalEvent,
  input_queue: handleInputQueue,
  permission_request: handlePermissionRequest,
  permission_request_resolved: handlePermissionResolved,
  room_deleted: handleRoomEvent,
  room_directed_message: handleRoomEvent,
  room_directed_message_consumed: handleRoomEvent,
  room_member_added: handleRoomEvent,
  room_member_removed: handleRoomEvent,
  room_resync_required: handleRoomResync,
  round_status: handleRoundStatus,
  session_resync_required: handleSessionResync,
  session_status: handleSessionStatus,
  stream_cancelled: createMessageStatusHandler("cancelled"),
  stream_end: createMessageStatusHandler("done"),
  stream_start: createMessageStatusHandler("streaming"),
  workspace_event: handleWorkspaceEvent,
};
