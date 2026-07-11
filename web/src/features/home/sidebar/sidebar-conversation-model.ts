import { isMainAgent } from "@/config/options";
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
}: {
  agents: LauncherAgentSummary[];
  agentRuntimeStatuses: Record<string, AgentRuntimeStatus>;
  conversations: LauncherConversationSummary[];
  rooms: LauncherRoomSummary[];
  untitledRoomLabel: string;
}): SidebarConversationItem[] {
  const context: ConversationProjectionContext = {
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
  agentRuntimeStatuses,
  dmAgentId,
  isDm,
  latest,
}: {
  agentRuntimeStatuses: Record<string, AgentRuntimeStatus>;
  dmAgentId?: string;
  isDm: boolean;
  latest: LauncherConversationSummary;
}): number {
  if (isDm) {
    return dmAgentId ? (agentRuntimeStatuses[dmAgentId]?.running_task_count ?? 0) : 0;
  }
  return latest.is_active === true || latest.status === "active" ? 1 : 0;
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
