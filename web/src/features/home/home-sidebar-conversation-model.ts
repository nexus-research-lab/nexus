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
  activeTarget: ActiveChatNotificationTarget | null,
): boolean {
  return is_chat_notification_target_active(activeTarget, {
    key: item.notification_key,
    room_id: item.room_id,
  });
}

function toTimestamp(value?: string | null): number {
  if (!value) {
    return 0;
  }
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function formatSidebarTime(timestamp: number): string {
  if (!timestamp) {
    return "";
  }

  const date = new Date(timestamp);
  const now = new Date();
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const itemDayStart = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const dayDelta = Math.floor((todayStart - itemDayStart) / 86400000);

  if (dayDelta <= 0) {
    return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  }
  if (dayDelta === 1) {
    return "昨天";
  }
  if (dayDelta < 7) {
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

function buildLatestConversationByRoomId(
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
    if (!current || toTimestamp(conversation.last_activity) > toTimestamp(current.last_activity)) {
      result.set(conversation.room_id, conversation);
    }
  }
  return result;
}

function isLauncherConversationActive(
  conversation?: LauncherConversationSummary,
): boolean {
  if (!conversation) {
    return false;
  }
  return conversation.is_active === true || conversation.status === "active";
}

function runningTaskCountForSidebarConversation({
  agent_runtime_statuses: agentRuntimeStatuses,
  dm_agent_id: dmAgentId,
  is_dm: isDm,
  latest,
}: {
  agent_runtime_statuses: Record<string, AgentRuntimeStatus>;
  dm_agent_id?: string;
  is_dm: boolean;
  latest?: LauncherConversationSummary;
}): number {
  if (isDm) {
    return dmAgentId ? (agentRuntimeStatuses[dmAgentId]?.running_task_count ?? 0) : 0;
  }

  return isLauncherConversationActive(latest) ? 1 : 0;
}

export function build_conversation_items({
  agents,
  agent_runtime_statuses: agentRuntimeStatuses,
  conversations,
  format_running_tasks_summary: formatRunningTasksSummary,
  rooms,
  untitled_room_label: untitledRoomLabel,
}: {
  agents: LauncherAgentSummary[];
  agent_runtime_statuses: Record<string, AgentRuntimeStatus>;
  conversations: LauncherConversationSummary[];
  format_running_tasks_summary: (count: number) => string;
  rooms: LauncherRoomSummary[];
  untitled_room_label: string;
}): SidebarConversationItem[] {
  const agentById = new Map(agents.map((agent) => [agent.id, agent]));
  const latestByRoomId = buildLatestConversationByRoomId(conversations);
  const items: SidebarConversationItem[] = [];

  for (const room of rooms) {
    if (is_main_agent_dm_room(room)) {
      continue;
    }
    const latestRoom = latestByRoomId.get(room.id);
    if (!latestRoom) {
      continue;
    }
    const lastActivityAt = toTimestamp(latestRoom.last_activity);
    const isDm = room.room_type === "dm";
    const dmAgent = room.dm_target_agent_id ? agentById.get(room.dm_target_agent_id) : undefined;
    const members = isDm
      ? dmAgent ? [{ id: dmAgent.id, name: dmAgent.name, avatar: dmAgent.avatar }] : []
      : room.members ?? [];
    const runningTaskCount = runningTaskCountForSidebarConversation({
      agent_runtime_statuses: agentRuntimeStatuses,
      dm_agent_id: room.dm_target_agent_id,
      is_dm: isDm,
      latest: latestRoom,
    });
    const title = isDm
      ? dmAgent?.name ?? room.name?.trim() ?? "DM"
      : room.name?.trim() || untitledRoomLabel;

    items.push({
      id: room.id,
      kind: isDm ? "dm" : "room",
      title,
      summary: runningTaskCount > 0
        ? formatRunningTasksSummary(runningTaskCount)
        : latestRoom.title.trim(),
      time_label: formatSidebarTime(lastActivityAt),
      members,
      avatar: room.avatar,
      room_id: room.id,
      route_room_id: room.id,
      conversation_id: latestRoom.conversation_id,
      session_key: latestRoom.session_key,
      agent_id: room.dm_target_agent_id,
      last_activity_at: lastActivityAt,
      message_count: latestRoom.message_count ?? 0,
      running_task_count: runningTaskCount,
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
  chat_unread_counts: chatUnreadCounts,
  chat_unread_targets: chatUnreadTargets,
  chat_unread_timestamps: chatUnreadTimestamps,
  notification_key: notificationKey,
  room_id: roomId,
  session_key: sessionKey,
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
  const normalizedRoomId = roomId?.trim();
  let unreadCount = 0;
  let unreadTarget: ChatNotificationTargetState | null = null;
  let unreadTargetTimestamp = -1;
  const countedKeys = new Set<string>();

  if (normalizedRoomId) {
    for (const [key, target] of Object.entries(chatUnreadTargets)) {
      if (target.room_id !== normalizedRoomId) {
        continue;
      }
      const count = chatUnreadCounts[key] ?? 0;
      if (count <= 0) {
        continue;
      }
      countedKeys.add(key);
      unreadCount += count;
      const timestamp = chatUnreadTimestamps[key] ?? 0;
      if (timestamp >= unreadTargetTimestamp) {
        unreadTarget = target;
        unreadTargetTimestamp = timestamp;
      }
    }

    const roomKey = `room:${normalizedRoomId}`;
    const roomConversationKeyPrefix = `${roomKey}:conversation:`;
    for (const [key, count] of Object.entries(chatUnreadCounts)) {
      if (countedKeys.has(key) || count <= 0) {
        continue;
      }
      if (key !== roomKey && !key.startsWith(roomConversationKeyPrefix)) {
        continue;
      }
      unreadCount += count;
      const timestamp = chatUnreadTimestamps[key] ?? 0;
      if (timestamp >= unreadTargetTimestamp) {
        unreadTarget = chatUnreadTargets[key] ?? {
          conversation_id: key.startsWith(roomConversationKeyPrefix)
            ? key.slice(roomConversationKeyPrefix.length)
            : null,
          key,
          room_id: normalizedRoomId,
        };
        unreadTargetTimestamp = timestamp;
      }
    }
  } else if (notificationKey) {
    unreadCount = chatUnreadCounts[notificationKey] ?? 0;
    if (unreadCount > 0) {
      unreadTarget = chatUnreadTargets[notificationKey] ?? {
        key: notificationKey,
        room_id: roomId,
      };
    }
  }

  const sessionNotificationKey = build_chat_notification_target_key({ session_key: sessionKey });
  if (sessionNotificationKey && !countedKeys.has(sessionNotificationKey)) {
    const sessionUnreadCount = chatUnreadCounts[sessionNotificationKey] ?? 0;
    if (sessionUnreadCount > 0) {
      unreadCount += sessionUnreadCount;
      const timestamp = chatUnreadTimestamps[sessionNotificationKey] ?? 0;
      if (timestamp >= unreadTargetTimestamp) {
        unreadTarget = chatUnreadTargets[sessionNotificationKey] ?? {
          conversation_id: null,
          key: sessionNotificationKey,
          room_id: roomId,
          session_key: sessionKey,
        };
        unreadTargetTimestamp = timestamp;
      }
    }
  }

  return {
    unread_conversation_id: unreadTarget?.conversation_id ?? null,
    unread_count: unreadCount,
    unread_target_key: unreadTarget?.key ?? null,
  };
}

export function build_sidebar_item_notification_key(item: SidebarConversationItem): string | null {
  return build_chat_notification_target_key({
    conversation_id: item.conversation_id,
    room_id: item.room_id,
    session_key: item.session_key,
  });
}
