import type { EventMessage } from "@/types/generated/protocol";
import type { WebSocketMessage } from "@/types/system/websocket";

import {
  buildRoomSubscriptionMessage,
  buildSessionBindMessage,
} from "../../actions/conversation-command-builders";
import type {
  AgentEventContext,
  AgentEventHandlerMap,
} from "../agent-event-context";

interface SequenceCursor {
  current: number;
}

function advanceSequenceCursor(cursor: SequenceCursor, value: unknown): void {
  if (typeof value === "number") {
    cursor.current = Math.max(cursor.current, value);
  }
}

function reloadAndResubscribe(
  context: AgentEventContext,
  buildMessage: () => WebSocketMessage | null,
): void {
  void context.transport.reloadCurrentSession().finally(() => {
    if (context.transport.wsStateRef.current !== "connected") {
      return;
    }
    const message = buildMessage();
    if (message) {
      context.transport.wsSendRef.current(message);
    }
  });
}

function rewrittenRoundId(event: EventMessage): string | null {
  const reason = event.data?.reason;
  const targetRoundId = event.data?.target_round_id;
  if (reason !== "history_rewrite" || typeof targetRoundId !== "string") {
    return null;
  }
  return targetRoundId.trim() || null;
}

function handleRoomResync(
  event: EventMessage,
  context: AgentEventContext,
): void {
  const { roomId, conversationId } = context.scope;
  if (event.room_id !== roomId) {
    return;
  }

  advanceSequenceCursor(
    context.transport.roomSeqCursorRef,
    event.data?.latest_room_seq,
  );
  context.callbacks.onRoomEvent(event.event_type, event.data ?? {});

  reloadAndResubscribe(context, () => roomId
    ? buildRoomSubscriptionMessage({
      type: "subscribe_room",
      room_id: roomId,
      conversation_id: conversationId,
      last_seen_room_seq: context.transport.roomSeqCursorRef.current,
    })
    : null);
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

  advanceSequenceCursor(
    context.transport.sessionSeqCursorRef,
    event.data?.latest_session_seq,
  );
  const targetRoundId = rewrittenRoundId(event);
  if (targetRoundId) {
    context.runtime.removeRewrittenRound(targetRoundId);
  }
  context.callbacks.onRoomEvent(event.event_type, event.data ?? {});

  reloadAndResubscribe(context, () => {
    const { agentId, conversationId, roomId, sessionKey } = context.scope;
    return sessionKey ? buildSessionBindMessage({
      session_key: sessionKey,
      last_seen_session_seq: context.transport.sessionSeqCursorRef.current,
      agent_id: agentId,
      room_id: roomId,
      conversation_id: conversationId,
    }) : null;
  });
}

export const AGENT_RESYNC_EVENT_HANDLERS: AgentEventHandlerMap = {
  room_resync_required: handleRoomResync,
  session_resync_required: handleSessionResync,
};
