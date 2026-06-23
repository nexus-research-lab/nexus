import type {
  AssistantMessage,
  AssistantMessageStatus,
  ChatAckData,
  Message,
  RoomPendingAgentSlotState,
  RoundLifecycleStatus,
} from "@/types";
import type { AgentConversationChatType } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/permission";
import {
  get_terminal_message_status,
  matches_round_lifecycle,
} from "./conversation-runtime-state";
import { is_ephemeral_message } from "./conversation-volatile-snapshot";

export function filter_round_pending_agent_slots(
  slots: RoomPendingAgentSlotState[],
  round_id: string,
): RoomPendingAgentSlotState[] {
  return slots.filter(
    (slot) => !matches_round_lifecycle(slot.round_id, round_id),
  );
}

export function filter_round_pending_permissions(
  permissions: PendingPermission[],
  round_id: string,
): PendingPermission[] {
  return permissions.filter((permission) => {
    if (!permission.caused_by) {
      return true;
    }
    return !matches_round_lifecycle(permission.caused_by, round_id);
  });
}

export function remove_failed_outbound_user_message(
  messages: Message[],
  round_id: string,
): Message[] {
  return messages.filter(
    (message) =>
      !(
        message.role === "user" &&
        message.message_id === round_id &&
        message.round_id === round_id
      ),
  );
}

export function cancel_running_agent_slots(
  slots: RoomPendingAgentSlotState[],
): RoomPendingAgentSlotState[] {
  return slots.map((slot) =>
    slot.status === "cancelled" || slot.status === "error"
      ? slot
      : {
          ...slot,
          status: "cancelled",
        },
  );
}

export function reconcile_stopped_session_messages(
  messages: Message[],
  terminal_round_ids: string[],
  chat_type: AgentConversationChatType,
): Message[] {
  const terminal_round_set = new Set(terminal_round_ids);
  const is_terminal_round = (round_id: string) => {
    if (terminal_round_set.has(round_id)) {
      return true;
    }
    if (chat_type !== "group") {
      return false;
    }
    for (const terminal_round_id of terminal_round_set) {
      if (round_id.startsWith(`${terminal_round_id}:`)) {
        return true;
      }
    }
    return false;
  };

  let has_changes = false;
  const next_messages: Message[] = [];
  for (const message of messages) {
    if (is_ephemeral_message(message)) {
      has_changes = true;
      continue;
    }
    if (message.role !== "assistant") {
      next_messages.push(message);
      continue;
    }
    if (is_terminal_round(message.round_id)) {
      next_messages.push(message);
      continue;
    }
    if (
      message.stop_reason ||
      message.stream_status === "done" ||
      message.stream_status === "cancelled" ||
      message.stream_status === "error"
    ) {
      next_messages.push(message);
      continue;
    }
    has_changes = true;
    next_messages.push({
      ...message,
      stream_status: "cancelled" as const,
    });
  }
  return has_changes ? next_messages : messages;
}

export function update_assistant_message_status(
  messages: Message[],
  msg_id: string,
  status: AssistantMessageStatus,
): Message[] {
  return messages.map((message) =>
    message.message_id === msg_id && message.role === "assistant"
      ? { ...(message as AssistantMessage), stream_status: status }
      : message,
  );
}

export function update_pending_agent_slot_status(
  slots: RoomPendingAgentSlotState[],
  msg_id: string,
  status: AssistantMessageStatus,
  round_id?: string | null,
): RoomPendingAgentSlotState[] {
  return slots.map((slot) =>
    slot.msg_id === msg_id
      ? {
          ...slot,
          round_id: round_id ?? slot.round_id,
          status,
        }
      : slot,
  );
}

export function merge_chat_ack_pending_slots(
  slots: RoomPendingAgentSlotState[],
  ack: ChatAckData,
): RoomPendingAgentSlotState[] {
  const pending_count = ack.pending?.length ?? 0;
  const preserved_slots = slots.filter((slot) => {
    const base_round_id = slot.round_id.split(":", 1)[0];
    return base_round_id !== ack.round_id;
  });
  const next_slots = (ack.pending ?? []).map((slot) => ({
    agent_id: slot.agent_id,
    msg_id: slot.msg_id,
    round_id:
      slot.round_id ||
      (pending_count > 1
        ? `${ack.round_id}:${slot.agent_id}`
        : ack.round_id),
    status: (slot.status ?? "pending") as AssistantMessageStatus,
    timestamp: slot.timestamp ?? Date.now(),
  }));
  return [...preserved_slots, ...next_slots];
}

export function apply_terminal_round_message_status(
  messages: Message[],
  round_id: string,
  status: RoundLifecycleStatus,
): Message[] {
  const terminal_status = get_terminal_message_status(status);
  let has_changes = false;
  const next_messages: Message[] = [];

  for (const message of messages) {
    if (
      matches_round_lifecycle(message.round_id, round_id) &&
      is_ephemeral_message(message)
    ) {
      has_changes = true;
      continue;
    }
    if (message.role !== "assistant") {
      next_messages.push(message);
      continue;
    }
    if (!matches_round_lifecycle(message.round_id, round_id)) {
      next_messages.push(message);
      continue;
    }
    if (
      message.stream_status === terminal_status ||
      message.stream_status === "cancelled" ||
      message.stream_status === "error" ||
      message.stream_status === "done"
    ) {
      next_messages.push(message);
      continue;
    }
    has_changes = true;
    next_messages.push({
      ...message,
      stream_status: terminal_status,
    });
  }
  return has_changes ? next_messages : messages;
}
