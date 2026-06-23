import { is_main_agent } from "@/config/options";
import { is_external_session_channel } from "@/features/conversation/external-session-labels";
import type { ChatNotificationTargetState } from "@/store/sidebar";
import type { AgentRuntimeStatus } from "@/types/agent/agent";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomMemberSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

import {
  build_chat_notification_target_key,
  is_chat_notification_target_active,
  type ActiveChatNotificationTarget,
} from "./chat-notification-target";

export interface SidebarConversationItem {
  id: string;
  kind: "room" | "dm";
  title: string;
  summary: string;
  time_label: string;
  members: LauncherRoomMemberSummary[];
  avatar?: string | null;
  room_id?: string;
  route_room_id?: string;
  conversation_id?: string;
  session_key?: string;
  agent_id?: string;
  last_activity_at: number;
  message_count: number;
  notification_key?: string | null;
  running_task_count: number;
  unread_conversation_id?: string | null;
  unread_count?: number;
  unread_target_key?: string | null;
  can_delete: boolean;
}

export function normalize_query(value: string): string {
  return value.trim().toLowerCase();
}

export function is_active_sidebar_chat_item(
  item: SidebarConversationItem,
  active_target: ActiveChatNotificationTarget | null,
): boolean {
  return is_chat_notification_target_active(active_target, {
    key: item.notification_key,
    room_id: item.room_id,
  });
}

