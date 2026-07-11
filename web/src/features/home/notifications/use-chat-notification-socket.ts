import { useCallback, useEffect, useRef } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/options";
import { notifyRoomDirectoryUpdated } from "@/lib/conversation/room-directory-events";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import type {
  AssistantMessage,
  EventMessage,
  Message,
} from "@/types/conversation/message";

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
    const event = rawMessage as EventMessage;
    if (event.event_type === "directory_changed") {
      notifyRoomDirectoryUpdated();
      return;
    }
    recordRoomSequence(roomSeqCursorRef.current, event);
    if (event.event_type === "room_resync_required") {
      recordResyncSequence(roomSeqCursorRef.current, event);
      notifyRoomDirectoryUpdated();
      return;
    }
    if (event.event_type !== "message" || event.delivery_mode === "ephemeral") {
      return;
    }
    const message = event.data as Message;
    if (isCompletedAssistantMessage(message)) {
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
    if (state !== "connected" || !roomIdsKey) {
      return undefined;
    }
    const roomIds = roomIdsKey.split("\n");
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
