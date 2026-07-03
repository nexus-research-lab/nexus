import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation } from "react-router-dom";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/options";
import { getLauncherBootstrapApi } from "@/lib/api/launcher-api";
import {
  notifyRoomDirectoryUpdated,
  subscribeRoomDirectoryUpdates,
} from "@/lib/api/room-api";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import {
  type ChatNotificationTargetState,
  useSidebarStore,
} from "@/store/sidebar";
import {
  buildChatNotificationTargetKey,
  getActiveChatTargetFromPath,
  isChatNotificationTargetActive,
  type ActiveChatNotificationTarget,
} from "./chat-notification-target";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";
import type {
  AssistantMessage,
  ContentBlock,
  EventMessage,
  Message,
} from "@/types/conversation/message";

interface ChatNotificationDirectory {
  agents: LauncherAgentSummary[];
  conversations: LauncherConversationSummary[];
  rooms: LauncherRoomSummary[];
}

interface ChatNotificationTarget {
  agent_id?: string | null;
  conversation_id?: string | null;
  key: string;
  room_id?: string | null;
  session_key?: string | null;
}

const EMPTY_DIRECTORY: ChatNotificationDirectory = {
  agents: [],
  conversations: [],
  rooms: [],
};
const CHAT_NOTIFICATION_TEXT_LIMIT = 120;

let chatNotificationDirectoryCache: ChatNotificationDirectory | null = null;

function isWindowActive(): boolean {
  if (typeof document === "undefined") {
    return true;
  }
  return document.visibilityState === "visible" && document.hasFocus();
}

function supportsBrowserNotification(): boolean {
  return typeof window !== "undefined" && "Notification" in window;
}

function requestNotificationPermission(): void {
  if (!supportsBrowserNotification() || Notification.permission !== "default") {
    return;
  }

  void Notification.requestPermission().catch(() => {});
}

function showBrowserNotification(title: string, body: string, tag: string): void {
  if (!supportsBrowserNotification() || Notification.permission !== "granted") {
    return;
  }
  if (isWindowActive()) {
    return;
  }

  const notification = new Notification(title, {
    body,
    tag,
  });
  notification.onclick = () => {
    window.focus();
    notification.close();
  };
}

function isCompletedAssistantMessage(message: Message): message is AssistantMessage {
  if (message.role !== "assistant") {
    return false;
  }
  if (message.result_summary?.subtype === "interrupted") {
    return false;
  }
  return Boolean(
    message.result_summary ||
      message.is_complete ||
      message.stop_reason ||
      message.stream_status === "done" ||
      message.stream_status === "error",
  );
}

function extractTextFromContent(content?: ContentBlock[] | null): string {
  if (!content || content.length === 0) {
    return "";
  }

  return content
    .filter((block): block is Extract<ContentBlock, { type: "text" }> =>
      block.type === "text" && Boolean(block.text.trim()))
    .map((block) => block.text.trim())
    .join("\n\n");
}

function compactNotificationText(value: string): string {
  const normalized = value.replace(/\s+/g, " ").trim();
  if (normalized.length <= CHAT_NOTIFICATION_TEXT_LIMIT) {
    return normalized;
  }
  return `${normalized.slice(0, CHAT_NOTIFICATION_TEXT_LIMIT - 1)}…`;
}

function getMessageNotificationBody(message: AssistantMessage): string {
  const summaryResult = message.result_summary?.result?.trim();
  if (summaryResult) {
    return compactNotificationText(summaryResult);
  }
  if (message.result_summary?.subtype === "error" || message.result_summary?.is_error) {
    return "执行失败";
  }

  const text = extractTextFromContent(message.content);
  if (text) {
    return compactNotificationText(text);
  }
  return "处理完成";
}

function buildDirectoryMaps(directory: ChatNotificationDirectory) {
  const conversationsWithId = directory.conversations.filter((conversation) => conversation.conversation_id);
  const conversationsWithSessionKey = directory.conversations.filter((conversation) => conversation.session_key);
  return {
    agent_by_id: new Map(directory.agents.map((agent) => [agent.id, agent])),
    conversation_by_id: new Map(
      conversationsWithId.map((conversation) => [conversation.conversation_id as string, conversation]),
    ),
    conversation_by_session_key: new Map(
      conversationsWithSessionKey.map((conversation) => [conversation.session_key, conversation]),
    ),
    room_by_id: new Map(directory.rooms.map((room) => [room.id, room])),
  };
}

