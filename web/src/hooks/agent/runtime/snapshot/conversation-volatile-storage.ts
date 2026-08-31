/**
 * INPUT: 当前 owner generation、Session key 与可恢复的浏览器易失快照。
 * OUTPUT: 仅允许当前 owner 读写的 sessionStorage 投影，以及 owner 切换时的同步定向清理。
 * POS: Agent 会话的非 canonical 浏览器恢复层；不改变服务端 Session identity 或持久化协议。
 */
import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import { isAuthOwnerScopeGenerationCurrent } from "@/shared/auth/auth-owner-generation";

import type { VolatileConversationSnapshot } from "./conversation-volatile-model";

const VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX =
  "nexus.agent_conversation.volatile";
const MAX_VOLATILE_MESSAGE_COUNT = 200;
let volatileConversationOwnerScope: string | null = null;

function buildStorageKey(sessionKey: string): string | null {
  if (!volatileConversationOwnerScope) {
    return null;
  }
  return [
    VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX,
    String(volatileConversationOwnerScope.length),
    volatileConversationOwnerScope,
    sessionKey,
  ].join(":");
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
  ownerGeneration: number,
): VolatileConversationSnapshot | null {
  if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
    return null;
  }
  const storageKey = buildStorageKey(sessionKey);
  if (!storageKey) {
    return null;
  }
  const storage = getSessionStorage();
  if (!storage) {
    return null;
  }

  try {
    const raw = storage.getItem(storageKey);
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
  ownerGeneration: number,
): void {
  if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
    return;
  }
  const storageKey = buildStorageKey(sessionKey);
  if (!storageKey) {
    return;
  }
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
    storage.setItem(storageKey, JSON.stringify(cappedSnapshot));
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

export function removeVolatileConversationSnapshot(
  sessionKey: string,
  ownerGeneration: number,
): void {
  if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
    return;
  }
  const storageKey = buildStorageKey(sessionKey);
  if (!storageKey) {
    return;
  }
  const storage = getSessionStorage();
  if (!storage) {
    return;
  }

  try {
    storage.removeItem(storageKey);
  } catch {
    // 清理失败不影响后端会话，下一次写入仍会覆盖同一键。
  }
}

/**
 * 身份边界推进后同步清除本标签页的旧 owner 会话正文。
 * 只删除本模块拥有的精确前缀，其他 sessionStorage 数据保持不变。
 */
export function setVolatileConversationOwnerScope(
  ownerScope: string | null,
): void {
  volatileConversationOwnerScope = ownerScope;
}

export function resetVolatileConversationOwnerScope(
  nextOwnerScope: string | null,
): void {
  // 清理失败时也不能让新 owner 认领旧 key；先撤下 namespace，最后只启用新 scope。
  volatileConversationOwnerScope = null;
  const storage = getSessionStorage();
  if (storage) {
    try {
      for (let index = storage.length - 1; index >= 0; index -= 1) {
        const key = storage.key(index);
        if (
          key === VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX
          || key?.startsWith(`${VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX}:`)
        ) {
          storage.removeItem(key);
        }
      }
    } catch {
      // 保留旧 key 也不可见：新 owner 只会访问自己的 namespace。
    }
  }
  volatileConversationOwnerScope = nextOwnerScope;
}
