import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";

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
  resultSummaries: Map<string, ResultSummary>;
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

function buildResultSummaries(messages: Message[]): Map<string, ResultSummary> {
  const summaries = new Map<string, ResultSummary>();
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (
      message.role !== "assistant" ||
      !message.agent_id ||
      !message.result_summary ||
      summaries.has(message.agent_id)
    ) {
      continue;
    }
    summaries.set(message.agent_id, message.result_summary);
  }
  return summaries;
}

function buildPendingSlots(
  slots: RoomPendingAgentSlotState[],
): Map<string, RoomPendingAgentSlotState> {
  return new Map(slots.map((slot) => [slot.agent_id, slot]));
}

function buildRoomAgentRoundIndex(
  messages: Message[],
  slots: RoomPendingAgentSlotState[],
): RoomAgentRoundIndex {
  const messageGroups = buildMessageGroups(messages);
  const resultSummaries = buildResultSummaries(messages);
  const pendingSlots = buildPendingSlots(slots);
  return {
    agentIds: new Set([
      ...messageGroups.keys(),
      ...resultSummaries.keys(),
      ...pendingSlots.keys(),
    ]),
    messageGroups,
    pendingSlots,
    resultSummaries,
  };
}

function getAgentRoundStatus(
  messages: AssistantMessage[],
  resultSummary?: ResultSummary,
  pendingSlot?: RoomPendingAgentSlotState,
): AgentRoundStatus {
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
  const assistantMessages = index.messageGroups.get(agentId) ?? [];
  const resultSummary = index.resultSummaries.get(agentId);
  const pendingSlot = index.pendingSlots.get(agentId);
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
  const normalizedText = text.replace(/\s+/g, " ").trim();
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
