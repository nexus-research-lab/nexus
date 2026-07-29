/**
 * INPUT: 当前 pending permission 队列与 request 快照/完成身份。
 * OUTPUT: 同 request 原位更新、首次到达顺序稳定的下一份队列。
 * POS: permission WebSocket 事件写入 volatile conversation state 的纯状态模型。
 */
import type { PendingPermission } from "@/types/conversation/interaction/permission";

export function upsertPendingPermission(
  current: PendingPermission[],
  permission: PendingPermission,
): PendingPermission[] {
  const position = current.findIndex(
    (item) => item.request_id === permission.request_id,
  );
  if (position < 0) {
    return [...current, permission];
  }
  const next = [...current];
  next[position] = permission;
  return next;
}

export function removePendingPermission(
  current: PendingPermission[],
  requestId: string,
): PendingPermission[] {
  const next = current.filter(
    (permission) => permission.request_id !== requestId,
  );
  return next.length === current.length ? current : next;
}
