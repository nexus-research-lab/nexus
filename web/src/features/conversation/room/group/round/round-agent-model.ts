/**
 * INPUT: Room 根轮次消息、agent slot、人工介入请求与当前 Session 的 execution 首见锚点。
 * OUTPUT: 证据按 agent_round_id 聚合且顺序不变；历史消息不建活动态，live slot/interaction/lifecycle 仍保留 shell。
 * POS: Room feed 与 thread 共用的 Agent 执行轮次投影。
 */
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
  ResultSummary,
} from "@/types/conversation/message/entity";
import {
  extractTextFromContentBlocks,
} from "@/features/conversation/shared/message/message-content-model";
import { isVisibleRoomAgentExecutionState } from "@/hooks/agent/runtime/model/room-agent-execution-state";

export type AgentRoundStatus = AssistantMessageStatus;

export interface RoomAgentRoundEntry {
  entry_id: string;
  agent_id: string;
  agent_round_id: string | null;
  assistant_messages: AssistantMessage[];
  result_summary?: ResultSummary;
  pending_slot?: RoomPendingAgentSlotState;
  status: AgentRoundStatus;
  timestamp: number;
  display_order: number;
}

interface RoomAgentRoundIndex {
  entryIds: Set<string>;
  executionStates: Map<string, RoomAgentExecutionState>;
  executionStateOrders: Map<string, number>;
  messageGroups: Map<string, AssistantMessage[]>;
  messageOrders: Map<string, number>;
  permissionIdentities: Map<string, PermissionEntryIdentity>;
  permissionOrders: Map<string, number>;
  pendingSlots: Map<string, RoomPendingAgentSlotState>;
  pendingSlotOrders: Map<string, number>;
}

interface PermissionEntryIdentity {
  agentId: string;
  agentRoundId: string;
}

const MESSAGE_STATUS_PRIORITY: readonly AgentRoundStatus[] = [
  "streaming",
  "pending",
  "error",
  "cancelled",
  "done",
];
const RESULT_STATUS: Record<ResultSummary["subtype"], AgentRoundStatus> = {
  error: "error",
  interrupted: "cancelled",
  success: "done",
};
const ACTIVE_STATUSES = new Set<AgentRoundStatus>(["pending", "streaming"]);
const TERMINAL_SLOT_STATUSES = new Set<AgentRoundStatus>([
  "cancelled",
  "done",
  "error",
]);
const TERMINAL_STATUS_PRIORITY: Partial<Record<AgentRoundStatus, number>> = {
  cancelled: 2,
  done: 1,
  error: 3,
};
const ROOM_DISPLAY_ORDER_SCALE = 1_000;

export function hasRoomAgentRoundEntries(
  messages: Message[],
  pendingSlots: RoomPendingAgentSlotState[] = [],
  pendingPermissions: PendingPermission[] = [],
  executionStates: RoomAgentExecutionState[] = [],
): boolean {
  return (
    pendingSlots.length > 0 ||
    pendingPermissions.some(hasExactPermissionEntryIdentity) ||
    executionStates.some(isVisibleRoomAgentExecutionState) ||
    messages.some(
      (message) => Boolean(message.agent_id) && message.role === "assistant",
    )
  );
}

function buildMessageGroups(
  messages: Message[],
  pendingSlotsByAgent: Map<string, RoomPendingAgentSlotState[]>,
): {
  groups: Map<string, AssistantMessage[]>;
  orders: Map<string, number>;
} {
  const groups = new Map<string, AssistantMessage[]>();
  const orders = new Map<string, number>();
  messages.forEach((message, order) => {
    if (message.role !== "assistant" || !message.agent_id) {
      return;
    }
    const entryId = resolveMessageEntryId(message, pendingSlotsByAgent);
    const group = groups.get(entryId);
    if (group) {
      group.push(message);
    } else {
      groups.set(entryId, [message]);
    }
    const displayOrder = message.display_order;
    if (typeof displayOrder === "number" && Number.isFinite(displayOrder)) {
      orders.set(entryId, displayOrder);
    } else if (!orders.has(entryId)) {
      orders.set(
        entryId,
        resolveObservedDisplayOrder(message.timestamp, order),
      );
    }
  });
  return { groups, orders };
}

