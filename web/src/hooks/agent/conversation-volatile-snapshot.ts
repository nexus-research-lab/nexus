import {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
  RoomPendingAgentSlotState,
} from "@/types";
import {
  collect_unresolved_tool_use_candidates,
  match_pending_permissions_to_tool_uses,
  PendingPermission,
} from "@/types/conversation/permission";
import { AgentConversationRuntimeSnapshot } from "./agent-conversation-runtime-machine";

interface VolatileConversationSnapshot {
  messages: Message[];
  pending_agent_slots: RoomPendingAgentSlotState[];
  updated_at: number;
}

const VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX =
  "nexus.agent_conversation.volatile";

function is_terminal_slot_status(status: AssistantMessageStatus): boolean {
  return status === "done" || status === "cancelled" || status === "error";
}

function build_volatile_conversation_storage_key(session_key: string): string {
  return `${VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX}:${session_key}`;
}

function get_volatile_conversation_storage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

export function read_volatile_conversation_snapshot(
  session_key: string,
): VolatileConversationSnapshot | null {
  const storage = get_volatile_conversation_storage();
  if (!storage) {
    return null;
  }

  try {
    const raw = storage.getItem(
      build_volatile_conversation_storage_key(session_key),
    );
    if (!raw) {
      return null;
    }

    const parsed = JSON.parse(
      raw,
    ) as Partial<VolatileConversationSnapshot> | null;
    if (!parsed) {
      return null;
    }

    return {
      messages: Array.isArray(parsed.messages)
        ? (parsed.messages as Message[])
        : [],
      pending_agent_slots: Array.isArray(parsed.pending_agent_slots)
        ? (parsed.pending_agent_slots as RoomPendingAgentSlotState[])
        : [],
      updated_at: typeof parsed.updated_at === "number" ? parsed.updated_at : 0,
    };
  } catch (err) {
    console.debug("[conversation] Failed to parse snapshot from sessionStorage:", err);
    return null;
  }
}

export function write_volatile_conversation_snapshot(
  session_key: string,
  snapshot: VolatileConversationSnapshot,
): void {
  const storage = get_volatile_conversation_storage();
  if (!storage) {
    return;
  }

  // 保留最近 200 条消息以防止序列化负载过大。
  const capped: VolatileConversationSnapshot =
    snapshot.messages.length > 200
      ? { ...snapshot, messages: snapshot.messages.slice(-200) }
      : snapshot;

  try {
    storage.setItem(
      build_volatile_conversation_storage_key(session_key),
      JSON.stringify(capped),
    );
  } catch (err) {
    const is_quota =
      err instanceof DOMException &&
      (err.code === 22 || err.name === "QuotaExceededError");
    if (is_quota) {
      console.warn("[conversation] sessionStorage quota exceeded, snapshot not persisted");
    } else {
      console.warn("[conversation] sessionStorage write failed:", err);
    }
  }
}

export function remove_volatile_conversation_snapshot(
  session_key: string,
): void {
  const storage = get_volatile_conversation_storage();
  if (!storage) {
    return;
  }

  try {
    storage.removeItem(build_volatile_conversation_storage_key(session_key));
  } catch {
    // 忽略移除失败
  }
}

export function merge_pending_agent_slots(
  restored_slots: RoomPendingAgentSlotState[],
  current_slots: RoomPendingAgentSlotState[],
): RoomPendingAgentSlotState[] {
  if (restored_slots.length === 0) {
    return current_slots;
  }

  const merged_slots = new Map<string, RoomPendingAgentSlotState>();
  for (const slot of restored_slots) {
    merged_slots.set(slot.msg_id, slot);
  }
  for (const slot of current_slots) {
    merged_slots.set(slot.msg_id, slot);
  }
  return Array.from(merged_slots.values());
}

export function is_ephemeral_message(message: Message): boolean {
  return message.delivery_mode === "ephemeral";
}

