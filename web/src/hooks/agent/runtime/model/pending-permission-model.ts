// INPUT: 待确认请求、历史消息、权威轮次终态与 Room 订阅快照中的待响应请求全集。
// OUTPUT: 仅依据过期、轮次终态、精确工具结果或后端权威 pending 全集清理的待确认队列。
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

/**
 * Room 订阅快照的 `pending_interaction_request_ids` 是后端此刻仍在等待的人工交互全集。
 * 本地队列中不在全集内的请求已在后端收口，只是 `permission_request_resolved` 在断线、
 * WebView 挂起或回放缓冲溢出期间丢失，必须移除。快照之后新建的请求会以更大的 room_seq
 * 在快照后回放，因此不会被误删。自动化确认可能因读取失败缺席快照，交给其独立生命周期。
 */
export function reconcilePendingPermissionsWithAuthoritativeIds(
  currentPermissions: PendingPermission[],
  authoritativeRequestIds: readonly unknown[],
): PendingPermission[] {
  if (currentPermissions.length === 0) {
    return currentPermissions;
  }
  const liveRequestIds = new Set(
    authoritativeRequestIds.flatMap((requestId) => (
      typeof requestId === "string" && requestId.trim()
        ? [requestId.trim()]
        : []
    )),
  );
  const nextPermissions = currentPermissions.filter((permission) => (
    permission.source === "automation"
    || liveRequestIds.has(permission.request_id)
  ));
  return nextPermissions.length === currentPermissions.length
    ? currentPermissions
    : nextPermissions;
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
