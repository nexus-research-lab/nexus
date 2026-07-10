import { isMainAgent } from "@/config/options";
import { isExternalSessionChannel } from "@/features/conversation/external-session-labels";
import type { ChatNotificationTargetState } from "@/store/sidebar";
import type { AgentRuntimeStatus } from "@/types/agent/agent";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomMemberSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

import {
  buildChatNotificationTargetKey,
  isChatNotificationTargetActive,
  type ActiveChatNotificationTarget,
} from "./chat-notification-target";

export interface SidebarConversationItem {
  id: string;
  kind: "room" | "dm";
  title: string;
  summary: string;
  timeLabel: string;
  members: LauncherRoomMemberSummary[];
  avatar?: string | null;
  roomId?: string;
  routeRoomId?: string;
  conversationId?: string;
  sessionKey?: string;
  agentId?: string;
  lastActivityAt: number;
  messageCount: number;
  notificationKey?: string | null;
  runningTaskCount: number;
  unreadConversationId?: string | null;
  unreadCount?: number;
  unreadTargetKey?: string | null;
  canDelete: boolean;
}

export function normalizeQuery(value: string): string {
  return value.trim().toLowerCase();
}

export function isActiveSidebarChatItem(
  item: SidebarConversationItem,
  activeTarget: ActiveChatNotificationTarget | null,
): boolean {
  return isChatNotificationTargetActive(activeTarget, {
    key: item.notificationKey,
    room_id: item.roomId,
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

export function isMainAgentDmRoom(room: LauncherRoomSummary): boolean {
  if (room.room_type !== "dm") {
    return false;
  }
  return Boolean(room.dm_target_agent_id && isMainAgent(room.dm_target_agent_id));
}

function buildLatestConversationByRoomId(
  conversations: LauncherConversationSummary[],
): Map<string, LauncherConversationSummary> {
  const result = new Map<string, LauncherConversationSummary>();
  for (const conversation of conversations) {
    if (
      !conversation.room_id ||
      isExternalSessionChannel(conversation.channel_type, conversation.session_key)
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
  agentRuntimeStatuses,
  dmAgentId,
  isDm,
  latest,
}: {
  agentRuntimeStatuses: Record<string, AgentRuntimeStatus>;
  dmAgentId?: string;
  isDm: boolean;
  latest?: LauncherConversationSummary;
}): number {
  if (isDm) {
    return dmAgentId ? (agentRuntimeStatuses[dmAgentId]?.running_task_count ?? 0) : 0;
  }

  return isLauncherConversationActive(latest) ? 1 : 0;
}

export function buildConversationItems({
  agents,
  agentRuntimeStatuses,
  conversations,
  rooms,
  untitledRoomLabel,
}: {
  agents: LauncherAgentSummary[];
  agentRuntimeStatuses: Record<string, AgentRuntimeStatus>;
  conversations: LauncherConversationSummary[];
  rooms: LauncherRoomSummary[];
  untitledRoomLabel: string;
}): SidebarConversationItem[] {
  const agentById = new Map(agents.map((agent) => [agent.id, agent]));
  const latestByRoomId = buildLatestConversationByRoomId(conversations);
  const items: SidebarConversationItem[] = [];

  for (const room of rooms) {
    if (isMainAgentDmRoom(room)) {
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
      agentRuntimeStatuses,
      dmAgentId: room.dm_target_agent_id,
      isDm,
      latest: latestRoom,
    });
    const title = isDm
      ? dmAgent?.name ?? room.name?.trim() ?? "DM"
      : room.name?.trim() || untitledRoomLabel;

    items.push({
      id: room.id,
      kind: isDm ? "dm" : "room",
      title,
      summary: latestRoom.last_reply_preview?.trim() ?? "",
      timeLabel: formatSidebarTime(lastActivityAt),
      members,
      avatar: room.avatar,
      roomId: room.id,
      routeRoomId: room.id,
      conversationId: latestRoom.conversation_id,
      sessionKey: latestRoom.session_key,
      agentId: room.dm_target_agent_id,
      lastActivityAt,
      messageCount: latestRoom.message_count ?? 0,
      runningTaskCount,
      canDelete: true,
    });
  }

  return items.sort((left, right) => {
    if (left.lastActivityAt !== right.lastActivityAt) {
      return right.lastActivityAt - left.lastActivityAt;
    }
    return left.title.localeCompare(right.title, "zh-CN");
  });
}

export function getSidebarItemUnreadState({
  chatUnreadCounts,
  chatUnreadTargets,
  chatUnreadTimestamps,
  notificationKey,
  roomId,
  sessionKey,
}: {
  chatUnreadCounts: Record<string, number>;
  chatUnreadTargets: Record<string, ChatNotificationTargetState>;
  chatUnreadTimestamps: Record<string, number>;
  notificationKey?: string | null;
  roomId?: string | null;
  sessionKey?: string | null;
}): {
  unreadConversationId: string | null;
  unreadCount: number;
  unreadTargetKey: string | null;
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

  const sessionNotificationKey = buildChatNotificationTargetKey({ session_key: sessionKey });
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
    unreadConversationId: unreadTarget?.conversation_id ?? null,
    unreadCount,
    unreadTargetKey: unreadTarget?.key ?? null,
  };
}

export function buildSidebarItemNotificationKey(item: SidebarConversationItem): string | null {
  return buildChatNotificationTargetKey({
    conversation_id: item.conversationId,
    room_id: item.roomId,
    session_key: item.sessionKey,
  });
}
