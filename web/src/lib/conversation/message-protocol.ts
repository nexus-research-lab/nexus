/**
 * INPUT: WebSocket message / stream 的未知协议载荷与信封回退字段。
 * OUTPUT: 校验后的 Conversation Message 或 Stream Message。
 * POS: 前端消息协议入口；Room 用户与宿主合成 assistant 可无 agent_id，普通 assistant/stream 必须有 agent_id。
 */
import {
  asUnknownRecord,
  hasFiniteNumberFields,
  hasNonEmptyStringFields,
  readString,
  readStringFromSet,
  type UnknownRecord,
} from "@/lib/unknown-value";
import type { Message } from "@/types/conversation/message/entity";
import type { StreamMessage } from "@/types/conversation/message/event";
import type { ContentBlock } from "@/types/conversation/message/content";

const MESSAGE_ROLES = new Set(["assistant", "system", "user"]);
const STREAM_MESSAGE_TYPES = new Set([
  "message_start",
  "content_block_start",
  "content_block_delta",
  "content_block_stop",
  "message_delta",
  "message_stop",
]);
const CONTENT_BLOCK_TYPES = new Set<ContentBlock["type"]>([
  "document",
  "image",
  "progress_update",
  "redacted_thinking",
  "resource_link",
  "search_result",
  "system_event",
  "task_progress",
  "text",
  "thinking",
  "tool_result",
  "tool_use",
  "tool_use_error",
  "unsupported",
  "workspace_file_artifact",
]);
const MESSAGE_IDENTITY_STRING_FIELDS = [
  "message_id",
  "round_id",
] as const;
const MESSAGE_TIMESTAMP_FIELDS = ["timestamp"] as const;

interface MessageEnvelopeProjection {
  deliveryMode?: string;
  sessionKey?: string;
}

function hasMessageContent(role: string, content: unknown): boolean {
  return role === "assistant" ? Array.isArray(content) : typeof content === "string";
}

export function parseContentBlock(value: unknown): ContentBlock | null {
  const record = asUnknownRecord(value);
  const originalType = record ? readString(record, "type") : null;
  if (!record || !originalType) {
    return null;
  }
  if (
    CONTENT_BLOCK_TYPES.has(originalType as ContentBlock["type"])
    && hasContentBlockShape(originalType as ContentBlock["type"], record)
  ) {
    return { ...record } as unknown as ContentBlock;
  }
  return {
    type: "unsupported",
    original_type: originalType,
    payload: { ...record },
  };
}

function hasContentBlockShape(
  type: ContentBlock["type"],
  record: UnknownRecord,
): boolean {
  switch (type) {
    case "text":
      return typeof record.text === "string";
    case "thinking":
      return typeof record.thinking === "string";
    case "tool_use":
      return typeof record.id === "string" && typeof record.name === "string";
    case "tool_result":
      return typeof record.tool_use_id === "string";
    case "tool_use_error":
      return typeof record.content === "string";
    case "task_progress":
      return typeof record.task_id === "string" && typeof record.description === "string";
    case "progress_update":
      return typeof record.text === "string";
    case "workspace_file_artifact":
      return typeof record.path === "string";
    case "system_event":
      return typeof record.content === "string" && typeof record.label === "string";
    case "unsupported":
      return typeof record.original_type === "string" && asUnknownRecord(record.payload) !== null;
    default:
      return true;
  }
}

function parseAssistantContent(value: unknown): ContentBlock[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map(parseContentBlock)
    .filter((block): block is ContentBlock => block !== null);
}

function hasMessageIdentity(
  record: UnknownRecord,
  role: string,
  allowAgentlessRoomAssistant = false,
): boolean {
  return (
    hasNonEmptyStringFields(record, MESSAGE_IDENTITY_STRING_FIELDS) &&
    hasFiniteNumberFields(record, MESSAGE_TIMESTAMP_FIELDS) &&
    typeof record.agent_id === "string" &&
    (
      role !== "assistant"
      || record.agent_id.length > 0
      || (allowAgentlessRoomAssistant && Boolean(readString(record, "room_id")))
    )
  );
}

function readDeliveryMode(
  record: Record<string, unknown>,
  envelopeMode?: string,
): Message["delivery_mode"] {
  const mode = envelopeMode ?? readString(record, "delivery_mode");
  return mode === "durable"
    || mode === "ephemeral"
    || mode === "transient"
    ? mode
    : undefined;
}

export function parseConversationMessage(
  value: unknown,
  envelope: MessageEnvelopeProjection = {},
): Message | null {
  const record = asUnknownRecord(value);
  if (!record) {
    return null;
  }
  const role = readStringFromSet(record, "role", MESSAGE_ROLES);
  const sessionKey = readString(record, "session_key") ?? envelope.sessionKey ?? null;
  if (
    !role
    || !sessionKey
    || !hasMessageIdentity(record, role, true)
    || !hasMessageContent(role, record.content)
  ) {
    return null;
  }

  const { delivery_mode: _ignoredDeliveryMode, ...messageFields } = record;
  const deliveryMode = readDeliveryMode(record, envelope.deliveryMode);
  return {
    ...messageFields,
    session_key: sessionKey,
    ...(role === "assistant"
      ? { content: parseAssistantContent(record.content) }
      : {}),
    ...(deliveryMode ? { delivery_mode: deliveryMode } : {}),
  } as unknown as Message;
}

export function parseStreamMessage(
  value: unknown,
  fallbackSessionKey?: string,
): StreamMessage | null {
  const record = asUnknownRecord(value);
  if (!record) {
    return null;
  }
  const sessionKey = readString(record, "session_key") ?? fallbackSessionKey ?? null;
  const type = readStringFromSet(record, "type", STREAM_MESSAGE_TYPES);
  if (
    !sessionKey
    || !type
    || !hasMessageIdentity(record, "assistant")
  ) {
    return null;
  }
  const contentBlock = record.content_block === undefined
    ? undefined
    : parseContentBlock(record.content_block) ?? undefined;
  return {
    ...record,
    session_key: sessionKey,
    ...(contentBlock ? { content_block: contentBlock } : {}),
  } as unknown as StreamMessage;
}
