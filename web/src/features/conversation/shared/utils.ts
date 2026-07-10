import {
  AssistantMessage,
  Message,
  ResultSummary,
  RoomPendingAgentSlotState,
} from "@/types/conversation/message";
import { PendingPermission } from "@/types/conversation/permission";
import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
export { isAutomationTriggerUserMessage as is_automation_trigger_user_message } from "@/types/conversation/automation-message";

/** 将消息按 root round_id 分组。round_id 由后端保证存在且为 root round。 */
export function groupMessagesByRound(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();
  for (const message of messages) {
    const roundId = message.round_id;
    if (!roundId) {
      continue;
    }
    if (!groups.has(roundId)) {
      groups.set(roundId, []);
    }
    groups.get(roundId)!.push(message);
  }
  return groups;
}

/** Room 时间线分组：消息自带 root round_id，直接分组。 */
export function groupRoomMessagesByRound(messages: Message[]): Map<string, Message[]> {
  return groupMessagesByRound(messages);
}

/** Room 权限请求分组：按显式 round_id 归并，供主时间线与 Thread 共用。 */
export function groupRoomPendingPermissionsByRound(
  pendingPermissions: PendingPermission[],
): Map<string, PendingPermission[]> {
  const groups = new Map<string, PendingPermission[]>();

  for (const permission of pendingPermissions) {
    const roundId = permission.round_id;
    if (!roundId) {
      continue;
    }
    if (!groups.has(roundId)) {
      groups.set(roundId, []);
    }
    groups.get(roundId)!.push(permission);
  }

  return groups;
}

// ── 多 Agent 轮次工具函数 ──────────────────────────────────────────────

/** 聚合状态：单个 Agent 在某轮中的整体状态 */
export type AgentRoundStatus = "pending" | "streaming" | "done" | "error" | "cancelled";

/** Room 中单个 Agent 在某轮里的聚合结果。 */
export interface RoomAgentRoundEntry {
  agent_id: string;
  assistant_messages: AssistantMessage[];
  result_summary?: ResultSummary;
  pending_slot?: RoomPendingAgentSlotState;
  status: AgentRoundStatus;
  timestamp: number;
}

/** 判断一轮 Room 消息是否已经出现可归属到 Agent 的回复。 */
export function hasRoomAgentRoundEntries(
  messages: Message[],
  pendingSlots: RoomPendingAgentSlotState[] = [],
): boolean {
  return pendingSlots.length > 0 || messages.some((message) => (
    Boolean(message.agent_id) && message.role === "assistant"
  ));
}

/** 将一轮消息按 agentId 分组，仅分组 assistant 消息 */
function groupRoundByAgent(messages: Message[]): Map<string, AssistantMessage[]> {
  const groups = new Map<string, AssistantMessage[]>();
  for (const msg of messages) {
    if (msg.role !== "assistant" || !msg.agent_id) continue;
    const existing = groups.get(msg.agent_id);
    if (existing) {
      existing.push(msg as AssistantMessage);
    } else {
      groups.set(msg.agent_id, [msg as AssistantMessage]);
    }
  }
  return groups;
}

function buildResultSummaryMap(messages: Message[]): Map<string, ResultSummary> {
  const summaryMap = new Map<string, ResultSummary>();
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role !== "assistant" || !message.agent_id || summaryMap.has(message.agent_id)) {
      continue;
    }
    const assistant = message as AssistantMessage;
    if (!assistant.result_summary) {
      continue;
    }
    summaryMap.set(message.agent_id, assistant.result_summary);
  }
  return summaryMap;
}

/** 将当前主 round 下的 pending slot 按 agentId 索引。 */
function buildPendingSlotMap(
  pendingSlots: RoomPendingAgentSlotState[],
): Map<string, RoomPendingAgentSlotState> {
  const slotMap = new Map<string, RoomPendingAgentSlotState>();
  for (const slot of pendingSlots) {
    slotMap.set(slot.agent_id, slot);
  }
  return slotMap;
}

