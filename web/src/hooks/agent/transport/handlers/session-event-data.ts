import {
  asUnknownRecord,
  isStringArray,
  readNumber,
  readString,
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
const INPUT_QUEUE_SCOPES = new Set(["dm", "room"]);
const INPUT_QUEUE_SOURCES = new Set([
  "agent_public_mention",
  "agent_room_directed_message",
  "user",
]);
const DELIVERY_POLICIES = new Set(["auto", "guide", "interrupt", "queue"]);
const ASSISTANT_MESSAGE_STATUSES = new Set<
  ChatAckData["pending"][number]["status"]
>(["cancelled", "done", "error", "pending", "streaming"]);

function readRoundStatus(record: UnknownRecord): RoundLifecycleStatus | null {
  const status = readString(record, "status") as RoundLifecycleStatus | null;
  return status && ROUND_STATUSES.has(status) ? status : null;
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
  const scope = record ? readString(record, "scope") : null;
  const source = record ? readString(record, "source") : null;
  const deliveryPolicy = record ? readString(record, "delivery_policy") : null;
  return Boolean(
    record
    && scope
    && INPUT_QUEUE_SCOPES.has(scope)
    && source
    && INPUT_QUEUE_SOURCES.has(source)
    && deliveryPolicy
    && DELIVERY_POLICIES.has(deliveryPolicy)
    && readString(record, "id")
    && readString(record, "session_key")
    && typeof record.content === "string"
    && readNumber(record, "created_at") !== null
    && readNumber(record, "updated_at") !== null,
  );
}

export function parseInputQueueEventPayload(
  data: UnknownRecord,
): InputQueueEventPayload | null {
  const scope = readString(data, "scope");
  if (
    !scope
    || !INPUT_QUEUE_SCOPES.has(scope)
    || !Array.isArray(data.items)
    || !data.items.every(isInputQueueItem)
  ) {
    return null;
  }
  return { scope: scope as InputQueueEventPayload["scope"], items: data.items };
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
  const status = record ? readString(record, "status") : null;
  return Boolean(
    record
    && readString(record, "agent_id")
    && readString(record, "agent_round_id")
    && readString(record, "msg_id")
    && status
    && ASSISTANT_MESSAGE_STATUSES.has(
      status as ChatAckData["pending"][number]["status"],
    )
    && readNumber(record, "timestamp") !== null
    && readNumber(record, "index") !== null,
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
