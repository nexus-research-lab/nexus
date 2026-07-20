import { useCallback, useEffect, useRef } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import {
  pruneRoomActivity,
  replaceRoomActivitySnapshot,
  updateRoomActivity,
} from "@/features/home/room-activity-resource";
import { parseConversationMessage } from "@/lib/conversation/message-protocol";
import { notifyRoomDirectoryUpdated } from "@/lib/conversation/room-directory-events";
import { readString } from "@/lib/unknown-value";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";
import type { AssistantMessage } from "@/types/conversation/message/entity";
import type { EventMessage } from "@/types/generated/protocol";

import { isCompletedAssistantMessage } from "./chat-notification-model";

interface UseChatNotificationSocketOptions {
  onCompletedMessage: (event: EventMessage, message: AssistantMessage) => void;
  roomIdsKey: string;
}

export function useChatNotificationSocket({
  onCompletedMessage,
  roomIdsKey,
}: UseChatNotificationSocketOptions): void {
  const roomSeqCursorRef = useRef<Record<string, number>>({});
  const handleMessage = useCallback((rawMessage: unknown) => {
    const event = parseEventMessage(rawMessage);
    if (!event) {
      return;
    }
    if (event.event_type === "directory_changed") {
      notifyRoomDirectoryUpdated();
      return;
    }
    syncRoomActivity(event);
    recordRoomSequence(roomSeqCursorRef.current, event);
    if (event.event_type === "room_resync_required") {
      recordResyncSequence(roomSeqCursorRef.current, event);
      notifyRoomDirectoryUpdated();
      return;
    }
    if (event.event_type !== "message" || event.delivery_mode === "ephemeral") {
      return;
    }
    const message = parseConversationMessage(event.data, {
      deliveryMode: event.delivery_mode,
      sessionKey: event.session_key,
    });
    if (message && isCompletedAssistantMessage(message)) {
      notifyRoomDirectoryUpdated();
      onCompletedMessage(event, message);
    }
  }, [onCompletedMessage]);

  const { send, state } = useWebSocket({
    url: getAgentWsUrl(),
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30_000,
    onMessage: handleMessage,
  });
  useAppEventSubscription(send, state);

  useEffect(() => {
    const roomIds = roomIdsKey ? roomIdsKey.split("\n") : [];
    pruneRoomActivity(new Set(roomIds));
    if (state !== "connected" || roomIds.length === 0) {
      return undefined;
    }
    for (const roomId of roomIds) {
      const lastSeenRoomSeq = roomSeqCursorRef.current[roomId] ?? 0;
      send({
        type: "subscribe_room",
        room_id: roomId,
        ...(lastSeenRoomSeq > 0 ? { last_seen_room_seq: lastSeenRoomSeq } : {}),
      });
    }
    return () => {
      for (const roomId of roomIds) {
        send({ type: "unsubscribe_room", room_id: roomId });
      }
    };
  }, [roomIdsKey, send, state]);
}

function syncRoomActivity(event: EventMessage): void {
  const roomId = event.room_id?.trim();
  if (!roomId) {
    return;
  }

  if (event.event_type === "round_status") {
    updateRoomActivity(
      roomId,
      readString(event.data, "round_id") ?? event.round_id,
      readString(event.data, "status"),
    );
    return;
  }

  if (event.event_type === "agent_round_status") {
    updateRoomActivity(
      roomId,
      readString(event.data, "round_id") ?? event.round_id,
      readString(event.data, "status"),
      "agent_round",
      readString(event.data, "agent_round_id") ?? event.agent_round_id,
    );
    return;
  }

  if (event.event_type !== "chat_ack" || event.data.pending_snapshot !== true) {
    return;
  }
  const pending = Array.isArray(event.data.pending) ? event.data.pending : [];
  replaceRoomActivitySnapshot(
    roomId,
    readString(event.data, "round_id") ?? event.round_id,
    pending.length > 0,
  );
}

function recordRoomSequence(cursor: Record<string, number>, event: EventMessage): void {
  if (event.room_id && typeof event.room_seq === "number") {
    cursor[event.room_id] = Math.max(cursor[event.room_id] ?? 0, event.room_seq);
  }
}

function recordResyncSequence(cursor: Record<string, number>, event: EventMessage): void {
  if (event.room_id && typeof event.data?.latest_room_seq === "number") {
    cursor[event.room_id] = Math.max(
      cursor[event.room_id] ?? 0,
      event.data.latest_room_seq,
    );
  }
}
