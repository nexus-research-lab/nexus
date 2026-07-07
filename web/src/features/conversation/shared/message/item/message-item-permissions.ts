import type { Message } from "@/types/conversation/message";
import {
  collectUnresolvedToolUseCandidates,
  matchPendingPermissionsToToolUses,
  type PendingPermission,
} from "@/types/conversation/permission";

export interface MessageItemPermissionMatch {
  matchedPendingPermissionsByToolUseId: Map<string, PendingPermission>;
  unmatchedPendingPermissions: PendingPermission[];
}

/**
 * 权限只按 message_id + tool_use_id 精确绑定（在 matchPendingPermissionsToToolUses 内），
 * 不再做单候选或跨消息补配；匹配不上的保留为未匹配卡片。
 */
export function resolveMessageItemPermissions(
  messages: Message[],
  pendingPermissions: PendingPermission[],
): MessageItemPermissionMatch {
  if (pendingPermissions.length === 0) {
    return {
      matchedPendingPermissionsByToolUseId: new Map(),
      unmatchedPendingPermissions: [],
    };
  }

  const permissionMatchResult = matchPendingPermissionsToToolUses(
    pendingPermissions,
    collectUnresolvedToolUseCandidates(messages),
  );

  return {
    matchedPendingPermissionsByToolUseId:
      permissionMatchResult.matched_permissions_by_tool_use_id,
    unmatchedPendingPermissions: permissionMatchResult.unmatched_permissions,
  };
}
