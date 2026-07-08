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
import { getTerminalMessageStatus } from "./conversation-runtime-state";
import { isEphemeralMessage } from "./conversation-volatile-snapshot";

export function filterRoundPendingAgentSlots(
  slots: RoomPendingAgentSlotState[],
  roundId: string,
): RoomPendingAgentSlotState[] {
  return slots.filter((slot) => slot.round_id !== roundId);
}

export function filterAgentRoundPendingAgentSlots(
  slots: RoomPendingAgentSlotState[],
  agentRoundId: string,
): RoomPendingAgentSlotState[] {
  return slots.filter((slot) => slot.agent_round_id !== agentRoundId);
}

export function filterRoundPendingPermissions(
  permissions: PendingPermission[],
  roundId: string,
): PendingPermission[] {
  return permissions.filter((permission) => {
    if (!permission.round_id) {
      return true;
    }
    return permission.round_id !== roundId;
  });
}

export function removeFailedOutboundUserMessage(
  messages: Message[],
  clientMessageId: string,
): Message[] {
  return messages.filter(
    (message) =>
      !(message.role === "user" && message.message_id === clientMessageId),
  );
}

/** ack 后把 optimistic user message 替换成 canonical id。 */
export function replaceOptimisticUserMessage(
  messages: Message[],
  clientMessageId: string,
  userMessageId: string,
  roundId: string,
): Message[] {
  let hasChanges = false;
  const next = messages.map((message) => {
    if (message.role !== "user" || message.message_id !== clientMessageId) {
      return message;
    }
    hasChanges = true;
    return {
      ...message,
      message_id: userMessageId,
      round_id: roundId,
    };
  });
  return hasChanges ? next : messages;
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
  _chatType: AgentConversationChatType,
): Message[] {
  const terminalRoundSet = new Set(terminalRoundIds);
  const isTerminalRound = (roundId: string) => terminalRoundSet.has(roundId);

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
  const preservedSlots = slots.filter((slot) => slot.round_id !== ack.round_id);
  const nextSlots = (ack.pending ?? []).map((slot) => ({
    agent_id: slot.agent_id,
    agent_round_id: slot.agent_round_id,
    msg_id: slot.msg_id,
    round_id: ack.round_id,
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
    if (message.round_id === roundId && isEphemeralMessage(message)) {
      hasChanges = true;
      continue;
    }
    if (message.role !== "assistant") {
      nextMessages.push(message);
      continue;
    }
    if (message.round_id !== roundId) {
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