function buildNotificationTitleAndBody(
  target: ChatNotificationTarget,
  message: AssistantMessage,
  directory: ChatNotificationDirectory,
): { body: string; title: string } {
  const { agent_by_id: agentById, conversation_by_id: conversationById, room_by_id: roomById } = buildDirectoryMaps(directory);
  const room = target.room_id ? roomById.get(target.room_id) : undefined;
  const conversation = target.conversation_id
    ? conversationById.get(target.conversation_id)
    : undefined;
  const agent = message.agent_id ? agentById.get(message.agent_id) : undefined;

  const title = room?.room_type === "dm"
    ? agent?.name ?? conversation?.title ?? room?.name ?? "Nexus"
    : room?.name?.trim() || conversation?.title?.trim() || "群聊";
  const body = getMessageNotificationBody(message);
  if (room?.room_type === "room" && agent?.name) {
    return {
      title,
      body: compactNotificationText(`${agent.name}: ${body}`),
    };
  }
  return { title, body };
}

function buildMessageTarget(
  event: EventMessage,
  message: Message,
  directory: ChatNotificationDirectory,
): ChatNotificationTarget | null {
  const { conversation_by_id: conversationById, conversation_by_session_key: conversationBySessionKey } = buildDirectoryMaps(directory);
  const eventConversationId = event.conversation_id ?? message.conversation_id ?? null;
  const sessionKey = event.session_key ?? message.session_key ?? null;
  const directoryConversation = eventConversationId
    ? conversationById.get(eventConversationId)
    : sessionKey ? conversationBySessionKey.get(sessionKey) : undefined;
  const conversationId = eventConversationId ?? directoryConversation?.conversation_id ?? null;
  const roomId = event.room_id ?? message.room_id ?? directoryConversation?.room_id ?? null;
  const key = buildChatNotificationTargetKey({
    conversation_id: conversationId,
    room_id: roomId,
    session_key: sessionKey,
  });
  if (!key) {
    return null;
  }
  return {
    agent_id: event.agent_id ?? message.agent_id ?? null,
    conversation_id: conversationId,
    key,
    room_id: roomId,
    session_key: sessionKey,
  };
}

function toChatNotificationTargetState(
  target: ChatNotificationTarget,
): ChatNotificationTargetState {
  return {
    conversation_id: target.conversation_id,
    key: target.key,
    room_id: target.room_id,
    session_key: target.session_key,
  };
}

function getNotificationMessageId(
  event: EventMessage,
  message: AssistantMessage,
  targetKey: string,
): string {
  return (
    message.message_id ||
    event.message_id ||
    message.result_summary?.message_id ||
    `${targetKey}:${message.round_id}:${event.timestamp}`
  );
}