function getLatestResultSummary(
  messages: AssistantMessage[],
): ResultSummary | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.result_summary) {
      return message.result_summary;
    }
  }
  return undefined;
}

function buildPendingSlots(slots: RoomPendingAgentSlotState[]): {
  byAgent: Map<string, RoomPendingAgentSlotState[]>;
  orders: Map<string, number>;
  slots: Map<string, RoomPendingAgentSlotState>;
} {
  const pendingSlots = new Map<string, RoomPendingAgentSlotState>();
  const pendingSlotOrders = new Map<string, number>();
  const pendingSlotsByAgent = new Map<string, RoomPendingAgentSlotState[]>();
  slots.forEach((slot, order) => {
    const entryId = buildAgentRoundEntryId(slot.agent_id, slot.agent_round_id);
    const current = pendingSlots.get(entryId);
    if (!current || slot.timestamp >= current.timestamp) {
      pendingSlots.set(entryId, slot);
      pendingSlotOrders.set(
        entryId,
        resolvePendingSlotDisplayOrder(slot, order),
      );
    }
    const agentSlots = pendingSlotsByAgent.get(slot.agent_id) ?? [];
    agentSlots.push(slot);
    pendingSlotsByAgent.set(slot.agent_id, agentSlots);
  });
  return {
    byAgent: pendingSlotsByAgent,
    orders: pendingSlotOrders,
    slots: pendingSlots,
  };
}

function resolvePendingSlotDisplayOrder(
  slot: RoomPendingAgentSlotState,
  fallbackOrder: number,
): number {
  return resolveObservedDisplayOrder(
    slot.timestamp,
    slot.index ?? fallbackOrder,
  );
}

function resolveObservedDisplayOrder(
  observedAt: number,
  fallbackOrder: number,
): number {
  if (Number.isFinite(observedAt) && observedAt > 0) {
    return (
      Math.trunc(observedAt) * ROOM_DISPLAY_ORDER_SCALE
      + Math.max(fallbackOrder, 0)
    );
  }
  return Math.max(fallbackOrder, 0);
}

function buildRoomAgentRoundIndex(
  messages: Message[],
  slots: RoomPendingAgentSlotState[],
  permissions: PendingPermission[],
  executionStates: RoomAgentExecutionState[],
): RoomAgentRoundIndex {
  const pending = buildPendingSlots(slots);
  const messageGroups = buildMessageGroups(messages, pending.byAgent);
  const permissionEntries = buildPermissionEntries(permissions);
  const executionEntries = buildExecutionStateEntries(executionStates);
  return {
    entryIds: new Set([
      ...messageGroups.groups.keys(),
      ...pending.slots.keys(),
      ...permissionEntries.identities.keys(),
      ...executionEntries.states.keys(),
    ]),
    executionStates: executionEntries.states,
    executionStateOrders: executionEntries.orders,
    messageGroups: messageGroups.groups,
    messageOrders: messageGroups.orders,
    permissionIdentities: permissionEntries.identities,
    permissionOrders: permissionEntries.orders,
    pendingSlots: pending.slots,
    pendingSlotOrders: pending.orders,
  };
}

function buildExecutionStateEntries(
  states: RoomAgentExecutionState[],
): {
  orders: Map<string, number>;
  states: Map<string, RoomAgentExecutionState>;
} {
  const entries = new Map<string, RoomAgentExecutionState>();
  const orders = new Map<string, number>();
  for (const state of states) {
    const entryId = buildAgentRoundEntryId(
      state.agent_id,
      state.agent_round_id,
    );
    entries.set(entryId, state);
    orders.set(entryId, state.display_order);
  }
  return { orders, states: entries };
}

function buildPermissionEntries(permissions: PendingPermission[]): {
  identities: Map<string, PermissionEntryIdentity>;
  orders: Map<string, number>;
} {
  const identities = new Map<string, PermissionEntryIdentity>();
  const orders = new Map<string, number>();
  permissions.forEach((permission, order) => {
    const identity = resolvePermissionEntryIdentity(permission);
    if (!identity) {
      return;
    }
    const entryId = buildAgentRoundEntryId(
      identity.agentId,
      identity.agentRoundId,
    );
    if (!identities.has(entryId)) {
      identities.set(entryId, identity);
      orders.set(entryId, order);
    }
  });
  return { identities, orders };
}

