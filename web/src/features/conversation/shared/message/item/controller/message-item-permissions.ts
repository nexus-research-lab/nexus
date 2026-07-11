import type { Message } from "@/types/conversation/message/entity";
import { matchPendingPermissionsToMessages } from "@/lib/conversation/pending-permission-match";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

export interface MessageItemPermissionMatch {
  matchedPendingPermissionsByToolUseId: Map<string, PendingPermission>;
  unmatchedPendingPermissions: PendingPermission[];
}

/**
 * 权限只按 message_id + tool_use_id 精确绑定，
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

  const permissionMatchResult = matchPendingPermissionsToMessages(
    messages,
    pendingPermissions,
  );

  return {
    matchedPendingPermissionsByToolUseId:
      permissionMatchResult.matchedByToolUseId,
    unmatchedPendingPermissions: permissionMatchResult.unmatchedPermissions,
  };
}
