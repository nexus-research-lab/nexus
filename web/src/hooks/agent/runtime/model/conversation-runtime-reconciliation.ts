import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
} from "@/types/conversation/message/entity";
import type { ChatAckData } from "@/types/conversation/message/event";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { RoundLifecycleStatus } from "@/types/conversation/message/event";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import {
  getTerminalMessageStatus,
  isEphemeralMessage,
} from "./conversation-runtime-state";

type MessageReconciliationAction =
  | { kind: "keep" }
  | { kind: "remove" }
  | {
      kind: "update_status";
      message: AssistantMessage;
      status: AssistantMessageStatus;
    };

const KEEP_MESSAGE: MessageReconciliationAction = { kind: "keep" };
const REMOVE_MESSAGE: MessageReconciliationAction = { kind: "remove" };
const TERMINAL_ASSISTANT_STATUSES = new Set<AssistantMessageStatus>([
  "cancelled",
  "done",
  "error",
]);

function reconcileMessages(
  messages: Message[],
  resolveAction: (message: Message) => MessageReconciliationAction,
): Message[] {
  let hasChanges = false;
  const nextMessages: Message[] = [];
  for (const message of messages) {
    const action = resolveAction(message);
    if (action.kind === "keep") {
      nextMessages.push(message);
      continue;
    }
    hasChanges = true;
    if (action.kind === "update_status") {
      nextMessages.push({ ...action.message, stream_status: action.status });
    }
  }
  return hasChanges ? nextMessages : messages;
}

function updateAssistantStatus(
  message: AssistantMessage,
  status: AssistantMessageStatus,
): MessageReconciliationAction {
  return { kind: "update_status", message, status };
}

export function filterRoundPendingAgentSlots(
  slots: RoomPendingAgentSlotState[],
  roundId: string,
): RoomPendingAgentSlotState[] {
  return slots.filter((slot) => slot.round_id !== roundId);
}

export function reconcileAgentRoundPendingSlots(
  slots: RoomPendingAgentSlotState[],
  agentRoundId: string,
  isTerminal: boolean,
): RoomPendingAgentSlotState[] {
  if (isTerminal) {
    return slots.filter((slot) => slot.agent_round_id !== agentRoundId);
  }
  return slots.map((slot) => slot.agent_round_id === agentRoundId
    ? { ...slot, status: "streaming" }
    : slot);
}

export function filterPendingSlotsFromSnapshot(
  currentSlots: RoomPendingAgentSlotState[],
  messages: Message[],
  isRoundTerminal: (roundId: string) => boolean,
): RoomPendingAgentSlotState[] {
  if (currentSlots.length === 0) {
    return currentSlots;
  }
  const loadedMessageIds = new Set(
    messages
      .filter(
        (message): message is AssistantMessage => message.role === "assistant",
      )
      .map((message) => message.message_id),
  );
  return currentSlots.filter(
    (slot) => !isRoundTerminal(slot.round_id)
      && !loadedMessageIds.has(slot.msg_id),
  );
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

export function removeRoundMessages(
  messages: Message[],
  roundId: string,
): Message[] {
  const normalizedRoundId = roundId.trim();
  if (!normalizedRoundId) {
    return messages;
  }
  const next = messages.filter(
    (message) => message.round_id !== normalizedRoundId,
  );
  return next.length === messages.length ? messages : next;
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
): Message[] {
  const terminalRoundSet = new Set(terminalRoundIds);
  return reconcileMessages(messages, (message) => {
    if (isEphemeralMessage(message)) {
      return REMOVE_MESSAGE;
    }
    if (
      message.role !== "assistant" ||
      terminalRoundSet.has(message.round_id) ||
      message.stop_reason ||
      TERMINAL_ASSISTANT_STATUSES.has(message.stream_status ?? "pending")
    ) {
      return KEEP_MESSAGE;
    }
    return updateAssistantStatus(message, "cancelled");
  });
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
  const nextSlots = ack.pending.map((slot) => ({
    agent_id: slot.agent_id,
    agent_round_id: slot.agent_round_id,
    msg_id: slot.msg_id,
    round_id: ack.round_id,
    status: slot.status,
    timestamp: slot.timestamp,
  }));
  return [...preservedSlots, ...nextSlots];
}

export function applyTerminalRoundMessageStatus(
  messages: Message[],
  roundId: string,
  status: RoundLifecycleStatus,
): Message[] {
  const terminalStatus = getTerminalMessageStatus(status);
  return reconcileMessages(messages, (message) => {
    if (message.round_id !== roundId) {
      return KEEP_MESSAGE;
    }
    if (isEphemeralMessage(message)) {
      return REMOVE_MESSAGE;
    }
    if (
      message.role !== "assistant" ||
      TERMINAL_ASSISTANT_STATUSES.has(message.stream_status ?? "pending")
    ) {
      return KEEP_MESSAGE;
    }
    return updateAssistantStatus(message, terminalStatus);
  });
}
