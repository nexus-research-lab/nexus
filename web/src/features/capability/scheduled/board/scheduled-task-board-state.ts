// INPUT: 定时任务主列表的快照、加载和失败事实。
// OUTPUT: 首次加载、失败、空态或可展示快照的唯一视图状态。
// POS: Scheduled 主视图纯投影；不发请求、不判断恢复动作。

import type { ResourceFailure } from "@/lib/error-message";

export type ScheduledTaskBoardStateKind = "empty" | "error" | "loading" | "ready";

export function getScheduledTaskBoardState({
  failure,
  hasSnapshot,
  isLoading,
  itemCount,
}: {
  failure: ResourceFailure | null;
  hasSnapshot: boolean;
  isLoading: boolean;
  itemCount: number;
}): ScheduledTaskBoardStateKind {
  if (failure?.access || (failure && !hasSnapshot)) {
    return "error";
  }
  if (isLoading && !hasSnapshot) {
    return "loading";
  }
  return itemCount === 0 ? "empty" : "ready";
}
