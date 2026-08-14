/**
 * INPUT: 会话消息、Room slot/permission/execution 易失态、运行轮次与服务端 round 索引。
 * OUTPUT: DM / Room 共用的消息及易失态根轮次分组、历史窗口可见性与 feed 顺序纯投影。
 * POS: 时间线顺序的唯一真相源，feed 与 navigator 不得自行修正轮次。
 */
import type {
  AssistantMessage,
  Message,
  UserMessage,
} from "@/types/conversation/message/entity";
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { SessionRoundIndexItem } from "@/types/conversation/history";
import { hasVisibleAssistantOutput } from "../message/message-content-model";

/** DM / Room 共用的唯一时间线投影。 */
export interface ConversationTimeline {
  message_groups: Map<string, Message[]>;
  room_agent_execution_state_groups: Map<string, RoomAgentExecutionState[]>;
  pending_slot_groups: Map<string, RoomPendingAgentSlotState[]>;
  pending_permission_groups: Map<string, PendingPermission[]>;
  loaded_round_ids: string[];
  feed_round_ids: string[];
  round_index_items: SessionRoundIndexItem[];
  live_round_ids: string[];
}

export interface VisibleRoomTimelineEvidence {
  executionStates: RoomAgentExecutionState[];
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
}

function groupByRound<T>(
  items: T[],
  getRoundId: (item: T) => string | null | undefined,
): Map<string, T[]> {
  const groups = new Map<string, T[]>();
  for (const item of items) {
    const roundId = getRoundId(item)?.trim();
    if (!roundId) {
      continue;
    }
    const group = groups.get(roundId);
    if (group) {
      group.push(item);
    } else {
      groups.set(roundId, [item]);
    }
  }
  return groups;
}

/** 消息的 round_id 已由后端归一为根轮次。 */
export function groupMessagesByRound(
  messages: Message[],
): Map<string, Message[]> {
  return groupByRound(
    filterVisibleTimelineMessages(messages),
    (message) => message.round_id,
  );
}

/** 用户不可见消息在 canonical timeline 入口统一移除。 */
export function filterVisibleTimelineMessages(
  messages: Message[],
): Message[] {
  const visible = messages.filter(
    (message) => message.hidden_from_user !== true,
  );
  return visible.length === messages.length ? messages : visible;
}

/**
 * Room 私域执行仍参与宿主运行态，但不能进入公区 Feed、Thread 或导航。
 * slot 与 execution 任一携带隐藏事实，都按精确 agent round 隐藏整组易失证据。
 */
export function projectVisibleRoomTimelineEvidence(
  pendingSlots: RoomPendingAgentSlotState[],
  pendingPermissions: PendingPermission[],
  executionStates: RoomAgentExecutionState[],
): VisibleRoomTimelineEvidence {
  const hiddenAgentRounds = new Set<string>();
  pendingSlots.forEach((slot) => {
    if (slot.hidden_from_user === true) {
      addRoomAgentRoundIdentity(
        hiddenAgentRounds,
        slot.agent_id,
        slot.agent_round_id,
      );
    }
  });
  executionStates.forEach((state) => {
    if (state.hidden_from_user === true) {
      addRoomAgentRoundIdentity(
        hiddenAgentRounds,
        state.agent_id,
        state.agent_round_id,
      );
    }
  });
  if (hiddenAgentRounds.size === 0) {
    return { executionStates, pendingPermissions, pendingSlots };
  }
  return {
    executionStates: executionStates.filter(
      (state) => !hasRoomAgentRoundIdentity(
        hiddenAgentRounds,
        state.agent_id,
        state.agent_round_id,
      ),
    ),
    pendingPermissions: pendingPermissions.filter(
      (permission) => !hasRoomAgentRoundIdentity(
        hiddenAgentRounds,
        permission.agent_id,
        permission.agent_round_id,
      ),
    ),
    pendingSlots: pendingSlots.filter(
      (slot) => !hasRoomAgentRoundIdentity(
        hiddenAgentRounds,
        slot.agent_id,
        slot.agent_round_id,
      ),
    ),
  };
}

function addRoomAgentRoundIdentity(
  identities: Set<string>,
  agentId: string | null | undefined,
  agentRoundId: string | null | undefined,
): void {
  const identity = buildRoomAgentRoundIdentity(agentId, agentRoundId);
  if (identity) {
    identities.add(identity);
  }
}

function hasRoomAgentRoundIdentity(
  identities: ReadonlySet<string>,
  agentId: string | null | undefined,
  agentRoundId: string | null | undefined,
): boolean {
  const identity = buildRoomAgentRoundIdentity(agentId, agentRoundId);
  return Boolean(identity && identities.has(identity));
}

