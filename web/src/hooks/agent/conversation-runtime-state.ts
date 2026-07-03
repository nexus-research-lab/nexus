import {
  AssistantMessageStatus,
  RoundLifecycleStatus,
} from "@/types";
import type { AgentConversationRuntimeSnapshot } from "./agent-conversation-runtime-machine";

export function areRuntimeSnapshotsEqual(
  left: AgentConversationRuntimeSnapshot,
  right: AgentConversationRuntimeSnapshot,
): boolean {
  if (
    left.phase !== right.phase ||
    left.pendingPermissionCount !== right.pendingPermissionCount ||
    left.isLoading !== right.isLoading
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
    !areStringArraysEqual(left.sendingRoundIds, right.sendingRoundIds) ||
    !areStringArraysEqual(left.runningRoundIds, right.runningRoundIds) ||
    !areStringArraysEqual(
      left.terminalRoundIds,
      right.terminalRoundIds,
    ) ||
    !areStringArraysEqual(left.liveRoundIds, right.liveRoundIds)
  ) {
    return false;
  }

  const leftMessageIds = Object.keys(left.activeMessages);
  const rightMessageIds = Object.keys(right.activeMessages);
  if (!areStringArraysEqual(leftMessageIds, rightMessageIds)) {
    return false;
  }

  for (const messageId of leftMessageIds) {
    const leftTracker = left.activeMessages[messageId];
    const rightTracker = right.activeMessages[messageId];
    if (
      !rightTracker ||
      leftTracker.roundId !== rightTracker.roundId ||
      leftTracker.status !== rightTracker.status
    ) {
      return false;
    }
  }

  return true;
}

export function matchesRoundLifecycle(
  roundId: string,
  targetRoundId: string,
): boolean {
  return (
    roundId === targetRoundId || roundId.startsWith(`${targetRoundId}:`)
  );
}

export function getTerminalMessageStatus(
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
