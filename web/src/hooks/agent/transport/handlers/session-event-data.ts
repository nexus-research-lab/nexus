/**
 * INPUT: Session/runtime/queue/round/chat/interrupt ACK 的未知 WebSocket 载荷。
 * OUTPUT: 经字段级校验且保留 public handoff 与精确停止关联的窄事件数据。
 * POS: Agent Session 事件副作用前的协议解码边界。
 */
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
  AgentConversationRuntimeStatus,
} from "@/types/agent/agent-conversation";
import type {
  AgentRoundStatusEventPayload,
  ChatAckData,
  RoundLifecycleStatus,
  RoundStatusEventPayload,
} from "@/types/conversation/message/event";
import type {
  CommandCatalogData,
  CommandCatalogStatus,
  CommandDescriptor,
  CommandExecution,
  ContextUsageData,
  InputQueueAckData,
  InterruptAckData,
  RuntimeStatusData,
  SessionStatusData,
} from "@/types/generated/protocol";

const ROUND_STATUSES = new Set<RoundLifecycleStatus>([
  "error",
  "finished",
  "interrupted",
  "running",
]);
const RUNTIME_STATUSES = new Set<Exclude<AgentConversationRuntimeStatus, null>>([
  "compacting",
]);
const COMMAND_CATALOG_STATUSES = new Set<CommandCatalogStatus>([
  "cold",
  "ready",
  "unavailable",
]);
const COMMAND_EXECUTIONS = new Set<CommandExecution>([
  "host",
  "runtime",
  "unsupported",
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
const CHAT_ACK_USER_MESSAGE_DELIVERY_MODES = new Set<
  NonNullable<ChatAckData["user_message_delivery_mode"]>
>(["durable", "ephemeral", "transient"]);
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
const CHAT_ACK_SLOT_OPTIONAL_STRING_FIELDS = ["handoff_id"] as const;

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

export function parseRuntimeStatusData(
  data: UnknownRecord,
): RuntimeStatusData | null {
  if (data.status === null) {
    return {status: null};
  }
  const status = readStringFromSet(data, "status", RUNTIME_STATUSES);
  return status ? {status} : null;
}

function isCommandDescriptor(value: unknown): value is CommandDescriptor {
  const record = asUnknownRecord(value);
  if (!record) {
    return false;
  }
  const execution = readStringFromSet(
    record,
    "execution",
    COMMAND_EXECUTIONS,
  );
  return Boolean(
    readString(record, "name")
    && execution
    && typeof record.enabled === "boolean"
    && (
      record.description === undefined
      || typeof record.description === "string"
    )
    && (
      record.argument_hint === undefined
      || typeof record.argument_hint === "string"
    )
    && (
      record.disabled_reason === undefined
      || typeof record.disabled_reason === "string"
    ),
  );
}

export function parseCommandCatalogData(
  data: UnknownRecord,
): CommandCatalogData | null {
  const status = readStringFromSet(
    data,
    "status",
    COMMAND_CATALOG_STATUSES,
  );
  if (
    !status
    || !Array.isArray(data.commands)
    || !data.commands.every(isCommandDescriptor)
  ) {
    return null;
  }
  const revision = readString(data, "revision");
  const generation = readNumber(data, "generation");
  const runtimeKind = readString(data, "runtime_kind");
  const agentId = readString(data, "agent_id");
  return {
    status,
    commands: data.commands,
    ...(revision ? { revision } : {}),
    ...(generation !== null && generation >= 0 ? { generation } : {}),
    ...(runtimeKind ? { runtime_kind: runtimeKind } : {}),
    ...(agentId ? { agent_id: agentId } : {}),
  };
}

export function parseContextUsageData(
  data: UnknownRecord,
): ContextUsageData | null {
  const totalTokens = readNumber(data, "total_tokens");
  const maxTokens = readNumber(data, "max_tokens");
  const percentage = readNumber(data, "percentage");
  if (
    totalTokens === null
    || totalTokens < 0
    || maxTokens === null
    || maxTokens <= 0
    || percentage === null
    || percentage < 0
  ) {
    return null;
  }
  const model = readString(data, "model");
  return {
    total_tokens: totalTokens,
    max_tokens: maxTokens,
    percentage,
    ...(model ? { model } : {}),
  };
}

const COMMAND_CATALOG_STATUS_RANK: Record<CommandCatalogStatus, number> = {
  cold: 0,
  unavailable: 1,
  ready: 2,
};

export function selectCommandCatalogSnapshot(
  current: CommandCatalogData,
  incoming: CommandCatalogData,
): CommandCatalogData {
  const currentGeneration = current.generation ?? 0;
  const incomingGeneration = incoming.generation ?? 0;
  if (incomingGeneration < currentGeneration) {
    return current;
  }
  if (
    incomingGeneration === currentGeneration
    && COMMAND_CATALOG_STATUS_RANK[incoming.status]
      < COMMAND_CATALOG_STATUS_RANK[current.status]
  ) {
    return current;
  }
  return incoming;
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
  const errorMessage = readString(data, "message")
    ?? readString(data, "error_message");
  return {
    round_id: roundId,
    status,
    is_terminal: data.is_terminal,
    ...(resultSubtype ? { result_subtype: resultSubtype as RoundStatusEventPayload["result_subtype"] } : {}),
    ...(errorMessage ? { error_message: errorMessage } : {}),
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
  if (
    record.round_id !== undefined
    && !readString(record, "round_id")
  ) {
    return false;
  }
  if (CHAT_ACK_SLOT_OPTIONAL_STRING_FIELDS.some(
    (field) => record[field] !== undefined && !readString(record, field),
  )) {
    return false;
  }
  if (
    record.hidden_from_user !== undefined
    && typeof record.hidden_from_user !== "boolean"
  ) {
    return false;
  }
  return Boolean(
    status
    && hasNonEmptyStringFields(record, CHAT_ACK_SLOT_REQUIRED_STRING_FIELDS)
    && hasFiniteNumberFields(record, CHAT_ACK_SLOT_REQUIRED_NUMBER_FIELDS),
  );
}

export function parseChatAckData(data: UnknownRecord): ChatAckData | null {
  const invalidUserMessageDeliveryMode = (
    data.user_message_delivery_mode !== undefined
    && readStringFromSet(
      data,
      "user_message_delivery_mode",
      CHAT_ACK_USER_MESSAGE_DELIVERY_MODES,
    ) === null
  );
  if (
    typeof data.user_message_committed !== "boolean"
    || typeof data.pending_snapshot !== "boolean"
    || !Array.isArray(data.pending)
    || !data.pending.every(isChatAckPendingSlot)
    || readNumber(data, "ack_timeout_ms") === null
    || invalidUserMessageDeliveryMode
  ) {
    return null;
  }

  const roundId = readString(data, "round_id");
  const hasClientCorrelation = Boolean(
    readString(data, "client_request_id")
    && readString(data, "client_message_id")
    && readString(data, "user_message_id"),
  );
  const hasEmptyCorrelation = (
    data.client_request_id === ""
    && data.client_message_id === ""
    && data.user_message_id === ""
  );
  const isAuthoritativeSnapshot = (
    data.pending_snapshot
    && hasEmptyCorrelation
    && data.user_message_committed === false
    && (
      data.pending.length === 0
      || Boolean(roundId)
      || data.pending.every((slot) => Boolean(slot.round_id))
    )
  );
  const isServerInitiatedPendingAck = (
    !data.pending_snapshot
    && hasEmptyCorrelation
    && data.user_message_committed === false
    && data.pending.length > 0
    && Boolean(roundId)
  );
  const isClientAck = (
    !data.pending_snapshot
    && hasClientCorrelation
    && Boolean(roundId)
  );
  if (!isClientAck && !isServerInitiatedPendingAck && !isAuthoritativeSnapshot) {
    return null;
  }
  return data as unknown as ChatAckData;
}

export function parseInputQueueAckData(
  data: UnknownRecord,
): InputQueueAckData | null {
  if (
    !readString(data, "client_request_id")
    || !readString(data, "client_message_id")
    || !readString(data, "action")
    || !readString(data, "item_id")
    || typeof data.accepted !== "boolean"
    || typeof data.duplicate !== "boolean"
    || readNumber(data, "ack_timeout_ms") === null
  ) {
    return null;
  }
  return data as unknown as InputQueueAckData;
}

export function parseInterruptAckData(
  data: UnknownRecord,
): InterruptAckData | null {
  if (
    !readString(data, "client_request_id")
    || typeof data.accepted !== "boolean"
    || readNumber(data, "ack_timeout_ms") === null
    || (
      data.round_id !== undefined
      && data.round_id !== ""
      && !readString(data, "round_id")
    )
    || (
      data.agent_round_id !== undefined
      && data.agent_round_id !== ""
      && !readString(data, "agent_round_id")
    )
  ) {
    return null;
  }
  return data as unknown as InterruptAckData;
}
