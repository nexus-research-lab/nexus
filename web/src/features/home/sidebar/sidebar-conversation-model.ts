import { isMainAgent } from "@/config/runtime-options";
import { isExternalSessionChannel } from "@/lib/conversation/external-session";
import type { AgentRuntimeStatus } from "@/types/agent/agent";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomMemberSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

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

interface ConversationProjectionContext {
  activeRoomAgentIds: ReadonlySet<string>;
  activeRoomIds: ReadonlySet<string>;
  agentById: Map<string, LauncherAgentSummary>;
  agentRuntimeStatuses: Record<string, AgentRuntimeStatus>;
  latestByRoomId: Map<string, LauncherConversationSummary>;
  untitledRoomLabel: string;
}

export function normalizeSidebarQuery(value: string): string {
  return value.trim().toLowerCase();
}

export function buildConversationItems({
  agents,
  agentRuntimeStatuses,
  conversations,
  rooms,
  untitledRoomLabel,
  activeRoomIds = EMPTY_ACTIVE_ROOM_IDS,
}: {
  agents: LauncherAgentSummary[];
  agentRuntimeStatuses: Record<string, AgentRuntimeStatus>;
  conversations: LauncherConversationSummary[];
  rooms: LauncherRoomSummary[];
  untitledRoomLabel: string;
  activeRoomIds?: ReadonlySet<string>;
}): SidebarConversationItem[] {
  const resolvedActiveRoomIds = buildActiveRoomIds({
    activeRoomIds,
    conversations,
    rooms,
  });
  const context: ConversationProjectionContext = {
    activeRoomAgentIds: buildActiveRoomAgentIds(rooms, resolvedActiveRoomIds),
    activeRoomIds: resolvedActiveRoomIds,
    agentById: new Map(agents.map((agent) => [agent.id, agent])),
    agentRuntimeStatuses,
    latestByRoomId: buildLatestConversationByRoomId(conversations),
    untitledRoomLabel,
  };
  const items = rooms
    .map((room) => projectConversationItem(room, context))
    .filter((item): item is SidebarConversationItem => item !== null);

  return items.sort((left, right) => {
    if (left.lastActivityAt !== right.lastActivityAt) {
      return right.lastActivityAt - left.lastActivityAt;
    }
    return left.title.localeCompare(right.title, "zh-CN");
  });
}

export function isMainAgentDmRoom(room: LauncherRoomSummary): boolean {
  return room.room_type === "dm" && Boolean(
    room.dm_target_agent_id && isMainAgent(room.dm_target_agent_id),
  );
}

function projectConversationItem(
  room: LauncherRoomSummary,
  context: ConversationProjectionContext,
): SidebarConversationItem | null {
  if (isMainAgentDmRoom(room)) {
    return null;
  }
  const latest = context.latestByRoomId.get(room.id);
  if (!latest) {
    return null;
  }

  const isDm = room.room_type === "dm";
  const dmAgent = room.dm_target_agent_id
    ? context.agentById.get(room.dm_target_agent_id)
    : undefined;
  const lastActivityAt = toTimestamp(latest.last_activity);

  return {
    agentId: room.dm_target_agent_id,
    avatar: room.avatar,
    canDelete: true,
    conversationId: latest.conversation_id,
    id: room.id,
    kind: isDm ? "dm" : "room",
    lastActivityAt,
    members: resolveConversationMembers(room, dmAgent),
    messageCount: latest.message_count ?? 0,
    roomId: room.id,
    routeRoomId: room.id,
    runningTaskCount: resolveRunningTaskCount({
      activeRoom: context.activeRoomIds.has(room.id),
      activeRoomAgentIds: context.activeRoomAgentIds,
      agentRuntimeStatuses: context.agentRuntimeStatuses,
      dmAgentId: room.dm_target_agent_id,
      isDm,
      latest,
    }),
    sessionKey: latest.session_key,
    summary: latest.last_reply_preview?.trim() ?? "",
    timeLabel: formatSidebarTime(lastActivityAt),
    title: resolveConversationTitle(room, dmAgent, context.untitledRoomLabel),
  };
}

