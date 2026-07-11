import type { EventMessage } from "@/types/generated/protocol";

import {
  buildRoomSubscriptionMessage,
  buildSessionBindMessage,
} from "../../actions/conversation-command-builders";
import type {
  AgentEventContext,
  AgentEventHandlerMap,
} from "../agent-event-context";

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
    !incomingSessionKey
    || !context.scope.isCurrentSessionEvent(incomingSessionKey)
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
      last_seen_session_seq: context.transport.sessionSeqCursorRef.current,
      agent_id: agentId,
      room_id: roomId,
      conversation_id: conversationId,
    }));
  });
}

export const AGENT_RESYNC_EVENT_HANDLERS: AgentEventHandlerMap = {
  room_resync_required: handleRoomResync,
  session_resync_required: handleSessionResync,
};
