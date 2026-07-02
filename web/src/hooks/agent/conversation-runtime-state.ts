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

  const areStringArraysEqual = (lhs: string[], rhs: string[]): boolean => {
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
    !areStringArraysEqual(left.sending_round_ids, right.sending_round_ids) ||
    !areStringArraysEqual(left.running_round_ids, right.running_round_ids) ||
    !areStringArraysEqual(
      left.terminal_round_ids,
      right.terminal_round_ids,
    ) ||
    !areStringArraysEqual(left.live_round_ids, right.live_round_ids)
  ) {
    return false;
  }

  const leftMessageIds = Object.keys(left.active_messages);
  const rightMessageIds = Object.keys(right.active_messages);
  if (!areStringArraysEqual(leftMessageIds, rightMessageIds)) {
    return false;
  }

  for (const messageId of leftMessageIds) {
    const leftTracker = left.active_messages[messageId];
    const rightTracker = right.active_messages[messageId];
    if (
      !rightTracker ||
      leftTracker.round_id !== rightTracker.round_id ||
      leftTracker.status !== rightTracker.status
    ) {
      return false;
    }
  }

  return true;
}

export function matches_round_lifecycle(
  roundId: string,
  targetRoundId: string,
): boolean {
  return (
    roundId === targetRoundId || roundId.startsWith(`${targetRoundId}:`)
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
