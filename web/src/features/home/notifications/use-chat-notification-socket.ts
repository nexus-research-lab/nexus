/**
 * INPUT: 全局 Room 订阅目录、完成消息与删除事件回调。
 * OUTPUT: 可重连的单 WebSocket 完成事件流、按 Conversation/Session source 隔离的 Room 活动态和目录刷新信号。
 * POS: Home 全局聊天通知的协议边界；未读/删除状态由上层回调处理。
 */
import { useCallback, useEffect, useRef } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import {
  pruneRoomActivity,
  replaceRoomActivitySources,
  replaceRoomActivitySourceSnapshot,
  replaceRoomInteractionSnapshot,
  updateRoomActivity,
  updateRoomInteraction,
} from "@/features/home/room-activity-resource";
import { parseConversationMessage } from "@/lib/conversation/message-protocol";
import { notifyRoomDirectoryUpdated } from "@/lib/conversation/room-directory-events";
import { notifySessionRuntimeSettingsUpdated } from "@/lib/conversation/session-runtime-settings-events";
import { notifyCapabilitySummaryMutated } from "@/features/capability/capability-summary-events";
import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "@/features/conversation/shared/execution/workgraph-distillation-intent";
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
      if (readString(event.data, "reason") === "workgraph_distillation_changed") {
        notifyCapabilitySummaryMutated({ domain: "workgraph_distillation" });
        window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
      }
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

export function syncRoomActivity(
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

  const sourceKey = resolveRoomActivitySourceKey(event, roomId);

  if (event.event_type === "session_status") {
    // session_status 是 DM bind/reconnect 的权威恢复值。Room slot 有自己的
    // agent_round_status 与全局 active_sources，不能把成员 runtime 混进私聊。
    if (directoryIndex.roomsById.get(roomId)?.room_type !== "dm") {
      return;
    }
    const runningRoundIds = isStringArray(event.data.running_round_ids)
      ? event.data.running_round_ids
      : [];
    replaceRoomActivitySourceSnapshot(
      roomId,
      sourceKey,
      runningRoundIds,
      event.data.is_generating === true,
    );
    return;
  }

  if (event.event_type === "round_status") {
    const roundId = readString(event.data, "round_id") ?? event.round_id;
    updateRoomActivity(
      roomId,
      sourceKey,
      roundId,
      readString(event.data, "status"),
    );
    return;
  }

  if (event.event_type === "agent_round_status") {
    updateRoomActivity(
      roomId,
      sourceKey,
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
    if (event.data.activity_snapshot === true) {
      replaceRoomActivitySources(
        roomId,
        parseRoomActivitySources(event.data.active_sources),
      );
    }
    return;
  }

  if (event.event_type !== "chat_ack" || event.data.pending_snapshot !== true) {
    return;
  }
  const pending = Array.isArray(event.data.pending) ? event.data.pending : [];
  const runningRoundIds = pending
    .map((slot) => readString(slot, "round_id"))
    .filter((roundId): roundId is string => Boolean(roundId));
  const eventRoundId = readString(event.data, "round_id") ?? event.round_id;
  if (runningRoundIds.length === 0 && pending.length > 0 && eventRoundId) {
    runningRoundIds.push(eventRoundId);
  }
  replaceRoomActivitySourceSnapshot(
    roomId,
    sourceKey,
    runningRoundIds,
    pending.length > 0,
  );
}

function parseRoomActivitySources(value: unknown): Array<{
  runningRoundIds: string[];
  sourceKey: string;
}> {
  if (!Array.isArray(value)) {
    return [];
  }
  const sources: Array<{ runningRoundIds: string[]; sourceKey: string }> = [];
  for (const source of value) {
    const record = typeof source === "object" && source !== null
      ? source as Record<string, unknown>
      : null;
    if (!record) {
      continue;
    }
    const sessionKey = readString(record, "session_key");
    if (!sessionKey) {
      continue;
    }
    const runningRoundIds = isStringArray(record.running_round_ids)
      ? record.running_round_ids
      : [];
    sources.push({
      runningRoundIds,
      sourceKey: activitySourceKey(sessionKey),
    });
  }
  return sources;
}

function resolveRoomActivitySourceKey(event: EventMessage, roomId: string): string {
  const sessionKey = normalize(event.session_key);
  if (sessionKey) {
    return activitySourceKey(sessionKey);
  }
  const conversationId = normalize(event.conversation_id);
  if (conversationId) {
    return `conversation:${conversationId}`;
  }
  return `room:${roomId}`;
}

function activitySourceKey(sessionKey: string): string {
  return `session:${sessionKey}`;
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