function to_timestamp(value?: string | null): number {
  if (!value) {
    return 0;
  }
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function format_sidebar_time(timestamp: number): string {
  if (!timestamp) {
    return "";
  }

  const date = new Date(timestamp);
  const now = new Date();
  const today_start = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const item_day_start = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const day_delta = Math.floor((today_start - item_day_start) / 86400000);

  if (day_delta <= 0) {
    return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  }
  if (day_delta === 1) {
    return "昨天";
  }
  if (day_delta < 7) {
    return `周${"日一二三四五六"[date.getDay()]}`;
  }
  return `${date.getMonth() + 1}/${date.getDate()}`;
}

export function is_main_agent_dm_room(room: LauncherRoomSummary): boolean {
  if (room.room_type !== "dm") {
    return false;
  }
  return Boolean(room.dm_target_agent_id && is_main_agent(room.dm_target_agent_id));
}

function build_latest_conversation_by_room_id(
  conversations: LauncherConversationSummary[],
): Map<string, LauncherConversationSummary> {
  const result = new Map<string, LauncherConversationSummary>();
  for (const conversation of conversations) {
    if (
      !conversation.room_id ||
      is_external_session_channel(conversation.channel_type, conversation.session_key)
    ) {
      continue;
    }
    const current = result.get(conversation.room_id);
    if (!current || to_timestamp(conversation.last_activity) > to_timestamp(current.last_activity)) {
      result.set(conversation.room_id, conversation);
    }
  }
  return result;
}

function is_launcher_conversation_active(
  conversation?: LauncherConversationSummary,
): boolean {
  if (!conversation) {
    return false;
  }
  return conversation.is_active === true || conversation.status === "active";
}

function running_task_count_for_sidebar_conversation({
  agent_runtime_statuses,
  dm_agent_id,
  is_dm,
  latest,
}: {
  agent_runtime_statuses: Record<string, AgentRuntimeStatus>;
  dm_agent_id?: string;
  is_dm: boolean;
  latest?: LauncherConversationSummary;
}): number {
  if (is_dm) {
    return dm_agent_id ? (agent_runtime_statuses[dm_agent_id]?.running_task_count ?? 0) : 0;
  }

  return is_launcher_conversation_active(latest) ? 1 : 0;
}

export function build_conversation_items({
  agents,
  agent_runtime_statuses,
  conversations,
  format_running_tasks_summary,
  rooms,
  untitled_room_label,
}: {
  agents: LauncherAgentSummary[];
  agent_runtime_statuses: Record<string, AgentRuntimeStatus>;
  conversations: LauncherConversationSummary[];
  format_running_tasks_summary: (count: number) => string;
  rooms: LauncherRoomSummary[];
  untitled_room_label: string;
}): SidebarConversationItem[] {
  const agent_by_id = new Map(agents.map((agent) => [agent.id, agent]));
  const latest_by_room_id = build_latest_conversation_by_room_id(conversations);
  const items: SidebarConversationItem[] = [];

  for (const room of rooms) {
    if (is_main_agent_dm_room(room)) {
      continue;
    }
    const latest_room = latest_by_room_id.get(room.id);
    if (!latest_room) {
      continue;
    }
    const last_activity_at = to_timestamp(latest_room.last_activity);
    const is_dm = room.room_type === "dm";
    const dm_agent = room.dm_target_agent_id ? agent_by_id.get(room.dm_target_agent_id) : undefined;
    const members = is_dm
      ? dm_agent ? [{ id: dm_agent.id, name: dm_agent.name, avatar: dm_agent.avatar }] : []
      : room.members ?? [];
    const running_task_count = running_task_count_for_sidebar_conversation({
      agent_runtime_statuses,
      dm_agent_id: room.dm_target_agent_id,
      is_dm,
      latest: latest_room,
    });
    const title = is_dm
      ? dm_agent?.name ?? room.name?.trim() ?? "DM"
      : room.name?.trim() || untitled_room_label;

    items.push({
      id: room.id,
      kind: is_dm ? "dm" : "room",
      title,
      summary: running_task_count > 0
        ? format_running_tasks_summary(running_task_count)
        : latest_room.title.trim(),
      time_label: format_sidebar_time(last_activity_at),
      members,
      avatar: room.avatar,
      room_id: room.id,
      route_room_id: room.id,
      conversation_id: latest_room.conversation_id,
      session_key: latest_room.session_key,
      agent_id: room.dm_target_agent_id,
      last_activity_at,
      message_count: latest_room.message_count ?? 0,
      running_task_count,
      can_delete: true,
    });
  }

  return items.sort((left, right) => {
    if (left.last_activity_at !== right.last_activity_at) {
      return right.last_activity_at - left.last_activity_at;
    }
    return left.title.localeCompare(right.title, "zh-CN");
  });
}

export function get_sidebar_item_unread_state({
  chat_unread_counts,
  chat_unread_targets,
  chat_unread_timestamps,
  notification_key,
  room_id,
  session_key,
}: {
  chat_unread_counts: Record<string, number>;
  chat_unread_targets: Record<string, ChatNotificationTargetState>;
  chat_unread_timestamps: Record<string, number>;
  notification_key?: string | null;
  room_id?: string | null;
  session_key?: string | null;
}): {
  unread_conversation_id: string | null;
  unread_count: number;
  unread_target_key: string | null;
} {
  const normalized_room_id = room_id?.trim();
  let unread_count = 0;
  let unread_target: ChatNotificationTargetState | null = null;
  let unread_target_timestamp = -1;
  const counted_keys = new Set<string>();

  if (normalized_room_id) {
    for (const [key, target] of Object.entries(chat_unread_targets)) {
      if (target.room_id !== normalized_room_id) {
        continue;
      }
      const count = chat_unread_counts[key] ?? 0;
      if (count <= 0) {
        continue;
      }
      counted_keys.add(key);
      unread_count += count;
      const timestamp = chat_unread_timestamps[key] ?? 0;
      if (timestamp >= unread_target_timestamp) {
        unread_target = target;
        unread_target_timestamp = timestamp;
      }
    }

    const room_key = `room:${normalized_room_id}`;
    const room_conversation_key_prefix = `${room_key}:conversation:`;
    for (const [key, count] of Object.entries(chat_unread_counts)) {
      if (counted_keys.has(key) || count <= 0) {
        continue;
      }
      if (key !== room_key && !key.startsWith(room_conversation_key_prefix)) {
        continue;
      }
      unread_count += count;
      const timestamp = chat_unread_timestamps[key] ?? 0;
      if (timestamp >= unread_target_timestamp) {
        unread_target = chat_unread_targets[key] ?? {
          conversation_id: key.startsWith(room_conversation_key_prefix)
            ? key.slice(room_conversation_key_prefix.length)
            : null,
          key,
          room_id: normalized_room_id,
        };
        unread_target_timestamp = timestamp;
      }
    }
  } else if (notification_key) {
    unread_count = chat_unread_counts[notification_key] ?? 0;
    if (unread_count > 0) {
      unread_target = chat_unread_targets[notification_key] ?? {
        key: notification_key,
        room_id,
      };
    }
  }

  const session_notification_key = build_chat_notification_target_key({ session_key });
  if (session_notification_key && !counted_keys.has(session_notification_key)) {
    const session_unread_count = chat_unread_counts[session_notification_key] ?? 0;
    if (session_unread_count > 0) {
      unread_count += session_unread_count;
      const timestamp = chat_unread_timestamps[session_notification_key] ?? 0;
      if (timestamp >= unread_target_timestamp) {
        unread_target = chat_unread_targets[session_notification_key] ?? {
          conversation_id: null,
          key: session_notification_key,
          room_id,
          session_key,
        };
        unread_target_timestamp = timestamp;
      }
    }
  }

  return {
    unread_conversation_id: unread_target?.conversation_id ?? null,
    unread_count,
    unread_target_key: unread_target?.key ?? null,
  };
}

export function build_sidebar_item_notification_key(item: SidebarConversationItem): string | null {
  return build_chat_notification_target_key({
    conversation_id: item.conversation_id,
    room_id: item.room_id,
    session_key: item.session_key,
  });
}