function buildLatestConversationByRoomId(
  conversations: LauncherConversationSummary[],
): Map<string, LauncherConversationSummary> {
  const latestByRoomId = new Map<string, LauncherConversationSummary>();
  for (const conversation of conversations) {
    if (
      !conversation.room_id ||
      isExternalSessionChannel(conversation.channel_type, conversation.session_key)
    ) {
      continue;
    }
    const current = latestByRoomId.get(conversation.room_id);
    if (!current) {
      latestByRoomId.set(conversation.room_id, conversation);
      continue;
    }
    const candidate = toTimestamp(conversation.last_activity) > toTimestamp(current.last_activity)
      ? conversation
      : current;
    // Room 有多个成员 session，任一成员运行都应让 Room 行保持激活。
    if (isConversationActive(conversation) || isConversationActive(current)) {
      latestByRoomId.set(conversation.room_id, {
        ...candidate,
        is_active: true,
        status: "active",
      });
    } else {
      latestByRoomId.set(conversation.room_id, candidate);
    }
  }
  return latestByRoomId;
}

function buildActiveRoomIds({
  activeRoomIds,
  conversations,
  rooms,
}: {
  activeRoomIds: ReadonlySet<string>;
  conversations: LauncherConversationSummary[];
  rooms: LauncherRoomSummary[];
}): ReadonlySet<string> {
  const roomTypeById = new Map(rooms.map((room) => [room.id, room.room_type]));
  const resolved = new Set(activeRoomIds);
  for (const conversation of conversations) {
    if (
      conversation.room_id
      && roomTypeById.get(conversation.room_id) === "room"
      && isConversationActive(conversation)
    ) {
      resolved.add(conversation.room_id);
    }
  }
  return resolved;
}

function buildActiveRoomAgentIds(
  rooms: LauncherRoomSummary[],
  activeRoomIds: ReadonlySet<string>,
): ReadonlySet<string> {
  const agentIds = new Set<string>();
  for (const room of rooms) {
    if (room.room_type !== "room" || !activeRoomIds.has(room.id)) {
      continue;
    }
    for (const member of room.members ?? []) {
      if (member.id) {
        agentIds.add(member.id);
      }
    }
  }
  return agentIds;
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
    : [];
}

function resolveConversationTitle(
  room: LauncherRoomSummary,
  dmAgent: LauncherAgentSummary | undefined,
  untitledRoomLabel: string,
): string {
  if (room.room_type === "dm") {
    return dmAgent?.name ?? room.name?.trim() ?? "DM";
  }
  return room.name?.trim() || untitledRoomLabel;
}

function resolveRunningTaskCount({
  activeRoom,
  activeRoomAgentIds,
  agentRuntimeStatuses,
  dmAgentId,
  isDm,
  latest,
}: {
  activeRoom: boolean;
  activeRoomAgentIds: ReadonlySet<string>;
  agentRuntimeStatuses: Record<string, AgentRuntimeStatus>;
  dmAgentId?: string;
  isDm: boolean;
  latest: LauncherConversationSummary;
}): number {
  if (isDm) {
    // 群聊执行态归属 Room，不能在成员的 DM 行再次显示。
    if (dmAgentId && activeRoomAgentIds.has(dmAgentId)) {
      return 0;
    }
    if (activeRoom) {
      return 1;
    }
    const runtimeCount = dmAgentId
      ? (agentRuntimeStatuses[dmAgentId]?.running_task_count ?? 0)
      : 0;
    return runtimeCount > 0 || isConversationActive(latest) ? 1 : 0;
  }
  return activeRoom || isConversationActive(latest) ? 1 : 0;
}

function isConversationActive(conversation: LauncherConversationSummary): boolean {
  return conversation.is_active === true || conversation.status === "active";
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
  const dayDelta = Math.floor((todayStart - itemDayStart) / 86_400_000);

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

const EMPTY_ACTIVE_ROOM_IDS: ReadonlySet<string> = new Set();