/** 从一组 assistant 消息中推导该 Agent 的聚合状态 */
function getAgentRoundStatus(
  messages: AssistantMessage[],
  resultSummary?: ResultSummary | null,
  pendingSlot?: RoomPendingAgentSlotState | null,
): AgentRoundStatus {
  if (resultSummary) {
    if (resultSummary.subtype === "error" || resultSummary.is_error) {
      return "error";
    }
    if (resultSummary.subtype === "interrupted") {
      return "cancelled";
    }
    return "done";
  }

  if (pendingSlot?.status === "error") {
    return "error";
  }
  if (pendingSlot?.status === "cancelled") {
    return "cancelled";
  }
  if (pendingSlot?.status === "streaming") {
    return "streaming";
  }
  if (pendingSlot?.status === "pending") {
    return "pending";
  }

  if (messages.length === 0) return "pending";

  let hasStreaming = false;
  let hasPending = false;
  let hasError = false;
  let hasCancelled = false;
  let hasDone = false;

  for (const msg of messages) {
    const status = msg.stream_status;
    if (status === "streaming") hasStreaming = true;
    else if (status === "pending") hasPending = true;
    else if (status === "error") hasError = true;
    else if (status === "cancelled") hasCancelled = true;
    else if (status === "done" || Boolean(msg.stop_reason)) hasDone = true;
  }

  // 优先级：streaming > pending > error > cancelled > done
  if (hasStreaming) return "streaming";
  if (hasPending) return "pending";
  if (hasError) return "error";
  if (hasCancelled) return "cancelled";
  if (hasDone) return "done";

  // Room 的执行态必须由 pending slot 或 resultSummary 驱动。
  // 仅凭“历史里留着 assistant 过程消息”不能继续判成 streaming，
  // 但如果 assistant 本身已经明确收口为 done，则仍应视为完成，
  // 这样无独立结果消息的正常结束轮次才能正确回退显示最终 assistant。
  return "cancelled";
}

/** 判断某个 Agent 子轮次是否仍在执行。 */
export function isAgentRoundActive(status: AgentRoundStatus): boolean {
  return status === "pending" || status === "streaming";
}

/** 计算 Agent 回复在时间线中的排序时间，优先使用 result 的完成时间。 */
function getAgentRoundTimestamp(
  messages: AssistantMessage[],
  resultSummary?: ResultSummary | null,
  pendingSlot?: RoomPendingAgentSlotState | null,
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

  if (pendingSlot?.timestamp) {
    return pendingSlot.timestamp;
  }

  return 0;
}

/** 构造一轮中所有 Agent 的聚合回复，用于主时间线和 Thread 共用。 */
export function buildRoomAgentRoundEntries(
  messages: Message[],
  pendingSlots: RoomPendingAgentSlotState[] = [],
): RoomAgentRoundEntry[] {
  const summaryMap = buildResultSummaryMap(messages);
  const agentGroups = groupRoundByAgent(messages);
  const pendingSlotMap = buildPendingSlotMap(pendingSlots);
  const agentIds = new Set<string>([
    ...agentGroups.keys(),
    ...summaryMap.keys(),
    ...pendingSlotMap.keys(),
  ]);

  return Array.from(agentIds).map((agentId) => {
    const assistantMessages = agentGroups.get(agentId) ?? [];
    const resultSummary = summaryMap.get(agentId);
    const pendingSlot = pendingSlotMap.get(agentId);

    return {
      agent_id: agentId,
      assistant_messages: assistantMessages,
      result_summary: resultSummary,
      pending_slot: pendingSlot,
      status: getAgentRoundStatus(assistantMessages, resultSummary, pendingSlot),
      timestamp: getAgentRoundTimestamp(assistantMessages, resultSummary, pendingSlot),
    };
  });
}

/** 读取某轮某个 Agent 的聚合回复。 */
export function getRoomAgentRoundEntry(
  messages: Message[],
  agentId: string,
  pendingSlots: RoomPendingAgentSlotState[] = [],
): RoomAgentRoundEntry | null {
  const summaryMap = buildResultSummaryMap(messages);
  const agentGroups = groupRoundByAgent(messages);
  const assistantMessages = agentGroups.get(agentId) ?? [];
  const resultSummary = summaryMap.get(agentId);
  const pendingSlot = pendingSlots.find((slot) => slot.agent_id === agentId);

  if (assistantMessages.length === 0 && !resultSummary && !pendingSlot) {
    return null;
  }

  return {
    agent_id: agentId,
    assistant_messages: assistantMessages,
    result_summary: resultSummary,
    pending_slot: pendingSlot,
    status: getAgentRoundStatus(assistantMessages, resultSummary, pendingSlot),
    timestamp: getAgentRoundTimestamp(assistantMessages, resultSummary, pendingSlot),
  };
}