function buildRoomAgentRoundIdentity(
  agentId: string | null | undefined,
  agentRoundId: string | null | undefined,
): string | null {
  const normalizedAgentId = agentId?.trim();
  const normalizedAgentRoundId = agentRoundId?.trim();
  return normalizedAgentId && normalizedAgentRoundId
    ? `${normalizedAgentId}\u001f${normalizedAgentRoundId}`
    : null;
}

export function groupPendingPermissionsByRound(
  permissions: PendingPermission[],
): Map<string, PendingPermission[]> {
  return groupByRound(permissions, (permission) => permission.round_id);
}

export function groupPendingSlotsByRound(
  slots: RoomPendingAgentSlotState[],
): Map<string, RoomPendingAgentSlotState[]> {
  return groupByRound(slots, (slot) => slot.round_id);
}

export function groupRoomAgentExecutionStatesByRound(
  states: RoomAgentExecutionState[],
): Map<string, RoomAgentExecutionState[]> {
  return groupByRound(states, (state) => state.round_id);
}

/** 只保留已经具备公区消息或公区运行证据的 Room live round。 */
export function filterVisibleRoomLiveRoundIds(
  liveRoundIds: string[],
  messageGroups: ReadonlyMap<string, readonly Message[]>,
  pendingPermissionGroups: ReadonlyMap<string, readonly PendingPermission[]>,
  pendingSlotGroups: ReadonlyMap<string, readonly RoomPendingAgentSlotState[]>,
  executionStateGroups: ReadonlyMap<
    string,
    readonly RoomAgentExecutionState[]
  >,
): string[] {
  const visible = liveRoundIds.filter((roundId) => (
    messageGroups.has(roundId)
    || pendingPermissionGroups.has(roundId)
    || pendingSlotGroups.has(roundId)
    || executionStateGroups.has(roundId)
  ));
  return visible.length === liveRoundIds.length ? liveRoundIds : visible;
}

/** 删除已被加载消息迁入其他根轮次的原始 round 索引。 */
export function filterSupersededRoundIndexItems(
  roundIndexItems: SessionRoundIndexItem[],
  messages: Message[],
): SessionRoundIndexItem[] {
  const targetRoundIdsBySource = new Map<string, Set<string>>();
  const loadedRoundIds = new Set<string>();
  for (const message of messages) {
    const roundId = message.round_id.trim();
    if (roundId) {
      loadedRoundIds.add(roundId);
    }
    const sourceRoundId = getMessageSourceRoundId(message);
    if (!sourceRoundId || !roundId || sourceRoundId === roundId) {
      continue;
    }
    const targetRoundIds = targetRoundIdsBySource.get(sourceRoundId)
      ?? new Set<string>();
    targetRoundIds.add(roundId);
    targetRoundIdsBySource.set(sourceRoundId, targetRoundIds);
  }
  if (targetRoundIdsBySource.size === 0) {
    return roundIndexItems;
  }

  const indexedRoundIds = new Set(
    roundIndexItems.map((item) => item.roundId.trim()).filter(Boolean),
  );
  return roundIndexItems.filter((item) => {
    const roundId = item.roundId.trim();
    const targetRoundIds = targetRoundIdsBySource.get(roundId);
    if (!targetRoundIds) {
      return true;
    }
    // 部分 Room 成员可被引导、其他成员仍在 source round 回复；这类 round
    // 有正文/运行态证据，不能随迁入旧 root 的用户消息一起从导航删除。
    if (
      loadedRoundIds.has(roundId)
      || item.isLive
      || item.agentIds.length > 0
      || item.status !== null
      || item.durationMs !== null
    ) {
      return true;
    }
    // 目标 root 尚未进入索引时保留 source，避免 navigator 出现空洞。
    return !Array.from(targetRoundIds).some((targetRoundId) => (
      indexedRoundIds.has(targetRoundId)
    ));
  });
}

/**
 * 目标窗口请求成功后，只保留实际投影出用户可见内容的已解析轮次。
 * 未解析轮次仍保留短暂占位，避免请求完成前导航与 feed 出现空洞。
 */
export function filterResolvedEmptyRoundIndexItems(
  roundIndexItems: SessionRoundIndexItem[],
  visibleRoundIds: string[],
  resolvedRoundIds: string[],
): SessionRoundIndexItem[] {
  if (resolvedRoundIds.length === 0) {
    return roundIndexItems;
  }

  const visible = new Set(
    visibleRoundIds.map((roundId) => roundId.trim()).filter(Boolean),
  );
  const resolved = new Set(
    resolvedRoundIds.map((roundId) => roundId.trim()).filter(Boolean),
  );
  return roundIndexItems.filter((item) => {
    const roundId = item.roundId.trim();
    return !resolved.has(roundId) || visible.has(roundId);
  });
}

