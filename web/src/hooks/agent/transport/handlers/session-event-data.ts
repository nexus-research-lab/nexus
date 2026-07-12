import {
  asUnknownRecord,
  hasFiniteNumberFields,
  hasNonEmptyStringFields,
  isStringArray,
  readNumber,
  readString,
  readStringFromSet,
  type UnknownRecord,
} from "@/lib/unknown-value";
import type {
  InputQueueEventPayload,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import type {
  AgentRoundStatusEventPayload,
  ChatAckData,
  RoundLifecycleStatus,
  RoundStatusEventPayload,
} from "@/types/conversation/message/event";
import type { SessionStatusData } from "@/types/generated/protocol";

const ROUND_STATUSES = new Set<RoundLifecycleStatus>([
  "error",
  "finished",
  "interrupted",
  "running",
]);
const INPUT_QUEUE_SCOPES = new Set<InputQueueItem["scope"]>(["dm", "room"]);
const INPUT_QUEUE_SOURCES = new Set<InputQueueItem["source"]>([
  "agent_public_mention",
  "agent_room_directed_message",
  "user",
]);
const DELIVERY_POLICIES = new Set<InputQueueItem["delivery_policy"]>([
  "auto",
  "guide",
  "interrupt",
  "queue",
]);
const ASSISTANT_MESSAGE_STATUSES = new Set<
  ChatAckData["pending"][number]["status"]
>(["cancelled", "done", "error", "pending", "streaming"]);
const INPUT_QUEUE_REQUIRED_STRING_FIELDS = ["id", "session_key"] as const;
const INPUT_QUEUE_REQUIRED_NUMBER_FIELDS = [
  "created_at",
  "updated_at",
] as const;
const CHAT_ACK_SLOT_REQUIRED_STRING_FIELDS = [
  "agent_id",
  "agent_round_id",
  "msg_id",
] as const;
const CHAT_ACK_SLOT_REQUIRED_NUMBER_FIELDS = ["index", "timestamp"] as const;

function readRoundStatus(record: UnknownRecord): RoundLifecycleStatus | null {
  return readStringFromSet(record, "status", ROUND_STATUSES);
}

export function parseSessionStatusData(
  data: UnknownRecord,
): SessionStatusData | null {
  if (typeof data.is_generating !== "boolean") {
    return null;
  }
  if (
    data.running_round_ids !== undefined
    && !isStringArray(data.running_round_ids)
  ) {
    return null;
  }
  return {
    is_generating: data.is_generating,
    ...(data.running_round_ids
      ? { running_round_ids: data.running_round_ids }
      : {}),
  };
}

function isInputQueueItem(value: unknown): value is InputQueueItem {
  const record = asUnknownRecord(value);
  if (!record) {
    return false;
  }
  const scope = readStringFromSet(record, "scope", INPUT_QUEUE_SCOPES);
  const source = readStringFromSet(record, "source", INPUT_QUEUE_SOURCES);
  const deliveryPolicy = readStringFromSet(
    record,
    "delivery_policy",
    DELIVERY_POLICIES,
  );
  return Boolean(
    scope
    && source
    && deliveryPolicy
    && typeof record.content === "string"
    && hasNonEmptyStringFields(record, INPUT_QUEUE_REQUIRED_STRING_FIELDS)
    && hasFiniteNumberFields(record, INPUT_QUEUE_REQUIRED_NUMBER_FIELDS),
  );
}

export function parseInputQueueEventPayload(
  data: UnknownRecord,
): InputQueueEventPayload | null {
  const scope = readStringFromSet(data, "scope", INPUT_QUEUE_SCOPES);
  if (
    !scope
    || !Array.isArray(data.items)
    || !data.items.every(isInputQueueItem)
  ) {
    return null;
  }
  return { scope, items: data.items };
}

export function parseRoundStatusEventPayload(
  data: UnknownRecord,
): RoundStatusEventPayload | null {
  const roundId = readString(data, "round_id");
  const status = readRoundStatus(data);
  if (!roundId || !status || typeof data.is_terminal !== "boolean") {
    return null;
  }
  const resultSubtype = readString(data, "result_subtype");
  return {
    round_id: roundId,
    status,
    is_terminal: data.is_terminal,
    ...(resultSubtype ? { result_subtype: resultSubtype as RoundStatusEventPayload["result_subtype"] } : {}),
  };
}

export function parseAgentRoundStatusEventPayload(
  data: UnknownRecord,
): AgentRoundStatusEventPayload | null {
  const roundId = readString(data, "round_id");
  const agentRoundId = readString(data, "agent_round_id");
  const agentId = readString(data, "agent_id");
  const status = readRoundStatus(data);
  if (
    !roundId
    || !agentRoundId
    || !agentId
    || !status
    || typeof data.is_terminal !== "boolean"
  ) {
    return null;
  }
  return {
    agent_id: agentId,
    agent_round_id: agentRoundId,
    is_terminal: data.is_terminal,
    round_id: roundId,
    status,
  };
}

function isChatAckPendingSlot(
  value: unknown,
): value is ChatAckData["pending"][number] {
  const record = asUnknownRecord(value);
  if (!record) {
    return false;
  }
  const status = readStringFromSet(
    record,
    "status",
    ASSISTANT_MESSAGE_STATUSES,
  );
  return Boolean(
    status
    && hasNonEmptyStringFields(record, CHAT_ACK_SLOT_REQUIRED_STRING_FIELDS)
    && hasFiniteNumberFields(record, CHAT_ACK_SLOT_REQUIRED_NUMBER_FIELDS),
  );
}

export function parseChatAckData(data: UnknownRecord): ChatAckData | null {
  if (
    !readString(data, "client_request_id")
    || !readString(data, "client_message_id")
    || !readString(data, "round_id")
    || !readString(data, "user_message_id")
    || !Array.isArray(data.pending)
    || !data.pending.every(isChatAckPendingSlot)
    || readNumber(data, "ack_timeout_ms") === null
  ) {
    return null;
  }
  return data as unknown as ChatAckData;
}