export function useChatCompletionNotifications(): void {
  const location = useLocation();
  const wsUrl = getAgentWsUrl();
  const recordChatNotification = useSidebarStore((s) => s.record_chat_notification);
  const clearChatNotificationsForTarget = useSidebarStore(
    (s) => s.clear_chat_notifications_for_target,
  );
  const clearChatNotificationsForRoom = useSidebarStore(
    (s) => s.clear_chat_notifications_for_room,
  );
  const [directory, setDirectory] = useState<ChatNotificationDirectory>(
    () => chatNotificationDirectoryCache ?? EMPTY_DIRECTORY,
  );
  const activeTargetRef = useRef<ActiveChatNotificationTarget | null>(
    getActiveChatTargetFromPath(location.pathname),
  );
  const directoryRef = useRef(directory);
  const roomSeqCursorRef = useRef<Record<string, number>>({});

  const clearRoomNotifications = useCallback((roomId: string | null | undefined) => {
    if (!roomId) {
      return;
    }
    clearChatNotificationsForRoom(roomId);
    const sessionTargetKeys = new Set(
      directoryRef.current.conversations
        .filter((conversation) => conversation.room_id === roomId)
        .map((conversation) => buildChatNotificationTargetKey({
          session_key: conversation.session_key,
        }))
        .filter((key): key is string => Boolean(key)),
    );
    for (const sessionTargetKey of sessionTargetKeys) {
      clearChatNotificationsForTarget(sessionTargetKey);
    }
  }, [clearChatNotificationsForRoom, clearChatNotificationsForTarget]);

  const clearActiveTargetNotifications = useCallback(() => {
    if (!isWindowActive()) {
      return;
    }
    const activeTarget = activeTargetRef.current;
    if (activeTarget?.room_id) {
      clearRoomNotifications(activeTarget.room_id);
      return;
    }
    clearChatNotificationsForTarget(activeTarget?.key);
  }, [clearChatNotificationsForTarget, clearRoomNotifications]);

  useEffect(() => {
    activeTargetRef.current = getActiveChatTargetFromPath(location.pathname);
    clearActiveTargetNotifications();
  }, [clearActiveTargetNotifications, location.pathname]);

  useEffect(() => {
    directoryRef.current = directory;
    clearActiveTargetNotifications();
  }, [clearActiveTargetNotifications, directory]);

  useEffect(() => {
    if (!supportsBrowserNotification() || Notification.permission !== "default") {
      return;
    }

    window.addEventListener("pointerdown", requestNotificationPermission, {
      capture: true,
      once: true,
    });
    window.addEventListener("keydown", requestNotificationPermission, {
      capture: true,
      once: true,
    });
    return () => {
      window.removeEventListener("pointerdown", requestNotificationPermission, {
        capture: true,
      });
      window.removeEventListener("keydown", requestNotificationPermission, {
        capture: true,
      });
    };
  }, []);

  const refreshDirectory = useCallback(() => {
    void getLauncherBootstrapApi().then((payload) => {
      const nextDirectory = {
        agents: payload.agents,
        conversations: payload.conversations,
        rooms: payload.rooms,
      };
      chatNotificationDirectoryCache = nextDirectory;
      setDirectory(nextDirectory);
    }).catch((error) => {
      console.error("[ChatCompletionNotifications] 加载聊天通知目录失败:", error);
    });
  }, []);

  useEffect(() => {
    refreshDirectory();
    return subscribeRoomDirectoryUpdates(refreshDirectory);
  }, [refreshDirectory]);

  useEffect(() => {
    window.addEventListener("focus", clearActiveTargetNotifications);
    document.addEventListener("visibilitychange", clearActiveTargetNotifications);
    return () => {
      window.removeEventListener("focus", clearActiveTargetNotifications);
      document.removeEventListener("visibilitychange", clearActiveTargetNotifications);
    };
  }, [clearActiveTargetNotifications]);

  const roomIds = useMemo(
    () => directory.rooms.map((room) => room.id).filter(Boolean).sort(),
    [directory.rooms],
  );
  const roomIdsKey = roomIds.join("\n");

  const handleWebsocketMessage = useCallback((rawMessage: unknown) => {
    const event = rawMessage as EventMessage;
    if (event.event_type === "directory_changed") {
      notifyRoomDirectoryUpdated();
      return;
    }
    if (event.room_id && typeof event.room_seq === "number") {
      roomSeqCursorRef.current[event.room_id] = Math.max(
        roomSeqCursorRef.current[event.room_id] ?? 0,
        event.room_seq,
      );
    }

    if (event.event_type === "room_resync_required") {
      if (event.room_id && typeof event.data?.latest_room_seq === "number") {
        roomSeqCursorRef.current[event.room_id] = Math.max(
          roomSeqCursorRef.current[event.room_id] ?? 0,
          event.data.latest_room_seq,
        );
      }
      notifyRoomDirectoryUpdated();
      return;
    }

    if (event.event_type !== "message" || event.delivery_mode === "ephemeral") {
      return;
    }

    const message = event.data as Message;
    if (!isCompletedAssistantMessage(message)) {
      return;
    }

    const target = buildMessageTarget(event, message, directoryRef.current);
    if (!target) {
      return;
    }

    notifyRoomDirectoryUpdated();
    const activeTarget = activeTargetRef.current;
    const targetIsActive = isChatNotificationTargetActive(activeTarget, target);
    if (targetIsActive && isWindowActive()) {
      if (target.room_id) {
        clearRoomNotifications(target.room_id);
      } else {
        clearChatNotificationsForTarget(target.key);
      }
      return;
    }

    const messageId = getNotificationMessageId(event, message, target.key);
    const didRecord = recordChatNotification(toChatNotificationTargetState(target), messageId);
    if (!didRecord) {
      return;
    }

    const { body, title } = buildNotificationTitleAndBody(
      target,
      message,
      directoryRef.current,
    );
    showBrowserNotification(title, body, messageId);
  }, [clearChatNotificationsForTarget, clearRoomNotifications, recordChatNotification]);

  const { send: wsSend, state: wsState } = useWebSocket({
    url: wsUrl,
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: handleWebsocketMessage,
  });

  useAppEventSubscription(wsSend, wsState);

  useEffect(() => {
    if (wsState !== "connected" || roomIds.length === 0) {
      return;
    }

    for (const roomId of roomIds) {
      const lastSeenRoomSeq = roomSeqCursorRef.current[roomId] ?? 0;
      wsSend({
        type: "subscribe_room",
        room_id: roomId,
        ...(lastSeenRoomSeq > 0 ? { last_seen_room_seq: lastSeenRoomSeq } : {}),
      });
    }

    return () => {
      for (const roomId of roomIds) {
        wsSend({
          type: "unsubscribe_room",
          room_id: roomId,
        });
      }
    };
    // roomIdsKey 是稳定依赖，避免数组引用导致反复重订阅。
  }, [roomIds, roomIdsKey, wsSend, wsState]);
}
