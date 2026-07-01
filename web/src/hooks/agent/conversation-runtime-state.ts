import {
  AssistantMessageStatus,
  RoundLifecycleStatus,
} from "@/types";
import type { AgentConversationRuntimeSnapshot } from "./agent-conversation-runtime-machine";

export function are_runtime_snapshots_equal(
  left: AgentConversationRuntimeSnapshot,
  right: AgentConversationRuntimeSnapshot,
): boolean {
  if (
    left.phase !== right.phase ||
    left.pending_permission_count !== right.pending_permission_count ||
    left.is_loading !== right.is_loading
  ) {
    return false;
  }

  const are_string_arrays_equal = (lhs: string[], rhs: string[]): boolean => {
    if (lhs.length !== rhs.length) {
      return false;
    }

    for (let index = 0; index < lhs.length; index += 1) {
      if (lhs[index] !== rhs[index]) {
        return false;
      }
    }
    return true;
  };

  if (
    !are_string_arrays_equal(left.sending_round_ids, right.sending_round_ids) ||
    !are_string_arrays_equal(left.running_round_ids, right.running_round_ids) ||
    !are_string_arrays_equal(
      left.terminal_round_ids,
      right.terminal_round_ids,
    ) ||
    !are_string_arrays_equal(left.live_round_ids, right.live_round_ids)
  ) {
    return false;
  }

  const left_message_ids = Object.keys(left.active_messages);
  const right_message_ids = Object.keys(right.active_messages);
  if (!are_string_arrays_equal(left_message_ids, right_message_ids)) {
    return false;
  }

  for (const message_id of left_message_ids) {
    const left_tracker = left.active_messages[message_id];
    const right_tracker = right.active_messages[message_id];
    if (
      !right_tracker ||
      left_tracker.round_id !== right_tracker.round_id ||
      left_tracker.status !== right_tracker.status
    ) {
      return false;
    }
  }

  return true;
}

export function matches_round_lifecycle(
  round_id: string,
  target_round_id: string,
): boolean {
  return (
    round_id === target_round_id || round_id.startsWith(`${target_round_id}:`)
  );
}

export function get_terminal_message_status(
  status: RoundLifecycleStatus,
): AssistantMessageStatus {
  if (status === "interrupted") {
    return "cancelled";
  }
  if (status === "error") {
    return "error";
  }
  return "done";
}
