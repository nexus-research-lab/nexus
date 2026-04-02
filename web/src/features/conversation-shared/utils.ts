import { AssistantMessage, Message, ResultMessage } from "@/types/message";

/** 将消息按 round_id 分组 */
export function groupMessagesByRound(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();
  for (const message of messages) {
    const round_id = message.round_id || message.message_id;
    if (!groups.has(round_id)) {
      groups.set(round_id, []);
    }
    groups.get(round_id)!.push(message);
  }
  return groups;
}

/**
 * Room 模式下使用 `原始用户 round_id:agent_id` 作为 agent 子轮次。
 * 前端时间线需要把它重新折叠回用户发起的主 round_id，否则同一轮会被拆成多段。
 */
export function getRoomBaseRoundId(round_id: string, agent_id?: string | null): string {
  if (!round_id) {
    return round_id;
  }

  if (agent_id) {
    const suffix = `:${agent_id}`;
    if (round_id.endsWith(suffix)) {
      return round_id.slice(0, -suffix.length);
    }
  }

  return round_id;
}

/** Room 时间线分组：将多 Agent 子轮次归并回同一条用户轮次。 */
export function groupRoomMessagesByRound(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();

  for (const message of messages) {
    const round_id = getRoomBaseRoundId(message.round_id || message.message_id, message.agent_id);
    if (!groups.has(round_id)) {
      groups.set(round_id, []);
    }
    groups.get(round_id)!.push(message);
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
  result_message?: ResultMessage;
  status: AgentRoundStatus;
  timestamp: number;
}

/** 判断一个轮次是否包含多个 Agent 的 assistant 消息 */
export function isMultiAgentRound(messages: Message[]): boolean {
  const agent_ids = new Set<string>();
  for (const msg of messages) {
    if (msg.role === "assistant" && msg.agent_id) {
      agent_ids.add(msg.agent_id);
      if (agent_ids.size > 1) return true;
    }
  }
  return false;
}

/** 判断一轮 Room 消息是否已经出现可归属到 Agent 的回复。 */
export function hasRoomAgentRoundEntries(messages: Message[]): boolean {
  return messages.some((message) => (
    Boolean(message.agent_id) &&
    (message.role === "assistant" || message.role === "result")
  ));
}

/** 将一轮消息按 agent_id 分组，仅分组 assistant 消息 */
export function groupRoundByAgent(messages: Message[]): Map<string, AssistantMessage[]> {
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

function buildResultMessageMap(messages: Message[]): Map<string, ResultMessage> {
  const result_map = new Map<string, ResultMessage>();
  for (const message of messages) {
    if (message.role === "result" && message.agent_id) {
      result_map.set(message.agent_id, message as ResultMessage);
    }
  }
  return result_map;
}

/** 从一组 assistant 消息中推导该 Agent 的聚合状态 */
export function getAgentRoundStatus(
  messages: AssistantMessage[],
  result_message?: ResultMessage | null,
): AgentRoundStatus {
  if (result_message) {
    if (result_message.subtype === "error" || result_message.is_error) {
      return "error";
    }
    if (result_message.subtype === "interrupted") {
      return "cancelled";
    }
    return "done";
  }

  if (messages.length === 0) return "pending";

  let has_streaming = false;
  let has_pending = false;
  let has_error = false;
  let has_cancelled = false;

  for (const msg of messages) {
    const status = msg.stream_status;
    if (status === "streaming") has_streaming = true;
    else if (status === "pending") has_pending = true;
    else if (status === "error") has_error = true;
    else if (status === "cancelled") has_cancelled = true;
  }

  // 优先级：streaming > pending > error > cancelled > done
  if (has_streaming) return "streaming";
  if (has_pending) return "pending";
  if (has_error) return "error";
  if (has_cancelled) return "cancelled";
  return "done";
}

/** 判断某个 Agent 子轮次是否仍在执行。 */
export function isAgentRoundActive(status: AgentRoundStatus): boolean {
  return status === "pending" || status === "streaming";
}

/** 计算 Agent 回复在时间线中的排序时间，优先使用 result 的完成时间。 */
export function getAgentRoundTimestamp(
  messages: AssistantMessage[],
  result_message?: ResultMessage | null,
): number {
  if (result_message?.timestamp) {
    return result_message.timestamp;
  }

  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const timestamp = messages[index]?.timestamp;
    if (timestamp) {
      return timestamp;
    }
  }

  return 0;
}

/** 构造一轮中所有 Agent 的聚合回复，用于主时间线和 Thread 共用。 */
export function buildRoomAgentRoundEntries(messages: Message[]): RoomAgentRoundEntry[] {
  const result_map = buildResultMessageMap(messages);
  const agent_groups = groupRoundByAgent(messages);
  const agent_ids = new Set<string>([
    ...agent_groups.keys(),
    ...result_map.keys(),
  ]);

  return Array.from(agent_ids).map((agent_id) => {
    const assistant_messages = agent_groups.get(agent_id) ?? [];
    const result_message = result_map.get(agent_id);

    return {
      agent_id,
      assistant_messages,
      result_message,
      status: getAgentRoundStatus(assistant_messages, result_message),
      timestamp: getAgentRoundTimestamp(assistant_messages, result_message),
    };
  });
}

/** 读取某轮某个 Agent 的聚合回复。 */
export function getRoomAgentRoundEntry(
  messages: Message[],
  agent_id: string,
): RoomAgentRoundEntry | null {
  const result_map = buildResultMessageMap(messages);
  const agent_groups = groupRoundByAgent(messages);
  const assistant_messages = agent_groups.get(agent_id) ?? [];
  const result_message = result_map.get(agent_id);

  if (assistant_messages.length === 0 && !result_message) {
    return null;
  }

  return {
    agent_id,
    assistant_messages,
    result_message,
    status: getAgentRoundStatus(assistant_messages, result_message),
    timestamp: getAgentRoundTimestamp(assistant_messages, result_message),
  };
}

/** 过滤出 Thread 需要展示的用户消息和目标 Agent 消息。 */
export function getRoomThreadMessages(messages: Message[], agent_id: string): Message[] {
  return messages.filter((message) => (
    message.role === "user" ||
    (message.agent_id === agent_id && (message.role === "assistant" || message.role === "result"))
  ));
}

/** 从 assistant 消息中提取纯文本预览（截取前 80 字符） */
export function extractAgentPreviewText(messages: AssistantMessage[], max_length = 80): string {
  for (const msg of messages) {
    if (!Array.isArray(msg.content)) continue;
    for (const block of msg.content) {
      if (block.type === "text" && block.text?.trim()) {
        const text = block.text.trim();
        return text.length > max_length ? text.slice(0, max_length) + "…" : text;
      }
    }
  }
  return "";
}

/** 获取最近一条 assistant/result 消息的时间戳 */
export function get_latest_reply_timestamp(messages: Message[]): number | null {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg.role !== "assistant" && msg.role !== "result") continue;
    if (Number.isFinite(msg.timestamp) && msg.timestamp > 0) return msg.timestamp;
  }
  const last = messages[messages.length - 1];
  if (last && Number.isFinite(last.timestamp) && last.timestamp > 0) return last.timestamp;
  return null;
}
