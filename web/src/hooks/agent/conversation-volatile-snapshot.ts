import {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
  RoomPendingAgentSlotState,
} from "@/types";
import {
  collectUnresolvedToolUseCandidates,
  matchPendingPermissionsToToolUses,
  PendingPermission,
} from "@/types/conversation/permission";
import { AgentConversationRuntimeSnapshot } from "./agent-conversation-runtime-machine";

interface VolatileConversationSnapshot {
  messages: Message[];
  pending_agent_slots: RoomPendingAgentSlotState[];
  updated_at: number;
}

const VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX =
  "nexus.agent_conversation.volatile";

function isTerminalSlotStatus(status: AssistantMessageStatus): boolean {
  return status === "done" || status === "cancelled" || status === "error";
}

function buildVolatileConversationStorageKey(sessionKey: string): string {
  return `${VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX}:${sessionKey}`;
}

function getVolatileConversationStorage(): Storage | null {
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
  const storage = getVolatileConversationStorage();
  if (!storage) {
    return null;
  }

  try {
    const raw = storage.getItem(
      buildVolatileConversationStorageKey(sessionKey),
    );
    if (!raw) {
      return null;
    }

    const parsed = JSON.parse(
      raw,
    ) as Partial<VolatileConversationSnapshot> | null;
    if (!parsed) {
      return null;
    }

    return {
      messages: Array.isArray(parsed.messages)
        ? (parsed.messages as Message[])
        : [],
      pending_agent_slots: Array.isArray(parsed.pending_agent_slots)
        ? (parsed.pending_agent_slots as RoomPendingAgentSlotState[])
        : [],
      updated_at: typeof parsed.updated_at === "number" ? parsed.updated_at : 0,
    };
  } catch (err) {
    console.debug("[conversation] Failed to parse snapshot from sessionStorage:", err);
    return null;
  }
}

export function writeVolatileConversationSnapshot(
  sessionKey: string,
  snapshot: VolatileConversationSnapshot,
): void {
  const storage = getVolatileConversationStorage();
  if (!storage) {
    return;
  }

  // 保留最近 200 条消息以防止序列化负载过大。
  const capped: VolatileConversationSnapshot =
    snapshot.messages.length > 200
      ? { ...snapshot, messages: snapshot.messages.slice(-200) }
      : snapshot;

  try {
    storage.setItem(
      buildVolatileConversationStorageKey(sessionKey),
      JSON.stringify(capped),
    );
  } catch (err) {
    const isQuota =
      err instanceof DOMException &&
      (err.code === 22 || err.name === "QuotaExceededError");
    if (isQuota) {
      console.warn("[conversation] sessionStorage quota exceeded, snapshot not persisted");
    } else {
      console.warn("[conversation] sessionStorage write failed:", err);
    }
  }
}

export function removeVolatileConversationSnapshot(
  sessionKey: string,
): void {
  const storage = getVolatileConversationStorage();
  if (!storage) {
    return;
  }

  try {
    storage.removeItem(buildVolatileConversationStorageKey(sessionKey));
  } catch {
    // 忽略移除失败
  }
}

export function mergePendingAgentSlots(
  restoredSlots: RoomPendingAgentSlotState[],
  currentSlots: RoomPendingAgentSlotState[],
): RoomPendingAgentSlotState[] {
  if (restoredSlots.length === 0) {
    return currentSlots;
  }

  const mergedSlots = new Map<string, RoomPendingAgentSlotState>();
  for (const slot of restoredSlots) {
    mergedSlots.set(slot.msg_id, slot);
  }
  for (const slot of currentSlots) {
    mergedSlots.set(slot.msg_id, slot);
  }
  return Array.from(mergedSlots.values());
}

export function isEphemeralMessage(message: Message): boolean {
  return message.delivery_mode === "ephemeral";
}

