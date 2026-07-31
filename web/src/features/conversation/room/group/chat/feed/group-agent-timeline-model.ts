/**
 * INPUT: Room 根轮次 feed、消息、slot、权限与 execution 首见锚点投影。
 * OUTPUT: 以稳定 agent_round 节点展开、按 parent slot 精确消费 legacy terminal 且从启动到完成保持原位的 feed。
 * POS: Room feed 专属时间线投影；canonical root 数据仍由 shared timeline 保存给 Thread。
 */
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import {
  filterPendingPermissionsForTerminalRoomExecutions,
} from "@/lib/conversation/pending-permission-match";

import {
  buildGroupRoundCardModel,
  type GroupRoundAgentCardModel,
} from "../../thread/round-card/group-round-card-model";
interface ProjectGroupAgentTimelineOptions {
  messageGroups: Map<string, Message[]>;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  roomAgentExecutionStateGroups?: Map<string, RoomAgentExecutionState[]>;
  roundIds: string[];
}

export interface GroupAgentTimelineProjection {
  messageGroups: Map<string, Message[]>;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  roomAgentExecutionStateGroups: Map<string, RoomAgentExecutionState[]>;
  rootRoundIds: Map<string, string>;
  roundIds: string[];
}

interface TimelineNode {
  messages: Message[];
  nodeId: string;
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
  roomAgentExecutionStates: RoomAgentExecutionState[];
  rootRoundId: string;
}

const ROOM_AGENT_NODE_PREFIX = "room-agent-round:";
const ROOM_THREAD_ONLY_SYSTEM_SUBTYPES = new Set([
  "memory_recalled",
  "memory_saved",
]);

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
  roomAgentExecutionStateGroups = new Map<string, RoomAgentExecutionState[]>(),
  roundIds,
}: ProjectGroupAgentTimelineOptions): GroupAgentTimelineProjection {
  const nodes = roundIds.flatMap((rootRoundId) => (
    buildRootTimelineNodes({
      messageGroups,
      pendingPermissionGroups,
      pendingSlotGroups,
      roomAgentExecutionStateGroups,
      rootRoundId,
    })
  ));

  const projectedMessages = new Map<string, Message[]>();
  const projectedPermissions = new Map<string, PendingPermission[]>();
  const projectedSlots = new Map<string, RoomPendingAgentSlotState[]>();
  const projectedExecutionStates = new Map<string, RoomAgentExecutionState[]>();
  const rootRoundIds = new Map<string, string>();
  for (const node of nodes) {
    projectedMessages.set(node.nodeId, node.messages);
    projectedPermissions.set(node.nodeId, node.pendingPermissions);
    projectedSlots.set(node.nodeId, node.pendingSlots);
    projectedExecutionStates.set(node.nodeId, node.roomAgentExecutionStates);
    rootRoundIds.set(node.nodeId, node.rootRoundId);
  }
  return {
    messageGroups: projectedMessages,
    pendingPermissionGroups: projectedPermissions,
    pendingSlotGroups: projectedSlots,
    roomAgentExecutionStateGroups: projectedExecutionStates,
    rootRoundIds,
    roundIds: nodes.map((node) => node.nodeId),
  };
}

