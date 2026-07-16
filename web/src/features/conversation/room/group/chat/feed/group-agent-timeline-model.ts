/**
 * INPUT: Room 根轮次 feed、消息、slot 与权限投影。
 * OUTPUT: 以稳定 agent_round 节点展开、按用户发生/Agent 完成时间排序的 feed。
 * POS: Room feed 专属时间线投影；canonical root 数据仍由 shared timeline 保存给 Thread。
 */
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { SessionRoundIndexItem } from "@/types/conversation/history";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import {
  buildGroupRoundCardModel,
  type GroupRoundAgentCardModel,
} from "../../thread/round-card/group-round-card-model";
import {
  getActiveAgentRoundSortOrder,
  isAgentRoundActive,
} from "../../round/round-agent-model";

interface ProjectGroupAgentTimelineOptions {
  messageGroups: Map<string, Message[]>;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  roundIds: string[];
  roundIndexItems?: SessionRoundIndexItem[];
}

export interface GroupAgentTimelineProjection {
  messageGroups: Map<string, Message[]>;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  rootRoundIds: Map<string, string>;
  roundIds: string[];
}

interface TimelineNode {
  active: boolean;
  activeSortOrder: number;
  kind: "agent" | "root";
  messages: Message[];
  nodeId: string;
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
  rootRoundId: string;
  sourceOrder: number;
  timestamp: number;
}

const ROOM_AGENT_NODE_PREFIX = "room-agent-round:";

/** 每次 agent_round 从 pending 到 terminal 都保持同一个 feed node identity。 */
export function buildGroupAgentTimelineNodeId(
  rootRoundId: string,
  entryId: string,
): string {
  return `${ROOM_AGENT_NODE_PREFIX}${encodeURIComponent(rootRoundId)}:${encodeURIComponent(entryId)}`;
}

export function projectGroupAgentTimeline({
  messageGroups,
  pendingPermissionGroups,
  pendingSlotGroups,
  roundIds,
  roundIndexItems = [],
}: ProjectGroupAgentTimelineOptions): GroupAgentTimelineProjection {
  const anchors = resolveRoundTimelineAnchors(
    roundIds,
    messageGroups,
    roundIndexItems,
  );
  const nodes = roundIds.flatMap((rootRoundId, rootOrder) => (
    buildRootTimelineNodes({
      anchor: anchors[rootOrder] ?? rootOrder,
      messageGroups,
      pendingPermissionGroups,
      pendingSlotGroups,
      rootOrder,
      rootRoundId,
    })
  ));
  nodes.sort(compareTimelineNodes);

  const projectedMessages = new Map<string, Message[]>();
  const projectedPermissions = new Map<string, PendingPermission[]>();
  const projectedSlots = new Map<string, RoomPendingAgentSlotState[]>();
  const rootRoundIds = new Map<string, string>();
  for (const node of nodes) {
    projectedMessages.set(node.nodeId, node.messages);
    projectedPermissions.set(node.nodeId, node.pendingPermissions);
    projectedSlots.set(node.nodeId, node.pendingSlots);
    rootRoundIds.set(node.nodeId, node.rootRoundId);
  }
  return {
    messageGroups: projectedMessages,
    pendingPermissionGroups: projectedPermissions,
    pendingSlotGroups: projectedSlots,
    rootRoundIds,
    roundIds: nodes.map((node) => node.nodeId),
  };
}

function buildRootTimelineNodes({
  anchor,
  messageGroups,
  pendingPermissionGroups,
  pendingSlotGroups,
  rootOrder,
  rootRoundId,
}: {
  anchor: number;
  messageGroups: Map<string, Message[]>;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  rootOrder: number;
  rootRoundId: string;
}): TimelineNode[] {
  const messages = messageGroups.get(rootRoundId) ?? [];
  const pendingPermissions = pendingPermissionGroups.get(rootRoundId) ?? [];
  const pendingSlots = pendingSlotGroups.get(rootRoundId) ?? [];
  if (
    messages.length === 0
    && pendingPermissions.length === 0
    && pendingSlots.length === 0
  ) {
    return [buildRootNode(
      rootRoundId,
      rootOrder,
      anchor,
      messages,
      pendingPermissions,
      pendingSlots,
    )];
  }

  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {},
    messages,
    pendingPermissions,
    pendingSlots,
  });
  if (model.entries.length === 0) {
    return [buildRootNode(
      rootRoundId,
      rootOrder,
      anchor,
      messages,
      pendingPermissions,
      pendingSlots,
    )];
  }

  const assignedAssistantIds = resolveAssignedAssistantIds(
    messages,
    model.entries,
  );
  const assignedGuideIds = new Set(model.entries.flatMap((entry) => (
    entry.guidedUserMessages.map(({ message }) => message.message_id)
  )));
  const assignedPermissionIds = new Set(model.entries.flatMap((entry) => (
    entry.pendingPermissions.map((permission) => permission.request_id)
  )));
  const assignedSlotKeys = new Set(model.entries.map(buildEntrySlotKey));
  const rootMessages = messages.filter((message) => (
    !assignedAssistantIds.has(message.message_id)
    && !assignedGuideIds.has(message.message_id)
  ));
  const rootPermissions = pendingPermissions.filter(
    (permission) => !assignedPermissionIds.has(permission.request_id),
  );
  const rootSlots = pendingSlots.filter(
    (slot) => !assignedSlotKeys.has(buildSlotKey(slot.agent_id, slot.agent_round_id)),
  );
  const nodes: TimelineNode[] = [];
  if (
    rootMessages.length > 0
    || rootPermissions.length > 0
    || rootSlots.length > 0
  ) {
    nodes.push(buildRootNode(
      rootRoundId,
      rootOrder,
      anchor,
      rootMessages,
      rootPermissions,
      rootSlots,
    ));
  }
  nodes.push(...model.entries.map((entry, entryOrder) => ({
    active: isAgentRoundActive(entry.status),
    activeSortOrder: getActiveAgentRoundSortOrder(entry.status),
    kind: "agent" as const,
    messages: [
      ...entry.guidedUserMessages.map(({ message }) => message),
      ...entry.assistant_messages,
    ],
    nodeId: buildGroupAgentTimelineNodeId(rootRoundId, entry.entry_id),
    pendingPermissions: entry.pendingPermissions,
    pendingSlots: entry.pending_slot ? [entry.pending_slot] : [],
    rootRoundId,
    sourceOrder: rootOrder * 10_000 + entryOrder + 1,
    timestamp: entry.timestamp || anchor,
  })));
  return nodes;
}

