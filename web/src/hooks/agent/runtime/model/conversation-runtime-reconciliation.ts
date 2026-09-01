/**
 * [INPUT]: WebSocket ack/生命周期事件、已加载消息与本地临时运行态。
 * [OUTPUT]: 对话消息、权限和 Room agent slot 的单调归并结果；终态 slot 保留到消息或 root 收口。
 * [POS]: Agent conversation runtime 的纯状态协调层。
 */
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
const AGENT_ROUND_SLOT_STATUS: Record<
  RoundLifecycleStatus,
  AssistantMessageStatus
> = {
  error: "error",
  finished: "done",
  interrupted: "cancelled",
  running: "streaming",
};

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
  status: RoundLifecycleStatus,
): RoomPendingAgentSlotState[] {
  const nextStatus = AGENT_ROUND_SLOT_STATUS[status];
  return slots.map((slot) => {
    if (slot.agent_round_id !== agentRoundId) {
      return slot;
    }
    // WebSocket 事件可能乱序；终态一旦落地，迟到的 running 不能把槽位复活。
    if (TERMINAL_ASSISTANT_STATUSES.has(slot.status)) {
      return slot;
    }
    return { ...slot, status: nextStatus };
  });
}

/**
 * terminal message/result 已经能稳定投影同一张卡时，移除仅用于填补消息空窗的
 * slot；流式 assistant 到达时仍保留 slot 的 index 与启动时间。
 */
export function reconcilePendingSlotsWithAssistantMessage(
  slots: RoomPendingAgentSlotState[],
  message: AssistantMessage,
): RoomPendingAgentSlotState[] {
  if (!isTerminalAssistantMessage(message)) {
    return slots;
  }
  const agentRoundId = message.agent_round_id?.trim();
  const next = slots.filter((slot) => (
    agentRoundId
      ? slot.agent_round_id !== agentRoundId
      : slot.msg_id !== message.message_id
  ));
  return next.length === slots.length ? slots : next;
}

function isTerminalAssistantMessage(message: AssistantMessage): boolean {
  return Boolean(
    message.result_summary
    || message.is_complete
    || message.stop_reason
    || TERMINAL_ASSISTANT_STATUSES.has(message.stream_status ?? "pending"),
  );
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
  userMessageCommitted: boolean,
  userMessageDeliveryMode?: Message["delivery_mode"],
): Message[] {
  if (
    !userMessageCommitted
    && userMessageDeliveryMode !== "transient"
  ) {
    const next = messages.filter(
      (message) => message.message_id !== clientMessageId,
    );
    return next.length === messages.length ? messages : next;
  }
  const hasCanonicalMessage = messages.some(
    (message) => message.message_id === userMessageId,
  );
  // Room 会先广播 durable user，再返回 ACK；已有 canonical 时移除本地副本，
  // 同时补回只存在于当前页面的 visual identity。
  if (hasCanonicalMessage && clientMessageId !== userMessageId) {
    let hasChanges = false;
    const next = messages.flatMap((message) => {
      if (message.message_id === clientMessageId) {
        hasChanges = true;
        return [];
      }
      if (
        message.role === "user"
        && message.message_id === userMessageId
        && message.client_message_id !== clientMessageId
      ) {
        hasChanges = true;
        return [{
          ...message,
          client_message_id: clientMessageId,
          ...(userMessageDeliveryMode
            ? { delivery_mode: userMessageDeliveryMode }
            : {}),
        }];
      }
      return [message];
    });
    return hasChanges ? next : messages;
  }

  let hasChanges = false;
  const next = messages.map((message) => {
    if (message.role !== "user" || message.message_id !== clientMessageId) {
      return message;
    }
    hasChanges = true;
    return {
      ...message,
      client_message_id: clientMessageId,
      message_id: userMessageId,
      round_id: roundId,
      ...(userMessageDeliveryMode
        ? { delivery_mode: userMessageDeliveryMode }
        : {}),
    };
  });
  return hasChanges ? next : messages;
}

export function cancelRunningAgentSlots(
  slots: RoomPendingAgentSlotState[],
): RoomPendingAgentSlotState[] {
  return slots.map((slot) =>
    TERMINAL_ASSISTANT_STATUSES.has(slot.status)
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
          // ACK / snapshot 决定 slot 的稳定展示 root；stream 生命周期事件
          // 只能推进状态，不能把已展示卡片搬到另一个 feed root。
          round_id: slot.round_id || roundId || "",
          status,
        }
      : slot,
  );
}

export function mergeChatAckPendingSlots(
  slots: RoomPendingAgentSlotState[],
  ack: ChatAckData,
): RoomPendingAgentSlotState[] {
  // 普通 ACK 的空 pending 只表示本次没有新 slot，不能清空当前运行中的 slot。
  if (!ack.pending_snapshot && ack.pending.length === 0) {
    return slots;
  }
  const nextSlots = ack.pending.map((slot) => ({
    agent_id: slot.agent_id,
    agent_round_id: slot.agent_round_id,
    ...(slot.handoff_id ? { handoff_id: slot.handoff_id } : {}),
    ...(slot.hidden_from_user ? { hidden_from_user: true } : {}),
    ...(slot.source ? { source: slot.source } : {}),
    msg_id: slot.msg_id,
    round_id: slot.round_id?.trim() || ack.round_id,
    status: slot.status,
    timestamp: slot.timestamp,
    index: slot.index,
  }));
  if (ack.pending_snapshot) {
    return nextSlots;
  }
  const incomingAgentRoundIds = new Set(
    nextSlots.map((slot) => slot.agent_round_id),
  );
  const incomingMessageIds = new Set(nextSlots.map((slot) => slot.msg_id));
  const preservedSlots = slots.filter((slot) => (
    !incomingAgentRoundIds.has(slot.agent_round_id)
    && !incomingMessageIds.has(slot.msg_id)
  ));
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

/**
 * Room slot 可能以 no-reply 收口，终态 assistant/result 因而不会进入公区。
 * 此时必须用精确的 agent_round_id 结束已发布的 thinking 快照，不能等待整个
 * root round 或依赖一个刻意被抑制的最终消息。
 */
export function applyTerminalAgentRoundMessageStatus(
  messages: Message[],
  agentRoundId: string,
  status: RoundLifecycleStatus,
): Message[] {
  const normalizedAgentRoundId = agentRoundId.trim();
  if (!normalizedAgentRoundId || status === "running") {
    return messages;
  }
  const terminalStatus = getTerminalMessageStatus(status);
  return reconcileMessages(messages, (message) => {
    if (message.agent_round_id?.trim() !== normalizedAgentRoundId) {
      return KEEP_MESSAGE;
    }
    if (isEphemeralMessage(message)) {
      return REMOVE_MESSAGE;
    }
    if (
      message.role !== "assistant"
      || TERMINAL_ASSISTANT_STATUSES.has(
        message.stream_status ?? "pending",
      )
    ) {
      return KEEP_MESSAGE;
    }
    return updateAssistantStatus(message, terminalStatus);
  });
}
