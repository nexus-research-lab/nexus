import type {
  AssistantMessageStatus,
  Message,
  RoomPendingAgentSlotState,
} from "@/types";

import {
  isEphemeralMessage,
  type AgentConversationRuntimeSnapshot,
} from "../model/conversation-runtime-state";

export interface VolatileConversationSnapshot {
  messages: Message[];
  pending_agent_slots: RoomPendingAgentSlotState[];
  updated_at: number;
}

function isTerminalSlotStatus(status: AssistantMessageStatus): boolean {
  return status === "done" || status === "cancelled" || status === "error";
}

export function mergePendingAgentSlots(
  restoredSlots: RoomPendingAgentSlotState[],
  currentSlots: RoomPendingAgentSlotState[],
): RoomPendingAgentSlotState[] {
  if (restoredSlots.length === 0) {
    return currentSlots;
  }

  const mergedSlots = new Map<string, RoomPendingAgentSlotState>();
  for (const slot of restoredSlots) {
    mergedSlots.set(slot.msg_id, slot);
  }
  for (const slot of currentSlots) {
    mergedSlots.set(slot.msg_id, slot);
  }
  return Array.from(mergedSlots.values());
}

export function buildVolatileConversationSnapshot(
  messages: Message[],
  runtimeSnapshot: AgentConversationRuntimeSnapshot,
  pendingAgentSlots: RoomPendingAgentSlotState[],
): VolatileConversationSnapshot | null {
  const activeRoundIds = new Set(runtimeSnapshot.liveRoundIds);

  for (const slot of pendingAgentSlots) {
    if (!isTerminalSlotStatus(slot.status)) {
      activeRoundIds.add(slot.round_id);
    }
  }

  if (activeRoundIds.size === 0) {
    return null;
  }

  const volatileMessages = messages.filter((message) => {
    if (isEphemeralMessage(message)) {
      return false;
    }
    if (activeRoundIds.has(message.round_id)) {
      return true;
    }
    return message.role === "assistant"
      && !isTerminalSlotStatus(message.stream_status ?? "streaming");
  });
  const volatileSlots = pendingAgentSlots.filter(
    (slot) => !isTerminalSlotStatus(slot.status),
  );

  if (volatileMessages.length === 0 && volatileSlots.length === 0) {
    return null;
  }

  return {
    messages: volatileMessages,
    pending_agent_slots: volatileSlots,
    updated_at: Date.now(),
  };
}