/** 将 Room 前端占位槽位按 root round_id 分组。 */
export function groupRoomPendingSlotsByRound(
  pendingSlots: RoomPendingAgentSlotState[],
): Map<string, RoomPendingAgentSlotState[]> {
  const groups = new Map<string, RoomPendingAgentSlotState[]>();

  for (const slot of pendingSlots) {
    const roundId = slot.round_id;
    if (!groups.has(roundId)) {
      groups.set(roundId, []);
    }
    groups.get(roundId)!.push(slot);
  }

  return groups;
}

/** 过滤出 Thread 需要展示的用户消息和目标 Agent 的执行链。 */
export function getRoomThreadMessages(messages: Message[], agentId: string): Message[] {
  return messages.filter((message) => (
    (message.role === "user" && !isAutomationTriggerUserMessage(message)) ||
    (
      message.role === "system" &&
      message.agent_id === agentId &&
      message.metadata?.subtype === "guided_input"
    ) ||
    // Thread 只看过程，不展示 result。
    // 最终结果只留在 Room 主时间线，避免中间 assistant 被误当成最终回答。
    (message.agent_id === agentId && message.role === "assistant")
  ));
}

function normalizePreviewText(text: string, maxLength: number): string {
  const normalizedText = text.replace(/\s+/g, " ").trim();
  if (!normalizedText) {
    return "";
  }

  return normalizedText.length > maxLength
    ? normalizedText.slice(0, maxLength) + "…"
    : normalizedText;
}

/** 从 assistant 消息中提取最新的文本/思路预览（截取前 80 字符） */
export function extractAgentPreviewText(messages: AssistantMessage[], maxLength = 80): string {
  // Room 主时间线的占位摘要应该跟随“最新一段 assistant 完整消息”推进，
  // 而不是永远停在第一段文本上。这里只看 text / thinking，忽略工具块。
  for (let messageIndex = messages.length - 1; messageIndex >= 0; messageIndex -= 1) {
    const message = messages[messageIndex];
    if (!Array.isArray(message.content)) {
      continue;
    }

    for (let blockIndex = message.content.length - 1; blockIndex >= 0; blockIndex -= 1) {
      const block = message.content[blockIndex];

      if (block.type === "text") {
        const preview = normalizePreviewText(block.text, maxLength);
        if (preview) {
          return preview;
        }
        continue;
      }

      if (block.type === "thinking") {
        const preview = normalizePreviewText(block.thinking, maxLength);
        if (preview) {
          return preview;
        }
      }
    }
  }

  return "";
}

/** 获取最近一条 assistant/result 消息的时间戳 */
export function getLatestReplyTimestamp(messages: Message[]): number | null {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg.role !== "assistant") continue;
    const assistant = msg as AssistantMessage;
    const timestamp = assistant.result_summary?.timestamp ?? assistant.timestamp;
    if (Number.isFinite(timestamp) && timestamp > 0) return timestamp;
  }
  const last = messages[messages.length - 1];
  if (last && Number.isFinite(last.timestamp) && last.timestamp > 0) return last.timestamp;
  return null;
}

export interface ConversationActivitySnapshot {
  scope_key: string;
  latest_reply_timestamp: number | null;
}

export function buildConversationActivitySnapshot(
  scopeKey: string,
  latestReplyTimestamp: number | null,
): ConversationActivitySnapshot {
  return {
    scope_key: scopeKey,
    latest_reply_timestamp: latestReplyTimestamp,
  };
}

/** 历史加载只建立基线；只有同一会话出现更新回复时才刷新活跃时间。 */
export function shouldEmitConversationActivity(
  previous: ConversationActivitySnapshot | null,
  scopeKey: string,
  latestReplyTimestamp: number | null,
): boolean {
  return Boolean(
    latestReplyTimestamp &&
      previous?.scope_key === scopeKey &&
      latestReplyTimestamp > (previous.latest_reply_timestamp ?? 0),
  );
}
