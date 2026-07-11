import type {
  AssistantMessageStatus,
  Message,
  RoundLifecycleStatus,
} from "@/types";
import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";

export interface AgentConversationRuntimeSnapshot {
  phase: AgentConversationRuntimePhase;
  terminalRoundIds: string[];
  liveRoundIds: string[];
  isLoading: boolean;
}

export function isEphemeralMessage(message: Message): boolean {
  return message.delivery_mode === "ephemeral";
}

function areStringArraysEqual(left: string[], right: string[]): boolean {
  return left.length === right.length
    && left.every((value, index) => value === right[index]);
}

export function areRuntimeSnapshotsEqual(
  left: AgentConversationRuntimeSnapshot,
  right: AgentConversationRuntimeSnapshot,
): boolean {
  if (
    left.phase !== right.phase ||
    left.isLoading !== right.isLoading
  ) {
    return false;
  }
  return areStringArraysEqual(left.terminalRoundIds, right.terminalRoundIds)
    && areStringArraysEqual(left.liveRoundIds, right.liveRoundIds);
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