function hasExactPermissionEntryIdentity(
  permission: PendingPermission,
): boolean {
  return resolvePermissionEntryIdentity(permission) !== null;
}

function resolvePermissionEntryIdentity(
  permission: PendingPermission,
): PermissionEntryIdentity | null {
  const agentId = permission.agent_id?.trim();
  const agentRoundId = permission.agent_round_id?.trim();
  return agentId && agentRoundId ? { agentId, agentRoundId } : null;
}

function resolveMessageEntryId(
  message: AssistantMessage,
  pendingSlotsByAgent: Map<string, RoomPendingAgentSlotState[]>,
): string {
  const agentRoundId = message.agent_round_id?.trim();
  if (agentRoundId) {
    return buildAgentRoundEntryId(message.agent_id, agentRoundId);
  }
  const agentSlots = pendingSlotsByAgent.get(message.agent_id) ?? [];
  const parentId = message.parent_id?.trim();
  if (parentId) {
    const parentSlot = agentSlots.find((slot) => slot.msg_id === parentId);
    if (parentSlot) {
      // 某些中断路径先广播了缺少 agent_round_id 的合成结果，
      // 但 parent_id 仍精确指向同一个 slot；优先用这个稳定身份归并。
      return buildAgentRoundEntryId(
        message.agent_id,
        parentSlot.agent_round_id,
      );
    }
  }
  if (agentSlots.length === 1 && isLegacyActiveAssistantMessage(message)) {
    return buildAgentRoundEntryId(
      message.agent_id,
      agentSlots[0].agent_round_id,
    );
  }
  return buildAgentRoundEntryId(message.agent_id, null);
}

function buildAgentRoundEntryId(
  agentId: string,
  agentRoundId?: string | null,
): string {
  const normalizedRoundId = agentRoundId?.trim();
  return normalizedRoundId
    ? `${agentId}:agent-round:${normalizedRoundId}`
    : `${agentId}:legacy-round`;
}

function isLegacyActiveAssistantMessage(message: AssistantMessage): boolean {
  const status = message.stream_status
    ?? (message.stop_reason || message.is_complete ? "done" : "streaming");
  return !message.result_summary && ACTIVE_STATUSES.has(status);
}

function replaceSyntheticResultWithCanonical(
  messages: AssistantMessage[],
): AssistantMessage[] {
  const canonical = messages.filter((message) => !isSyntheticResult(message));
  if (canonical.length === 0 || canonical.length === messages.length) {
    return messages;
  }
  const synthetic = [...messages].reverse().find(isSyntheticResult);
  if (!synthetic) {
    return canonical;
  }
  const next = [...canonical];
  const lastIndex = next.length - 1;
  const last = next[lastIndex];
  const syntheticText = extractTextFromContentBlocks(synthetic.content);
  const resultSummary = synthetic.result_summary
    ? {
        ...synthetic.result_summary,
        ...(synthetic.result_summary.result || !syntheticText
          ? {}
          : { result: syntheticText }),
      }
    : undefined;
  next[lastIndex] = {
    ...last,
    is_complete: synthetic.is_complete ?? true,
    result_summary: last.result_summary ?? resultSummary,
    stop_reason: last.stop_reason ?? synthetic.stop_reason,
    stream_status: last.result_summary
      ? last.stream_status
      : synthetic.stream_status ?? last.stream_status,
  };
  return next;
}

function isSyntheticResult(message: AssistantMessage): boolean {
  const resultMessageId = message.result_summary?.message_id?.trim();
  if (resultMessageId) {
    return message.message_id === `assistant_${resultMessageId}`;
  }
  return message.message_id === `assistant_result_${message.round_id}`;
}

function getAgentRoundStatus(
  messages: AssistantMessage[],
  resultSummary?: ResultSummary,
  pendingSlot?: RoomPendingAgentSlotState,
  continuingToolTurn: boolean = false,
): AgentRoundStatus {
  const resultStatus = resolveResultStatus(resultSummary);
  if (resultStatus) {
    // 终态 result 是同一 slot 的权威收口，不能被尚未清理的 pending slot 覆盖。
    return resultStatus;
  }
  const messageStatus = resolveMessageStatus(messages);
  if (pendingSlot && ACTIVE_STATUSES.has(pendingSlot.status)) {
    if (continuingToolTurn) {
      return pendingSlot.status;
    }
    if (TERMINAL_SLOT_STATUSES.has(messageStatus)) {
      return messageStatus;
    }
    return messageStatus === "streaming"
      ? "streaming"
      : pendingSlot.status;
  }
  return pendingSlot?.status ?? messageStatus;
}

