import { DEFAULT_AGENT_ID } from "@/config/options";

const ROOM_SHARED_SESSION_PREFIX = "room:group:";

function resolveAgentId(agent_id?: string | null): string {
  const normalized_agent_id = agent_id?.trim();
  return normalized_agent_id || DEFAULT_AGENT_ID;
}

function buildWsSessionKey(
  ref: string,
  chat_type: "dm" | "group",
  agent_id?: string | null,
): string {
  return `agent:${resolveAgentId(agent_id)}:ws:${chat_type}:${ref}`;
}

export function buildWsDmSessionKey(ref: string, agent_id?: string | null): string {
  return buildWsSessionKey(ref, "dm", agent_id);
}

export function buildRoomSharedSessionKey(conversation_id: string): string {
  return `${ROOM_SHARED_SESSION_PREFIX}${conversation_id}`;
}

export function buildRoomAgentSessionKey(
  conversation_id: string,
  agent_id: string,
  room_type: "dm" | "room" | "group" = "room",
): string {
  return buildWsSessionKey(
    conversation_id,
    room_type === "dm" ? "dm" : "group",
    agent_id,
  );
}

export function isRoomSharedSessionKey(session_key: string): boolean {
  return session_key.startsWith(ROOM_SHARED_SESSION_PREFIX);
}

export function parseRoomConversationId(session_key: string): string | null {
  if (!isRoomSharedSessionKey(session_key)) {
    return null;
  }
  return session_key.slice(ROOM_SHARED_SESSION_PREFIX.length) || null;
}
