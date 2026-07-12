import type {
  ApiAgentSession,
  AgentSession,
} from "@/types/agent/agent";
import type {
  ApiConversation,
  Conversation,
} from "@/types/conversation/conversation";
import type {
  ApiSessionRoundIndex,
  ApiSessionRoundIndexItem,
  SessionRoundIndexItem,
} from "@/types/conversation/history";

import { toTimestamp } from "../core/timestamp";

export function transformApiConversation(api: ApiConversation): Conversation {
  return {
    agent_id: api.agent_id,
    conversation_id: api.conversation_id ?? null,
    created_at: new Date(api.created_at).getTime(),
    is_active: api.is_active,
    last_activity_at: new Date(api.last_activity).getTime(),
    message_count: api.message_count,
    options: api.options || {},
    room_id: api.room_id ?? null,
    room_session_id: api.room_session_id ?? null,
    session_id: api.session_id,
    session_key: api.session_key,
    title: api.title || "未命名会话",
  };
}

export function transformApiAgentSession(api: ApiAgentSession): AgentSession {
  return {
    agent_id: api.agent_id,
    channel_type: api.channel_type,
    chat_type: api.chat_type,
    conversation_id: api.conversation_id ?? null,
    created_at: toTimestamp(api.created_at),
    last_activity_at: toTimestamp(api.last_activity),
    message_count: api.message_count,
    options: api.options || {},
    room_id: api.room_id ?? null,
    room_session_id: api.room_session_id ?? null,
    session_id: api.session_id,
    session_key: api.session_key,
    status: api.status,
    title: api.title || "未命名会话",
  };
}

function normalizePositiveNumber(
  value: number | null | undefined,
): number | null {
  return value && value > 0 ? value : null;
}

function normalizeOptionalText(value: string | undefined): string | null {
  return value?.trim() || null;
}

function normalizeAgentIds(agentIds: string[] | null | undefined): string[] {
  return (agentIds ?? [])
    .map((agentId) => agentId.trim())
    .filter(Boolean);
}

function transformApiSessionRoundIndexItem(
  item: ApiSessionRoundIndexItem,
): SessionRoundIndexItem | null {
  const roundId = item.round_id.trim();
  if (!roundId) {
    return null;
  }
  return {
    agentIds: normalizeAgentIds(item.agent_ids),
    durationMs: normalizePositiveNumber(item.duration_ms),
    hasUserMessage: item.has_user_message ?? false,
    isLive: item.is_live ?? false,
    roundId,
    status: normalizeOptionalText(item.status),
    timestamp: normalizePositiveNumber(item.timestamp),
    title: normalizeOptionalText(item.title) ?? "",
  };
}

export function transformApiSessionRoundIndex(
  index: ApiSessionRoundIndex,
): SessionRoundIndexItem[] {
  return (index.items ?? [])
    .map(transformApiSessionRoundIndexItem)
    .filter((item): item is SessionRoundIndexItem => item !== null);
}
