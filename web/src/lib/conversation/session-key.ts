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

interface AgentRefParts {
  account_id: string | null;
  error: string | null;
  ref_start: number;
}

interface ParsedAgentReference {
  account_id: string | null;
  ref: string | null;
  thread_id: string | null;
}

interface SessionKeyRule {
  parse: (sessionKey: string, isStructured: boolean) => ParsedSessionKey;
  prefix: string;
  validate: (sessionKey: string) => string | null;
}

function createParsedSessionKey(raw: string): ParsedSessionKey {
  return {
    raw,
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
}

function findTopicIndex(parts: string[], minIndex: number): number {
  return parts.findIndex(
    (part, index) => part === TOPIC_SEGMENT && index >= minIndex,
  );
}

function agentSessionKeyShapeError(): string {
  return "session_key must match agent:<agent_id>:<channel>:<chat_type>[:acct:<account_id>]:<ref>[:topic:<thread_id>]";
}

function splitAgentRefParts(parts: string[]): AgentRefParts {
  if (parts[4] !== ACCOUNT_SEGMENT) {
    return { account_id: null, error: null, ref_start: 4 };
  }
  if (parts.length < 7) {
    return {
      account_id: null,
      error: agentSessionKeyShapeError(),
      ref_start: 0,
    };
  }
  const accountId = parts[5]?.trim() ?? "";
  return accountId
    ? { account_id: accountId, error: null, ref_start: 6 }
    : {
        account_id: null,
        error: "session_key account_id is required after acct segment",
        ref_start: 0,
      };
}

function parseAgentReference(parts: string[]): ParsedAgentReference {
  const split = splitAgentRefParts(parts);
  if (split.error) {
    return { account_id: null, ref: null, thread_id: null };
  }

  // `:topic:` 是协议边界，ref 内部仍可包含冒号。
  const topicIndex = findTopicIndex(parts, split.ref_start);
  const hasTopic = topicIndex >= 0;
  const refEnd = hasTopic ? topicIndex : parts.length;
  return {
    account_id: split.account_id,
    ref: parts.slice(split.ref_start, refEnd).join(":") || null,
    thread_id: hasTopic
      ? parts.slice(topicIndex + 1).join(":") || null
      : null,
  };
}

function hasAgentSessionHeader(parts: string[]): boolean {
  return parts.length >= 5 && parts.slice(1, 4).every(Boolean);
}

function validateAgentReference(
  parts: string[],
  refStart: number,
): string | null {
  const topicIndex = findTopicIndex(parts, refStart);
  if (topicIndex < 0) {
    return parts.slice(refStart).join(":").trim()
      ? null
      : agentSessionKeyShapeError();
  }
  const ref = parts.slice(refStart, topicIndex).join(":").trim();
  const threadId = parts.slice(topicIndex + 1).join(":").trim();
  return ref && threadId ? null : agentSessionKeyShapeError();
}

function validateAgentSessionKey(sessionKey: string): string | null {
  const parts = sessionKey.split(":");
  if (!hasAgentSessionHeader(parts)) {
    return agentSessionKeyShapeError();
  }

  const split = splitAgentRefParts(parts);
  if (split.error) {
    return split.error;
  }
  return validateAgentReference(parts, split.ref_start);
}

function parseAgentSessionKey(
  sessionKey: string,
  isStructured: boolean,
): ParsedSessionKey {
  const parts = sessionKey.split(":");
  return {
    ...createParsedSessionKey(sessionKey),
    ...parseAgentReference(parts),
    kind: "agent",
    is_structured: isStructured,
    agent_id: resolveAgentId(parts[1]),
    channel: parts[2] || null,
    chat_type: parts[3] || "dm",
  };
}

function validateRoomSessionKey(sessionKey: string): string | null {
  const parts = sessionKey.split(":");
  const conversationId = parts.slice(2).join(":").trim();
  return parts.length >= 3 && parts[1] === "group" && conversationId
    ? null
    : "session_key must match room:group:<conversation_id>";
}

function parseRoomSessionKey(
  sessionKey: string,
  isStructured: boolean,
): ParsedSessionKey {
  const parts = sessionKey.split(":");
  const conversationId = parts.slice(2).join(":").trim() || null;
  return {
    ...createParsedSessionKey(sessionKey),
    kind: "room",
    is_structured: isStructured,
    is_shared: isStructured,
    chat_type: parts[1] || "group",
    ref: conversationId,
    conversation_id: conversationId,
  };
}

const SESSION_KEY_RULES: SessionKeyRule[] = [
  {
    parse: parseAgentSessionKey,
    prefix: `${AGENT_SESSION_PREFIX}:`,
    validate: validateAgentSessionKey,
  },
  {
    parse: parseRoomSessionKey,
    prefix: `${ROOM_SESSION_PREFIX}:`,
    validate: validateRoomSessionKey,
  },
];

function findSessionKeyRule(sessionKey: string): SessionKeyRule | undefined {
  return SESSION_KEY_RULES.find((rule) => sessionKey.startsWith(rule.prefix));
}

function getSessionKeyValidationError(
  sessionKey: string | null | undefined,
): string | null {
  const normalizedKey = (sessionKey ?? "").trim();
  if (!normalizedKey) {
    return "session_key is required";
  }
  const rule = findSessionKeyRule(normalizedKey);
  return rule
    ? rule.validate(normalizedKey)
    : "session_key must use structured gateway format";
}

/** 前后端必须共用协议构造入口，避免业务层手拼身份。 */
export function buildSessionKey({
  channel,
  chat_type: chatType,
  ref,
  agent_id: agentId,
  account_id: accountId,
  thread_id: threadId,
}: BuildSessionKeyOptions): string {
  const segments = [
    AGENT_SESSION_PREFIX,
    resolveAgentId(agentId),
    channel.trim(),
    chatType.trim(),
  ];
  const resolvedAccountId = accountId?.trim();
  if (resolvedAccountId) {
    segments.push(ACCOUNT_SEGMENT, resolvedAccountId);
  }
  segments.push(ref.trim());
  const resolvedThreadId = threadId?.trim();
  if (resolvedThreadId) {
    segments.push(TOPIC_SEGMENT, resolvedThreadId);
  }
  return segments.join(":");
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

export function isStructuredSessionKey(sessionKey: string): boolean {
  return getSessionKeyValidationError(sessionKey) === null;
}

export function assertStructuredSessionKey(
  sessionKey: string | null | undefined,
): string {
  const errorMessage = getSessionKeyValidationError(sessionKey);
  if (errorMessage) {
    throw new Error(errorMessage);
  }
  return (sessionKey ?? "").trim();
}

export function parseSessionKey(
  sessionKey: string | null | undefined,
): ParsedSessionKey {
  const normalizedKey = (sessionKey ?? "").trim();
  const rule = findSessionKeyRule(normalizedKey);
  return rule
    ? rule.parse(normalizedKey, rule.validate(normalizedKey) === null)
    : createParsedSessionKey(normalizedKey);
}

export function getSessionKeyIdentity(
  sessionKey: string | null | undefined,
): string | null {
  const parsed = parseSessionKey(sessionKey);
  if (!parsed.raw) {
    return null;
  }

  // Room 身份只认 conversationId，避免别名演进影响前端比较。
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
