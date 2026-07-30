/**
 * INPUT: 当前可见消息与 runtime pending requests。
 * OUTPUT: 按 request_id 收敛的稳定交互，以及与可见 tool use 的精确关联。
 * POS: MessageItem 人工介入身份与匹配的纯投影真相源。
 */
import type { Message } from "@/types/conversation/message/entity";
import {
  coalescePendingPermissions,
  matchPendingPermissionsToMessages,
} from "@/lib/conversation/pending-permission-match";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

export interface MessageItemPermissionMatch {
  matchedPendingPermissionsByToolUseId: Map<string, PendingPermission>;
  pendingInteractionPermissions: PendingPermission[];
  unmatchedPendingPermissions: PendingPermission[];
}

/**
 * 权限只按 message_id + tool_use_id 精确绑定到当前实际可见的工具块，
 * 同一 request_id 只保留一个稳定交互身份；Room 由独立交互轨道持续持有，
 * DM 仍可把精确匹配请求交给原工具块。
 */
export function resolveMessageItemPermissions(
  messages: Message[],
  pendingPermissions: PendingPermission[],
  visibleToolUseIds?: ReadonlySet<string>,
): MessageItemPermissionMatch {
  const pendingInteractionPermissions = coalescePendingPermissions(
    pendingPermissions,
  );
  if (pendingInteractionPermissions.length === 0) {
    return {
      matchedPendingPermissionsByToolUseId: new Map(),
      pendingInteractionPermissions: [],
      unmatchedPendingPermissions: [],
    };
  }

  const permissionMatchResult = matchPendingPermissionsToMessages(
    messages,
    pendingInteractionPermissions,
    { visibleToolUseIds },
  );

  return {
    matchedPendingPermissionsByToolUseId:
      permissionMatchResult.matchedByToolUseId,
    pendingInteractionPermissions,
    unmatchedPendingPermissions: permissionMatchResult.unmatchedPermissions,
  };
}
