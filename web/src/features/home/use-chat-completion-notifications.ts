import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation } from "react-router-dom";

import { get_desktop_websocket_protocols } from "@/config/desktop-runtime";
import { get_agent_ws_url } from "@/config/options";
import { get_launcher_bootstrap_api } from "@/lib/api/launcher-api";
import {
  notify_room_directory_updated,
  subscribe_room_directory_updates,
} from "@/lib/api/room-api";
import { useWebSocket } from "@/lib/websocket";
import {
  type ChatNotificationTargetState,
  useSidebarStore,
} from "@/store/sidebar";
import {
  build_chat_notification_target_key,
  get_active_chat_target_from_path,
  is_chat_notification_target_active,
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

let chat_notification_directory_cache: ChatNotificationDirectory | null = null;

function is_window_active(): boolean {
  if (typeof document === "undefined") {
    return true;
  }
  return document.visibilityState === "visible" && document.hasFocus();
}

function supports_browser_notification(): boolean {
  return typeof window !== "undefined" && "Notification" in window;
}

function request_notification_permission(): void {
  if (!supports_browser_notification() || Notification.permission !== "default") {
    return;
  }

  void Notification.requestPermission().catch(() => {});
}

function show_browser_notification(title: string, body: string, tag: string): void {
  if (!supports_browser_notification() || Notification.permission !== "granted") {
    return;
  }
  if (is_window_active()) {
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

function is_completed_assistant_message(message: Message): message is AssistantMessage {
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

function extract_text_from_content(content?: ContentBlock[] | null): string {
  if (!content || content.length === 0) {
    return "";
  }

  return content
    .filter((block): block is Extract<ContentBlock, { type: "text" }> =>
      block.type === "text" && Boolean(block.text.trim()))
    .map((block) => block.text.trim())
    .join("\n\n");
}

function compact_notification_text(value: string): string {
  const normalized = value.replace(/\s+/g, " ").trim();
  if (normalized.length <= CHAT_NOTIFICATION_TEXT_LIMIT) {
    return normalized;
  }
  return `${normalized.slice(0, CHAT_NOTIFICATION_TEXT_LIMIT - 1)}…`;
}

function get_message_notification_body(message: AssistantMessage): string {
  const summary_result = message.result_summary?.result?.trim();
  if (summary_result) {
    return compact_notification_text(summary_result);
  }
  if (message.result_summary?.subtype === "error" || message.result_summary?.is_error) {
    return "执行失败";
  }

  const text = extract_text_from_content(message.content);
  if (text) {
    return compact_notification_text(text);
  }
  return "处理完成";
}

function build_directory_maps(directory: ChatNotificationDirectory) {
  const conversations_with_id = directory.conversations.filter((conversation) => conversation.conversation_id);
  const conversations_with_session_key = directory.conversations.filter((conversation) => conversation.session_key);
  return {
    agent_by_id: new Map(directory.agents.map((agent) => [agent.id, agent])),
    conversation_by_id: new Map(
      conversations_with_id.map((conversation) => [conversation.conversation_id as string, conversation]),
    ),
    conversation_by_session_key: new Map(
      conversations_with_session_key.map((conversation) => [conversation.session_key, conversation]),
    ),
    room_by_id: new Map(directory.rooms.map((room) => [room.id, room])),
  };
}

function build_notification_title_and_body(
  target: ChatNotificationTarget,
  message: AssistantMessage,
  directory: ChatNotificationDirectory,
): { body: string; title: string } {
  const { agent_by_id, conversation_by_id, room_by_id } = build_directory_maps(directory);
  const room = target.room_id ? room_by_id.get(target.room_id) : undefined;
  const conversation = target.conversation_id
    ? conversation_by_id.get(target.conversation_id)
    : undefined;
  const agent = message.agent_id ? agent_by_id.get(message.agent_id) : undefined;

  const title = room?.room_type === "dm"
    ? agent?.name ?? conversation?.title ?? room?.name ?? "Nexus"
    : room?.name?.trim() || conversation?.title?.trim() || "群聊";
  const body = get_message_notification_body(message);
  if (room?.room_type === "room" && agent?.name) {
    return {
      title,
      body: compact_notification_text(`${agent.name}: ${body}`),
    };
  }
  return { title, body };
}

function build_message_target(
  event: EventMessage,
  message: Message,
  directory: ChatNotificationDirectory,
): ChatNotificationTarget | null {
  const { conversation_by_id, conversation_by_session_key } = build_directory_maps(directory);
  const event_conversation_id = event.conversation_id ?? message.conversation_id ?? null;
  const session_key = event.session_key ?? message.session_key ?? null;
  const directory_conversation = event_conversation_id
    ? conversation_by_id.get(event_conversation_id)
    : session_key ? conversation_by_session_key.get(session_key) : undefined;
  const conversation_id = event_conversation_id ?? directory_conversation?.conversation_id ?? null;
  const room_id = event.room_id ?? message.room_id ?? directory_conversation?.room_id ?? null;
  const key = build_chat_notification_target_key({
    conversation_id,
    room_id,
    session_key,
  });
  if (!key) {
    return null;
  }
  return {
    agent_id: event.agent_id ?? message.agent_id ?? null,
    conversation_id,
    key,
    room_id,
    session_key,
  };
}

function to_chat_notification_target_state(
  target: ChatNotificationTarget,
): ChatNotificationTargetState {
  return {
    conversation_id: target.conversation_id,
    key: target.key,
    room_id: target.room_id,
    session_key: target.session_key,
  };
}

function get_notification_message_id(
  event: EventMessage,
  message: AssistantMessage,
  target_key: string,
): string {
  return (
    message.message_id ||
    event.message_id ||
    message.result_summary?.message_id ||
    `${target_key}:${message.round_id}:${event.timestamp}`
  );
}

export function useChatCompletionNotifications(): void {
  const location = useLocation();
  const ws_url = get_agent_ws_url();
  const record_chat_notification = useSidebarStore((s) => s.record_chat_notification);
  const clear_chat_notifications_for_target = useSidebarStore(
    (s) => s.clear_chat_notifications_for_target,
  );
  const clear_chat_notifications_for_room = useSidebarStore(
    (s) => s.clear_chat_notifications_for_room,
  );
  const [directory, set_directory] = useState<ChatNotificationDirectory>(
    () => chat_notification_directory_cache ?? EMPTY_DIRECTORY,
  );
  const active_target_ref = useRef<ActiveChatNotificationTarget | null>(
    get_active_chat_target_from_path(location.pathname),
  );
  const directory_ref = useRef(directory);
  const room_seq_cursor_ref = useRef<Record<string, number>>({});

  const clear_room_notifications = useCallback((room_id: string | null | undefined) => {
    if (!room_id) {
      return;
    }
    clear_chat_notifications_for_room(room_id);
    const session_target_keys = new Set(
      directory_ref.current.conversations
        .filter((conversation) => conversation.room_id === room_id)
        .map((conversation) => build_chat_notification_target_key({
          session_key: conversation.session_key,
        }))
        .filter((key): key is string => Boolean(key)),
    );
    for (const session_target_key of session_target_keys) {
      clear_chat_notifications_for_target(session_target_key);
    }
  }, [clear_chat_notifications_for_room, clear_chat_notifications_for_target]);

  const clear_active_target_notifications = useCallback(() => {
    if (!is_window_active()) {
      return;
    }
    const active_target = active_target_ref.current;
    if (active_target?.room_id) {
      clear_room_notifications(active_target.room_id);
      return;
    }
    clear_chat_notifications_for_target(active_target?.key);
  }, [clear_chat_notifications_for_target, clear_room_notifications]);

  useEffect(() => {
    active_target_ref.current = get_active_chat_target_from_path(location.pathname);
    clear_active_target_notifications();
  }, [clear_active_target_notifications, location.pathname]);

  useEffect(() => {
    directory_ref.current = directory;
    clear_active_target_notifications();
  }, [clear_active_target_notifications, directory]);

  useEffect(() => {
    if (!supports_browser_notification() || Notification.permission !== "default") {
      return;
    }

    window.addEventListener("pointerdown", request_notification_permission, {
      capture: true,
      once: true,
    });
    window.addEventListener("keydown", request_notification_permission, {
      capture: true,
      once: true,
    });
    return () => {
      window.removeEventListener("pointerdown", request_notification_permission, {
        capture: true,
      });
      window.removeEventListener("keydown", request_notification_permission, {
        capture: true,
      });
    };
  }, []);

  const refresh_directory = useCallback(() => {
    void get_launcher_bootstrap_api().then((payload) => {
      const next_directory = {
        agents: payload.agents,
        conversations: payload.conversations,
        rooms: payload.rooms,
      };
      chat_notification_directory_cache = next_directory;
      set_directory(next_directory);
    }).catch((error) => {
      console.error("[ChatCompletionNotifications] 加载聊天通知目录失败:", error);
    });
  }, []);

  useEffect(() => {
    refresh_directory();
    return subscribe_room_directory_updates(refresh_directory);
  }, [refresh_directory]);

  useEffect(() => {
    window.addEventListener("focus", clear_active_target_notifications);
    document.addEventListener("visibilitychange", clear_active_target_notifications);
    return () => {
      window.removeEventListener("focus", clear_active_target_notifications);
      document.removeEventListener("visibilitychange", clear_active_target_notifications);
    };
  }, [clear_active_target_notifications]);

  const room_ids = useMemo(
    () => directory.rooms.map((room) => room.id).filter(Boolean).sort(),
    [directory.rooms],
  );
  const room_ids_key = room_ids.join("\n");

  const handle_websocket_message = useCallback((raw_message: unknown) => {
    const event = raw_message as EventMessage;
    if (event.room_id && typeof event.room_seq === "number") {
      room_seq_cursor_ref.current[event.room_id] = Math.max(
        room_seq_cursor_ref.current[event.room_id] ?? 0,
        event.room_seq,
      );
    }

    if (event.event_type === "room_resync_required") {
      if (event.room_id && typeof event.data?.latest_room_seq === "number") {
        room_seq_cursor_ref.current[event.room_id] = Math.max(
          room_seq_cursor_ref.current[event.room_id] ?? 0,
          event.data.latest_room_seq,
        );
      }
      notify_room_directory_updated();
      return;
    }

    if (event.event_type !== "message" || event.delivery_mode === "ephemeral") {
      return;
    }

    const message = event.data as Message;
    if (!is_completed_assistant_message(message)) {
      return;
    }

    const target = build_message_target(event, message, directory_ref.current);
    if (!target) {
      return;
    }

    notify_room_directory_updated();
    const active_target = active_target_ref.current;
    const target_is_active = is_chat_notification_target_active(active_target, target);
    if (target_is_active && is_window_active()) {
      if (target.room_id) {
        clear_room_notifications(target.room_id);
      } else {
        clear_chat_notifications_for_target(target.key);
      }
      return;
    }

    const message_id = get_notification_message_id(event, message, target.key);
    const did_record = record_chat_notification(to_chat_notification_target_state(target), message_id);
    if (!did_record) {
      return;
    }

    const { body, title } = build_notification_title_and_body(
      target,
      message,
      directory_ref.current,
    );
    show_browser_notification(title, body, message_id);
  }, [clear_chat_notifications_for_target, clear_room_notifications, record_chat_notification]);

  const { send: ws_send, state: ws_state } = useWebSocket({
    url: ws_url,
    protocols: get_desktop_websocket_protocols(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_websocket_message,
  });

  useEffect(() => {
    if (ws_state !== "connected" || room_ids.length === 0) {
      return;
    }

    for (const room_id of room_ids) {
      const last_seen_room_seq = room_seq_cursor_ref.current[room_id] ?? 0;
      ws_send({
        type: "subscribe_room",
        room_id,
        ...(last_seen_room_seq > 0 ? { last_seen_room_seq } : {}),
      });
    }

    return () => {
      for (const room_id of room_ids) {
        ws_send({
          type: "unsubscribe_room",
          room_id,
        });
      }
    };
    // room_ids_key 是稳定依赖，避免数组引用导致反复重订阅。
  }, [room_ids, room_ids_key, ws_send, ws_state]);
}
