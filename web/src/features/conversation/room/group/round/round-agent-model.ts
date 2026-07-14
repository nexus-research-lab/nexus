/**
 * INPUT: Room 根轮次消息与尚未结束的 agent slot。
 * OUTPUT: 按 agent 聚合、按 agent_round_id 对齐且不含 Room 控制标记的回复卡片。
 * POS: Room feed 与 thread 共用的 Agent 执行轮次投影。
 */
import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import { stripRoomControlMarkers } from "@/features/conversation/shared/message/message-content-model";

export type AgentRoundStatus = AssistantMessageStatus;

export interface RoomAgentRoundEntry {
  agent_id: string;
  assistant_messages: AssistantMessage[];
  result_summary?: ResultSummary;
  pending_slot?: RoomPendingAgentSlotState;
  status: AgentRoundStatus;
  timestamp: number;
}

interface RoomAgentRoundIndex {
  agentIds: Set<string>;
  messageGroups: Map<string, AssistantMessage[]>;
  pendingSlots: Map<string, RoomPendingAgentSlotState>;
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
): Map<string, AssistantMessage[]> {
  const groups = new Map<string, AssistantMessage[]>();
  for (const message of messages) {
    if (message.role !== "assistant" || !message.agent_id) {
      continue;
    }
    const group = groups.get(message.agent_id);
    if (group) {
      group.push(message);
    } else {
      groups.set(message.agent_id, [message]);
    }
  }
  return groups;
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

function buildPendingSlots(
  slots: RoomPendingAgentSlotState[],
): Map<string, RoomPendingAgentSlotState> {
  const pendingSlots = new Map<string, RoomPendingAgentSlotState>();
  for (const slot of slots) {
    const current = pendingSlots.get(slot.agent_id);
    if (!current || slot.timestamp >= current.timestamp) {
      pendingSlots.set(slot.agent_id, slot);
    }
  }
  return pendingSlots;
}

function buildRoomAgentRoundIndex(
  messages: Message[],
  slots: RoomPendingAgentSlotState[],
): RoomAgentRoundIndex {
  const messageGroups = buildMessageGroups(messages);
  const pendingSlots = buildPendingSlots(slots);
  return {
    agentIds: new Set([
      ...messageGroups.keys(),
      ...pendingSlots.keys(),
    ]),
    messageGroups,
    pendingSlots,
  };
}

function selectCurrentAgentRoundMessages(
  messages: AssistantMessage[],
  pendingSlot?: RoomPendingAgentSlotState,
): AssistantMessage[] {
  const agentRoundId = pendingSlot?.agent_round_id?.trim();
  if (!agentRoundId) {
    return messages;
  }
  const matchingMessages = messages.filter(
    (message) => message.agent_round_id?.trim() === agentRoundId,
  );
  if (matchingMessages.length > 0) {
    return matchingMessages;
  }
  if (messages.some((message) => message.agent_round_id?.trim())) {
    return [];
  }
  // 旧历史没有 agent_round_id；活跃 slot 只继承仍在流式的 legacy 消息，
  // 不能让同 Agent 的旧 result 把新执行投影成 done。
  return messages.filter(isLegacyActiveAssistantMessage);
}

function isLegacyActiveAssistantMessage(message: AssistantMessage): boolean {
  const status = message.stream_status
    ?? (message.stop_reason || message.is_complete ? "done" : "streaming");
  return !message.result_summary && ACTIVE_STATUSES.has(status);
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
  agentId: string,
): RoomAgentRoundEntry | null {
  const pendingSlot = index.pendingSlots.get(agentId);
  const assistantMessages = selectCurrentAgentRoundMessages(
    index.messageGroups.get(agentId) ?? [],
    pendingSlot,
  );
  const resultSummary = getLatestResultSummary(assistantMessages);
  if (assistantMessages.length === 0 && !resultSummary && !pendingSlot) {
    return null;
  }
  return {
    agent_id: agentId,
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
  return Array.from(index.agentIds).flatMap((agentId) => {
    const entry = buildRoomAgentRoundEntry(index, agentId);
    return entry ? [entry] : [];
  });
}

export function getRoomAgentRoundEntry(
  messages: Message[],
  agentId: string,
  pendingSlots: RoomPendingAgentSlotState[] = [],
): RoomAgentRoundEntry | null {
  return buildRoomAgentRoundEntry(
    buildRoomAgentRoundIndex(messages, pendingSlots),
    agentId,
  );
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
