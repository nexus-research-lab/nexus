/**
 * INPUT: 全局 Room 订阅目录、完成消息与删除事件回调。
 * OUTPUT: 可重连的单 WebSocket 完成事件流、Room 活动态和目录刷新信号。
 * POS: Home 全局聊天通知的协议边界；未读/删除状态由上层回调处理。
 */
import { useCallback, useEffect, useRef } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import {
  pruneRoomActivity,
  replaceRoomActivitySnapshot,
  replaceRoomInteractionSnapshot,
  updateRoomActivity,
  updateRoomInteraction,
} from "@/features/home/room-activity-resource";
import { parseConversationMessage } from "@/lib/conversation/message-protocol";
import { notifyRoomDirectoryUpdated } from "@/lib/conversation/room-directory-events";
import { notifySessionRuntimeSettingsUpdated } from "@/lib/conversation/session-runtime-settings-events";
import { isStringArray, readString } from "@/lib/unknown-value";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";
import { useAgentStore } from "@/store/agent";
import type { AssistantMessage } from "@/types/conversation/message/entity";
import type { EventMessage } from "@/types/generated/protocol";

import type { ChatNotificationDirectoryIndex } from "./chat-notification-directory";
import { isCompletedAssistantMessage } from "./chat-notification-model";

interface UseChatNotificationSocketOptions {
  directoryIndex: ChatNotificationDirectoryIndex;
  onCompletedMessage: (event: EventMessage, message: AssistantMessage) => void;
  onRoomDeleted?: (roomId: string) => void;
  roomIdsKey: string;
}

export function useChatNotificationSocket({
  directoryIndex,
  onCompletedMessage,
  onRoomDeleted,
  roomIdsKey,
}: UseChatNotificationSocketOptions): void {
  const roomSeqCursorRef = useRef<Record<string, number>>({});
  const hasConnectedRef = useRef(false);
  const directoryIndexRef = useRef(directoryIndex);
  directoryIndexRef.current = directoryIndex;
  const handleMessage = useCallback((rawMessage: unknown) => {
    const event = parseEventMessage(rawMessage);
    if (!event) {
      return;
    }
    if (event.event_type === "room_deleted") {
      const roomId = resolveRoomActivityRoomId(
        event,
        directoryIndexRef.current,
      );
      if (roomId) {
        onRoomDeleted?.(roomId);
      }
      notifyRoomDirectoryUpdated();
      return;
    }
    if (event.event_type === "directory_changed") {
      if (
        readString(event.data, "reason")
        === "session_runtime_settings_updated"
      ) {
        notifySessionRuntimeSettingsUpdated(
          readString(event.data, "session_key") ?? "",
        );
      }
      refreshAgentDirectory(event);
      notifyRoomDirectoryUpdated();
      return;
    }
    syncRoomActivity(event, directoryIndexRef.current);
    recordRoomSequence(roomSeqCursorRef.current, event);
    if (event.event_type === "room_resync_required") {
      recordResyncSequence(roomSeqCursorRef.current, event);
      notifyRoomDirectoryUpdated();
      return;
    }
    if (event.event_type !== "message" || event.delivery_mode !== "durable") {
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
  }, [onCompletedMessage, onRoomDeleted]);

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
    if (state !== "connected") {
      return;
    }
    if (hasConnectedRef.current) {
      // 重连期间可能错过全局目录事件，连接恢复后只补刷一次。
      notifyRoomDirectoryUpdated();
    }
    hasConnectedRef.current = true;
  }, [state]);

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

function refreshAgentDirectory(event: EventMessage): void {
  const reason = readString(event.data, "reason") ?? "";
  if (!reason.startsWith("agent_")) {
    return;
  }
  void useAgentStore.getState().load_agents_from_server();
}

function syncRoomActivity(
  event: EventMessage,
  directoryIndex: ChatNotificationDirectoryIndex,
): void {
  const roomId = resolveRoomActivityRoomId(event, directoryIndex);
  if (!roomId) {
    return;
  }

  if (event.event_type === "permission_request") {
    updateRoomInteraction(
      roomId,
      readString(event.data, "request_id"),
      true,
    );
    return;
  }

  if (event.event_type === "permission_request_resolved") {
    updateRoomInteraction(
      roomId,
      readString(event.data, "request_id"),
      false,
    );
    return;
  }

  if (event.event_type === "round_status") {
    const roundId = readString(event.data, "round_id") ?? event.round_id;
    updateRoomActivity(roomId, roundId, readString(event.data, "status"));
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

  if (
    event.event_type === "chat_ack"
    && event.data.pending_interaction_snapshot === true
  ) {
    replaceRoomInteractionSnapshot(
      roomId,
      isStringArray(event.data.pending_interaction_request_ids)
        ? event.data.pending_interaction_request_ids
        : [],
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
    isStringArray(event.data.pending_interaction_request_ids)
      ? event.data.pending_interaction_request_ids
      : [],
  );
}

function resolveRoomActivityRoomId(
  event: EventMessage,
  directoryIndex: ChatNotificationDirectoryIndex,
): string | null {
  const eventRoomId = normalize(event.room_id);
  const eventConversationId = normalize(event.conversation_id);
  const sessionKey = normalize(event.session_key);
  const sessionConversation = sessionKey
    ? directoryIndex.conversationsBySessionKey.get(sessionKey)
    : undefined;
  const conversation = (eventConversationId
    ? directoryIndex.conversationsById.get(eventConversationId)
    : undefined) ?? sessionConversation;

  return eventRoomId || normalize(conversation?.room_id) || null;
}

function normalize(value: string | null | undefined): string {
  return value?.trim() ?? "";
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
