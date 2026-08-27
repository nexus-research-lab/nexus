import { isMainAgent } from "@/config/runtime-options";
import { isExternalSessionChannel } from "@/lib/conversation/external-session";
import type { Locale } from "@/shared/i18n/messages";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomMemberSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

import type { RoomActivityStatus } from "../room-activity-resource";

export interface SidebarConversationItem {
  id: string;
  isPinned: boolean;
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
  activityStatus: RoomActivityStatus | null;
  unreadConversationId?: string | null;
  unreadCount?: number;
  unreadTargetKey?: string | null;
  canDelete: boolean;
}

interface ConversationProjectionContext {
  roomActivity: ReadonlyMap<string, RoomActivityStatus>;
  agentById: Map<string, LauncherAgentSummary>;
  latestByRoomId: Map<string, LauncherConversationSummary>;
  locale: Locale;
  untitledRoomLabel: string;
}

export function normalizeSidebarQuery(value: string): string {
  return value.trim().toLowerCase();
}

export function buildConversationItems({
  agents,
  conversations,
  locale = "zh",
  rooms,
  untitledRoomLabel,
  roomActivity = EMPTY_ROOM_ACTIVITY,
}: {
  agents: LauncherAgentSummary[];
  conversations: LauncherConversationSummary[];
  locale?: Locale;
  rooms: LauncherRoomSummary[];
  untitledRoomLabel: string;
  roomActivity?: ReadonlyMap<string, RoomActivityStatus>;
}): SidebarConversationItem[] {
  const context: ConversationProjectionContext = {
    roomActivity,
    agentById: new Map(agents.map((agent) => [agent.id, agent])),
    latestByRoomId: buildLatestConversationByRoomId(conversations),
    locale,
    untitledRoomLabel,
  };
  const items = rooms
    .map((room) => projectConversationItem(room, context))
    .filter((item): item is SidebarConversationItem => item !== null);

  return items.sort((left, right) => {
    if (left.isPinned !== right.isPinned) {
      return left.isPinned ? -1 : 1;
    }
    if (left.lastActivityAt !== right.lastActivityAt) {
      return right.lastActivityAt - left.lastActivityAt;
    }
    return left.title.localeCompare(right.title, resolveIntlLocale(locale));
  });
}

function isMainAgentDmRoom(room: LauncherRoomSummary): boolean {
  return room.room_type === "dm" && Boolean(
    room.dm_target_agent_id && isMainAgent(room.dm_target_agent_id),
  );
}

function projectConversationItem(
  room: LauncherRoomSummary,
  context: ConversationProjectionContext,
): SidebarConversationItem | null {
  const latest = context.latestByRoomId.get(room.id);
  if (!latest) {
    return null;
  }
  const isDm = room.room_type === "dm";
  const isPinned = isMainAgentDmRoom(room);
  const dmAgent = room.dm_target_agent_id
    ? context.agentById.get(room.dm_target_agent_id)
    : undefined;
  const lastActivityAt = toTimestamp(latest.last_activity);
  const title = resolveConversationTitle(room, dmAgent, context.untitledRoomLabel);

  return {
    agentId: room.dm_target_agent_id,
    avatar: room.avatar,
    canDelete: !isPinned,
    conversationId: latest.conversation_id,
    id: room.id,
    isPinned,
    kind: isDm ? "dm" : "room",
    lastActivityAt,
    members: resolveConversationMembers(room, dmAgent),
    messageCount: latest.message_count ?? 0,
    roomId: room.id,
    routeRoomId: room.id,
    activityStatus: context.roomActivity.get(room.id) ?? null,
    sessionKey: latest.session_key,
    summary: latest.last_reply_preview?.trim() ?? "",
    timeLabel: formatSidebarTime(lastActivityAt, context.locale),
    title,
  };
}

function buildLatestConversationByRoomId(
  conversations: LauncherConversationSummary[],
): Map<string, LauncherConversationSummary> {
  const latestByRoomId = new Map<string, LauncherConversationSummary>();
  for (const conversation of conversations) {
    if (!conversation.room_id
      || isExternalSessionChannel(conversation.channel_type, conversation.session_key)) {
      continue;
    }
    const current = latestByRoomId.get(conversation.room_id);
    if (!current || toTimestamp(conversation.last_activity) > toTimestamp(current.last_activity)) {
      latestByRoomId.set(conversation.room_id, conversation);
    }
  }
  return latestByRoomId;
}

function resolveConversationMembers(
  room: LauncherRoomSummary,
  dmAgent?: LauncherAgentSummary,
): LauncherRoomMemberSummary[] {
  if (room.room_type !== "dm") {
    return room.members ?? [];
  }
  return dmAgent
    ? [{ id: dmAgent.id, name: dmAgent.name, avatar: dmAgent.avatar }]
    : room.members ?? [];
}

function resolveConversationTitle(
  room: LauncherRoomSummary,
  dmAgent: LauncherAgentSummary | undefined,
  untitledRoomLabel: string,
): string {
  if (room.room_type === "dm") {
    return dmAgent?.name
      ?? room.members?.[0]?.name
      ?? room.name?.trim()
      ?? "DM";
  }
  return room.name?.trim() || untitledRoomLabel;
}

function toTimestamp(value?: string | null): number {
  if (!value) {
    return 0;
  }
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function formatSidebarTime(timestamp: number, locale: Locale): string {
  if (!timestamp) {
    return "";
  }
  const date = new Date(timestamp);
  const now = new Date();
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const itemDayStart = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const dayDelta = Math.floor((todayStart - itemDayStart) / 86_400_000);
  const intlLocale = resolveIntlLocale(locale);

  if (dayDelta <= 0) {
    return date.toLocaleTimeString(intlLocale, {
      hour: "2-digit",
      hourCycle: "h23",
      minute: "2-digit",
    });
  }
  if (dayDelta === 1) {
    return capitalizeFirst(new Intl.RelativeTimeFormat(intlLocale, {
      numeric: "auto",
    }).format(-1, "day"), intlLocale);
  }
  if (dayDelta < 7) {
    return new Intl.DateTimeFormat(intlLocale, { weekday: "short" })
      .format(date);
  }
  return new Intl.DateTimeFormat(intlLocale, {
    day: "numeric",
    month: "numeric",
  }).format(date);
}

function resolveIntlLocale(locale: Locale): string {
  return locale === "zh" ? "zh-CN" : "en-US";
}

function capitalizeFirst(value: string, locale: string): string {
  return value.length > 0
    ? `${value[0].toLocaleUpperCase(locale)}${value.slice(1)}`
    : value;
}

const EMPTY_ROOM_ACTIVITY: ReadonlyMap<string, RoomActivityStatus> = new Map();
