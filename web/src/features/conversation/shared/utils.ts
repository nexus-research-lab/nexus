import {
  AssistantMessage,
  Message,
  ResultSummary,
  RoomPendingAgentSlotState,
} from "@/types/conversation/message";
import { PendingPermission } from "@/types/conversation/permission";

/** 将消息按 round_id 分组 */
export function group_messages_by_round(messages: Message[]): Map<string, Message[]> {
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
export function get_room_base_round_id(round_id: string, agent_id?: string | null): string {
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
export function group_room_messages_by_round(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();

  for (const message of messages) {
    const round_id = get_room_base_round_id(message.round_id || message.message_id, message.agent_id);
    if (!groups.has(round_id)) {
      groups.set(round_id, []);
    }
    groups.get(round_id)!.push(message);
  }

  return groups;
}

/** Room 权限请求分组：按主 round_id 归并，供主时间线与 Thread 共用。 */
export function group_room_pending_permissions_by_round(
  pending_permissions: PendingPermission[],
): Map<string, PendingPermission[]> {
  const groups = new Map<string, PendingPermission[]>();

  for (const permission of pending_permissions) {
    if (!permission.caused_by) {
      continue;
    }
    const round_id = get_room_base_round_id(permission.caused_by, permission.agent_id);
    if (!groups.has(round_id)) {
      groups.set(round_id, []);
    }
    groups.get(round_id)!.push(permission);
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

/** 判断一个轮次是否包含多个 Agent 的 assistant 消息 */
export function is_multi_agent_round(messages: Message[]): boolean {
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
export function has_room_agent_round_entries(
  messages: Message[],
  pending_slots: RoomPendingAgentSlotState[] = [],
): boolean {
  return pending_slots.length > 0 || messages.some((message) => (
    Boolean(message.agent_id) && message.role === "assistant"
  ));
}

/** 将一轮消息按 agent_id 分组，仅分组 assistant 消息 */
export function group_round_by_agent(messages: Message[]): Map<string, AssistantMessage[]> {
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

function build_result_summary_map(messages: Message[]): Map<string, ResultSummary> {
  const summary_map = new Map<string, ResultSummary>();
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role !== "assistant" || !message.agent_id || summary_map.has(message.agent_id)) {
      continue;
    }
    const assistant = message as AssistantMessage;
    if (!assistant.result_summary) {
      continue;
    }
    summary_map.set(message.agent_id, assistant.result_summary);
  }
  return summary_map;
}

/** 将当前主 round 下的 pending slot 按 agent_id 索引。 */
function build_pending_slot_map(
  pending_slots: RoomPendingAgentSlotState[],
): Map<string, RoomPendingAgentSlotState> {
  const slot_map = new Map<string, RoomPendingAgentSlotState>();
  for (const slot of pending_slots) {
    slot_map.set(slot.agent_id, slot);
  }
  return slot_map;
}

/** 从一组 assistant 消息中推导该 Agent 的聚合状态 */
export function get_agent_round_status(
  messages: AssistantMessage[],
  result_summary?: ResultSummary | null,
  pending_slot?: RoomPendingAgentSlotState | null,
): AgentRoundStatus {
  if (result_summary) {
    if (result_summary.subtype === "error" || result_summary.is_error) {
      return "error";
    }
    if (result_summary.subtype === "interrupted") {
      return "cancelled";
    }
    return "done";
  }

  if (pending_slot?.status === "error") {
    return "error";
  }
  if (pending_slot?.status === "cancelled") {
    return "cancelled";
  }
  if (pending_slot?.status === "streaming") {
    return "streaming";
  }
  if (pending_slot?.status === "pending") {
    return "pending";
  }

  if (messages.length === 0) return "pending";

  let has_streaming = false;
  let has_pending = false;
  let has_error = false;
  let has_cancelled = false;
  let has_done = false;

  for (const msg of messages) {
    const status = msg.stream_status;
    if (status === "streaming") has_streaming = true;
    else if (status === "pending") has_pending = true;
    else if (status === "error") has_error = true;
    else if (status === "cancelled") has_cancelled = true;
    else if (status === "done" || Boolean(msg.stop_reason)) has_done = true;
  }

  // 优先级：streaming > pending > error > cancelled > done
  if (has_streaming) return "streaming";
  if (has_pending) return "pending";
  if (has_error) return "error";
  if (has_cancelled) return "cancelled";
  if (has_done) return "done";

  // Room 的执行态必须由 pending slot 或 result_summary 驱动。
  // 仅凭“历史里留着 assistant 过程消息”不能继续判成 streaming，
  // 但如果 assistant 本身已经明确收口为 done，则仍应视为完成，
  // 这样无独立结果消息的正常结束轮次才能正确回退显示最终 assistant。
  return "cancelled";
}

/** 判断某个 Agent 子轮次是否仍在执行。 */
export function is_agent_round_active(status: AgentRoundStatus): boolean {
  return status === "pending" || status === "streaming";
}

/** 计算 Agent 回复在时间线中的排序时间，优先使用 result 的完成时间。 */
export function get_agent_round_timestamp(
  messages: AssistantMessage[],
  result_summary?: ResultSummary | null,
  pending_slot?: RoomPendingAgentSlotState | null,
): number {
  if (result_summary?.timestamp) {
    return result_summary.timestamp;
  }

  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const timestamp = messages[index]?.timestamp;
    if (timestamp) {
      return timestamp;
    }
  }

  if (pending_slot?.timestamp) {
    return pending_slot.timestamp;
  }

  return 0;
}

/** 构造一轮中所有 Agent 的聚合回复，用于主时间线和 Thread 共用。 */
export function build_room_agent_round_entries(
  messages: Message[],
  pending_slots: RoomPendingAgentSlotState[] = [],
): RoomAgentRoundEntry[] {
  const summary_map = build_result_summary_map(messages);
  const agent_groups = group_round_by_agent(messages);
  const pending_slot_map = build_pending_slot_map(pending_slots);
  const agent_ids = new Set<string>([
    ...agent_groups.keys(),
    ...summary_map.keys(),
    ...pending_slot_map.keys(),
  ]);

  return Array.from(agent_ids).map((agent_id) => {
    const assistant_messages = agent_groups.get(agent_id) ?? [];
    const result_summary = summary_map.get(agent_id);
    const pending_slot = pending_slot_map.get(agent_id);

    return {
      agent_id,
      assistant_messages,
      result_summary,
      pending_slot,
      status: get_agent_round_status(assistant_messages, result_summary, pending_slot),
      timestamp: get_agent_round_timestamp(assistant_messages, result_summary, pending_slot),
    };
  });
}

/** 读取某轮某个 Agent 的聚合回复。 */
export function get_room_agent_round_entry(
  messages: Message[],
  agent_id: string,
  pending_slots: RoomPendingAgentSlotState[] = [],
): RoomAgentRoundEntry | null {
  const summary_map = build_result_summary_map(messages);
  const agent_groups = group_round_by_agent(messages);
  const assistant_messages = agent_groups.get(agent_id) ?? [];
  const result_summary = summary_map.get(agent_id);
  const pending_slot = pending_slots.find((slot) => slot.agent_id === agent_id);

  if (assistant_messages.length === 0 && !result_summary && !pending_slot) {
    return null;
  }

  return {
    agent_id,
    assistant_messages,
    result_summary,
    pending_slot,
    status: get_agent_round_status(assistant_messages, result_summary, pending_slot),
    timestamp: get_agent_round_timestamp(assistant_messages, result_summary, pending_slot),
  };
}

/** 将 Room 前端占位槽位按主 round_id 分组。 */
export function group_room_pending_slots_by_round(
  pending_slots: RoomPendingAgentSlotState[],
): Map<string, RoomPendingAgentSlotState[]> {
  const groups = new Map<string, RoomPendingAgentSlotState[]>();

  for (const slot of pending_slots) {
    const round_id = get_room_base_round_id(slot.round_id, slot.agent_id);
    if (!groups.has(round_id)) {
      groups.set(round_id, []);
    }
    groups.get(round_id)!.push(slot);
  }

  return groups;
}

/** 过滤出 Thread 需要展示的用户消息和目标 Agent 的执行链。 */
export function get_room_thread_messages(messages: Message[], agent_id: string): Message[] {
  return messages.filter((message) => (
    message.role === "user" ||
    (
      message.role === "system" &&
      message.agent_id === agent_id &&
      message.metadata?.subtype === "guided_input"
    ) ||
    // Thread 只看过程，不展示 result。
    // 最终结果只留在 Room 主时间线，避免中间 assistant 被误当成最终回答。
    (message.agent_id === agent_id && message.role === "assistant")
  ));
}

function normalize_preview_text(text: string, max_length: number): string {
  const normalized_text = text.replace(/\s+/g, " ").trim();
  if (!normalized_text) {
    return "";
  }

  return normalized_text.length > max_length
    ? normalized_text.slice(0, max_length) + "…"
    : normalized_text;
}

/** 从 assistant 消息中提取最新的文本/思路预览（截取前 80 字符） */
export function extract_agent_preview_text(messages: AssistantMessage[], max_length = 80): string {
  // Room 主时间线的占位摘要应该跟随“最新一段 assistant 完整消息”推进，
  // 而不是永远停在第一段文本上。这里只看 text / thinking，忽略 tool_* 块。
  for (let message_index = messages.length - 1; message_index >= 0; message_index -= 1) {
    const message = messages[message_index];
    if (!Array.isArray(message.content)) {
      continue;
    }

    for (let block_index = message.content.length - 1; block_index >= 0; block_index -= 1) {
      const block = message.content[block_index];

      if (block.type === "text") {
        const preview = normalize_preview_text(block.text, max_length);
        if (preview) {
          return preview;
        }
        continue;
      }

      if (block.type === "thinking") {
        const preview = normalize_preview_text(block.thinking, max_length);
        if (preview) {
          return preview;
        }
      }
    }
  }

  return "";
}

/** 获取最近一条 assistant/result 消息的时间戳 */
export function get_latest_reply_timestamp(messages: Message[]): number | null {
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
