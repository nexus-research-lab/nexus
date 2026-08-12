import { parseSessionKey } from "@/lib/conversation/session-key";
import type { ExternalSessionIdentity } from "@/types/agent/agent";
import type { BaseConversation } from "@/types/conversation/conversation";

const CHANNEL_LABELS: Record<string, string> = {
  dingtalk: "钉钉",
  dt: "钉钉",
  discord: "Discord",
  dg: "Discord",
  feishu: "飞书",
  fs: "飞书",
  telegram: "Telegram",
  tg: "Telegram",
  wechat: "企业微信",
  wx: "企业微信",
  "weixin-personal": "微信",
};

const INTERNAL_CHANNELS = new Set(["", "websocket", "ws", "internal"]);
const SUPPORTED_EXTERNAL_CHANNELS = new Set(Object.keys(CHANNEL_LABELS));
const EXTERNAL_SESSION_CONVERSATION_PREFIX = "external-session:";

function normalizeChannel(
  channelType?: string | null,
  sessionKey?: string | null,
): string {
  const parsed = parseSessionKey(sessionKey);
  return (channelType || parsed.channel || "").trim();
}

export function isExternalSessionChannel(
  channelType?: string | null,
  sessionKey?: string | null,
): boolean {
  const channel = normalizeChannel(channelType, sessionKey);
  return !INTERNAL_CHANNELS.has(channel) && SUPPORTED_EXTERNAL_CHANNELS.has(channel);
}

export function getExternalSessionChannelLabel(
  channelType?: string | null,
  sessionKey?: string | null,
): string | null {
  const channel = normalizeChannel(channelType, sessionKey);
  if (INTERNAL_CHANNELS.has(channel) || !SUPPORTED_EXTERNAL_CHANNELS.has(channel)) {
    return null;
  }
  return CHANNEL_LABELS[channel] ?? (channel || "外部通道");
}

export function isExternalSessionConversation(
  conversation?: Pick<BaseConversation, "options">,
): boolean {
  return conversation?.options.external_session === true;
}

export function getExternalSessionConversationLabel(
  conversation: Pick<BaseConversation, "options" | "session_key">,
): string | null {
  if (!isExternalSessionConversation(conversation)) {
    return null;
  }
  return getExternalSessionDisplayLabel(
    readStringOption(conversation.options, "channel_type"),
    conversation.session_key,
    readExternalSessionIdentity(conversation.options),
  );
}

export function getExternalSessionDisplayLabel(
  channelType?: string | null,
  sessionKey?: string | null,
  identity?: ExternalSessionIdentity | null,
): string | null {
  const channelLabel = getExternalSessionChannelLabel(channelType, sessionKey);
  if (!channelLabel) {
    return null;
  }
  if (!identity) {
    return channelLabel;
  }
  const parts = [channelLabel];
  const accountHint = identity.account_hint?.trim();
  if (accountHint) {
    parts.push(`账号 ${accountHint}`);
  } else if (identity.legacy_session_hint?.trim()) {
    parts.push(`旧会话 ${identity.legacy_session_hint.trim()}`);
  }
  parts.push(identity.current_pairing ? "当前" : "历史");
  const taskReferenceCount = Math.max(0, identity.task_reference_count ?? 0);
  if (taskReferenceCount > 0) {
    parts.push(`${taskReferenceCount} 个任务`);
  }
  return parts.join(" · ");
}

export function isExternalSessionConversationDeletable(
  conversation: Pick<BaseConversation, "options">,
): boolean {
  const identity = readExternalSessionIdentity(conversation.options);
  return identity?.can_delete === true
    && identity.current_pairing === false;
}

export function getExternalSessionTaskReferenceCount(
  conversation?: Pick<BaseConversation, "options"> | null,
): number {
  if (!conversation) {
    return 0;
  }
  const identity = readExternalSessionIdentity(conversation.options);
  return Math.max(0, identity?.task_reference_count ?? 0);
}

export function formatExternalSessionTitle({
  title,
}: {
  title?: string | null;
}): string {
  return (title ?? "").trim() || "New Chat";
}

export function buildExternalSessionConversationId(sessionKey: string): string {
  return `${EXTERNAL_SESSION_CONVERSATION_PREFIX}${sessionKey.trim()}`;
}

export function getExternalSessionKeyFromConversationId(
  conversationId?: string | null,
): string | null {
  const normalized = (conversationId ?? "").trim();
  if (!normalized.startsWith(EXTERNAL_SESSION_CONVERSATION_PREFIX)) {
    return null;
  }
  return normalized.slice(EXTERNAL_SESSION_CONVERSATION_PREFIX.length).trim() || null;
}

export function isExternalSessionConversationId(
  conversationId?: string | null,
): boolean {
  return getExternalSessionKeyFromConversationId(conversationId) !== null;
}

function readStringOption(
  options: Record<string, unknown>,
  key: string,
): string | null {
  const value = options[key];
  return typeof value === "string" ? value : null;
}

function readExternalSessionIdentity(
  options: Record<string, unknown>,
): ExternalSessionIdentity | null {
  const value = options.external_identity;
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  const identity = value as Partial<ExternalSessionIdentity>;
  if (typeof identity.current_pairing !== "boolean"
    || typeof identity.can_delete !== "boolean") {
    return null;
  }
  return {
    account_hint: typeof identity.account_hint === "string"
      ? identity.account_hint
      : undefined,
    account_status: typeof identity.account_status === "string"
      ? identity.account_status
      : undefined,
    can_delete: identity.can_delete,
    channel_type: typeof identity.channel_type === "string"
      ? identity.channel_type
      : "",
    current_pairing: identity.current_pairing,
    legacy_session_hint: typeof identity.legacy_session_hint === "string"
      ? identity.legacy_session_hint
      : undefined,
    pairing_status: typeof identity.pairing_status === "string"
      ? identity.pairing_status
      : "unpaired",
    peer_name: typeof identity.peer_name === "string"
      ? identity.peer_name
      : undefined,
    task_reference_count: typeof identity.task_reference_count === "number"
      ? identity.task_reference_count
      : undefined,
  };
}
