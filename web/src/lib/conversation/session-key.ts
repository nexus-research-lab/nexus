import { resolveAgentId } from "@/config/runtime-options";

const AGENT_SESSION_PREFIX = "agent";
const ROOM_SESSION_PREFIX = "room";
const ROOM_SHARED_SESSION_PREFIX = "room:group:";
const TOPIC_SEGMENT = "topic";
const ACCOUNT_SEGMENT = "acct";

export interface BuildSessionKeyOptions {
  channel: string;
  chat_type: string;
  ref: string;
  agent_id?: string | null;
  account_id?: string | null;
  thread_id?: string | null;
}

type SessionKeyKind = "agent" | "room" | "unknown";

export interface ParsedSessionKey {
  raw: string;
  kind: SessionKeyKind;
  is_structured: boolean;
  is_shared: boolean;
  agent_id: string | null;
  channel: string | null;
  chat_type: string | null;
  account_id: string | null;
  ref: string | null;
  thread_id: string | null;
  conversation_id: string | null;
}

function findTopicIndex(parts: string[], minIndex: number): number {
  return parts.findIndex((part, index) => part === TOPIC_SEGMENT && index >= minIndex);
}

function agentSessionKeyShapeError(): string {
  return "session_key must match agent:<agent_id>:<channel>:<chat_type>[:acct:<account_id>]:<ref>[:topic:<thread_id>]";
}

function splitAgentRefParts(parts: string[]): {
  account_id: string | null;
  ref_start: number;
  error: string | null;
} {
  if (parts[4] === ACCOUNT_SEGMENT) {
    if (parts.length < 7) {
      return { account_id: null, ref_start: 0, error: agentSessionKeyShapeError() };
    }
    const accountId = parts[5]?.trim() ?? "";
    if (!accountId) {
      return { account_id: null, ref_start: 0, error: "session_key account_id is required after acct segment" };
    }
    return { account_id: accountId, ref_start: 6, error: null };
  }
  return { account_id: null, ref_start: 4, error: null };
}

/**
 * 中文注释：前后端共享同一套 sessionKey 语义，前端不要再散落手拼字符串。
 */
export function buildSessionKey({
  channel,
  chat_type: chatType,
  ref,
  agent_id: agentId,
  account_id: accountId,
  thread_id: threadId,
}: BuildSessionKeyOptions): string {
  const resolvedAgentId = resolveAgentId(agentId);
  const resolvedChannel = channel.trim();
  const resolvedChatType = chatType.trim();
  const resolvedRef = ref.trim();
  const resolvedAccountId = accountId?.trim() ?? "";
  let key = resolvedAccountId
    ? `${AGENT_SESSION_PREFIX}:${resolvedAgentId}:${resolvedChannel}:${resolvedChatType}:${ACCOUNT_SEGMENT}:${resolvedAccountId}:${resolvedRef}`
    : `${AGENT_SESSION_PREFIX}:${resolvedAgentId}:${resolvedChannel}:${resolvedChatType}:${resolvedRef}`;
  if (threadId?.trim()) {
    key += `:${TOPIC_SEGMENT}:${threadId.trim()}`;
  }
  return key;
}

export function buildRoomSharedSessionKey(conversationId: string): string {
  return `${ROOM_SHARED_SESSION_PREFIX}${conversationId}`;
}

export function buildRoomAgentSessionKey(
  conversationId: string,
  agentId: string,
  roomType: "dm" | "room" = "room",
): string {
  return buildSessionKey({
    channel: "ws",
    chat_type: roomType === "dm" ? "dm" : "group",
    ref: conversationId,
    agent_id: agentId,
  });
}

