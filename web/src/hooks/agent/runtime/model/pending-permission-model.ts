import type { Message } from "@/types";
import {
  collectUnresolvedToolUseCandidates,
  matchPendingPermissionsToToolUses,
  type PendingPermission,
} from "@/types/conversation/permission";

function getExpirationTime(permission: PendingPermission): number | null {
  if (!permission.expires_at) {
    return null;
  }
  const expiresAt = Date.parse(permission.expires_at);
  return Number.isFinite(expiresAt) ? expiresAt : null;
}

function isExpired(
  permission: PendingPermission,
  now: number = Date.now(),
): boolean {
  const expiresAt = getExpirationTime(permission);
  return expiresAt != null && expiresAt <= now;
}

export function filterPendingPermissionsFromSnapshot(
  currentPermissions: PendingPermission[],
  messages: Message[],
  isRoundTerminal: (roundId: string) => boolean,
): PendingPermission[] {
  if (currentPermissions.length === 0) {
    return currentPermissions;
  }

  const loadedAssistantMessageIds = new Set(
    messages
      .filter((message) => message.role === "assistant")
      .map((message) => message.message_id),
  );
  const matchResult = matchPendingPermissionsToToolUses(
    currentPermissions,
    collectUnresolvedToolUseCandidates(messages),
  );

  return currentPermissions.filter((permission) => {
    if (isExpired(permission)) {
      return false;
    }
    if (permission.round_id && isRoundTerminal(permission.round_id)) {
      return false;
    }
    if (matchResult.matched_request_ids.has(permission.request_id)) {
      return true;
    }
    // 旧事件缺少 messageId，无法唯一绑定，只能等待明确结果或重载收口。
    return !permission.message_id
      || !loadedAssistantMessageIds.has(permission.message_id);
  });
}

export function pruneExpiredPendingPermissions(
  currentPermissions: PendingPermission[],
  now: number = Date.now(),
): PendingPermission[] {
  if (currentPermissions.length === 0) {
    return currentPermissions;
  }

  const nextPermissions = currentPermissions.filter(
    (permission) => !isExpired(permission, now),
  );
  return nextPermissions.length === currentPermissions.length
    ? currentPermissions
    : nextPermissions;
}

export function getNextPendingPermissionTimeoutMs(
  currentPermissions: PendingPermission[],
  now: number = Date.now(),
): number | null {
  let nextTimeoutMs: number | null = null;

  for (const permission of currentPermissions) {
    const expiresAt = getExpirationTime(permission);
    if (expiresAt == null) {
      continue;
    }
    const timeoutMs = Math.max(expiresAt - now, 0);
    nextTimeoutMs = nextTimeoutMs == null
      ? timeoutMs
      : Math.min(nextTimeoutMs, timeoutMs);
  }
  return nextTimeoutMs;
}
