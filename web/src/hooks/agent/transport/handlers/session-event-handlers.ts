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
import type { AssistantMessageStatus } from "@/types/conversation/message";

import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../agent-event-context";
import { withCurrentSessionEvent } from "./handler-scope";

function getEventRoundId(event: EventMessage): string | null {
  const dataRoundId = typeof event.data?.round_id === "string"
    ? event.data.round_id.trim()
    : "";
  const envelopeRoundId = typeof event.round_id === "string"
    ? event.round_id.trim()
    : "";
  return dataRoundId || envelopeRoundId || null;
}

const handleErrorEvent: AgentEventHandler = (event, context) => {
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
  const message = event.data?.message || "Unknown error";
  const clientRequestId = typeof event.data?.client_request_id === "string"
    ? event.data.client_request_id
    : "";
  if (clientRequestId) {
    context.runtime.rejectChatAck(clientRequestId, message);
  }
  context.state.setError(message);
};

const handleSessionStatus = withCurrentSessionEvent((event, context) => {
  context.runtime.syncSessionStatus(
    (event.data ?? {}) as SessionStatusEventPayload,
  );
});

const handleInputQueue = withCurrentSessionEvent((event, context) => {
  const payload = (event.data ?? {}) as InputQueueEventPayload;
  context.state.setInputQueueItems(
    Array.isArray(payload.items) ? payload.items : [],
  );
});

const handleGoalEvent = withCurrentSessionEvent((event, context) => {
  context.callbacks.onRoomEvent(
    event.event_type,
    (event.data ?? {}) as RoomEventPayload,
  );
});

const handleRoundStatus = withCurrentSessionEvent((event, context) => {
  const payload = (event.data ?? {}) as RoundStatusEventPayload;
  if (payload.round_id && payload.status) {
    context.runtime.applyRoundStatus(payload.round_id, payload.status);
  }
});

const handleAgentRoundStatus = withCurrentSessionEvent((event, context) => {
  const payload = (event.data ?? {}) as AgentRoundStatusEventPayload;
  if (payload.agent_round_id && payload.status) {
    context.runtime.applyAgentRoundStatus(payload);
  }
});

const handleChatAck = withCurrentSessionEvent((event, context) => {
  const ack = event.data as ChatAckData;
  if (ack?.round_id) {
    context.runtime.trackChatAck(ack);
  }
});

function createMessageStatusHandler(
  status: AssistantMessageStatus,
): AgentEventHandler {
  return withCurrentSessionEvent((event, context) => {
    const messageId = event.message_id || event.data?.msg_id;
    if (typeof messageId === "string" && messageId) {
      context.runtime.updateMessageStatus(
        messageId,
        status,
        event.data?.round_id,
      );
    }
  });
}

export const AGENT_SESSION_EVENT_HANDLERS: AgentEventHandlerMap = {
  agent_round_status: handleAgentRoundStatus,
  chat_ack: handleChatAck,
  error: handleErrorEvent,
  goal_cleared: handleGoalEvent,
  goal_continuation: handleGoalEvent,
  goal_created: handleGoalEvent,
  goal_progress: handleGoalEvent,
  goal_status_changed: handleGoalEvent,
  goal_updated: handleGoalEvent,
  input_queue: handleInputQueue,
  round_status: handleRoundStatus,
  session_status: handleSessionStatus,
  stream_cancelled: createMessageStatusHandler("cancelled"),
  stream_end: createMessageStatusHandler("done"),
  stream_start: createMessageStatusHandler("streaming"),
};
