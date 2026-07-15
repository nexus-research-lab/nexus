/**
 * INPUT: Room 根轮次消息与尚未结束的 agent slot。
 * OUTPUT: 按 agent_round_id 聚合、按终态时间排序且不含 Room 控制标记的回复卡片。
 * POS: Room feed 与 thread 共用的 Agent 执行轮次投影。
 */
import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import {
  extractTextFromContentBlocks,
  stripRoomControlMarkers,
} from "@/features/conversation/shared/message/message-content-model";

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
  messageGroups: Map<string, AssistantMessage[]>;
  messageOrders: Map<string, number>;
  pendingSlots: Map<string, RoomPendingAgentSlotState>;
  pendingSlotOrders: Map<string, number>;
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

export function hasRoomAgentRoundEntries(
  messages: Message[],
  pendingSlots: RoomPendingAgentSlotState[] = [],
): boolean {
  return (
    pendingSlots.length > 0 ||
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
    orders.set(entryId, order);
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
      pendingSlotOrders.set(entryId, slot.index ?? order);
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

function buildRoomAgentRoundIndex(
  messages: Message[],
  slots: RoomPendingAgentSlotState[],
): RoomAgentRoundIndex {
  const pending = buildPendingSlots(slots);
  const messageGroups = buildMessageGroups(messages, pending.byAgent);
  return {
    entryIds: new Set([
      ...messageGroups.groups.keys(),
      ...pending.slots.keys(),
    ]),
    messageGroups: messageGroups.groups,
    messageOrders: messageGroups.orders,
    pendingSlots: pending.slots,
    pendingSlotOrders: pending.orders,
  };
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
): AgentRoundStatus {
  if (pendingSlot && ACTIVE_STATUSES.has(pendingSlot.status)) {
    return resolveMessageStatus(messages) === "streaming"
      ? "streaming"
      : pendingSlot.status;
  }
  return (
    resolveResultStatus(resultSummary) ??
    pendingSlot?.status ??
    resolveMessageStatus(messages)
  );
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
    if (message.stop_reason) {
      statuses.add("done");
    }
  }
  return (
    MESSAGE_STATUS_PRIORITY.find((status) => statuses.has(status)) ??
    "cancelled"
  );
}

function getAgentRoundTimestamp(
  messages: AssistantMessage[],
  resultSummary?: ResultSummary,
  pendingSlot?: RoomPendingAgentSlotState,
): number {
  if (resultSummary?.timestamp) {
    return resultSummary.timestamp;
  }
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const timestamp = messages[index]?.timestamp;
    if (timestamp) {
      return timestamp;
    }
  }
  return pendingSlot?.timestamp ?? 0;
}

function buildRoomAgentRoundEntry(
  index: RoomAgentRoundIndex,
  entryId: string,
): RoomAgentRoundEntry | null {
  const pendingSlot = index.pendingSlots.get(entryId);
  const assistantMessages = replaceSyntheticResultWithCanonical(
    index.messageGroups.get(entryId) ?? [],
  );
  const resultSummary = getLatestResultSummary(assistantMessages);
  if (assistantMessages.length === 0 && !resultSummary && !pendingSlot) {
    return null;
  }
  const identity = assistantMessages.at(-1);
  const agentId = pendingSlot?.agent_id ?? identity?.agent_id;
  if (!agentId) {
    return null;
  }
  const agentRoundId = pendingSlot?.agent_round_id?.trim()
    || identity?.agent_round_id?.trim()
    || null;
  return {
    entry_id: entryId,
    agent_id: agentId,
    agent_round_id: agentRoundId,
    assistant_messages: assistantMessages,
    result_summary: resultSummary,
    pending_slot: pendingSlot,
    status: getAgentRoundStatus(
      assistantMessages,
      resultSummary,
      pendingSlot,
    ),
    timestamp: getAgentRoundTimestamp(
      assistantMessages,
      resultSummary,
      pendingSlot,
    ),
    display_order: Math.max(
      index.messageOrders.get(entryId) ?? -1,
      index.pendingSlotOrders.get(entryId) ?? -1,
    ),
  };
}

export function isAgentRoundActive(status: AgentRoundStatus): boolean {
  return ACTIVE_STATUSES.has(status);
}

export function buildRoomAgentRoundEntries(
  messages: Message[],
  pendingSlots: RoomPendingAgentSlotState[] = [],
): RoomAgentRoundEntry[] {
  const index = buildRoomAgentRoundIndex(messages, pendingSlots);
  return Array.from(index.entryIds).flatMap((entryId) => {
    const entry = buildRoomAgentRoundEntry(index, entryId);
    return entry ? [entry] : [];
  }).sort(compareAgentRoundDisplayOrder);
}

export function getRoomAgentRoundEntry(
  messages: Message[],
  agentId: string,
  pendingSlots: RoomPendingAgentSlotState[] = [],
  agentRoundId?: string | null,
): RoomAgentRoundEntry | null {
  const entries = buildRoomAgentRoundEntries(messages, pendingSlots).filter(
    (entry) => entry.agent_id === agentId,
  );
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
  return left.timestamp - right.timestamp
    || left.display_order - right.display_order
    || left.entry_id.localeCompare(right.entry_id);
}

function normalizePreviewText(text: string, maxLength: number): string {
  const normalizedText = stripRoomControlMarkers(text).replace(/\s+/g, " ").trim();
  if (!normalizedText) {
    return "";
  }
  return normalizedText.length > maxLength
    ? `${normalizedText.slice(0, maxLength)}…`
    : normalizedText;
}

/** 占位摘要跟随最新完整消息推进，工具块不参与文本预览。 */
export function extractAgentPreviewText(
  messages: AssistantMessage[],
  maxLength = 80,
): string {
  for (let messageIndex = messages.length - 1; messageIndex >= 0; messageIndex -= 1) {
    const content = messages[messageIndex]?.content;
    if (!Array.isArray(content)) {
      continue;
    }
    for (let blockIndex = content.length - 1; blockIndex >= 0; blockIndex -= 1) {
      const block = content[blockIndex];
      if (block.type !== "text" && block.type !== "thinking") {
        continue;
      }
      const text = block.type === "text" ? block.text : block.thinking;
      const preview = normalizePreviewText(text, maxLength);
      if (preview) {
        return preview;
      }
    }
  }
  return "";
}