function buildRootNode(
  rootRoundId: string,
  rootOrder: number,
  anchor: number,
  messages: Message[],
  pendingPermissions: PendingPermission[],
  pendingSlots: RoomPendingAgentSlotState[],
): TimelineNode {
  return {
    active: false,
    activeSortOrder: -1,
    kind: "root",
    messages,
    nodeId: rootRoundId,
    pendingPermissions,
    pendingSlots,
    rootRoundId,
    sourceOrder: rootOrder * 10_000,
    timestamp: earliestMessageTimestamp(messages) ?? anchor,
  };
}

function resolveAssignedAssistantIds(
  messages: Message[],
  entries: GroupRoundAgentCardModel[],
): Set<string> {
  const ids = new Set(entries.flatMap((entry) => (
    entry.assistant_messages.map((message) => message.message_id)
  )));
  const entriesByAgent = new Map<string, GroupRoundAgentCardModel[]>();
  for (const entry of entries) {
    const group = entriesByAgent.get(entry.agent_id) ?? [];
    group.push(entry);
    entriesByAgent.set(entry.agent_id, group);
  }
  // synthetic result 会在 Agent entry 内合并进 canonical assistant；仍需从 root 删除原块。
  for (const message of messages) {
    if (message.role !== "assistant" || !message.agent_id) {
      continue;
    }
    const candidates = entriesByAgent.get(message.agent_id) ?? [];
    const agentRoundId = message.agent_round_id?.trim();
    if (
      candidates.length === 1
      || candidates.some((entry) => (
        agentRoundId && entry.agent_round_id === agentRoundId
      ))
    ) {
      ids.add(message.message_id);
    }
  }
  return ids;
}

function buildEntrySlotKey(entry: GroupRoundAgentCardModel): string {
  return buildSlotKey(entry.agent_id, entry.agent_round_id);
}

function buildSlotKey(
  agentId: string,
  agentRoundId: string | null | undefined,
): string {
  return `${agentId}:${agentRoundId?.trim() ?? ""}`;
}

function compareTimelineNodes(left: TimelineNode, right: TimelineNode): number {
  if (left.active !== right.active) {
    return left.active ? 1 : -1;
  }
  if (left.active && right.active) {
    const statusOrder = left.activeSortOrder - right.activeSortOrder;
    if (statusOrder !== 0) {
      return statusOrder;
    }
  }
  return left.timestamp - right.timestamp
    || (left.kind === right.kind ? 0 : left.kind === "root" ? -1 : 1)
    || left.sourceOrder - right.sourceOrder
    || left.nodeId.localeCompare(right.nodeId);
}

function resolveRoundTimelineAnchors(
  roundIds: string[],
  messageGroups: Map<string, Message[]>,
  roundIndexItems: SessionRoundIndexItem[],
): number[] {
  const indexTimestamps = new Map(roundIndexItems.map((item) => (
    [item.roundId, item.timestamp]
  )));
  const anchors = roundIds.map((roundId) => (
    indexTimestamps.get(roundId)
    ?? earliestMessageTimestamp(messageGroups.get(roundId) ?? [])
    ?? null
  ));
  return anchors.map((anchor, index) => {
    if (anchor !== null) {
      return anchor;
    }
    const previous = findKnownAnchor(anchors, index, -1);
    const next = findKnownAnchor(anchors, index, 1);
    if (previous && next && next.value > previous.value) {
      const span = next.index - previous.index;
      return previous.value
        + ((next.value - previous.value) * (index - previous.index)) / span;
    }
    return previous?.value ?? next?.value ?? index;
  });
}

function findKnownAnchor(
  anchors: Array<number | null>,
  start: number,
  direction: -1 | 1,
): { index: number; value: number } | null {
  for (
    let index = start + direction;
    index >= 0 && index < anchors.length;
    index += direction
  ) {
    const value = anchors[index];
    if (value !== null) {
      return { index, value };
    }
  }
  return null;
}

function earliestMessageTimestamp(messages: Message[]): number | null {
  let earliest: number | null = null;
  for (const message of messages) {
    if (!Number.isFinite(message.timestamp)) {
      continue;
    }
    earliest = earliest === null
      ? message.timestamp
      : Math.min(earliest, message.timestamp);
  }
  return earliest;
}
