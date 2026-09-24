// INPUT: 待确认请求、历史消息与权威轮次终态。
// OUTPUT: 仅依据过期、轮次终态或精确工具结果清理的待确认队列。
// POS: DM/Room 共用的权限历史对账与过期策略。
import type { Message } from "@/types/conversation/message/entity";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

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

  const resolvedToolUseIds = new Set(
    messages.flatMap((message) => message.role === "assistant"
      ? message.content.flatMap((block) => block.type === "tool_result"
        ? [block.tool_use_id]
        : [])
      : []),
  );

  return currentPermissions.filter((permission) => {
    if (isExpired(permission)) {
      return false;
    }
    if (permission.round_id && isRoundTerminal(permission.round_id)) {
      return false;
    }
    // 宿主确认可独立于原始工具块；历史缺少匹配项不代表请求已完成。
    const toolUseId = permission.tool_use_id?.trim();
    return !toolUseId || !resolvedToolUseIds.has(toolUseId);
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