function getMessageSourceRoundId(message: Message): string {
  const directSourceRoundId = message.source_round_id?.trim();
  if (directSourceRoundId) {
    return directSourceRoundId;
  }
  if (
    message.role !== "system"
    || message.metadata?.subtype !== "guided_input"
  ) {
    return "";
  }
  const metadataSourceRoundId = message.metadata.source_round_id;
  return typeof metadataSourceRoundId === "string"
    ? metadataSourceRoundId.trim()
    : "";
}

// 终态轮次里 assistant 仅剩无回复标记（剥离后无文本、无工具/图片等块）时，
// 视为纯 no-reply，不在时间线显示。保守判定：任何工具/非文本块都算可见输出。
function hasVisibleUserContent(message: UserMessage): boolean {
  return Boolean(message.content.trim()) || Boolean(message.attachments?.length);
}

function isBlankNoReplyRound(messages: Message[]): boolean {
  // 用户消息不能因 Assistant 无回复而被整轮吞掉。
  const hasVisibleUserMessage = messages
    .filter((message): message is UserMessage => message.role === "user")
    .some(hasVisibleUserContent);
  const assistants = messages.filter(
    (message): message is AssistantMessage => message.role === "assistant",
  );
  return !hasVisibleUserMessage
    && assistants.length > 0
    && !assistants.some(hasVisibleAssistantOutput);
}

/** 时间线除历史消息外，也要显示已启动但尚未产生消息的运行轮次。 */
export function buildTimelineRoundIds(
  messageGroups: Map<string, Message[]>,
  liveRoundIds: string[] = [],
  extraRoundIds: Iterable<string> = [],
): string[] {
  const live = new Set(liveRoundIds);
  const roundIds = Array.from(messageGroups.keys()).filter(
    (roundId) =>
      live.has(roundId) ||
      !isBlankNoReplyRound(messageGroups.get(roundId) ?? []),
  );
  const seen = new Set(roundIds);

  for (const roundId of extraRoundIds) {
    appendUniqueRoundId(roundIds, seen, roundId);
  }
  for (const roundId of liveRoundIds) {
    appendUniqueRoundId(roundIds, seen, roundId);
  }
  return roundIds;
}

function appendUniqueRoundId(
  roundIds: string[],
  seen: Set<string>,
  roundId: string | null | undefined,
) {
  const normalized = roundId?.trim();
  if (!normalized || seen.has(normalized)) {
    return;
  }
  seen.add(normalized);
  roundIds.push(normalized);
}

function normalizeRoundIds(roundIds: Iterable<string>): string[] {
  const normalized: string[] = [];
  const seen = new Set<string>();
  for (const roundId of roundIds) {
    appendUniqueRoundId(normalized, seen, roundId);
  }
  return normalized;
}

/**
 * 用共同轮次作锚点，把只存在于已加载 transcript 的旧轮次放回正确位置。
 *
 * DM 的 durable overlay 可能晚于 transcript 启用，因此服务端轻量索引可以只含
 * 新轮次；已加载消息仍是其窗口内的时间顺序真相，不能统一追加到索引尾部。
 */
function mergeIndexedAndLoadedRoundIds(
  indexedRoundIds: string[],
  loadedRoundIds: string[],
): string[] {
  const indexed = normalizeRoundIds(indexedRoundIds);
  const loaded = normalizeRoundIds(loadedRoundIds);
  if (indexed.length === 0) {
    return loaded;
  }
  if (loaded.length === 0) {
    return indexed;
  }

  const indexedPositionByRoundId = new Map<string, number>();
  indexed.forEach((roundId, index) => {
    indexedPositionByRoundId.set(roundId, index);
  });

  const nextIndexedPositions = new Array<number | null>(loaded.length).fill(null);
  let nextIndexedPosition: number | null = null;
  for (let index = loaded.length - 1; index >= 0; index -= 1) {
    const position = indexedPositionByRoundId.get(loaded[index]);
    if (position !== undefined) {
      nextIndexedPosition = position;
    }
    nextIndexedPositions[index] = nextIndexedPosition;
  }

  const insertionBuckets = Array.from(
    { length: indexed.length + 1 },
    () => [] as string[],
  );
  let previousIndexedPosition: number | null = null;
  loaded.forEach((roundId, index) => {
    const indexedPosition = indexedPositionByRoundId.get(roundId);
    if (indexedPosition !== undefined) {
      previousIndexedPosition = indexedPosition;
      return;
    }

    const nextPosition = nextIndexedPositions[index];
    const canInsertBeforeNext = nextPosition !== null
      && (previousIndexedPosition === null || nextPosition > previousIndexedPosition);
    const insertionPosition = canInsertBeforeNext
      ? nextPosition
      : previousIndexedPosition !== null
        ? previousIndexedPosition + 1
        : indexed.length;
    insertionBuckets[insertionPosition].push(roundId);
  });

  const merged: string[] = [];
  for (let index = 0; index <= indexed.length; index += 1) {
    merged.push(...insertionBuckets[index]);
    if (index < indexed.length) {
      merged.push(indexed[index]);
    }
  }
  return merged;
}