export function build_volatile_conversation_snapshot(
  messages: Message[],
  runtime_snapshot: AgentConversationRuntimeSnapshot,
  pending_agent_slots: RoomPendingAgentSlotState[],
): VolatileConversationSnapshot | null {
  const active_round_ids = new Set<string>(runtime_snapshot.live_round_ids);

  for (const slot of pending_agent_slots) {
    if (!is_terminal_slot_status(slot.status)) {
      active_round_ids.add(slot.round_id);
    }
  }

  if (active_round_ids.size === 0) {
    return null;
  }

  const volatile_messages = messages.filter((message) => {
    if (is_ephemeral_message(message)) {
      return false;
    }
    if (active_round_ids.has(message.round_id)) {
      return true;
    }

    return (
      message.role === "assistant" &&
      !is_terminal_slot_status(message.stream_status ?? "streaming")
    );
  });
  const volatile_slots = pending_agent_slots.filter(
    (slot) => !is_terminal_slot_status(slot.status),
  );

  if (volatile_messages.length === 0 && volatile_slots.length === 0) {
    return null;
  }

  return {
    messages: volatile_messages,
    pending_agent_slots: volatile_slots,
    updated_at: Date.now(),
  };
}

export function filter_pending_slots_from_snapshot(
  current_slots: RoomPendingAgentSlotState[],
  messages: Message[],
  is_round_terminal: (round_id: string) => boolean,
): RoomPendingAgentSlotState[] {
  if (current_slots.length === 0) {
    return current_slots;
  }
  const loaded_message_ids = new Set(
    messages
      .filter(
        (message): message is AssistantMessage => message.role === "assistant",
      )
      .map((message) => message.message_id),
  );

  return current_slots.filter(
    (slot) =>
      !is_round_terminal(slot.round_id) && !loaded_message_ids.has(slot.msg_id),
  );
}

export function filter_pending_permissions_from_snapshot(
  current_permissions: PendingPermission[],
  messages: Message[],
  is_round_terminal: (round_id: string) => boolean,
): PendingPermission[] {
  if (current_permissions.length === 0) {
    return current_permissions;
  }
  const loaded_assistant_message_ids = new Set<string>();
  const unresolved_tool_use_candidates =
    collect_unresolved_tool_use_candidates(messages);
  const permission_match_result = match_pending_permissions_to_tool_uses(
    current_permissions,
    unresolved_tool_use_candidates,
  );

  for (const message of messages) {
    if (message.role === "assistant") {
      loaded_assistant_message_ids.add(message.message_id);
    }
  }

  return current_permissions.filter((permission) => {
    if (is_pending_permission_expired(permission)) {
      return false;
    }

    if (permission.caused_by && is_round_terminal(permission.caused_by)) {
      return false;
    }

    if (
      permission_match_result.matched_request_ids.has(permission.request_id)
    ) {
      return true;
    }

    if (!permission.message_id) {
      // 缺少 message_id 的旧权限事件无法做唯一绑定，
      // 快照阶段只能保留，等待明确的 result / reload 收口。
      return true;
    }

    return !loaded_assistant_message_ids.has(permission.message_id);
  });
}

function get_pending_permission_expiration_ms(
  permission: PendingPermission,
): number | null {
  if (!permission.expires_at) {
    return null;
  }
  const expires_at_ms = Date.parse(permission.expires_at);
  return Number.isFinite(expires_at_ms) ? expires_at_ms : null;
}

function is_pending_permission_expired(
  permission: PendingPermission,
  now_ms: number = Date.now(),
): boolean {
  const expires_at_ms = get_pending_permission_expiration_ms(permission);
  return expires_at_ms != null && expires_at_ms <= now_ms;
}

export function prune_expired_pending_permissions(
  current_permissions: PendingPermission[],
  now_ms: number = Date.now(),
): PendingPermission[] {
  if (current_permissions.length === 0) {
    return current_permissions;
  }

  const next_permissions = current_permissions.filter(
    (permission) => !is_pending_permission_expired(permission, now_ms),
  );
  return next_permissions.length === current_permissions.length
    ? current_permissions
    : next_permissions;
}

export function get_next_pending_permission_timeout_ms(
  current_permissions: PendingPermission[],
  now_ms: number = Date.now(),
): number | null {
  let next_timeout_ms: number | null = null;

  for (const permission of current_permissions) {
    const expires_at_ms = get_pending_permission_expiration_ms(permission);
    if (expires_at_ms == null) {
      continue;
    }
    const timeout_ms = Math.max(expires_at_ms - now_ms, 0);
    if (next_timeout_ms == null || timeout_ms < next_timeout_ms) {
      next_timeout_ms = timeout_ms;
    }
  }

  return next_timeout_ms;
}
