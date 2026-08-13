/**
 * INPUT: session-scoped WebSocket protocol events 与当前 conversation handler context。
 * OUTPUT: envelope session 匹配的资源事件投影，以及跨视图切换仍按本地 client_request_id 所有权收口的 ACK/明确拒绝。
 * POS: agent transport 的会话事件路由表；请求收口与当前页面投影分离，不猜测跨 session 活动。
 */
import { readString } from "@/lib/unknown-value";
import type { RoomEventPayload } from "@/types/agent/agent-conversation";
import type { AssistantMessageStatus } from "@/types/conversation/message/entity";
import type { EventMessage } from "@/types/generated/protocol";

import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../agent-event-context";
import { withCurrentSessionEvent } from "./handler-scope";
import {
  parseAgentRoundStatusEventPayload,
  parseChatAckData,
  parseCommandCatalogData,
  parseContextUsageData,
  parseInputQueueAckData,
  parseInterruptAckData,
  parseInputQueueEventPayload,
  parseRoundStatusEventPayload,
  parseRuntimeStatusData,
  selectCommandCatalogSnapshot,
  parseSessionStatusData,
} from "./session-event-data";

function getEventRoundId(event: EventMessage): string | null {
  const dataRoundId = typeof event.data?.round_id === "string"
    ? event.data.round_id.trim()
    : "";
  const envelopeRoundId = typeof event.round_id === "string"
    ? event.round_id.trim()
    : "";
  return dataRoundId || envelopeRoundId || null;
}

function isWorkspaceSubscriptionError(event: EventMessage): boolean {
  return readString(event.data, "type") === "subscribe_workspace"
    || readString(event.data, "error_type") === "workspace_subscription_error"
    || readString(event.data, "error_type") === "invalid_workspace_subscription";
}

const handleErrorEvent: AgentEventHandler = (event, context) => {
  if (isWorkspaceSubscriptionError(event)) {
    return;
  }
  const message = readString(event.data, "message") ?? "Unknown error";
  const clientRequestId = readString(event.data, "client_request_id") ?? "";
  if (clientRequestId) {
    // 请求归属由本地 mint 的 request ID 决定。即使用户已经切换页面，
    // 旧会话的明确拒绝仍必须结束原 Promise，但不能污染当前会话 UI。
    context.runtime.rejectPendingRequestAck(clientRequestId, message);
  }
  const incomingSessionKey = event.session_key || null;
  if (
    incomingSessionKey
    && !context.scope.isCurrentSessionEvent(incomingSessionKey)
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
  context.state.setError(message);
};

const handleSessionStatus = withCurrentSessionEvent((event, context) => {
  const payload = parseSessionStatusData(event.data);
  if (payload) {
    context.runtime.syncSessionStatus(payload);
  }
  context.callbacks?.onRoomEvent?.(event.event_type, event.data ?? {});
});

const handleRuntimeStatus = withCurrentSessionEvent((event, context) => {
  const payload = parseRuntimeStatusData(event.data);
  if (payload) {
    context.runtime.setRuntimeStatus(payload.status);
  }
});

const handleCommandCatalog: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  const currentAgentID = context.scope.agentId?.trim() ?? "";
  const incomingAgentID = event.agent_id?.trim() ?? "";
  if (!currentAgentID || !incomingAgentID || currentAgentID !== incomingAgentID) {
    return;
  }
  const payload = parseCommandCatalogData(event.data);
  if (
    payload
    && (!payload.agent_id || payload.agent_id === currentAgentID)
  ) {
    context.state.setCommandCatalog((current) => (
      selectCommandCatalogSnapshot(current, payload)
    ));
  }
};

const handleContextUsage: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
    return;
  }
  const incomingAgentID = event.agent_id?.trim() ?? "";
  const payload = parseContextUsageData(event.data);
  if (!incomingAgentID || !payload) {
    return;
  }
  context.state.setContextUsageByAgent((current) => ({
    ...current,
    [incomingAgentID]: payload,
  }));
};

const handleInputQueue = withCurrentSessionEvent((event, context) => {
  const payload = parseInputQueueEventPayload(event.data);
  if (payload) {
    context.state.setInputQueueItems(payload.items);
  }
});

const handleInputQueueAck: AgentEventHandler = (event, context) => {
  const ack = parseInputQueueAckData(event.data);
  if (ack?.accepted) {
    context.runtime.resolvePendingRequestAck(ack.client_request_id);
  }
};

const handleInterruptAck: AgentEventHandler = (event, context) => {
  const ack = parseInterruptAckData(event.data);
  if (ack?.accepted) {
    context.runtime.resolvePendingRequestAck(ack.client_request_id);
  }
};

const handleGoalEvent = withCurrentSessionEvent((event, context) => {
  context.callbacks.onRoomEvent(
    event.event_type,
    (event.data ?? {}) as RoomEventPayload,
  );
});

const handleExecutionInvalidated = withCurrentSessionEvent((event, context) => {
  context.callbacks.onRoomEvent(
    event.event_type,
    (event.data ?? {}) as RoomEventPayload,
  );
});

const handleRoundStatus = withCurrentSessionEvent((event, context) => {
  const payload = parseRoundStatusEventPayload(event.data);
  if (payload) {
    context.runtime.applyRoundStatus(payload.round_id, payload.status);
  }
  context.callbacks?.onRoomEvent?.(event.event_type, event.data ?? {});
});

const handleAgentRoundStatus = withCurrentSessionEvent((event, context) => {
  const payload = parseAgentRoundStatusEventPayload(event.data);
  if (payload) {
    context.runtime.applyAgentRoundStatus(payload);
  }
  context.callbacks?.onRoomEvent?.(event.event_type, event.data ?? {});
});

const handleChatAck: AgentEventHandler = (event, context) => {
  const ack = parseChatAckData(event.data);
  if (!ack) {
    return;
  }
  if (context.scope.isCurrentSessionEvent(event.session_key || null)) {
    context.runtime.trackChatAck(ack);
    return;
  }
  // 会话切换只阻止旧消息投影进当前 Feed，不能吞掉旧请求的 ACK。
  context.runtime.resolvePendingRequestAck(ack.client_request_id);
};

function createMessageStatusHandler(
  status: AssistantMessageStatus,
): AgentEventHandler {
  return withCurrentSessionEvent((event, context) => {
    const messageId = event.message_id || readString(event.data, "msg_id");
    if (messageId) {
      context.runtime.updateMessageStatus(
        messageId,
        status,
        readString(event.data, "round_id"),
      );
    }
  });
}

export const AGENT_SESSION_EVENT_HANDLERS: AgentEventHandlerMap = {
  agent_round_status: handleAgentRoundStatus,
  chat_ack: handleChatAck,
  command_catalog: handleCommandCatalog,
  context_usage: handleContextUsage,
  error: handleErrorEvent,
  execution_invalidated: handleExecutionInvalidated,
  goal_cleared: handleGoalEvent,
  goal_continuation: handleGoalEvent,
  goal_created: handleGoalEvent,
  goal_progress: handleGoalEvent,
  goal_status_changed: handleGoalEvent,
  goal_updated: handleGoalEvent,
  input_queue: handleInputQueue,
  input_queue_ack: handleInputQueueAck,
  interrupt_ack: handleInterruptAck,
  round_status: handleRoundStatus,
  runtime_status: handleRuntimeStatus,
  session_status: handleSessionStatus,
  stream_cancelled: createMessageStatusHandler("cancelled"),
  stream_end: createMessageStatusHandler("done"),
  stream_start: createMessageStatusHandler("streaming"),
};