export function buildVolatileConversationSnapshot(
  messages: Message[],
  runtimeSnapshot: AgentConversationRuntimeSnapshot,
  pendingAgentSlots: RoomPendingAgentSlotState[],
): VolatileConversationSnapshot | null {
  const activeRoundIds = new Set<string>(runtimeSnapshot.liveRoundIds);

  for (const slot of pendingAgentSlots) {
    if (!isTerminalSlotStatus(slot.status)) {
      activeRoundIds.add(slot.round_id);
    }
  }

  if (activeRoundIds.size === 0) {
    return null;
  }

  const volatileMessages = messages.filter((message) => {
    if (isEphemeralMessage(message)) {
      return false;
    }
    if (activeRoundIds.has(message.round_id)) {
      return true;
    }

    return (
      message.role === "assistant" &&
      !isTerminalSlotStatus(message.stream_status ?? "streaming")
    );
  });
  const volatileSlots = pendingAgentSlots.filter(
    (slot) => !isTerminalSlotStatus(slot.status),
  );

  if (volatileMessages.length === 0 && volatileSlots.length === 0) {
    return null;
  }

  return {
    messages: volatileMessages,
    pending_agent_slots: volatileSlots,
    updated_at: Date.now(),
  };
}

export function filterPendingSlotsFromSnapshot(
  currentSlots: RoomPendingAgentSlotState[],
  messages: Message[],
  isRoundTerminal: (roundId: string) => boolean,
): RoomPendingAgentSlotState[] {
  if (currentSlots.length === 0) {
    return currentSlots;
  }
  const loadedMessageIds = new Set(
    messages
      .filter(
        (message): message is AssistantMessage => message.role === "assistant",
      )
      .map((message) => message.message_id),
  );

  return currentSlots.filter(
    (slot) =>
      !isRoundTerminal(slot.round_id) && !loadedMessageIds.has(slot.msg_id),
  );
}

export function filterPendingPermissionsFromSnapshot(
  currentPermissions: PendingPermission[],
  messages: Message[],
  isRoundTerminal: (roundId: string) => boolean,
): PendingPermission[] {
  if (currentPermissions.length === 0) {
    return currentPermissions;
  }
  const loadedAssistantMessageIds = new Set<string>();
  const unresolvedToolUseCandidates =
    collectUnresolvedToolUseCandidates(messages);
  const permissionMatchResult = matchPendingPermissionsToToolUses(
    currentPermissions,
    unresolvedToolUseCandidates,
  );

  for (const message of messages) {
    if (message.role === "assistant") {
      loadedAssistantMessageIds.add(message.message_id);
    }
  }

  return currentPermissions.filter((permission) => {
    if (isPendingPermissionExpired(permission)) {
      return false;
    }

    if (permission.round_id && isRoundTerminal(permission.round_id)) {
      return false;
    }

    if (
      permissionMatchResult.matched_request_ids.has(permission.request_id)
    ) {
      return true;
    }

    if (!permission.message_id) {
      // 缺少 messageId 的旧权限事件无法做唯一绑定，
      // 快照阶段只能保留，等待明确的 result / reload 收口。
      return true;
    }

    return !loadedAssistantMessageIds.has(permission.message_id);
  });
}

function getPendingPermissionExpirationMs(
  permission: PendingPermission,
): number | null {
  if (!permission.expires_at) {
    return null;
  }
  const expiresAtMs = Date.parse(permission.expires_at);
  return Number.isFinite(expiresAtMs) ? expiresAtMs : null;
}

function isPendingPermissionExpired(
  permission: PendingPermission,
  nowMs: number = Date.now(),
): boolean {
  const expiresAtMs = getPendingPermissionExpirationMs(permission);
  return expiresAtMs != null && expiresAtMs <= nowMs;
}

export function pruneExpiredPendingPermissions(
  currentPermissions: PendingPermission[],
  nowMs: number = Date.now(),
): PendingPermission[] {
  if (currentPermissions.length === 0) {
    return currentPermissions;
  }

  const nextPermissions = currentPermissions.filter(
    (permission) => !isPendingPermissionExpired(permission, nowMs),
  );
  return nextPermissions.length === currentPermissions.length
    ? currentPermissions
    : nextPermissions;
}

export function getNextPendingPermissionTimeoutMs(
  currentPermissions: PendingPermission[],
  nowMs: number = Date.now(),
): number | null {
  let nextTimeoutMs: number | null = null;

  for (const permission of currentPermissions) {
    const expiresAtMs = getPendingPermissionExpirationMs(permission);
    if (expiresAtMs == null) {
      continue;
    }
    const timeoutMs = Math.max(expiresAtMs - nowMs, 0);
    if (nextTimeoutMs == null || timeoutMs < nextTimeoutMs) {
      nextTimeoutMs = timeoutMs;
    }
  }

  return nextTimeoutMs;
}
