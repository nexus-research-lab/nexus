import type { Message, RoomPendingAgentSlotState } from "@/types";

import type { VolatileConversationSnapshot } from "./conversation-volatile-model";

const VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX =
  "nexus.agent_conversation.volatile";
const MAX_VOLATILE_MESSAGE_COUNT = 200;

function buildStorageKey(sessionKey: string): string {
  return `${VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX}:${sessionKey}`;
}

function getSessionStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

export function readVolatileConversationSnapshot(
  sessionKey: string,
): VolatileConversationSnapshot | null {
  const storage = getSessionStorage();
  if (!storage) {
    return null;
  }

  try {
    const raw = storage.getItem(buildStorageKey(sessionKey));
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as Partial<VolatileConversationSnapshot>;
    return {
      messages: Array.isArray(parsed.messages)
        ? parsed.messages as Message[]
        : [],
      pending_agent_slots: Array.isArray(parsed.pending_agent_slots)
        ? parsed.pending_agent_slots as RoomPendingAgentSlotState[]
        : [],
      updated_at: typeof parsed.updated_at === "number" ? parsed.updated_at : 0,
    };
  } catch (error) {
    console.debug("[conversation] Failed to parse volatile snapshot:", error);
    return null;
  }
}

export function writeVolatileConversationSnapshot(
  sessionKey: string,
  snapshot: VolatileConversationSnapshot,
): void {
  const storage = getSessionStorage();
  if (!storage) {
    return;
  }

  const cappedSnapshot = snapshot.messages.length > MAX_VOLATILE_MESSAGE_COUNT
    ? {
        ...snapshot,
        messages: snapshot.messages.slice(-MAX_VOLATILE_MESSAGE_COUNT),
      }
    : snapshot;

  try {
    storage.setItem(buildStorageKey(sessionKey), JSON.stringify(cappedSnapshot));
  } catch (error) {
    const quotaExceeded = error instanceof DOMException
      && (error.code === 22 || error.name === "QuotaExceededError");
    console.warn(
      quotaExceeded
        ? "[conversation] Volatile snapshot exceeds sessionStorage quota"
        : "[conversation] Failed to persist volatile snapshot",
      error,
    );
  }
}

export function removeVolatileConversationSnapshot(sessionKey: string): void {
  const storage = getSessionStorage();
  if (!storage) {
    return;
  }

  try {
    storage.removeItem(buildStorageKey(sessionKey));
  } catch {
    // 清理失败不影响后端会话，下一次写入仍会覆盖同一键。
  }
}
