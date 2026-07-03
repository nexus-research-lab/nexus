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
  getTerminalMessageStatus,
  matchesRoundLifecycle,
} from "./conversation-runtime-state";
import { isEphemeralMessage } from "./conversation-volatile-snapshot";

export function filterRoundPendingAgentSlots(
  slots: RoomPendingAgentSlotState[],
  roundId: string,
): RoomPendingAgentSlotState[] {
  return slots.filter(
    (slot) => !matchesRoundLifecycle(slot.round_id, roundId),
  );
}

export function filterRoundPendingPermissions(
  permissions: PendingPermission[],
  roundId: string,
): PendingPermission[] {
  return permissions.filter((permission) => {
    if (!permission.caused_by) {
      return true;
    }
    return !matchesRoundLifecycle(permission.caused_by, roundId);
  });
}

export function removeFailedOutboundUserMessage(
  messages: Message[],
  roundId: string,
): Message[] {
  return messages.filter(
    (message) =>
      !(
        message.role === "user" &&
        message.message_id === roundId &&
        message.round_id === roundId
      ),
  );
}

export function cancelRunningAgentSlots(
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

export function reconcileStoppedSessionMessages(
  messages: Message[],
  terminalRoundIds: string[],
  chatType: AgentConversationChatType,
): Message[] {
  const terminalRoundSet = new Set(terminalRoundIds);
  const isTerminalRound = (roundId: string) => {
    if (terminalRoundSet.has(roundId)) {
      return true;
    }
    if (chatType !== "group") {
      return false;
    }
    for (const terminalRoundId of terminalRoundSet) {
      if (roundId.startsWith(`${terminalRoundId}:`)) {
        return true;
      }
    }
    return false;
  };

  let hasChanges = false;
  const nextMessages: Message[] = [];
  for (const message of messages) {
    if (isEphemeralMessage(message)) {
      hasChanges = true;
      continue;
    }
    if (message.role !== "assistant") {
      nextMessages.push(message);
      continue;
    }
    if (isTerminalRound(message.round_id)) {
      nextMessages.push(message);
      continue;
    }
    if (
      message.stop_reason ||
      message.stream_status === "done" ||
      message.stream_status === "cancelled" ||
      message.stream_status === "error"
    ) {
      nextMessages.push(message);
      continue;
    }
    hasChanges = true;
    nextMessages.push({
      ...message,
      stream_status: "cancelled" as const,
    });
  }
  return hasChanges ? nextMessages : messages;
}

export function updateAssistantMessageStatus(
  messages: Message[],
  msgId: string,
  status: AssistantMessageStatus,
): Message[] {
  return messages.map((message) =>
    message.message_id === msgId && message.role === "assistant"
      ? { ...(message as AssistantMessage), stream_status: status }
      : message,
  );
}

export function updatePendingAgentSlotStatus(
  slots: RoomPendingAgentSlotState[],
  msgId: string,
  status: AssistantMessageStatus,
  roundId?: string | null,
): RoomPendingAgentSlotState[] {
  return slots.map((slot) =>
    slot.msg_id === msgId
      ? {
          ...slot,
          round_id: roundId ?? slot.round_id,
          status,
        }
      : slot,
  );
}

export function mergeChatAckPendingSlots(
  slots: RoomPendingAgentSlotState[],
  ack: ChatAckData,
): RoomPendingAgentSlotState[] {
  const pendingCount = ack.pending?.length ?? 0;
  const preservedSlots = slots.filter((slot) => {
    const baseRoundId = slot.round_id.split(":", 1)[0];
    return baseRoundId !== ack.round_id;
  });
  const nextSlots = (ack.pending ?? []).map((slot) => ({
    agent_id: slot.agent_id,
    msg_id: slot.msg_id,
    round_id:
      slot.round_id ||
      (pendingCount > 1
        ? `${ack.round_id}:${slot.agent_id}`
        : ack.round_id),
    status: (slot.status ?? "pending") as AssistantMessageStatus,
    timestamp: slot.timestamp ?? Date.now(),
  }));
  return [...preservedSlots, ...nextSlots];
}

export function applyTerminalRoundMessageStatus(
  messages: Message[],
  roundId: string,
  status: RoundLifecycleStatus,
): Message[] {
  const terminalStatus = getTerminalMessageStatus(status);
  let hasChanges = false;
  const nextMessages: Message[] = [];

  for (const message of messages) {
    if (
      matchesRoundLifecycle(message.round_id, roundId) &&
      isEphemeralMessage(message)
    ) {
      hasChanges = true;
      continue;
    }
    if (message.role !== "assistant") {
      nextMessages.push(message);
      continue;
    }
    if (!matchesRoundLifecycle(message.round_id, roundId)) {
      nextMessages.push(message);
      continue;
    }
    if (
      message.stream_status === terminalStatus ||
      message.stream_status === "cancelled" ||
      message.stream_status === "error" ||
      message.stream_status === "done"
    ) {
      nextMessages.push(message);
      continue;
    }
    hasChanges = true;
    nextMessages.push({
      ...message,
      stream_status: terminalStatus,
    });
  }
  return hasChanges ? nextMessages : messages;
}