function getSessionKeyValidationError(sessionKey: string | null | undefined): string | null {
  const normalizedKey = (sessionKey ?? "").trim();
  if (!normalizedKey) {
    return "session_key is required";
  }

  if (normalizedKey.startsWith(`${AGENT_SESSION_PREFIX}:`)) {
    const parts = normalizedKey.split(":");
    if (parts.length < 5 || !parts[1] || !parts[2] || !parts[3]) {
      return agentSessionKeyShapeError();
    }

    const split = splitAgentRefParts(parts);
    if (split.error) {
      return split.error;
    }
    const topicIndex = findTopicIndex(parts, split.ref_start);
    if (topicIndex >= 0) {
      const ref = parts.slice(split.ref_start, topicIndex).join(":").trim();
      const threadId = parts.slice(topicIndex + 1).join(":").trim();
      return ref && threadId ? null : agentSessionKeyShapeError();
    }

    return parts.slice(split.ref_start).join(":").trim() ? null : agentSessionKeyShapeError();
  }

  if (normalizedKey.startsWith(`${ROOM_SESSION_PREFIX}:`)) {
    const parts = normalizedKey.split(":");
    const conversationId = parts.slice(2).join(":").trim();
    return parts.length >= 3 && parts[1] === "group" && conversationId
      ? null
      : "session_key must match room:group:<conversation_id>";
  }

  return "session_key must use structured gateway format";
}

export function isStructuredSessionKey(sessionKey: string): boolean {
  return getSessionKeyValidationError(sessionKey) === null;
}

export function assertStructuredSessionKey(sessionKey: string | null | undefined): string {
  const errorMessage = getSessionKeyValidationError(sessionKey);
  if (errorMessage) {
    throw new Error(errorMessage);
  }
  return (sessionKey ?? "").trim();
}


export function parseSessionKey(sessionKey: string | null | undefined): ParsedSessionKey {
  const normalizedKey = (sessionKey ?? "").trim();
  const validationError = getSessionKeyValidationError(normalizedKey);
  const result: ParsedSessionKey = {
    raw: normalizedKey,
    kind: "unknown",
    is_structured: false,
    is_shared: false,
    agent_id: null,
    channel: null,
    chat_type: null,
    account_id: null,
    ref: null,
    thread_id: null,
    conversation_id: null,
  };

  if (normalizedKey.startsWith(`${AGENT_SESSION_PREFIX}:`)) {
    const parts = normalizedKey.split(":");
    result.kind = "agent";
    result.is_structured = validationError === null;
    result.agent_id = resolveAgentId(parts[1]);
    result.channel = parts[2] || null;
    result.chat_type = parts[3] || "dm";

    // `:topic:` 是协议保留边界，ref 中允许冒号，但不能跨过该边界。
    const split = splitAgentRefParts(parts);
    if (split.error) {
      return result;
    }
    result.account_id = split.account_id;
    const topicIndex = findTopicIndex(parts, split.ref_start);
    if (topicIndex >= 0) {
      result.ref = parts.slice(split.ref_start, topicIndex).join(":") || null;
      result.thread_id = parts.slice(topicIndex + 1).join(":") || null;
    } else {
      result.ref = parts.slice(split.ref_start).join(":") || null;
    }
    return result;
  }

  if (normalizedKey.startsWith(`${ROOM_SESSION_PREFIX}:`)) {
    const parts = normalizedKey.split(":");
    const conversationId = parts.slice(2).join(":").trim();
    result.kind = "room";
    result.is_structured = validationError === null;
    result.is_shared = validationError === null;
    result.chat_type = parts[1] || "group";
    result.ref = conversationId || null;
    result.conversation_id = conversationId || null;
  }

  return result;
}

export function getSessionKeyIdentity(sessionKey: string | null | undefined): string | null {
  const parsed = parseSessionKey(sessionKey);
  if (!parsed.raw) {
    return null;
  }

  // Room 键比较时只认 conversationId，避免未来 alias 演进时前端错判。
  if (parsed.kind === "room" && parsed.conversation_id) {
    return `${ROOM_SESSION_PREFIX}:${parsed.conversation_id}`;
  }

  return parsed.raw;
}

export function areEquivalentSessionKeys(
  left: string | null | undefined,
  right: string | null | undefined,
): boolean {
  const leftIdentity = getSessionKeyIdentity(left);
  const rightIdentity = getSessionKeyIdentity(right);
  return Boolean(leftIdentity && rightIdentity && leftIdentity === rightIdentity);
}