function buildRootTimelineNodes({
  messageGroups,
  pendingPermissionGroups,
  pendingSlotGroups,
  roomAgentExecutionStateGroups,
  rootRoundId,
}: {
  messageGroups: Map<string, Message[]>;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  roomAgentExecutionStateGroups: Map<string, RoomAgentExecutionState[]>;
  rootRoundId: string;
}): TimelineNode[] {
  const messages = (messageGroups.get(rootRoundId) ?? [])
    .filter(isRoomMainFeedMessage);
  const pendingPermissions =
    filterPendingPermissionsForTerminalRoomExecutions(
    pendingPermissionGroups.get(rootRoundId) ?? [],
    roomAgentExecutionStateGroups.get(rootRoundId) ?? [],
  );
  const pendingSlots = pendingSlotGroups.get(rootRoundId) ?? [];
  const roomAgentExecutionStates =
    roomAgentExecutionStateGroups.get(rootRoundId) ?? [];
  if (
    messages.length === 0
    && pendingPermissions.length === 0
    && pendingSlots.length === 0
    && roomAgentExecutionStates.length === 0
  ) {
    return [buildRootNode(
      rootRoundId,
      messages,
      pendingPermissions,
      pendingSlots,
      roomAgentExecutionStates,
    )];
  }

  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {},
    messages,
    pendingPermissions,
    pendingSlots,
    executionStates: roomAgentExecutionStates,
  });
  if (model.entries.length === 0) {
    return [buildRootNode(
      rootRoundId,
      messages,
      pendingPermissions,
      pendingSlots,
      roomAgentExecutionStates,
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
  const assignedExecutionKeys = new Set(model.entries.map(buildEntrySlotKey));
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
  const rootExecutionStates = roomAgentExecutionStates.filter(
    (state) => !assignedExecutionKeys.has(
      buildSlotKey(state.agent_id, state.agent_round_id),
    ),
  );
  const nodes: TimelineNode[] = [];
  if (
    rootMessages.length > 0
    || rootPermissions.length > 0
    || rootSlots.length > 0
    || rootExecutionStates.length > 0
  ) {
    nodes.push(buildRootNode(
      rootRoundId,
      rootMessages,
      rootPermissions,
      rootSlots,
      rootExecutionStates,
    ));
  }
  nodes.push(...model.entries.map((entry) => ({
    messages: [
      ...entry.guidedUserMessages.map(({ message }) => message),
      ...entry.assistant_messages,
    ],
    nodeId: buildGroupAgentTimelineNodeId(rootRoundId, entry.entry_id),
    pendingPermissions: entry.pendingPermissions,
    pendingSlots: entry.pending_slot ? [entry.pending_slot] : [],
    roomAgentExecutionStates: roomAgentExecutionStates.filter((state) => (
      buildSlotKey(state.agent_id, state.agent_round_id)
        === buildEntrySlotKey(entry)
    )),
    rootRoundId,
  })));
  return nodes;
}

/** 记忆加载属于单个 Agent 的执行上下文，只在对应 Thread 展示。 */
function isRoomMainFeedMessage(message: Message): boolean {
  return message.role !== "system"
    || !ROOM_THREAD_ONLY_SYSTEM_SUBTYPES.has(
      message.metadata?.subtype ?? "",
    );
}

function buildRootNode(
  rootRoundId: string,
  messages: Message[],
  pendingPermissions: PendingPermission[],
  pendingSlots: RoomPendingAgentSlotState[],
  roomAgentExecutionStates: RoomAgentExecutionState[],
): TimelineNode {
  return {
    messages,
    nodeId: resolveStableRootNodeId(rootRoundId, messages),
    pendingPermissions,
    pendingSlots,
    roomAgentExecutionStates,
    rootRoundId,
  };
}

/**
 * optimistic user 与 durable echo 的 canonical round_id 不同；服务端回传的
 * client_message_id 继续作为 React/virtual feed 身份，语义 round 仍由映射保存。
 */
function resolveStableRootNodeId(
  rootRoundId: string,
  messages: readonly Message[],
): string {
  for (const message of messages) {
    if (message.role !== "user") {
      continue;
    }
    const clientMessageId = message.client_message_id?.trim();
    if (clientMessageId) {
      return clientMessageId;
    }
  }
  return rootRoundId;
}

function resolveAssignedAssistantIds(
  messages: Message[],
  entries: GroupRoundAgentCardModel[],
): Set<string> {
  const ids = new Set(entries.flatMap((entry) => (
    entry.assistant_messages.map((message) => message.message_id)
  )));
  // synthetic result 会在 Agent entry 内合并进 canonical assistant；仍需从 root 删除原块。
  for (const message of messages) {
    if (message.role !== "assistant" || !message.agent_id) {
      continue;
    }
    const agentRoundId = message.agent_round_id?.trim();
    const parentId = message.parent_id?.trim();
    const assigned = entries.some((entry) => {
      if (entry.agent_id !== message.agent_id) {
        return false;
      }
      if (agentRoundId) {
        return entry.agent_round_id === agentRoundId;
      }
      return Boolean(
        parentId
        && entry.pending_slot?.msg_id.trim() === parentId,
      );
    });
    if (assigned) {
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
