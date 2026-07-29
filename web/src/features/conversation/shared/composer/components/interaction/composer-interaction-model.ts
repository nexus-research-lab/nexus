/**
 * INPUT: DM/Room runtime 尚未完成的人工交互请求。
 * OUTPUT: 按首次到达顺序收敛、一次只推进一个请求的 Composer 替换面模型。
 * POS: pending interaction 到共享 Composer 视图之间的纯投影边界。
 */
import { coalescePendingPermissions } from "@/lib/conversation/pending-permission-match";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import { ASK_USER_QUESTION_TOOL_NAME } from "../../../message/message-tool-names";

export type ComposerInteractionKind = "permission" | "plan" | "question";

export interface ComposerInteractionQueue {
  current: PendingPermission | null;
  items: PendingPermission[];
  kind: ComposerInteractionKind | null;
  total: number;
}

const PLAN_TOOL_NAMES = new Set(["EnterPlanMode", "ExitPlanMode"]);

export function buildComposerInteractionQueue(
  permissions: readonly PendingPermission[],
): ComposerInteractionQueue {
  const items = coalescePendingPermissions(permissions);
  const current = items[0] ?? null;
  return {
    current,
    items,
    kind: current ? resolveComposerInteractionKind(current) : null,
    total: items.length,
  };
}

export function resolveComposerInteractionKind(
  permission: PendingPermission,
): ComposerInteractionKind {
  if (
    permission.interaction_mode === "question"
    || permission.tool_name === ASK_USER_QUESTION_TOOL_NAME
  ) {
    return "question";
  }
  return PLAN_TOOL_NAMES.has(permission.tool_name) ? "plan" : "permission";
}