function boundStatusByExecutionLifecycle(
  status: AgentRoundStatus,
  executionState?: RoomAgentExecutionState,
  hasTerminalResult: boolean = false,
  hasLiveActivityEvidence: boolean = false,
): AgentRoundStatus {
  if (executionState?.phase === "active" && !hasTerminalResult) {
    // Thread lifecycle 仍 active 时，一次 Assistant message_stop / is_complete
    // 只代表 turn 边界；主 Feed 必须继续投影动态活动状态。
    return executionState.status;
  }
  if (executionState?.phase !== "terminal") {
    // Message history can contain an interrupted Assistant row whose persisted
    // stream_status was never rewritten. Without a live slot or execution
    // lifecycle that row is structural history, not evidence of current work.
    return !hasLiveActivityEvidence && ACTIVE_STATUSES.has(status) ? "done" : status;
  }
  const executionStatus = executionState.status;
  if (!TERMINAL_SLOT_STATUSES.has(status)) {
    return executionStatus;
  }
  return (TERMINAL_STATUS_PRIORITY[executionStatus] ?? 0)
      > (TERMINAL_STATUS_PRIORITY[status] ?? 0)
    ? executionStatus
    : status;
}

function resolveResultStatus(
  summary?: ResultSummary,
): AgentRoundStatus | null {
  if (!summary) {
    return null;
  }
  return summary.is_error ? "error" : RESULT_STATUS[summary.subtype];
}

function resolveMessageStatus(
  messages: AssistantMessage[],
): AgentRoundStatus {
  if (messages.length === 0) {
    return "pending";
  }

  const statuses = new Set<AgentRoundStatus>();
  for (const message of messages) {
    if (message.stream_status) {
      statuses.add(message.stream_status);
    }
    if (message.is_complete || message.stop_reason) {
      statuses.add("done");
    }
  }
  return (
    MESSAGE_STATUS_PRIORITY.find((status) => statuses.has(status)) ??
    "cancelled"
  );
}

/**
 * Agent 卡片的时间语义只由执行状态决定：运行态保持启动时间稳定，
 * 终态使用 result 的完成时间。feed 排序和卡片 header 必须复用该值。
 */
export function resolveRoomAgentRoundTimestamp(
  status: AgentRoundStatus,
  messages: AssistantMessage[],
  resultSummary?: ResultSummary,
  pendingSlot?: RoomPendingAgentSlotState,
  executionState?: RoomAgentExecutionState,
): number {
  if (isAgentRoundActive(status)) {
    return executionState?.first_seen_at
      ?? pendingSlot?.timestamp
      ?? messages[0]?.timestamp
      ?? resultSummary?.timestamp
      ?? 0;
  }
  return resultSummary?.timestamp
    ?? messages.at(-1)?.timestamp
    ?? pendingSlot?.timestamp
    ?? executionState?.first_seen_at
    ?? 0;
}