function createLoadedRoundIndexItem(roundId: string): SessionRoundIndexItem {
  return {
    agentIds: [],
    durationMs: null,
    hasUserMessage: false,
    isLive: false,
    roundId,
    status: null,
    timestamp: null,
    title: "",
  };
}

/** 让 feed 与 navigator 共用包含旧 transcript 轮次的完整有序投影。 */
export function mergeLoadedRoundIndexItems(
  roundIndexItems: SessionRoundIndexItem[],
  loadedRoundIds: string[],
): SessionRoundIndexItem[] {
  const itemByRoundId = new Map<string, SessionRoundIndexItem>();
  for (const item of roundIndexItems) {
    const roundId = item.roundId.trim();
    if (roundId && !itemByRoundId.has(roundId)) {
      itemByRoundId.set(roundId, item);
    }
  }
  const orderedRoundIds = mergeIndexedAndLoadedRoundIds(
    Array.from(itemByRoundId.keys()),
    loadedRoundIds,
  );
  return orderedRoundIds.map(
    (roundId) => itemByRoundId.get(roundId) ?? createLoadedRoundIndexItem(roundId),
  );
}

function getIndexedLoadedRoundIndexes(
  indexedRoundIds: string[],
  loadedRoundIds: string[],
): number[] {
  const indexByRoundId = new Map<string, number>();
  indexedRoundIds.forEach((roundId, index) => {
    indexByRoundId.set(roundId, index);
  });

  const indexes = new Set<number>();
  for (const roundId of loadedRoundIds) {
    const index = indexByRoundId.get(roundId);
    if (index !== undefined) {
      indexes.add(index);
    }
  }
  return Array.from(indexes).sort((left, right) => left - right);
}

function isLatestLoadedWindow(
  indexedRoundIds: string[],
  loadedIndexes: number[],
): boolean {
  if (loadedIndexes.length === 0) {
    return false;
  }
  const firstLoadedIndex = loadedIndexes[0];
  const expectedLength = indexedRoundIds.length - firstLoadedIndex;
  if (expectedLength !== loadedIndexes.length) {
    return false;
  }
  return loadedIndexes.every(
    (index, offset) => index === firstLoadedIndex + offset,
  );
}

/**
 * 用完整索引确定 feed 顺序，但正文只渲染已加载窗口。
 *
 * 最新历史页不插入未加载哨兵，避免新打开旧 session 时因为全量索引
 * 直接产生很长的空滚动；非最新窗口保留相邻哨兵，让点击定位后还能
 * 继续通过正常滚动触发局部加载。哨兵不渲染任何用户可见状态。
 */
export function buildIndexedTimelineRoundIds(
  roundIndexItems: SessionRoundIndexItem[],
  loadedRoundIds: string[],
): string[] {
  if (roundIndexItems.length === 0) {
    return loadedRoundIds;
  }

  const indexedRoundIds = mergeIndexedAndLoadedRoundIds(
    roundIndexItems.map((item) => item.roundId),
    loadedRoundIds,
  );
  const indexedRoundIdSet = new Set(indexedRoundIds);
  const loadedIndexes = getIndexedLoadedRoundIndexes(
    indexedRoundIds,
    loadedRoundIds,
  );
  const shouldIncludeBoundaryPlaceholders =
    !isLatestLoadedWindow(indexedRoundIds, loadedIndexes);
  const seen = new Set<string>();
  const roundIds: string[] = [];

  const visibleIndexSet = new Set(loadedIndexes);
  if (shouldIncludeBoundaryPlaceholders) {
    for (const index of loadedIndexes) {
      if (index > 0) {
        visibleIndexSet.add(index - 1);
      }
      if (index < indexedRoundIds.length - 1) {
        visibleIndexSet.add(index + 1);
      }
    }
  }

  for (const index of Array.from(visibleIndexSet).sort((left, right) => left - right)) {
    appendUniqueRoundId(roundIds, seen, indexedRoundIds[index]);
  }
  for (const roundId of loadedRoundIds) {
    if (!indexedRoundIdSet.has(roundId)) {
      appendUniqueRoundId(roundIds, seen, roundId);
    }
  }
  return roundIds;
}