function buildRoomAgentRoundEntry(
  index: RoomAgentRoundIndex,
  entryId: string,
): RoomAgentRoundEntry | null {
  const pendingSlot = index.pendingSlots.get(entryId);
  const permissionIdentity = index.permissionIdentities.get(entryId);
  const executionState = index.executionStates.get(entryId);
  const assistantMessages = replaceSyntheticResultWithCanonical(
    index.messageGroups.get(entryId) ?? [],
  );
  const resultSummary = getLatestResultSummary(assistantMessages);
  const continuingToolTurn = !resultSummary
    && assistantMessages.at(-1)?.stop_reason === "tool_use";
  if (
    assistantMessages.length === 0
    && !resultSummary
    && !pendingSlot
    && !permissionIdentity
    && (!executionState || !isVisibleRoomAgentExecutionState(executionState))
  ) {
    return null;
  }
  const identity = assistantMessages.at(-1);
  const agentId = pendingSlot?.agent_id
    ?? identity?.agent_id
    ?? permissionIdentity?.agentId
    ?? executionState?.agent_id;
  if (!agentId) {
    return null;
  }
  const agentRoundId = pendingSlot?.agent_round_id?.trim()
    || identity?.agent_round_id?.trim()
    || permissionIdentity?.agentRoundId
    || executionState?.agent_round_id
    || null;
  const projectedStatus = assistantMessages.length > 0 || pendingSlot
    ? getAgentRoundStatus(
        assistantMessages,
        resultSummary,
        pendingSlot,
        continuingToolTurn,
      )
    : executionState?.status ?? "pending";
  const status = boundStatusByExecutionLifecycle(
    projectedStatus,
    executionState,
    Boolean(resultSummary),
    Boolean(pendingSlot || permissionIdentity),
  );
  return {
    entry_id: entryId,
    agent_id: agentId,
    agent_round_id: agentRoundId,
    assistant_messages: assistantMessages,
    result_summary: resultSummary,
    pending_slot: pendingSlot,
    status,
    timestamp: resolveRoomAgentRoundTimestamp(
      status,
      assistantMessages,
      resultSummary,
      pendingSlot,
      executionState,
    ),
    // 首次观察哪种证据就登记哪一槽；后到 permission / slot / message
    // 只补齐同一 execution，不再以来源 precedence 改写并行卡片顺序。
    display_order: index.executionStateOrders.get(entryId)
      ?? index.permissionOrders.get(entryId)
      ?? index.pendingSlotOrders.get(entryId)
      ?? index.messageOrders.get(entryId)
      ?? Number.MAX_SAFE_INTEGER,
  };
}

export function isAgentRoundActive(status: AgentRoundStatus): boolean {
  return ACTIVE_STATUSES.has(status);
}

export function buildRoomAgentRoundEntries(
  messages: Message[],
  pendingSlots: RoomPendingAgentSlotState[] = [],
  pendingPermissions: PendingPermission[] = [],
  executionStates: RoomAgentExecutionState[] = [],
): RoomAgentRoundEntry[] {
  const index = buildRoomAgentRoundIndex(
    messages,
    pendingSlots,
    pendingPermissions,
    executionStates,
  );
  const entries = Array.from(index.entryIds).flatMap((entryId) => {
    const entry = buildRoomAgentRoundEntry(index, entryId);
    return entry ? [entry] : [];
  }).sort(compareAgentRoundDisplayOrder);
  return settleSupersededToolTurns(entries);
}

function settleSupersededToolTurns(
  entries: RoomAgentRoundEntry[],
): RoomAgentRoundEntry[] {
  const laterExecutions = new Set<string>();
  let changed = false;
  const next = [...entries];
  for (let index = entries.length - 1; index >= 0; index -= 1) {
    const entry = entries[index];
    const roundId = entry.pending_slot?.round_id
      ?? entry.assistant_messages.at(-1)?.round_id;
    if (!roundId) {
      continue;
    }
    const key = `${roundId}\u001f${entry.agent_id}`;
    const lastMessage = entry.assistant_messages.at(-1);
    if (
      laterExecutions.has(key)
      && !entry.result_summary
      && lastMessage?.stop_reason === "tool_use"
      && entry.status !== "cancelled"
    ) {
      next[index] = { ...entry, status: "cancelled" };
      changed = true;
    }
    laterExecutions.add(key);
  }
  return changed ? next : entries;
}

export function getRoomAgentRoundEntry(
  messages: Message[],
  agentId: string,
  pendingSlots: RoomPendingAgentSlotState[] = [],
  agentRoundId?: string | null,
  executionStates: RoomAgentExecutionState[] = [],
): RoomAgentRoundEntry | null {
  const entries = buildRoomAgentRoundEntries(
    messages,
    pendingSlots,
    [],
    executionStates,
  ).filter((entry) => entry.agent_id === agentId);
  const normalizedRoundId = agentRoundId?.trim();
  if (normalizedRoundId) {
    return entries.find(
      (entry) => entry.agent_round_id === normalizedRoundId,
    ) ?? null;
  }
  return entries.filter((entry) => isAgentRoundActive(entry.status)).at(-1)
    ?? entries.at(-1)
    ?? null;
}

function compareAgentRoundDisplayOrder(
  left: RoomAgentRoundEntry,
  right: RoomAgentRoundEntry,
): number {
  return left.display_order - right.display_order
    || left.timestamp - right.timestamp
    || left.entry_id.localeCompare(right.entry_id);
}
